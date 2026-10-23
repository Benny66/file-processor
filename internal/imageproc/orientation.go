package imageproc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// parseOrientation 读取 JPEG APP1 / PNG eXIf / WebP EXIF / TIFF IFD0 的方向；
// 无方向信息时返回 1。
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
