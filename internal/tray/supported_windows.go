//go:build windows

package tray

// Supported 报告 Windows 是否具备可用的系统托盘。
//
// 恒为 true：Shell_NotifyIcon 由系统提供，注册失败会显式报错（见 start_other.go
// 的调用方处理），不存在「以为有托盘其实没有」的静默状态。
// 规格 US-34 要求 Windows 关闭窗口隐藏到托盘。
func Supported() bool { return true }
