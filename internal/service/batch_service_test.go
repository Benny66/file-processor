package service

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"file-processor/internal/apperr"
	"file-processor/internal/imageproc"
)

func TestProcessBatchConvertsAndNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "photo.png")
	srcBytes := batchTestPNG(t, 20, 10)
	if err := os.WriteFile(src, srcBytes, 0o644); err != nil {
		t.Fatalf("写入源文件失败: %v", err)
	}

	svc := &Service{}
	var progress []BatchProgress
	result, err := svc.ProcessBatch(
		BatchRequest{
			Paths: []string{src},
			Rules: BatchRules{OutputFormat: imageproc.OutputJPEG, MaxWidth: 200, MaxHeight: 200},
		},
		func(p BatchProgress) { progress = append(progress, p) },
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if result.Success != 1 || result.Failed != 0 || !result.OK {
		t.Fatalf("批次结果不对：%+v", result)
	}
	item := result.Items[0]
	if item.Status != StatusSuccess {
		t.Fatalf("item 状态 = %q，期望 success", item.Status)
	}
	if item.OutputPath != filepath.Join(dir, "photo-out.jpg") {
		t.Errorf("输出路径 = %q", item.OutputPath)
	}
	if item.SourceBytes != int64(len(srcBytes)) || item.OutputBytes <= 0 {
		t.Errorf("体积统计不对：%+v", item)
	}
	if item.Width != 20 || item.Height != 10 {
		t.Errorf("输出尺寸 = %dx%d，期望 20x10", item.Width, item.Height)
	}
	if len(progress) != 1 || progress[0].Done != 1 || progress[0].Total != 1 ||
		progress[0].Current == nil || progress[0].Current.Status != StatusSuccess {
		t.Errorf("进度事件不对：%+v", progress)
	}

	firstOutput, err := os.ReadFile(item.OutputPath)
	if err != nil {
		t.Fatalf("读取第一个输出失败: %v", err)
	}
	sourceNow, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("读取源文件失败: %v", err)
	}
	if string(sourceNow) != string(srcBytes) {
		t.Fatalf("源文件被修改")
	}

	result2, err := svc.ProcessBatch(
		BatchRequest{
			Paths: []string{src},
			Rules: BatchRules{OutputFormat: imageproc.OutputJPEG, MaxWidth: 200, MaxHeight: 200},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("第二次 ProcessBatch 失败: %v", err)
	}
	secondPath := result2.Items[0].OutputPath
	if secondPath != filepath.Join(dir, "photo-out-2.jpg") {
		t.Errorf("冲突输出路径 = %q，期望 -out-2", secondPath)
	}
	firstOutputNow, err := os.ReadFile(item.OutputPath)
	if err != nil {
		t.Fatalf("读取第一个输出失败: %v", err)
	}
	if string(firstOutputNow) != string(firstOutput) {
		t.Errorf("已有输出被覆盖")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取输出目录失败: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".imageworkshop-") {
			t.Errorf("残留临时文件: %s", entry.Name())
		}
	}
}

