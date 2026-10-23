package main

import "testing"

// TestHideOnCloseRequested 关闭语义：托盘活着且非主动退出才隐藏；
// 托盘不可用时必须真退出，否则窗口隐藏后没有任何入口找回应用（001 验收）。
func TestHideOnCloseRequested(t *testing.T) {
	cases := []struct {
		name      string
		trayAlive bool
		quitting  bool
		want      bool
	}{
		{"托盘可用时关闭隐藏", true, false, true},
		{"托盘退出路径不拦截", true, true, false},
		{"无托盘时关闭即退出", false, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{trayAlive: tc.trayAlive, quitting: tc.quitting}
			if got := a.hideOnCloseRequested(); got != tc.want {
				t.Errorf("hideOnCloseRequested() = %v，期望 %v", got, tc.want)
			}
		})
	}
}
