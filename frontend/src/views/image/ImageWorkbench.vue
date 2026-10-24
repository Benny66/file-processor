<template>
  <section class="workbench">
    <header class="workbench__head">
      <div class="workbench__intro">
        <h1 class="workbench__title">图片处理</h1>
        <p class="workbench__desc">批量转换、压缩图片。全程在本机完成，不上传。</p>
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

    <div class="presets">
      <label class="field" for="preset-select">
        <span class="field__label">常用设置</span>
        <select
          id="preset-select"
          class="select select--preset"
          :value="selectedPresetId"
          :disabled="busy || presets.length === 0"
          @change="onPresetSelect"
        >
          <option value="">{{ presets.length === 0 ? '还没保存过设置' : '选择常用设置…' }}</option>
          <option v-for="preset in presets" :key="preset.id" :value="String(preset.id)">
            {{ preset.name }}
          </option>
        </select>
      </label>
      <button class="btn" type="button" :disabled="busy" @click="openPresetDialog">
        管理常用设置
      </button>
    </div>

    <div v-if="items.length > 0" class="rules">
      <label class="field" for="output-format">
        <span class="field__label">转成哪种格式</span>
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
        <span class="field__label">压到指定大小</span>
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
      <label class="field" for="output-mode">
        <span class="field__label">输出到</span>
        <select
          id="output-mode"
          class="select"
          :value="outputMode"
          :disabled="busy"
          @change="onOutputModeChange"
        >
          <option :value="OutputMode.Source">源文件同目录</option>
          <option :value="OutputMode.CustomFlat">指定目录</option>
          <option :value="OutputMode.CustomTree">指定目录·保留结构</option>
        </select>
      </label>
      <button
        v-if="outputMode !== OutputMode.Source"
        class="btn btn--dir"
        type="button"
        :disabled="busy"
        :title="outputDir || '选择输出目录'"
        @click="chooseOutputDir"
      >
        {{ outputDir || '选择目录…' }}
      </button>
      <label class="field" for="output-pattern">
        <span class="field__label">命名</span>
        <input
          id="output-pattern"
          class="input input--pattern"
          type="text"
          placeholder="{name}-out"
          :value="outputPattern"
          :disabled="busy"
          @input="onOutputPattern"
        />
      </label>
      <span class="field__unit rules__example" :title="namingExample">{{ namingExample }}</span>
      <button class="btn btn--primary" type="button" :disabled="!canStart" @click="startBatch">
        {{ processing ? '处理中…' : '开始处理' }}
      </button>
      <button v-if="processing" class="btn" type="button" :disabled="canceling" @click="cancelBatch">
        {{ canceling ? '取消中…' : '取消' }}
      </button>
      <span v-if="processing" class="rules__progress">{{ processed }} / {{ total }}</span>
    </div>
    <!-- 文案来自 Go 的 validateBatchStart/validateBatchRules：判定与提示同一个出处 -->
    <p v-if="items.length > 0 && ruleProblem" class="rules__hint">{{ ruleProblem }}</p>
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

    <div v-if="resultsStale" class="notice notice--stale" role="status">
      <span class="notice__text">设置改过了，重新点「开始处理」即可。</span>
    </div>
    <div v-if="importNote" class="notice" role="status" :title="importNote.detail">
      <span class="notice__text">{{ importNote.text }}</span>
      <button class="notice__close" type="button" aria-label="关闭导入提示" @click="dismissNote">
        ×
      </button>
    </div>
    <div v-if="batchSummary" class="notice" role="status">
      <span class="notice__text">{{ batchSummary }}</span>
      <!-- US-43：批次结束后一键重跑全部失败/已取消项；成功/跳过结果不动。 -->
      <button
        v-if="retryCount > 0 && !busy"
        class="notice__retry"
        type="button"
        @click="retryUnfinished"
      >
        把没成功的 {{ retryCount }} 张再试一次
      </button>
      <button class="notice__close" type="button" aria-label="关闭处理结果" @click="dismissSummary">
        ×
      </button>
    </div>

    <div v-if="items.length === 0" class="dropzone" role="region" aria-label="图片导入区">
      <div class="dropzone__icon" aria-hidden="true">＋</div>
      <p class="dropzone__title">将图片或文件夹拖到这里</p>
      <p v-if="supportedFormats" class="dropzone__hint">支持 {{ supportedFormats }}</p>
      <!-- D-46：与 README 首句同口径的隐私承诺，第一屏可见（Squoosh 先例：零成本高信任）。 -->
      <p class="dropzone__hint">所有处理在本机完成：不上传、不联网、不采集遥测</p>
    </div>

    <ul v-else class="queue" aria-label="待处理图片">
      <QueueRow
        v-for="item in items"
        :key="item.path"
        :item="item"
        :busy="busy"
        :stale="resultsStale"
        @inspect="requestInspect"
        @remove="removeItem"
        @compare="openCompare"
        @retry="retryOne"
        @tune="openTune"
      />
    </ul>

    <CompareDialog :open="compareOpen" :item="compareItem" @close="closeCompare" />
    <!-- 调参工作台（US-45~48）：打开时播种当前规则；「应用/存预设」后才回写工作台。 -->
    <TuneDialog
      :open="tuneOpen"
      :item="tuneItem"
      :busy="busy"
      :rules="currentRules"
      @close="tuneOpen = false"
      @apply="applyTuneRules"
      @saved="refreshPresets"
    />
    <PresetDialog
      :open="presetDialogOpen"
      :presets="presets"
      :rules="currentRules"
      :loading="presetsLoading"
      :load-error="presetsError?.message ?? ''"
      @close="presetDialogOpen = false"
      @refresh="refreshPresets"
      @apply="applyPresetFromDialog"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import {
  CancelBatch,
  GetImageSettings,
  InspectImage,
  ListPresets,
  ProcessBatch,
  RenderOutputSample,
  SaveImageSettings,
  ScanPaths,
  SelectDirectory,
  SelectFiles,
  SupportedFormatHints,
  ValidateImageRules,
} from '../../../wailsjs/go/main/App'
import { service } from '../../../wailsjs/go/models'
import { useEvent } from '../../composables/useEvent'
import { usePagedList } from '../../composables/usePagedList'
import { EventAction, eventName } from '../../lib/event'
import { invoke, type AppError } from '../../lib/invoke'
import { BoxDimension, ItemStatus, OutputFormat, OutputMode, type OutputFormatValue, type OutputModeValue } from '../../lib/image'
import { PageSize } from '../../lib/page'
import { logError, logInfo, logWarn } from '../../lib/log'
import CompareDialog from './CompareDialog.vue'
import PresetDialog from './PresetDialog.vue'
import TuneDialog from './TuneDialog.vue'
import { formatBytes } from './format'
import QueueRow from './QueueRow.vue'
import { isRetryableStatus, type BatchResult, type ImageProgress, type ItemResult, type Preset, type PresetRules, type QueueItem, type ScanResult } from './types'

