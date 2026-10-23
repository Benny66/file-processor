<template>
  <section class="workbench">
    <header class="workbench__head">
      <div class="workbench__intro">
        <h1 class="workbench__title">图片处理</h1>
        <p class="workbench__desc">批量转换格式、压缩体积、限制尺寸。图片只在本机处理。</p>
      </div>
      <div class="workbench__actions">
        <span v-if="items.length > 0" class="workbench__count">{{ items.length }} 项</span>
        <button class="btn btn--primary" type="button" :disabled="busy" @click="addFiles">
          {{ importing ? '扫描中…' : '添加文件' }}
        </button>
        <button class="btn" type="button" :disabled="busy" @click="addFolder">添加文件夹</button>
        <button
          v-if="items.length > 0"
          class="btn btn--danger"
          type="button"
          :disabled="busy"
          @click="clearQueue"
        >
          清空
        </button>
      </div>
    </header>

    <div v-if="items.length > 0" class="rules">
      <label class="field" for="output-format">
        <span class="field__label">输出格式</span>
        <select
          id="output-format"
          class="select"
          :value="outputFormat"
          :disabled="busy"
          @change="onFormatChange"
        >
          <option v-for="option in formatOptions" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </label>
      <label class="field field--check">
        <input
          type="checkbox"
          class="checkbox"
          :checked="sizeBoxEnabled"
          :disabled="busy"
          @change="onBoxToggle"
        />
        <span class="field__label">限制尺寸</span>
      </label>
      <template v-if="sizeBoxEnabled">
        <input
          id="max-width"
          class="input"
          type="number"
          :min="BoxDimension.Min"
          :max="BoxDimension.Max"
          step="1"
          aria-label="最大宽度"
          :value="maxWidth"
          :disabled="busy"
          @input="onMaxWidth"
        />
        <span class="field__sep">×</span>
        <input
          id="max-height"
          class="input"
          type="number"
          :min="BoxDimension.Min"
          :max="BoxDimension.Max"
          step="1"
          aria-label="最大高度"
          :value="maxHeight"
          :disabled="busy"
          @input="onMaxHeight"
        />
        <span class="field__unit">px</span>
      </template>
      <label class="field field--check">
        <input
          type="checkbox"
          class="checkbox"
          :checked="targetEnabled"
          :disabled="busy"
          @change="onTargetToggle"
        />
        <span class="field__label">目标体积</span>
      </label>
      <template v-if="targetEnabled">
        <input
          id="target-value"
          class="input input--target"
          type="number"
          min="1"
          step="1"
          aria-label="目标体积"
          :value="targetValue"
          :disabled="busy"
          @input="onTargetValue"
        />
        <select
          id="target-unit"
          class="select select--unit"
          :value="targetUnit"
          :disabled="busy"
          @change="onTargetUnit"
        >
          <option value="KB">KB</option>
          <option value="MB">MB</option>
        </select>
      </template>
      <button class="btn btn--primary" type="button" :disabled="!canStart" @click="startBatch">
        {{ processing ? '处理中…' : '开始处理' }}
      </button>
      <button v-if="processing" class="btn" type="button" :disabled="canceling" @click="cancelBatch">
        {{ canceling ? '取消中…' : '取消' }}
      </button>
      <span v-if="processing" class="rules__progress">{{ processed }} / {{ total }}</span>
    </div>
    <p v-if="items.length > 0 && targetEnabled && !targetValid" class="rules__hint">
      目标体积需在 1 KB–1 GB。
    </p>
    <p v-else-if="items.length > 0 && sizeBoxEnabled && !sizeBoxValid" class="rules__hint">
      宽高需为 1–65535 的整数。
    </p>
    <p v-else-if="items.length > 0 && !hasRule" class="rules__hint">
      请选择输出格式，或开启目标体积/尺寸限制。
    </p>
    <div
      v-if="processing"
      class="progress"
      role="progressbar"
      :aria-valuenow="processed"
      aria-valuemin="0"
      :aria-valuemax="total"
    >
      <div class="progress__bar" :style="{ width: progressPercent + '%' }" />
    </div>

    <p v-if="actionError" class="workbench__error" role="alert">
      [{{ actionError.code }}] {{ actionError.message }}
    </p>

    <div v-if="importNote" class="notice" role="status" :title="importNote.detail">
      <span class="notice__text">{{ importNote.text }}</span>
      <button class="notice__close" type="button" aria-label="关闭导入提示" @click="dismissNote">
        ×
      </button>
    </div>
    <div v-if="batchSummary" class="notice" role="status">
      <span class="notice__text">{{ batchSummary }}</span>
      <button class="notice__close" type="button" aria-label="关闭处理结果" @click="dismissSummary">
        ×
      </button>
    </div>

    <div v-if="items.length === 0" class="dropzone" role="region" aria-label="图片导入区">
      <div class="dropzone__icon" aria-hidden="true">＋</div>
      <p class="dropzone__title">将图片或文件夹拖到这里</p>
      <p class="dropzone__hint">支持 {{ supportedFormats }}</p>
    </div>

    <ul v-else class="queue" aria-label="待处理图片">
      <QueueRow
        v-for="item in items"
        :key="item.path"
        :item="item"
        :busy="busy"
        @inspect="requestInspect"
        @remove="removeItem"
      />
    </ul>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  CancelBatch,
  InspectImage,
  ProcessBatch,
  ScanPaths,
  SelectDirectory,
  SelectFiles,
} from '../../../wailsjs/go/main/App'
import { service } from '../../../wailsjs/go/models'
import { useEvent } from '../../composables/useEvent'
import { EventAction, eventName } from '../../lib/event'
import { invoke, type AppError } from '../../lib/invoke'
import { BoxDimension, OutputFormat, TargetBytes, type OutputFormatValue } from '../../lib/image'
import { logError, logInfo, logWarn } from '../../lib/log'
import { formatBytes } from './format'
import QueueRow from './QueueRow.vue'
import type { BatchResult, ImageProgress, ItemResult, QueueItem, ScanResult } from './types'

