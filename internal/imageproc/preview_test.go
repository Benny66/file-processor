package imageproc

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"
)

func TestRenderPreviewOpaque(t *testing.T) {
	src := testOpaqueImage(400, 300)
	got, err := RenderPreview(encodeForTest(t, "jpeg", src), "image.jpg", 100)
	if err != nil {
		t.Fatalf("RenderPreview 失败: %v", err)
	}
	if got.Width != 100 || got.Height != 75 {
		t.Errorf("预览尺寸 = %dx%d，期望 100x75", got.Width, got.Height)
	}
	if got.HasAlpha {
		t.Errorf("不透明图片不应标记 alpha")
	}
	if !strings.HasPrefix(got.DataURL, "data:image/jpeg;base64,") {
		t.Errorf("不透明预览应为 JPEG：%q", got.DataURL[:40])
	}
	img := decodeDataURL(t, got.DataURL, "jpeg")
	if img.Bounds().Dx() != 100 || img.Bounds().Dy() != 75 {
		t.Errorf("解码预览尺寸 = %v", img.Bounds())
	}
}

func TestRenderPreviewTransparent(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
		}
	}
	got, err := RenderPreview(encodeForTest(t, "png", src), "image.png", 100)
	if err != nil {
		t.Fatalf("RenderPreview 失败: %v", err)
	}
	if got.Width != 40 || got.Height != 20 {
		t.Errorf("小图不应放大：%dx%d", got.Width, got.Height)
	}
	if !got.HasAlpha {
		t.Errorf("透明图片应标记 alpha")
	}
	if !strings.HasPrefix(got.DataURL, "data:image/png;base64,") {
		t.Errorf("透明预览应为 PNG")
	}
	img := decodeDataURL(t, got.DataURL, "png")
	_, _, _, a := img.At(0, 0).RGBA()
	if a == 0xffff {
		t.Errorf("预览丢失透明通道")
	}
}

func TestRenderPreviewAppliesOrientation(t *testing.T) {
	base := encodeForTest(t, "jpeg", testOpaqueImage(4, 3))
	rotated := jpegWithOrientation(base, 6)
	got, err := RenderPreview(rotated, "image.jpg", 100)
	if err != nil {
		t.Fatalf("RenderPreview 失败: %v", err)
	}
	if got.Width != 3 || got.Height != 4 {
		t.Errorf("预览方向未归一：%dx%d", got.Width, got.Height)
	}
}

func TestRenderPreviewRejectsAnimated(t *testing.T) {
	if _, err := RenderPreview(encodeAnimatedGIF(t), "image.gif", 100); err == nil {
		t.Fatalf("动图应失败")
	}
}

func TestRenderPreviewClampsMaxEdge(t *testing.T) {
	src := encodeForTest(t, "jpeg", testOpaqueImage(3000, 1500))
	cases := []struct {
		maxEdge int
		wantW   int
		wantH   int
	}{
		{0, 1024, 512},
		{1, 64, 32},
		{5000, 2048, 1024},
	}
	for _, tc := range cases {
		got, err := RenderPreview(src, "image.jpg", tc.maxEdge)
		if err != nil {
			t.Fatalf("maxEdge=%d 预览失败: %v", tc.maxEdge, err)
		}
		if got.Width != tc.wantW || got.Height != tc.wantH {
			t.Errorf("maxEdge=%d 尺寸 = %dx%d，期望 %dx%d", tc.maxEdge, got.Width, got.Height, tc.wantW, tc.wantH)
		}
	}
}

func decodeDataURL(t *testing.T, dataURL, format string) image.Image {
	t.Helper()
	prefix := "data:image/" + format + ";base64,"
	if !strings.HasPrefix(dataURL, prefix) {
		t.Fatalf("不是 %s data URL", format)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, prefix))
	if err != nil {
		t.Fatalf("base64 解码失败: %v", err)
	}
	var img image.Image
	switch format {
	case "jpeg":
		img, err = jpeg.Decode(bytes.NewReader(raw))
	case "png":
		img, err = png.Decode(bytes.NewReader(raw))
	default:
		t.Fatalf("未知预览格式: %s", format)
	}
	if err != nil {
		t.Fatalf("解码预览失败: %v", err)
	}
	return img
}
