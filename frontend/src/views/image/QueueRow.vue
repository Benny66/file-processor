<template>
  <li ref="root" class="row">
    <div
      class="thumb checkerboard"
      :class="{ 'thumb--failed': hasFailure, 'thumb--pending': !item.info && !hasFailure }"
    >
      <img v-if="item.info" :src="item.info.thumbnail" alt="" />
      <span v-else class="thumb__mark" aria-hidden="true">{{ hasFailure ? '!' : '…' }}</span>
    </div>

    <div class="row__main">
      <p class="row__name" :title="item.path">{{ item.name }}</p>
      <div class="row__meta">
        <span class="row__path" :title="item.path">{{ item.path }}</span>
        <template v-if="dimensions || item.info">
          <span v-if="dimensions" class="row__sep">·</span>
          <span v-if="dimensions" class="row__dims">{{ dimensions }}</span>
          <template v-if="item.info">
            <span class="row__sep">·</span>
            <span class="row__format">{{ formatLabel(item.info.format) }}</span>
          </template>
        </template>
      </div>
      <p v-if="error" class="row__error" :title="error">{{ error }}</p>
      <p v-else-if="warning" class="row__warning" :title="warning">{{ warning }}</p>
    </div>

    <div class="row__sizes">
      <span class="row__size">{{ formatBytes(item.size) }}</span>
      <span v-if="outputSize" class="row__output">→ {{ outputSize }}</span>
    </div>
    <span class="row__status" :class="statusClass">{{ statusLabel }}</span>
    <span v-if="item.result && stale" class="row__stale">结果已过期</span>
    <span v-if="item.result?.extreme_degraded" class="row__extreme">已明显降质</span>
    <!-- US-14：处理期间仍要能查看结果，所以对比不受 busy 限制；
         US-06 只禁止「编辑队列」，移除按钮才需要 busy。 -->
    <button
      v-if="canCompare"
      class="row__compare"
      type="button"
      @click="emit('compare', item.path)"
    >
      对比
    </button>
    <!-- US-44：失败/已取消行可单独重跑；批次运行中禁用（US-06 只禁编辑队列，重跑是新批次）。 -->
    <button
      v-if="canRetry"
      class="row__compare row__retry"
      type="button"
      :disabled="busy"
      @click="emit('retry', item.path)"
    >
      重跑
    </button>
    <!-- US-45：已检查的图可进调参工作台；批次运行中禁用（D-41：避免与 worker 抢 CPU）。 -->
    <button
      v-if="item.info && !item.error"
      class="row__compare row__tune"
      type="button"
      :disabled="busy"
      @click="emit('tune', item.path)"
    >
      调参
    </button>
    <button class="row__remove" type="button" :disabled="busy" @click="emit('remove', item.path)">
      移除
    </button>
  </li>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ItemStatus, type ItemStatusValue } from '../../lib/image'
import { formatBytes } from './format'
import { isRetryableStatus, type QueueItem } from './types'

const props = defineProps<{ item: QueueItem; busy: boolean; stale: boolean }>()
const emit = defineEmits<{
  (e: 'inspect', path: string): void
  (e: 'remove', path: string): void
  (e: 'compare', path: string): void
  (e: 'retry', path: string): void
  (e: 'tune', path: string): void
}>()

const root = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

const statusLabel = computed(() => statusPresentation().label)
const statusClass = computed(() => statusPresentation().cls)

// 状态到「文案 + 样式」的单一映射。
//
// 为什么合并：此前 statusLabel 与 statusClass 各写一份 4 分支 switch，Go 新增
// Status* 时 parity 护栏只校验常量镜像、管不到前端 switch——漏改一个分支不报错，
// 只是在队列里显示「可处理」（铁律 5 说的无症状失效）。合并成一张表后只有改表一处。
// 表里没有的取值原样带出状态名，让「漏登记」在界面上Visible，而不是回落到「可处理」。
function statusPresentation(): { label: string; cls: string } {
  const result = props.item.result
  if (result) {
    const known = RESULT_STATUS[result.status as ItemStatusValue]
    return known ?? { label: `未知状态(${result.status})`, cls: 'row__status--failed' }
  }
  if (props.item.processing) return { label: '处理中', cls: 'row__status--busy' }
  if (props.item.queued) return { label: '排队', cls: 'row__status--queued' }
  if (props.item.inspecting) return { label: '检查中', cls: 'row__status--busy' }
  if (props.item.error) return { label: '失败', cls: 'row__status--failed' }
  if (props.item.info) return { label: '可处理', cls: 'row__status--ok' }
  return { label: '待检查', cls: '' }
}

const RESULT_STATUS: Record<ItemStatusValue, { label: string; cls: string }> = {
  [ItemStatus.Success]: { label: '成功', cls: 'row__status--ok' },
  [ItemStatus.Failed]: { label: '失败', cls: 'row__status--failed' },
  [ItemStatus.Skipped]: { label: '已跳过', cls: 'row__status--skipped' },
  [ItemStatus.Canceled]: { label: '已取消', cls: 'row__status--canceled' },
  [ItemStatus.Processing]: { label: '处理中', cls: 'row__status--busy' },
}

const hasFailure = computed(
  () => props.item.result?.status === ItemStatus.Failed || !!props.item.error,
)

const error = computed(() => {
  if (props.item.result?.status === ItemStatus.Failed) {
    return props.item.result.message || '处理失败'
  }
  return props.item.error ?? ''
})

