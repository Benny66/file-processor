# 001 — Kylin 兼容性 spike

**Blocked by**: 无

**规格**: §5.3、§7.2；D-03、D-04、D-30

## 目标

在真实银河麒麟 V10 SP1（2203，x86_64）机器上确认 WebKit/GTK/glibc、构建标签、托盘可用性与 `.deb` 最小依赖，为 011 给出明确方案；本切片不改产品功能。

## 涉及层

- [ ] 数据层：无。
- [ ] 逻辑层：在麒麟机器执行诊断；用当前脚手架或最小 Wails 示例分别验证 WebKit 4.0/4.1 构建与启动；验证原生对话框、拖拽、关闭、托盘。
- [ ] UI 层：只做启动与交互验证，不新增产品 UI。
- [ ] 测试：记录命令输出、构建日志、安装/启动结果；结论写入本文件；与 ADR-0002 不一致时更新 ADR。

## 验收标准

- `ldd --version`、`uname -m`、`dpkg -l | grep -E 'libgtk-3|libwebkit2gtk|appindicator'` 输出记录在案。
- 明确 Wails 构建 tag（默认 4.0 或 `webkit2_41`）与构建基础镜像；镜像 glibc 不得高于麒麟。
- 明确托盘是否可用；不可用时关闭即退出，不依赖托盘恢复窗口。
- 明确 `.deb` 的 Depends 最小集合、desktop entry、图标与安装路径。
- 与 ADR-0002 初始假设不同时，先更新 ADR 与本文件，再继续。
- 不在产品路径留下临时 spike 代码；临时工程放在 `.scratch` 或明确清理。
- 未在麒麟真机上跑过时不得标记完成；证据不足就保持进行中。

## 环境结论（2026-10-23，客户回传真机）

| 项目 | 实测 | 结论 |
|---|---|---|
| 架构 | x86_64 | 构建目标 linux/amd64 |
| glibc | 2.31（Ubuntu GLIBC 2.31-0kylin9.1k20.5） | Ubuntu 20.04 基础镜像与目标同版本，不得更高 |
| WebKit | 仅 `libwebkit2gtk-4.0-37` 2.28.1，无 4.1 | 使用 Wails 默认 4.0 tag；禁止 `webkit2_41` |
| GTK3 | `libgtk-3-0` 3.24.30 | 运行时齐全 |
| 开发包 | 无 `libgtk-3-dev`、无 `libwebkit2gtk-4.0-dev` | 真机不能自编译，必须外部产出二进制 |
| 会话 | `XDG_SESSION_TYPE=x11` | 走 X11 路径，不是 Wayland |
| appindicator | `libappindicator3-1` 已装 | 不能作为托盘可用证据（代码走 DBus StatusNotifierItem） |

**派生方案**
- 构建：Ubuntu 20.04 容器内 `wails build -platform linux/amd64 -tags nodynamic`（默认 4.0）；前端在宿主构建后随源码带入，容器不装 Node。Apple Silicon 上容器必须加 `--platform linux/amd64`（已踩过默认 arm64 的坑），脚本内含 `uname -m` 断言。
- `.deb`（011 实现）：Depends 最小集合 `libc6 (>= 2.31)`、`libgtk-3-0`、`libwebkit2gtk-4.0-37`；二进制 `/usr/bin/imageworkshop`；desktop entry `/usr/share/applications/imageworkshop.desktop`；图标 `/usr/share/icons/hicolor/256x256/apps/imageworkshop.png`；不声明 appindicator。
- 兼容修正（本切片已完成）：显式开启 `DragAndDrop.EnableFileDrop` 并整窗接收拖拽；关闭语义改为运行期判定——托盘注册成功才隐藏，否则真退出（`OnBeforeClose` + `TrayAvailable` 绑定，附单测）。
- 现有脚本冲突（011 必须处理）：`scripts/package.sh` 与 `scripts/release.sh` 的 Linux 分支硬编码 `-tags webkit2_41`，在只有 4.0 的麒麟上会链接失败；需改为默认 4.0，并在 Ubuntu 20.04 容器内构建 `.deb`。
- 构建网络（011 必须处理）：容器内 `proxy.golang.org` 不可达；apt 换国内镜像、Go 设 `GOPROXY=https://goproxy.cn,direct` 后可继续。Apple Silicon 还需 `--platform linux/amd64`。

## 真机冒烟清单（待执行）

先回答 6 个问题，每项都有明确判定：

