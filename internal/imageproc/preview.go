package imageproc

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"

	"file-processor/internal/apperr"
)

const (
	// PreviewDefaultMaxEdge 是未指定时的预览最长边。
	PreviewDefaultMaxEdge = 1024
	// PreviewMinMaxEdge / PreviewMaxMaxEdge 是预览最长边的合法范围。
	PreviewMinMaxEdge = 64
	PreviewMaxMaxEdge = 2048
)

// Preview 是单张对比用的预览结果；data_url 为 PNG/JPEG data URL。
type Preview struct {
	DataURL  string `json:"data_url"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	HasAlpha bool   `json:"has_alpha"`
}

// RenderPreview 解码、方向归一并等比缩放到 maxEdge 内；透明图输出 PNG，其余输出 JPEG。
func RenderPreview(data []byte, path string, maxEdge int) (Preview, error) {
	if maxEdge <= 0 {
		maxEdge = PreviewDefaultMaxEdge
	}
	if maxEdge < PreviewMinMaxEdge {
		maxEdge = PreviewMinMaxEdge
	}
	if maxEdge > PreviewMaxMaxEdge {
		maxEdge = PreviewMaxMaxEdge
	}

	p, err := prepareImage(data, path)
	if err != nil {
		return Preview{}, err
	}
	alpha := imageHasAlpha(p.img)
	width, height := FitWithin(p.width, p.height, maxEdge, maxEdge)
	img := p.img
	if width != p.width || height != p.height {
		img = resizeImage(img, width, height)
	}

	var buf bytes.Buffer
	var prefix string
	if alpha {
		enc := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := enc.Encode(&buf, img); err != nil {
			return Preview{}, apperr.Wrap(err)
		}
		prefix = "data:image/png;base64,"
	} else {
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			return Preview{}, apperr.Wrap(err)
		}
		prefix = "data:image/jpeg;base64,"
	}
	return Preview{
		DataURL:  prefix + base64.StdEncoding.EncodeToString(buf.Bytes()),
		Width:    width,
		Height:   height,
		HasAlpha: alpha,
	}, nil
}

func imageHasAlpha(img image.Image) bool {
	type opacity interface {
		Opaque() bool
	}
	if o, ok := img.(opacity); ok {
		return !o.Opaque()
	}
	return false
}
