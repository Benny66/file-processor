# 普通用户化改造方案 —— 人话文案、暂存+手动下载、历史记录

- 状态：已实施（2026-10-24：三变更均已交付——文案口语化 f3974ac / 暂存+下载 4f163fc / 历史记录页 34467be）
- 依据：客户反馈（2026-10-24）——界面词汇偏程序员、默认不应写原图文件夹、要有下载按钮与历史记录；六项决策已与用户对齐（暂存区架构 / 输出设置迁移到下载时 / 默认下载文件夹 / 历史按「一次处理」/ 保留 30 次 / 词汇表基调）
- 配套文档：术语表 `GLOSSARY.md`（界面文案单一真相）、`docs/adr/0007-staged-results-manual-download.md`（架构决定，废止 D-12/D-35、修订 D-19）
- 编号续接：US-58 起、D-54 起（P0 方案文档占用了 US-54~57 / D-47~53）

---

## 变更一：界面文案全面口语化（US-58，D-57）

词汇以 `GLOSSARY.md` 为单一真相；**Go 侧返回的用户可见文案同一张表**（铁律 1：判定与文案的真相在 Go，前端只转述——本次扩展为「文案词汇的真相在 GLOSSARY」）。代码标识符（`BatchRules`/`OutputPlan`/`Preset` 等）一律不动，只改字符串。

### 替换表（按文件，实施时全量核对）

**Go 侧（用户可见文案出处）** `internal/service` + `internal/imageproc`：

| 现在 | 换成 |
|---|---|
| 请选择输出格式，或开启目标体积/尺寸限制 | 请至少选一项：转格式、压到指定大小或限制尺寸 |
| 宽高需同时设置 | 宽和高要一起填 |
| 宽高需在 1–65535 px | 宽和高要在 1 到 65535 之间 |
| 目标体积需在 1 KB–1 GB | 大小要在 1 KB 到 1 GB 之间 |
| 目标体积需为整数 KB 或 MB | 大小要填整 KB 或整 MB |
| 已达标 | 无需处理（原图已经合适） |
| 自动选择 WebP（最优） | 已自动选了最省空间的格式（WebP） |
| 首版不支持动图 | 会动的图片（动图）暂不支持 |
| 已在队列中 / 重复路径 | 已经在列表里了 / 重复了 |
| 批次已取消 / 批次完成，部分文件失败 | 已取消 / 处理完成，有 N 张没成功 |
| 目标体积过小，无法达到 | 这个大小压不到，请调大一点 |

**前端** `views/image/*` + `Settings.vue` + `AppShell`：

| 现在 | 换成 |
|---|---|
| 预设 / 管理预设 / 暂无预设 / 选择预设… | 常用设置 / 管理常用设置 / 还没保存过设置 / 选择常用设置… |
| 输出格式 | 转成哪种格式 |
| 目标体积 | 压到指定大小 |
| 已跳过（状态） | 无需处理 |
| 成功（状态） | 已完成 |
| 已明显降质 | 画质变差了 |
| 结果已过期 | 设置改过了 |
| 重跑（按钮）/ 重跑失败/未完成项（n） | 再试一次 / 把没成功的 n 张再试一次 |
| 调参（按钮）/ 调参 · 文件名 | 单张试一试 |
| 试算中… / 三格式竞速 | 正在试效果… / 三种格式比一比 |
| 原图 / 结果（对比弹窗两侧） | 原图 / 处理后 |
| 透明背景已铺白 | 透明背景已填成白色 |
| 参数或队列已变化，已有结果标记为过期… | 设置改过了，重新点「开始处理」即可 |
| describeRules：`WebP · 目标 500 KB · 1920×1080` | `转成 WebP · 压到 500 KB · 最宽 1920` |

JPEG/PNG/WebP/KB/MB 等通用词保留。缩略的「检查中/可处理/待检查」等状态词保持简单口语。

**护栏**：文案是人写的，不设机器护栏（同 D-46 的处理）；code review 对照 GLOSSARY 守护，并把 `docs/map.md` 与 AGENTS.md 里补一句「用户文案词汇以 GLOSSARY.md 为准」。

工作量：**S–M**（面广但机械；Go 文案改动会影响既有断言 message 的测试，需同步）。

---

## 变更二：暂存区 + 手动下载（US-59~61，D-54/55/58）

### 现状 → 目标

现状（P1 刚做的）：处理时按 `OutputPlan`（源目录/指定目录/保留结构）直接落盘，命名模板处理前设置。目标（ADR-0007）：**处理只进暂存区，原图文件夹零写入；下载是唯一落盘出口**。

### Go 侧

**1. 暂存区布局**（`internal/appdir` 语义扩展，实际目录在 service 组装）：

```
<UserConfigDir>/ImageWorkshop/
  history/<batchKey>/<itemKey>.<ext>   # 暂存的结果图片
  ImageWorkshop.db / config.json / app.log ...
```

