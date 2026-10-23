# 010 — 预设与上次设置

**Blocked by**: 008

**规格**: US-12、US-13；D-19

## 目标

用户可以把当前参数保存为命名预设，支持创建、应用、重命名、删除并跨重启持久化；应用重启后恢复上次使用的参数。

## 涉及层

- [x] 数据层：新增 `ImagePreset` 模型（`BaseModel`、唯一名称、输出格式、目标体积、最大宽/高）并登记 `AllModels()`；SQLite 迁移；`config.json` 增加上次处理参数字段。
- [x] 逻辑层：service 提供预设 CRUD、名称去空白与唯一性校验、参数校验；`ListPresets` 返回 `page.Result`；`SaveImageSettings` / `GetImageSettings` 读写配置；绑定层错误走 `apperr`。
- [x] UI 层：预设下拉与管理对话框；保存当前参数为预设；应用预设只改参数、不自动开始；重命名/删除；校验错误展示；启动时恢复上次参数；配置损坏时回退默认值且不阻断启动。
- [x] 测试：真实 SQLite 的 CRUD（不用 mock）；名称唯一；上次设置往返；损坏配置回退；前端手动验证。

## 验收标准

- 预设可创建、应用、重命名、删除，跨重启保留；名称去空白后不可重复。
- 应用预设只更新参数，不自动开始处理；已有结果标记过期。
- 应用重启恢复上次使用的参数；配置缺失或损坏时回退默认值，不阻断启动。
- `ListPresets` 使用分页契约；新模型已登记；依赖无需新增。
- 不实现任务历史、批量重跑或失败重试队列。
- `make test` / `make lint` 通过；macOS 手动验证通过。

## 完成记录

- 状态：完成（2026-10-23）
- 实现说明：
  - 数据层：新增 `model.ImagePreset`（BaseModel + 唯一名称 + 格式/目标/宽高）并登记 `AllModels()`，启动时由既有 AutoMigrate 建表；`config.Config` 增加 `image_output_format/_target_bytes/_max_width/_max_height`。
  - 配置回退：`config.Load` 在文件不可读或 JSON 损坏时返回可用默认配置并附带错误，同时把损坏文件改写为默认值；`main.go` 记录警告后继续启动，不再 Fatal。旧配置缺失图片字段时按“保持原格式 / 全部关闭”解释。
  - 服务：抽出 `validateBatchRules` 供批次、预设与上次设置共用；持久化层额外要求目标体积为整数 KB/MB，保证能原样回填输入框。预设支持分页列表、创建/更新（`SavePreset`，ID=0 新建）、去空白后唯一校验、删除；删除使用硬删除，避免软删除行永久占用唯一名称。设置读写复用 `BatchRules`，无效值不覆盖旧设置。
  - 绑定：新增 `ListPresets` / `SavePreset` / `DeletePreset` / `GetImageSettings` / `SaveImageSettings`；Wails bindings 与 models 已重新生成。
  - UI：工作台新增预设下拉与“管理预设”对话框（保存当前参数、应用、行内重命名、删除前原生确认、重复/校验错误展示）；应用预设只更新参数并在对话框关闭后回到工作台，已有结果经既有 watcher 标记过期；启动时恢复上次参数，参数变化以 300ms 防抖写回配置，应用预设/开始批次/离开页面时立即冲刷；对话框共用新抽出的 `useModalFocus`（Esc、背景 inert、焦点归还）。
- 验收证据：
  - `go test -tags nodynamic ./internal/config ./internal/service ./internal/guard -count=1` 通过：损坏配置回退默认值并可重新加载、图片设置字段 snake_case 落盘；预设 CRUD、去空白重名拒绝、重命名后旧名释放、分页、重开 SQLite 后保留、7 类非法参数、not_found、设置往返（重新 Load 配置）与非法设置不覆盖旧值。
  - `make test`、`make lint`、`make build` 通过；`AllModels()` 护栏与绑定方法护栏通过。
  - Playwright 1280×900（真实构建产物 + 假绑定）：启动恢复 WebP / 目标 300 KB / 盒 1600×1200；选择“电商主图”后变为 JPEG / 100 KB / 无盒，未出现进度、未开始处理；处理完成后再应用“高清存档”出现过期提示与 2 个“结果已过期”徽标；管理对话框展示 2 个预设及规则摘要，保存“ 新预设 ”自动去空白，重复名提示“预设名称已存在”，重命名为“新预设 2”后删除（Confirm 1 次）；从对话框应用预设后关闭且未新建批次；手动改 WebP 后重载页面，恢复 WebP / 100 KB / 无盒且 2 个预设仍在。
  - 截图：`.scratch/image-workshop/evidence/imageworkshop-presets-restored-1280.png`、`imageworkshop-preset-dialog-1280.png`、`imageworkshop-preset-stale-1280.png`；复现脚本：`.scratch/image-workshop/visual/verify-presets.js`。对比回归脚本 `verify-compare.js` 在 `useModalFocus` 抽取后复跑通过。
  - 实际 app 的 SQLite 持久化与真实照片视觉点验留给 012；服务端用真实文件库测试覆盖跨连接重开。
- 偏离与遗留：
  - `SavePreset` 同时承担新建与更新；重命名由 UI 带完整预设字段提交，语义仍是仅名称变化。
  - 持久化目标体积限定整数 KB/MB（UI 只有这两档）；批次引擎本身仍接受任意字节目标。
  - 预设删除为硬删除（理由见服务注释）；未实现撤销。
  - 下拉在应用预设后保留选中项，之后手动改参数不会自动清空/标记“已修改”；不提供当前预设脏状态。
  - 管理对话框一次加载最多 200 条（分页上限），未做列表分页控件；后端已支持分页。
  - 设置自动保存失败只写日志、不阻断操作；配置写入沿用既有非串行模式（主题/窗口/图片设置并发保存概率极低）。
  - 队列仍是页面本地状态；真实 app 重启恢复的最终点验与 001 的 Kylin 验证一起留给后续冒烟。
  - 001 Kylin 兼容性 spike 仍待真机诊断输出，011/012 仍被阻塞。
