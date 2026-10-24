package imageproc

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"file-processor/internal/apperr"
)

// buildTestPDF 造一个内嵌高质量噪声 JPEG 的 PDF——重压缩的典型受益形态
// （噪声在 q100 与 q75 之间体积差异巨大，断言确定性最好）。
func buildTestPDF(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 480, 480))
	rng := rand.New(rand.NewSource(42))
	for i := range img.Pix {
		img.Pix[i] = uint8(rng.Intn(256))
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "noise.jpg")
	if err := os.WriteFile(src, jpg.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	pdfPath := filepath.Join(dir, "noise.pdf")
	if err := api.ImportImagesFile(context.Background(), []string{src}, pdfPath, nil, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestIsPDF(t *testing.T) {
	if !IsPDF(buildTestPDF(t)) {
		t.Error("真 PDF 应识别为 PDF")
	}
	cases := [][]byte{
		nil,
		[]byte("plain text, definitely not a pdf"),
		[]byte{0xFF, 0xD8, 0xFF, 0xE0}, // JPEG 头
	}
	for _, data := range cases {
		if IsPDF(data) {
			t.Errorf("非 PDF 不应识别: %q", data)
		}
	}
}

func TestInspectPDF(t *testing.T) {
	src := buildTestPDF(t)
	info := InspectPDF(src, "/tmp/a.pdf")
	if info.Format != "pdf" || info.Bytes != int64(len(src)) || info.Thumbnail != "" {
		t.Errorf("PDF 检查结果不符: %+v", info)
	}
}

func TestCompressPDFShrinksNoisyJPEG(t *testing.T) {
	src := buildTestPDF(t)
	out, stats, err := CompressPDF(context.Background(), src)
	if err != nil {
		t.Fatalf("压缩失败: %v", err)
	}
	if int64(len(out)) >= int64(len(src)) {
		t.Errorf("噪声 JPEG 的 PDF 应显著变小: %d → %d", len(src), len(out))
	}
	if stats.ImagesTotal != 1 || stats.ImagesReplaced != 1 ||
		stats.ImagesUnsupported != 0 || stats.ImagesNotSmaller != 0 {
		t.Errorf("图片统计不符: %+v", stats)
	}
}

// buildFlateTestPDF 造一个内嵌 FlateDecode 图像（PNG 导入）的 PDF：
// 不在重压缩处理面，必须原样保持并计入「不支持的图片类型」。
func buildFlateTestPDF(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 120, 90))
	for y := range 90 {
		for x := range 120 {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * 2), G: uint8(y * 2), B: 128, A: 255})
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "flat.png")
	if err := os.WriteFile(src, pngBuf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	pdfPath := filepath.Join(dir, "flat.pdf")
	if err := api.ImportImagesFile(context.Background(), []string{src}, pdfPath, nil, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCompressPDFCountsUnsupportedFlateImage(t *testing.T) {
	out, stats, err := CompressPDF(context.Background(), buildFlateTestPDF(t))
	if err != nil {
		t.Fatalf("压缩失败: %v", err)
	}
	// 契约是「图片不被替换、计入不支持」；整文件体积随无损优化微增微减都正常，不作断言。
	if stats.ImagesTotal != 1 || stats.ImagesReplaced != 0 ||
		stats.ImagesUnsupported != 1 || stats.ImagesNotSmaller != 0 {
		t.Errorf("Flate 图应计入「不支持的图片类型」: %+v", stats)
	}
	if len(out) == 0 {
		t.Error("应产出压缩后的 PDF 字节")
	}
}

func TestCompressPDFEncrypted(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.pdf")
	enc := filepath.Join(dir, "enc.pdf")
	if err := os.WriteFile(src, buildTestPDF(t), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := model.NewDefaultConfiguration()
	conf.OwnerPW = "test123"
	if err := api.EncryptFile(context.Background(), src, enc, conf); err != nil {
		t.Fatalf("造加密夹具失败: %v", err)
	}
	data, err := os.ReadFile(enc)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = CompressPDF(context.Background(), data)
	ae := apperr.Wrap(err)
	if ae.Code != apperr.CodePDFEncrypted {
		t.Errorf("加密 PDF 应返回 pdf_encrypted，得到 %s（%v）", ae.Code, err)
	}
}

func TestCompressPDFDamaged(t *testing.T) {
	_, _, err := CompressPDF(context.Background(), []byte("%PDF-1.4\nthis is not a real pdf\n"))
	ae := apperr.Wrap(err)
	if ae.Code != apperr.CodePDFDamaged {
		t.Errorf("损坏 PDF 应返回 pdf_damaged，得到 %s（%v）", ae.Code, err)
	}
}
