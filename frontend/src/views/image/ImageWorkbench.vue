<template>
  <section class="workbench">
    <header class="workbench__head">
      <div class="workbench__intro">
        <h1 class="workbench__title">图片处理</h1>
        <p class="workbench__desc">批量转换格式、压缩体积、限制尺寸。图片只在本机处理。</p>
      </div>
      <div class="workbench__actions">
        <span v-if="items.length > 0" class="workbench__count">{{ items.length }} 项</span>
        <button class="btn btn--primary" type="button" :disabled="importing" @click="addFiles">
          {{ importing ? '扫描中…' : '添加文件' }}
        </button>
        <button class="btn" type="button" :disabled="importing" @click="addFolder">添加文件夹</button>
        <button
          v-if="items.length > 0"
          class="btn btn--danger"
          type="button"
          :disabled="importing"
          @click="clearQueue"
        >
          清空
        </button>
      </div>
    </header>

    <p v-if="actionError" class="workbench__error" role="alert">
      [{{ actionError.code }}] {{ actionError.message }}
    </p>

    <div v-if="importNote" class="notice" role="status" :title="importNote.detail">
      <span class="notice__text">{{ importNote.text }}</span>
      <button class="notice__close" type="button" aria-label="关闭导入提示" @click="dismissNote">
        ×
      </button>
    </div>

    <div v-if="items.length === 0" class="dropzone" role="region" aria-label="图片导入区">
      <div class="dropzone__icon" aria-hidden="true">＋</div>
      <p class="dropzone__title">将图片或文件夹拖到这里</p>
      <p class="dropzone__hint">支持 {{ supportedFormats }}</p>
    </div>

    <ul v-else class="queue" aria-label="待处理图片">
      <li v-for="item in items" :key="item.path" class="queue__row">
        <div class="queue__icon" aria-hidden="true">▤</div>
        <div class="queue__main">
          <p class="queue__name" :title="item.path">{{ item.name }}</p>
          <p class="queue__path" :title="item.path">{{ item.path }}</p>
        </div>
        <span class="queue__size">{{ formatBytes(item.size) }}</span>
        <span class="queue__status">待检查</span>
        <button
          class="queue__remove"
          type="button"
          :disabled="importing"
          @click="removeItem(item.path)"
        >
          移除
        </button>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { ScanPaths, SelectDirectory, SelectFiles } from '../../../wailsjs/go/main/App'
import { invoke, type AppError } from '../../lib/invoke'
import { logError, logInfo } from '../../lib/log'

const supportedFormats = 'JPG / PNG / WebP / BMP / TIFF / GIF（静态）'

type ScanResult = Awaited<ReturnType<typeof ScanPaths>>
type QueueItem = ScanResult['files'][number]

interface ImportNote {
  text: string
  detail: string
}

const items = ref<QueueItem[]>([])
const importing = ref(false)
const importNote = ref<ImportNote | null>(null)
const actionError = ref<AppError | null>(null)

let unregisterDrop: (() => void) | null = null
let disposed = false

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '—'
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes / 1024
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  return `${value >= 10 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`
}

function currentQueuePaths(): string[] {
  return items.value.map((item) => item.path)
}

function scanNote(result: ScanResult, added: number): ImportNote {
  const rejected = result.rejected
  if (rejected.length === 0) {
    return {
      text: added > 0 ? `已加入 ${added} 个文件` : '没有找到支持的图片',
      detail: '',
    }
  }
  const counts = new Map<string, number>()
  for (const item of rejected) {
    counts.set(item.reason, (counts.get(item.reason) ?? 0) + 1)
  }
  const summary = Array.from(counts.entries())
    .map(([reason, count]) => `${reason} ${count}`)
    .join('、')
  return {
    text: `${added > 0 ? `已加入 ${added} 个` : '未加入新文件'}；跳过 ${rejected.length} 个（${summary}）`,
    detail: rejected.map((item) => `${item.path}：${item.reason}`).join('\n'),
  }
}

async function importPaths(paths: string[]) {
  if (importing.value || paths.length === 0) return
  importing.value = true
  actionError.value = null
  importNote.value = null
  try {
    const result = await invoke(() => ScanPaths(paths, currentQueuePaths()), 'ScanPaths')
    const added = result.files
    if (added.length > 0) {
      items.value = items.value.concat(added)
    }
    importNote.value = scanNote(result, added.length)
    logInfo('图片导入完成', {
      requested: paths.length,
      added: added.length,
      rejected: result.rejected.length,
    })
  } catch (e) {
    actionError.value = e as AppError
    logError('图片导入失败', e)
  } finally {
    importing.value = false
  }
}

async function addFiles() {
  try {
    const paths = await invoke(() => SelectFiles('添加图片'), 'SelectFiles')
    if (paths && paths.length > 0) await importPaths(paths)
  } catch (e) {
    actionError.value = e as AppError
    logError('打开文件选择器失败', e)
  }
}

async function addFolder() {
  try {
    const directory = await invoke(() => SelectDirectory('添加图片文件夹'), 'SelectDirectory')
    if (directory) await importPaths([directory])
  } catch (e) {
    actionError.value = e as AppError
    logError('打开文件夹选择器失败', e)
  }
}

