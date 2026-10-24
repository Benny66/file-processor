# P1–P2 迭代方案 —— 输出、重跑、调参工作台、格式竞速、arm64、隐私文案

- 状态：实施中（2026-10-24：变更一「输出目的地与命名」与变更二「队列内重跑」已交付，其余待排期）
- 依据：`docs/竞品对比与迭代建议.md`（2026-10）P1/P2 条目；代码事实以 2026-10-24 三路代码盘点为准
- 产出阶段：方案 → 逐变更切片实施（每个变更独立成切片，走 git 提交 + 规格修订）
- 范围说明：麒麟**真机验收推迟**（暂无硬件，原 P0-1 挂起）；P0-2（签名公证/身份收口）不在本方案内，仍是正式对外分发的前置

本文新增用户故事与决策**续接** `docs/specs/2026-10-23-image-workshop.md` 的编号（US-39 起、D-34 起），交付时合并回规格并同步修订 §10 列出的既有条款。

---

## 变更一（P1）：输出目的地与命名模板

竞品依据：Caesium / XnConvert / 图压均支持自定义输出目录 + 保留文件夹结构 + 重命名；图片工坊取件环节全面落后（竞品对比报告 §4）。

### 现状（代码事实）

- 输出路径固定源文件同目录：`convertOnce` 中 `dir := filepath.Dir(path)`（`internal/service/batch_service.go:344`），词干 + `-out` 后缀 + 重名 `-out-2` 递增，硬链接原子落位、绝不覆盖（`writeConvertedFile`/`placeOutputFile`，batch_service.go:363-405）。临时文件建在**源目录**。
- `ScannedFile` 只有 `Path/Name/Size`，**不携带相对路径**（`internal/service/image_service.go:15-19`）——保留结构的前提缺口。
- `BatchRequest{Paths []string, Rules BatchRules}`（batch_service.go:88-91）；设置平铺存 `config.json` 四字段（`internal/config/config.go:44-45`）；预设逐列存 SQLite（`internal/model/image_preset.go`）。
- 前端规则区无输出设置区块（`ImageWorkbench.vue:47-140`）；`SelectDirectory` 绑定已存在（app.go:174-176）。
- 既有决策 D-12「输出同目录、-out 后缀」（规格 §4）需修订。

### 设计

**核心类型**（落 `internal/service/batch_service.go`，常量进 parity 镜像）：

```go
// 输出目的地模式（前端镜像 frontend/src/lib/image.ts，parity 新增一行校验）
const (
    OutputModeSource     = "source"      // 源文件同目录（默认，行为同现状）
    OutputModeCustomFlat = "custom_flat" // 自定义目录，平铺
    OutputModeCustomTree = "custom_tree" // 自定义目录，按导入相对路径保留结构
)

type OutputPlan struct {
    Mode      string `json:"mode"`      // 默认 source
    Directory string `json:"directory"` // custom_* 时必填绝对路径
    Pattern   string `json:"pattern"`   // 词干命名模板，默认 "{name}-out"
}

type BatchItem struct {
    Path    string `json:"path"`
    RelPath string `json:"rel_path"` // 以导入根为基准的相对路径（含文件名）
}

type BatchRequest struct {
    Items  []BatchItem `json:"items"`  // 取代原 Paths（绑定重新生成，前端唯一调用方）
    Rules  BatchRules  `json:"rules"`
    Output OutputPlan  `json:"output"`
}

// 上次设置从 BatchRules 扩为整体（Get/SaveImageSettings 签名同步改）
type ImageSettings struct {
    Rules  BatchRules `json:"rules"`
    Output OutputPlan `json:"output"`
}
```

**扫描侧**：`ScannedFile` 增加 `RelPath`。`ScanPaths` 对目录根记录 WalkDir 相对路径；直接选中的文件 `RelPath` = 文件名。前端 `QueueItem`（`types.ts:37-46`）随之携带。

