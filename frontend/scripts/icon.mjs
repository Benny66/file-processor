// ==========================================================================
// icon.mjs — 图标资产派生管线唯一入口（make icon）
// --------------------------------------------------------------------------
// 源（唯一真相：改这里，不许直接改派生物）：
//   build/appicon.svg            应用图标（全平台单源，ADR-0008）
//   build/tray.svg               托盘单色剪影
//   build/darwin/dmg-bg-tpl.svg  dmg 安装背景模板
//
// 派生物（入库；CI 不重新栅格化，本地改完源跑 make icon 再提交）：
//   build/appicon.png                    1024（macOS icns 源 / Linux hicolor 源）
//   build/windows/icon.ico               16–256（exe 与 NSIS 安装器）
//   build/tray.png                       32（main.go //go:embed 作托盘图标）
//   build/darwin/dmg-background.tpl.png  600×400（compose-dmg-bg.sh 打包期叠加合成）
//   build/icon.manifest                  源+派生物 sha256 登记表
//
// 护栏 internal/guard/icon_test.go 校验 manifest 哈希：
// 改了 SVG 没跑 make icon、或手改了派生物，make test 直接红。
//
// 依赖：@resvg/resvg-js（栅格化，支持 feDropShadow）、png-to-ico（PNG-in-ICO）。
// 二者是 devDependencies 工具链，按 deps.yaml 现行口径不登记（理由记于 ADR-0008）。
// ==========================================================================
import { createHash } from 'node:crypto'
import { readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { Resvg } from '@resvg/resvg-js'
import pngToIcoModule from 'png-to-ico'

const pngToIco = pngToIcoModule.default ?? pngToIcoModule
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')

const sha256 = (buf) => createHash('sha256').update(buf).digest('hex')

/** 按目标像素宽栅格化（等比）；fitTo 保证 600×400 这类非方形源也精确落尺寸。 */
const render = (svg, width) =>
  new Resvg(svg, {
    fitTo: { mode: 'width', value: width },
    font: { loadSystemFonts: true, defaultFontFamily: 'PingFang SC' },
  }).render().asPng()

const sources = {}
const derived = {}
const save = async (rel, buf) => {
  await writeFile(path.join(ROOT, rel), buf)
  return rel
}

// ---- 1. 应用图标：1024 主图 + 多尺寸 ICO ----
const appiconSvg = await readFile(path.join(ROOT, 'build/appicon.svg'), 'utf8')
sources['build/appicon.svg'] = sha256(Buffer.from(appiconSvg))
const appiconPng = render(appiconSvg, 1024)
derived[await save('build/appicon.png', appiconPng)] = sha256(appiconPng)

const ICO_SIZES = [16, 24, 32, 48, 64, 128, 256]
const ico = await pngToIco(ICO_SIZES.map((s) => render(appiconSvg, s)))
derived[await save('build/windows/icon.ico', ico)] = sha256(ico)

// ---- 2. 托盘剪影：32px ----
const traySvg = await readFile(path.join(ROOT, 'build/tray.svg'), 'utf8')
sources['build/tray.svg'] = sha256(Buffer.from(traySvg))
const trayPng = render(traySvg, 32)
derived[await save('build/tray.png', trayPng)] = sha256(trayPng)

// ---- 3. dmg 背景模板：600×400 ----
const dmgSvg = await readFile(path.join(ROOT, 'build/darwin/dmg-bg-tpl.svg'), 'utf8')
sources['build/darwin/dmg-bg-tpl.svg'] = sha256(Buffer.from(dmgSvg))
const dmgPng = render(dmgSvg, 600)
derived[await save('build/darwin/dmg-background.tpl.png', dmgPng)] = sha256(dmgPng)

// ---- 4. 登记表（guard 消费；键序固定，diff 友好） ----
const manifest = { version: 1, sources, derived }
await writeFile(path.join(ROOT, 'build/icon.manifest'), `${JSON.stringify(manifest, null, 2)}\n`)

console.log('图标派生完成：')
for (const [rel, hash] of Object.entries(derived)) console.log(`  ${rel}  ${hash.slice(0, 12)}…`)
console.log('已更新 build/icon.manifest；请连同派生物一起提交。')
