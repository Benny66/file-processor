// Package imageproc 提供图片处理的纯函数能力。
package imageproc

import (
	"path/filepath"
	"strings"
)

var supportedExtensions = [...]string{
	".jpg", ".jpeg", ".png", ".webp", ".bmp", ".tif", ".tiff", ".gif",
}

// IsSupportedPath 判断路径扩展名是否受支持（大小写不敏感）。
func IsSupportedPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, supported := range supportedExtensions {
		if ext == supported {
			return true
		}
	}
	return false
}

// DialogPattern 返回文件对话框过滤模式，同时包含小写与大写扩展名。
func DialogPattern() string {
	patterns := make([]string, 0, len(supportedExtensions)*2)
	for _, ext := range supportedExtensions {
		patterns = append(patterns, "*"+ext, "*"+strings.ToUpper(ext))
	}
	return strings.Join(patterns, ";")
}
