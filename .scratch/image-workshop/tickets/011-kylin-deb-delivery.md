# 011 — Kylin 构建与 .deb 交付

**Blocked by**: 001, 003, 006

**规格**: US-31、US-32、US-34；D-03、D-04、D-30

## 目标

CI 产出可安装到银河麒麟 V10 SP1 的 `.deb` 安装包；应用能启动、使用原生对话框与拖拽，并在关闭时不会出现无法恢复的状态。

## 涉及层

- [x] 数据层：`.deb` 元数据、desktop entry、图标、Depends 清单；不改应用数据模型。
- [x] 逻辑层：按 001 的结论选择构建容器（glibc 不高于麒麟）与 Wails WebKit 标签；容器内使用 Go 1.26.x；扩展 `scripts/package.sh` 的 Linux 路径以产出 `.deb`；新增可重复的 make 目标；新增 Linux CI 构建与制品上传；按 spike 结论处理 Linux 关闭/托盘行为，移除对 `webkit2_41` 的硬编码假设。
- [x] UI 层：不新增 UI；验证对话框、拖拽、关闭行为。
- [ ] 测试：CI 构建；麒麟真机安装/启动/关闭；`ldd` 检查；依赖完整性；macOS/Windows 原有构建不回归。

## 验收标准

- CI 在确认过的基础镜像、Go 1.26.x 与 WebKit 标签下产出 `.deb`，并上传制品。
- `.deb` 可在麒麟 V10 SP1（2203，x86_64）安装；应用启动成功；原生对话框与拖拽可用。
- 关闭窗口不会出现“窗口消失且无法恢复”；托盘不可用时关闭即退出。
- `.deb` 的 Depends 与实际链接库一致，安装后无未声明的缺失库。
- `make package os=linux` 或等价文档化目标在构建容器内可用。
- macOS/Windows 构建与打包不回归。
- 麒麟安装/启动证据记录在 ticket；未验证不得标完成。
- 若 spike 结论与 ADR-0002 不同，更新 ADR 与本 ticket。

## 完成记录

- 状态：进行中（本地 `.deb` 已产出并静态验证；待 CI 运行与麒麟真机安装/启动）
- 实现说明：
  - 构建：新增 `build/linux/Dockerfile`（Ubuntu 20.04 / Go 1.26.2 / WebKitGTK 4.0 **2.28.1**）。x86_64 宿主原生 gcc；Apple Silicon 用原生 arm64 容器 + clang/lld 交叉编译 x86_64（QEMU 模拟 amd64 在 dpkg/Go 阶段会段错误，已放弃）。
  - 打包：新增 `scripts/package-deb.sh`：`/usr/bin/ImageWorkshop`、`/usr/share/applications/imageworkshop.desktop`、hicolor 1024 图标；Depends `libc6 (>= 2.31), libgtk-3-0, libwebkit2gtk-4.0-37 (>= 2.28.1)`；宿主无 `dpkg-deb` 时用容器打包。
  - 入口：`scripts/package.sh linux` 改为产出 `.deb`（Linux 原生；macOS/Windows 经 Docker），新增 `make linux-deb`；`scripts/release.sh` 的 Linux 产物改为 `.deb`。
  - CI：`build.yml` 新增 `linux-deb` job（`container: ubuntu:20.04`，WebKitGTK 2.28.1，上传 `linux-deb` artifact）。
  - 关闭/托盘：001 已改为运行期判定；Linux 不再硬编码 `webkit2_41`。
    - **005→011 累计审查修正**：运行期判定不可达（托盘库注册失败不回传），改为平台声明表；Linux 在 001 真机确认前不启用托盘，关闭即退出。验收标准第 3 条「不会出现窗口消失且无法恢复」由此从「靠运行期发现」变成「编译期就不给隐藏的机会」。
  - 交付链路修正（同轮审查）：
    - `scripts/build-linux-container.sh` 原裸调 `wails build`，绕过版本注入 —— `go version -m` 实测旧产物只有 `-ldflags="-w -s"`，`.deb` control 写 0.1.0 而二进制自报 `dev`。现容器内改走 `scripts/build.sh`，提交号由宿主经 `BUILD_COMMIT` 传入；新产物 `go version -m` 可见 `InjectedVersion=0.1.0`、`InjectedCommit`。
    - `scripts/package-deb.sh` 与 `app-name.sh` 原用 python3 读 `wails.json`，而 `ubuntu:20.04` 镜像【本身没有 python3】——它能跑起来只是因为 job 里 `apt-get install libgtk-3-dev …` 把 python3 当传递依赖顺带装了（模拟安装可见 `Inst python3`）。**审查时我据此判「job 一推必红」是错的**：推送后 `b7d1eb2` 与 `ef6cea4` 的 `linux-deb` 都实测 success。改用新增的 `scripts/wails-info.sh`（sed 实现、字段缺失即报错）依然值得做，但理由是「打包链不该依赖没声明的解释器」，不是「当下就坏」。在 CI 同款镜像内验证六个字段读取通过，重新产出的 `.deb` control 逐项一致。
    - 新增护栏：`TestWailsBuildHasSingleEntryPoint`（`wails build` 只允许在 `build.sh`；首跑又抓到 `smoke.sh`、`verify-gen.sh` 两处历史旁路，已一并收口）、`TestBuildScriptsDoNotDependOnPython`（构建脚本禁依赖 python3）。