function removeItem(path: string) {
  items.value = items.value.filter((item) => item.path !== path)
}

function dismissNote() {
  importNote.value = null
}

function clearQueue() {
  if (importing.value) return
  items.value = []
  importNote.value = null
  actionError.value = null
}

onMounted(async () => {
  try {
    const rt = await import('../../../wailsjs/runtime/runtime')
    if (typeof rt.OnFileDrop !== 'function' || disposed) return
    rt.OnFileDrop((_x, _y, paths) => {
      void importPaths(paths)
    }, true)
    unregisterDrop = () => rt.OnFileDropOff()
  } catch {
    // 非 Wails 环境没有文件拖拽能力，按钮仍可通过绑定工作。
  }
})

onUnmounted(() => {
  disposed = true
  unregisterDrop?.()
  unregisterDrop = null
})
</script>

<style scoped>
.workbench {
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.workbench__head {
  flex: none;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-4);
  flex-wrap: wrap;
}

.workbench__intro {
  flex: 1;
  min-width: 240px;
}

.workbench__title {
  font-size: var(--font-size-xl);
  margin: 0 0 var(--space-2);
}

.workbench__desc {
  margin: 0;
  color: var(--text-2);
}

.workbench__actions {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex-wrap: wrap;
}

.workbench__count {
  color: var(--text-3);
  font-size: var(--font-size-sm);
  font-variant-numeric: tabular-nums;
  padding: 2px var(--space-2);
  border-radius: var(--radius-full);
  background: var(--bg-hover);
}

.workbench__error {
  margin: 0;
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

.btn {
  border: 1px solid var(--border-subtle);
  background: var(--bg-raised);
  color: var(--text-2);
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-md);
  font-family: inherit;
  font-size: var(--font-size-sm);
  cursor: pointer;
  transition: border-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard);
}

.btn:hover:not(:disabled) {
  border-color: var(--color-primary-border);
  background: var(--bg-hover);
  color: var(--text-1);
}

.btn--primary {
  border-color: var(--color-primary);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.btn--primary:hover:not(:disabled) {
  background: var(--color-primary-200);
  color: var(--color-primary-700);
}

.btn--danger {
  border-color: var(--border-subtle);
  color: var(--text-2);
}

.btn--danger:hover:not(:disabled) {
  border-color: var(--color-danger);
  background: var(--bg-hover);
  color: var(--color-danger);
}

.btn:disabled,
.queue__remove:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn:focus-visible,
.queue__remove:focus-visible,
.notice__close:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

.notice {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-raised);
  color: var(--text-2);
  font-size: var(--font-size-sm);
}

.notice__text {
  flex: 1;
  min-width: 0;
}

.notice__close {
  flex: none;
  border: none;
  background: transparent;
  color: var(--text-3);
  font-size: var(--font-size-lg);
  line-height: 1;
  cursor: pointer;
  padding: var(--space-1);
  border-radius: var(--radius-sm);
}

.notice__close:hover {
  color: var(--text-1);
}

.dropzone {
  flex: 1;
  min-height: 320px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-3);
  padding: var(--space-8);
  text-align: center;
  border: 1px dashed var(--border-strong);
  border-radius: var(--radius-lg);
  background: var(--bg-raised);
  transition: border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard);
}

.dropzone:hover {
  border-color: var(--color-primary);
  background: var(--bg-hover);
}

.dropzone__icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-full);
  background: var(--bg-hover);
  color: var(--color-primary);
  font-size: 24px;
  line-height: 1;
}

.dropzone__title {
  margin: 0;
  color: var(--text-1);
  font-size: var(--font-size-lg);
}

.dropzone__hint {
  margin: 0;
  color: var(--text-3);
  font-size: var(--font-size-sm);
}

.queue {
  flex: 1;
  min-height: 0;
  margin: 0;
  padding: 0;
  list-style: none;
  overflow-y: auto;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-raised);
}

.queue__row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
  transition: background-color var(--motion-fast) var(--ease-standard);
}

.queue__row:last-child {
  border-bottom: none;
}

.queue__row:hover {
  background: var(--bg-hover);
}

.queue__icon {
  flex: none;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-md);
  background: var(--bg-hover);
  color: var(--color-primary);
  font-size: var(--font-size-md);
}

.queue__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.queue__name,
.queue__path {
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.queue__name {
  color: var(--text-1);
  font-size: var(--font-size-md);
}

.queue__path {
  color: var(--text-3);
  font-size: var(--font-size-xs);
}

.queue__size {
  flex: none;
  color: var(--text-2);
  font-size: var(--font-size-sm);
  font-variant-numeric: tabular-nums;
}

.queue__status {
  flex: none;
  padding: 2px var(--space-2);
  border-radius: var(--radius-full);
  background: var(--bg-hover);
  color: var(--text-2);
  font-size: var(--font-size-xs);
}

.queue__remove {
  flex: none;
  border: none;
  background: transparent;
  color: var(--text-3);
  font-family: inherit;
  font-size: var(--font-size-sm);
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard);
}

.queue__remove:hover:not(:disabled) {
  background: var(--bg-hover);
  color: var(--color-danger);
}
</style>