**路径决策**（`convertOnce` 内新增 `resolveOutput(item, plan, ext)`）：

- `source`：目录 = 源目录，行为与现状逐字节一致（默认值不改变任何现有结果）。
- `custom_flat`：目录 = `plan.Directory`。
- `custom_tree`：目录 = `plan.Directory` + `filepath.Dir(item.RelPath)`；启动批次时 `MkdirAll`。
- 命名：`render(pattern)` 替换 `{name}`（原词干）与 `{n}`（队列序号，导入顺序，1 起）；扩展名由输出格式追加，模板不含扩展名。`placeOutputFile` 推广为：首个候选 `渲染词干 + ext`，冲突则 `渲染词干-2 + ext` 递增——防覆盖语义不变。
- **临时文件改建于目标目录**（不再固定源目录）：保证跨卷落位时链接/改名仍在同一文件系统；`os.Link` 失败（非 ErrExist）时回退 `os.Rename`。

**校验**（`validateBatchRules` 扩展 + `ValidateImageRules(rules, output)` 签名加参，文案单一真相在 Go）：

- mode 必须是三常量之一；`custom_*` 时 Directory 必填、须为绝对路径（前端选择器保证，Go 仍校验）；`MkdirAll` 失败报 Validation。
- Pattern 非空（空则用默认）、渲染后非空、**必须包含 `{name}` 或 `{n}`**、不含路径分隔符与 `..`、长度 ≤ 64。
- 预设**不存** OutputPlan（见 D-34）；`config.json` 增加 `image_output_mode / image_output_directory / image_output_pattern` 三字段，`SaveImageSettings` 走新 `ImageSettings`。

**前端**（`ImageWorkbench.vue` 规则区末尾新增「输出」块）：

- 模式 select（三选一）+ 自定义目录行（「选择…」按钮走 `SelectDirectory`，显示路径）+ 模板输入 + 实时示例（用队列第一张或 `photo` 示例渲染 `photo.jpg → photo-out.jpg`）。
- 三个输出字段的 watch 并入现有 `scheduleSettingsSave` 防抖（:414-421）与 `markResultsStale`（:368-370）触发点；`busy` 期间禁用。

### 用户故事（增补）

**US-39 选择输出目的地**：用户可选「源文件同目录 / 自定义目录平铺 / 自定义目录并保留导入时的文件夹结构」；默认值与现有行为完全一致；自定义目录不存在时自动创建，创建失败该批次开始前报错。
**US-40 保留文件夹结构**：以「自定义目录 + 保留结构」输出时，多根导入（不同文件夹拖入）各自保持相对层级；不同根下同名同路径文件靠序号后缀防覆盖。
**US-41 命名模板**：模板支持 `{name}`（原文件词干）与 `{n}`（队列序号）；必含其一；界面实时预览渲染效果；重名防覆盖序号规则与现状一致。
**US-42 输出设置持久化**：输出目的地三项随「上次设置」防抖落盘并重启恢复；预设**不包含**输出目的地（见 D-34）。

### 决策（增补）

| ID | 决策 | 理由 / 后果 |
|---|---|---|
| D-34 | OutputPlan 是「环境粘性设置」而非规则：进 config.json，不进预设 | 输出目录是机器相关的环境事实；进预设会让「应用预设」意外改写落盘位置。PresetDialog `describeRules` 不变 |
| D-35 | 默认 mode=source，行为与 D-12 现状逐字节一致（D-12 修订为「默认同目录，可选自定义」） | 升级零惊讶；已达标跳过/失败不产出等语义不变 |
| D-36 | 模板变量 v1 仅 `{name}` `{n}`，扩展名由输出格式决定、不进模板 | 变量越少越可测；`{ext}` 语义与输出格式冲突 |
| D-37 | 临时文件建于目标目录；Link→Rename 回退 | 跨卷（U 盘/网络盘）硬链接必失败；保持原子落位与「失败不留半成品」 |