// 导入格式清单取自 Go（imageproc.supportedFormats 的展示投影）。
// 此前这里是手写字符串：往引擎加一种格式，界面这句话不会跟着变。
const supportedFormats = ref('')

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
// 输出目的地（US-39~42）：目录仅 custom 模式携带；命名模板空串由 Go 按默认值处理。
const outputMode = ref<OutputModeValue>(OutputMode.Source)
const outputDir = ref('')
const outputPattern = ref('')
const resultsStale = ref(false)
const compareOpen = ref(false)
const compareItem = ref<QueueItem | null>(null)
const tuneOpen = ref(false)
const tuneItem = ref<QueueItem | null>(null)
const presetDialogOpen = ref(false)
const selectedPresetId = ref('')

const busy = computed(() => importing.value || processing.value)
const targetBytes = computed(() =>
  targetUnit.value === 'MB' ? targetValue.value * 1024 * 1024 : targetValue.value * 1024,
)
const currentRules = computed<PresetRules>(() => ({
  output_format: outputFormat.value,
  target_bytes: targetEnabled.value ? targetBytes.value : 0,
  max_width: sizeBoxEnabled.value ? maxWidth.value : 0,
  max_height: sizeBoxEnabled.value ? maxHeight.value : 0,
}))

const outputPlan = computed<service.OutputPlan>(() => ({
  mode: outputMode.value,
  directory: outputMode.value === OutputMode.Source ? '' : outputDir.value,
  pattern: outputPattern.value,
}))

// 「上次设置」的整体：规则 + 输出目的地（预设只带规则，D-34）。
// 保持纯对象形状；传给绑定时经 new service.ImageSettings(...) 转换（生成类带 convertValues）。
const currentSettings = computed(() => ({
  rules: currentRules.value,
  output: outputPlan.value,
}))

