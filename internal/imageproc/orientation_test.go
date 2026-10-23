package imageproc

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	genwebp "github.com/gen2brain/webp"
)

func TestParseOrientationJPEG(t *testing.T) {
	base := encodeJPEGForTest(t)
	for o := 1; o <= 8; o++ {
		jpg := jpegWithOrientation(base, o)
		got, err := parseOrientation(jpg)
		if err != nil || got != o {
			t.Fatalf("jpeg orientation %d: got %d err=%v", o, got, err)
		}
	}
}

func TestParseOrientationBigEndian(t *testing.T) {
	jpg := jpegWithOrientationBE(encodeJPEGForTest(t), 6)
	if got, err := parseOrientation(jpg); err != nil || got != 6 {
		t.Fatalf("big-endian jpeg orientation: got %d err=%v", got, err)
	}
}

func TestParseOrientationContainers(t *testing.T) {
	pngBytes := pngWithOrientation(encodePNGForTest(t), 6)
	if got, err := parseOrientation(pngBytes); err != nil || got != 6 {
		t.Fatalf("png orientation: got %d err=%v", got, err)
	}

	webpBytes := webpWithOrientation(encodeWebPForTest(t), 6)
	if got, err := parseOrientation(webpBytes); err != nil || got != 6 {
		t.Fatalf("webp orientation: got %d err=%v", got, err)
	}

	if got, err := parseOrientation(tiffWithOrientation(6, false)); err != nil || got != 6 {
		t.Fatalf("tiff orientation: got %d err=%v", got, err)
	}
}

func TestApplyOrientation(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	red := color.NRGBA{R: 255, A: 255}
	green := color.NRGBA{G: 255, A: 255}
	blue := color.NRGBA{B: 255, A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	src.SetNRGBA(0, 0, red)
	src.SetNRGBA(2, 0, green)
	src.SetNRGBA(0, 1, blue)
	src.SetNRGBA(2, 1, white)

	cases := []struct {
		o        int
		w, h     int
		topLeft  color.NRGBA
		topRight color.NRGBA
	}{
		{1, 3, 2, red, green},
		{2, 3, 2, green, red},
		{3, 3, 2, white, blue},
		{4, 3, 2, blue, white},
		{5, 2, 3, red, blue},
		{6, 2, 3, blue, red},
		{7, 2, 3, white, green},
		{8, 2, 3, green, white},
	}
	for _, tc := range cases {
		got := applyOrientation(src, tc.o)
		if got.Bounds().Dx() != tc.w || got.Bounds().Dy() != tc.h {
			t.Fatalf("orientation %d bounds = %v, want %dx%d", tc.o, got.Bounds(), tc.w, tc.h)
		}
		if c := got.NRGBAAt(0, 0); c != tc.topLeft {
			t.Fatalf("orientation %d top-left = %v, want %v", tc.o, c, tc.topLeft)
		}
		if c := got.NRGBAAt(tc.w-1, 0); c != tc.topRight {
			t.Fatalf("orientation %d top-right = %v, want %v", tc.o, c, tc.topRight)
		}
	}
}

func TestApplyOrientationPreservesAlphaColor(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	c := color.NRGBA{R: 200, G: 100, B: 50, A: 128}
	src.SetNRGBA(0, 0, c)
	got := applyOrientation(src, 1)
	if got.NRGBAAt(0, 0) != c {
		t.Fatalf("orientation 1 alpha color = %v, want %v", got.NRGBAAt(0, 0), c)
	}
	got = applyOrientation(src, 2)
	if got.NRGBAAt(1, 0) != c {
		t.Fatalf("orientation 2 alpha color = %v, want %v", got.NRGBAAt(1, 0), c)
	}
}

func encodeJPEGForTest(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("jpeg encode: %v", err)
	}
	return buf.Bytes()
}

