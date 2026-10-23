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