// 命名示例（US-41）：渲染走 Go 的 RenderOutputSample——模板替换与扩展名映射和真实
// 落盘同一套实现。此前前端手抄了 renderOutputStem 与格式→扩展名表，边界迟早分叉。
const namingExample = ref('photo.jpg → photo-out.jpg')
const namingSample = computed(() => items.value.find((item) => !item.error)?.name ?? 'photo.jpg')
let namingToken = 0
let namingTimer: ReturnType<typeof setTimeout> | null = null
async function refreshNamingExample() {
  const token = ++namingToken
  try {
    const rendered = await invoke(
      () => RenderOutputSample(outputPattern.value, namingSample.value, outputFormat.value),
      'RenderOutputSample',
    )
    if (token === namingToken && !disposed) {
      namingExample.value = `${namingSample.value} → ${rendered}`
    }
  } catch {
    // 模板/格式非法：具体原因由规则预检（ruleProblem）展示，这里只标注示例不可用。
    if (token === namingToken) namingExample.value = '命名示例不可用（见上方提示）'
  }
}
watch([outputPattern, outputFormat, namingSample], () => {
  if (namingTimer) clearTimeout(namingTimer)
  namingTimer = setTimeout(() => {
    namingTimer = null
    void refreshNamingExample()
  }, 200)
}, { immediate: true })

// 「能不能开始」由 Go 判定（US-11），前端只转述它给的原因。
//
// 此前这里复制了一份 Go 的范围校验（targetValid/sizeBoxValid）加三条手写文案，
// 是【同一条规则的两个实现】，而且已经漂移：Go 有「宽高需同时设置」，前端没有，
// 宽填 800、高留空时前端提示「宽高需为 1–65535 的整数」，真实被拒原因却是另一条。
// 铁律 1：镜像只许复制常量与类型，不许复制逻辑。
const ruleProblem = ref('')
let ruleToken = 0
async function refreshRuleProblem(): Promise<void> {
  const token = ++ruleToken
  let message: string
  try {
    message =
      (await invoke(() => ValidateImageRules(currentRules.value, outputPlan.value), 'ValidateImageRules')) ?? ''
  } catch (e) {
    // 预检本身失败时【失败关闭】：宁可不让开始，也不能带着未知规则开跑。
    message = e instanceof Error ? e.message : '规则校验暂时不可用'
    logWarn('规则预检失败', e)
  }
  // 只采纳最后一次请求的结果：连续输入时旧响应可能后到。
  if (disposed || token !== ruleToken) return
  ruleProblem.value = message
}
watch([currentRules, outputPlan], () => void refreshRuleProblem(), { immediate: true })

const canStart = computed(
  () =>
    items.value.some((item) => !item.error) &&
    !busy.value &&
    ruleProblem.value === '' &&
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
  // US-49：每项最多 3 次完整编码（有目标体积时最多 ~24 次整图编码），选项文案如实标注较慢。
  { value: OutputFormat.Auto, label: '自动选最省空间的（较慢）' },
]

const {
  list: presets,
  loading: presetsLoading,
  error: presetsError,
  load: loadPresets,
} = usePagedList<Preset>((req) => ListPresets(req), {
  pageSize: PageSize.Max,
  label: 'ListPresets',
})

let unregisterDrop: (() => void) | null = null
let disposed = false
let settingsLoaded = false
let settingsTimer: ReturnType<typeof setTimeout> | null = null

const inspectQueue: string[] = []
const queuedForInspect = new Set<string>()
let inspecting = false

function currentQueuePaths(): string[] {
  return items.value.map((item) => item.path)
}

function applyItemResult(result: ItemResult) {
  const item = items.value.find((entry) => entry.path === result.path)
  if (!item) return
  item.result = result
  // 结果落定后，「排队/处理中」这两格就该退场——它们是过程态。
  item.queued = false
  item.processing = false
}

// US-14「处理中」的唯一来源：Go 在 worker 领取任务时播报的 started。
// 前端不自己推断谁在跑——两个 worker 并行时推断必然出错。
function applyItemStarted(started: ItemResult) {
  const item = items.value.find((entry) => entry.path === started.path)
  if (!item) return
  item.queued = false
  item.processing = true
}

function markResultsStale() {
  if (items.value.some((item) => item.result)) resultsStale.value = true
}

