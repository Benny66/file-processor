// capture-readme-shots.js — 为 README 拍摄当前构建的真实界面
//
// 前端是真实构建产物（frontend/dist），只有 Go 绑定用替身，因此界面/文案/状态
// 都是当前代码的；队列里的图片信息是【示例数据】，README 里也照此标注。
//
// 前置：`cd frontend && npm run build` + `npx vite preview --host 127.0.0.1 --port 4173`
// 产物：docs/images/{工作台-批量处理.gif, 工作台-处理结果.png, 单张对比.png, 预设管理.png, 设置页面.png}
//       帧图在 /tmp/readme-frames/（供 ffmpeg 合成 GIF）
const { chromium } = require('/Users/bennywen/.npm/_npx/6bcb61ec6d5aea22/node_modules/playwright')
const fs = require('fs')
const path = require('path')

const FRAME_DIR = '/tmp/readme-frames'
const OUT_DIR = 'docs/images'

// 示例队列：覆盖格式、中文与空格路径、透明、动图失败、扩展名不一致、已达标
const QUEUE = [
  { p: '/Users/benny/Pictures/商品主图.jpg', n: '商品主图.jpg', s: 2457000, fmt: 'jpeg', w: 1920, h: 1920, c: '#5b8def' },
  { p: '/Users/benny/Pictures/细节图 1.png', n: '细节图 1.png', s: 1126000, fmt: 'png', w: 1200, h: 1600, c: '#30a46c' },
  { p: '/Users/benny/Pictures/banner.webp', n: 'banner.webp', s: 492000, fmt: 'webp', w: 1920, h: 600, c: '#e8a33d' },
  { p: '/Users/benny/Pictures/logo.png', n: 'logo.png', s: 63000, fmt: 'png', w: 400, h: 400, c: '#9b6bd6' },
  { p: '/Users/benny/Pictures/老照片.bmp', n: '老照片.bmp', s: 3212000, fmt: 'bmp', w: 1024, h: 768, c: '#c9705a' },
  { p: '/Users/benny/Pictures/扫描件.tif', n: '扫描件.tif', s: 5100000, fmt: 'tiff', w: 1600, h: 2200, c: '#6b7280' },
  { p: '/Users/benny/Pictures/动态表情.gif', n: '动态表情.gif', s: 122000, fmt: 'gif', w: 320, h: 320, c: '#d64b7b' },
  { p: '/Users/benny/Pictures/伪装成jpg.png', n: '伪装成jpg.jpg', s: 90000, fmt: 'png', w: 800, h: 600, c: '#3fb0b0' },
  { p: '/Users/benny/Pictures/缩略图.png', n: '缩略图.png', s: 184000, fmt: 'png', w: 800, h: 600, c: '#8a9a5b' },
]

// 每条的处理结果（对应「JPEG + 目标 300 KB + 宽高盒 1600」这组规则）
const OUTCOMES = {
  '/Users/benny/Pictures/商品主图.jpg': { status: 'success', out: 288000, w: 1600, h: 1600, ext: 'jpg' },
  '/Users/benny/Pictures/细节图 1.png': { status: 'success', out: 299000, w: 1200, h: 1600, ext: 'jpg' },
  '/Users/benny/Pictures/banner.webp': { status: 'success', out: 191000, w: 1600, h: 500, ext: 'jpg' },
  '/Users/benny/Pictures/logo.png': { status: 'success', out: 41000, w: 400, h: 400, ext: 'jpg', alpha: true },
  '/Users/benny/Pictures/老照片.bmp': { status: 'success', out: 213000, w: 1024, h: 768, ext: 'jpg' },
  '/Users/benny/Pictures/扫描件.tif': { status: 'success', out: 300000, w: 1163, h: 1600, ext: 'jpg', extreme: true },
  '/Users/benny/Pictures/动态表情.gif': { status: 'failed', out: 0, w: 0, h: 0, msg: '首版不支持动图' },
  '/Users/benny/Pictures/伪装成jpg.png': { status: 'success', out: 76000, w: 800, h: 600, ext: 'jpg' },
  '/Users/benny/Pictures/缩略图.png': { status: 'skipped', out: 0, w: 800, h: 600, msg: '已达标' },
}

