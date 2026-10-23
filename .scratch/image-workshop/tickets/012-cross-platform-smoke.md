# 012 — 跨平台最终冒烟

**Blocked by**: 008, 009, 010, 011

**规格**: US-31、US-32、US-33、US-34、US-36、US-37；全部决策

## 目标

三平台都完成真实安装与处理验收，规格中的核心用户故事全部通过，并留下可审计的证据。

## 涉及层

- [ ] 数据层：使用真实图片、数据库与配置；除缺陷修复外不改产品代码。
- [ ] 逻辑层：在 macOS、Kylin、Windows 跑完整批次；验证离线、源文件不修改、无网络请求。
- [ ] UI 层：手动走查所有状态：空队列、导入中、处理中、失败、跳过、极端降级、对比、过期、预设、重启恢复。
- [ ] 测试：`make test` / `make lint`；CI 三平台构建；Kylin 冒烟清单；Windows 真机/VM 冒烟；证据归档。

## 验收标准

- macOS `.dmg`、Windows NSIS 安装器、Kylin `.deb` 均成功构建。
- macOS：导入 JPEG/PNG/WebP/BMP/TIFF/静态 GIF；验证格式转换、目标体积、宽高盒、对比、预设、重启恢复、源文件不变、无网络请求。
- Kylin：安装 `.deb`，处理 JPEG 与 PNG（含目标体积与宽高盒），验证对比、关闭行为、中文路径与无网络请求。
- Windows：安装/启动并完成一次真实处理；没有 Windows 真机/VM 时本 ticket 保持未完成，不以“CI 编译通过”代替。
- 所有开放失败项清零；已知限制写入交付文档。
- 证据（命令输出、截图/录屏、产物校验）记录在 ticket。
- 不覆盖源文件、无半成品、事件订阅无重复。

## 完成记录

- 状态：进行中（macOS 打包与启动已验；待 Windows、麒麟真机与完整 UI 走查）
- 实现说明：
  - 本切片不做产品代码修改；截至当前只执行了 macOS 产物构建与启动级冒烟。
- 验收证据：
  - `make package os=macos` 成功产出 `build/bin/ImageWorkshop.dmg`（7.7 MB，SHA256 `fa89422032abef6fc7d35ad453d98d7c00b36402471a540d44933d70754bb85b`）。
  - 只读挂载 dmg：包含 `ImageWorkshop.app` 与 `Applications` 软链；Info.plist 版本 0.1.0，bundle id 为占位 `com.example.imageworkshop`（发布前需替换，D-32）。
  - 从挂载卷启动 `.app`：进程存活；app.log 显示版本 0.1.0、日志/数据库/配置初始化、前端挂载 146ms；macOS 按设计跳过托盘；数据目录生成 `ImageWorkshop.db`、`config.json`、`app.log`。证据：`.scratch/kylin/evidence/macos-startup-log.txt`、`macos-dmg-sha256.txt`。
  - `make test` / `make lint` / `make build` 通过。
  - 未完成：完整 UI 走查（六种输入格式、目标体积、宽高盒、对比、预设、重启恢复、源文件不变、无网络请求）；Windows 安装/启动（本机无 Windows 真机/VM，且未安装 NSIS）；麒麟 `.deb` 安装与处理。
- 偏离与遗留：
  - dmg 无 Finder 美化（本会话无 GUI/Finder 自动化权限），但不影响挂载与安装；`STRICT_POLISH` 未开启。
  - 尝试 AppleScript 退出应用被系统拒绝（-128），最终终止进程；因此优雅退出与窗口几何落盘未在本轮验证。
  - 麒麟/Windows 真机未就绪，012 不能标记完成；013 及 tag Release 的 Linux 接入仍被阻塞。
