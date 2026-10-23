package spike

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"math/rand"
	"testing"

	dsoprea "github.com/dsoprea/go-exif/v3"
	genwebp "github.com/gen2brain/webp"
	xbmp "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	xtiff "golang.org/x/image/tiff"
	xwebp "golang.org/x/image/webp"
)

func noisyImage(w, h int) *image.NRGBA {
	rng := rand.New(rand.NewSource(42))
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	denom := w - 1
	if denom < 1 {
		denom = 1
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8((x*7 + rng.Intn(64)) & 0xff),
				G: uint8((y*5 + rng.Intn(64)) & 0xff),
				B: uint8((x + y + rng.Intn(64)) & 0xff),
				A: uint8(64 + (x*192)/denom),
			})
		}
	}
	return img
}

func opaque(img *image.NRGBA) *image.NRGBA {
	out := image.NewNRGBA(img.Bounds())
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			c := img.NRGBAAt(x, y)
			c.A = 255
			out.SetNRGBA(x, y, c)
		}
	}
	return out
}

func encodeJPEG(t *testing.T, img image.Image, quality int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		t.Fatalf("jpeg encode: %v", err)
	}
	return buf.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

func encodeGIF(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, &gif.Options{NumColors: 256}); err != nil {
		t.Fatalf("gif encode: %v", err)
	}
	return buf.Bytes()
}

func encodeBMP(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := xbmp.Encode(&buf, img); err != nil {
		t.Fatalf("bmp encode: %v", err)
	}
	return buf.Bytes()
}

func encodeTIFF(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := xtiff.Encode(&buf, img, nil); err != nil {
		t.Fatalf("tiff encode: %v", err)
	}
	return buf.Bytes()
}

func encodeWebP(t *testing.T, img image.Image, quality int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := genwebp.Encode(&buf, img, genwebp.Options{Quality: quality, Method: 4}); err != nil {
		t.Fatalf("webp encode: %v", err)
	}
	return buf.Bytes()
}

func fit(sw, sh, mw, mh int) (int, int) {
	if mw <= 0 || mh <= 0 {
		return sw, sh
	}
	if sw <= mw && sh <= mh {
		return sw, sh
	}
	scale := math.Min(float64(mw)/float64(sw), float64(mh)/float64(sh))
	w := int(math.Round(float64(sw) * scale))
	h := int(math.Round(float64(sh) * scale))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

func TestWebPEncodeQualityAndAlpha(t *testing.T) {
	src := noisyImage(320, 240)
	low := encodeWebP(t, src, 20)
	high := encodeWebP(t, src, 90)
	t.Logf("webp q20=%d bytes, q90=%d bytes", len(low), len(high))
	if len(low) >= len(high) {
		t.Fatalf("expected q20 < q90, got %d >= %d", len(low), len(high))
	}

	img, err := xwebp.Decode(bytes.NewReader(high))
	if err != nil {
		t.Fatalf("x/image/webp decode: %v", err)
	}
	if got := img.Bounds().Dx(); got != 320 {
		t.Fatalf("x/image/webp width=%d", got)
	}
	_, _, _, a := img.At(10, 10).RGBA()
	if a == 0xffff {
		t.Fatalf("expected alpha to survive lossy webp, got opaque pixel")
	}

	img2, err := genwebp.Decode(bytes.NewReader(high))
	if err != nil {
		t.Fatalf("gen2brain/webp decode: %v", err)
	}
	if got := img2.Bounds().Dy(); got != 240 {
		t.Fatalf("gen2brain/webp height=%d", got)
	}
}

func TestAnimatedWebPDetection(t *testing.T) {
	f1 := opaque(noisyImage(64, 64))
	f2 := opaque(noisyImage(64, 64))
	f2.Pix[0] ^= 0xff

	var buf bytes.Buffer
	err := genwebp.EncodeAll(&buf, &genwebp.WEBP{
		Image:     []image.Image{f1, f2},
		Delay:     []int{100, 100},
		LoopCount: 0,
	})
	if err != nil {
		t.Fatalf("webp EncodeAll: %v", err)
	}
	all, err := genwebp.DecodeAll(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("webp DecodeAll: %v", err)
	}
	if len(all.Image) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(all.Image))
	}
	t.Logf("animated webp bytes=%d frames=%d", buf.Len(), len(all.Image))
}

func TestAnimatedGIFDetection(t *testing.T) {
	pal := color.Palette{color.Black, color.White}
	f1 := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
	f2 := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
	for i := range f2.Pix {
		f2.Pix[i] = 1
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, &gif.GIF{
		Image:     []*image.Paletted{f1, f2},
		Delay:     []int{10, 10},
		LoopCount: 0,
	}); err != nil {
		t.Fatalf("gif EncodeAll: %v", err)
	}
	all, err := gif.DecodeAll(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("gif DecodeAll: %v", err)
	}
	if len(all.Image) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(all.Image))
	}
}

