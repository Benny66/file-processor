<template>
  <Teleport to="body">
    <div v-if="open && item" class="tune__overlay" @click.self="emit('close')">
      <div ref="panel" class="tune" role="dialog" aria-label="单张试一试">
        <header class="tune__head">
          <div class="tune__title" :title="item.path">单张试一试 · {{ item.name }}</div>
          <button class="tune__close" type="button" aria-label="关闭单张试一试" @click="emit('close')">
            ×
          </button>
        </header>

        <div class="tune__body">
          <!-- 左栏：规则（可编辑副本）+ 试算结果 + 动作。D-39：只调三类规则，质量作为结果展示。 -->
          <div class="tune__controls">
            <label class="field" for="tune-format">
              <span class="field__label">输出格式</span>
              <select id="tune-format" class="select" :value="format" @change="onFormat">
                <option v-for="option in formatOptions" :key="option.value" :value="option.value">
                  {{ option.label }}
                </option>
              </select>
            </label>
            <label class="field field--check">
              <input type="checkbox" class="checkbox" :checked="boxEnabled" @change="onBoxToggle" />
              <span class="field__label">限制尺寸</span>
            </label>
            <template v-if="boxEnabled">
              <input
                class="input"
                type="number"
                :min="BoxDimension.Min"
                :max="BoxDimension.Max"
                step="1"
                aria-label="最大宽度"
                :value="maxWidth"
                @input="onMaxWidth"
              />
              <span class="field__sep">×</span>
              <input
                class="input"
                type="number"
                :min="BoxDimension.Min"
                :max="BoxDimension.Max"
                step="1"
                aria-label="最大高度"
                :value="maxHeight"
                @input="onMaxHeight"
              />
              <span class="field__unit">px</span>
            </template>
            <label class="field field--check">
              <input type="checkbox" class="checkbox" :checked="targetEnabled" @change="onTargetToggle" />
              <span class="field__label">目标体积</span>
            </label>
            <template v-if="targetEnabled">
              <input
                class="input input--target"
                type="number"
                min="1"
                step="1"
                aria-label="目标体积"
                :value="targetValue"
                @input="onTargetValue"
              />
              <select class="select select--unit" :value="targetUnit" @change="onTargetUnit">
                <option value="KB">KB</option>
                <option value="MB">MB</option>
              </select>
            </template>

            <p v-if="busyHint" class="tune__hint">{{ busyHint }}</p>
            <!-- 结果：体积/尺寸/质量都来自 Go 的真实试算，预览图不参与测量（US-46）。 -->
            <p v-else-if="trialBusy" class="tune__hint">正在试效果…</p>
            <p v-else-if="trialError" class="tune__hint tune__hint--error">{{ trialError }}</p>
            <div v-else-if="trial" class="tune__result">
              <p class="tune__numbers">{{ resultLine }}</p>
              <p v-if="trial.message" class="tune__hint">{{ trial.message }}</p>
              <div class="tune__badges">
                <span v-if="trial.extreme_degraded" class="tune__badge tune__badge--warn">已明显降质</span>
                <span v-if="trial.alpha_flattened" class="tune__badge">透明背景已铺白</span>
              </div>
            </div>
            <p v-else class="tune__hint">改一改设置，马上能看到效果；大小和清晰度都是真实结果。</p>

            <!-- US-50 三格式竞速：同一张图按当前规则分别跑 JPEG/WebP/PNG，最小行高亮，点选采用。 -->
            <button
              class="btn btn--race"
              type="button"
              :disabled="!!busyHint || raceBusy"
              @click="runRace"
            >
              {{ raceBusy ? '比一比中…' : '三种格式比一比' }}
            </button>
            <table v-if="raceRows.length > 0" class="tune__race">
              <thead>
                <tr><th>格式</th><th>体积</th><th>质量</th><th>尺寸</th><th></th></tr>
              </thead>
              <tbody>
                <tr
                  v-for="row in raceRows"
                  :key="row.format"
                  :class="{ 'tune__race-row--best': row.isBest }"
                  :title="row.note"
                >
                  <td>{{ row.label }}{{ row.isBest ? ' ✓' : '' }}</td>
                  <td>{{ row.bytes > 0 ? formatBytes(row.bytes) : '—' }}</td>
                  <td>{{ row.quality > 0 ? row.quality : '—' }}</td>
                  <td>{{ row.dims }}</td>
                  <td>
                    <button
                      class="tune__adopt"
                      type="button"
                      :disabled="!row.adoptable"
                      @click="adoptRaceFormat(row.format)"
                    >
                      采用
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- 右栏：滑块对比 + 预览级放大（D-40）。底层源图，上层当前规则输出，分割线拖动对比。 -->
          <div
            class="tune__stage checkerboard"
            @pointerdown="onPanStart"
            @pointermove="onPanMove"
            @pointerup="endDrag"
            @pointercancel="endDrag"
            @wheel.prevent="onWheel"
          >
            <div class="tune__zoom" :style="zoomStyle">
              <img v-if="sourceURL" class="tune__img" :src="sourceURL" alt="原图" draggable="false" />
              <img
                v-if="outputURL"
                class="tune__img tune__img--top"
                :src="outputURL"
                alt="当前规则输出"
                draggable="false"
                :style="{ clipPath: `inset(0 0 0 ${split}%` + ')' }"
              />
            </div>
            <div
              class="tune__divider"
              :style="{ left: split + '%' }"
              @pointerdown.stop.prevent="onSplitStart"
              @pointermove="onSplitMove"
              @pointerup="endDrag"
            >
              <span class="tune__handle" aria-hidden="true">⇔</span>
            </div>
            <span class="tune__zoomlabel">{{ zoomLabel }}</span>
            <span v-if="!sourceURL" class="tune__loading">加载预览…</span>
          </div>
        </div>

        <footer class="tune__foot">
          <input
            v-model="presetName"
            class="input input--name"
            type="text"
            placeholder="预设名称"
            maxlength="100"
          />
          <button class="btn" type="button" :disabled="!presetName.trim() || saveBusy" @click="saveAsPreset">
            {{ saveBusy ? '保存中…' : '存为预设' }}
          </button>
          <span class="tune__spacer" />
          <button class="btn btn--primary" type="button" @click="applyToAll">应用到全部</button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, toRef, watch } from 'vue'
