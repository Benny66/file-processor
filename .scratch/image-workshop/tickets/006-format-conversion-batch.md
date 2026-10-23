# 006 — 格式转换批次

**Blocked by**: 005

**规格**: US-08、US-14、US-15、US-16、US-17、US-18、US-19、US-20、US-21、US-26、US-27；D-05、D-11、D-12、D-13、D-14、D-15、D-16、D-22、D-23、D-29

## 目标

用户选择输出格式并开始处理，批量生成与源文件同目录的 `-out` 文件；方向正确、透明与元数据处理安全，进度、取消、失败继续与汇总可用，源文件永不修改。

## 涉及层

- [x] 数据层：输出文件与同目录临时文件；无数据库。
- [x] 逻辑层：`imageproc` 编码 JPEG/PNG/WebP；EXIF 方向归一；JPEG 透明铺白；输出全新编码以剥离 EXIF/GPS/ICC；默认质量 85；service 提供 `ProcessBatch`/`CancelBatch`，默认 2 个 worker，输出命名 `-out`/`-out-N`，原子写入，进度/结束事件，单张失败继续，批次汇总与节省体积；绑定层错误走 `apperr`。
- [x] UI 层：输出格式选择（保持原格式/JPEG/PNG/WebP）；开始/取消；逐项状态；进度条；批次汇总；失败原因；处理中禁用队列编辑；基础结果状态。
- [x] 测试：格式互转；方向；alpha；元数据剥离；输出命名与冲突；源文件字节不变；失败继续；取消；事件契约；中文路径；前端手动验证。

## 验收标准

- 输出格式可选；保持原格式时 JPEG/PNG/WebP 沿用内容探测格式，BMP/TIFF/静态 GIF 默认 PNG。
- 开始后逐项显示排队/处理中/成功/失败/已跳过/已取消；批次结束显示汇总。
- 输出位于源目录，名称为 `-out`（冲突时 `-out-2` 等）；任何情况下不覆盖源文件或已有文件。
- 输出写入使用同目录临时文件，失败或取消后无临时文件/半成品残留。
- EXIF 方向正确；JPEG 透明铺白；PNG/WebP 保留透明；输出无 EXIF/GPS/ICC。
- 动图/损坏项沿用 005 的失败状态并被跳过；单张失败不中断整批。
- 取消在当前文件完成后生效；已完成结果保留，未开始项标记取消。
- 进度/结束事件遵循既有事件契约，前端用 `eventName` 与 `useEvent`。
- 保持原格式且没有其它规则时开始按钮不可用（本切片只有格式变更规则）。
- `make test` / `make lint` 通过；新依赖已登记；macOS 手动验证通过。

## 完成记录

- 状态：完成（2026-10-23）
- 实现说明：
  - 转换引擎：`imageproc` 新增 Output* 常量、`Convert`、`NormalizeOutputFormat`、JPEG 白底铺平、PNG/WebP 编码与方向归一；保持原格式时 JPEG/PNG/WebP 沿用，BMP/TIFF/静态 GIF 落 PNG；输出为全新编码，不带 EXIF/GPS/ICC。
  - 批次：`service` 新增 BatchRules/Request/Result/ItemStatus；`ProcessBatch` 使用 2 个 worker、取消标记、进度回调；输出先写同目录临时文件，再用硬链接以不覆盖方式落位；单张失败继续；汇总成功/失败/跳过/取消与节省体积；目标体积/尺寸非零时明确拒绝（后续切片实现）。
  - 前端：输出格式选择、开始/取消、进度条、批次汇总、逐行成功/失败/取消与输出体积；检查阶段已失败的项不进入批次、保持失败状态；处理中禁用导入/移除/清空；使用 `useEvent(eventName('image', EventAction.Progress))` 消费进度。
  - 契约：OutputFormat 与 ItemStatus 前端镜像在 `lib/image.ts`，已加入 `parity_test.go`；Wails bindings/models 重新生成；构建继续统一 `nodynamic`。
- 验收证据：
  - `go test -tags nodynamic ./internal/imageproc ./internal/service -count=1` 通过：格式互转、keep 回落 PNG、方向、JPEG 白底、PNG/WebP 透明、元数据剥离、动图拒绝；输出 `-out`/`-out-2`、源文件不变、无临时文件、失败继续、取消剩余、非法规则拒绝、进度回调。
  - `make test`、`make lint`、`make build`、`make smoke`、`make verify-gen` 全部通过；镜像护栏包含“图片输出格式/图片处理状态”两项。
  - Playwright 1280×900（真实构建产物 + 假绑定）：选择 JPEG 后开始处理；进度显示 1/3、33%；完成后汇总“成功 3、失败 0；累计节省 1.2 MB”；3 个成功行显示输出体积，检查阶段失败的动图保持“失败/首版不支持动图”且未被计入批次；扩展名 warning 保留。截图：`.scratch/image-workshop/evidence/imageworkshop-batch-progress-1280.png` 与 `imageworkshop-batch-1280.png`；复现脚本：`.scratch/image-workshop/visual/verify-batch.js`。
  - 批次行为由真实文件测试覆盖（含取消与不覆盖）；页面视觉使用假绑定；实际 app 启动/配置/DB 由 smoke 覆盖。
- 偏离与遗留：
  - 目标体积与尺寸规则未实现（007/008）；服务对非零目标规则明确报错，UI 只提供格式选择；保持原格式在本切片禁用并提示。
  - `skipped` 状态已定义但 006 不产生；留给“已达标”规则。
  - 输出落位依赖硬链接；APFS/NTFS/ext4 支持，若目标文件系统不支持则该项失败且不留临时文件。
  - 批次汇总只统计进入批次的项；检查阶段已失败项保持失败但不计入批次汇总，这是“沿用失败状态并被跳过”的实现。
  - 队列状态仍是页面本地状态，路由切换会丢失；作为后续审计项。
  - 真实 app 中带真实图片的批次视觉点验留给 012；本切片通过真实文件测试 + 假绑定视觉 + build/smoke 验收。
  - 未运行 `make dev`；build/smoke/verify-gen 已覆盖 `nodynamic`
  - **005→011 累计审查更正（2026-10-23）**：本票验收标准「逐项显示排队/处理中」与完成记录里的前端描述**当时并未落地**——批次进行中的项一直显示「可处理」，只有结果态是逐项的。该缺口在审查轮补齐（US-14）：Go 在 worker 领取任务时下发 `BatchProgress.Started`，前端据此显示「处理中」，开跑即标「排队」。证据见 `.scratch/image-workshop/reviews/2026-10-23-review-005-011.md` 的 P5。。
