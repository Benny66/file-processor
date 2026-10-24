package imageproc

import (
	"strings"
	"testing"
)

func TestIsSupportedPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"a.jpg", true},
		{"a.JPEG", true},
		{"a.png", true},
		{"a.WEBP", true},
		{"a.bmp", true},
		{"a.tif", true},
		{"a.TIFF", true},
		{"a.gif", true},
		{"a.pdf", true},
		{"a.PDF", true},
		{"a.txt", false},
		{"a", false},
		{"a.jpg ", false},
		{"a.svga", false},
	}
	for _, tc := range cases {
		if got := IsSupportedPath(tc.path); got != tc.want {
			t.Errorf("IsSupportedPath(%q) = %v，期望 %v", tc.path, got, tc.want)
		}
	}
}

func TestDialogPatternIncludesBothCases(t *testing.T) {
	pattern := DialogPattern()
	for _, want := range []string{"*.jpg", "*.JPG", "*.jpeg", "*.GIF"} {
		if !strings.Contains(pattern, want) {
			t.Errorf("DialogPattern() 缺少 %q：%s", want, pattern)
		}
	}
}

func TestFormatFromPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"a.JPG", "jpeg"},
		{"a.jpeg", "jpeg"},
		{"a.TIF", "tiff"},
		{"a.pdf", "pdf"},
		{"a.txt", ""},
	}
	for _, tc := range cases {
		if got := FormatFromPath(tc.path); got != tc.want {
			t.Errorf("FormatFromPath(%q) = %q，期望 %q", tc.path, got, tc.want)
		}
	}
}

// TestSupportedFormatHintsCoverEverySupportedFormat 防止「引擎支持、清单不显示」：
// supportedFormats 里每个格式都必须有展示名，且去重后各出现一次。
func TestSupportedFormatHintsCoverEverySupportedFormat(t *testing.T) {
	hints := SupportedFormatHints()
	formats := make(map[string]bool)
	for _, spec := range supportedFormats {
		formats[spec.format] = true
	}
	if len(hints) != len(formats) {
		t.Fatalf("展示清单有 %d 项，内容格式有 %d 种——去重或展示名表漂移：%v", len(hints), len(formats), hints)
	}
	for _, spec := range supportedFormats {
		name, ok := formatDisplayNames[spec.format]
		if !ok {
			t.Errorf("格式 %q 没有展示名，前端清单会静默少一种（会走大写兜底）", spec.format)
			continue
		}
		found := false
		for _, hint := range hints {
			if hint == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("清单缺少 %q（格式 %q）：%v", name, spec.format, hints)
		}
	}
}
