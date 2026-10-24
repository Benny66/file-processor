# build/ — 构建资源与图标资产

## 图标资产管线（ADR-0008）

SVG 是唯一源，派生物入库、**不许手改**（护栏 `internal/guard/icon_test.go` 校验哈希，
不一致 `make test` 直接红）。改图标：改 SVG → `make icon` → 连同派生物与 manifest 提交。

| 文件 | 角色 |
|---|---|
| `appicon.svg` | **源**：应用图标（全平台单源，叠照 + 斜压箭头，品牌蓝渐变） |
| `tray.svg` | **源**：托盘单色剪影（品牌蓝双阶，深浅任务栏都可见） |
| `darwin/dmg-bg-tpl.svg` | **源**：dmg 安装背景模板（应用名与箭头由打包期 swift 合成） |
| `appicon.png` | 派生 1024：macOS icns 源（Wails 构建时生成）/ Linux hicolor 源（package-deb.sh 复制） |
| `windows/icon.ico` | 派生 16–256：exe 与 NSIS 安装器图标 |
| `tray.png` | 派生 32：托盘图标（`main.go` //go:embed） |
| `darwin/dmg-background.tpl.png` | 派生 600×400：`scripts/compose-dmg-bg.sh` 的底图 |
| `icon.manifest` | 源 + 派生物 sha256 登记表（`frontend/scripts/icon.mjs` 生成，护栏消费） |

## 各平台打包资源

- `darwin/`：Info.plist（`wails build` / `wails dev` 各一份）。
- `windows/`：manifest、icon.ico、NSIS 安装器脚本、info.json。
- `linux/`：desktop entry、Ubuntu 20.04 构建镜像（.deb 的 hicolor 图标由 package-deb.sh
  从 `appicon.png` 复制，无需单独的图标文件）。
- `bin/`：构建产物（不入库）。
