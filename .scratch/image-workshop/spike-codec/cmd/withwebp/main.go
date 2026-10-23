package main

import (
	"bytes"
	genwebp "github.com/gen2brain/webp"
	"image"
)

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	var buf bytes.Buffer
	_ = genwebp.Encode(&buf, img, genwebp.Options{Quality: 80})
	println(buf.Len())
}
