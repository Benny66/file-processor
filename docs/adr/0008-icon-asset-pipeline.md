# 图标资产：SVG 单源派生 + 哈希护栏收口

应用图标此前是三处互不关联的脚手架模板残留（`build/appicon.png` 是 Wails 默认图、`build/windows/icon.ico` 与 dmg 背景同源），且 `main.go` 把全彩 `appicon.png` 直接 embed 当托盘图标——16px 下糊成色块。决定设计新图标（叠照 + 斜压箭头，品牌蓝渐变，全平台单源一张图），并把图标资产按本仓库「单一真相 + 会红的护栏」惯例整体收口：**`build/*.svg` 是唯一源，`make icon`（`frontend/scripts/icon.mjs`）本地派生 PNG/ICO，派生物入库，`build/icon.manifest` 登记源与派生物的 sha256，护栏 `internal/guard/icon_test.go` 校验哈希**——改源不重派生、手改派生物、产物清单脱节，`make test` 直接红。托盘图标独立为 `build/tray.svg` 派生的单色剪影（品牌蓝双阶而非纯黑，深浅任务栏都可见）。

**备选方案**：手放各平台 PNG（现状——4 个落点要人肉记得全改，漏一处不报错，正是铁律 5 说的无症状失效）；CI 构建时装 librsvg/ImageMagick 现场栅格化（三平台含麒麟 Ubuntu 20.04 容器都要加依赖，CI 变重，被否）；每平台一个变体（macOS 底板留边、Windows 满幅——地道但管线×3、一致性要人工守护，选了单源折中：底板 848/1024 居中，macOS 基本地道，Windows/Linux 圆角方块完全正常）。

**栅格化依赖**：`@resvg/resvg-js`（resvg 引擎，支持 feDropShadow，prebuilt 零系统依赖）与 `png-to-ico`（PNG-in-ICO）。二者是 devDependencies 工具链，按 `deps.yaml` 现行口径「devDependencies 不登记」处理，理由记于此处；派生确定性已验证（同机重复运行字节一致，源不含字体渲染之外的变量；dmg 模板含中文提示文字，跨机器重派生可能因字体差异哈希漂移——漂移即红，重跑 `make icon` 更新登记即可）。

**后果**：改图标的流程固定为「改 `build/*.svg` → `make icon` → 连同派生物与 manifest 一起提交」；CI 零新增依赖；`build/darwin/dmg-background.png` 占位图删除（dmg 背景由模板 + 打包期 `compose-dmg-bg.sh` swift 合成，模板缺失时脚本自带降级链）；Linux hicolor 与 macOS icns 继续由 Wails 从 `appicon.png` 派生，不新增产物。
