package imageproc

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	genwebp "github.com/gen2brain/webp"

	"file-processor/internal/apperr"
)

func TestFitWithin(t *testing.T) {
	cases := []struct {
		width, height, maxWidth, maxHeight int
		wantW, wantH                       int
	}{
		{4000, 3000, 1920, 1080, 1440, 1080},
		{800, 600, 1920, 1080, 800, 600},
		{300, 400, 100, 100, 75, 100},
		{100, 50, 100, 100, 100, 50},
		{50, 100, 100, 100, 50, 100},
		{1000, 10, 100, 0, 100, 1},
		{100, 100, 0, 0, 100, 100},
	}
	for _, tc := range cases {
		gotW, gotH := FitWithin(tc.width, tc.height, tc.maxWidth, tc.maxHeight)
		if gotW != tc.wantW || gotH != tc.wantH {
			t.Errorf("FitWithin(%d,%d,%d,%d) = %dx%d，期望 %dx%d",
				tc.width, tc.height, tc.maxWidth, tc.maxHeight, gotW, gotH, tc.wantW, tc.wantH)
		}
	}
}

func TestConvertFitsWithinBox(t *testing.T) {
	src := testOpaqueImage(400, 300)
	got, err := Convert(
		encodeForTest(t, "png", src),
		"image.png",
		ConvertOptions{Format: OutputPNG, MaxWidth: 200, MaxHeight: 200},
	)
	if err != nil {
		t.Fatalf("Convert 失败: %v", err)
	}
	if got.Width != 200 || got.Height != 150 {
		t.Fatalf("盒内尺寸 = %dx%d，期望 200x150", got.Width, got.Height)
	}
	out, err := png.Decode(bytes.NewReader(got.Data))
	if err != nil {
		t.Fatalf("解码输出失败: %v", err)
	}
	if out.Bounds().Dx() != 200 || out.Bounds().Dy() != 150 {
		t.Errorf("输出像素尺寸 = %v", out.Bounds())
	}

	small := testOpaqueImage(100, 50)
	got, err = Convert(
		encodeForTest(t, "png", small),
		"image.png",
		ConvertOptions{Format: OutputPNG, MaxWidth: 200, MaxHeight: 200},
	)
	if err != nil {
		t.Fatalf("Convert 小图失败: %v", err)
	}
	if got.Width != 100 || got.Height != 50 {
		t.Errorf("小图不应放大：%+v", got)
	}
}

