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

// formatDisplayNames 是各内容格式在前端「支持 …」清单里的展示名。
// 「静态」限定词写在这里：动图会在检查阶段失败（见 animation.go），
// 这是输入能力的一部分，不是文案装饰。
var formatDisplayNames = map[string]string{
	"jpeg": "JPG",
	"png":  "PNG",
	"webp": "WebP",
	"bmp":  "BMP",
	"tiff": "TIFF",
	"gif":  "GIF（静态）",
}

// SupportedFormatHints 按 supportedFormats 的顺序返回去重后的展示名，供前端渲染导入提示。
//
// 为什么由 Go 提供（005→011 累计审查的修复）：这条清单此前在 ImageWorkbench.vue 里
// 手写了一份字符串。往 supportedFormats 加一种格式时，前端那份不会跟着变——
// 「界面宣称支持、引擎其实不支持」是没有任何症状的失效。清单的单一真相只能在 Go。
func SupportedFormatHints() []string {
	seen := make(map[string]bool, len(supportedFormats))
	hints := make([]string, 0, len(formatDisplayNames))
	for _, spec := range supportedFormats {
		if seen[spec.format] {
			continue
		}
		seen[spec.format] = true
		name, ok := formatDisplayNames[spec.format]
		if !ok {
			// 缺展示名也要让这一项出现在清单里（大写兜底），而不是静默少一种格式。
			name = strings.ToUpper(spec.format)
		}
		hints = append(hints, name)
	}
	return hints
}