func TestResizeFit(t *testing.T) {
	cases := []struct {
		sw, sh, mw, mh int
		wantW, wantH   int
	}{
		{4000, 3000, 1920, 1080, 1440, 1080},
		{800, 600, 1920, 1080, 800, 600},
		{300, 400, 100, 100, 75, 100},
		{100, 50, 100, 100, 100, 50},
		{50, 100, 100, 100, 50, 100},
	}
	for _, tc := range cases {
		gotW, gotH := fit(tc.sw, tc.sh, tc.mw, tc.mh)
		if gotW != tc.wantW || gotH != tc.wantH {
			t.Fatalf("fit(%d,%d,%d,%d) = %dx%d, want %dx%d", tc.sw, tc.sh, tc.mw, tc.mh, gotW, gotH, tc.wantW, tc.wantH)
		}
	}

	src := image.NewNRGBA(image.Rect(0, 0, 4000, 3000))
	dst := image.NewNRGBA(image.Rect(0, 0, 1440, 1080))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	if dst.Bounds().Dx() != 1440 || dst.Bounds().Dy() != 1080 {
		t.Fatalf("resize bounds = %v", dst.Bounds())
	}
}

func TestDecodeFormats(t *testing.T) {
	src := opaque(noisyImage(120, 80))

	jpg := encodeJPEG(t, src, 85)
	if img, err := jpeg.Decode(bytes.NewReader(jpg)); err != nil || img.Bounds().Dx() != 120 {
		t.Fatalf("jpeg decode: %v %v", img, err)
	}

	pngBytes := encodePNG(t, src)
	if img, err := png.Decode(bytes.NewReader(pngBytes)); err != nil || img.Bounds().Dy() != 80 {
		t.Fatalf("png decode: %v %v", img, err)
	}

	gifBytes := encodeGIF(t, src)
	if img, err := gif.Decode(bytes.NewReader(gifBytes)); err != nil || img.Bounds().Dx() != 120 {
		t.Fatalf("gif decode: %v %v", img, err)
	}

	bmpBytes := encodeBMP(t, src)
	if img, err := xbmp.Decode(bytes.NewReader(bmpBytes)); err != nil || img.Bounds().Dy() != 80 {
		t.Fatalf("bmp decode: %v %v", img, err)
	}

	tiffBytes := encodeTIFF(t, src)
	if img, err := xtiff.Decode(bytes.NewReader(tiffBytes)); err != nil || img.Bounds().Dx() != 120 {
		t.Fatalf("tiff decode: %v %v", img, err)
	}

	webpBytes := encodeWebP(t, src, 80)
	if img, err := xwebp.Decode(bytes.NewReader(webpBytes)); err != nil || img.Bounds().Dy() != 80 {
		t.Fatalf("x/image/webp decode: %v %v", img, err)
	}
	if img, err := genwebp.Decode(bytes.NewReader(webpBytes)); err != nil || img.Bounds().Dx() != 120 {
		t.Fatalf("gen2brain/webp decode: %v %v", img, err)
	}

	t.Logf("sizes jpg=%d png=%d gif=%d bmp=%d tiff=%d webp=%d",
		len(jpg), len(pngBytes), len(gifBytes), len(bmpBytes), len(tiffBytes), len(webpBytes))
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
		o             int
		w, h          int
		topLeft, topR color.NRGBA
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
		if c := got.NRGBAAt(tc.w-1, 0); c != tc.topR {
			t.Fatalf("orientation %d top-right = %v, want %v", tc.o, c, tc.topR)
		}
	}
}

func TestOrientationContainers(t *testing.T) {
	src := opaque(noisyImage(32, 24))
	baseJPEG := encodeJPEG(t, src, 85)
	basePNG := encodePNG(t, src)
	baseWebP := encodeWebP(t, src, 80)

	for o := 1; o <= 8; o++ {
		jpg := jpegWithOrientation(baseJPEG, o)
		got, err := parseOrientation(jpg)
		if err != nil || got != o {
			t.Fatalf("jpeg orientation %d: got %d err=%v", o, got, err)
		}
		if libOri, ok := dsopreaOrientation(jpg); ok && libOri != o {
			t.Fatalf("dsoprea orientation %d: got %d", o, libOri)
		}
	}

	beJPEG := jpegWithOrientationBE(baseJPEG, 6)
	if got, err := parseOrientation(beJPEG); err != nil || got != 6 {
		t.Fatalf("big-endian jpeg orientation: got %d err=%v", got, err)
	}

	pngBytes := pngWithOrientation(basePNG, 6)
	if got, err := parseOrientation(pngBytes); err != nil || got != 6 {
		t.Fatalf("png orientation: got %d err=%v", got, err)
	}

	webpBytes := webpWithOrientation(baseWebP, 6)
	if got, err := parseOrientation(webpBytes); err != nil || got != 6 {
		t.Fatalf("webp orientation: got %d err=%v", got, err)
	}

	if got, err := parseOrientation(tiffWithOrientation(6, false)); err != nil || got != 6 {
		t.Fatalf("tiff orientation: got %d err=%v", got, err)
	}

	rotated, err := genwebp.Decode(bytes.NewReader(webpBytes), genwebp.Options{AutoRotate: true})
	if err != nil {
		t.Fatalf("genwebp auto-rotate decode: %v", err)
	}
	if rotated.Bounds().Dx() != 24 || rotated.Bounds().Dy() != 32 {
		t.Fatalf("genwebp auto-rotate bounds = %v, want 24x32", rotated.Bounds())
	}
}