1. **能否启动**：窗口出现、标题为“图片工坊”、中文正常；进程不退出。失败则取 `~/.config/ImageWorkshop/app.log`。
2. **托盘是否可用**：UKUI 面板出现应用图标，右键菜单可“显示/退出”。图标不出现即判不可用。
3. **关闭语义是否正确**：有托盘时点关闭应隐藏，再次双击二进制应恢复原窗口；无托盘时点关闭应直接退出（`pgrep -a ImageWorkshop` 无输出）。
4. **原生对话框**：点“添加文件”弹出 GTK 文件选择器，选中一张 JPG 后队列出现该行（取消也不应报错）。
5. **拖拽**：把一张 JPG 或文件夹拖进窗口，队列出现对应文件或递归文件。
6. **一次真实处理**：开启目标体积 100 KB 后开始，产物为同目录 `-out.jpg` 且体积 ≤ 100 KB；打开对比能看到原图与结果。

逐条执行，不要一次贴完；每条结果回报后再给下一条。启动命令：

```bash
chmod +x ImageWorkshop-linux-amd64
./ImageWorkshop-linux-amd64
```

失败时补一条：

```bash
tail -n 50 ~/.config/ImageWorkshop/app.log
```

## 完成记录

- 状态：进行中（环境结论与外部构建完成；待真机冒烟）
- 实现说明：
  - 环境诊断由客户在真机执行并回传摘要；结论已写入上表与 ADR-0002。
  - `main.go` 显式开启 `DragAndDrop.EnableFileDrop`；`OnFileDrop` 改为整窗接收（队列非空时页面没有 drop-target 元素，依赖 CSS 落点会让拖拽静默失效）。
  - 关闭行为改为运行期判定：`App.beforeClose` 仅在托盘注册成功且非主动退出时隐藏，否则放行退出；新增 `TrayAvailable` 绑定供自绘标题栏的关闭按钮使用；删除静态 `HideWindowOnClose` 开关。
  - Ubuntu 20.04 容器构建 linux/amd64 二进制（默认 4.0 tag、`nodynamic`、跳过容器内前端构建），产物与 `ldd` 证据存放于 `.scratch/kylin/`。
- 验收证据：
  - 客户回传：架构、glibc、WebKit、GTK3、开发包缺失、`XDG_SESSION_TYPE=x11`（见上表）。
  - 构建：原生 arm64 Ubuntu 20.04 容器 + clang/lld 交叉编译 linux/amd64。Apple Silicon 上 QEMU amd64 在 apt postinst（py3compile）与 Go 编译阶段均段错误，已放弃；最终按麒麟的 WebKitGTK 2.28.1 链接（GTK 3.24.20）。
  - 产物：`.scratch/kylin/out/ImageWorkshop-linux-amd64`（19,386,664 字节，SHA256 `53df51afc36b32ad95c984ea066c92b531dfa7e58d4277e6598bc86c668ba279`）。二进制体积大且不入库（`.gitignore` 已忽略 `out/`），通过 U 盘交付。
  - 证据：`.scratch/kylin/evidence/build-record.txt`、`elf.txt`、`needed.txt`、`glibc.txt`、`pkgconfig.txt`、`sha256.txt`。ELF64/x86-64；依赖 `libwebkit2gtk-4.0.so.37`、`libgtk-3.so.0` 等；GLIBC 符号需求仅 2.2.5/2.3.2（麒麟 2.31 满足）。
  - `make test` / `make lint` / `make build` 通过；`hideOnCloseRequested` 单测覆盖“有托盘隐藏 / 托盘退出不拦截 / 无托盘退出”；wiring 护栏新增 `DragAndDrop` 必需项。
  - 真机冒烟结果：待客户执行上面 6 项后回填。
- 偏离与遗留：
  - 托盘实际可用性、原生对话框、拖拽、关闭与真实处理都尚未在麒麟运行，001 不能标记完成。
  - 客户回传的是摘要表，原始 `dpkg -l` 行未逐行提供；`libappindicator3-1` 的实际安装信息来自此前只读检查。
  - `.deb` 打包与安装路径验证属于 011/012。
  - 容器构建脚本目前是 `.scratch/kylin/cross-build.sh` 临时脚本（原生 arm64 容器 + clang/lld 交叉编译）；正式入口留给 011。
  - 二进制只在构建容器内验证了链接与 ELF/GLIBC 需求，尚未在麒麟上执行。
