const { chromium } = require('/Users/bennywen/.npm/_npx/6bcb61ec6d5aea22/node_modules/playwright')

const initScript = `
window.go = {
  main: {
    App: {
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
  TrayAvailable: async () => true,
      SetTheme: async () => {},
      ScanPaths: async () => ({
        files: [
          { path: '/tmp/示例 图.jpg', name: '示例 图.jpg', size: 2456789 },
          { path: '/tmp/产品图.png', name: '产品图.png', size: 98765 },
          { path: '/tmp/banner.webp', name: 'banner.webp', size: 456789 }
        ],
        rejected: [
          { path: '/tmp/说明.txt', reason: '不支持的格式' },
          { path: '/tmp/重复.jpg', reason: '已在队列中' }
        ]
      }),
      SelectFiles: async () => ['/tmp/示例 图.jpg', '/tmp/产品图.png', '/tmp/banner.webp', '/tmp/说明.txt', '/tmp/重复.jpg'],
      SelectDirectory: async () => '/tmp',
      GetAppInfo: async () => ({ version: '0.1.0', commit: '', platform: 'darwin/arm64', data_dir: '/tmp', log_file: '/tmp/app.log' }),
      GetDataDir: async () => '/tmp',
      OpenDataDir: async () => {},
      ExportDatabase: async () => {},
      SelectSaveFile: async () => '',
      Confirm: async () => true,
      GetLaunchArgs: async () => [],
      SelectFile: async () => ''
    }
  }
};
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

  const initial = await page.evaluate(() => ({
    title: document.title,
    theme: document.documentElement.getAttribute('data-theme'),
    dropzone: document.querySelector('.dropzone__title')?.textContent ?? null,
    buttons: [...document.querySelectorAll('button')].map((b) => b.textContent.trim()),
  }))

  await page.getByRole('button', { name: '添加文件', exact: true }).click()
  await page.waitForSelector('.queue__row')
  await page.waitForTimeout(100)

  const queue = await page.evaluate(() => ({
    rows: [...document.querySelectorAll('.queue__row')].map((row) => ({
      name: row.querySelector('.queue__name')?.textContent ?? null,
      path: row.querySelector('.queue__path')?.textContent ?? null,
      size: row.querySelector('.queue__size')?.textContent ?? null,
      status: row.querySelector('.queue__status')?.textContent ?? null,
    })),
    count: document.querySelector('.workbench__count')?.textContent ?? null,
    notice: document.querySelector('.notice__text')?.textContent ?? null,
    noticeTitle: document.querySelector('.notice')?.getAttribute('title') ?? null,
    error: document.querySelector('.workbench__error')?.textContent ?? null,
  }))

  await page.screenshot({ path: '.scratch/image-workshop/evidence/imageworkshop-queue-1280.png' })

  await page.locator('.queue__remove').first().click()
  await page.waitForTimeout(50)
  const afterRemove = await page.evaluate(() => ({
    rows: document.querySelectorAll('.queue__row').length,
    count: document.querySelector('.workbench__count')?.textContent ?? null,
  }))

  await page.getByRole('button', { name: '清空' }).click()
  await page.waitForTimeout(50)
  const afterClear = await page.evaluate(() => ({
    rows: document.querySelectorAll('.queue__row').length,
    dropzone: document.querySelector('.dropzone__title')?.textContent ?? null,
    count: document.querySelector('.workbench__count')?.textContent ?? null,
    notice: document.querySelector('.notice')?.textContent ?? null,
  }))

  console.log(JSON.stringify({ initial, queue, afterRemove, afterClear }, null, 2))
  await browser.close()
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