### 护栏与测试

- parity 镜像新增一行「图片输出模式」（Go `OutputMode*` ↔ `image.ts`）；无新错误码、无新事件、无新依赖。
- Go 单测矩阵：三模式路径解析（含多根、深层 RelPath）、模板渲染/非法模板拒绝、序号防覆盖（-2/-3）、MkdirAll 失败、custom 模式下已达标跳过仍不产出；`placeOutputFile` 回退路径以注入 link 失败函数覆盖。
- 前端：vue-tsc + 冒烟；设置回填（含旧 config.json 无新字段 → 默认 source）单测在 Go 侧覆盖。

工作量：**M**。

---

## 变更二（P1）：队列内重跑失败/未完成项

竞品依据：批量工具通用兜底预期；现状唯一路径是整批重跑且会清掉全部成功结果（`startBatch` 先清空所有 `item.result`，ImageWorkbench.vue:656-661）。

### 现状

- `ProcessBatch` 无状态：接受任意路径列表即可跑（batch_service.go:149-252）；Go 侧无需任何改动即支持「子集重跑」。
- 规格「明确不做」第 3 条（任务历史、批量重跑、失败重试队列）与 D-19 需 carve-out（见 D-38）。
- QueueRow emits 仅 `inspect/remove/compare`（QueueRow.vue:58-62）；汇总条 `summaryText` 依赖 `BatchResult`（:610-629）。

### 设计

- **批次结束后**（`!busy` 且存在 `failed || canceled` 项）：汇总区出现「重跑失败/未完成项 (n)」按钮；`failed` 与 `canceled` 行尾出现单行「重跑」按钮（`busy` 禁用）。
- 点击后：收集目标项的 `BatchItem`（Path + RelPath，变更一产物）→ 以**当前规则与输出计划**发 `ProcessBatch`；返回结果**只合并回被重跑的项**，成功项的既有结果原样保留；`resultsStale` 若因规则变化已置位则维持置位（旧成功结果仍标「结果已过期」，新结果新鲜）。
- 汇总条改为**从 items 派生**（成功/失败/跳过/取消/极端降级 + 累计节省按项累加），不再只信最后一次 `BatchResult`——合并语义下 items 才是唯一真相。
- 取消按钮、进度事件、互斥锁语义全部复用，不改 Go。

### 用户故事与决策

**US-43 重跑失败与未完成项**：批次结束后可一键重跑全部 `失败` 与 `已取消` 项；成功/跳过项结果不动；重跑仍受单批次互斥与取消语义约束。
**US-44 单行重跑**：失败/取消行可单独重跑；处理进行中禁用。

| ID | 决策 | 理由 / 后果 |
|---|---|---|
| D-38 | 「队列内重跑」是对「明确不做」第 3 条的**范围澄清**而非违约：D-19 继续排除持久化任务历史；重跑仅存活于当前队列会话，清空队列即消失 | 不引入历史数据负担的初衷保留；规格第 3 条改写为「任务历史（持久化）、跨会话失败重试队列」 |
| D-38a | 重跑集合 = 失败 + 已取消 | 取消后最自然的意图是「把剩下的跑完」；按钮文案明示两者，避免歧义（决策点 §9-2） |

### 护栏与测试

- 无新接缝、无新事件、无 Go 改动（依赖变更一的 `Items` 结构）。
- 前端合并逻辑用 vue-tsc 守住类型；Go 侧既有 `ProcessBatch` 测试即覆盖子集路径（补一个「子集请求只返回子集结果」断言）。

工作量：**S**（依赖变更一先行，避免 BatchRequest 两次改形）。

---

## 变更三（P2）：单张调参工作台

竞品依据：Squoosh 滑块对比 + 放大镜 + 实时体积读数是被验证的交互，且官方拒绝做批量（竞品对比报告 B1/A7）；「调好一张 → 存为预设 → 批量」是承接图压遗孤用户的最短路径。

