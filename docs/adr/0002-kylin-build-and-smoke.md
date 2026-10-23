# Kylin 采用 CI 近似构建 + 真机冒烟

Kylin 是本产品的一等目标，但 Wails 无法从 macOS 交叉编译 Linux。决定：在 CI 的旧 glibc 容器（初始 Ubuntu 20.04，最终基础镜像由 spike 的 `ldd` 结果决定）构建 Linux 产物并打成 .deb，再在真实麒麟 V10 SP1 机器上安装冒烟；不在麒麟机器上维护完整工具链。

**备选方案**：直接在麒麟上安装 Go/Node/wails 构建（兼容性最真，但企业环境常缺管理员权限或无法联网）；双环境互验（最稳但成本最高）。

**后果**：首个 Linux 任务必须先做兼容性 spike，确认麒麟的 glibc、WebKit/GTK 包版本；若麒麟的 glibc 低于 Ubuntu 20.04，构建基础镜像必须再往旧版本退，不能用更新环境产出的二进制代替。后续 Linux 构建以该 spike 的结果为准，不能把“Ubuntu 上能跑”当作“麒麟上能跑”。

## Spike 结论（2026-10-23，客户回传真机环境）

| 项目 | 实测 | 影响 |
|---|---|---|
| 架构 | x86_64 | 构建目标 linux/amd64 |
| glibc | 2.31 | Ubuntu 20.04 基础镜像合适，不得更高 |
| WebKit | libwebkit2gtk-4.0-37 2.28.1，无 4.1 | 使用 Wails 默认 4.0 tag；禁止 `webkit2_41` |
| GTK3 | libgtk-3-0 3.24.30 | 运行时齐全 |
| 开发包 | 无 libgtk-3-dev / libwebkit2gtk-4.0-dev | 真机不能自编译，必须外部产出二进制 |
| 会话 | XDG_SESSION_TYPE=x11 | 走 X11 路径，不是 Wayland |
| appindicator | 回传摘要未出现该包 | 与托盘走 DBus StatusNotifierItem 一致；实际可用性待运行确认 |

**派生决定**
- 构建：Ubuntu 20.04 容器内构建 linux/amd64，Webkit 走默认 4.0 tag；前端在宿主构建后随源码带入，容器不装 Node。构建必须经 `scripts/build.sh`（版本与提交号注入的唯一入口）——005→011 累计审查发现容器曾裸调 `wails build`，导致 `.deb` 写着 0.1.0 而二进制自报 `dev`，现由护栏 `TestWailsBuildHasSingleEntryPoint` 焊住。
- `.deb` Depends 最小集合（011/012 复核）：`libc6`、`libgtk-3-0`、`libwebkit2gtk-4.0-37`；不声明 appindicator。
- 关闭语义：托盘能力由**平台编译期声明** `tray.Supported()`（darwin=false、windows=true、linux=false），不是运行期探测——`energye/systray` 在注册失败时不回传任何错误，「等运行期发现」在麒麟上没有实现手段。规则仍是 D-30：**未验证可用即关闭退出**。麒麟真机（001）确认 StatusNotifierWatcher 在位、托盘图标与菜单可用后，把 `internal/tray/supported_linux.go` 改为 true 即可切回「隐藏到托盘」，`app_close_test.go` 的平台表需同步。拖拽需显式开启 `DragAndDrop.EnableFileDrop` 并整窗接收。
- 本机（Apple Silicon）构建：`--platform linux/amd64` 的 QEMU 容器在 apt postinst 与 Go 编译阶段都会段错误，不可用；实际采用**原生 arm64 Ubuntu 20.04 容器 + clang/lld 交叉编译**，并按麒麟版本固定 WebKitGTK 2.28.1。CI 的 x86_64 runner 可直接在 amd64 容器内构建，无需交叉。（本条曾与上一条「必须加 `--platform linux/amd64`、脚本已加断言」互相矛盾，2026-10-23 累计审查后以本条为准。）
- **001 仍未完成**：真机启动、原生对话框、拖拽、关闭、托盘与一次真实处理尚未执行，不能把“Ubuntu 20.04 能构建”当成“麒麟上能跑”。
