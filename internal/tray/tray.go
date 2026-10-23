// Package tray 封装系统托盘。
//
// 平台能力由 Supported() 声明，它是**关闭语义的唯一输入**：
//   - Windows：Supported()=true，关闭窗口隐藏到托盘（US-34）。
//   - macOS：Supported()=false（Wails v2 的 NSApplication delegate 被 frontend 占住，
//     与所有 systray 库互斥），关闭窗口即退出。
//   - Linux/麒麟：Supported()=false，直到 001 真机确认 D-Bus StatusNotifierWatcher
//     在位为止；D-30 的口径是「托盘未验证可用时关闭即退出」。
//
// 每个平台必须在 supported_<goos>.go 里给出 Supported()——**漏一个平台就编译失败**，
// 这条缺失本身即护栏（AGENTS.md 铁律 5：无症状的失效要编译成会红的检查）。
// 托盘线程实现见 start_other.go（非 darwin），darwin 侧是显式失败（start_darwin.go）。
package tray

// Handlers 是托盘交互回调，由调用方注入。
type Handlers struct {
	// OnShow 双击/单击托盘图标或点"显示"时触发（恢复窗口）。
	OnShow func()
	// OnQuit 点"退出"时触发（优雅退出应用）。
	OnQuit func()
}

// Options 是托盘启动参数。
type Options struct {
	// Icon 是托盘图标二进制（.ico 或 .png）。macOS 建议用模板图（黑白）。
	Icon []byte
	// Tooltip 悬停提示。
	Tooltip string
}