func dsopreaOrientation(jpegData []byte) (int, bool) {
	raw, err := dsoprea.SearchAndExtractExif(jpegData)
	if err != nil {
		return 0, false
	}
	tags, _, err := dsoprea.GetFlatExifData(raw, nil)
	if err != nil {
		return 0, false
	}
	for _, tag := range tags {
		if tag.TagId != 0x0112 {
			continue
		}
		switch v := tag.Value.(type) {
		case []uint16:
			if len(v) > 0 {
				return int(v[0]), true
			}
		case uint16:
			return int(v), true
		case int:
			return v, true
		}
	}
	return 0, false
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

func parseOrientation(data []byte) (int, error) {
	switch {
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8:
		return parseJPEGOrientation(data)
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return parsePNGOrientation(data)
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return parseWebPOrientation(data)
	case len(data) >= 8 && (string(data[:2]) == "II" || string(data[:2]) == "MM"):
		return parseTIFFOrientation(data)
	default:
		return 1, nil
	}
}

func parseJPEGOrientation(data []byte) (int, error) {
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1, fmt.Errorf("jpeg: expected marker at %d", i)
		}
		marker := data[i+1]
		if marker == 0xDA || marker == 0xD9 {
			break
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			i += 2
			continue
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if segLen < 2 || i+2+segLen > len(data) {
			break
		}
		payload := data[i+4 : i+2+segLen]
		if marker == 0xE1 && bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
			return parseTIFFOrientation(payload[6:])
		}
		i += 2 + segLen
	}
	return 1, nil
}

func parsePNGOrientation(data []byte) (int, error) {
	i := 8
	for i+8 <= len(data) {
		size := int(binary.BigEndian.Uint32(data[i : i+4]))
		typ := string(data[i+4 : i+8])
		if i+12+size > len(data) {
			break
		}
		if typ == "eXIf" {
			return parseTIFFOrientation(data[i+8 : i+8+size])
		}
		i += 12 + size
	}
	return 1, nil
}