function thumb(item, edge) {
  const size = edge || 96
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}">` +
    `<rect width="${size}" height="${size}" fill="#1b1e24"/>` +
    `<rect x="6" y="${size * 0.18}" width="${size - 12}" height="${size * 0.64}" rx="4" fill="${item.c}"/>` +
    `<circle cx="${size * 0.3}" cy="${size * 0.42}" r="${size * 0.09}" fill="#ffffff" opacity="0.85"/>` +
    `</svg>`
  return 'data:image/svg+xml,' + encodeURIComponent(svg)
}

function previewDataUrl(item, width, height, faded) {
  const w = 620
  const h = Math.max(80, Math.round((w * height) / Math.max(1, width)))
  const svg =
    `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">` +
    `<rect width="${w}" height="${h}" fill="${faded ? '#22262e' : '#171a20'}"/>` +
    `<rect x="18" y="18" width="${w - 36}" height="${h - 36}" rx="10" fill="${item.c}" opacity="${faded ? 0.55 : 0.9}"/>` +
    `<circle cx="${w * 0.28}" cy="${h * 0.4}" r="${h * 0.16}" fill="#fff" opacity="0.8"/>` +
    `<rect x="${w * 0.18}" y="${h * 0.66}" width="${w * 0.64}" height="${h * 0.12}" rx="6" fill="#0e0f12" opacity="0.35"/>` +
    `</svg>`
  return 'data:image/svg+xml,' + encodeURIComponent(svg)
}

// 预览表：键是「页面上真正会去请求的路径」——源路径与 -out 输出路径各一条。
// 全部在 Node 侧算好，注入侧只查表，避免两处各自拼 stem 而对不上。
const PREVIEW_TABLE = {}
for (const i of QUEUE) {
  PREVIEW_TABLE[i.p] = { url: previewDataUrl(i, i.w, i.h, false), w: i.w, h: i.h, alpha: i.fmt === 'png' }
  const o = OUTCOMES[i.p]
  if (o.status === 'success') {
    const out = i.p.replace(/\.[^.]+$/, '') + '-out.' + o.ext
    PREVIEW_TABLE[out] = { url: previewDataUrl(i, o.w, o.h, true), w: o.w, h: o.h, alpha: false }
  }
}