### 现状

- `imageproc.Convert(data, path, opts)` 已导出，**就是**「单图+规则 → 编码字节 + 尺寸 + 质量 + 标志位」的一次性函数（convert.go:92）；目标体积路径 JPEG/WebP 固定 8 次整图编码（q=1 探测 + 7 次二分，convert.go:185-235），单张试算典型 0.3–1 s、上限受 512MB/100MP 保护（inspect.go:24-30）。
- `RenderPreview` 只呈现**源图**重编码（preview.go:30，方向归一、maxEdge≤2048 夹取），与规则无关——无法回答「当前规则下输出长什么样」。
- 对比弹窗是自绘 Teleport 模式 + `useModalFocus` + 棋盘格（CompareDialog.vue），**全库无任何滑块/缩放实现**。
- 异步先例：`RenderPreview` 双图并行 + token 防错序（CompareDialog.vue:92-126）；规则预检 watch+token（ImageWorkbench.vue:292-308）；慢调用 50ms 阈值仅记日志不中断（invoke.ts:96-114）——试算必触发慢日志，属预期噪音。
- `BatchRules` 无 Quality 字段；D-23「目标体积是主控制面、不暴露质量滑杆」。

### 设计

**新绑定**（service + app.go 一行转调，模式同 RenderPreview）：

```go
type PreviewConvertResult struct {
    Status          string `json:"status"`          // success / skipped
    Message         string `json:"message,omitempty"` // skipped 原因（已达标）或失败原因
    OutputBytes     int64  `json:"output_bytes"`
    Format          string `json:"format"`
    Width, Height   int    `json:"width/height"`
    Quality         int    `json:"quality"`           // 达标所用最高质量（PNG 为 0）
    ExtremeDegraded bool   `json:"extreme_degraded"`
    AlphaFlattened  bool   `json:"alpha_flattened"`
    PreviewURL      string `json:"preview_url"`       // 输出像素的高质量预览（非源图重编码）
}

// PreviewConvert：读文件 → 已达标判定（复用 convertOnce 判据）→ imageproc.Convert
// → 对输出字节渲染预览（fit ≤2048，透明 PNG / 其余 JPEG），不落盘
func (s *Service) PreviewConvert(path string, rules BatchRules) (PreviewConvertResult, error)
```

要点：**不落盘**（试算永不写出文件）；`PreviewURL` 是输出像素的预览级重编码，体积数字一律来自 `OutputBytes`（真实编码字节），弹窗内注明。

**前端 `TuneDialog.vue`**（复用 CompareDialog 的 Teleport + useModalFocus 模式）：

- 入口：QueueRow 新增「调参」按钮（有 `info` 且非 `busy` 时可用）。
- 左栏：当前规则的**可编辑副本**（格式 / 目标体积 / 宽高盒，与工作台同控件）；改动 300ms 防抖后 `PreviewConvert`，token 最新胜出；进行中显示骨架；失败显示 Go 返回的中文原因。
- 右栏：滑块对比——源图 `RenderPreview(path, 2048)` 为底层，输出 `PreviewURL` 为上层，`clip-path: inset()` + 可拖分隔线；两图同舞台 `object-fit: contain`（盒缩放导致的尺寸差异正是对比点）。放大镜 = 舞台整体 `scale(1–4x)` 双图同步、`transform-origin` 跟随指针，**预览级缩放**（见 D-40）。
- 结果条：输出体积 + 节省% + 尺寸 + 质量 + 「已明显降质 / 透明铺白」徽标（复用 QueueRow 徽标样式 QueueRow.vue:344-357）。
- 底部动作：「应用到全部」（回写工作台规则 refs → 触发 resultsStale）、「存为预设」（内联名称输入，直接 `SavePreset`，服务端校验/重名/刷新链路原样复用）、「关闭」。

