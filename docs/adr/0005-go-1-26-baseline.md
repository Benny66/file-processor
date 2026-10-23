# 构建基线升到 Go 1.26

引入纯 Go WebP 有损编码器 `github.com/gen2brain/webp` 后，其 2.6MB WASM 转译源在 Go 1.25 下冷编译耗时 516s、峰值内存 2.245GB，并在当前环境被 kill（原因未确认为 OOM 还是工具内存限制）；Go 1.26.2 冷编译 34.9s、峰值 898MB。决定：产品构建基线升到 Go 1.26.x，`go.mod`、CI `setup-go` 与 README 三处同步，所有平台使用同一基线。

**备选方案**：继续 Go 1.25 并更换 WebP 方案（仅无损 WebP，或引入 CGO libwebp，牺牲目标体积或无额外 CGO 约束）；把 WebP 移出首版（违背已定格式范围）。

**后果**：Go 1.26 成为 Wails、CI 与 Kylin 构建容器的共同前提；003 必须在 005 引入 WebP 依赖前完成升级，否则 CI 会在编译 WebP 库时失败。后续升级 WebP 库或 Go 版本时需重新测量冷编译内存与时间。