`batchKey` 批次开始时生成（时间戳+序号，如 `20261024-1530-01`）；`itemKey` 用条目在批次内的序号（`0001`），暂存区内无重名，不再需要防覆盖序号——**防覆盖逻辑整体后移到下载时**。

**2. 新模型**（`internal/model/history.go`，登记 `AllModels()`）：

```go
type HistoryBatch struct {
    BaseModel
    BatchKey   string // 暂存目录名，与磁盘一一对应
    OutputFormat string; TargetBytes int64; MaxWidth, MaxHeight int
    Summary    string // 人话设置摘要（写入时生成，读取方不再重算——避免两份拼装逻辑）
    Total, Success, Failed, Skipped, Canceled int
    SavedBytes int64
    FilesPurged bool // 暂存图片已被清理（历史条目仍在，不可再下载）
}
type HistoryItem struct {
    BaseModel
    BatchID     uint // HistoryBatch 外键（索引）
    SourcePath  string; SourceName string; SourceBytes int64
    RelPath     string // 导入相对路径（下载保留层级用）
    Status      string; Message string
    Format      string; Width, Height int; OutputBytes int64
    ExtremeDegraded, AlphaFlattened bool
    Thumbnail   string // 96px base64（成功项由输出字节生成；历史明细不再依赖源文件存在）
    StagingName string // 暂存文件名（成功项；清理后置空）
    Downloaded  bool; DownloadedAt int64
}
```

**3. `ProcessBatch` 改造**：`convertOnce` 的输出段从「`resolveOutputDir` + `MkdirAll` + 命名落盘」改为「写入 `history/<batchKey>/<itemKey>.<ext>`」；批次结束时写 `HistoryBatch` + 全部 `HistoryItem`（单事务）。`resolveOutputDir`/`ensureOutputDir`/`renderOutputStem`/`placeOutputFile`/`writeConvertedFile` 全部保留，改为**下载链路**使用。`BatchRequest.Output`（OutputPlan）移除；新增缩略图生成入口（`imageproc` 导出 `ThumbnailDataURL(data []byte)`，解码输出字节生成——成功项输出比原图小，代价可控）。

**4. 下载绑定**（新增）：

```go
type DownloadOptions struct {
    Directory     string `json:"directory"`      // 必填绝对路径
    KeepStructure bool   `json:"keep_structure"` // 按导入层级下钻子文件夹
    Pattern       string `json:"pattern"`        // 文件名格式，空 = "{name}-out"
}
type DownloadResult struct { ItemID uint; Path string; Message string; OK bool }

DownloadBatchItems(batchKey string, itemIDs []uint, opts DownloadOptions) ([]DownloadResult, error)
DefaultDownloadDir() (string, error)   // 上次目录 → 系统 Downloads → 用户主目录
```

下载实现 = 读暂存字节 → `ensureOutputDir` → 按需下钻 `RelPath` → `renderOutputStem`（`{n}` = 清单内序号）→ `writeConvertedFile`（临时文件同目录 + 链接/独占复制回退，防覆盖 -2/-3 全套复用）→ 更新 `Downloaded/DownloadedAt`。**下载是复制**：暂存文件保留，可反复下载（直到历史清理）。

**5. 配置**：`config.json` 新增 `download_dir`（上次选择）；`image_output_mode/directory/pattern` 三字段废止——结构体字段保留以兼容旧文件读取，但不再写入、不再被任何逻辑消费（ADR-0007 后果条款）。

**6. 清理**：`PruneHistory(keep=30)`——保留最近 30 个 `HistoryBatch`（按创建时间），更早的删除暂存目录、置 `FilesPurged=true`、清空各 item `StagingName`；在应用启动（库打开后）与每次批次落库后各跑一次。`ClearHistory()` 绑定：全删（目录+行）。

**7. 汇总语义**：`BatchResult`/队列行为不变（取消、重跑、无需处理照旧）；重跑产生**新的**一条历史（独立成条，不合并）。

### 前端（工作台）

- **删除**整个「输出到」区块（模式下拉/目录按钮/命名输入/示例）与 `outputPlan` 状态——`startBatch` 请求只剩 `items + rules`。
- **结果区新增**：顶部「全部下载（n）」（n=本次成功数，n=0 隐藏）；成功行「下载」按钮（替代原对比按钮旁新增）。
- **下载选项框（单张与全部共用，D-58）**：点任一「下载」都先弹同一个选项框——保存位置（默认记忆值，可「更改位置」）+「保留原来的文件夹层级」勾选 +「文件名格式」输入（带 `RenderOutputSample` 实时示例，该绑定与命名/防覆盖逻辑原样复用，只挪了触发时机）。选项（目录/层级/格式）随「上次选择」记忆到 `config.json`，下次打开预填。确认后：单张下载保存该项，全部下载保存全部成功项。
- 下载完成 notice：「已保存 n 张到 xxx」+「打开文件夹」（复用 `reveal.Open` 经 `OpenDataDir` 同款绑定模式，新增 `RevealPath(path)` 绑定）。
- 队列行的「已下载」徽标；重新处理后重置。