const initScript = `
const QUEUE_DATA = ${JSON.stringify(QUEUE.map((i) => ({ p: i.p, n: i.n, s: i.s, fmt: i.fmt, w: i.w, h: i.h })))};
const THUMBS = ${JSON.stringify(Object.fromEntries(QUEUE.map((i) => [i.p, thumb(i)])))};
const PREVIEWS = ${JSON.stringify(PREVIEW_TABLE)};
const OUTCOMES = ${JSON.stringify(OUTCOMES)};
const eventHandlers = {};
function emitEvent(name, payload) {
  for (const cb of [...(eventHandlers[name] || [])]) cb(payload);
}
// 预设的字段形状必须与 wails 生成物 model.ImagePreset 一致：
// 扁平的 output_format/target_bytes/max_width/max_height + 数值时间戳。
// （上一版这里写成嵌套 rules + ISO 字符串，弹窗渲染时 undefined.toUpperCase() 抛错，
//  于是弹窗没画出来、#app 还停在 inert —— 顺带暴露了「弹窗渲染抛错会把整页锁死」。）
const presets = [
  { id: 1, name: '电商主图 · JPEG ≤300KB', created_at: 1761200000, updated_at: 1761200000,
    output_format: 'jpeg', target_bytes: 307200, max_width: 1600, max_height: 1600 },
  { id: 2, name: '公众号封面 · 1080 宽', created_at: 1761190000, updated_at: 1761190000,
    output_format: 'jpeg', target_bytes: 0, max_width: 1080, max_height: 0 },
  { id: 3, name: '存档 · 保持原格式', created_at: 1761180000, updated_at: 1761180000,
    output_format: 'keep', target_bytes: 0, max_width: 0, max_height: 0 },
];


window.go = { main: { App: {
  GetTheme: async () => 'dark',
  SetTheme: async () => {},
  SupportedFormatHints: async () => ['JPG', 'PNG', 'WebP', 'BMP', 'TIFF', 'GIF（静态）'],
  ValidateImageRules: async (rules) => {
    const t = (rules && rules.target_bytes) || 0;
    const w = (rules && rules.max_width) || 0;
    const h = (rules && rules.max_height) || 0;
    const f = (rules && rules.output_format) || 'keep';
    if (t !== 0 && (t < 1024 || t > 1073741824)) return '目标体积需在 1 KB–1 GB';
    if ((w === 0) !== (h === 0)) return '宽高需同时设置';
    if (w !== 0 && (w > 65535 || h > 65535)) return '宽高需在 1–65535 px';
    if (f === 'keep' && t === 0 && w === 0) return '请选择输出格式，或开启目标体积/尺寸限制';
    return '';
  },
  ScanPaths: async () => ({
    files: QUEUE_DATA.map((i) => ({ path: i.p, name: i.n, size: i.s })),
    rejected: [{ path: '/Users/benny/Pictures/说明.txt', reason: '不支持的格式' }]
  }),
  SelectFiles: async () => QUEUE_DATA.map((i) => i.p),
  SelectFile: async () => '',
  SelectDirectory: async () => '/Users/benny/Pictures',
  SelectSaveFile: async () => '',
  Confirm: async () => true,
  GetLaunchArgs: async () => [],
  InspectImage: async (p) => {
    const i = QUEUE_DATA.find((x) => x.p === p);
    return {
      path: p, format: i.fmt, width: i.w, height: i.h, bytes: i.s,
      thumbnail: THUMBS[p],
      warning: p.indexOf('伪装成jpg') >= 0 ? '扩展名与实际格式不一致（.JPG → PNG）' : ''
    };
  },
  ProcessBatch: async (req) => {
    const paths = req.paths || [];
    const items = [];
    let success = 0, failed = 0, skipped = 0, extreme = 0, saved = 0, alpha = 0;
    for (let i = 0; i < paths.length; i++) {
      const p = paths[i];
      emitEvent('image:progress', { done: i, total: paths.length, started: { path: p, status: 'processing' } });
      await new Promise((r) => setTimeout(r, 420));
      const o = OUTCOMES[p];
      const src = QUEUE_DATA.find((x) => x.p === p).s;
      const item = {
        path: p,
        output_path: o.status === 'success' ? p.replace(/\\.[^.]+$/, '') + '-out.' + o.ext : '',
        status: o.status,
        source_bytes: src,
        output_bytes: o.out,
        width: o.w,
        height: o.h,
        extreme_degraded: !!o.extreme,
        alpha_flattened: !!o.alpha,
        message: o.msg || ''
      };
      items.push(item);
      if (o.status === 'success') { success++; saved += src - o.out; if (o.extreme) extreme++; if (o.alpha) alpha++; }
      if (o.status === 'failed') failed++;
      if (o.status === 'skipped') skipped++;
      emitEvent('image:progress', { done: i + 1, total: paths.length, current: item });
    }
    return {
      ok: failed === 0, message: failed === 0 ? '' : '批次完成，部分文件失败',
      items, success, failed, skipped, canceled: 0, extreme_degraded: extreme, saved_bytes: saved
    };
  },
  CancelBatch: async () => {},
  RenderPreview: async (p) => {
    const e = PREVIEWS[p];
    if (!e) return { data_url: '', width: 0, height: 0, has_alpha: false };
    return { data_url: e.url, width: e.w, height: e.h, has_alpha: e.alpha };
  },
  GetImageSettings: async () => ({ output_format: 'jpeg', target_bytes: 307200, max_width: 1600, max_height: 1600 }),
  SaveImageSettings: async () => {},
  ListPresets: async (req) => ({ list: presets, total: presets.length, page: 1, page_size: (req && req.page_size) || 20 }),
  SavePreset: async (name, rules) => ({
    id: Date.now(), name, created_at: Math.floor(Date.now() / 1000), updated_at: Math.floor(Date.now() / 1000),
    output_format: rules.output_format, target_bytes: rules.target_bytes,
    max_width: rules.max_width, max_height: rules.max_height
  }),
  RenamePreset: async () => ({}),
  DeletePreset: async () => ({}),
  TrayAvailable: async () => false,
  GetAppInfo: async () => ({
    version: '0.1.0', commit: 'a8c502c', platform: 'darwin/arm64',
    data_dir: '/Users/benny/Library/Application Support/ImageWorkshop',
    log_file: '/Users/benny/Library/Application Support/ImageWorkshop/app.log'
  }),
  GetDataDir: async () => '/Users/benny/Library/Application Support/ImageWorkshop',
  OpenDataDir: async () => {},
  ExportDatabase: async () => {}
}}};

window.runtime = {
  OnFileDrop: () => {}, OnFileDropOff: () => {},
  LogInfo: () => {}, LogError: () => {}, LogWarning: () => {}, LogDebug: () => {},
  Show: () => {}, WindowUnminimise: () => {}, WindowMinimise: () => {},
  WindowToggleMaximise: () => {}, WindowIsMaximised: async () => false,
  WindowGetSize: async () => ({ width: 1160, height: 820 }),
  WindowHide: () => {}, Quit: () => {},
  EventsOn: (name, cb) => {
    (eventHandlers[name] = eventHandlers[name] || []).push(cb);
    return () => { eventHandlers[name] = (eventHandlers[name] || []).filter((x) => x !== cb); };
  },
  EventsOnMultiple: (name, cb) => {
    (eventHandlers[name] = eventHandlers[name] || []).push(cb);
    return () => { eventHandlers[name] = (eventHandlers[name] || []).filter((x) => x !== cb); };
  },
  EventsOff: (name) => { delete eventHandlers[name]; }
};
`

