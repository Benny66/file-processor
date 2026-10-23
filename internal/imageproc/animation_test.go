package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"testing"

	genwebp "github.com/gen2brain/webp"
)

func TestAnimationDetection(t *testing.T) {
	staticGIF := encodeStaticGIF(t)
	if isAnimatedGIF(staticGIF) {
		t.Fatalf("static gif misdetected as animated")
	}
	animatedGIF := encodeAnimatedGIF(t)
	if !isAnimatedGIF(animatedGIF) {
		t.Fatalf("animated gif not detected")
	}

	staticWebP := encodeStaticWebP(t)
	if isAnimatedWebP(staticWebP) {
		t.Fatalf("static webp misdetected as animated")
	}
	animatedWebP := encodeAnimatedWebP(t)
	if !isAnimatedWebP(animatedWebP) {
		t.Fatalf("animated webp not detected")
	}
}

func encodeStaticGIF(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 16), G: uint8(y * 16), B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, &gif.Options{NumColors: 16}); err != nil {
		t.Fatalf("gif encode: %v", err)
	}
	return buf.Bytes()
}

func encodeAnimatedGIF(t *testing.T) []byte {
	t.Helper()
	pal := color.Palette{color.Black, color.White}
	f1 := image.NewPaletted(image.Rect(0, 0, 16, 16), pal)
	f2 := image.NewPaletted(image.Rect(0, 0, 16, 16), pal)
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
	return buf.Bytes()
}

func encodeStaticWebP(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 16), G: uint8(y * 16), B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := genwebp.Encode(&buf, img, genwebp.Options{Quality: 80}); err != nil {
		t.Fatalf("webp encode: %v", err)
	}
	return buf.Bytes()
}

func encodeAnimatedWebP(t *testing.T) []byte {
	t.Helper()
	f1 := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	f2 := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			f1.SetNRGBA(x, y, color.NRGBA{R: 40, G: 80, B: 160, A: 255})
			f2.SetNRGBA(x, y, color.NRGBA{R: 160, G: 80, B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := genwebp.EncodeAll(&buf, &genwebp.WEBP{
		Image:     []image.Image{f1, f2},
		Delay:     []int{100, 100},
		LoopCount: 0,
	}); err != nil {
		t.Fatalf("webp EncodeAll: %v", err)
	}
	return buf.Bytes()
}