import { PreviewConvert, RenderPreview, SavePreset } from '../../../wailsjs/go/main/App'
import { model, service } from '../../../wailsjs/go/models'
import { useModalFocus } from '../../composables/useModalFocus'
import { BoxDimension, OutputFormat, type OutputFormatValue } from '../../lib/image'
import { invoke } from '../../lib/invoke'
import { logInfo, logWarn } from '../../lib/log'
import { formatBytes } from './format'
import type { PresetRules, QueueItem } from './types'

// 调参工作台（US-45~48）：选中一张图 → 改规则副本实时试算（PreviewConvert，不落盘）
// → 满意后「应用到全部」或「存为预设」。批次运行中试算被 Go 拒绝（D-41），这里给出提示。
const props = defineProps<{
  open: boolean
  item: QueueItem | null
  /** 批次/导入进行中：试算禁用（D-41）。 */
  busy: boolean
  /** 打开时的规则种子（工作台当前规则）。 */
  rules: PresetRules
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'apply', rules: PresetRules): void
  (e: 'saved'): void
}>()

const panel = ref<HTMLElement | null>(null)
useModalFocus(toRef(props, 'open'), panel, () => emit('close'))

// ---- 规则的可编辑副本（打开时从种子复制；改动只影响本弹窗，直到「应用/存预设」） ----
const format = ref<OutputFormatValue>(OutputFormat.Keep)
const boxEnabled = ref(false)
const maxWidth = ref(1920)
const maxHeight = ref(1080)
const targetEnabled = ref(false)
const targetValue = ref(500)
const targetUnit = ref<'KB' | 'MB'>('KB')

const formatOptions: { value: OutputFormatValue; label: string }[] = [
  { value: OutputFormat.Keep, label: '保持原格式' },
  { value: OutputFormat.JPEG, label: 'JPEG' },
  { value: OutputFormat.PNG, label: 'PNG' },
  { value: OutputFormat.WebP, label: 'WebP' },
  { value: OutputFormat.Auto, label: '自动（最小体积，较慢）' },
]

function splitBytes(bytes: number): { value: number; unit: 'KB' | 'MB' } {
  const megabyte = 1024 * 1024
  if (bytes >= megabyte && bytes % megabyte === 0) return { value: bytes / megabyte, unit: 'MB' }
  return { value: Math.max(1, Math.round(bytes / 1024)), unit: 'KB' }
}

const localRules = computed<PresetRules>(() => ({
  output_format: format.value,
  target_bytes: targetEnabled.value
    ? targetUnit.value === 'MB'
      ? targetValue.value * 1024 * 1024
      : targetValue.value * 1024
    : 0,
  max_width: boxEnabled.value ? maxWidth.value : 0,
  max_height: boxEnabled.value ? maxHeight.value : 0,
}))

