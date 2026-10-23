# 003 — 产品身份与工作台骨架

**Blocked by**: 无

**规格**: US-35、US-38；D-01、D-32

## 目标

应用以「图片工坊 / ImageWorkshop」启动，默认进入空的图片处理工作台，侧栏与设置页可用，三平台的名称、数据目录与单实例标识一致；同时把构建基线升到 Go 1.26（002 spike 发现的 WebP 依赖硬前置）。

## 涉及层

- [x] 数据层：`appName` 改为 ImageWorkshop，配置/数据库/日志路径随之变化；不迁移旧 `file-processor` 数据；单实例锁换成 ImageWorkshop 专属 ID。
- [x] 逻辑层：更新 `wails.json` 的 productName/outputfilename/version；`main.go` 窗口标题与 appName；appinfo/config/logging/tray 文案；打包脚本与 CI 中的名称引用。Go module 路径可保留 `file-processor`。**构建基线：`go.mod` 升到 `go 1.26.0`，CI `setup-go` 用 1.26，README 环境要求同步；验证 Wails 构建在 Go 1.26 下可用。**
- [x] UI 层：默认路由指向 `views/image/ImageWorkbench.vue`；空状态与拖拽区（视觉占位）；侧栏“图片处理/设置”；移除“仪表盘”占位；设置页保留主题/数据目录/版本/日志/数据库导出。
- [x] 测试：Go 测试适配；macOS `make build` 与 `make lint`；检查用户可见的 `file-processor` 残留；核对产物名称；确认 Go 版本在 `go.mod`、CI、README 三处一致。

## 验收标准

- 启动后直接进入图片处理工作台，不再出现“仪表盘”占位。
- 窗口标题、产品名、数据目录、日志名、单实例 ID 均使用 ImageWorkshop/图片工坊。
- 设置页原能力仍可用。
- 空状态明确说明可拖入图片或文件夹，不允许无标签占位块。
- macOS 构建与 lint 通过；三平台产物名称计划写明。
- Go 基线为 1.26.x；`go.mod`、CI `setup-go`、README 三处一致，本机 `make build` 通过。
- 用户可见表面无 `file-processor` 残留；仓库模块路径允许保留。
- 本切片不实现导入处理；拖拽区只是视觉占位，由 004 接线。

## 完成记录

- 状态：完成（2026-10-23）
- 实现说明：
  - 身份：`main.go` 的 `appName` 改为 `ImageWorkshop`，新增 `appDisplayName=图片工坊`，窗口标题与托盘提示共用；单实例 ID 改为 `ImageWorkshop-single-instance`；`wails.json` 的 name/outputfilename 改为 `ImageWorkshop`、productName 改为“图片工坊”、productVersion 改为 `0.1.0`。数据目录、日志名、数据库文件随 appName 变为 ImageWorkshop；旧 `file-processor` 数据不迁移。
  - Go 基线：`go.mod` 升到 `go 1.26.0`；build/release 两条 CI 的 `setup-go` 均改为 `1.26`；README 环境要求改为 ≥1.26。
  - 工作台：默认路由 `/` 指向 `frontend/src/views/image/ImageWorkbench.vue`；侧栏改为“图片处理/设置”；删除旧的 `Home.vue` 占位页；空状态含拖拽区、支持格式说明与隐私说明，尚未接拖拽事件。组件按 template→script→style 三段式组织，不含 ticket 引用注释。
  - 打包与导航：新增 `scripts/app-name.sh` 作为 outputfilename 唯一读取入口，`package.sh`/`release.sh`/`smoke.sh` 共用；`.gitignore` 增加 `ImageWorkshop`；`docs/map.md` 增加 image 视图与脚本说明；README 标题/简介/模块说明同步。
  - 三平台产物名计划：macOS `ImageWorkshop.app` / `ImageWorkshop.dmg`；Windows `ImageWorkshop-amd64-installer.exe`（发布工作流会重命名为 `ImageWorkshop-<version>-windows-amd64-setup.exe`）；Kylin `.deb` 包名 `image-workshop`，具体产物名在 011 定稿。
- 验收证据：
  - 评审修复后复跑：`make test` 通过（含 guard 包）；`make lint` 通过（gofmt + guard + go vet + ESLint + vue-tsc）；`npm run build` 通过；`bash scripts/app-name.sh` 输出 `ImageWorkshop`。评审修复项：SFC 三段式顺序、移除 ticket 引用注释、脚本 JSON 解析收口到 `app-name.sh`、托盘提示改用中文显示名。
  - `make build` 使用 Wails v2.15.0 + Go 1.26.2 成功，产物 `build/bin/ImageWorkshop.app`；Info.plist：CFBundleName=图片工坊、CFBundleExecutable=ImageWorkshop、CFBundleIdentifier=com.example.imageworkshop、版本 0.1.0。
  - `make smoke` 通过：进程存活，`~/Library/Application Support/ImageWorkshop/config.json` 与 `ImageWorkshop.db` 落盘。
  - `make verify-gen` 通过：连续生成两个模块、bindings、编译、gofmt、护栏全绿。
  - 真实前端 headless 渲染：`document.title=图片工坊`；侧栏“图片处理/设置”；默认激活“图片处理”；标题与拖拽区文案正确；无“仪表盘”/“脚手架已就绪”占位。截图见 `.scratch/image-workshop/evidence/imageworkshop-workbench.png` 与 `imageworkshop-workbench-dark.png`，亮/暗主题均清晰可读。
  - 设置路由渲染正常（标题、侧栏、主题按钮）；静态浏览器没有 Wails bindings，后端值显示“—”并报 `[internal] Cannot read properties of undefined (reading 'main')`，属于静态检查环境的预期限制，不是产品缺陷。实际应用启动、配置与数据库路径已由 smoke 验证，设置页业务代码未修改。
- 偏离与遗留：
  - README 的完整产品化改写由 013 负责；本切片只同步标题、简介、Go 版本与 module 说明。
  - 工作台拖拽区目前是视觉占位，导入事件由 004 接线。
  - bundle id 仍是 `com.example.imageworkshop`，发布前必须替换；由 013 收口。
  - Kylin/Windows 的产物身份与安装包未在本切片验证，留给 011/012。
  - Windows/Linux 关闭到托盘的行为未改；Kylin 托盘/关闭兜底由 011 处理。
  - 设置页的配置/版本/日志等后端值未在原生窗口逐项点验；静态浏览器没有 Wails bindings，只验证了路由与控件渲染。真实 app 的启动、配置与数据库路径由 smoke 覆盖，设置页代码未修改；可在 004 接线后再补一次真机点验。
  - 本切片未提交。