**竞速面板**（本弹窗内，变更四 B2b 的挂载点）：「三格式竞速」按钮并行跑 jpeg/webp/png 三次 `PreviewConvert`，表格列出体积/质量/尺寸，最小项高亮，点击行即采用该格式（详见变更四）。

### 用户故事与决策

**US-45 进入调参工作台**：队列中任一已检查图片可打开工作台；批次处理进行中禁用（避免与 2 worker 争 CPU）。
**US-46 实时试算**：改规则后防抖自动重算，立即看到真实输出体积、尺寸、质量与标志位；已达标时明确说「已达标，不会产出」；试算不写任何文件。
**US-47 滑块对比与放大**：拖动分割线对比源图与当前规则输出；1–4 倍同步缩放查看细节；透明区域棋盘格。
**US-48 应用与沉淀**：调好的规则可「应用到全部」或「存为预设」，两者都不自动开始批次。

| ID | 决策 | 理由 / 后果 |
|---|---|---|
| D-39 | 工作台只调既有三类规则，**不暴露质量滑杆**（D-23 维持）；`Convert` 的 Quality 槽位不进 BatchRules | 目标体积仍是主控制面；质量值作为**结果**展示（达标所用最高质量），不是输入 |
| D-40 | 放大镜是预览级 CSS 缩放（源 2048px），不做服务端像素级裁剪 | 规格 §6「不做像素级对比查看器」边界维持；预览级缩放足够看清块效应/边缘伪影；像素级另立 P3 评估 |
| D-41 | 批次运行中禁用试算 | 2 worker + 试算会过订阅 CPU；取消后自动恢复可用 |

### 护栏与测试

- 新绑定 → bindings 护栏要求 `make build` 重新生成；无新事件（结果走返回值，不触发 wiring13）；无新错误码；无新依赖。
- Go 单测：小图金样（体积>0、格式、dataURL 前缀、方向归一生效）、已达标路径返回 skipped、动图/损坏图返回可读错误、目标过小失败文案。
- 前端：vue-tsc；冒烟脚本不涉及。

工作量：**L**（弹窗交互占大头，Go 侧薄）。

---

## 变更四（P2）：多格式竞速与「自动（最小体积）」

竞品依据：TinyPNG「下载最佳结果」验证的心智；目标体积场景天然适配（同体积比质量，同质量比体积）。

### 现状

- 输出格式三处硬编码联动：Go `ValidateOutputFormat/NormalizeOutputFormat`（convert.go:63-89）、前端 `formatOptions`（ImageWorkbench.vue:320-325）、`OutputFormat` 镜像（image.ts，parity 强制）。
- 候选格式 jpeg/png/webp 的单次编码成本已知（变更三）；批量路径 `convertOnce` 一次只跑一个格式。

### 设计（两个独立切片）

**B2a 批量「自动（最小体积）」输出格式**（先行，纯 Go + 一个下拉选项）：

- `OutputFormat` 增加常量 `OutputAuto = "auto"`（Go + image.ts 镜像 + formatOptions 加「自动（最小体积）」，parity 现有行自动覆盖）。
- `convertOnce` 的 auto 分支：候选 = jpeg、webp、png；逐个 `imageproc.Convert`（透明→JPEG 铺白仍允许并打标）；有目标体积时取**满足目标的最小体积者**，全部不达标则该项失败（语义与单格式一致）；无目标体积时取最小体积者。`ItemResult.Message` 注明「自动选择 WebP（最优）」——复用现有 message 展示管道，不加字段。
- 已达标跳过（auto 语义）：`targetOK && boxOK` 即跳过、Message「已达标」（不再要求格式一致——auto 本无格式偏好，见 D-42）。
- 成本如实标注：auto 每项最多 3× 编码时间（有目标体积时最多 ~24 次整图编码）；UI 选项旁给一句「较慢」提示。

**B2b 工作台三格式竞速**（依赖变更三）：