// ---- 试算：300ms 防抖 + token 最新胜出（规则预检/设置保存同款模式） ----
type TuneResult = Awaited<ReturnType<typeof PreviewConvert>>
const trial = ref<TuneResult | null>(null)
const trialBusy = ref(false)
const trialError = ref('')
let trialToken = 0
let trialTimer: ReturnType<typeof setTimeout> | null = null

const busyHint = computed(() =>
  props.busy ? '正在处理图片，等它完成后再来试效果' : '',
)

async function runTrial() {
  if (!props.item) return
  const token = ++trialToken
  trialBusy.value = true
  trialError.value = ''
  try {
    // new service.BatchRules(...) 构造生成类实例（嵌套字段经 convertValues）。
    const result = await invoke(
      () => PreviewConvert(props.item!.path, new service.BatchRules(localRules.value)),
      'PreviewConvert',
    )
    if (token !== trialToken) return
    trial.value = result
  } catch (e) {
    if (token !== trialToken) return
    trial.value = null
    trialError.value = e instanceof Error ? e.message : '试效果暂时不可用'
    logWarn('试算失败', e)
  } finally {
    if (token === trialToken) trialBusy.value = false
  }
}

function scheduleTrial() {
  if (trialTimer) clearTimeout(trialTimer)
  trialTimer = setTimeout(() => {
    trialTimer = null
    if (props.busy) return
    void runTrial()
  }, 300)
}

watch(localRules, () => {
  if (!props.open) return
  scheduleTrial()
})

// ---- 源图预览：每张图加载一次，最大边 2048（imageproc 会夹取；D-40 的缩放底图） ----
const sourceURL = ref('')
let previewToken = 0
watch(
  () => [props.open, props.item?.path],
  ([open]) => {
    if (!open || !props.item) return
    const token = ++previewToken
    sourceURL.value = ''
    void (async () => {
      try {
        const preview = await invoke(() => RenderPreview(props.item!.path, 2048), 'RenderPreview')
        if (token === previewToken) sourceURL.value = preview.data_url
      } catch (e) {
        logWarn('加载源图预览失败', e)
      }
    })()
  },
)

// 打开时复制种子规则并立即试算一次。
watch(
  () => props.open,
  (open) => {
    if (!open || !props.item) return
    // 种子的格式取值经已知选项过滤（配置可能带历史/异常值，回落保持原格式）。
    const seedFormat = formatOptions.find((option) => option.value === props.rules.output_format)
    format.value = seedFormat ? seedFormat.value : OutputFormat.Keep
    if (props.rules.target_bytes > 0) {
      const split = splitBytes(props.rules.target_bytes)
      targetEnabled.value = true
      targetValue.value = split.value
      targetUnit.value = split.unit
    } else {
      targetEnabled.value = false
    }
    if (props.rules.max_width > 0 && props.rules.max_height > 0) {
      boxEnabled.value = true
      maxWidth.value = props.rules.max_width
      maxHeight.value = props.rules.max_height
    } else {
      boxEnabled.value = false
    }
    presetName.value = ''
    trial.value = null
    trialError.value = ''
    void runTrial()
  },
)

const outputURL = computed(() =>
  trial.value && trial.value.status === 'success' ? trial.value.preview_url : '',
)

const resultLine = computed(() => {
  const t = trial.value
  if (!t) return ''
  if (t.status === 'skipped') return t.message
  if (t.status === 'failed') return t.message
  const source = props.item?.size ?? 0
  const parts = [
    `${t.format.toUpperCase()} · ${formatBytes(t.output_bytes)}`,
    `${t.width} × ${t.height}`,
  ]
  if (t.quality > 0) parts.push(`质量 ${t.quality}`)
  if (source > 0) {
    const saved = source - t.output_bytes
    const percent = Math.round((Math.abs(saved) / source) * 100)
    if (percent > 0) parts.push(`${saved > 0 ? '节省' : '增大'} ${percent}%`)
  }
  return parts.join(' · ')
})

// ---- 滑块对比与预览级放大：底层源图 / 上层输出，clip-path 随分割线裁剪；双图同一变换 ----
const split = ref(50)
const zoom = ref(1)
const panX = ref(0)
const panY = ref(0)
let dragging: 'pan' | 'split' | null = null
let lastX = 0
let lastY = 0
let stageEl: HTMLElement | null = null

const zoomStyle = computed(() => ({
  transform: `translate(${panX.value}px, ${panY.value}px) scale(${zoom.value})`,
}))

const zoomLabel = computed(() => `${Math.round(zoom.value * 100)}%`)

