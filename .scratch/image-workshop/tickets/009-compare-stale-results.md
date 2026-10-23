# 009 — 单张对比与结果过期

**Blocked by**: 008

**规格**: US-28、US-29、US-30；D-18、D-28

## 目标

用户选中成功项后可以查看原图与结果图的并排或切换对比；预览方向正确、透明可见；参数或队列变化后结果被标记为过期。

## 涉及层

- [x] 数据层：无数据库。
- [x] 逻辑层：service 提供 `RenderPreview(path, maxEdge)`，返回 base64、宽高、是否含透明；方向归一；限制最大预览边长；错误走 `apperr`；不为任意路径开放 HTTP 静态服务。
- [x] UI 层：对比对话框/并排或切换；两侧尺寸与体积；加载与错误状态；透明棋盘格；参数或队列变化时显示“结果已过期”；开始新批次清除旧状态；不做缩放/拖拽/像素级检查。
- [x] 测试：预览尺寸与方向；透明预览；绑定错误；结果过期状态；前端手动视觉验证。

## 验收标准

- 选中成功项后可打开对比，原图与结果图均可见，并显示两侧尺寸与体积。
- 预览按方向归一后的像素显示；透明图用棋盘格背景。
- 预览经绑定返回 base64，不通过本地 HTTP 暴露文件路径。
- 参数或队列变化后，已有结果标记“结果已过期”；开始新批次后清除旧状态。
- 首版不做缩放、拖拽、滑动对比与像素级检查。
- `make test` / `make lint` 通过；macOS 手动视觉验证通过。

## 完成记录

- 状态：完成（2026-10-23）
- 实现说明：
  - 引擎：`imageproc.RenderPreview` 复用 `prepareImage` 做解码、超限/动图检查与 EXIF 方向归一，用 `FitWithin` 等比缩到最长边内（默认 1024，夹取 64–2048，不放大）；含 alpha 输出 PNG `BestSpeed`，否则输出 JPEG q85；返回 PNG/JPEG base64 data URL、预览宽高与 `has_alpha`，不开放文件路径。
  - 服务与绑定：service 抽出 `readImageFile` 统一路径校验、512 MB 上限与读取，`InspectImage` 与新增 `RenderPreview` 共用；`app.go` 暴露 `RenderPreview(path, maxEdge)`，Wails 绑定与 models 已重新生成。前端传 `maxEdge=0` 使用 Go 侧默认值，避免镜像预览上限。
  - UI：成功项出现“对比”按钮，打开并排对比对话框（`CompareDialog.vue`），两侧显示原图/结果预览、方向归一后的尺寸与体积，含加载/错误状态；透明区域用共享 `.checkerboard` 背景；支持遮罩点击、Esc、关闭按钮退出；打开时对 `#app` 加 `inert` 并聚焦对话框，关闭后恢复焦点。
  - 过期：参数（格式、盒、目标体积/单位）或队列（导入/移除）变化后，结果行显示“结果已过期”徽标并出现顶部提示；开始新批次、批次完成或清空队列时清除过期与旧结果。
- 验收证据：
  - `go test -tags nodynamic ./internal/imageproc ./internal/service -count=1` 通过：不透明预览 100×75 且输出 JPEG、透明预览保留 alpha 且输出 PNG、方向归一（4×3 旋转为 3×4）、动图拒绝、maxEdge 0/1/5000 分别夹取到 1024/64/2048、service 读取真实文件与缺失文件 not_found。
  - `make test`、`make lint`、`make build` 全部通过。
  - Playwright 1280×900（真实构建产物 + 假 `RenderPreview` 绑定）：2 个成功项均出现“对比”；产品图对话框显示“原图 800 × 600 · 96 KB / 结果 800 × 600 · 49 KB”，两张预览均为 data URL，两侧棋盘格生效；打开时 `#app` 为 inert、焦点在对话框，Esc 关闭后 inert 移除且焦点回到“对比”按钮；把目标体积从 100 改为 200 后出现过期提示与 2 个“结果已过期”徽标，重新处理后提示与徽标消失。截图：`.scratch/image-workshop/evidence/imageworkshop-compare-1280.png` 与 `imageworkshop-stale-1280.png`；复现脚本：`.scratch/image-workshop/visual/verify-compare.js`。
  - 页面视觉使用真实前端构建产物 + 假绑定；预览像素与方向由 Go 真实图片测试覆盖；实际 app 中带真实照片的对比视觉点验留给 012。
- 偏离与遗留：
  - 首版按规格只做并排对比，不含缩放、拖拽、滑动对比与像素级检查。
  - 过期标记为粘性：参数改回原值也保持过期，直到重新处理；批次汇总文本仍保留，靠过期提示与逐行徽标说明其已过期。
  - 对比对话框未提供独立文件重新读取按钮；结果文件被外部删除时在结果侧显示错误，原图侧仍可查看。
  - 队列仍是页面本地状态，路由切换会丢失；作为后续审计项。
  - 未运行 `make dev` / `make smoke`；`make build` 已覆盖真实 Wails 编译与绑定生成。