function isOutputFormat(value: string): value is OutputFormatValue {
  return formatOptions.some((option) => option.value === value)
}

function splitTargetBytes(bytes: number): { value: number; unit: 'KB' | 'MB' } {
  const megabyte = 1024 * 1024
  if (bytes >= megabyte && bytes % megabyte === 0) {
    return { value: bytes / megabyte, unit: 'MB' }
  }
  // 持久化层要求整数 KB；旧配置/手改值按最近的 KB 回填，避免输入框出现小数。
  return { value: Math.max(1, Math.round(bytes / 1024)), unit: 'KB' }
}

function applyRules(rules: PresetRules) {
  if (isOutputFormat(rules.output_format)) {
    outputFormat.value = rules.output_format
  }
  if (rules.target_bytes > 0) {
    const split = splitTargetBytes(rules.target_bytes)
    targetEnabled.value = true
    targetValue.value = split.value
    targetUnit.value = split.unit
  } else {
    targetEnabled.value = false
  }
  if (rules.max_width > 0 && rules.max_height > 0) {
    sizeBoxEnabled.value = true
    maxWidth.value = rules.max_width
    maxHeight.value = rules.max_height
  } else {
    sizeBoxEnabled.value = false
  }
}

function applyOutput(output: service.OutputPlan) {
  if (
    output.mode === OutputMode.Source ||
    output.mode === OutputMode.CustomFlat ||
    output.mode === OutputMode.CustomTree
  ) {
    outputMode.value = output.mode
  } else {
    outputMode.value = OutputMode.Source
  }
  outputDir.value = output.directory ?? ''
  outputPattern.value = output.pattern ?? ''
}

async function saveImageSettings() {
  try {
    await invoke(
      () => SaveImageSettings(new service.ImageSettings(currentSettings.value)),
      'SaveImageSettings',
    )
  } catch (e) {
    logWarn('保存上次设置失败', e)
  }
}

function scheduleSettingsSave() {
  if (!settingsLoaded) return
  if (settingsTimer) clearTimeout(settingsTimer)
  settingsTimer = setTimeout(() => {
    settingsTimer = null
    void saveImageSettings()
  }, 300)
}

async function flushSettingsSave() {
  if (settingsTimer) {
    clearTimeout(settingsTimer)
    settingsTimer = null
  }
  await saveImageSettings()
}

// 导入格式清单向 Go 取一次（imageproc 是单一真相），界面不再手写。
async function loadSupportedFormats(): Promise<void> {
  try {
    const hints = await invoke(() => SupportedFormatHints(), 'SupportedFormatHints')
    if (disposed || !hints) return
    supportedFormats.value = hints.join(' / ')
  } catch (e) {
    logWarn('读取受支持格式清单失败', e)
  }
}

async function loadImageSettings() {
  try {
    const saved = await invoke(() => GetImageSettings(), 'GetImageSettings')
    applyRules(saved.rules)
    applyOutput(saved.output)
  } catch (e) {
    logWarn('恢复上次设置失败，使用默认值', e)
  } finally {
    settingsLoaded = true
  }
}

watch(
  [
    outputFormat,
    sizeBoxEnabled,
    maxWidth,
    maxHeight,
    targetEnabled,
    targetValue,
    targetUnit,
    outputMode,
    outputDir,
    outputPattern,
  ],
  () => {
    markResultsStale()
    scheduleSettingsSave()
  },
)

watch(presets, (list) => {
  if (selectedPresetId.value && !list.some((preset) => String(preset.id) === selectedPresetId.value)) {
    selectedPresetId.value = ''
  }
})

function refreshPresets() {
  void loadPresets()
}

function openPresetDialog() {
  if (busy.value) return
  presetDialogOpen.value = true
  refreshPresets()
}

function applyPreset(preset: Preset) {
  selectedPresetId.value = String(preset.id)
  applyRules({
    output_format: preset.output_format,
    target_bytes: preset.target_bytes,
    max_width: preset.max_width,
    max_height: preset.max_height,
  })
  logInfo('已应用预设', { id: preset.id, name: preset.name })
  void flushSettingsSave()
}

function applyPresetFromDialog(preset: Preset) {
  applyPreset(preset)
  presetDialogOpen.value = false
}