function onWheel(event: WheelEvent) {
  const next = zoom.value * (event.deltaY < 0 ? 1.2 : 1 / 1.2)
  zoom.value = Math.min(4, Math.max(1, next))
  if (zoom.value === 1) {
    panX.value = 0
    panY.value = 0
  }
}

function onPanStart(event: PointerEvent) {
  if (zoom.value === 1 || !event.currentTarget) return
  dragging = 'pan'
  stageEl = event.currentTarget as HTMLElement
  lastX = event.clientX
  lastY = event.clientY
  stageEl.setPointerCapture(event.pointerId)
}

function onSplitStart(event: PointerEvent) {
  dragging = 'split'
  lastX = event.clientX
  stageEl = (event.currentTarget as HTMLElement).parentElement
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}

function onPanMove(event: PointerEvent) {
  if (dragging !== 'pan') return
  panX.value += event.clientX - lastX
  panY.value += event.clientY - lastY
  lastX = event.clientX
  lastY = event.clientY
}

function onSplitMove(event: PointerEvent) {
  if (dragging !== 'split' || !stageEl) return
  const rect = stageEl.getBoundingClientRect()
  const pct = ((event.clientX - rect.left) / rect.width) * 100
  split.value = Math.min(100, Math.max(0, pct))
}

function endDrag() {
  dragging = null
}

// ---- 动作：存为预设（复用 SavePreset 全套校验）/ 应用到全部（只改参数不开跑，US-48） ----
const presetName = ref('')
const saveBusy = ref(false)

async function saveAsPreset() {
  const name = presetName.value.trim()
  if (!name || saveBusy.value) return
  saveBusy.value = true
  try {
    await invoke(
      () => SavePreset(new model.ImagePreset({ name, ...localRules.value })),
      'SavePreset',
    )
    presetName.value = ''
    emit('saved')
    logInfo('调参工作台已存为预设', { name })
  } catch (e) {
    logWarn('保存预设失败', e)
    trialError.value = e instanceof Error ? e.message : '保存预设失败'
  } finally {
    saveBusy.value = false
  }
}

function applyToAll() {
  emit('apply', localRules.value)
  emit('close')
}

// ---- 三格式竞速（US-50）：三次试算并行；失败格式行显示原因，成功行按体积高亮最小 ----
interface RaceRow {
  format: OutputFormatValue
  label: string
  bytes: number
  quality: number
  dims: string
  note: string
  adoptable: boolean
  isBest: boolean
}

const RACE_FORMATS: { value: OutputFormatValue; label: string }[] = [
  { value: OutputFormat.JPEG, label: 'JPEG' },
  { value: OutputFormat.WebP, label: 'WebP' },
  { value: OutputFormat.PNG, label: 'PNG' },
]

const raceBusy = ref(false)
const raceRows = ref<RaceRow[]>([])

async function runRace() {
  if (raceBusy.value || busyHint.value || !props.item) return
  raceBusy.value = true
  raceRows.value = []
  try {
    const settled = await Promise.all(
      RACE_FORMATS.map(async ({ value, label }) => {
        try {
          const result = await invoke(
            () =>
              PreviewConvert(
                props.item!.path,
                new service.BatchRules({ ...localRules.value, output_format: value }),
              ),
            'PreviewConvert',
          )
          return { value, label, result }
        } catch {
          // 单格式试算的传输级失败不拖垮整表：该行以失败态呈现。
          return { value, label, result: null }
        }
      }),
    )
    const okBytes = settled
      .filter((entry) => entry.result?.status === 'success')
      .map((entry) => entry.result!.output_bytes)
    const minBytes = okBytes.length > 0 ? Math.min(...okBytes) : -1
    raceRows.value = settled.map(({ value, label, result }) => {
      if (result?.status === 'success') {
        return {
          format: value,
          label,
          bytes: result.output_bytes,
          quality: result.quality,
          dims: `${result.width} × ${result.height}`,
          note: result.message || '达标',
          adoptable: true,
          isBest: result.output_bytes === minBytes,
        }
      }
      return {
        format: value,
        label,
        bytes: 0,
        quality: 0,
        dims: '—',
        note: result?.message ?? '试效果失败',
        adoptable: false,
        isBest: false,
      }
    })
  } finally {
    raceBusy.value = false
  }
}

// 采用某行格式：改本地格式即触发试算 watch，「体积最小」的结论交给新一轮试算复核。
function adoptRaceFormat(next: OutputFormatValue) {
  format.value = next
}

function onFormat(event: Event) {
  format.value = (event.target as HTMLSelectElement).value as OutputFormatValue
}
function onBoxToggle(event: Event) {
  boxEnabled.value = (event.target as HTMLInputElement).checked
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
</script>

<style scoped>
.tune__overlay {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-6);
  background: color-mix(in oklab, var(--bg-base) 72%, transparent);
}