- TuneDialog 内并行三次 `PreviewConvert`（仅换格式），表格列：格式 / 体积 / 质量 / 尺寸，最小体积行高亮 + 「采用」；点击回写弹窗内规则副本。失败格式行显示原因（如目标过小）。

### 用户故事与决策

**US-49 自动最小格式**：输出格式可选「自动（最小体积）」；逐项独立选优并在行内注明所选格式；已达标项照旧跳过；所有格式都无法达标时该张失败且不产出。
**US-50 工作台竞速**：调参时一键比较三种输出格式的真实体积，点选采用。

| ID | 决策 | 理由 / 后果 |
|---|---|---|
| D-42 | auto 的已达标判定 = 目标体积与尺寸盒均满足（忽略格式） | auto 用户的意图是「尽量小」，源文件已满足时产出反而更大；「保持原格式」用户不受影响 |
| D-43 | 候选集固定 jpeg/png/webp，不因透明自动剔除 JPEG（铺白仍可用并提示） | 保持「几乎总能达标」哲学；透明图用户看到铺白标记后可自行排除 |

### 护栏与测试

- parity：`OutputFormat` 镜像行需同步加 `auto`（双向护栏强制）。
- Go 单测：auto 取最小、全不达标失败、透明→jpeg 铺白参与竞速、已达标跳过、`ValidateOutputFormat` 接受 auto。
- 工作台竞速复用 PreviewConvert 测试。

工作量：B2a **M**；B2b **S**。

---

## 变更五（P2）：ARM64 Linux 构建腿（飞腾/鲲鹏）

竞品依据：全部竞品无官方 ARM Linux 构建；图片处理纯 Go（nodynamic）使交叉成本远低于对手（竞品对比报告 B3）。

### 现状（硬编码清单）

- `build-linux-container.sh:57-67,75-82`：aarch64 容器分支专门为**交叉产 amd64** 而设（clang `--target=x86_64-linux-gnu` + lld + `:amd64` 库）；构建命令 `-platform linux/amd64`；产物断言只认 x86-64 ELF。
- `package-deb.sh:59,85`：`Architecture: amd64` 与 `_amd64.deb` 硬编码；:31-42 二进制校验只认 x86-64。
- `build/linux/Dockerfile` 已双架构感知：arm64 容器有原生路径（ports 源 + 按 arch 装 Go 工具链），:19-40 的 `--add-architecture` + `:amd64` 库 + clang/lld 交叉模式**正好可以镜像复用**到「amd64 容器交叉 arm64」。
- CI `linux-deb` job 在 `container: ubuntu:20.04` 内**原生**构建（build.yml:47-94）；容器 job 内不能再起 docker。ADR-0002 记录过 QEMU 段错误，明确不走模拟。
- 护栏：`wails build` 只许出现在 `scripts/build.sh`（wiring_test.go:164-236）；脚本禁 python3。规格 `ci-pipeline`（环境耦合平台不进矩阵）与 `package-release` 需增补。

### 设计

