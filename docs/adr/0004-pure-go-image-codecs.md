# 图像编解码保持纯 Go，首版不做 AVIF / HEIC

三平台（含 Kylin）交付要求尽量不引入额外的系统编解码器与 CGO 依赖。决定：输出只做 JPEG / PNG / WebP，输入额外兼容 BMP / TIFF / GIF（仅静态图）；首版不支持 AVIF / HEIC。WebP 编码器也必须选择无 CGO 的实现。

**备选方案**：加 AVIF（更小体积，但需 CGO 或较重的转译层，打包与 CI 成本上升）；全格式含 HEIC（需系统编解码器，Windows 与麒麟环境一致性最难）。

**后果**：会失去 AVIF 的压缩优势和 iPhone HEIC 的原生输入；如果后续要支持，必须重新评估 Linux/.deb 依赖与三平台打包链路，不能只加一个 Go 依赖了事。

**确认（2026-10-23，002 spike）**：WebP 采用 `github.com/gen2brain/webp` v0.6.4 并统一带 `nodynamic` 构建标签，只走内置纯 Go 后端；EXIF 方向用自研最小解析器。`HugoSmits86/nativewebp` 仅支持无损、`dsoprea/go-exif` 依赖过重，均不采用。