func TestNormalizeOutputFormat(t *testing.T) {
	cases := []struct {
		format, source, want string
		wantErr              bool
	}{
		{OutputKeep, "jpeg", OutputJPEG, false},
		{OutputKeep, "png", OutputPNG, false},
		{OutputKeep, "webp", OutputWebP, false},
		{OutputKeep, "gif", OutputPNG, false},
		{OutputKeep, "bmp", OutputPNG, false},
		{OutputKeep, "tiff", OutputPNG, false},
		{OutputJPEG, "png", OutputJPEG, false},
		{OutputPNG, "jpeg", OutputPNG, false},
		{OutputWebP, "jpeg", OutputWebP, false},
		{"gif", "jpeg", "", true},
	}
	for _, tc := range cases {
		got, err := NormalizeOutputFormat(tc.format, tc.source)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("NormalizeOutputFormat(%q,%q) = %q,%v，期望 %q,err=%v", tc.format, tc.source, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestConvertFormats(t *testing.T) {
	src := testOpaqueImage(20, 10)
	pngBytes := encodeForTest(t, "png", src)
	for _, outFormat := range []string{OutputJPEG, OutputPNG, OutputWebP} {
		got, err := Convert(pngBytes, "image.png", ConvertOptions{Format: outFormat})
		if err != nil {
			t.Fatalf("Convert(%s) 失败: %v", outFormat, err)
		}
		if got.Format != outFormat || got.Width != 20 || got.Height != 10 {
			t.Errorf("Convert(%s) = %+v", outFormat, got)
		}
		if !hasFormatSignature(got.Format, got.Data) {
			t.Errorf("输出 %s 的签名不对", outFormat)
		}
	}
}

func TestConvertKeepFallbackToPNG(t *testing.T) {
	src := testOpaqueImage(20, 10)
	for _, srcFormat := range []string{"bmp", "tiff", "gif"} {
		data := encodeForTest(t, srcFormat, src)
		got, err := Convert(data, "image."+srcFormat, ConvertOptions{Format: OutputKeep})
		if err != nil {
			t.Fatalf("Convert keep(%s) 失败: %v", srcFormat, err)
		}
		if got.Format != OutputPNG {
			t.Errorf("Convert keep(%s) Format = %q，期望 png", srcFormat, got.Format)
		}
		if !hasFormatSignature(OutputPNG, got.Data) {
			t.Errorf("Convert keep(%s) 输出不是 PNG", srcFormat)
		}
	}
}

func TestConvertAppliesOrientation(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	red := color.NRGBA{R: 255, A: 255}
	green := color.NRGBA{G: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	src.SetNRGBA(0, 0, red)
	src.SetNRGBA(2, 0, green)
	src.SetNRGBA(0, 1, blue)
	src.SetNRGBA(2, 1, white)

	base := encodeForTest(t, "png", src)
	rotated := pngWithOrientation(base, 6)
	got, err := Convert(rotated, "image.png", ConvertOptions{Format: OutputPNG})
	if err != nil {
		t.Fatalf("Convert 失败: %v", err)
	}
	if got.Width != 2 || got.Height != 3 {
		t.Fatalf("转换后尺寸 = %dx%d，期望 2x3", got.Width, got.Height)
	}
	out, err := png.Decode(bytes.NewReader(got.Data))
	if err != nil {
		t.Fatalf("解码输出 PNG 失败: %v", err)
	}
	if c := color.NRGBAModel.Convert(out.At(0, 0)).(color.NRGBA); c.B < 200 {
		t.Errorf("orientation 6 左上角应为蓝色，实际 %v", c)
	}
	if c := color.NRGBAModel.Convert(out.At(1, 0)).(color.NRGBA); c.R < 200 {
		t.Errorf("orientation 6 右上角应为红色，实际 %v", c)
	}
}

func TestConvertFlattensAlphaToWhiteForJPEG(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if x >= 4 {
				src.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
			}
		}
	}
	got, err := Convert(encodeForTest(t, "png", src), "image.png", ConvertOptions{Format: OutputJPEG})
	if err != nil {
		t.Fatalf("Convert 失败: %v", err)
	}
	out, err := jpeg.Decode(bytes.NewReader(got.Data))
	if err != nil {
		t.Fatalf("解码输出 JPEG 失败: %v", err)
	}
	whitePixel := color.NRGBAModel.Convert(out.At(1, 4)).(color.NRGBA)
	if whitePixel.R < 220 || whitePixel.G < 220 || whitePixel.B < 220 {
		t.Errorf("透明区域没有铺白：%v", whitePixel)
	}
	redPixel := color.NRGBAModel.Convert(out.At(6, 4)).(color.NRGBA)
	if redPixel.R < 200 || redPixel.G > 60 || redPixel.B > 60 {
		t.Errorf("不透明红色区域失真：%v", redPixel)
	}
	// US-20：铺白这个事实必须【回传】，否则前端给不出「透明背景已铺白」的提示。
	if !got.AlphaFlattened {
		t.Errorf("透明图转 JPEG 未标记 AlphaFlattened —— 队列提示将没有依据")
	}
}

// TestConvertAlphaFlattenedOnlyWhenActuallyTransparent 防止把提示做成无条件噪声：
// 不透明源转 JPEG 不该标记铺白。
func TestConvertAlphaFlattenedOnlyWhenActuallyTransparent(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	got, err := Convert(encodeForTest(t, "png", src), "solid.png", ConvertOptions{Format: OutputJPEG})
	if err != nil {
		t.Fatalf("Convert 失败: %v", err)
	}
	if got.AlphaFlattened {
		t.Errorf("不透明源被标记为已铺白 —— 会在队列里制造无意义的提示")
	}
	// 输出保留透明时同样不该标记（铺白只发生在 JPEG）。
	pngOut, err := Convert(encodeForTest(t, "png", src), "solid.png", ConvertOptions{Format: OutputPNG})
	if err != nil {
		t.Fatalf("PNG 转换失败: %v", err)
	}
	if pngOut.AlphaFlattened {
		t.Errorf("PNG 输出被标记为已铺白")
	}
}

func TestConvertPreservesAlpha(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
		}
	}
	data := encodeForTest(t, "png", src)
	for _, outFormat := range []string{OutputPNG, OutputWebP} {
		got, err := Convert(data, "image.png", ConvertOptions{Format: outFormat})
		if err != nil {
			t.Fatalf("Convert(%s) 失败: %v", outFormat, err)
		}
		var out image.Image
		switch outFormat {
		case OutputPNG:
			out, err = png.Decode(bytes.NewReader(got.Data))
		case OutputWebP:
			out, err = genwebp.Decode(bytes.NewReader(got.Data))
		}
		if err != nil {
			t.Fatalf("解码输出 %s 失败: %v", outFormat, err)
		}
		_, _, _, a := out.At(0, 0).RGBA()
		if a == 0xffff {
			t.Errorf("%s 输出丢失透明通道", outFormat)
		}
	}
}

