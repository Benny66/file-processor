package main

import (
	"bytes"
	"image"
	"image/png"
)

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	println(buf.Len())
}