func encodePNGForTest(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

func encodeWebPForTest(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	var buf bytes.Buffer
	if err := genwebp.Encode(&buf, img, genwebp.Options{Quality: 80}); err != nil {
		t.Fatalf("webp encode: %v", err)
	}
	return buf.Bytes()
}

func jpegWithOrientation(base []byte, o int) []byte {
	return jpegWithEXIF(base, tiffWithOrientation(o, false))
}

func jpegWithOrientationBE(base []byte, o int) []byte {
	return jpegWithEXIF(base, tiffWithOrientation(o, true))
}

func jpegWithEXIF(base, tiff []byte) []byte {
	payload := append([]byte("Exif\x00\x00"), tiff...)
	segLen := len(payload) + 2
	app1 := make([]byte, 0, 2+segLen)
	app1 = append(app1, 0xFF, 0xE1)
	app1 = binary.BigEndian.AppendUint16(app1, uint16(segLen))
	app1 = append(app1, payload...)
	out := make([]byte, 0, len(base)+len(app1))
	out = append(out, base[:2]...)
	out = append(out, app1...)
	out = append(out, base[2:]...)
	return out
}

func pngWithOrientation(base []byte, o int) []byte {
	tiff := tiffWithOrientation(o, false)
	var chunk bytes.Buffer
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(tiff)))
	chunk.Write(lenBuf[:])
	chunk.WriteString("eXIf")
	chunk.Write(tiff)
	crc := crc32.NewIEEE()
	crc.Write([]byte("eXIf"))
	crc.Write(tiff)
	var crcBuf [4]byte
	binary.BigEndian.PutUint32(crcBuf[:], crc.Sum32())
	chunk.Write(crcBuf[:])

	idx := bytes.LastIndex(base, []byte("IEND"))
	if idx < 4 {
		return base
	}
	insertAt := idx - 4
	out := make([]byte, 0, len(base)+chunk.Len())
	out = append(out, base[:insertAt]...)
	out = append(out, chunk.Bytes()...)
	out = append(out, base[insertAt:]...)
	return out
}

func webpWithOrientation(base []byte, o int) []byte {
	if len(base) < 12 || string(base[:4]) != "RIFF" || string(base[8:12]) != "WEBP" {
		return base
	}
	tiff := tiffWithOrientation(o, false)
	var chunks bytes.Buffer
	for i := 12; i+8 <= len(base); {
		size := int(binary.LittleEndian.Uint32(base[i+4 : i+8]))
		end := i + 8 + size
		if end > len(base) {
			break
		}
		chunks.Write(base[i:end])
		if size%2 == 1 && end < len(base) {
			chunks.WriteByte(base[end])
			end++
		}
		i = end
	}
	var hdr [8]byte
	copy(hdr[:4], "EXIF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(len(tiff)))
	chunks.Write(hdr[:])
	chunks.Write(tiff)
	if len(tiff)%2 == 1 {
		chunks.WriteByte(0)
	}
	body := append([]byte("WEBP"), chunks.Bytes()...)
	out := make([]byte, 8+len(body))
	copy(out[:4], "RIFF")
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(body)))
	copy(out[8:], body)
	return out
}

func tiffWithOrientation(o int, bigEndian bool) []byte {
	var buf bytes.Buffer
	var order binary.ByteOrder = binary.LittleEndian
	if bigEndian {
		buf.WriteString("MM")
		order = binary.BigEndian
	} else {
		buf.WriteString("II")
	}
	var tmp [8]byte
	order.PutUint16(tmp[0:2], 42)
	order.PutUint32(tmp[2:6], 8)
	buf.Write(tmp[:6])

	order.PutUint16(tmp[0:2], 1)
	buf.Write(tmp[:2])
	order.PutUint16(tmp[0:2], 0x0112)
	order.PutUint16(tmp[2:4], 3)
	order.PutUint32(tmp[4:8], 1)
	buf.Write(tmp[:8])
	var value [4]byte
	order.PutUint16(value[0:2], uint16(o))
	buf.Write(value[:])
	order.PutUint32(tmp[0:4], 0)
	buf.Write(tmp[:4])
	return buf.Bytes()
}
