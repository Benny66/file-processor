// Package imageproc 提供图片处理的纯函数能力。
package imageproc

import (
	"path/filepath"
	"strings"
)

type formatSpec struct {
	ext    string
	format string
}

var supportedFormats = []formatSpec{
	{".jpg", "jpeg"},
	{".jpeg", "jpeg"},
	{".png", "png"},
	{".webp", "webp"},
	{".bmp", "bmp"},
	{".tif", "tiff"},
	{".tiff", "tiff"},
	{".gif", "gif"},
}

// FormatFromPath 按扩展名返回内容格式（jpeg/png/webp/bmp/tiff/gif），未知返回空串。
func FormatFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	for _, spec := range supportedFormats {
		if spec.ext == ext {
			return spec.format
		}
	}
	return ""
}

// IsSupportedPath 判断路径扩展名是否受支持（大小写不敏感）。
func IsSupportedPath(path string) bool {
	return FormatFromPath(path) != ""
}

// DialogPattern 返回文件对话框过滤模式，同时包含小写与大写扩展名。
func DialogPattern() string {
	patterns := make([]string, 0, len(supportedFormats)*2)
	for _, spec := range supportedFormats {
		patterns = append(patterns, "*"+spec.ext, "*"+strings.ToUpper(spec.ext))
	}
	return strings.Join(patterns, ";")
}
