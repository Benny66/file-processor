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
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

	"file-processor/internal/apperr"
)

// buildTestPDF 造一个内嵌高质量噪声 JPEG 的 PDF——重压缩的典型受益形态
// （噪声在 q100 与 q75 之间体积差异巨大，断言确定性最好）。
func buildTestPDF(t *testing.T) []byte {
	t.Helper()
	return buildBigTestPDF(t, 480)
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
	// 导入页是 72 DPI（图 480px = 页 480pt），低于 150 下限 → 不触发降采样。
	if stats.ImagesTotal != 1 || stats.ImagesReplaced != 1 || stats.ImagesDownsampled != 0 ||
		stats.ImagesUnsupported != 0 || stats.ImagesNotSmaller != 0 {
		t.Errorf("图片统计不符: %+v", stats)
	}
}

// buildScanTestPDFWithBox 造「满页单图」扫描页：导入 1200px 噪声 JPEG（页 1200pt），
// 再把 MediaBox 缩到 boxW×boxH pt——boxW=boxH=288（4 英寸方页）即 300 DPI 扫描件。
func buildScanTestPDFWithBox(t *testing.T, boxW, boxH int) []byte {
	t.Helper()
	base := buildBigTestPDF(t, 1200)
	ctx, err := api.ReadContext(context.Background(), bytes.NewReader(base), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ctx.XRefTable.EnsurePageCount(); err != nil {
		t.Fatal(err)
	}
	pageDict, _, _, err := ctx.XRefTable.PageDict(context.Background(), 1, false)
	if err != nil {
		t.Fatal(err)
	}
	pageDict["MediaBox"] = types.Array{
		types.Integer(0), types.Integer(0), types.Integer(boxW), types.Integer(boxH),
	}
	var buf bytes.Buffer
	if err := api.WriteContext(context.Background(), ctx, &buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// buildScanTestPDF 是标准 300 DPI 扫描夹具（4×4 英寸方页）。
func buildScanTestPDF(t *testing.T) []byte {
	t.Helper()
	return buildScanTestPDFWithBox(t, 288, 288)
}

// buildBigTestPDF 是 buildTestPDF 的可调尺寸版（噪声 JPEG，质量 100）。
func buildBigTestPDF(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
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
	return path2Bytes(t, pdfPath)
}

func path2Bytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCompressPDFDownsamplesFullPageScan(t *testing.T) {
	src := buildScanTestPDF(t)
	out, stats, err := CompressPDF(context.Background(), src)
	if err != nil {
		t.Fatalf("压缩失败: %v", err)
	}
	if stats.ImagesTotal != 1 || stats.ImagesReplaced != 1 || stats.ImagesDownsampled != 1 ||
		stats.ImagesUnsupported != 0 || stats.ImagesNotSmaller != 0 {
		t.Fatalf("扫描页统计不符: %+v", stats)
	}
	// 读回输出验证像素确实压到了 600（288pt=4in × 150dpi）。
	doc, err := api.ReadContext(context.Background(), bytes.NewReader(out), nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range doc.XRefTable.Table {
		sd, ok := entry.Object.(types.StreamDict)
		if !ok || !sd.Image() {
			continue
		}
		w, h := pdfImageDims(sd)
		if w != 600 || h != 600 {
			t.Errorf("扫描图应压到 600x600，得到 %dx%d", w, h)
		}
		found = true
	}
	if !found {
		t.Error("输出里应仍有图像对象")
	}
	if int64(len(out)) >= int64(len(src)) {
		t.Errorf("300 DPI 扫描页应显著变小: %d → %d", len(src), len(out))
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
	return path2Bytes(t, pdfPath)
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

func TestCompressPDFAspectMismatchNoDownsample(t *testing.T) {
	// 页面 288×144（2:1）装着 1:1 的方图：图没填满页面，不算扫描页——
	// 不缩像素，只按质量下限整幅重存。
	out, stats, err := CompressPDF(context.Background(), buildScanTestPDFWithBox(t, 288, 144))
	if err != nil {
		t.Fatalf("压缩失败: %v", err)
	}
	if stats.ImagesDownsampled != 0 || stats.ImagesReplaced != 1 {
		t.Errorf("长宽比不符不应触发降采样: %+v", stats)
	}
	if len(out) == 0 {
		t.Error("应产出压缩后的 PDF 字节")
	}
}

func TestRecompressPDFImageTargetEqualsSizeFallsBackToResave(t *testing.T) {
	// 窄带回归（审阅发现）：DPI 略超 150 时目标像素 round 后恰等于原尺寸，
	// 必须回落为普通重存，而不是误计「不支持的图片类型」整图跳过。
	doc, err := api.ReadContext(context.Background(), bytes.NewReader(buildTestPDF(t)), nil)
	if err != nil {
		t.Fatal(err)
	}
	var sd types.StreamDict
	for _, entry := range doc.XRefTable.Table {
		if cand, ok := entry.Object.(types.StreamDict); ok && cand.Image() {
			sd = cand
			break
		}
	}
	if sd.Dict == nil {
		t.Fatal("夹具里没找到图像对象")
	}
	recompressed, result := recompressPDFImage(sd, &image.Point{X: 480, Y: 480})
	if result != pdfImageReplaced {
		t.Fatalf("目标等于原尺寸应回落普通重存，得到 %v", result)
	}
	if len(recompressed.Raw) == 0 || len(recompressed.Raw) >= len(sd.Raw) {
		t.Errorf("回落重存应产出更小的 JPEG: %d → %d", len(sd.Raw), len(recompressed.Raw))
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
