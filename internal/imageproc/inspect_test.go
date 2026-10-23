package imageproc

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
	"testing"

	genwebp "github.com/gen2brain/webp"
	xbmp "golang.org/x/image/bmp"
	xtiff "golang.org/x/image/tiff"

	"file-processor/internal/apperr"
)

func TestInspectFormats(t *testing.T) {
	src := testOpaqueImage(200, 100)
	cases := []struct {
		ext    string
		format string
		data   []byte
	}{
		{".jpg", "jpeg", encodeForTest(t, "jpeg", src)},
		{".png", "png", encodeForTest(t, "png", src)},
		{".gif", "gif", encodeForTest(t, "gif", src)},
		{".bmp", "bmp", encodeForTest(t, "bmp", src)},
		{".tiff", "tiff", encodeForTest(t, "tiff", src)},
		{".webp", "webp", encodeForTest(t, "webp", src)},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			path := "image" + tc.ext
			info, err := Inspect(tc.data, path)
			if err != nil {
				t.Fatalf("Inspect(%s) 失败: %v", tc.format, err)
			}
			if info.Path != path {
				t.Errorf("Path = %q，期望 %q", info.Path, path)
			}
			if info.Format != tc.format {
				t.Errorf("Format = %q，期望 %q", info.Format, tc.format)
			}
			if info.Width != 200 || info.Height != 100 {
				t.Errorf("尺寸 = %dx%d，期望 200x100", info.Width, info.Height)
			}
			if info.Bytes != int64(len(tc.data)) {
				t.Errorf("Bytes = %d，期望 %d", info.Bytes, len(tc.data))
			}
			if info.Warning != "" {
				t.Errorf("不应有 warning：%q", info.Warning)
			}
			thumb := decodeThumbnail(t, info.Thumbnail)
			if thumb.Bounds().Dx() != 96 || thumb.Bounds().Dy() != 48 {
				t.Errorf("缩略图尺寸 = %v，期望 96x48", thumb.Bounds())
			}
		})
	}
}

func TestInspectAppliesOrientation(t *testing.T) {
	base := encodeForTest(t, "jpeg", testOpaqueImage(4, 3))
	jpg := jpegWithOrientation(base, 6)

	info, err := Inspect(jpg, "image.jpg")
	if err != nil {
		t.Fatalf("Inspect 失败: %v", err)
	}
	if info.Width != 3 || info.Height != 4 {
		t.Fatalf("方向归一尺寸 = %dx%d，期望 3x4", info.Width, info.Height)
	}
	thumb := decodeThumbnail(t, info.Thumbnail)
	if thumb.Bounds().Dx() != 3 || thumb.Bounds().Dy() != 4 {
		t.Errorf("缩略图方向未归一：%v", thumb.Bounds())
	}
}

func TestInspectPreservesTransparencyInThumbnail(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
		}
	}
	info, err := Inspect(encodeForTest(t, "png", src), "image.png")
	if err != nil {
		t.Fatalf("Inspect 失败: %v", err)
	}
	thumb := decodeThumbnail(t, info.Thumbnail)
	_, _, _, a := thumb.At(0, 0).RGBA()
	if a == 0xffff {
		t.Fatalf("缩略图丢失了透明通道")
	}
}

func TestInspectWarnsExtensionMismatch(t *testing.T) {
	pngBytes := encodeForTest(t, "png", testOpaqueImage(20, 10))
	info, err := Inspect(pngBytes, "image.jpg")
	if err != nil {
		t.Fatalf("Inspect 失败: %v", err)
	}
	if info.Format != "png" {
		t.Fatalf("Format = %q，期望 png", info.Format)
	}
	if !strings.Contains(info.Warning, "不一致") {
		t.Errorf("扩展名不一致没有 warning：%q", info.Warning)
	}
}

func TestInspectRejectsAnimated(t *testing.T) {
	assertInspectError(t, encodeAnimatedGIF(t), "image.gif", "动图")
	assertInspectError(t, encodeAnimatedWebP(t), "image.webp", "动图")
}

func TestInspectRejectsCorrupt(t *testing.T) {
	assertInspectError(t, []byte{0xFF, 0xD8, 0xFF, 0x00, 0x00}, "image.jpg", "无法解码")
	assertInspectError(t, []byte("hello"), "image.jpg", "无法识别")
}

func TestInspectRejectsPixelLimit(t *testing.T) {
	assertInspectError(t, pngHeaderOnly(10001, 10000), "big.png", "100 MP")
}

func TestInspectChineseAndSpaces(t *testing.T) {
	path := filepath.Join("图 片", "照 片.png")
	info, err := Inspect(encodeForTest(t, "png", testOpaqueImage(20, 10)), path)
	if err != nil {
		t.Fatalf("Inspect 失败: %v", err)
	}
	if info.Path != path {
		t.Errorf("Path = %q，期望 %q", info.Path, path)
	}
}

func testOpaqueImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 80, A: 255})
		}
	}
	return img
}

func encodeForTest(t *testing.T, format string, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	var err error
	switch format {
	case "jpeg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
	case "png":
		err = png.Encode(&buf, img)
	case "gif":
		err = gif.Encode(&buf, img, &gif.Options{NumColors: 256})
	case "bmp":
		err = xbmp.Encode(&buf, img)
	case "tiff":
		err = xtiff.Encode(&buf, img, nil)
	case "webp":
		err = genwebp.Encode(&buf, img, genwebp.Options{Quality: 80})
	default:
		t.Fatalf("未知格式: %s", format)
	}
	if err != nil {
		t.Fatalf("%s encode: %v", format, err)
	}
	return buf.Bytes()
}

func decodeThumbnail(t *testing.T, dataURL string) image.Image {
	t.Helper()
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) {
		t.Fatalf("缩略图不是 PNG data URL：%q", dataURL)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, prefix))
	if err != nil {
		t.Fatalf("解码缩略图 base64 失败: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("解码缩略图 PNG 失败: %v", err)
	}
	return img
}

func assertInspectError(t *testing.T, data []byte, path, want string) {
	t.Helper()
	_, err := Inspect(data, path)
	if err == nil {
		t.Fatalf("Inspect(%q) 应失败", path)
	}
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("错误不是 apperr：%T %v", err, err)
	}
	if ae.Code != apperr.CodeValidation {
		t.Errorf("错误码 = %q，期望 validation", ae.Code)
	}
	if !strings.Contains(ae.Error(), want) {
		t.Errorf("错误 %q 不包含 %q", ae.Error(), want)
	}
}

func pngHeaderOnly(width, height int) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], uint32(width))
	binary.BigEndian.PutUint32(ihdr[4:8], uint32(height))
	ihdr[8] = 8
	ihdr[9] = 2
	writePNGChunk(&buf, "IHDR", ihdr)
	writePNGChunk(&buf, "IEND", nil)
	return buf.Bytes()
}

func writePNGChunk(buf *bytes.Buffer, typ string, data []byte) {
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	buf.Write(lenBuf[:])
	buf.WriteString(typ)
	buf.Write(data)
	crc := crc32.NewIEEE()
	crc.Write([]byte(typ))
	crc.Write(data)
	var crcBuf [4]byte
	binary.BigEndian.PutUint32(crcBuf[:], crc.Sum32())
	buf.Write(crcBuf[:])
}
