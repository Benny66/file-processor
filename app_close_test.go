package main

import (
	"context"
	"runtime"
	"testing"

	"file-processor/internal/tray"
)

// TestHideOnCloseRequested 关闭语义谓词：托盘活着且非主动退出才隐藏。
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
		{"无托盘且主动退出同样直接放行", false, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{}
			a.trayAlive.Store(tc.trayAlive)
			a.quitting.Store(tc.quitting)
			if got := a.hideOnCloseRequested(); got != tc.want {
				t.Errorf("hideOnCloseRequested() = %v，期望 %v", got, tc.want)
			}
		})
	}
}

// TestTraySupportedPerPlatform 平台托盘声明表。
//
// 为什么要有这张表：托盘声明是关闭语义的唯一输入，判错不报错、不崩溃，
// 只是「关掉窗口后应用再也找不回来」（US-34 / D-30 禁止的状态）。
// 本测试把三平台的期望值写死在这里，改声明必须同时改期望与 ADR-0002。
func TestTraySupportedPerPlatform(t *testing.T) {
	want := map[string]bool{
		"darwin":  false, // Wails v2 占住 NSApplication delegate，无解
		"windows": true,  // Shell_NotifyIcon 可用，US-34 要求关到托盘
		"linux":   false, // 麒麟 watcher 未经 001 真机验证，D-30 取「未验证即退出」
	}
	expected, ok := want[runtime.GOOS]
	if !ok {
		t.Fatalf("未登记的运行平台 %s —— 请先在 internal/tray/supported_<goos>.go 声明托盘能力并补进本表", runtime.GOOS)
	}
	if got := tray.Supported(); got != expected {
		t.Errorf("tray.Supported()(%s) = %v，期望 %v（若确要变更，同步更新 ADR-0002 与 tickets/001、011）",
			runtime.GOOS, got, expected)
	}
}

// TestStartupWithoutTrayKeepsCloseAsQuit 是真正的接线回归测试。
//
// 曾经的缺陷：macOS 的 tray.Start 是 no-op 却返回 nil error，startup 据此把
// trayAlive 置真，关闭变成隐藏——而上面的谓词测试喂的是手工 trayAlive，
// 完全测不到这条链路，绿灯照过。本测试从 startup 的实际产物（trayAlive）断言，
// 不看函数的返回码。
func TestStartupWithoutTrayKeepsCloseAsQuit(t *testing.T) {
	if tray.Supported() {
		t.Skip("本平台声明支持托盘，跳过「无托盘」分支")
	}
	a := NewApp(nil, nil)
	a.startup(context.Background())
	if a.trayAlive.Load() {
		t.Fatal("平台不支持托盘，startup 后 trayAlive 仍为真 —— 关闭窗口会变成无法恢复的隐藏")
	}
	if a.hideOnCloseRequested() {
		t.Fatal("无托盘时 hideOnCloseRequested() 应为 false（关闭即退出）")
	}
}
