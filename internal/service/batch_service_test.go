package service

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"log/slog"
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

	svc := newBatchTestService(t)
	var progress []BatchProgress
	result, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(src),
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
	// US-59（ADR-0007）：输出进暂存区（history/<批次>/0001.jpg），原图文件夹零写入。
	stagingWant := filepath.Join(svc.historyRoot, result.BatchKey, "0001.jpg")
	if item.OutputPath != stagingWant {
		t.Errorf("暂存路径 = %q，期望 %q", item.OutputPath, stagingWant)
	}
	if item.SourceBytes != int64(len(srcBytes)) || item.OutputBytes <= 0 {
		t.Errorf("体积统计不对：%+v", item)
	}
	if item.Width != 20 || item.Height != 10 {
		t.Errorf("输出尺寸 = %dx%d，期望 20x10", item.Width, item.Height)
	}
	// US-14 状态机：开工先播一帧「处理中」，完成再播一帧结果。
	// 「处理中」只能由 Go 发——两个 worker 并行，前端从完成事件反推不出谁在跑。
	if len(progress) != 2 {
		t.Fatalf("进度事件帧数 = %d，期望 2（started + current）：%+v", len(progress), progress)
	}
	if p0 := progress[0]; p0.Current != nil || p0.Started == nil ||
		p0.Started.Status != StatusProcessing || p0.Started.Path != src || p0.Done != 0 {
		t.Errorf("开工帧不对：%+v", progress[0])
	}
	if p1 := progress[1]; p1.Started != nil || p1.Done != 1 || p1.Total != 1 ||
		p1.Current == nil || p1.Current.Status != StatusSuccess {
		t.Errorf("完成帧不对：%+v", progress[1])
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
			Items: batchItems(src),
			Rules: BatchRules{OutputFormat: imageproc.OutputJPEG, MaxWidth: 200, MaxHeight: 200},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("第二次 ProcessBatch 失败: %v", err)
	}
	// 第二次处理是新批次：新的暂存目录，两次结果互不覆盖；防覆盖语义在下载层验证。
	secondPath := result2.Items[0].OutputPath
	if secondPath == item.OutputPath || result2.BatchKey == result.BatchKey {
		t.Errorf("第二次处理应产生新暂存目录：%q vs %q", secondPath, item.OutputPath)
	}
	firstOutputNow, err := os.ReadFile(item.OutputPath)
	if err != nil {
		t.Fatalf("读取第一个输出失败: %v", err)
	}
	if string(firstOutputNow) != string(firstOutput) {
		t.Errorf("已有暂存输出被覆盖")
	}

	// 源目录零写入（US-59）：除了源文件本身，不应出现任何新文件。
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取输出目录失败: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "photo.png" {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Errorf("源目录应只有源文件，实际：%v", names)
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

	svc := newBatchTestService(t)
	result, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(corrupt, valid),
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
	if result.Items[0].Status != StatusFailed || !strings.Contains(result.Items[0].Message, "认不出") {
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

// TestProcessBatchLogsEachFailure US-26：每个失败都要写入日志（路径 + 原因），
// 且日志不得包含图片内容。断言的是【日志产物】，不是「函数没报错」。
func TestProcessBatchLogsEachFailure(t *testing.T) {
	dir := t.TempDir()
	corrupt := filepath.Join(dir, "bad.jpg")
	body := "fake-image-bytes"
	if err := os.WriteFile(corrupt, []byte(body), 0o644); err != nil {
		t.Fatalf("写入损坏文件失败: %v", err)
	}

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	svc := newBatchTestService(t)
	if _, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(corrupt),
			Rules: BatchRules{OutputFormat: imageproc.OutputJPEG},
		},
		nil,
	); err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}

	logged := buf.String()
	if !strings.Contains(logged, "图片处理失败") {
		t.Errorf("失败没有落日志，日志内容：%q", logged)
	}
	if !strings.Contains(logged, corrupt) {
		t.Errorf("日志缺路径：%q", logged)
	}
	if !strings.Contains(logged, "认不出") {
		t.Errorf("日志缺原因：%q", logged)
	}
	if strings.Contains(logged, body) {
		t.Errorf("日志包含了文件内容：%q", logged)
	}
}

