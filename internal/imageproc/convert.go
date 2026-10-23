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
	Format      string
	Quality     int
	MaxWidth    int
	MaxHeight   int
	TargetBytes int64
}

// Converted 是一次转换的结果。
type Converted struct {
	Data    []byte
	Format  string
	Width   int
	Height  int
	Quality int
	// ExtremeDegraded 由目标体积搜索判定（阈值见下方常量），前端只负责展示。
	ExtremeDegraded bool
	// AlphaFlattened 表示透明区域被铺了白底（仅 JPEG 输出会出现；US-20 的提示依据）。
	AlphaFlattened bool
}

// DefaultQuality 是未指定质量时 JPEG/WebP 的编码质量。
// 单一真相在此：Convert 与 preview 都引用它，两处各写 85 会漂移。
const DefaultQuality = 85

// 极端降级阈值（单一真相；结果以布尔标记回传，前端不重复判断）。
const (
	ExtremeQualityThreshold  = 20
	ExtremeLongEdgeThreshold = 128
)

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
	outWidth, outHeight := FitWithin(p.width, p.height, opts.MaxWidth, opts.MaxHeight)
	img := p.img
	if outWidth != p.width || outHeight != p.height {
		img = resizeImage(img, outWidth, outHeight)
	}
	// US-20：透明图转 JPEG 会铺白底（编码发生在 encodeImage 内部），这里只判定
	// 「这张图确实带透明」并把结论回传给调用方去提示，前端不再自己判断。
	alphaFlattened := outFormat == OutputJPEG && !isOpaque(img)

	if opts.TargetBytes > 0 {
		converted, err := encodeToTarget(img, outWidth, outHeight, outFormat, opts)
		if err != nil {
			return converted, err
		}
		converted.AlphaFlattened = alphaFlattened
		return converted, nil
	}

	quality := opts.Quality
	if quality <= 0 {
		quality = DefaultQuality
	}
	if outFormat == OutputPNG {
		quality = 0
	}
	out, err := encodeImage(img, outFormat, quality)
	if err != nil {
		return Converted{}, err
	}
	return Converted{
		Data:           out,
		Format:         outFormat,
		Width:          outWidth,
		Height:         outHeight,
		Quality:        quality,
		AlphaFlattened: alphaFlattened,
	}, nil
}

// isOpaque 判断图像是否完全不透明（带 alpha 通道且有半透明像素时为 false）。
// 走标准库的 Opaque()（*image.RGBA / *image.NRGBA 都实现了），不在前端重复一份判断。
func isOpaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return true
}

func encodeToTarget(img image.Image, width, height int, format string, opts ConvertOptions) (Converted, error) {
	current := img
	currentW, currentH := width, height
	shrunk := false
	boxOff := opts.MaxWidth == 0 && opts.MaxHeight == 0
	boxLong := opts.MaxWidth
	if opts.MaxHeight > boxLong {
		boxLong = opts.MaxHeight
	}

	for {
		if format == OutputPNG {
			data, err := encodeImage(current, format, 0)
			if err != nil {
				return Converted{}, err
			}
			if int64(len(data)) <= opts.TargetBytes {
				return Converted{
					Data:            data,
					Format:          format,
					Width:           currentW,
					Height:          currentH,
					Quality:         0,
					ExtremeDegraded: isExtreme(0, currentW, currentH, boxOff, boxLong, shrunk),
				}, nil
			}
			if currentW <= 1 && currentH <= 1 {
				return Converted{}, apperr.Validation("目标体积过小，无法达到")
			}
			nextW, nextH := shrinkDimensions(currentW, currentH, opts.TargetBytes, int64(len(data)))
			current = resizeImage(img, nextW, nextH)
			currentW, currentH = nextW, nextH
			shrunk = true
			continue
		}

		minData, err := encodeImage(current, format, 1)
		if err != nil {
			return Converted{}, err
		}
		if int64(len(minData)) <= opts.TargetBytes {
			data, quality, err := searchQuality(current, format, opts.TargetBytes)
			if err != nil {
				return Converted{}, err
			}
			return Converted{
				Data:            data,
				Format:          format,
				Width:           currentW,
				Height:          currentH,
				Quality:         quality,
				ExtremeDegraded: isExtreme(quality, currentW, currentH, boxOff, boxLong, shrunk),
			}, nil
		}
		if currentW <= 1 && currentH <= 1 {
			return Converted{}, apperr.Validation("目标体积过小，无法达到")
		}
		nextW, nextH := shrinkDimensions(currentW, currentH, opts.TargetBytes, int64(len(minData)))
		current = resizeImage(img, nextW, nextH)
		currentW, currentH = nextW, nextH
		shrunk = true
	}
}

func searchQuality(img image.Image, format string, target int64) ([]byte, int, error) {
	lo, hi := 1, 100
	var best []byte
	bestQuality := 0
	for lo <= hi {
		mid := (lo + hi) / 2
		data, err := encodeImage(img, format, mid)
		if err != nil {
			return nil, 0, err
		}
		if int64(len(data)) <= target {
			best = data
			bestQuality = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if best == nil {
		return nil, 0, apperr.Validation("目标体积过小，无法达到")
	}
	return best, bestQuality, nil
}

func shrinkDimensions(width, height int, target, currentBytes int64) (int, int) {
	ratio := math.Sqrt(float64(target) / float64(currentBytes))
	if !(ratio > 0) || ratio >= 1 {
		ratio = 0.85
	} else {
		ratio *= 0.95
	}
	if ratio > 0.95 {
		ratio = 0.95
	}
	if ratio < 0.1 {
		ratio = 0.1
	}
	nextW := int(math.Floor(float64(width) * ratio))
	nextH := int(math.Floor(float64(height) * ratio))
	if nextW < 1 {
		nextW = 1
	}
	if nextH < 1 {
		nextH = 1
	}
	if nextW == width && nextH == height {
		if width > 1 {
			nextW = width - 1
		}
		if height > 1 {
			nextH = height - 1
		}
	}
	return nextW, nextH
}

func isExtreme(quality, width, height int, boxOff bool, boxLong int, shrunk bool) bool {
	if quality > 0 && quality < ExtremeQualityThreshold {
		return true
	}
	if !shrunk {
		return false
	}
	longEdge := width
	if height > longEdge {
		longEdge = height
	}
	if longEdge >= ExtremeLongEdgeThreshold {
		return false
	}
	if boxOff {
		return true
	}
	return longEdge < boxLong
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
