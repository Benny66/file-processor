package imageproc

import "encoding/binary"

// isAnimatedWebP 通过 RIFF 块判断是否多帧，不解码全部帧。
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

// isAnimatedGIF 统计 GIF 图像描述符数量，不解码像素。
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
