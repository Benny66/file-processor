//go:build linux

package tray

// Supported 报告 Linux（含银河麒麟 V10 SP1）是否具备**已验证可用**的系统托盘。
//
// 当前为 false，依据是规格 D-30 的裁决口径：「托盘未验证可用时关闭即退出」。
// 麒麟的托盘走 D-Bus StatusNotifierWatcher，而这一点在 001 的真机冒烟里尚未确认——
// 「装了 libappindicator3-1」不是证据，UKUI 是否实现 watcher 才是。
// energye/systray 在注册失败时**不回传任何错误**（RunWithExternalLoop 无回调结果），
// 因此运行期无法感知；在没有真机结论前，唯一安全的取值是 false。
//
// 后果：麒麟上关闭窗口即退出，不启动托盘线程，也就不会出现
// 「窗口消失且无法恢复」（US-34、011 验收标准第 3 条）。
//
// 若 001 真机确认 watcher 在位、托盘图标与右键菜单可用，把此值改为 true 即可，
// 关闭语义随之变为「隐藏到托盘」；同时更新 ADR-0002 与 tickets/001、011 的记录。
func Supported() bool { return false }
