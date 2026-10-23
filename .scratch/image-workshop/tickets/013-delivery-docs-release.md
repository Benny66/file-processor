# 013 — 交付文档与发布基线

**Blocked by**: 012

**规格**: US-35、US-36；D-32

## 目标

把仓库从脚手架文档改为「图片工坊」产品文档：功能、安装、构建、Kylin 说明、已知限制与隐私边界齐全，版本与发布信息可追溯。

## 涉及层

- [ ] 数据层：无。
- [ ] 逻辑层：核对文档中的命令与脚本真实可用；版本号 0.1.0；bundle id 占位与替换说明；依赖清单与 `deps.yaml` 一致。
- [ ] UI 层：使用真实界面截图（如有），不使用脚手架占位图。
- [ ] 测试：逐条运行文档命令；`make test` / `make lint`；检查脚手架残留文案。

## 验收标准

- README 与产品文档不再描述脚手架母版，明确产品名、三大能力、批量队列工作流。
- 安装/构建说明覆盖 macOS、Windows、Kylin；Kylin 的构建容器与 `.deb` 安装步骤可复现。
- 已知限制写清：AVIF/HEIC、动图、ICC 色彩、代码签名/公证、bundle id 占位、Kylin 构建环境。
- 隐私说明写明本地处理、无上传、无遥测；权限与数据目录可定位。
- 版本 0.1.0 与发布流程一致；依赖登记与 `deps.yaml` 一致。
- 文档中的命令逐条验证通过。
- 无脚手架残留文案（历史设计说明可保留但需标注）。

## 预置待办（005→011 累计审查分流，2026-10-23）

来源：`.scratch/image-workshop/reviews/2026-10-23-review-005-011.md`。这些不在审查轮做，但不该丢。

- **交付链完整性**
  - `release.yml` 的 tag 发布仍只有 macOS/Windows 两条腿，Linux `.deb` 只在 `build.yml` 上传 artifact（011 已声明留给本票）。
  - `.deb` 的 Depends 目前按 001 结论硬编码 `libc6 (>= 2.31)`，未从二进制的 `GLIBC_*` 符号推导并断言（US-31「Depends 与实际链接库一致」目前靠人工映射表 `.scratch/kylin/evidence/deb-needs.txt`）。
  - Kylin 依赖 pin 写了三处（`build/linux/Dockerfile` 两个分支 + `.github/workflows/build.yml`），`.deb` Depends 是第四处；`build-linux-container.sh` 与 `package-deb.sh` 里还硬编码了应用名与包名，而 `scripts/app-name.sh` 才是真相。
- **干净性（铁律 3）**
  - `.scratch/` 已入库约 27 MB（13 张证据 PNG + kylin 证据），`.gitignore` 只排 `/.scratch/kylin/out/`。交付前决定：证据走 release artifact 还是继续入库。
- **代码规范分流项（标准轴 建议级）**
  - `image_service.go:119-144` 与 `batch_service.go` 的「读文件 + 普通文件 + 512 MB」两处实现合并；`batch_service.go` 里 `SourceBytes` 连续两次赋值（前者是死存储）。
  - `PresetDialog.vue` 的 trim/空名校验与 `preset_service.go` 重复；`removePreset` 绕过既有 `run()` 包装。
  - `preset_service.go:45` 用 `fmt.Errorf` 而非 `apperr`——护栏 `TestBindingsUseAppErr` 只解析 `app.go`，扫不到 service 层（护栏覆盖面本身也是待办）。
  - `imageproc` 内 `"jpeg"/"png"/…` 字面量与 `Output*` 常量并存；`inspect.go` 与 `orientation.go` 各写一份魔数嗅探。
  - `make test` 不带 `-race`：批次的 `progressMu`/`batchMu` 与托盘回调的 atomic 都属并发面。
  - `ImageWorkbench.vue` 仍是 1100+ 行、6 类职责的过大类（导入队列 / 规则表单 / 预设 / 批次 / 缩略图懒调度 / 上次设置防抖）；拆分会与本轮修复冲突，故留到 012 走查之后单独做。
- **范围蔓延登记（规格轴）**
  - `useModalFocus.ts`（Esc 关闭、遮罩 inert、焦点归还）超出 US-28；`preset_service.go:22` 额外要求目标体积为 1024 整数倍；`config.go:100-106` 在配置损坏时**覆写**用户文件（US-13 只要求回退默认值）。写进交付文档或收敛实现，二选一。

## 完成记录

- 状态：进行中（README 已产品化并配真实构建截图；其余交付文档与发布基线待开始）
- 已完成（2026-10-23，README 专项）：
  - `README.md` 重写为产品文档：定位 + 英文摘要 + 三平台安装包表 + 能力/边界表 + 界面截图（带说明）
    + 使用与构建 + 工程约束 + 已知限制；脚手架内容收进底部「附：工程基座」。
  - 截图来自**当前构建的前端**（`frontend/dist` + Playwright headless，队列为示例数据，README 内已标注）：
    `docs/images/工作台-批量处理.gif`（处理中状态机）、`工作台-处理结果.png`、`单张对比.png`、`预设管理.png`、
    `设置页面.png`（替换脚手架时期旧图）。拍摄脚本：`.scratch/image-workshop/visual/capture-readme-shots.js`。
  - 删除 `docs/images/仪表盘.png`：该页面已被 US-38 移除，留着会误导（恢复：`git checkout <旧提交> -- docs/images/仪表盘.png`）。
  - 事实核对：macOS 最低版本按 `build/darwin/Info.plist` 写 10.13（不是 11+）；签名状态写「ad-hoc 自签」；
    未写「Element Plus 组件」——它在 `deps.yaml` 里登记为基座组件库，但产品页面用的是自绘组件 + tokens。
- 待办新增（README 拍摄时发现，属交付/健壮性范围）：
  - **弹窗渲染抛错会把整页锁死**：`useModalFocus` 打开时给 `#app` 加 `inert`，若弹窗子树在渲染中抛错
    （例：预设字段形状与 `model.ImagePreset` 不符，`preset.output_format.toUpperCase()` 抛 TypeError），
    `inert` 不会随关闭移除，此后整个应用点不动、且没有任何提示。本次是替身数据形状不对触发的，
    但真实触发条件就是「前后端契约漂移」——正是本仓最怕的无症状失效类别。
    建议：给对话框渲染加兜底（字段缺失时显示占位而非抛错），并补一条护栏/测试断言
    「弹窗关闭后 `#app` 不得残留 inert」。归入本票或 012 走查后单独修。
- 验收证据：
- 偏离与遗留：