// TestProcessBatchReportsAlphaFlattened US-20：铺白这个事实要透到队列项上，
// 否则前端的「透明背景已铺白」提示无从产生（判定在 imageproc，服务层只透传）。
func TestProcessBatchReportsAlphaFlattened(t *testing.T) {
	dir := t.TempDir()
	src := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if x < 4 {
				src.SetNRGBA(x, y, color.NRGBA{}) // 左半透明
			} else {
				src.SetNRGBA(x, y, color.NRGBA{R: 200, G: 40, B: 40, A: 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("编码测试图失败: %v", err)
	}
	path := filepath.Join(dir, "alpha.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	svc := newBatchTestService(t)
	result, err := svc.ProcessBatch(
		BatchRequest{Items: batchItems(path), Rules: BatchRules{OutputFormat: imageproc.OutputJPEG}},
		nil,
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if result.Items[0].Status != StatusSuccess {
		t.Fatalf("未成功：%+v", result.Items[0])
	}
	if !result.Items[0].AlphaFlattened {
		t.Errorf("透明图转 JPEG 未回传 AlphaFlattened：%+v", result.Items[0])
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

	svc := newBatchTestService(t)
	result, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(within, outside, same),
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
	if result.Items[0].Status != StatusSkipped || result.Items[0].Message != "无需处理（原图已经合适）" {
		t.Errorf("within 应跳过：%+v", result.Items[0])
	}
	resized := result.Items[1]
	if resized.Status != StatusSuccess || resized.OutputPath == "" {
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
	svc := newBatchTestService(t)
	result, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(src),
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
	svc := newBatchTestService(t)
	result, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(src),
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
	svc := newBatchTestService(t)
	result, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(src),
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
	if item.Status != StatusSkipped || item.Message != "无需处理（原图已经合适）" || item.OutputPath != "" {
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
	svc := newBatchTestService(t)
	result, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(src),
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
	svc := newBatchTestService(t)

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
		_, err := svc.ProcessBatch(BatchRequest{Items: batchItems(src), Rules: rules}, nil)
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

	svc := newBatchTestService(t)
	var once sync.Once
	resultCh := make(chan BatchResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := svc.ProcessBatch(
			BatchRequest{Items: batchItems(paths...), Rules: BatchRules{OutputFormat: imageproc.OutputJPEG}},
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

// TestProcessBatchAutoFormatPicksSmallest US-49：auto 逐格式跑完整转换取最小体积者，
// 并把所选格式随 Message 回传；无目标体积时三个候选都可达标，取字节数最小。
func TestProcessBatchAutoFormatPicksSmallest(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "noise.png")
	if err := os.WriteFile(src, batchNoisePNG(t, 400, 300), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	// 基准：三种格式各自单跑的输出体积，auto 的结果必须等于三者最小值。
	sizes := map[string]int{}
	for _, format := range []string{imageproc.OutputJPEG, imageproc.OutputWebP, imageproc.OutputPNG} {
		converted, err := imageproc.Convert(batchTestSource(t, src), src, imageproc.ConvertOptions{Format: format})
		if err != nil {
			t.Fatalf("基准转换 %s 失败: %v", format, err)
		}
		sizes[format] = len(converted.Data)
	}
	minSize := sizes[imageproc.OutputJPEG]
	for _, size := range sizes {
		if size < minSize {
			minSize = size
		}
	}

	svc := newBatchTestService(t)
	result, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(src),
			Rules: BatchRules{OutputFormat: imageproc.OutputAuto},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	item := result.Items[0]
	if item.Status != StatusSuccess {
		t.Fatalf("auto 未成功：%+v", item)
	}
	if int(item.OutputBytes) != minSize {
		t.Errorf("auto 输出体积 = %d，期望三格式最小值 %d（%v）", item.OutputBytes, minSize, sizes)
	}
	if !strings.Contains(item.Message, "已自动选了最省空间的格式") {
		t.Errorf("auto 未回传所选格式：%+v", item)
	}
}

// TestProcessBatchAutoSkipsCompliant D-42：auto 无格式偏好，目标体积与尺寸盒
// 已满足即跳过——源文件已达标时产出新文件反而更大。
func TestProcessBatchAutoSkipsCompliant(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "small.png")
	if err := os.WriteFile(src, batchTestPNG(t, 100, 50), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	svc := newBatchTestService(t)
	result, err := svc.ProcessBatch(
		BatchRequest{
			Items: batchItems(src),
			Rules: BatchRules{OutputFormat: imageproc.OutputAuto, TargetBytes: 1024 * 1024},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	item := result.Items[0]
	if item.Status != StatusSkipped || item.Message != "无需处理（原图已经合适）" || item.OutputPath != "" {
		t.Fatalf("auto 应按体积达标跳过：%+v", item)
	}
	if _, err := os.Stat(filepath.Join(dir, "small-out.png")); !os.IsNotExist(err) {
		t.Errorf("跳过项不应产出文件")
	}
}

// batchTestSource 读回测试图片的字节（auto 基准转换用）。
func batchTestSource(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取测试图片失败: %v", err)
	}
	return data
}

// batchItems 把绝对路径列表装进批次请求（RelPath 为空：source / custom_flat 模式用不到）。
// newBatchTestService 返回注入了暂存目录的批次服务（ADR-0007 后批次必须先有暂存区）。
func newBatchTestService(t *testing.T) *Service {
	t.Helper()
	svc := &Service{}
	svc.SetHistoryRoot(t.TempDir())
	return svc
}

func batchItems(paths ...string) []BatchItem {
	items := make([]BatchItem, len(paths))
	for i, path := range paths {
		items[i] = BatchItem{Path: path}
	}
	return items
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