const warning = computed(() => {
  const parts: string[] = []
  if (props.item.result?.status === ItemStatus.Skipped) {
    parts.push(props.item.result.message || '已跳过')
  }
  // US-49：auto 格式的「自动选择 XX（最优）」随成功结果回传，这里转述。
  if (props.item.result?.status === ItemStatus.Success && props.item.result.message) {
    parts.push(props.item.result.message)
  }
  // US-20：铺白由引擎判定并回传（alpha_flattened），前端只转述不再判断。
  if (props.item.result?.alpha_flattened) {
    parts.push('透明背景已铺白')
  }
  if (props.item.info?.warning) {
    parts.push(props.item.info.warning)
  }
  return parts.join('；')
})

const dimensions = computed(() => {
  const result = props.item.result
  if (result && result.width > 0) {
    return `${result.width} × ${result.height}`
  }
  if (props.item.info) {
    return `${props.item.info.width} × ${props.item.info.height}`
  }
  return ''
})

// US-27：成功项显示输出体积与节省比例；输出更大时显示「增大 x%」（负节省）。
const outputSize = computed(() => {
  const result = props.item.result
  if (result?.status !== ItemStatus.Success) return ''
  const text = formatBytes(result.output_bytes)
  const source = result.source_bytes || props.item.size
  if (source <= 0) return text
  const saved = source - result.output_bytes
  const percent = Math.round((Math.abs(saved) / source) * 100)
  if (percent === 0) return `${text}（体积持平）`
  return `${text}（${saved > 0 ? '节省' : '增大'} ${percent}%）`
})

const canCompare = computed(
  () =>
    props.item.result?.status === ItemStatus.Success && !!props.item.result.output_path,
)

const canRetry = computed(() => isRetryableStatus(props.item.result?.status ?? ''))

function formatLabel(format: string): string {
  return format.toUpperCase()
}

onMounted(() => {
  if (!root.value) return
  observer = new IntersectionObserver(
    (entries) => {
      if (!entries.some((entry) => entry.isIntersecting)) return
      if (props.item.info || props.item.error || props.item.inspecting) return
      emit('inspect', props.item.path)
    },
    { rootMargin: '120px' },
  )
  observer.observe(root.value)
})

onUnmounted(() => {
  observer?.disconnect()
  observer = null
})
</script>

<style scoped>
.row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
  transition: background-color var(--motion-fast) var(--ease-standard);
}

.row:last-child {
  border-bottom: none;
}

.row:hover {
  background: var(--bg-hover);
}

.thumb {
  flex: none;
  width: 44px;
  height: 44px;
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
}

.thumb img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.thumb--pending,
.thumb--failed {
  background-image: none;
  background-color: var(--bg-hover);
}

.thumb--failed {
  border-color: var(--color-danger);
}

.thumb__mark {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-3);
  font-size: var(--font-size-lg);
}

.thumb--failed .thumb__mark {
  color: var(--color-danger);
}

.row__main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.row__name,
.row__meta,
.row__error,
.row__warning {
  margin: 0;
}

.row__name {
  color: var(--text-1);
  font-size: var(--font-size-md);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row__meta {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
  color: var(--text-3);
  font-size: var(--font-size-xs);
}

.row__path {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row__sep,
.row__dims,
.row__format {
  flex: none;
}

.row__format {
  color: var(--text-2);
}

.row__error,
.row__warning {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--font-size-xs);
}

.row__error {
  color: var(--color-danger);
}

.row__warning {
  color: var(--color-warning);
}

.row__sizes {
  flex: none;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}

.row__size {
  color: var(--text-2);
  font-size: var(--font-size-sm);
  font-variant-numeric: tabular-nums;
}

.row__output {
  color: var(--text-2);
  font-size: var(--font-size-xs);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.row__status {
  flex: none;
  padding: 2px var(--space-2);
  border-radius: var(--radius-full);
  background: var(--bg-hover);
  color: var(--text-2);
  font-size: var(--font-size-xs);
}

.row__status--ok {
  color: var(--color-success);
}

.row__status--failed {
  color: var(--color-danger);
}

.row__status--skipped {
  color: var(--color-warning);
}

.row__status--canceled {
  color: var(--text-3);
}

.row__extreme,
.row__stale {
  flex: none;
  padding: 2px var(--space-2);
  border-radius: var(--radius-full);
  background: var(--bg-hover);
  color: var(--color-warning);
  font-size: var(--font-size-xs);
  white-space: nowrap;
}

.row__stale {
  color: var(--text-3);
}

.row__status--busy {
  color: var(--text-2);
}

/* US-14「排队」是比「处理中」更弱的一格：还没轮到它。 */
.row__status--queued {
  color: var(--text-3);
}

.row__compare {
  flex: none;
  border: 1px solid var(--border-subtle);
  background: transparent;
  color: var(--text-2);
  font-family: inherit;
  font-size: var(--font-size-sm);
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: border-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard);
}

.row__compare:hover:not(:disabled) {
  border-color: var(--color-primary-border);
  color: var(--color-primary);
}

.row__compare:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.row__compare:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

.row__retry {
  color: var(--color-warning);
}

.row__tune {
  color: var(--color-primary);
}

.row__remove {
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

.row__remove:hover:not(:disabled) {
  background: var(--bg-hover);
  color: var(--color-danger);
}

.row__remove:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.row__remove:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
</style>
