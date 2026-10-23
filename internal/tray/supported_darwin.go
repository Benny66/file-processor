//go:build darwin

package tray

// Supported 报告 macOS 是否具备可用的系统托盘。
//
// 恒为 false，且这条声明本身就是关闭语义的护栏：macOS 上「隐藏到托盘」等于
// 「窗口消失且无任何入口能找回」（Wails v2 的 darwin 前端没有 reopen 通道），
// 正是规格 US-34 / D-30 明令禁止的状态。
//
// macOS 之所以没有托盘：Wails v2 的 darwin frontend 自己持有 NSApplication 的
// delegate（跑 WebKit），而所有 systray 库（getlantern 及其 fork）都要 setDelegate
// 来创建 NSStatusBar，两者互斥——链接期 AppDelegate 符号冲突，运行时
// NSInternalInconsistencyException。
//
// 后果：macOS 关闭即退出，且不启动托盘线程。见 start_darwin.go。
func Supported() bool { return false }