async function main() {
  fs.rmSync(FRAME_DIR, { recursive: true, force: true })
  fs.mkdirSync(FRAME_DIR, { recursive: true })
  fs.mkdirSync(OUT_DIR, { recursive: true })

  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({
    viewport: { width: 1160, height: 820 },
    deviceScaleFactor: 2,
  })
  const page = await context.newPage()
  await page.addInitScript({ content: initScript })
  await page.goto('http://127.0.0.1:4173/', { waitUntil: 'networkidle' })
  await page.waitForSelector('.dropzone__title')

  await page.getByRole('button', { name: '添加文件', exact: true }).click()
  await page.waitForSelector('.row')

  // 检查是懒加载的（IntersectionObserver），滚一遍让每一行都被检查到，
  // 否则视口外的行会停在「待检查」，截图里的状态就不完整。
  await page.evaluate(async () => {
    for (const r of [...document.querySelectorAll('.row')]) {
      r.scrollIntoView({ block: 'center' })
      await new Promise((x) => setTimeout(x, 140))
    }
    window.scrollTo(0, 0)
    for (const el of [...document.querySelectorAll('*')]) el.scrollTop = 0
  })

  await page.waitForFunction(
    () => {
      const rows = [...document.querySelectorAll('.row')]
      if (rows.length !== 9) return false
      return rows.every((r) => {
        const s = r.querySelector('.row__status')?.textContent?.trim()
        return s === '可处理' || s === '失败'
      })
    },
    null,
    { timeout: 20000 },
  )

  // 目标 300 KB + 宽高盒 1600 已在「上次设置」里恢复，确认格式后开跑
  await page.selectOption('#output-format', 'jpeg')
  await page.waitForFunction(
    () => {
      const b = [...document.querySelectorAll('button')].find((x) => x.textContent.includes('开始处理'))
      return b && !b.disabled
    },
    null,
    { timeout: 5000 },
  )

  await page.getByRole('button', { name: '开始处理', exact: true }).click()
  let frame = 0
  const deadline = Date.now() + 6000
  while (Date.now() < deadline) {
    await page.screenshot({ path: path.join(FRAME_DIR, `frame-${String(frame).padStart(2, '0')}.png`) })
    frame += 1
    const finished = await page.evaluate(
      () => document.body.innerText.includes('处理完成：') || document.body.innerText.includes('批次完成'),
    )
    if (finished) break
    await page.waitForTimeout(280)
  }

  await page.waitForFunction(() => document.body.innerText.includes('处理完成：'), null, { timeout: 8000 })
  await page.screenshot({ path: `${OUT_DIR}/工作台-处理结果.png` })

  // 单张对比：第一张成功项。断言用弹窗自己的节点——上一版用「页面文字含『预设』」
  // 这种必然成立的判断，弹窗没开也算通过，等于没测。
  await page.locator('.row__compare').first().click()
  await page.waitForSelector('.compare__panel[role="dialog"]', { timeout: 6000 })
  await page.waitForFunction(
    () => {
      const imgs = [...document.querySelectorAll('.compare__body img')]
      return imgs.length === 2 && imgs.every((img) => img.naturalWidth > 0)
    },
    null,
    { timeout: 6000 },
  ).catch(async (e) => {
    console.log('IMG DIAG', JSON.stringify(await page.evaluate(() =>
      [...document.querySelectorAll('.compare__body img')].map((img) => ({
        src: String(img.getAttribute('src') || '').slice(0, 70),
        w: img.naturalWidth,
      }))), null))
    console.log('ERR DIAG', JSON.stringify(await page.evaluate(() =>
      [...document.querySelectorAll('.compare__body, .pane')].map((el) => el.textContent.trim().slice(0, 60)))))
    throw e
  })
  await page.waitForTimeout(400)
  await page.screenshot({ path: `${OUT_DIR}/单张对比.png` })
  await page.locator('.compare__close').click()
  await page.waitForSelector('.compare__panel[role="dialog"]', { state: 'detached', timeout: 4000 })

  // 预设管理
  await page.getByRole('button', { name: '管理预设', exact: true }).click()
  try {
    await page.waitForSelector('.preset-dialog__panel[role="dialog"]', { timeout: 6000 })
  } catch (e) {
    console.log('DIAG', JSON.stringify(await page.evaluate(() => ({
      inert: document.getElementById('app')?.hasAttribute('inert') ?? null,
      anyPresetNode: !!document.querySelector('[class*=preset-dialog]'),
      html: document.body.innerHTML.slice(0, 200),
      buttons: [...document.querySelectorAll('button')].map((b) => b.textContent.trim()).slice(0, 12),
    }))))
    throw e
  }
  await page.waitForFunction(
    () => document.body.innerText.includes('电商主图'),
    null,
    { timeout: 6000 },
  )
  await page.waitForTimeout(400)
  await page.screenshot({ path: `${OUT_DIR}/预设管理.png` })
  await page.locator('.preset-dialog__close').click()
  await page.waitForSelector('.preset-dialog__panel[role="dialog"]', { state: 'detached', timeout: 4000 })

  // 设置页
  await page.goto('http://127.0.0.1:4173/#/settings', { waitUntil: 'networkidle' })
  await page.waitForFunction(
    () => document.querySelector('.page__title')?.textContent?.trim() === '设置',
    null,
    { timeout: 6000 },
  )
  await page.waitForTimeout(500)
  await page.screenshot({ path: `${OUT_DIR}/设置页面.png` })

  console.log(JSON.stringify({ frames: frame, frameDir: FRAME_DIR }, null, 2))
  await browser.close()
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