- `build-linux-container.sh` 增 `arch` 参数（`amd64` 默认 / `arm64`）：arm64 时容器走「amd64 原生 + 交叉 arm64」——Dockerfile 加 `TARGET_ARCH` build-arg，x86_64 分支镜像现有 aarch64 分支写法：`dpkg --add-architecture arm64` + ports 源（`deb [arch=arm64] …/ubuntu-ports focal main`）+ `:arm64` 后缀的 gtk/webkit 开发库 + `clang --target=aarch64-linux-gnu` + `CGO_ENABLED=1 GOOS=linux GOARCH=arm64` + arm64 `PKG_CONFIG_LIBDIR`；构建命令 `build.sh -platform linux/arm64 -tags nodynamic -s`（版本注入唯一入口不变）。产物断言改为按 arch 匹配 `x86-64` / `aarch64` ELF。
- webkit 版本 pin：优先沿用 `=2.28.1-1`；**实施时核实** focal arm64 ports 是否有该精确版本，无则取 arm64 可得的 2.28.x 并在 Dockerfile 注明（`.deb` 的 Depends 本就是 `>= 2.28.1`，运行端不受 pin 影响）。
- `package-deb.sh` 增 arch 参数：`Architecture: arm64`、产物名 `_arm64.deb`、ELF 断言分支。`package.sh` 与 Makefile 透传：`make linux-deb arch=arm64`。
- CI 新 job `linux-deb-arm64`（插在 build.yml:94/95 之间）：**不用 container 指令**，跑在 ubuntu-latest VM 上（docker 可用）→ setup-node 构建前端 → `bash scripts/build-linux-container.sh arm64` → `bash scripts/package-deb.sh arm64` → upload-artifact `linux-deb-arm64`。job 内断言 deb 元数据 `Architecture: arm64`。
- **不接 release.yml**（D-45）：tag 发布仍只收 macOS/Windows + amd64 deb 排期状态不变，arm64 与 amd64 的 Release 接入一起等真机验收后统一做。
- `appinfo.Platform()` 动态取 `runtime.GOARCH`（appinfo.go:70-74），arm64 产物自动显示 `linux/arm64`，零改动。

### 用户故事与决策

**US-51 arm64 构建产物**：CI 每次构建产出 `ImageWorkshop_<版本>_arm64.deb`（artifact）；README 平台表注明「飞腾/鲲鹏（未经真机验收）」；`make linux-deb arch=arm64` 本地（需 Docker）等价可用。

| ID | 决策 | 理由 / 后果 |
|---|---|---|
| D-44 | arm64 用「amd64 容器 + clang/lld 交叉」，不用 QEMU 模拟 | 镜像 ADR-0002 已验证的同族路线（该 ADR 验证过的是反向：arm64 容器交叉 amd64）；QEMU 有段错误前科 |
| D-45 | arm64 .deb 仅 CI artifact，不进 Release、不宣称可用 | 与「真机验收推迟」一致；发布主张必须先在真机成立（新 ADR-0006 记录路线与边界） |

### 护栏与测试

- 现有两条脚本护栏自动覆盖（build 唯一入口、禁 python3）；无新依赖（clang/lld 进 Docker 镜像，不进 deps.yaml——其登记制针对 go.mod/package.json 直接依赖）。
- CI 断言：ELF arch、deb `Architecture` 字段、desktop entry 与图标存在（package-deb.sh 既有逻辑复用）。
- 文档同步：README 平台表 + 已知限制、`ci-pipeline`/`package-release` 规格、新 ADR-0006、`docs/map.md` scripts 表补一行。

工作量：**S–M**（脚本参数化为主，风险在 arm64 ports 依赖核实）。

---

## 变更六（P2）：首屏隐私主张文案

竞品依据：Squoosh 首屏 "Images never leave your device" 的零成本高信任表达（B4）；竞品对比报告已标注本产品「README 有、应用内未见」。

### 现状

- 页头 desc 已有一句「批量转换格式、压缩体积、限制尺寸。图片只在本机处理。」（ImageWorkbench.vue:6）；**空态拖拽区只有支持格式提示，无隐私句**（:174-178）。
- README 首句与 `.deb` control 描述已有完整隐私句；README 未引用空态截图（改文案**不需要**重截图）。
- Settings 关于区有版本/平台/路径行，无隐私行（Settings.vue:95-116）。

### 设计

- 空态拖拽区在支持格式行下加一行（`--text-3`）：**「所有处理在本机完成：不上传、不联网、不采集遥测」**——与 README 首句同口径（D-46）。
- Settings 关于区加「隐私」行：label「隐私」+ muted 文案「运行时不发起任何网络请求；日志与崩溃信息仅写入本机数据目录」。
- 样式全部用既有语义 token（`--text-3` / `--border-subtle`），不新增 token。

**US-52 首屏隐私声明**：空队列状态与设置页可见与本机处理承诺一致的隐私说明；文案与 README 单一口径。