### 护栏与测试

- parity：`OutputMode` 镜像行**删除**（Go 常量与 TS 镜像同步删，双向护栏保证）；无新增常量（`DownloadOptions` 字段走 wails 生成）。
- bindings 护栏：新增 `DownloadBatchItems`/`DefaultDownloadDir`/`RevealPath`/`ClearHistory`/`ListHistory*` 需 `make build` 再生成。
- Go 测试：暂存写入与批次落库（成功/失败/无需处理混合）；下载路径矩阵（平铺/保留层级/命名/重名防覆盖/暂存缺失报错）；`PruneHistory` 保留 30 与 `FilesPurged` 语义；`ClearHistory`；`DefaultDownloadDir` 回退链；旧 config（含 `image_output_*`）读取兼容。
- 存量测试改造：P1 输出计划的批次落盘测试（`TestProcessBatchCustom*`）语义整体迁到下载层测试；批次层只留暂存断言。

工作量：**M–L**（本方案最大块）。

---

## 变更三：历史记录页（US-62~64，D-56）

- **侧栏新增「历史记录」**（`AppShell` menuItems + `router` + `views/history/`），空态：「还没有处理过图片」。
- **列表**（`HistoryList.vue`，复用 `usePagedList` + `page` 契约）：每条 = 时间（`2026-10-24 15:30`）+ 人话摘要（`转成 WebP · 压到 500 KB · 最宽 1920`，直接读 `Summary` 列）+ 张数（`成功 12 · 无需处理 2 · 失败 1`）+ 一共省了 `xxx`。
- **明细**（点击进入明细视图或弹窗）：每项缩略图（`Thumbnail`，源文件已删也能看）、文件名、前后大小、状态（人话）、失败原因、「下载」按钮（成功且 `StagingName` 非空且未 `FilesPurged`）；顶部「全部下载」。文件被历史清理的条目显示「图片已清理，不能再下载」。
- **绑定**：`ListHistoryBatches(req page.Request)`、`ListHistoryItems(batchID, req)`（分页契约）。
- **设置页**：「清空历史记录」按钮（`Confirm` 二次确认，复用现有对话框绑定）+ 显示暂存区占用大小（`HistoryUsage() (int64, error)`，按需计算）。

工作量：**M**。

---

## 4. 规格修订清单（随各切片交付）

| 条款 | 修订 |
|---|---|
| US-16 / D-12 / D-35 | 标注废止，指向 ADR-0007；US-16 改写为「处理只进暂存，原图文件夹零写入；下载时命名/防覆盖/层级」 |
| D-19 | 修订为「有限历史：保留 30 次可下载暂存 + 可清空；无限增长的历史负担仍不做」 |
| §6「持久化任务历史」排除项 | 同步收窄为「无限历史」 |
| 主规格 §2 绑定表 | 增补下载/历史绑定；`ProcessBatch` 入参去掉 output |
| README「能做什么」 | 「到源文件同目录取 -out 结果」改为「点下载保存到你选的文件夹」+ 新增历史记录一段 |
| `docs/map.md` / `AGENTS.md` | 补「用户文案词汇以 GLOSSARY.md 为准」；map 增 GLOSSARY/历史页/新绑定行 |

**新增决策（进主规格决策表）**：D-54（暂存区取代直接写盘，ADR-0007）、D-55（下载为唯一落盘出口，选项=目录+保留层级+文件名格式）、D-56（历史保留 30 次+手动清空）、D-57（界面文案以 GLOSSARY 为单一真相，Go 文案同表）、D-58（单张与全部下载**都**弹同一选项框：目录+保留层级+文件名格式；选项记忆到下次，确认后才保存）。
**新增用户故事**：US-58（人话文案）、US-59（处理不写原图文件夹）、US-60（单张/全部下载）、US-61（下载完成反馈与已下载标记）、US-62（历史列表）、US-63（历史明细与再次下载）、US-64（历史保留与清空）。

## 5. 实施顺序与验证

```
变更一 文案+GLOSSARY（S–M，独立先行——客户最痛、风险最低）
  → make test（改 Go 文案断言）+ lint + smoke
变更二 暂存+下载（M–L：模型/暂存/下载/清理 → make build → 工作台改造）
变更三 历史页（M：绑定 + 列表/明细/清空）
每步 make test + lint；收尾 make smoke + 规格合并 + README
```

风险提示：变更二会**删除 P1 刚做的输出模式 UI 与 `OutputMode` 常量**（语义迁移而非丢弃——机制全部保留、触发点后移）；`config.json` 的三个输出字段从此闲置。这两点已在 ADR-0007 记录，属本次客户需求的必然代价。
