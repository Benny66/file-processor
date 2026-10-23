# 005 — 检查与缩略图

**Blocked by**: 002, 004

**规格**: US-04、US-05、US-07；D-13（部分）、D-15、D-17

## 目标

队列项显示方向正确的缩略图、像素尺寸、体积与状态；损坏、动图、超限文件在检查阶段被可读地标记，不阻塞其它文件。

## 涉及层

- [x] 数据层：无数据库。
- [x] 逻辑层：新增 `internal/imageproc` 的检查能力：内容格式探测、EXIF 方向归一后的宽高、字节数、GIF/WebP 动图检测、100 MP/512 MB 上限、96 px 缩略图（base64）；service 提供 `InspectImage`；绑定层错误走 `apperr`。
- [x] UI 层：队列可见项懒加载检查结果；缩略图、宽高、体积、状态徽标；失败原因展示；失败项可移除；透明缩略图用棋盘格背景。
- [x] 测试：各输入格式检查；方向缩略图；动图 GIF/WebP；损坏文件；超限；中文路径；前端手动视觉验证。

## 验收标准

- 每个可处理项显示方向正确的缩略图、宽高、源体积与状态。
- 动图、损坏、超限文件标记失败并显示原因；可单独移除；不阻止其它项继续。
- 透明图片缩略图在棋盘格上显示。
- 大量文件时缩略图按可见项逐步加载，界面保持可响应。
- 扩展名与内容格式不一致时按内容格式展示并提示，不判失败。
- 动画检测不静默取首帧。
- `make test` / `make lint` 通过；新依赖已登记；macOS 手动验证通过。
- 不在本切片实现处理与输出。

## 完成记录

- 状态：完成（2026-10-23）
- 实现说明：
  - 格式与检查：`internal/imageproc` 新增内容格式探测、`Inspect`、方向解析/应用（JPEG APP1、PNG eXIf、WebP EXIF、TIFF IFD0，覆盖大小端与 8 方向）、轻量 GIF/WebP 动图检测、100 MP/512 MB 上限、96px 最长边 PNG 缩略图（no upscale，保留透明），以及扩展名与内容格式不一致的 warning。动图/损坏/超限统一返回 `apperr.Validation`。
  - 绑定：`service.InspectImage` → `app.InspectImage`；Wails bindings 与 `imageproc.Info` 模型已重新生成。
  - 分层：文件 stat/读取与 512 MB 上限在 `service.InspectImage`；`imageproc.Inspect(data, path)` 只处理字节与纯图像逻辑，保持规格定义的纯函数接缝。
  - 前端：新增 `views/image/QueueRow.vue` 与 `types.ts`；每行用 IntersectionObserver 在进入视口时触发检查，父级用串行队列限制并发，避免大量图片同时解码；展示缩略图、宽高、内容格式、体积、状态、扩展名 warning 与失败原因；透明缩略图使用棋盘格背景；失败项可移除。
  - 构建：引入 `gen2brain/webp` v0.6.4 + `golang.org/x/image` v0.45.0 并登记 `deps.yaml`；Makefile/build.sh/smoke/verify-gen 统一带 `nodynamic`，避免加载系统 libwebp。
- 验收证据：
  - `go test -tags nodynamic ./internal/imageproc ./internal/service -count=1` 通过；imageproc 覆盖 6 种输入格式、8 方向 + 大端、PNG/WebP/TIFF 容器方向、半透明像素、动图检测、透明缩略图、扩展名不一致、损坏/100 MP 边界、中文路径；service 覆盖真实文件读取、缺失文件与 512 MB 稀疏文件上限。
  - `make test`、`make lint`、`make build`、`make smoke`、`make verify-gen` 全部通过；Wails 构建输出 `Tags [nodynamic]`。
  - Playwright 1280×900（真实构建产物 + 假绑定）：4 行队列，3 行“可处理”、1 行“失败”；缩略图/尺寸/格式/体积正确；透明缩略图可见棋盘格；`伪装.jpg` 显示“扩展名与实际格式不一致（.JPG → PNG）”；`banner.webp` 失败原因“首版不支持动图”；移除失败项后 3 行，清空后回到空态。截图：`.scratch/image-workshop/evidence/imageworkshop-inspect-1280.png`；复现脚本：`.scratch/image-workshop/visual/verify-inspect.js`。
  - 视觉检查使用假绑定验证布局；imageproc 测试使用真实编码字节，service 测试使用真实临时文件；实际 app 启动/配置/DB 由 smoke 覆盖。
- 偏离与遗留：
  - 动图通过 `apperr` 失败而不是返回 `Animated` 字段；规格中的 `InspectImage` 契约已同步为错误语义。
  - 缩略图生成会完整解码图片；100 MP 图片较慢，但检查串行且懒加载，内存受单张上限约束。
  - 队列状态仍是页面本地状态，切到“设置”再回来会丢失；规格未要求队列持久化，作为后续审计项。
  - 缩略图统一输出 PNG（保留透明），未做 WebP/JPEG 缩略图优化；96px 下体积可接受。
  - 真实 app 中带真实图片的缩略图最终点验留给 012；本切片通过 Go 测试 + 假绑定视觉 + build/smoke 验收。
  - 未运行 `make dev`；build/smoke/verify-gen 已覆盖 `nodynamic` 标签。
