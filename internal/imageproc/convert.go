package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"

	genwebp "github.com/gen2brain/webp"
	xdraw "golang.org/x/image/draw"

	"file-processor/internal/apperr"
)

// 输出格式取值（单一真相；前端镜像在 frontend/src/lib/image.ts）。
const (
	// OutputKeep 保持源格式（BMP/TIFF/静态 GIF 落到 PNG）。
	OutputKeep = "keep"
	// OutputJPEG 输出 JPEG。
	OutputJPEG = "jpeg"
	// OutputPNG 输出 PNG。
	OutputPNG = "png"
	// OutputWebP 输出 WebP。
	OutputWebP = "webp"
)

// ConvertOptions 控制一次格式转换。
type ConvertOptions struct {
	Format    string
	Quality   int
	MaxWidth  int
	MaxHeight int
}

// Converted 是一次转换的结果。
type Converted struct {
	Data   []byte
	Format string
	Width  int
	Height int
}

// ValidateOutputFormat 校验请求的输出格式取值。
func ValidateOutputFormat(format string) error {
	switch format {
	case OutputKeep, OutputJPEG, OutputPNG, OutputWebP:
		return nil
	default:
		return apperr.Validation("输出格式非法")
	}
}

// NormalizeOutputFormat 把输出选项解析为实际编码格式。
func NormalizeOutputFormat(format, sourceFormat string) (string, error) {
	switch format {
	case OutputKeep:
		switch sourceFormat {
		case "jpeg", "png", "webp":
			return sourceFormat, nil
		case "gif", "bmp", "tiff":
			return OutputPNG, nil
		default:
			return "", apperr.Validation("无法确定输出格式")
		}
	case OutputJPEG, OutputPNG, OutputWebP:
		return format, nil
	default:
		return "", apperr.Validation("输出格式非法")
	}
}

// Convert 解码、方向归一并按选项缩放/重新编码；输出不携带源文件元数据。
func Convert(data []byte, path string, opts ConvertOptions) (Converted, error) {
	p, err := prepareImage(data, path)
	if err != nil {
		return Converted{}, err
	}
	outFormat, err := NormalizeOutputFormat(opts.Format, p.format)
	if err != nil {
		return Converted{}, err
	}
	quality := opts.Quality
	if quality <= 0 {
		quality = 85
	}
	outWidth, outHeight := FitWithin(p.width, p.height, opts.MaxWidth, opts.MaxHeight)
	img := p.img
	if outWidth != p.width || outHeight != p.height {
		img = resizeImage(img, outWidth, outHeight)
	}
	out, err := encodeImage(img, outFormat, quality)
	if err != nil {
		return Converted{}, err
	}
	return Converted{
		Data:   out,
		Format: outFormat,
		Width:  outWidth,
		Height: outHeight,
	}, nil
}

// FitWithin 等比把图片放入宽高上限盒；任一边 <=0 表示该边不限；不放大、不裁切。
func FitWithin(width, height, maxWidth, maxHeight int) (int, int) {
	if width <= 0 || height <= 0 {
		return width, height
	}
	boundedW := maxWidth > 0
	boundedH := maxHeight > 0
	if !boundedW && !boundedH {
		return width, height
	}
	if (!boundedW || width <= maxWidth) && (!boundedH || height <= maxHeight) {
		return width, height
	}
	scaleW := math.Inf(1)
	if boundedW {
		scaleW = float64(maxWidth) / float64(width)
	}
	scaleH := math.Inf(1)
	if boundedH {
		scaleH = float64(maxHeight) / float64(height)
	}
	scale := math.Min(scaleW, scaleH)
	if scale >= 1 {
		return width, height
	}
	w := int(math.Floor(float64(width) * scale))
	h := int(math.Floor(float64(height) * scale))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

func resizeImage(src image.Image, width, height int) image.Image {
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	return dst
}

func encodeImage(img image.Image, format string, quality int) ([]byte, error) {
	var buf bytes.Buffer
	switch format {
	case OutputJPEG:
		if err := jpeg.Encode(&buf, flattenOnWhite(img), &jpeg.Options{Quality: quality}); err != nil {
			return nil, apperr.Wrap(err)
		}
	case OutputPNG:
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&buf, img); err != nil {
			return nil, apperr.Wrap(err)
		}
	case OutputWebP:
		if err := genwebp.Encode(&buf, img, genwebp.Options{Quality: quality}); err != nil {
			return nil, apperr.Wrap(err)
		}
	default:
		return nil, apperr.Validation("输出格式非法")
	}
	return buf.Bytes(), nil
}

func flattenOnWhite(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	return dst
}
