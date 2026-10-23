//go:build darwin

package tray

import (
	"errors"
)

// ErrUnsupported 表示本平台不提供托盘能力。
//
// 存在的意义：让「在 macOS 上调用 Start」成为一次**显式失败**，而不是一次
// 返回 nil 的伪成功。此前 macOS 的 Start 是 no-op 且返回 nil error，调用方据此
// 认定「托盘活着」，把关闭变成了隐藏——窗口消失且无法找回（US-34 禁止的状态）。
var ErrUnsupported = errors.New("本平台不支持系统托盘")

// Start 在 macOS 上不启动托盘，恒返回 ErrUnsupported（平台声明见 supported_darwin.go）。
func Start(opts Options, h Handlers) (stop func(), err error) {
	return nil, ErrUnsupported
}