func parseWebPOrientation(data []byte) (int, error) {
	i := 12
	for i+8 <= len(data) {
		size := int(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		if i+8+size > len(data) {
			break
		}
		typ := string(data[i : i+4])
		if typ == "EXIF" {
			return parseTIFFOrientation(data[i+8 : i+8+size])
		}
		i += 8 + size + (size & 1)
	}
	return 1, nil
}

func parseTIFFOrientation(data []byte) (int, error) {
	if bytes.HasPrefix(data, []byte("Exif\x00\x00")) {
		data = data[6:]
	}
	if len(data) < 8 {
		return 1, fmt.Errorf("tiff: short")
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1, fmt.Errorf("tiff: bad byte order")
	}
	if order.Uint16(data[2:4]) != 42 {
		return 1, fmt.Errorf("tiff: bad magic")
	}
	ifd := int(order.Uint32(data[4:8]))
	if ifd < 8 || ifd+2 > len(data) {
		return 1, fmt.Errorf("tiff: bad ifd offset")
	}
	count := int(order.Uint16(data[ifd : ifd+2]))
	pos := ifd + 2
	for j := 0; j < count; j++ {
		if pos+12 > len(data) {
			break
		}
		tag := order.Uint16(data[pos : pos+2])
		typ := order.Uint16(data[pos+2 : pos+4])
		num := order.Uint32(data[pos+4 : pos+8])
		value := data[pos+8 : pos+12]
		if tag == 0x0112 && num == 1 {
			switch typ {
			case 3:
				return int(order.Uint16(value[:2])), nil
			case 4:
				return int(order.Uint32(value)), nil
			}
		}
		pos += 12
	}
	return 1, nil
}

func applyOrientation(src image.Image, o int) *image.NRGBA {
	if o < 1 || o > 8 {
		o = 1
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var sx, sy int
			switch o {
			case 1:
				sx, sy = x, y
			case 2:
				sx, sy = w-1-x, y
			case 3:
				sx, sy = w-1-x, h-1-y
			case 4:
				sx, sy = x, h-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, h-1-x
			case 7:
				sx, sy = w-1-y, h-1-x
			case 8:
				sx, sy = w-1-y, x
			}
			dst.SetNRGBA(x, y, color.NRGBAModel.Convert(src.At(b.Min.X+sx, b.Min.Y+sy)).(color.NRGBA))
		}
	}
	return dst
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

func TestDsopreaCrossCheck(t *testing.T) {
	base := encodeJPEG(t, opaque(noisyImage(32, 24)), 85)
	for o := 1; o <= 8; o++ {
		jpg := jpegWithOrientation(base, o)
		got, ok := dsopreaOrientation(jpg)
		if !ok {
			t.Fatalf("dsoprea could not parse orientation %d", o)
		}
		if got != o {
			t.Fatalf("dsoprea orientation = %d, want %d", got, o)
		}
	}
}

func TestWebPAnimationChunks(t *testing.T) {
	f1 := opaque(noisyImage(64, 64))
	f2 := opaque(noisyImage(64, 64))
	f2.Pix[0] ^= 0xff
	var buf bytes.Buffer
	if err := genwebp.EncodeAll(&buf, &genwebp.WEBP{
		Image:     []image.Image{f1, f2},
		Delay:     []int{100, 100},
		LoopCount: 0,
	}); err != nil {
		t.Fatalf("webp EncodeAll: %v", err)
	}
	data := buf.Bytes()
	for i := 12; i+8 <= len(data); {
		typ := string(data[i : i+4])
		size := int(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		t.Logf("chunk %q size %d", typ, size)
		if i+8+size > len(data) {
			break
		}
		i += 8 + size + (size & 1)
	}
}

func isAnimatedWebP(data []byte) bool {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return false
	}
	for i := 12; i+8 <= len(data); {
		typ := string(data[i : i+4])
		size := int(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		if typ == "ANIM" || typ == "ANMF" {
			return true
		}
		if i+8+size > len(data) {
			return false
		}
		i += 8 + size + (size & 1)
	}
	return false
}

func isAnimatedGIF(data []byte) bool {
	if len(data) < 13 || (string(data[:6]) != "GIF87a" && string(data[:6]) != "GIF89a") {
		return false
	}
	i := 13
	packed := data[10]
	if packed&0x80 != 0 {
		i += 3 * (1 << ((packed & 0x07) + 1))
	}
	frames := 0
	for i < len(data) {
		switch data[i] {
		case 0x3B:
			return frames > 1
		case 0x21:
			i += 2
			for i < len(data) {
				n := int(data[i])
				i++
				if n == 0 {
					break
				}
				i += n
			}
		case 0x2C:
			frames++
			if frames > 1 {
				return true
			}
			if i+10 > len(data) {
				return false
			}
			packed = data[i+9]
			i += 10
			if packed&0x80 != 0 {
				i += 3 * (1 << ((packed & 0x07) + 1))
			}
			if i >= len(data) {
				return false
			}
			i++
			for i < len(data) {
				n := int(data[i])
				i++
				if n == 0 {
					break
				}
				i += n
			}
		default:
			return false
		}
	}
	return frames > 1
}

func TestLightweightAnimationDetection(t *testing.T) {
	animWebP := func() []byte {
		f1 := opaque(noisyImage(32, 32))
		f2 := opaque(noisyImage(32, 32))
		f2.Pix[0] ^= 0xff
		var buf bytes.Buffer
		if err := genwebp.EncodeAll(&buf, &genwebp.WEBP{
			Image:     []image.Image{f1, f2},
			Delay:     []int{100, 100},
			LoopCount: 0,
		}); err != nil {
			t.Fatalf("webp EncodeAll: %v", err)
		}
		return buf.Bytes()
	}()
	staticWebP := encodeWebP(t, opaque(noisyImage(32, 32)), 80)
	if !isAnimatedWebP(animWebP) {
		t.Fatalf("failed to detect animated webp")
	}
	if isAnimatedWebP(staticWebP) {
		t.Fatalf("static webp misdetected as animated")
	}

	pal := color.Palette{color.Black, color.White}
	f1 := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
	f2 := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
	for i := range f2.Pix {
		f2.Pix[i] = 1
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, &gif.GIF{
		Image:     []*image.Paletted{f1, f2},
		Delay:     []int{10, 10},
		LoopCount: 0,
	}); err != nil {
		t.Fatalf("gif EncodeAll: %v", err)
	}
	if !isAnimatedGIF(buf.Bytes()) {
		t.Fatalf("failed to detect animated gif")
	}
	staticGIF := encodeGIF(t, opaque(noisyImage(32, 32)))
	if isAnimatedGIF(staticGIF) {
		t.Fatalf("static gif misdetected as animated")
	}
}