.tune {
  display: flex;
  flex-direction: column;
  width: min(1080px, 100%);
  max-height: min(760px, 100%);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-raised);
  box-shadow: var(--shadow-lg);
  overflow: hidden;
}

.tune__head {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
}

.tune__title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-1);
  font-size: var(--font-size-md);
}

.tune__close {
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

.tune__close:hover {
  color: var(--text-1);
}

.tune__body {
  flex: 1;
  min-height: 0;
  display: flex;
  gap: var(--space-4);
  padding: var(--space-4);
}

.tune__controls {
  flex: none;
  width: 300px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  align-content: flex-start;
  gap: var(--space-3);
  overflow-y: auto;
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

.select {
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-raised);
  color: var(--text-1);
  padding: var(--space-2) var(--space-3);
  font-family: inherit;
  font-size: var(--font-size-sm);
  cursor: pointer;
}

.select--unit {
  width: 78px;
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
}

.input--target {
  width: 100px;
}

.input--name {
  width: 180px;
  text-align: left;
}

.field__sep,
.field__unit {
  color: var(--text-3);
  font-size: var(--font-size-sm);
}

.tune__hint {
  width: 100%;
  margin: 0;
  color: var(--text-3);
  font-size: var(--font-size-xs);
}

.tune__hint--error {
  color: var(--color-danger);
}

.tune__result {
  width: 100%;
}

.tune__numbers {
  margin: 0 0 var(--space-2);
  color: var(--text-1);
  font-size: var(--font-size-sm);
  font-variant-numeric: tabular-nums;
}

.tune__badges {
  display: flex;
  gap: var(--space-2);
  flex-wrap: wrap;
}

.tune__badge {
  padding: 2px var(--space-2);
  border-radius: var(--radius-full);
  background: var(--bg-hover);
  color: var(--text-2);
  font-size: var(--font-size-xs);
}

.tune__badge--warn {
  color: var(--color-warning);
}

.tune__stage {
  position: relative;
  flex: 1;
  min-width: 0;
  min-height: 320px;
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  --checker-size: 16px;
  touch-action: none;
  cursor: grab;
}

.tune__zoom {
  position: absolute;
  inset: 0;
  transform-origin: center center;
}

.tune__img {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: contain;
  user-select: none;
  -webkit-user-drag: none;
}

.tune__img--top {
  z-index: 1;
}

.tune__divider {
  position: absolute;
  top: 0;
  bottom: 0;
  z-index: 2;
  width: 2px;
  background: var(--color-primary);
  cursor: ew-resize;
}

.tune__handle {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-full);
  background: var(--color-primary);
  color: var(--color-on-primary);
  font-size: var(--font-size-sm);
}

.tune__zoomlabel {
  position: absolute;
  right: var(--space-2);
  bottom: var(--space-2);
  z-index: 2;
  padding: 2px var(--space-2);
  border-radius: var(--radius-full);
  background: var(--bg-hover);
  color: var(--text-3);
  font-size: var(--font-size-xs);
  font-variant-numeric: tabular-nums;
}

.tune__loading {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-3);
  font-size: var(--font-size-sm);
}

.tune__foot {
  flex: none;
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  border-top: 1px solid var(--border-subtle);
}

.btn--race {
  width: 100%;
}

.tune__race {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--font-size-xs);
  font-variant-numeric: tabular-nums;
}

.tune__race th,
.tune__race td {
  padding: var(--space-1) var(--space-2);
  text-align: left;
  border-bottom: 1px solid var(--border-subtle);
  color: var(--text-2);
  white-space: nowrap;
}

.tune__race th {
  color: var(--text-3);
  font-weight: normal;
}

.tune__race-row--best td {
  color: var(--color-primary);
  background: var(--color-primary-soft);
}

.tune__adopt {
  border: 1px solid var(--border-subtle);
  background: transparent;
  color: var(--text-2);
  font-family: inherit;
  font-size: var(--font-size-xs);
  padding: 2px var(--space-2);
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.tune__adopt:hover:not(:disabled) {
  border-color: var(--color-primary-border);
  color: var(--color-primary);
}

.tune__adopt:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.tune__spacer {
  flex: 1;
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
}

.btn:hover:not(:disabled) {
  border-color: var(--color-primary-border);
  color: var(--text-1);
}

.btn--primary {
  border-color: var(--color-primary);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

@media (max-width: 720px) {
  .tune__body {
    flex-direction: column;
  }

  .tune__controls {
    width: auto;
  }
}
</style>