func TestProcessBatchContinuesAfterFailure(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "ok.png")
	corrupt := filepath.Join(dir, "bad.jpg")
	validBytes := batchTestPNG(t, 20, 10)
	if err := os.WriteFile(valid, validBytes, 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	if err := os.WriteFile(corrupt, []byte("not an image"), 0o644); err != nil {
		t.Fatalf("写入损坏文件失败: %v", err)
	}

	svc := &Service{}
	result, err := svc.ProcessBatch(
		BatchRequest{
			Paths: []string{corrupt, valid},
			Rules: BatchRules{OutputFormat: imageproc.OutputJPEG},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if result.Success != 1 || result.Failed != 1 || result.OK {
		t.Fatalf("批次统计不对：%+v", result)
	}
	if len(result.Items) != 2 {
		t.Fatalf("结果条数 = %d", len(result.Items))
	}
	if result.Items[0].Status != StatusFailed || !strings.Contains(result.Items[0].Message, "无法识别") {
		t.Errorf("损坏文件结果不对：%+v", result.Items[0])
	}
	if result.Items[1].Status != StatusSuccess || result.Items[1].OutputPath == "" {
		t.Errorf("有效文件结果不对：%+v", result.Items[1])
	}
	sourceNow, err := os.ReadFile(valid)
	if err != nil {
		t.Fatalf("读取源文件失败: %v", err)
	}
	if string(sourceNow) != string(validBytes) {
		t.Errorf("源文件被修改")
	}
}

func TestProcessBatchBoxSkipsCompliantAndResizesOversized(t *testing.T) {
	dir := t.TempDir()
	within := filepath.Join(dir, "within.png")
	outside := filepath.Join(dir, "outside.png")
	same := filepath.Join(dir, "same.png")
	if err := os.WriteFile(within, batchTestPNG(t, 100, 50), 0o644); err != nil {
		t.Fatalf("写入 within 失败: %v", err)
	}
	outsideBytes := batchTestPNG(t, 400, 300)
	if err := os.WriteFile(outside, outsideBytes, 0o644); err != nil {
		t.Fatalf("写入 outside 失败: %v", err)
	}
	if err := os.WriteFile(same, batchTestPNG(t, 100, 50), 0o644); err != nil {
		t.Fatalf("写入 same 失败: %v", err)
	}

	svc := &Service{}
	result, err := svc.ProcessBatch(
		BatchRequest{
			Paths: []string{within, outside, same},
			Rules: BatchRules{OutputFormat: imageproc.OutputKeep, MaxWidth: 200, MaxHeight: 200},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if result.Success != 1 || result.Skipped != 2 || result.Failed != 0 {
		t.Fatalf("批次统计不对：%+v", result)
	}
	if result.Items[0].Status != StatusSkipped || result.Items[0].Message != "已达标" {
		t.Errorf("within 应跳过：%+v", result.Items[0])
	}
	resized := result.Items[1]
	if resized.Status != StatusSuccess || resized.OutputPath != filepath.Join(dir, "outside-out.png") {
		t.Errorf("outside 应缩放成功：%+v", resized)
	}
	if resized.Width != 200 || resized.Height != 150 {
		t.Errorf("outside 输出尺寸 = %dx%d，期望 200x150", resized.Width, resized.Height)
	}
	if result.Items[2].Status != StatusSkipped {
		t.Errorf("same 应跳过：%+v", result.Items[2])
	}

	if _, err := os.Stat(filepath.Join(dir, "within-out.png")); !os.IsNotExist(err) {
		t.Errorf("跳过项不应产出文件")
	}
	if _, err := os.Stat(resized.OutputPath); err != nil {
		t.Errorf("缩放输出不存在: %v", err)
	}
	after, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("读取源文件失败: %v", err)
	}
	if string(after) != string(outsideBytes) {
		t.Errorf("源文件被修改")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取输出目录失败: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".imageworkshop-") {
			t.Errorf("残留临时文件: %s", entry.Name())
		}
	}
}

func TestProcessBatchTargetSize(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "noise.png")
	if err := os.WriteFile(src, batchNoisePNG(t, 400, 300), 0o644); err != nil {
		t.Fatalf("写入源文件失败: %v", err)
	}
	svc := &Service{}
	result, err := svc.ProcessBatch(
		BatchRequest{
			Paths: []string{src},
			Rules: BatchRules{OutputFormat: imageproc.OutputJPEG, TargetBytes: 30 * 1024},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("批次统计不对：%+v", result)
	}
	item := result.Items[0]
	if item.Status != StatusSuccess || item.OutputPath == "" {
		t.Fatalf("结果不对：%+v", item)
	}
	if item.OutputBytes > 30*1024 {
		t.Errorf("目标体积未达成：%d > %d", item.OutputBytes, 30*1024)
	}
	if result.ExtremeDegraded != 0 {
		t.Errorf("30KB 目标不应极端降级：%+v", item)
	}
}

func TestProcessBatchTargetExtreme(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "noise.png")
	if err := os.WriteFile(src, batchNoisePNG(t, 400, 300), 0o644); err != nil {
		t.Fatalf("写入源文件失败: %v", err)
	}
	svc := &Service{}
	result, err := svc.ProcessBatch(
		BatchRequest{
			Paths: []string{src},
			Rules: BatchRules{
				OutputFormat: imageproc.OutputPNG,
				MaxWidth:     200,
				MaxHeight:    200,
				TargetBytes:  1024,
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if result.Success != 1 || result.ExtremeDegraded != 1 {
		t.Fatalf("极端降级统计不对：%+v", result)
	}
	item := result.Items[0]
	if !item.ExtremeDegraded {
		t.Errorf("item 未标记极端降级：%+v", item)
	}
	if item.Width > 200 || item.Height > 200 {
		t.Errorf("输出超出宽高盒：%dx%d", item.Width, item.Height)
	}
	if item.OutputBytes > 1024 {
		t.Errorf("目标体积未达成：%d", item.OutputBytes)
	}
}

func TestProcessBatchTargetSkipsSmallSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "small.png")
	srcBytes := batchTestPNG(t, 4, 4)
	if err := os.WriteFile(src, srcBytes, 0o644); err != nil {
		t.Fatalf("写入源文件失败: %v", err)
	}
	svc := &Service{}
	result, err := svc.ProcessBatch(
		BatchRequest{
			Paths: []string{src},
			Rules: BatchRules{OutputFormat: imageproc.OutputKeep, TargetBytes: 1024},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if result.Skipped != 1 || result.Success != 0 {
		t.Fatalf("批次统计不对：%+v", result)
	}
	item := result.Items[0]
	if item.Status != StatusSkipped || item.Message != "已达标" || item.OutputPath != "" {
		t.Errorf("应已达标跳过：%+v", item)
	}
	if _, err := os.Stat(filepath.Join(dir, "small-out.png")); !os.IsNotExist(err) {
		t.Errorf("跳过项不应产出文件")
	}
	after, err := os.ReadFile(src)
	if err != nil || string(after) != string(srcBytes) {
		t.Errorf("源文件被修改")
	}
}

func TestProcessBatchTargetWithBox(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "noise.png")
	if err := os.WriteFile(src, batchNoisePNG(t, 800, 600), 0o644); err != nil {
		t.Fatalf("写入源文件失败: %v", err)
	}
	svc := &Service{}
	result, err := svc.ProcessBatch(
		BatchRequest{
			Paths: []string{src},
			Rules: BatchRules{
				OutputFormat: imageproc.OutputJPEG,
				MaxWidth:     300,
				MaxHeight:    300,
				TargetBytes:  20 * 1024,
			},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("批次统计不对：%+v", result)
	}
	item := result.Items[0]
	if item.Width > 300 || item.Height > 300 {
		t.Errorf("输出超出宽高盒：%dx%d", item.Width, item.Height)
	}
	if item.OutputBytes > 20*1024 {
		t.Errorf("目标体积未达成：%d", item.OutputBytes)
	}
}

func TestProcessBatchRejectsUnsupportedRules(t *testing.T) {
	src := filepath.Join(t.TempDir(), "ok.png")
	if err := os.WriteFile(src, batchTestPNG(t, 10, 10), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	svc := &Service{}

	cases := []BatchRules{
		{OutputFormat: "gif"},
		{OutputFormat: imageproc.OutputPNG, TargetBytes: 512},
		{OutputFormat: imageproc.OutputPNG, TargetBytes: 1073741825},
		{OutputFormat: imageproc.OutputPNG, MaxWidth: 100},
		{OutputFormat: imageproc.OutputPNG, MaxHeight: 100},
		{OutputFormat: imageproc.OutputPNG, MaxWidth: 100, MaxHeight: 0},
		{OutputFormat: imageproc.OutputPNG, MaxWidth: 70000, MaxHeight: 100},
		{OutputFormat: imageproc.OutputPNG, MaxWidth: 100, MaxHeight: -1},
	}
	for _, rules := range cases {
		_, err := svc.ProcessBatch(BatchRequest{Paths: []string{src}, Rules: rules}, nil)
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Code != apperr.CodeValidation {
			t.Errorf("规则 %+v 未被拒绝：%v", rules, err)
		}
	}
}

func TestProcessBatchCancelsRemaining(t *testing.T) {
	dir := t.TempDir()
	data := batchTestPNG(t, 800, 600)
	paths := make([]string, 0, 12)
	for i := 0; i < 12; i++ {
		path := filepath.Join(dir, "image-"+string(rune('a'+i))+".png")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("写入测试文件失败: %v", err)
		}
		paths = append(paths, path)
	}

	svc := &Service{}
	var once sync.Once
	resultCh := make(chan BatchResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := svc.ProcessBatch(
			BatchRequest{Paths: paths, Rules: BatchRules{OutputFormat: imageproc.OutputJPEG}},
			func(p BatchProgress) {
				once.Do(func() { svc.CancelBatch() })
			},
		)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case result := <-resultCh:
		if len(result.Items) != len(paths) {
			t.Fatalf("结果条数 = %d，期望 %d", len(result.Items), len(paths))
		}
		if result.Canceled == 0 {
			t.Fatalf("没有取消任何剩余文件：%+v", result)
		}
		if result.Success+result.Failed+result.Canceled != len(paths) {
			t.Errorf("状态计数不守恒：%+v", result)
		}
		if result.OK {
			t.Errorf("取消的批次不应 OK")
		}
	case err := <-errCh:
		t.Fatalf("ProcessBatch 失败: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("ProcessBatch 超时")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取输出目录失败: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".imageworkshop-") {
			t.Errorf("残留临时文件: %s", entry.Name())
		}
	}
	svc.CancelBatch()
}

func batchNoisePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint32(x*73856093) ^ uint32(y*19349663)
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(v), G: uint8(v >> 8), B: uint8(v >> 16), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

func batchTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}