function onPresetSelect(event: Event) {
  const id = (event.target as HTMLSelectElement).value
  selectedPresetId.value = id
  const preset = presets.value.find((entry) => String(entry.id) === id)
  if (!preset) return
  applyPreset(preset)
}

function openCompare(path: string) {
  const item = items.value.find((entry) => entry.path === path)
  if (!item) return
  compareItem.value = item
  compareOpen.value = true
}

function closeCompare() {
  compareOpen.value = false
}

function openTune(path: string) {
  if (busy.value) return
  const item = items.value.find((entry) => entry.path === path)
  if (!item) return
  tuneItem.value = item
  tuneOpen.value = true
}

// 工作台规则回写（US-48）：只改参数不自动开跑；已有结果由既有 watch 标「已过期」。
function applyTuneRules(rules: PresetRules) {
  applyRules(rules)
  void flushSettingsSave()
}

useEvent<ImageProgress>(eventName('image', EventAction.Progress), (progress) => {
  processed.value = progress.done
  total.value = progress.total
  if (progress.started) applyItemStarted(progress.started)
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
      queued: false,
      processing: false,
    }))
    if (added.length > 0) {
      items.value = items.value.concat(added)
      markResultsStale()
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

// US-43 的重跑集合：批次结束后仍处于「失败/已取消」的项。
const retryCount = computed(
  () => items.value.filter((item) => isRetryableStatus(item.result?.status ?? '')).length,
)

// 汇总从 items 派生而非只信最后一次 BatchResult（US-43）：
// 重跑只合并回被重跑的项，合并后 items 才是唯一真相。
function refreshBatchSummary() {
  const results = items.value.map((item) => item.result).filter((r): r is ItemResult => r !== null)
  if (results.length === 0) {
    batchSummary.value = null
    return
  }
  let success = 0
  let failed = 0
  let skipped = 0
  let canceled = 0
  let extreme = 0
  let saved = 0
  let sourceTotal = 0
  for (const item of results) {
    switch (item.status) {
      case ItemStatus.Success:
        success++
        sourceTotal += item.source_bytes || 0
        saved += (item.source_bytes || 0) - (item.output_bytes || 0)
        if (item.extreme_degraded) extreme++
        break
      case ItemStatus.Failed:
        failed++
        break
      case ItemStatus.Skipped:
        skipped++
        break
      case ItemStatus.Canceled:
        canceled++
        break
    }
  }
  const parts = [`已完成 ${success}`, `失败 ${failed}`]
  if (skipped > 0) parts.push(`无需处理 ${skipped}`)
  if (canceled > 0) parts.push(`取消 ${canceled}`)
  if (extreme > 0) parts.push(`画质变差了 ${extreme}`)
  let text = `处理完成：${parts.join('、')}`
  // US-27：批次总计给出字节与百分比；输出更大时说「累计增加」。
  if (saved !== 0) {
    const label = saved > 0 ? '累计节省' : '累计增加'
    text += `；${label} ${formatBytes(Math.abs(saved))}`
    if (sourceTotal > 0) {
      const percent = Math.round((Math.abs(saved) / sourceTotal) * 100)
      if (percent > 0) text += `（${percent}%）`
    }
  }
  batchSummary.value = text
}

function applyBatchResult(result: BatchResult) {
  for (const item of result.items) applyItemResult(item)
  processed.value = result.items.length
  total.value = result.items.length
  resultsStale.value = false
  refreshBatchSummary()
  logInfo('批量处理完成', {
    success: result.success,
    failed: result.failed,
    canceled: result.canceled,
  })
}

// US-43/44：队列内重跑失败与未完成项。Go 侧 ProcessBatch 本就无状态、接受任意
// 子集，重跑即「以当前规则与输出计划再发一个只含目标项的批次」，返回结果只合并
// 回这些项。不构成任务历史：清空队列即消失（D-38，对「明确不做」的范围澄清）。
async function runRetry(targets: QueueItem[]) {
  if (busy.value || targets.length === 0) return
  processing.value = true
  canceling.value = false
  processed.value = 0
  total.value = targets.length
  actionError.value = null
  for (const item of targets) {
    item.result = null
    item.processing = false
    item.queued = true
  }
  try {
    await flushSettingsSave()
    const request = new service.BatchRequest({
      items: targets.map((item) => ({ path: item.path, rel_path: item.rel_path ?? '' })),
      rules: currentRules.value,
      output: outputPlan.value,
    })
    const result = await invoke(() => ProcessBatch(request), 'ProcessBatch')
    for (const item of result.items) applyItemResult(item)
    processed.value = total.value
    refreshBatchSummary()
    logInfo('重跑完成', {
      retried: result.items.length,
      failed: result.failed,
      canceled: result.canceled,
    })
  } catch (e) {
    actionError.value = e as AppError
    logError('重跑失败', e)
  } finally {
    processing.value = false
    canceling.value = false
    for (const item of items.value) {
      item.queued = false
      item.processing = false
    }
  }
}

async function retryUnfinished() {
  const targets = items.value.filter((item) => isRetryableStatus(item.result?.status ?? ''))
  await runRetry(targets)
}

async function retryOne(path: string) {
  if (busy.value) return
  const target = items.value.find((item) => item.path === path)
  if (!target) return
  await runRetry([target])
}

async function startBatch() {
  if (!canStart.value) return
  // Items 携带导入时的相对路径：保留结构模式（US-40）靠它重建目录层级。
  const batchItems = items.value
    .filter((item) => !item.error)
    .map((item) => ({ path: item.path, rel_path: item.rel_path ?? '' }))
  if (batchItems.length === 0) return
  processing.value = true
  canceling.value = false
  processed.value = 0
  total.value = batchItems.length
  batchSummary.value = null
  actionError.value = null
  importNote.value = null
  const inBatch = new Set(batchItems.map((item) => item.path))
  for (const item of items.value) {
    item.result = null
    item.processing = false
    // US-14：开跑即进入「排队」，之后由 Go 的 started 事件升级为「处理中」。
    item.queued = inBatch.has(item.path)
  }
  resultsStale.value = false
  try {
    await flushSettingsSave()
    const request = new service.BatchRequest({
      items: batchItems,
      rules: currentRules.value,
      output: outputPlan.value,
    })
    const result = await invoke(() => ProcessBatch(request), 'ProcessBatch')
    applyBatchResult(result)
  } catch (e) {
    actionError.value = e as AppError
    logError('批量处理失败', e)
  } finally {
    processing.value = false
    canceling.value = false
    // 批次结束或异常退出都不该留下过程态（否则「处理中」会永远挂着）。
    for (const item of items.value) {
      item.queued = false
      item.processing = false
    }
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

function onOutputModeChange(event: Event) {
  outputMode.value = (event.target as HTMLSelectElement).value as OutputModeValue
}

function onOutputPattern(event: Event) {
  outputPattern.value = (event.target as HTMLInputElement).value.trim()
}

async function chooseOutputDir() {
  try {
    const directory = await invoke(() => SelectDirectory('选择输出目录'), 'SelectDirectory')
    if (directory) outputDir.value = directory
  } catch (e) {
    actionError.value = e as AppError
    logError('打开输出目录选择器失败', e)
  }
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
  markResultsStale()
  refreshBatchSummary()
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
  resultsStale.value = false
  actionError.value = null
}

onMounted(async () => {
  void loadPresets()
  void loadSupportedFormats()
  void loadImageSettings()
  try {
    const rt = await import('../../../wailsjs/runtime/runtime')
    if (typeof rt.OnFileDrop !== 'function' || disposed) return
    // 第二个参数为 false：整窗接收拖拽。队列非空时 dropzone 不渲染，
    // 若要求 CSS drop-target 会导致有队列时拖拽静默失效。
    rt.OnFileDrop((_x, _y, paths) => {
      void importPaths(paths)
    }, false)
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
  if (settingsTimer) {
    clearTimeout(settingsTimer)
    settingsTimer = null
    void saveImageSettings()
  }
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

.presets {
  flex: none;
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex-wrap: wrap;
}

.select--preset {
  min-width: 150px;
  max-width: 220px;
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

.input--pattern {
  width: 150px;
  text-align: left;
}

.btn--dir {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rules__example {
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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

.notice--stale {
  border-color: var(--color-warning);
}

.notice__retry {
  flex: none;
  border: 1px solid var(--border-subtle);
  background: transparent;
  color: var(--color-warning);
  font-family: inherit;
  font-size: var(--font-size-sm);
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
  cursor: pointer;
  white-space: nowrap;
  transition: border-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard);
}

.notice__retry:hover:not(:disabled) {
  border-color: var(--color-warning);
  color: var(--text-1);
}

.notice__retry:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
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