- 验收证据：
  - `make linux-deb` 全链路通过（前端 → 构建镜像 → 容器内 wails build → .deb）。
  - 产物：`build/bin/ImageWorkshop_0.1.0_amd64.deb`（6.0 MB，当前 SHA256 `00aefafcbfdc26a5e074a5c7f8bc37687521b03f59255e8364a42ec7eb774b82`）。dpkg 打包时间戳会让每次构建的 .deb 哈希变化；内部二进制 BuildID 稳定为 `6ff734d40997ef3e7629fc5043a1c199041cbe9b`。
  - `dpkg-deb -I`：Package imageworkshop / Version 0.1.0 / Architecture amd64 / Depends 如上述；`-c` 路径与权限正确（`./` 为 0755）。
  - 解包二进制：ELF64 x86-64；NEEDED 覆盖 `libc6`、`libgtk-3-0`、`libwebkit2gtk-4.0-37` 及其传递依赖（映射见 `.scratch/kylin/evidence/deb-needs.txt`）。
  - 在 focal multiarch 容器 `dpkg -i` 安装成功（`install ok installed`），桌面文件与图标触发正常；证据见 `.scratch/kylin/evidence/deb-*.txt`。
  - `make test` / `make lint` / `make build`（macOS）通过。
- 偏离与遗留：
  - CI 已真跑并全绿（2026-10-23，run 39967974817 / `ef6cea4`）：`check`、`linux-deb`、`build (macos-latest)`、`build (windows-latest)` 均 success，`.deb` 作为 `linux-deb` artifact 上传。**仍未在麒麟真机安装/启动**——CI 绿只证明能在 focal 里构建打包，不证明麒麟上跑得起来（ADR-0002 的口径）；011 不能标记完成。
  - `release.yml` 的 tag 发布仍只有 macOS/Windows 两条腿，Linux `.deb` 目前只在 `build.yml` 上传 artifact；接入 Release 留给 013。
  - Windows 构建未在本地复跑；本次改动只触及 Linux 分支与新增脚本，macOS 构建已回归。
  - `StartupWMClass=ImageWorkshop` 与麒麟 UKUI 的实际 WM_CLASS 需真机确认；不稳定时只影响任务栏分组。
  - 安装路径使用 `/usr/bin/ImageWorkshop`（US-35 的英文标识），不是小写 `imageworkshop`。
  - `make package os=macos` 使用 `build.sh -clean`，会清空 `build/bin` 中的其他平台产物；交付前需先取走 `.deb`（本次已另存 `.scratch/kylin/out/`），或按 `release.sh` 的顺序最后构建 Linux。
