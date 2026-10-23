# 002 — 纯 Go 编解码与元数据 spike

**Blocked by**: 无

**规格**: §5.1、§7.2、§7.3；D-05、D-06、D-31

## 目标

验证纯 Go WebP 有损编码、EXIF 方向解析、`x/image` 解码与缩放能在 macOS/Windows/Linux 上不引入额外 CGO 或系统编解码器，并确定依赖与测试 fixture 方案。

## 涉及层

- [x] 数据层：无。
- [x] 逻辑层：临时 spike 程序测试 JPEG/PNG/WebP 编解码、alpha、8 种 EXIF 方向、BMP/TIFF/GIF 静态解码；在 macOS 跑 `go test`，对引擎包做 `GOOS=linux/windows` 交叉编译。
- [x] UI 层：无。
- [x] 测试：记录候选库版本、许可证、二进制体积、API、已知限制；给出 fixture 生成方案与依赖登记理由。

## 验收标准

- 确定 WebP 有损编码库：质量可调、支持 alpha、三平台无额外 CGO/系统库；不能只验证无损 WebP。
- 确定 EXIF 方向方案，覆盖 8 种方向；明确输出剥离 ICC 的策略。
- 确认 `golang.org/x/image` 对 BMP/TIFF/WebP 的解码能力与高质量缩放实现。
- 若没有可行的纯 Go 有损 WebP 方案，明确建议回退格式范围并更新 ADR-0004；不得静默改成无损 WebP。
- 依赖登记项与理由写入本文件；实现阶段直接登记 `deps.yaml`。
- 产出可复用的测试 fixture 生成思路，供 005/006 使用。
- 不留临时产品代码；结论写回本文件。

## 结论

- 采用 `github.com/gen2brain/webp` v0.6.4（MIT）作为 WebP 编解码；构建统一带 `nodynamic`，只走 WASM 转译的纯 Go 后端，不加载系统 libwebp。
- **构建基线必须升到 Go 1.26**：Go 1.25 编译该库 2.6MB 的转译源在 516s / 2.245GB 时被 kill（原因未确认为 OOM 还是工具内存限制）；Go 1.26.2 冷编译 34.9s / 898MB 成功。003 负责升级 `go.mod`、CI 与 README。
- 采用 `golang.org/x/image` v0.45.0（BSD-3，要求 Go 1.25）解码 BMP/TIFF/WebP 静态图与 CatmullRom 缩放；JPEG/PNG/GIF 用标准库。
- EXIF 方向不引入外部库：在 `internal/imageproc` 实现最小容器解析（JPEG APP1、PNG eXIf、WebP EXIF、TIFF IFD0），覆盖大小端与 8 方向。`dsoprea/go-exif` 仅用于 spike 交叉验证，因 130 个依赖不进入产品。
- 动图检测用轻量容器扫描（WebP `ANIM`/`ANMF`；GIF 图像描述符计数），不整段解码所有帧。
- 已否决：`HugoSmits86/nativewebp`（仅无损，不满足目标体积有损压缩）、CGO libwebp（破坏三平台无额外 CGO 目标）、`dsoprea/go-exif`（依赖面过大）。

## 完成记录

- 状态：完成（2026-10-23，spike 结论）
- 实现说明：
  - WebP：`gen2brain/webp` 支持有损质量、alpha、动图编解码与 `AutoRotate`；`nodynamic` 构建标签关闭系统 libwebp 动态加载，只使用内置纯 Go 转译后端。
  - 解码：JPEG/PNG/GIF 用标准库；BMP/TIFF/静态 WebP 用 `golang.org/x/image`；缩放用 `x/image/draw.CatmullRom`。
  - EXIF：最小解析器已覆盖 JPEG APP1、PNG eXIf、WebP EXIF、TIFF IFD0；大小端、8 种方向、方向应用映射均有测试。方向变换使用 `color.NRGBAModel.Convert`，并加了半透明像素回归测试（避免预乘 RGBA 导致变暗）。产品不需要 `dsoprea/go-exif`。
  - 动图：WebP 的轻量 RIFF 扫描可识别 `ANIM`/`ANMF`；GIF 的图像描述符计数可识别多帧；均不依赖完整解码。
  - fixture：测试内置 JPEG APP1、PNG eXIf、WebP EXIF、TIFF IFD0 构造器，可直接复用到 005/006 的 8 方向测试。
- 验收证据：
  - `CGO_ENABLED=0 go test -tags nodynamic ./...` 通过；默认 `go test ./...` 通过；`go vet ./...` 通过。
  - Go 工具链对比：`GOTOOLCHAIN=go1.25.0 go build -p 1` 在 516.50s、max RSS 2,245,033,984 bytes 时 `signal: killed`；`GOCACHE=/tmp/gocache126 go build -p 1` 在 34.88s、max RSS 898,088,960 bytes 时成功。
  - `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -tags nodynamic -c` 与 `GOOS=windows GOARCH=amd64` 均成功；Linux 产物为静态 ELF x86-64，Windows 为 PE32+ x86-64。
  - `otool -L` 对 `nodynamic` 构建无 libwebp 链接；`go list -deps -tags nodynamic` 不含 purego。
  - 二进制体积增量：最小程序从 1,896,770 bytes 增至 4,205,250 bytes（+2,308,480 bytes，约 2.2 MiB）。
  - 质量/体积：320×240 噪声图 WebP q20 = 18,062 bytes，q90 = 53,456 bytes；alpha 在有损 WebP 中保留。
  - 格式解码：JPEG/PNG/GIF/BMP/TIFF/WebP 全部可解码；120×80 噪声图输出 jpg=5,907、png=24,573、gif=10,429、bmp=28,854、tiff=38,594、webp=5,008 bytes。
  - EXIF：8 种方向的 JPEG 均解析正确；大端 JPEG、PNG eXIf、WebP EXIF、TIFF IFD0 均解析正确；`dsoprea` 对同一批 JPEG 交叉验证通过；`gen2brain/webp` 的 `AutoRotate` 对 32×24 + orientation 6 输出 24×32。
  - 动图：GIF 与 WebP 多帧 fixture 均被轻量扫描识别；静态 GIF/WebP 不误报。
  - 许可证：`gen2brain/webp` MIT、`golang.org/x/image` BSD-3、`purego` Apache-2.0（`nodynamic` 下不链接）、`dsoprea` MIT（不采用）。
- 偏离与遗留：
  - 未在 spike 中真实分配 100 MP 图片；产品在 005 用守卫逻辑测试上限判断，不做无意义的大内存分配。
  - 未验证真实手机拍摄的 EXIF fixture；005/006 应补一张真实 JPEG（含 orientation/GPS）验证剥离与方向。
  - 未验证 Wails 进程带 `nodynamic` 的构建；011 需把 `nodynamic` 接入 macOS/Windows/Linux 构建标签并做产物验证。
  - **Go 1.26 是 WebP 依赖的硬前置**：003 必须先升级 `go.mod`、CI `setup-go` 与 README 环境要求，否则 005/006 的 CI 会因 Go 1.25 编译被 kill 而失败。
  - spike 代码保留在 `.scratch/image-workshop/spike-codec/` 作为证据，不进入产品路径；产品依赖在 005/006 引入时再登记 `deps.yaml`（现在登记会造成护栏反向校验失败）。
