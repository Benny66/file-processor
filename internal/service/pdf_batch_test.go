package service

// compress-pdf 变更的批次级测试：分流、暂存、跳过语义、纯 PDF 批免规则。
// 图像压缩引擎的真压缩断言在 imageproc/pdf_test.go，这里用桩隔离批次行为。

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"file-processor/internal/apperr"
	"file-processor/internal/imageproc"
)

// stubCompressPDF 注入 PDF 压缩桩，测试结束后还原。
func stubCompressPDF(t *testing.T, fn func(ctx context.Context, src []byte) ([]byte, imageproc.PDFStats, error)) {
	t.Helper()
	old := compressPDFFunc
	compressPDFFunc = fn
	t.Cleanup(func() { compressPDFFunc = old })
}

// batchTestPDF 落一个可嗅探（%PDF- 头）的假 PDF；真压缩由桩接管，内容无所谓。
func batchTestPDF(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("%PDF-1.4\n%fake fixture for batch routing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProcessBatchCompressesPDF(t *testing.T) {
	dir := t.TempDir()
	src := batchTestPDF(t, dir, "doc.pdf")
	svc := newBatchTestService(t)
	stubCompressPDF(t, func(_ context.Context, data []byte) ([]byte, imageproc.PDFStats, error) {
		if !imageproc.IsPDF(data) {
			t.Errorf("桩收到的不是 PDF 字节")
		}
		return append([]byte(nil), data[:len(data)/2]...), imageproc.PDFStats{ImagesTotal: 2, ImagesReplaced: 1, ImagesNotSmaller: 1}, nil
	})

	result, err := svc.ProcessBatch(BatchRequest{
		Items: batchItems(src),
		// 图片规则对 PDF 不适用，但给了也不该挡批次（分流后直接走压缩管线）。
		Rules: BatchRules{OutputFormat: imageproc.OutputJPEG},
	}, nil)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if result.Success != 1 || !result.OK {
		t.Fatalf("批次结果不对：%+v", result)
	}
	item := result.Items[0]
	if item.Status != StatusSuccess || item.Format != "pdf" {
		t.Fatalf("PDF 项应成功且 format=pdf：%+v", item)
	}
	if want := filepath.Join(svc.historyRoot, result.BatchKey, "0001.pdf"); item.OutputPath != want {
		t.Errorf("暂存路径 = %q，期望 %q", item.OutputPath, want)
	}
	if item.Message != "已压缩 PDF（重新保存了 1 张图片，另 1 张已经很省空间，保持原样）" {
		t.Errorf("结果文案不对：%q", item.Message)
	}
	if item.Width != 0 || item.Height != 0 {
		t.Errorf("PDF 不应有像素尺寸：%+v", item)
	}
	if _, err := os.Stat(item.OutputPath); err != nil {
		t.Errorf("暂存文件应存在: %v", err)
	}
}

func TestProcessBatchPDFAlreadyCompactSkips(t *testing.T) {
	dir := t.TempDir()
	src := batchTestPDF(t, dir, "doc.pdf")
	svc := newBatchTestService(t)
	stubCompressPDF(t, func(_ context.Context, data []byte) ([]byte, imageproc.PDFStats, error) {
		return data, imageproc.PDFStats{}, nil // 压不动：原样奉还
	})

	result, err := svc.ProcessBatch(BatchRequest{Items: batchItems(src)}, nil)
	if err != nil {
		t.Fatalf("ProcessBatch 失败: %v", err)
	}
	if result.Skipped != 1 || result.Success != 0 {
		t.Fatalf("应跳过：%+v", result)
	}
	item := result.Items[0]
	if item.Status != StatusSkipped || item.Message != "无需处理（PDF 已经很紧凑）" {
		t.Errorf("跳过语义不对：%+v", item)
	}
	if item.OutputPath != "" {
		t.Errorf("紧凑 PDF 不应产出文件：%q", item.OutputPath)
	}
}

func TestProcessBatchPDFErrorFailsItem(t *testing.T) {
	dir := t.TempDir()
	src := batchTestPDF(t, dir, "enc.pdf")
	svc := newBatchTestService(t)
	stubCompressPDF(t, func(_ context.Context, _ []byte) ([]byte, imageproc.PDFStats, error) {
		return nil, imageproc.PDFStats{}, apperr.New(apperr.CodePDFEncrypted, "这个 PDF 加了密，暂不支持")
	})

	result, err := svc.ProcessBatch(BatchRequest{Items: batchItems(src)}, nil)
	if err != nil {
		t.Fatalf("单项失败不应让整批报错: %v", err)
	}
	if result.Failed != 1 || result.OK {
		t.Fatalf("应失败一项：%+v", result)
	}
	if result.Items[0].Message != "这个 PDF 加了密，暂不支持" {
		t.Errorf("失败文案不对：%q", result.Items[0].Message)
	}
}

func TestProcessBatchAllPDFSkipsRuleRequirement(t *testing.T) {
	dir := t.TempDir()
	src := batchTestPDF(t, dir, "doc.pdf")
	svc := newBatchTestService(t)
	stubCompressPDF(t, func(_ context.Context, data []byte) ([]byte, imageproc.PDFStats, error) {
		return append([]byte(nil), data[:len(data)/2]...), imageproc.PDFStats{}, nil
	})

	// 全零规则（OutputFormat 空串亦非合法图片格式）：纯 PDF 批必须放行。
	result, err := svc.ProcessBatch(BatchRequest{Items: batchItems(src)}, nil)
	if err != nil {
		t.Fatalf("纯 PDF 批不应要求规则: %v", err)
	}
	if result.Success != 1 {
		t.Fatalf("纯 PDF 批应成功：%+v", result)
	}
}

func TestProcessBatchMixedImageAndPDF(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "photo.png")
	if err := os.WriteFile(img, batchTestPNG(t, 400, 300), 0o644); err != nil {
		t.Fatal(err)
	}
	pdf := batchTestPDF(t, dir, "doc.pdf")
	svc := newBatchTestService(t)
	stubCompressPDF(t, func(_ context.Context, data []byte) ([]byte, imageproc.PDFStats, error) {
		return append([]byte(nil), data[:len(data)/2]...), imageproc.PDFStats{}, nil
	})

	result, err := svc.ProcessBatch(BatchRequest{
		Items: batchItems(img, pdf),
		// 混批必须给图片规则（纯图片项仍在场）；盒 200 让图片真被缩放而非「无需处理」。
		Rules: BatchRules{OutputFormat: imageproc.OutputKeep, MaxWidth: 200, MaxHeight: 200},
	}, nil)
	if err != nil {
		t.Fatalf("混批失败: %v", err)
	}
	if result.Success != 2 {
		t.Fatalf("两项都应成功：%+v", result)
	}
	// 图片项产物是 png（keep + 已在盒内 → 缩放路径产出），PDF 项产物是 pdf。
	for _, item := range result.Items {
		if item.OutputPath == "" || !strings.HasSuffix(item.OutputPath, filepath.Ext(item.Path)) {
			t.Errorf("产物扩展名应随类型：%+v", item)
		}
	}
}