const supportedFormats = 'JPG / PNG / WebP / BMP / TIFF / GIF（静态）'

interface ImportNote {
  text: string
  detail: string
}

const items = ref<QueueItem[]>([])
const importing = ref(false)
const importNote = ref<ImportNote | null>(null)
const actionError = ref<AppError | null>(null)
const outputFormat = ref<OutputFormatValue>(OutputFormat.Keep)
const processing = ref(false)
const canceling = ref(false)
const processed = ref(0)
const total = ref(0)
const batchSummary = ref<string | null>(null)
const sizeBoxEnabled = ref(false)
const maxWidth = ref(1920)
const maxHeight = ref(1080)
const targetEnabled = ref(false)
const targetValue = ref(500)
const targetUnit = ref<'KB' | 'MB'>('KB')

const busy = computed(() => importing.value || processing.value)
const sizeBoxValid = computed(() => {
  if (!sizeBoxEnabled.value) return true
  const valid = (value: number) =>
    Number.isInteger(value) && value >= BoxDimension.Min && value <= BoxDimension.Max
  return valid(maxWidth.value) && valid(maxHeight.value)
})
const targetBytes = computed(() =>
  targetUnit.value === 'MB' ? targetValue.value * 1024 * 1024 : targetValue.value * 1024,
)
const targetValid = computed(() => {
  if (!targetEnabled.value) return true
  const bytes = targetBytes.value
  return (
    Number.isFinite(bytes) &&
    Number.isInteger(bytes) &&
    bytes >= TargetBytes.Min &&
    bytes <= TargetBytes.Max
  )
})
const hasRule = computed(
  () => outputFormat.value !== OutputFormat.Keep || sizeBoxEnabled.value || targetEnabled.value,
)
const canStart = computed(
  () =>
    items.value.some((item) => !item.error) &&
    !busy.value &&
    hasRule.value &&
    sizeBoxValid.value &&
    targetValid.value &&
    !items.value.some((item) => item.inspecting),
)
const progressPercent = computed(() =>
  total.value > 0 ? Math.round((processed.value / total.value) * 100) : 0,
)
const formatOptions: { value: OutputFormatValue; label: string }[] = [
  { value: OutputFormat.Keep, label: '保持原格式' },
  { value: OutputFormat.JPEG, label: 'JPEG' },
  { value: OutputFormat.PNG, label: 'PNG' },
  { value: OutputFormat.WebP, label: 'WebP' },
]