func TestConvertStripsMetadata(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	withExif := jpegWithOrientation(encodeForTest(t, "jpeg", src), 6)

	jpegOut, err := Convert(withExif, "image.jpg", ConvertOptions{Format: OutputJPEG})
	if err != nil {
		t.Fatalf("Convert JPEG 失败: %v", err)
	}
	if bytes.Contains(jpegOut.Data, []byte("Exif\x00\x00")) || bytes.Contains(jpegOut.Data, []byte{0xFF, 0xE1}) {
		t.Errorf("JPEG 输出仍含 EXIF/APP1")
	}
	img, err := jpeg.Decode(bytes.NewReader(jpegOut.Data))
	if err != nil {
		t.Fatalf("解码 JPEG 输出失败: %v", err)
	}
	if img.Bounds().Dx() != 3 || img.Bounds().Dy() != 4 {
		t.Errorf("JPEG 输出未应用方向：%v", img.Bounds())
	}

	pngOut, err := Convert(withExif, "image.jpg", ConvertOptions{Format: OutputPNG})
	if err != nil {
		t.Fatalf("Convert PNG 失败: %v", err)
	}
	if bytes.Contains(pngOut.Data, []byte("eXIf")) || bytes.Contains(pngOut.Data, []byte("iCCP")) {
		t.Errorf("PNG 输出仍含 EXIF/ICC")
	}

	webpOut, err := Convert(withExif, "image.jpg", ConvertOptions{Format: OutputWebP})
	if err != nil {
		t.Fatalf("Convert WebP 失败: %v", err)
	}
	if bytes.Contains(webpOut.Data, []byte("EXIF")) || bytes.Contains(webpOut.Data, []byte("ICCP")) {
		t.Errorf("WebP 输出仍含 EXIF/ICC")
	}
}

func TestConvertRejectsAnimated(t *testing.T) {
	_, err := Convert(encodeAnimatedGIF(t), "image.gif", ConvertOptions{Format: OutputPNG})
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != apperr.CodeValidation || !strings.Contains(ae.Error(), "动图") {
		t.Fatalf("期望动图 validation 错误，实际：%v", err)
	}
}

func TestConvertTargetSizeJPEG(t *testing.T) {
	src := noiseImage(400, 300)
	pngBytes := encodeForTest(t, "png", src)

	got, err := Convert(pngBytes, "image.png", ConvertOptions{Format: OutputJPEG, TargetBytes: 50 * 1024})
	if err != nil {
		t.Fatalf("Convert target 失败: %v", err)
	}
	if len(got.Data) > 50*1024 {
		t.Fatalf("目标体积未达成：%d > %d", len(got.Data), 50*1024)
	}
	if got.Quality <= 20 || got.ExtremeDegraded {
		t.Errorf("50KB 目标不应极端降级：quality=%d extreme=%v", got.Quality, got.ExtremeDegraded)
	}
	if got.Width != 400 || got.Height != 300 {
		t.Errorf("50KB 目标不应缩尺寸：%dx%d", got.Width, got.Height)
	}

	small, err := Convert(pngBytes, "image.png", ConvertOptions{Format: OutputJPEG, TargetBytes: 5 * 1024})
	if err != nil {
		t.Fatalf("Convert 5KB 失败: %v", err)
	}
	if len(small.Data) > 5*1024 {
		t.Errorf("5KB 目标未达成：%d", len(small.Data))
	}
	if small.Width >= 400 && small.Height >= 300 {
		t.Errorf("5KB 目标应缩尺寸：%dx%d", small.Width, small.Height)
	}

	large := encodeForTest(t, "png", noiseImage(800, 600))
	extreme, err := Convert(large, "image.png", ConvertOptions{Format: OutputJPEG, TargetBytes: 800})
	if err != nil {
		t.Fatalf("Convert 800B 失败（不应因降质失败）: %v", err)
	}
	if len(extreme.Data) > 1024 {
		t.Errorf("800B 目标未达成：%d", len(extreme.Data))
	}
	if !extreme.ExtremeDegraded {
		t.Errorf("800B 目标应标记极端降级：quality=%d dims=%dx%d", extreme.Quality, extreme.Width, extreme.Height)
	}

	if _, err := Convert(pngBytes, "image.png", ConvertOptions{Format: OutputJPEG, TargetBytes: 50}); err == nil {
		t.Errorf("不可达目标应失败")
	}
}

