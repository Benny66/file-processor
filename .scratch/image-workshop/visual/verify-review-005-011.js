// verify-review-005-011.js — 005→011 累计审查修复项的真实渲染验证
//
// 覆盖本次审查修掉的四条：
//   US-14 逐项状态机（排队 → 处理中 → 结果，处理中仍可对比）
//   US-20 透明铺白的队列提示
//   US-27 节省比例（单项 + 批次总计）
//   规则校验单一真相（提示文案由 Go 给出，含前端曾经缺失的「宽高需同时设置」）
//
// 用法：先 `cd frontend && npm run build`，再起 `npx vite preview --host 127.0.0.1 --port 4173`，
// 然后 `node .scratch/image-workshop/visual/verify-review-005-011.js`。
const { chromium } = require('/Users/bennywen/.npm/_npx/6bcb61ec6d5aea22/node_modules/playwright')

const initScript = `
const eventHandlers = {};
function emitEvent(name, payload) {
  for (const cb of [...(eventHandlers[name] || [])]) cb(payload);
}
function svgThumb(fill) {
  const svg = '<svg xmlns="http://www.w3.org/2000/svg" width="96" height="54">' +
    '<rect width="96" height="54" fill="' + fill + '"/></svg>';
  return 'data:image/svg+xml,' + encodeURIComponent(svg);
}
window.go = { main: { App: {
  GetTheme: async () => 'dark',
  SetTheme: async () => {},
  // 清单来自 Go；这里给的是 imageproc.SupportedFormatHints() 的替身。
  SupportedFormatHints: async () => ['JPG', 'PNG', 'WebP', 'BMP', 'TIFF', 'GIF（静态）'],
  // 真实判定在 internal/service/batch_service.go 的 validateBatchStart/validateBatchRules。
  ValidateImageRules: async (rules) => {
    const t = (rules && rules.target_bytes) || 0;
    const w = (rules && rules.max_width) || 0;
    const h = (rules && rules.max_height) || 0;
    const fmt = (rules && rules.output_format) || 'keep';
    if (t !== 0 && (t < 1024 || t > 1073741824)) return '目标体积需在 1 KB–1 GB';
    if ((w === 0) !== (h === 0)) return '宽高需同时设置';
    if (w !== 0 && (w > 65535 || h > 65535)) return '宽高需在 1–65535 px';
    if (fmt === 'keep' && t === 0 && w === 0) return '请选择输出格式，或开启目标体积/尺寸限制';
    return '';
  },
  ScanPaths: async () => ({
    files: [
      { path: '/tmp/透明 图标.png', name: '透明 图标.png', size: 98765 },
      { path: '/tmp/照片.jpg', name: '照片.jpg', size: 2456789 }
    ],
    rejected: []
  }),
  SelectFiles: async () => ['/tmp/透明 图标.png', '/tmp/照片.jpg'],
  SelectDirectory: async () => '/tmp',
  InspectImage: async (path) => ({
    path,
    format: String(path).endsWith('.png') ? 'png' : 'jpeg',
    width: 800,
    height: 600,
    bytes: 98765,
    thumbnail: svgThumb('#5b8def'),
    warning: ''
  }),
  // 关键：worker 领取任务时先播 started（US-14 的「处理中」唯一来源），
  // 完成时播 current；透明图转 JPEG 带 alpha_flattened（US-20）。
  ProcessBatch: async (req) => {
    const paths = req.paths || [];
    const items = [];
    let saved = 0;
    for (let i = 0; i < paths.length; i++) {
      const path = paths[i];
      emitEvent('image:progress', {
        done: i, total: paths.length, started: { path, status: 'processing' }
      });
      await new Promise((r) => setTimeout(r, 700));
      const isPng = String(path).endsWith('.png');
      const item = {
        path,
        output_path: '/tmp/out' + (i + 1) + '.jpg',
        status: 'success',
        source_bytes: isPng ? 98765 : 2456789,
        output_bytes: isPng ? 45000 : 1351234,
        width: 800,
        height: 600,
        alpha_flattened: isPng,
        extreme_degraded: false,
        message: ''
      };
      items.push(item);
      saved += item.source_bytes - item.output_bytes;
      emitEvent('image:progress', { done: i + 1, total: paths.length, current: item });
    }
    return {
      ok: true, message: '', items,
      success: items.length, failed: 0, skipped: 0, canceled: 0,
      extreme_degraded: 0, saved_bytes: saved
    };
  },
  CancelBatch: async () => {},
  GetImageSettings: async () => ({ output_format: 'keep', target_bytes: 0, max_width: 0, max_height: 0 }),
  SaveImageSettings: async () => {},
  ListPresets: async () => ({ list: [], total: 0, page: 1, page_size: 200 }),
  TrayAvailable: async () => false,
  GetAppInfo: async () => ({
    version: '0.1.0', commit: '', platform: 'darwin/arm64', data_dir: '/tmp', log_file: '/tmp/app.log'
  }),
  GetDataDir: async () => '/tmp',
  OpenDataDir: async () => {},
  ExportDatabase: async () => {},
  SelectSaveFile: async () => '',
  Confirm: async () => true,
  GetLaunchArgs: async () => [],
  SelectFile: async () => ''
}}};
window.runtime = {
  OnFileDrop: () => {},
  OnFileDropOff: () => {},
  LogInfo: () => {},
  LogError: () => {},
  LogWarning: () => {},
  LogDebug: () => {},
  Show: () => {},
  WindowUnminimise: () => {},
  WindowMinimise: () => {},
  WindowToggleMaximise: () => {},
  WindowIsMaximised: async () => false,
  WindowGetSize: async () => ({ width: 1280, height: 900 }),
  WindowHide: () => {},
  Quit: () => {},
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

function statuses(page) {
  return page.evaluate(() =>
    [...document.querySelectorAll('.row')].map((row) => ({
      name: row.querySelector('.row__name')?.textContent ?? null,
      status: row.querySelector('.row__status')?.textContent?.trim() ?? null,
      cls: row.querySelector('.row__status')?.className ?? null,
      output: row.querySelector('.row__output')?.textContent?.trim() ?? null,
      warning: row.querySelector('.row__warning')?.textContent?.trim() ?? null,
      compareDisabled: row.querySelector('.row__compare')?.disabled ?? null,
    }))
  )
}

async function main() {
  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } })
  const page = await context.newPage()
  await page.addInitScript({ content: initScript })
  await page.goto('http://127.0.0.1:4173/', { waitUntil: 'networkidle' })
  await page.waitForSelector('.dropzone__title')

  const checks = {}

  // 1) 导入提示的格式清单取自 Go
  checks.dropzoneHint = await page.textContent('.dropzone__hint')

  // 2) 规则提示来自 Go —— 只填宽不填高，正是前端曾经复制逻辑时漏掉的那条
  await page.getByRole('button', { name: '添加文件', exact: true }).click()
  await page.waitForSelector('.row')
  await page.getByLabel('限制尺寸').check()
  await page.fill('#max-width', '800')
  await page.fill('#max-height', '')
  await page.waitForFunction(
    () => document.querySelector('.rules__hint')?.textContent?.includes('宽高需同时设置'),
    null,
    { timeout: 3000 },
  )
  checks.boxAsymmetryHint = await page.textContent('.rules__hint')
  checks.startDisabledOnBadRules = await page.evaluate(() => {
    const btn = [...document.querySelectorAll('button')].find((b) => b.textContent.includes('开始处理'))
    return btn ? btn.disabled : null
  })

  // 补齐高度，改输出格式为 JPEG，开始处理
  await page.fill('#max-height', '600')
  await page.selectOption('#output-format', 'jpeg')
  await page.waitForFunction(
    () => {
      const btn = [...document.querySelectorAll('button')].find((b) => b.textContent.includes('开始处理'))
      return btn && !btn.disabled
    },
    null,
    { timeout: 3000 },
  )
  await page.getByRole('button', { name: '开始处理', exact: true }).click()

  // 3) US-14：开工瞬间应出现「处理中」，未领取的应显示「排队」；
  //    第一项成功后（批次仍在跑）对比按钮必须可用——规格只禁止编辑队列。
  await page.waitForTimeout(900)
  checks.midFlight = await statuses(page)
  checks.midFlightProgress = await page.textContent('.rules__progress')
  await page.screenshot({ path: '.scratch/image-workshop/evidence/imageworkshop-review-states-1280.png' })

  // 4) 收尾：US-20 铺白提示 + US-27 节省比例
  await page.waitForFunction(() => document.body.innerText.includes('处理完成：'), null, { timeout: 8000 })
  checks.finalRows = await statuses(page)
  checks.summary = await page.evaluate(
    () =>
      [...document.querySelectorAll('.notice__text')]
        .map((el) => el.textContent)
        .find((text) => text?.includes('处理完成：')) ?? null,
  )
  await page.screenshot({ path: '.scratch/image-workshop/evidence/imageworkshop-review-final-1280.png' })

  console.log(JSON.stringify(checks, null, 2))
  await browser.close()
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
