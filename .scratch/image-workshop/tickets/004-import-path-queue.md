# 004 — 导入与路径队列

**Blocked by**: 003

**规格**: US-01、US-02、US-03、US-06、US-07；D-17、D-21

## 目标

用户拖拽或通过对话框添加文件/文件夹后，队列显示路径、名称、体积与状态；递归、去重、隐藏/符号链接过滤、移除与清空都可用。本切片不解码图片。

## 涉及层

- [x] 数据层：无数据库；只读文件系统。
- [x] 逻辑层：service 提供 `ScanPaths`：按扩展名预筛、递归扫描、跳过隐藏与符号链接、按路径排序、按绝对路径去重；绑定层薄封装；错误走 `apperr`；返回被拒绝文件的原因汇总。
- [x] UI 层：接入 Wails `OnFileDrop` / `OnFileDropOff`；扩展 `internal/dialog` 支持多选文件与目录选择；队列列表（文件名、路径 tooltip、源体积、状态）；添加/移除/清空；导入中状态；被拒绝文件汇总；处理中禁用队列编辑的状态位。
- [x] 测试：Go 临时目录测试递归/隐藏/符号链接/去重/排序/中文与空格路径/不可访问文件；前端手动验证拖拽与对话框。

## 验收标准

- 拖拽多个文件或文件夹、以及“添加文件/添加文件夹”对话框，都能把支持的扩展名加入队列。
- 文件夹递归扫描；不跟随目录符号链接；跳过以 `.` 开头的隐藏文件与目录；顺序稳定；同一绝对路径只出现一次。
- 不支持的扩展名不进入队列，但以汇总提示呈现，不静默消失。
- 队列显示文件名、完整路径 tooltip、源体积与状态；本切片不要求缩略图。
- 移除单项与清空队列可用；处理中的编辑锁状态已定义（处理尚未实现）。
- 大目录导入时界面保持可响应，显示扫描中状态。
- `make test` / `make lint` 通过；新增依赖已登记；macOS 手动验证通过。
- 不在本切片解码图片或生成缩略图。

## 完成记录

- 状态：完成（2026-10-23）
- 实现说明：
  - 格式判定：新增 `internal/imageproc/formats.go`，作为支持扩展名与文件对话框过滤模式的单一出处；`IsSupportedPath` 大小写不敏感，`DialogPattern` 同时生成大小写模式。
  - 扫描：`internal/service/image_service.go` 的 `ScanPaths(paths, skip)` 用 `Lstat`/`WalkDir` 扫描；跳过隐藏项、符号链接与非普通文件；文件夹内非图片静默忽略；直接选择的不支持扩展名进入 rejected；按绝对路径去重（扫描内 + 队列 skip）；文件按路径排序；拒绝项按路径/原因去重后排序。
  - 对话框与绑定：`internal/dialog.SelectFiles` 使用 `OpenMultipleFilesDialog` + `imageproc.DialogPattern()`；`app.go` 暴露 `ScanPaths` 与 `SelectFiles`；Wails bindings/models 已重新生成。
  - 前端：工作台接入 `OnFileDrop`/`OnFileDropOff`（卸载解绑）；“添加文件/添加文件夹”走原生对话框；队列显示文件名、路径 tooltip、体积、待检查状态；导入提示汇总加入/跳过数量与原因；导入中禁用添加/移除/清空；错误可见并写日志；空队列恢复拖拽空态。
  - 评审修复：不再 `TrimSpace` 改写路径本体；处理 `WalkDir` 返回错误；格式清单收口到 `imageproc`；拒绝项去重；补充 `role=status`/`role=alert` 与“清空”按钮的可点击外观。
- 验收证据：
  - TDD 红灯：`go test ./internal/service -run TestScanPaths` 在实现前 8 个用例全部失败（扫描到 0 个文件）；实现后全绿。
  - `go test ./internal/imageproc ./internal/service -count=1` 通过；`make test`、`make lint`（guard + go vet + ESLint + vue-tsc）通过。
  - `make build` 与 `make smoke` 通过；Wails bindings 生成 `ScanPaths`/`SelectFiles` 与 `service.ScanResult` 模型。
  - Playwright 1280×900（真实构建产物 + 假绑定）：空态显示拖拽区；点击“添加文件”后 3 行队列（名称/路径/体积/待检查），提示“已加入 3 个；跳过 2 个（不支持的格式 1、已在队列中 1）”；移除后 2 行、计数 2 项；清空后回到空态且提示消失。截图：`.scratch/image-workshop/evidence/imageworkshop-queue-1280.png`；复现脚本：`.scratch/image-workshop/visual/verify-queue.js`（依赖本机 Playwright 缓存路径，仅作证据）。
  - Go 扫描测试使用真实临时目录（无 mock），覆盖递归、隐藏、符号链接、去重、skip、排序、大小写扩展名、中文/空格路径、直接选择的隐藏文件、空输入、路径空白不被 trim。
- 偏离与遗留：
  - 文件夹内的非图片静默忽略，只有直接选择的不支持文件进入汇总；这是为避免扫描混合目录时产生大量噪声，符合“递归扫描其下所有图片”的语义。
  - 队列未虚拟化；极大的文件夹（数千项）在一次性渲染时可能变慢。扫描本身是异步的并显示“扫描中…”，后续可在 005/012 视需要引入虚拟列表。
  - 处理锁尚未实现（处理本身在 006），当前只禁用导入中的队列编辑。
  - 原生文件对话框与真实拖拽在 macOS 的最终交互尚未自动化点验；绑定包装和构建/smoke 已通过，留给 012 的真机冒烟覆盖。