func TestConvertTargetQualityPriority(t *testing.T) {
	src := noiseImage(400, 300)
	pngBytes := encodeForTest(t, "png", src)
	q85, err := Convert(pngBytes, "image.png", ConvertOptions{Format: OutputJPEG, Quality: 85})
	if err != nil {
		t.Fatalf("q85 基线失败: %v", err)
	}
	got, err := Convert(pngBytes, "image.png", ConvertOptions{Format: OutputJPEG, TargetBytes: int64(len(q85.Data))})
	if err != nil {
		t.Fatalf("目标 = q85 体积时失败: %v", err)
	}
	if len(got.Data) > len(q85.Data) {
		t.Errorf("输出体积超过目标")
	}
	if got.Quality < 85 {
		t.Errorf("应优先保留最高质量，实际 quality=%d", got.Quality)
	}
}

func TestConvertTargetWithBox(t *testing.T) {
	src := noiseImage(800, 600)
	pngBytes := encodeForTest(t, "png", src)
	got, err := Convert(pngBytes, "image.png", ConvertOptions{
		Format:      OutputJPEG,
		MaxWidth:    300,
		MaxHeight:   300,
		TargetBytes: 20 * 1024,
	})
	if err != nil {
		t.Fatalf("Convert 失败: %v", err)
	}
	if got.Width > 300 || got.Height > 300 {
		t.Errorf("输出超出宽高盒：%dx%d", got.Width, got.Height)
	}
	if len(got.Data) > 20*1024 {
		t.Errorf("目标体积未达成：%d", len(got.Data))
	}
}

func TestConvertTargetPNG(t *testing.T) {
	src := noiseImage(400, 300)
	got, err := Convert(encodeForTest(t, "png", src), "image.png", ConvertOptions{
		Format:      OutputPNG,
		TargetBytes: 50 * 1024,
	})
	if err != nil {
		t.Fatalf("Convert PNG target 失败: %v", err)
	}
	if got.Format != OutputPNG || len(got.Data) > 50*1024 {
		t.Errorf("PNG 目标结果不对：format=%s bytes=%d", got.Format, len(got.Data))
	}
	if _, err := png.Decode(bytes.NewReader(got.Data)); err != nil {
		t.Errorf("输出不是可解码 PNG: %v", err)
	}
}

func TestConvertTargetWebP(t *testing.T) {
	src := noiseImage(400, 300)
	got, err := Convert(encodeForTest(t, "png", src), "image.png", ConvertOptions{
		Format:      OutputWebP,
		TargetBytes: 30 * 1024,
	})
	if err != nil {
		t.Fatalf("Convert WebP target 失败: %v", err)
	}
	if len(got.Data) > 30*1024 || got.Format != OutputWebP {
		t.Errorf("WebP 目标结果不对：format=%s bytes=%d", got.Format, len(got.Data))
	}
}

func noiseImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint32(x*73856093) ^ uint32(y*19349663)
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(v),
				G: uint8(v >> 8),
				B: uint8(v >> 16),
				A: 255,
			})
		}
	}
	return img
}

func hasFormatSignature(format string, data []byte) bool {
	switch format {
	case OutputJPEG:
		return len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8
	case OutputPNG:
		return len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A})
	case OutputWebP:
		return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	default:
		return false
	}
}
