# 信创 arm64 交叉构建（artifact-only）

竞品对比（`docs/竞品对比与迭代建议.md` B3）确认全部竞品均无官方 ARM Linux 构建；图片工坊的图像处理是纯 Go（`nodynamic`），交叉构建成本远低于依赖原生编解码链的对手。决定：新增 linux/arm64 构建腿（信创飞腾/鲲鹏机型），路线与 ADR-0002 已验证的 amd64 交叉**同族**——容器内 clang/lld 交叉，不用 QEMU 模拟（001 记录过 QEMU 在 apt postinst 与 Go 编译阶段段错误）。

**具体路线**：x86_64 的 `ubuntu:20.04` 容器 + `dpkg --add-architecture arm64`（ubuntu-ports 源）+ `:arm64` 后缀的 GTK/WebKit 开发库，`CGO_ENABLED=1 GOARCH=arm64` + `clang --target=aarch64-linux-gnu` + lld；经 `scripts/build-linux-container.sh arm64`（内部仍只调 `scripts/build.sh`，版本注入单一入口不变）。镜像由 `TARGET_ARCH` build-arg 控制预装库的架构，Dockerfile 里 arm64 容器交叉 amd64 的既有分支与本次新增分支互为镜像。

**与 amd64 的差异**：arm64 侧 WebKitGTK **不钉 2.28.1-1**——focal arm64 ports 是否保有该精确版本未核实（无 arm64 真机/镜像可查），WebKitGTK 4.0 API 稳定且 Wails 官方支持更高版本，运行端兼容下限由 `.deb` 的 `Depends: libwebkit2gtk-4.0-37 (>= 2.28.1)` 声明；amd64 侧维持钉死（麒麟 x86_64 真机实测 2.28.1）。构建容器同为 Ubuntu 20.04，glibc 2.31 一致。

**产物边界（D-45）**：arm64 `.deb` 仅作为 CI artifact（`build.yml` 的 `linux-deb-arm64` job），**不接入 tag Release**。飞腾/鲲鹏未经真机验收前不宣称可发布——与「麒麟 x86_64 真机验收未完成」同一原则：能构建 ≠ 能跑。真机验收通过后，将其并入 `release.yml` 的 `package-linux` 腿（届时一并核对麒麟 arm64 真机的 glibc 与 WebKit 实际版本，必要时收紧构建镜像）。

**验证现状（2026-10-24）**：本机无 Docker，构建链由 CI 的 `linux-deb-arm64` job 首次验证（产物断言：aarch64 ELF + `dpkg-deb` 元数据 `Architecture: arm64` 由 `package-deb.sh` 与 job 共同把住）。若 focal arm64 ports 缺某依赖包导致 job 红，优先调整 Dockerfile 的 arm64 依赖清单，不得改为 QEMU。
