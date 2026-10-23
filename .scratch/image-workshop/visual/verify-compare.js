const { chromium } = require('/Users/bennywen/.npm/_npx/6bcb61ec6d5aea22/node_modules/playwright')

const initScript = `
function svgThumb(fill, transparent) {
  const rect = transparent ? '' : '<rect width="96" height="54" fill="' + fill + '"/>';
  const svg = '<svg xmlns="http://www.w3.org/2000/svg" width="96" height="54">' + rect +
    '<circle cx="48" cy="27" r="16" fill="' + fill + '" opacity="0.85"/></svg>';
  return 'data:image/svg+xml,' + encodeURIComponent(svg);
}
function preview(fill, alpha, width, height) {
  const svg = '<svg xmlns="http://www.w3.org/2000/svg" width="' + width + '" height="' + height + '">' +
    '<rect x="0" y="0" width="' + width + '" height="' + height + '" fill="' + fill + '" opacity="' + alpha + '"/>' +
    '<circle cx="' + (width / 2) + '" cy="' + (height / 2) + '" r="' + (height / 4) + '" fill="' + fill + '"/>' +
    '<text x="24" y="48" font-size="28" fill="#ffffff">IMAGE</text></svg>';
  return 'data:image/svg+xml,' + encodeURIComponent(svg);
}
const eventHandlers = {};
function emitEvent(name, payload) {
  for (const cb of [...(eventHandlers[name] || [])]) cb(payload);
}
window.go = { main: { App: {
  GetTheme: async () => 'dark',
  SetTheme: async () => {},
  ScanPaths: async () => ({
    files: [
      { path: '/tmp/示例 图.jpg', name: '示例 图.jpg', size: 2456789 },
      { path: '/tmp/产品图.png', name: '产品图.png', size: 98765 }
    ],
    rejected: []
  }),
  SelectFiles: async () => ['/tmp/示例 图.jpg', '/tmp/产品图.png'],
  SelectDirectory: async () => '/tmp',
  InspectImage: async (path) => {
    await new Promise((r) => setTimeout(r, 80));
    if (String(path).includes('产品图.png')) {
      return { path, format: 'png', width: 800, height: 600, bytes: 98765, thumbnail: svgThumb('#30a46c', true), warning: '' };
    }
    return { path, format: 'jpeg', width: 1920, height: 1080, bytes: 2456789, thumbnail: svgThumb('#5b8def', false), warning: '' };
  },
  RenderPreview: async (path, maxEdge) => {
    await new Promise((r) => setTimeout(r, 90));
    if (String(path).includes('产品图')) {
      return { data_url: preview('#30a46c', '0.35', 800, 600), width: 800, height: 600, has_alpha: true };
    }
    return { data_url: preview('#5b8def', '1', 1024, 576), width: 1024, height: 576, has_alpha: false };
  },
  ProcessBatch: async (req) => {
    const paths = req.paths || [];
    const format = req.rules?.output_format || 'keep';
    const meta = {
      '/tmp/示例 图.jpg': { source: 2456789, output: 90000, width: 800, height: 450, format: 'jpeg' },
      '/tmp/产品图.png': { source: 98765, output: 50000, width: 800, height: 600, format: 'png' }
    };
    const items = [];
    let saved = 0;
    for (let i = 0; i < paths.length; i++) {
      await new Promise((r) => setTimeout(r, 150));
      const path = paths[i];
      const m = meta[path];
      const outFormat = format === 'keep' ? m.format : format;
      const ext = outFormat === 'jpeg' ? 'jpg' : outFormat;
      const item = {
        path,
        output_path: String(path).replace(/(\\.[^.]+)$/, '-out.' + ext),
        status: 'success',
        source_bytes: m.source,
        output_bytes: m.output,
        width: m.width,
        height: m.height,
        extreme_degraded: false,
        message: ''
      };
      saved += m.source - m.output;
      items.push(item);
      emitEvent('image:progress', { done: i + 1, total: paths.length, current: item });
    }
    return { ok: true, message: '', items, success: items.length, failed: 0, skipped: 0, canceled: 0, extreme_degraded: 0, saved_bytes: saved };
  },
  CancelBatch: async () => {},
  TrayAvailable: async () => true,
  GetAppInfo: async () => ({ version: '0.1.0', commit: '', platform: 'darwin/arm64', data_dir: '/tmp', log_file: '/tmp/app.log' }),
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
  WindowMinimise: () => {},
  WindowToggleMaximise: () => {},
  WindowIsMaximised: async () => false,
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

async function main() {
  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } })
  const page = await context.newPage()
  await page.addInitScript({ content: initScript })
  await page.goto('http://127.0.0.1:4173/', { waitUntil: 'networkidle' })
  await page.waitForSelector('.dropzone__title')

  await page.getByRole('button', { name: '添加文件', exact: true }).click()
  await page.waitForSelector('.row')
  await page.waitForFunction(
    () => {
      const rows = [...document.querySelectorAll('.row')]
      return rows.length === 2 && rows.every((row) => row.querySelector('.row__status')?.textContent?.trim() === '可处理')
    },
    null,
    { timeout: 5000 },
  )

  await page.locator('.checkbox').nth(1).check()
  await page.fill('#target-value', '100')
  await page.getByRole('button', { name: '开始处理', exact: true }).click()
  await page.waitForFunction(() => document.body.innerText.includes('处理完成：'), null, { timeout: 5000 })

  const compareButtons = await page.getByRole('button', { name: '对比', exact: true }).count()
  await page.locator('.row').nth(1).getByRole('button', { name: '对比', exact: true }).click()
  await page.waitForSelector('.compare__panel')
  await page.waitForFunction(() => document.querySelectorAll('.pane__stage img').length === 2, null, { timeout: 5000 })
  await page.waitForTimeout(120)

  const compareState = await page.evaluate(() => {
    const panel = document.querySelector('.compare__panel')
    const stages = [...document.querySelectorAll('.pane__stage')]
    return {
      dialogRole: panel?.getAttribute('role') ?? null,
      ariaModal: panel?.getAttribute('aria-modal') ?? null,
      title: document.querySelector('.compare__title')?.textContent?.trim() ?? null,
      name: document.querySelector('.compare__name')?.textContent?.trim() ?? null,
      labels: [...document.querySelectorAll('.pane__label')].map((el) => el.textContent?.trim()),
      metas: [...document.querySelectorAll('.pane__meta')].map((el) => el.textContent?.trim()),
      alts: [...document.querySelectorAll('.pane__stage img')].map((img) => img.getAttribute('alt')),
      srcPrefixes: [...document.querySelectorAll('.pane__stage img')].map((img) => img.getAttribute('src')?.slice(0, 24)),
      checkerboard: stages.map((stage) => getComputedStyle(stage).backgroundImage.includes('linear-gradient')),
      hint: document.querySelector('.compare__hint')?.textContent?.trim() ?? null,
      appInert: document.getElementById('app')?.hasAttribute('inert') ?? null,
      focusedClass: document.activeElement?.className ?? null,
    }
  })
  await page.screenshot({ path: '.scratch/image-workshop/evidence/imageworkshop-compare-1280.png' })

  await page.keyboard.press('Escape')
  await page.waitForFunction(() => !document.querySelector('.compare__panel'), null, { timeout: 2000 })
  const closedByEscape = (await page.locator('.compare__panel').count()) === 0
  const closeState = await page.evaluate(() => ({
    appInert: document.getElementById('app')?.hasAttribute('inert') ?? null,
    focusedText: document.activeElement?.textContent?.trim() ?? null,
  }))

  await page.fill('#target-value', '200')
  await page.waitForSelector('.notice--stale')
  const staleState = await page.evaluate(() => ({
    notice: document.querySelector('.notice--stale .notice__text')?.textContent?.trim() ?? null,
    badges: [...document.querySelectorAll('.row__stale')].map((el) => el.textContent?.trim()),
    rows: document.querySelectorAll('.row').length,
  }))
  await page.screenshot({ path: '.scratch/image-workshop/evidence/imageworkshop-stale-1280.png' })

  await page.getByRole('button', { name: '开始处理', exact: true }).click()
  await page.waitForFunction(
    () => {
      const text = document.body.innerText
      return text.includes('处理完成：') && !document.querySelector('.notice--stale')
    },
    null,
    { timeout: 5000 },
  )
  const refreshedState = await page.evaluate(() => ({
    staleNotice: document.querySelector('.notice--stale') !== null,
    staleBadges: document.querySelectorAll('.row__stale').length,
    compareButtons: document.querySelectorAll('.row__compare').length,
    summary: [...document.querySelectorAll('.notice__text')]
      .map((el) => el.textContent)
      .find((text) => text?.includes('处理完成：')) ?? null,
  }))

  console.log(
    JSON.stringify({ compareButtons, compareState, closedByEscape, closeState, staleState, refreshedState }, null, 2),
  )
  await browser.close()
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
