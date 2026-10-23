const { chromium } = require('/Users/bennywen/.npm/_npx/6bcb61ec6d5aea22/node_modules/playwright')

const initScript = `
function svgThumb(fill, transparent) {
  const rect = transparent ? '' : '<rect width="96" height="54" fill="' + fill + '"/>';
  const svg = '<svg xmlns="http://www.w3.org/2000/svg" width="96" height="54">' + rect +
    '<circle cx="48" cy="27" r="16" fill="' + fill + '" opacity="0.85"/></svg>';
  return 'data:image/svg+xml,' + encodeURIComponent(svg);
}
window.go = { main: { App: {
  GetTheme: async () => 'dark',
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
  EventsOn: () => () => {}
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

  const inspectState = await page.evaluate(() => ({
    rows: [...document.querySelectorAll('.row')].map((row) => ({
      name: row.querySelector('.row__name')?.textContent ?? null,
      status: row.querySelector('.row__status')?.textContent?.trim() ?? null,
      dims: row.querySelector('.row__dims')?.textContent ?? null,
      format: row.querySelector('.row__format')?.textContent ?? null,
      warning: row.querySelector('.row__warning')?.textContent ?? null,
      error: row.querySelector('.row__error')?.textContent ?? null,
      thumb: row.querySelector('.thumb img')?.getAttribute('src') ?? null,
      thumbPending: row.querySelector('.thumb')?.classList.contains('thumb--pending') ?? false,
      thumbFailed: row.querySelector('.thumb')?.classList.contains('thumb--failed') ?? false,
    })),
    notice: document.querySelector('.notice__text')?.textContent ?? null,
    count: document.querySelector('.workbench__count')?.textContent ?? null,
  }))

  await page.screenshot({ path: '.scratch/image-workshop/evidence/imageworkshop-inspect-1280.png' })

  const failedRow = page.locator('.row').filter({ hasText: 'banner.webp' })
  await failedRow.getByRole('button', { name: '移除' }).click()
  await page.waitForTimeout(50)
  const afterRemove = await page.evaluate(() => ({
    rows: document.querySelectorAll('.row').length,
    count: document.querySelector('.workbench__count')?.textContent ?? null,
  }))

  await page.getByRole('button', { name: '清空' }).click()
  await page.waitForTimeout(50)
  const afterClear = await page.evaluate(() => ({
    rows: document.querySelectorAll('.row').length,
    dropzone: document.querySelector('.dropzone__title')?.textContent ?? null,
    notice: document.querySelector('.notice')?.textContent ?? null,
  }))

  console.log(JSON.stringify({ inspectState, afterRemove, afterClear }, null, 2))
  await browser.close()
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