let unregisterDrop: (() => void) | null = null
let disposed = false

const inspectQueue: string[] = []
const queuedForInspect = new Set<string>()
let inspecting = false

function currentQueuePaths(): string[] {
  return items.value.map((item) => item.path)
}

function applyItemResult(result: ItemResult) {
  const item = items.value.find((entry) => entry.path === result.path)
  if (item) item.result = result
}

useEvent<ImageProgress>(eventName('image', EventAction.Progress), (progress) => {
  processed.value = progress.done
  total.value = progress.total
  if (progress.current) applyItemResult(progress.current)
})

function requestInspect(path: string) {
  const item = items.value.find((entry) => entry.path === path)
  if (!item || item.info || item.error || item.inspecting || queuedForInspect.has(path)) return
  queuedForInspect.add(path)
  inspectQueue.push(path)
  void drainInspectQueue()
}

async function drainInspectQueue() {
  if (inspecting) return
  inspecting = true
  try {
    while (inspectQueue.length > 0) {
      const path = inspectQueue.shift()
      if (!path) continue
      queuedForInspect.delete(path)
      const item = items.value.find((entry) => entry.path === path)
      if (!item || item.info || item.error) continue
      item.inspecting = true
      try {
        item.info = await invoke(() => InspectImage(path), 'InspectImage')
      } catch (e) {
        const err = e as AppError
        item.error = err.message
        logWarn('图片检查失败', { path, code: err.code, message: err.message })
      } finally {
        item.inspecting = false
      }
    }
  } finally {
    inspecting = false
  }
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
  if (busy.value || paths.length === 0) return
  importing.value = true
  actionError.value = null
  importNote.value = null
  batchSummary.value = null
  try {
    const result = await invoke(() => ScanPaths(paths, currentQueuePaths()), 'ScanPaths')
    const added: QueueItem[] = result.files.map((file) => ({
      ...file,
      info: null,
      error: null,
      inspecting: false,
      result: null,
    }))
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

function summaryText(result: BatchResult): string {
  const parts = [`成功 ${result.success}`, `失败 ${result.failed}`]
  if (result.skipped > 0) parts.push(`跳过 ${result.skipped}`)
  if (result.canceled > 0) parts.push(`取消 ${result.canceled}`)
  if (result.extreme_degraded > 0) parts.push(`极端降级 ${result.extreme_degraded}`)
  let text = `处理完成：${parts.join('、')}`
  if (result.saved_bytes > 0) {
    text += `；累计节省 ${formatBytes(result.saved_bytes)}`
  } else if (result.saved_bytes < 0) {
    text += `；累计增加 ${formatBytes(-result.saved_bytes)}`
  }
  return text
}

function applyBatchResult(result: BatchResult) {
  for (const item of result.items) applyItemResult(item)
  processed.value = result.items.length
  total.value = result.items.length
  batchSummary.value = summaryText(result)
  logInfo('批量处理完成', {
    success: result.success,
    failed: result.failed,
    canceled: result.canceled,
  })
}

async function startBatch() {
  if (!canStart.value) return
  const batchPaths = items.value.filter((item) => !item.error).map((item) => item.path)
  if (batchPaths.length === 0) return
  processing.value = true
  canceling.value = false
  processed.value = 0
  total.value = batchPaths.length
  batchSummary.value = null
  actionError.value = null
  importNote.value = null
  for (const item of items.value) item.result = null
  try {
    const request = new service.BatchRequest({
      paths: batchPaths,
      rules: {
        output_format: outputFormat.value,
        target_bytes: targetEnabled.value ? targetBytes.value : 0,
        max_width: sizeBoxEnabled.value ? maxWidth.value : 0,
        max_height: sizeBoxEnabled.value ? maxHeight.value : 0,
      },
    })
    const result = await invoke(() => ProcessBatch(request), 'ProcessBatch')
    applyBatchResult(result)
  } catch (e) {
    actionError.value = e as AppError
    logError('批量处理失败', e)
  } finally {
    processing.value = false
    canceling.value = false
  }
}

async function cancelBatch() {
  if (!processing.value || canceling.value) return
  canceling.value = true
  try {
    await invoke(() => CancelBatch(), 'CancelBatch')
    logInfo('已请求取消批次')
  } catch (e) {
    canceling.value = false
    actionError.value = e as AppError
    logError('取消批次失败', e)
  }
}

function onFormatChange(event: Event) {
  outputFormat.value = (event.target as HTMLSelectElement).value as OutputFormatValue
}

function onBoxToggle(event: Event) {
  sizeBoxEnabled.value = (event.target as HTMLInputElement).checked
}

function onMaxWidth(event: Event) {
  maxWidth.value = Number((event.target as HTMLInputElement).value)
}

function onMaxHeight(event: Event) {
  maxHeight.value = Number((event.target as HTMLInputElement).value)
}

function onTargetToggle(event: Event) {
  targetEnabled.value = (event.target as HTMLInputElement).checked
}

function onTargetValue(event: Event) {
  targetValue.value = Number((event.target as HTMLInputElement).value)
}

function onTargetUnit(event: Event) {
  targetUnit.value = (event.target as HTMLSelectElement).value as 'KB' | 'MB'
}

function dismissSummary() {
  batchSummary.value = null
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
  if (busy.value) return
  items.value = items.value.filter((item) => item.path !== path)
}

function dismissNote() {
  importNote.value = null
}

function clearQueue() {
  if (busy.value) return
  items.value = []
  inspectQueue.length = 0
  queuedForInspect.clear()
  importNote.value = null
  batchSummary.value = null
  processed.value = 0
  total.value = 0
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
  inspectQueue.length = 0
  queuedForInspect.clear()
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

.rules {
  flex: none;
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex-wrap: wrap;
}

.field {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.field__label {
  color: var(--text-2);
  font-size: var(--font-size-sm);
}

.select {
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-raised);
  color: var(--text-1);
  padding: var(--space-2) var(--space-3);
  font-family: inherit;
  font-size: var(--font-size-sm);
  cursor: pointer;
  transition: border-color var(--motion-fast) var(--ease-standard);
}

.select:hover:not(:disabled) {
  border-color: var(--color-primary-border);
}

.select:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

.select:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.field--check {
  cursor: pointer;
  user-select: none;
}

.checkbox {
  width: 14px;
  height: 14px;
  accent-color: var(--color-primary);
  cursor: pointer;
}

.checkbox:disabled {
  cursor: not-allowed;
}

.input {
  width: 84px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-raised);
  color: var(--text-1);
  padding: var(--space-2) var(--space-3);
  font-family: inherit;
  font-size: var(--font-size-sm);
  font-variant-numeric: tabular-nums;
  text-align: center;
  transition: border-color var(--motion-fast) var(--ease-standard);
}

.input:hover:not(:disabled) {
  border-color: var(--color-primary-border);
}

.input:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

.input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.input--target {
  width: 100px;
}

.select--unit {
  width: 78px;
}

.field__sep,
.field__unit {
  color: var(--text-3);
  font-size: var(--font-size-sm);
}

.rules__progress {
  color: var(--text-2);
  font-size: var(--font-size-sm);
  font-variant-numeric: tabular-nums;
}

.rules__hint {
  margin: 0;
  color: var(--text-3);
  font-size: var(--font-size-xs);
}

.progress {
  flex: none;
  height: 4px;
  overflow: hidden;
  border-radius: var(--radius-full);
  background: var(--bg-hover);
}

.progress__bar {
  height: 100%;
  background: var(--color-primary);
  transition: width var(--motion-fast) var(--ease-standard);
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

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn:focus-visible,
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
</style>
