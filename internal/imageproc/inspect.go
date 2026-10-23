package imageproc

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"path/filepath"
	"strings"

	genwebp "github.com/gen2brain/webp"
	xbmp "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	xtiff "golang.org/x/image/tiff"

	"file-processor/internal/apperr"
)

const (
	// MaxFileBytes 单张图片文件体积上限。
	MaxFileBytes int64 = 512 * 1024 * 1024
	// MaxPixels 单张图片像素数上限（100 MP）。
	MaxPixels int64 = 100_000_000
	// ThumbnailMaxEdge 队列缩略图最长边。
	ThumbnailMaxEdge = 96
)

// Info 是图片检查结果；缩略图为 PNG data URL，保留透明通道。
type Info struct {
	Path      string `json:"path"`
	Format    string `json:"format"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Bytes     int64  `json:"bytes"`
	Thumbnail string `json:"thumbnail"`
	Warning   string `json:"warning,omitempty"`
}

// preparedImage 是检查与转换共用的解码结果（方向已归一）。
type preparedImage struct {
	format   string
	width    int
	height   int
	img      image.Image
	warnings []string
}

func prepareImage(data []byte, path string) (*preparedImage, error) {
	if int64(len(data)) > MaxFileBytes {
		return nil, apperr.Validation("图片超过 512 MB 上限")
	}

	format := detectContentFormat(data)
	if format == "" {
		return nil, apperr.Validation("无法识别的图片格式")
	}
	var warnings []string
	if extFormat := FormatFromPath(path); extFormat != "" && extFormat != format {
		warnings = append(warnings, fmt.Sprintf(
			"扩展名与实际格式不一致（%s → %s）",
			strings.ToUpper(filepath.Ext(path)),
			strings.ToUpper(format),
		))
	}

	if format == "gif" && isAnimatedGIF(data) {
		return nil, apperr.Validation("首版不支持动图")
	}
	if format == "webp" && isAnimatedWebP(data) {
		return nil, apperr.Validation("首版不支持动图")
	}

	orientation, orientErr := parseOrientation(data)
	if orientErr != nil {
		warnings = append(warnings, "无法读取拍摄方向，已按默认方向显示")
	}

	width, height, err := decodeConfig(format, data)
	if err != nil {
		return nil, apperr.Validation("无法解码图片，文件可能已损坏")
	}
	if int64(width)*int64(height) > MaxPixels {
		return nil, apperr.Validation("图片超过 100 MP 上限")
	}
	if orientation >= 5 && orientation <= 8 {
		width, height = height, width
	}

	img, err := decodeImage(format, data)
	if err != nil {
		return nil, apperr.Validation("无法解码图片，文件可能已损坏")
	}
	if orientation > 1 {
		img = applyOrientation(img, orientation)
	}
	return &preparedImage{
		format:   format,
		width:    width,
		height:   height,
		img:      img,
		warnings: warnings,
	}, nil
}

// Inspect 处理图片字节并生成方向归一后的缩略图；path 仅用于展示与扩展名提示。
// 动图、损坏、超限返回可读的 apperr，不生成缩略图。
func Inspect(data []byte, path string) (Info, error) {
	p, err := prepareImage(data, path)
	if err != nil {
		return Info{}, err
	}
	thumbnail, err := makeThumbnail(p.img, ThumbnailMaxEdge)
	if err != nil {
		return Info{}, apperr.Wrap(err)
	}
	return Info{
		Path:      path,
		Format:    p.format,
		Width:     p.width,
		Height:    p.height,
		Bytes:     int64(len(data)),
		Thumbnail: thumbnail,
		Warning:   strings.Join(p.warnings, "；"),
	}, nil
}

func detectContentFormat(data []byte) string {
	switch {
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8:
		return "jpeg"
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "png"
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "gif"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "webp"
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		return "bmp"
	case len(data) >= 4 && (bytes.Equal(data[:4], []byte{'I', 'I', 42, 0}) || bytes.Equal(data[:4], []byte{'M', 'M', 0, 42})):
		return "tiff"
	default:
		return ""
	}
}

func decodeConfig(format string, data []byte) (int, int, error) {
	var (
		cfg image.Config
		err error
	)
	switch format {
	case "jpeg":
		cfg, err = jpeg.DecodeConfig(bytes.NewReader(data))
	case "png":
		cfg, err = png.DecodeConfig(bytes.NewReader(data))
	case "gif":
		cfg, err = gif.DecodeConfig(bytes.NewReader(data))
	case "webp":
		cfg, err = genwebp.DecodeConfig(bytes.NewReader(data))
	case "bmp":
		cfg, err = xbmp.DecodeConfig(bytes.NewReader(data))
	case "tiff":
		cfg, err = xtiff.DecodeConfig(bytes.NewReader(data))
	default:
		return 0, 0, fmt.Errorf("不支持的格式: %s", format)
	}
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func decodeImage(format string, data []byte) (image.Image, error) {
	switch format {
	case "jpeg":
		return jpeg.Decode(bytes.NewReader(data))
	case "png":
		return png.Decode(bytes.NewReader(data))
	case "gif":
		return gif.Decode(bytes.NewReader(data))
	case "webp":
		return genwebp.Decode(bytes.NewReader(data))
	case "bmp":
		return xbmp.Decode(bytes.NewReader(data))
	case "tiff":
		return xtiff.Decode(bytes.NewReader(data))
	default:
		return nil, fmt.Errorf("不支持的格式: %s", format)
	}
}

func makeThumbnail(src image.Image, maxEdge int) (string, error) {
	thumb := fitThumbnail(src, maxEdge)
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, thumb); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func fitThumbnail(src image.Image, maxEdge int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	long := w
	if h > long {
		long = h
	}
	if long <= maxEdge {
		return src
	}
	scale := float64(maxEdge) / float64(long)
	tw := int(math.Round(float64(w) * scale))
	th := int(math.Round(float64(h) * scale))
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, tw, th))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}
