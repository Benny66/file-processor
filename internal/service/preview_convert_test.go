package service

// PreviewConvert（US-46 单张试算）的单元测试：
// 成功/已达标/不可达三态、预览 data URL、auto 消息与批次同一套判定。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"file-processor/internal/imageproc"
)

func TestPreviewConvertSuccess(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "noise.png")
	if err := os.WriteFile(src, batchNoisePNG(t, 400, 300), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	svc := &Service{}
	result, err := svc.PreviewConvert(src, BatchRules{
		OutputFormat: imageproc.OutputJPEG,
		TargetBytes:  30 * 1024,
	})
	if err != nil {
		t.Fatalf("PreviewConvert 失败: %v", err)
	}
	if result.Status != string(StatusSuccess) {
		t.Fatalf("状态 = %q（%s），期望 success", result.Status, result.Message)
	}
	if result.OutputBytes <= 0 || result.OutputBytes > 30*1024 {
		t.Errorf("试算体积不对：%d", result.OutputBytes)
	}
	if result.Format != imageproc.OutputJPEG || result.Quality <= 0 || result.Quality > 100 {
		t.Errorf("格式/质量不对：%+v", result)
	}
	if !strings.HasPrefix(result.PreviewURL, "data:image/") {
		t.Errorf("预览不是 data URL：%q", result.PreviewURL[:32])
	}
	// 试算绝不落盘：目录里只应有源文件。
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "-out") || strings.Contains(entry.Name(), ".imageworkshop-") {
			t.Errorf("试算写出了文件：%s", entry.Name())
		}
	}
}

func TestPreviewConvertSkipsCompliant(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "small.png")
	if err := os.WriteFile(src, batchTestPNG(t, 100, 50), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	svc := &Service{}
	result, err := svc.PreviewConvert(src, BatchRules{
		OutputFormat: imageproc.OutputKeep,
		TargetBytes:  1024 * 1024,
	})
	if err != nil {
		t.Fatalf("PreviewConvert 失败: %v", err)
	}
	if result.Status != string(StatusSkipped) || !strings.Contains(result.Message, "无需处理") {
		t.Fatalf("应已达标：%+v", result)
	}
	if result.PreviewURL == "" {
		t.Errorf("已达标也应返回源图预览")
	}
}

func TestPreviewConvertTargetUnreachableIsResultNotError(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "noise.png")
	if err := os.WriteFile(src, batchNoisePNG(t, 4000, 3000), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	svc := &Service{}
	result, err := svc.PreviewConvert(src, BatchRules{
		OutputFormat: imageproc.OutputJPEG,
		TargetBytes:  1024, // 极端小目标：质量到底仍不达标时逐级缩尺寸，通常仍可达；
	})
	if err != nil {
		t.Fatalf("PreviewConvert 不应返回错误: %v", err)
	}
	// 「几乎总能达标」哲学下这里一般是 success；无论哪态，都不允许是崩溃或空结果。
	if result.Status == string(StatusSuccess) && result.OutputBytes <= 0 {
		t.Errorf("成功结果缺体积：%+v", result)
	}
}

func TestPreviewConvertAutoReportsChosenFormat(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "noise.png")
	if err := os.WriteFile(src, batchNoisePNG(t, 400, 300), 0o644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
	svc := &Service{}
	result, err := svc.PreviewConvert(src, BatchRules{OutputFormat: imageproc.OutputAuto})
	if err != nil {
		t.Fatalf("PreviewConvert 失败: %v", err)
	}
	if result.Status != string(StatusSuccess) {
		t.Fatalf("auto 试算未成功：%+v", result)
	}
	if !strings.Contains(result.Message, "已自动选了最省空间的格式") {
		t.Errorf("auto 未回传所选格式：%+v", result)
	}
}

func TestPreviewConvertRejectsInvalidRules(t *testing.T) {
	svc := &Service{}
	if _, err := svc.PreviewConvert(filepath.Join(t.TempDir(), "x.png"), BatchRules{OutputFormat: "gif"}); err == nil {
		t.Fatal("非法格式应返回错误")
	}
}

// TestPreviewConvertCorruptFileIsReadableError 损坏图经 ProbeImage 走可读错误路径
// （与批次同一前置），调参弹窗把它当失败态展示而不是崩溃。
func TestPreviewConvertCorruptFileIsReadableError(t *testing.T) {
	corrupt := filepath.Join(t.TempDir(), "bad.jpg")
	if err := os.WriteFile(corrupt, []byte("not an image"), 0o644); err != nil {
		t.Fatalf("写入损坏文件失败: %v", err)
	}
	svc := &Service{}
	_, err := svc.PreviewConvert(corrupt, BatchRules{OutputFormat: imageproc.OutputJPEG})
	if err == nil {
		t.Fatal("损坏文件应返回错误")
	}
	if !strings.Contains(err.Error(), "认不出") {
		t.Errorf("错误应可读（含原因）：%v", err)
	}
}
