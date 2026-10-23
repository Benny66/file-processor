const { chromium } = require('/Users/bennywen/.npm/_npx/6bcb61ec6d5aea22/node_modules/playwright')

const initScript = `
function svgThumb(fill, transparent) {
  const rect = transparent ? '' : '<rect width="96" height="54" fill="' + fill + '"/>';
  const svg = '<svg xmlns="http://www.w3.org/2000/svg" width="96" height="54">' + rect +
    '<circle cx="48" cy="27" r="16" fill="' + fill + '" opacity="0.85"/></svg>';
  return 'data:image/svg+xml,' + encodeURIComponent(svg);
}
const eventHandlers = {};
function emitEvent(name, payload) {
  for (const cb of [...(eventHandlers[name] || [])]) cb(payload);
}
window.go = { main: { App: {
  GetTheme: async () => 'dark',
  SupportedFormatHints: async () => ['JPG', 'PNG', 'WebP', 'BMP', 'TIFF', 'GIF（静态）'],
  // 真实判定在 Go（internal/service/batch_service.go 的 validateBatchStart）；
  // 这里只是替身，让假绑定能给出与之一致的【结果】，不在前端复制规则。
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
  SetTheme: async () => {},
  ScanPaths: async () => ({
    files: [
      { path: '/tmp/示例 图.jpg', name: '示例 图.jpg', size: 2456789 },
      { path: '/tmp/产品图.png', name: '产品图.png', size: 98765 },
      { path: '/tmp/banner.webp', name: 'banner.webp', size: 456789 },
      { path: '/tmp/伪装.jpg', name: '伪装.jpg', size: 123456 }
    ],
    rejected: [
      { path: '/tmp/说明.txt', reason: '不支持的格式' },
      { path: '/tmp/重复.jpg', reason: '已在队列中' }
    ]
  }),
  SelectFiles: async () => ['/tmp/示例 图.jpg','/tmp/产品图.png','/tmp/banner.webp','/tmp/伪装.jpg','/tmp/说明.txt','/tmp/重复.jpg'],
  SelectDirectory: async () => '/tmp',
  InspectImage: async (path) => {
    await new Promise((r) => setTimeout(r, 120));
    if (String(path).includes('banner.webp')) {
      throw new Error(JSON.stringify({ code: 'validation', message: '首版不支持动图' }));
    }
    if (String(path).includes('伪装.jpg')) {
      return { path, format: 'png', width: 800, height: 600, bytes: 123456, thumbnail: svgThumb('#30a46c', false), warning: '扩展名与实际格式不一致（.JPG → PNG）' };
    }
    if (String(path).includes('产品图.png')) {
      return { path, format: 'png', width: 800, height: 600, bytes: 98765, thumbnail: svgThumb('#5b8def', true), warning: '' };
    }
    return { path, format: 'jpeg', width: 1920, height: 1080, bytes: 2456789, thumbnail: svgThumb('#5b8def', false), warning: '' };
  },
  ProcessBatch: async (req) => {
    const paths = req.paths || [];
    const target = req.rules?.target_bytes || 0;
    const format = req.rules?.output_format || 'keep';
    const meta = {
      '/tmp/示例 图.jpg': { source: 2456789, output: 90000, width: 800, height: 450, format: 'jpeg', extreme: true },
      '/tmp/产品图.png': { source: 98765, output: 50000, width: 800, height: 600, format: 'png', extreme: false },
      '/tmp/伪装.jpg': { source: 123456, output: 95000, width: 800, height: 600, format: 'png', extreme: false }
    };
    const items = [];
    let success = 0, failed = 0, skipped = 0, extreme = 0, saved = 0;
    for (let i = 0; i < paths.length; i++) {
      await new Promise((r) => setTimeout(r, 180));
      const path = paths[i];
      const m = meta[path];
      const outFormat = format === 'keep' ? m.format : format;
      let item;
      if (target > 0 && m.source <= target && outFormat === m.format) {
        item = { path, output_path: '', status: 'skipped', source_bytes: m.source, output_bytes: 0, width: m.width, height: m.height, extreme_degraded: false, message: '已达标' };
        skipped++;
      } else {
        const ext = outFormat === 'jpeg' ? 'jpg' : outFormat;
        item = {
          path,
          output_path: String(path).replace(/(\\.[^.]+)$/, '-out.' + ext),
          status: 'success',
          source_bytes: m.source,
          output_bytes: m.output,
          width: m.width,
          height: m.height,
          extreme_degraded: m.extreme,
          message: ''
        };
        success++;
        if (m.extreme) extreme++;
        saved += m.source - m.output;
      }
      items.push(item);
      emitEvent('image:progress', { done: i + 1, total: paths.length, current: item });
    }
    return { ok: failed === 0, message: '', items, success, failed, skipped, canceled: 0, extreme_degraded: extreme, saved_bytes: saved };
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
      if (rows.length !== 4) return false
      return rows.every((row) => {
        const status = row.querySelector('.row__status')?.textContent?.trim()
        return status === '可处理' || status === '失败'
      })
    },
    null,
    { timeout: 5000 },
  )

  await page.locator('.checkbox').nth(1).check()
  await page.fill('#target-value', '0')
  await page.waitForTimeout(50)
  const invalidState = await page.evaluate(() => ({
    startDisabled: [...document.querySelectorAll('button')].find((b) => b.textContent.includes('开始处理'))?.disabled ?? null,
    hint: [...document.querySelectorAll('.rules__hint')].map((el) => el.textContent?.trim()).join(' | '),
  }))
  await page.fill('#target-value', '100')
  await page.waitForFunction(
    () => {
      const btn = [...document.querySelectorAll('button')].find((b) => b.textContent.includes('开始处理'))
      return btn && !btn.disabled
    },
    null,
    { timeout: 2000 },
  )

  await page.getByRole('button', { name: '开始处理', exact: true }).click()
  await page.waitForTimeout(300)
  const progressState = await page.evaluate(() => ({
    progress: document.querySelector('.progress') !== null,
    progressWidth: document.querySelector('.progress__bar')?.getAttribute('style') ?? null,
    rulesProgress: document.querySelector('.rules__progress')?.textContent ?? null,
    targetDisabled: document.querySelector('#target-value')?.hasAttribute('disabled') ?? null,
  }))
  await page.screenshot({ path: '.scratch/image-workshop/evidence/imageworkshop-target-progress-1280.png' })

  await page.waitForFunction(() => document.body.innerText.includes('处理完成：'), null, { timeout: 5000 })
  const finalState = await page.evaluate(() => ({
    summary: [...document.querySelectorAll('.notice__text')]
      .map((el) => el.textContent)
      .find((text) => text?.includes('处理完成：')) ?? null,
    rows: [...document.querySelectorAll('.row')].map((row) => ({
      name: row.querySelector('.row__name')?.textContent ?? null,
      status: row.querySelector('.row__status')?.textContent?.trim() ?? null,
      statusClass: row.querySelector('.row__status')?.className ?? null,
      extreme: row.querySelector('.row__extreme')?.textContent?.trim() ?? null,
      dims: row.querySelector('.row__dims')?.textContent ?? null,
      output: row.querySelector('.row__output')?.textContent?.trim() ?? null,
      error: row.querySelector('.row__error')?.textContent ?? null,
      warning: row.querySelector('.row__warning')?.textContent ?? null,
    })),
    progressVisible: document.querySelector('.progress') !== null,
    targetChecked: document.querySelectorAll('.checkbox')[1]?.checked ?? null,
    targetValue: document.querySelector('#target-value')?.value ?? null,
    targetUnit: document.querySelector('#target-unit')?.value ?? null,
    startLabel: [...document.querySelectorAll('button')]
      .map((b) => b.textContent.trim())
      .find((text) => text.includes('开始处理')) ?? null,
  }))
  await page.screenshot({ path: '.scratch/image-workshop/evidence/imageworkshop-target-1280.png' })

  console.log(JSON.stringify({ invalidState, progressState, finalState }, null, 2))
  await browser.close()
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