| ID | 决策 | 理由 / 后果 |
|---|---|---|
| D-46 | 应用内隐私文案逐字对齐 README 首句口径，改动需三处（README/空态/设置页）同步 | 防止口径漂移；文案是人写的，不设护栏，code review 守护 |

工作量：**S**（小时级）。

---

## 7. 实施顺序与依赖

```
变更六 隐私文案 (S) ────────────── 独立，可立即做
变更五 arm64 (S–M) ─────────────── 独立，可与下面并行
变更一 输出目的地与命名 (M) ──┐
                              ├─ 变更二 队列内重跑 (S)
变更四 B2a 自动最小格式 (M) ──┘（与变更一无依赖，可穿插）
变更三 调参工作台 (L) ──────── 变更四 B2b 工作台竞速 (S)
```

推荐节奏：六 → 五 → 一 → 二 → 四a → 三 → 四b。变更一先行是为了 BatchRequest 一次改形（`Items` 取代 `Paths`），避免变更二跟着返工。

## 8. 跨变更护栏影响汇总

| 护栏 | 影响 |
|---|---|
| parity_test | 新增镜像行「图片输出模式」（变更一）；`OutputFormat` 增 `auto`（变更四，现有行自动覆盖） |
| bindings_test | 变更三新绑定 `PreviewConvert`、变更一改 `ProcessBatch`/`Get/SaveImageSettings`/`ValidateImageRules` 签名 → 都需 `make build` 重新生成 wailsjs |
| wiring_test | **无新事件**（试算走返回值，不发 image:* 事件）→ 护栏 13 不触发；脚本护栏对变更五自动生效 |
| deps_test | 无新依赖（全部标准库/既有库；clang/lld 在 Docker 镜像层） |
| ESLint/vue-tsc | 新组件沿用 token 体系，无字面色值 |

## 9. 决策点汇总（待确认，默认按推荐执行）

| # | 问题 | 推荐 |
|---|---|---|
| 1 | 输出目的地是否进预设？ | **不进**（粘性设置，进 config.json）——预设换机器不携带路径 |
| 2 | 重跑集合是否包含「已取消」？ | **包含**，按钮文案「重跑失败/未完成项」明示 |
| 3 | 自定义目录不存在时？ | **自动创建**（MkdirAll），失败在批次前报错 |
| 4 | 模板变量集？ | v1 仅 `{name}` `{n}`，必含其一 |
| 5 | auto 格式的已达标语义？ | 目标体积+尺寸盒满足即跳过（忽略格式） |
| 6 | 放大镜实现？ | 预览级 CSS 缩放（2048 源），像素级裁剪不做 |
| 7 | 批次运行中能否试算？ | 禁用，取消后恢复 |
| 8 | arm64 webkit pin？ | 实施时核实 focal arm64 ports 版本，无 2.28.1-1 则取最近 2.28.x 并注明 |

## 10. 既有规格修订清单（随各切片交付时执行）

| 条款 | 修订 |
|---|---|
| 规格 §4 D-12、US-16 | 「输出同目录」改为「默认同目录，可选自定义目录/结构保留/命名模板」（变更一） |
| 规格 §6「明确不做」第 3 条、D-19 | 排除范围收窄为「持久化任务历史、跨会话重试队列」；队列内重跑放行（变更二） |
| 规格 §6「完整缩放/拖拽/滑动对比查看器」、D-18 | 边界改写：调参工作台提供预览级滑块对比与缩放；像素级仍不做（变更三） |
| 规格 §2 绑定接缝表 | `ProcessBatch` 入参改形、新增 `PreviewConvert`、`Get/SaveImageSettings` 签名（变更一/三） |
| `openspec/specs/ci-pipeline`、`package-release` | 增补 arm64 job 与产物表述（变更五） |
| README | 平台表（arm64 deb、注明未真机验收）、已知限制两条（arm64 未验收、真机验收推迟说明） |
