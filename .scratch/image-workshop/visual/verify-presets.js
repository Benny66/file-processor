const { chromium } = require('/Users/bennywen/.npm/_npx/6bcb61ec6d5aea22/node_modules/playwright')

const initScript = `
const STORAGE_KEY = 'iw-preset-test-state';
function defaultState() {
  return {
    settings: { output_format: 'webp', target_bytes: 300 * 1024, max_width: 1600, max_height: 1200 },
    presets: [
      { id: 1, name: '电商主图', output_format: 'jpeg', target_bytes: 100 * 1024, max_width: 0, max_height: 0, created_at: 1, updated_at: 1 },
      { id: 2, name: '高清存档', output_format: 'png', target_bytes: 0, max_width: 4000, max_height: 3000, created_at: 1, updated_at: 1 }
    ],
    nextId: 3,
    settingsSaves: 0,
    confirmCalls: 0
  };
}
function loadState() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw);
  } catch {}
  return defaultState();
}
const state = loadState();
function persist() { localStorage.setItem(STORAGE_KEY, JSON.stringify(state)); }
function throwErr(code, message) {
  throw new Error(JSON.stringify({ code, message }));
}
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
  SetTheme: async () => {},
  ScanPaths: async (paths, skip) => ({
    files: [
      { path: '/tmp/示例 图.jpg', name: '示例 图.jpg', size: 2456789 },
      { path: '/tmp/产品图.png', name: '产品图.png', size: 98765 }
    ].filter((file) => !(skip || []).includes(file.path)),
    rejected: []
  }),
  SelectFiles: async () => ['/tmp/示例 图.jpg', '/tmp/产品图.png'],
  SelectDirectory: async () => '/tmp',
  InspectImage: async (path) => {
    await new Promise((r) => setTimeout(r, 60));
    if (String(path).includes('产品图.png')) {
      return { path, format: 'png', width: 800, height: 600, bytes: 98765, thumbnail: svgThumb('#30a46c', true), warning: '' };
    }
    return { path, format: 'jpeg', width: 1920, height: 1080, bytes: 2456789, thumbnail: svgThumb('#5b8def', false), warning: '' };
  },
  RenderPreview: async () => ({ data_url: '', width: 1, height: 1, has_alpha: false }),
  ProcessBatch: async (req) => {
    const paths = req.paths || [];
    const rules = req.rules || {};
    const meta = {
      '/tmp/示例 图.jpg': { source: 2456789, output: 90000, width: 1920, height: 1080, format: 'jpeg' },
      '/tmp/产品图.png': { source: 98765, output: 50000, width: 800, height: 600, format: 'png' }
    };
    const items = [];
    let saved = 0;
    for (let i = 0; i < paths.length; i++) {
      await new Promise((r) => setTimeout(r, 120));
      const path = paths[i];
      const m = meta[path];
      let width = m.width;
      let height = m.height;
      if (rules.max_width > 0 && rules.max_height > 0) {
        const scale = Math.min(1, rules.max_width / width, rules.max_height / height);
        width = Math.max(1, Math.round(width * scale));
        height = Math.max(1, Math.round(height * scale));
      }
      const outFormat = rules.output_format === 'keep' ? m.format : rules.output_format;
      const ext = outFormat === 'jpeg' ? 'jpg' : outFormat;
      const item = {
        path,
        output_path: String(path).replace(/(\\.[^.]+)$/, '-out.' + ext),
        status: 'success',
        source_bytes: m.source,
        output_bytes: m.output,
        width,
        height,
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
  ListPresets: async (req) => {
    const page = Math.max(1, req?.page || 1);
    const size = Math.max(1, Math.min(200, req?.page_size || 20));
    const sorted = [...state.presets].sort((a, b) => a.name.localeCompare(b.name, 'zh'));
    return {
      list: sorted.slice((page - 1) * size, page * size),
      total: sorted.length,
      page,
      page_size: size
    };
  },
  SavePreset: async (preset) => {
    const name = String(preset.name || '').trim();
    if (!name) throwErr('validation', '请输入预设名称');
    if (state.presets.some((item) => item.name === name && item.id !== preset.id)) {
      throwErr('validation', '预设名称已存在');
    }
    const fields = {
      name,
      output_format: preset.output_format,
      target_bytes: preset.target_bytes,
      max_width: preset.max_width,
      max_height: preset.max_height
    };
    if (preset.id) {
      const existing = state.presets.find((item) => item.id === preset.id);
      if (!existing) throwErr('not_found', '预设不存在');
      Object.assign(existing, fields, { updated_at: Date.now() });
      persist();
      return { ...existing };
    }
    const created = { id: state.nextId++, ...fields, created_at: Date.now(), updated_at: Date.now() };
    state.presets.push(created);
    persist();
    return { ...created };
  },
  DeletePreset: async (id) => {
    const index = state.presets.findIndex((item) => item.id === id);
    if (index < 0) throwErr('not_found', '预设不存在');
    state.presets.splice(index, 1);
    persist();
  },
  GetImageSettings: async () => ({ ...state.settings }),
  SaveImageSettings: async (rules) => {
    state.settings = {
      output_format: rules.output_format,
      target_bytes: rules.target_bytes,
      max_width: rules.max_width,
      max_height: rules.max_height
    };
    state.settingsSaves += 1;
    persist();
  },
  Confirm: async () => {
    state.confirmCalls += 1;
    persist();
    return true;
  },
  GetAppInfo: async () => ({ version: '0.1.0', commit: '', platform: 'darwin/arm64', data_dir: '/tmp', log_file: '/tmp/app.log' }),
  GetDataDir: async () => '/tmp',
  OpenDataDir: async () => {},
  ExportDatabase: async () => {},
  SelectSaveFile: async () => '',
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

async function readRules(page) {
  return page.evaluate(() => {
    const checks = [...document.querySelectorAll('.checkbox')]
    return {
      format: document.querySelector('#output-format')?.value ?? null,
      boxEnabled: checks[0]?.checked ?? null,
      width: document.querySelector('#max-width')?.value ?? null,
      height: document.querySelector('#max-height')?.value ?? null,
      targetEnabled: checks[1]?.checked ?? null,
      targetValue: document.querySelector('#target-value')?.value ?? null,
      targetUnit: document.querySelector('#target-unit')?.value ?? null,
    }
  })
}

async function importFiles(page) {
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
}

async function readStoredState(page) {
  return page.evaluate(() => JSON.parse(localStorage.getItem('iw-preset-test-state')))
}

async function main() {
  const browser = await chromium.launch({ headless: true })
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } })
  const page = await context.newPage()
  await page.addInitScript({ content: initScript })
  await page.goto('http://127.0.0.1:4173/', { waitUntil: 'networkidle' })
  await page.waitForSelector('.presets')

  await page.waitForFunction(
    () => document.querySelectorAll('#preset-select option').length === 3,
    null,
    { timeout: 5000 },
  )
  const initialOptions = await page.evaluate(() =>
    [...document.querySelectorAll('#preset-select option')].map((option) => option.textContent.trim()),
  )

  await importFiles(page)
  const restored = await readRules(page)
  await page.screenshot({ path: '.scratch/image-workshop/evidence/imageworkshop-presets-restored-1280.png' })

  await page.selectOption('#preset-select', '1')
  await page.waitForFunction(() => document.querySelector('#output-format')?.value === 'jpeg')
  const afterPreset = await readRules(page)
  const noAutoStart = await page.evaluate(() => ({
    progress: document.querySelector('.progress') !== null,
    summary: [...document.querySelectorAll('.notice__text')].some((el) => el.textContent?.includes('处理完成：')),
  }))
  await page.waitForTimeout(400)
  const storedAfterPreset = await readStoredState(page)

  await page.getByRole('button', { name: '开始处理', exact: true }).click()
  await page.waitForFunction(() => document.body.innerText.includes('处理完成：'), null, { timeout: 5000 })

  await page.selectOption('#preset-select', '2')
  await page.waitForSelector('.notice--stale')
  const staleState = await page.evaluate(() => ({
    notice: document.querySelector('.notice--stale .notice__text')?.textContent?.trim() ?? null,
    badges: [...document.querySelectorAll('.row__stale')].map((el) => el.textContent?.trim()),
  }))
  await page.screenshot({ path: '.scratch/image-workshop/evidence/imageworkshop-preset-stale-1280.png' })

  await page.getByRole('button', { name: '管理预设', exact: true }).click()
  await page.waitForSelector('.preset-dialog__panel')
  await page.waitForFunction(() => document.querySelectorAll('.preset-row').length === 2, null, { timeout: 5000 })
  const dialogRows = await page.evaluate(() =>
    [...document.querySelectorAll('.preset-row')].map((row) => ({
      name: row.querySelector('.preset-row__name')?.textContent?.trim() ?? null,
      rules: row.querySelector('.preset-row__rules')?.textContent?.trim() ?? null,
    })),
  )

  await page.fill('#preset-name', ' 新预设 ')
  await page.getByRole('button', { name: '保存当前参数', exact: true }).click()
  await page.waitForFunction(
    () => [...document.querySelectorAll('.preset-row__name')].some((el) => el.textContent?.trim() === '新预设'),
    null,
    { timeout: 5000 },
  )

  await page.fill('#preset-name', '新预设 ')
  await page.getByRole('button', { name: '保存当前参数', exact: true }).click()
  await page.waitForFunction(
    () => document.body.innerText.includes('预设名称已存在'),
    null,
    { timeout: 5000 },
  )
  const duplicateError = await page.evaluate(() =>
    document.querySelector('.preset-dialog__error')?.textContent?.trim() ?? null,
  )
  await page.screenshot({ path: '.scratch/image-workshop/evidence/imageworkshop-preset-dialog-1280.png' })

  const createdRow = page.locator('.preset-row').filter({ hasText: '新预设' }).first()
  await createdRow.getByRole('button', { name: '重命名', exact: true }).click()
  await page.locator('.preset-row__input').fill('新预设 2')
  await page.getByRole('button', { name: '保存', exact: true }).click()
  await page.waitForFunction(
    () => [...document.querySelectorAll('.preset-row__name')].some((el) => el.textContent?.trim() === '新预设 2'),
    null,
    { timeout: 5000 },
  )

  const renamedRow = page.locator('.preset-row').filter({ hasText: '新预设 2' })
  await renamedRow.getByRole('button', { name: '删除', exact: true }).click()
  await page.waitForFunction(() => document.querySelectorAll('.preset-row').length === 2, null, { timeout: 5000 })
  const afterDelete = await readStoredState(page)

  const summaryBeforeDialogApply = await page.evaluate(() =>
    [...document.querySelectorAll('.notice__text')].filter((el) => el.textContent?.includes('处理完成：')).length,
  )
  await page.locator('.preset-row').filter({ hasText: '电商主图' }).getByRole('button', { name: '应用', exact: true }).click()
  await page.waitForFunction(() => !document.querySelector('.preset-dialog__panel'), null, { timeout: 5000 })
  const afterDialogApply = await readRules(page)
  const appliedNoStart = await page.evaluate(() => ({
    progress: document.querySelector('.progress') !== null,
    summaryCount: [...document.querySelectorAll('.notice__text')].filter((el) => el.textContent?.includes('处理完成：')).length,
  }))

  await page.selectOption('#output-format', 'webp')
  await page.waitForTimeout(500)
  const storedAfterManualChange = await readStoredState(page)

  await page.reload({ waitUntil: 'networkidle' })
  await page.waitForSelector('.presets')
  await page.waitForFunction(
    () => document.querySelectorAll('#preset-select option').length === 3,
    null,
    { timeout: 5000 },
  )
  const optionsAfterReload = await page.evaluate(() =>
    [...document.querySelectorAll('#preset-select option')].map((option) => option.textContent.trim()),
  )
  await importFiles(page)
  const restoredAfterReload = await readRules(page)

  console.log(
    JSON.stringify({
      initialOptions,
      restored,
      afterPreset,
      noAutoStart,
      storedAfterPreset: storedAfterPreset.settings,
      staleState,
      dialogRows,
      duplicateError,
      afterDelete: {
        presets: afterDelete.presets.map((item) => item.name),
        confirmCalls: afterDelete.confirmCalls,
      },
      afterDialogApply,
      summaryBeforeDialogApply,
      appliedNoStart,
      storedAfterManualChange: storedAfterManualChange.settings,
      settingsSaves: storedAfterManualChange.settingsSaves,
      optionsAfterReload,
      restoredAfterReload,
    }, null, 2),
  )
  await browser.close()
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
