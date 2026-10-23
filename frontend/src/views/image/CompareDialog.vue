<template>
  <Teleport to="body">
    <div v-if="open" class="compare" @click.self="emit('close')">
      <div
        ref="panel"
        class="compare__panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="compare-title"
        tabindex="-1"
      >
        <header class="compare__head">
          <div class="compare__heading">
            <h2 id="compare-title" class="compare__title">对比</h2>
            <p class="compare__name" :title="item?.path">{{ item?.name }}</p>
          </div>
          <button class="compare__close" type="button" aria-label="关闭对比" @click="emit('close')">
            ×
          </button>
        </header>

        <div class="compare__body">
          <figure class="pane">
            <figcaption class="pane__head">
              <span class="pane__label">原图</span>
              <span class="pane__meta">{{ sourceMeta }}</span>
            </figcaption>
            <div class="pane__stage checkerboard">
              <img v-if="sourcePreview" :src="sourcePreview.data_url" alt="原图预览" />
              <span v-else-if="sourceError" class="pane__state pane__state--error">
                {{ sourceError }}
              </span>
              <span v-else class="pane__state">加载中…</span>
            </div>
          </figure>

          <figure class="pane">
            <figcaption class="pane__head">
              <span class="pane__label">结果</span>
              <span class="pane__meta">{{ resultMeta }}</span>
            </figcaption>
            <div class="pane__stage checkerboard">
              <img v-if="resultPreview" :src="resultPreview.data_url" alt="处理结果预览" />
              <span v-else-if="resultError" class="pane__state pane__state--error">
                {{ resultError }}
              </span>
              <span v-else class="pane__state">加载中…</span>
            </div>
          </figure>
        </div>

        <p class="compare__hint">
          两侧均为方向归一后的预览，已按最长边等比缩小；棋盘格表示透明区域。
        </p>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { RenderPreview } from '../../../wailsjs/go/main/App'
import { invoke, type AppError } from '../../lib/invoke'
import { formatBytes } from './format'
import type { Preview, QueueItem } from './types'

const props = defineProps<{ open: boolean; item: QueueItem | null }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const panel = ref<HTMLElement | null>(null)
const sourcePreview = ref<Preview | null>(null)
const resultPreview = ref<Preview | null>(null)
const sourceError = ref('')
const resultError = ref('')
let loadToken = 0

const sourceMeta = computed(() => {
  const item = props.item
  if (!item) return ''
  const info = item.info
  const dims = info ? `${info.width} × ${info.height}` : '未知尺寸'
  return `${dims} · ${formatBytes(item.size)}`
})

const resultMeta = computed(() => {
  const result = props.item?.result
  if (!result) return ''
  return `${result.width} × ${result.height} · ${formatBytes(result.output_bytes)}`
})

async function loadPreview(path: string, token: number, side: 'source' | 'result') {
  try {
    const preview = await invoke(() => RenderPreview(path, 0), 'RenderPreview')
    if (token !== loadToken) return
    if (side === 'source') sourcePreview.value = preview
    else resultPreview.value = preview
  } catch (e) {
    if (token !== loadToken) return
    const message = (e as AppError).message || '预览加载失败'
    if (side === 'source') sourceError.value = message
    else resultError.value = message
  }
}

async function loadPreviews() {
  const token = ++loadToken
  sourcePreview.value = null
  resultPreview.value = null
  sourceError.value = ''
  resultError.value = ''
  const item = props.item
  if (!props.open || !item) return

  const tasks = [loadPreview(item.path, token, 'source')]
  const outputPath = item.result?.output_path
  if (outputPath) tasks.push(loadPreview(outputPath, token, 'result'))
  else resultError.value = '没有可对比的结果文件'
  await Promise.all(tasks)
}

function onKeydown(event: KeyboardEvent) {
  if (props.open && event.key === 'Escape') {
    event.preventDefault()
    emit('close')
  }
}

watch(
  () => [props.open, props.item?.path, props.item?.result?.output_path],
  () => void loadPreviews(),
  { immediate: true },
)

let lastFocused: HTMLElement | null = null

watch(
  () => props.open,
  (open) => {
    const root = document.getElementById('app')
    if (open) {
      lastFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null
      root?.setAttribute('inert', '')
      void nextTick(() => panel.value?.focus())
      return
    }
    root?.removeAttribute('inert')
    lastFocused?.focus()
    lastFocused = null
  },
)

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  document.getElementById('app')?.removeAttribute('inert')
})
</script>

<style scoped>
.compare {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-6);
  background: color-mix(in oklab, var(--bg-base) 72%, transparent);
}

.compare__panel {
  width: min(1080px, 100%);
  height: min(760px, 100%);
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-4);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-raised);
  box-shadow: var(--shadow-lg);
  outline: none;
}

.compare__head {
  flex: none;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
}

.compare__heading {
  min-width: 0;
}

.compare__title {
  margin: 0;
  font-size: var(--font-size-lg);
}

.compare__name {
  margin: 2px 0 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-3);
  font-size: var(--font-size-xs);
}

.compare__close {
  flex: none;
  border: none;
  background: transparent;
  color: var(--text-3);
  font-family: inherit;
  font-size: var(--font-size-xl);
  line-height: 1;
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
  cursor: pointer;
}

.compare__close:hover {
  background: var(--bg-hover);
  color: var(--text-1);
}

.compare__close:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

.compare__body {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-3);
}

.pane {
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0;
}

.pane__head {
  flex: none;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-2);
  min-width: 0;
}

.pane__label {
  color: var(--text-1);
  font-size: var(--font-size-sm);
}

.pane__meta {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-3);
  font-size: var(--font-size-xs);
  font-variant-numeric: tabular-nums;
}

.pane__stage {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  --checker-size: 16px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
}

.pane__stage img {
  display: block;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.pane__state {
  padding: var(--space-3);
  color: var(--text-3);
  font-size: var(--font-size-sm);
  text-align: center;
}

.pane__state--error {
  color: var(--color-danger);
}

.compare__hint {
  flex: none;
  margin: 0;
  color: var(--text-3);
  font-size: var(--font-size-xs);
}

@media (max-width: 720px) {
  .compare__body {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: repeat(2, minmax(0, 1fr));
  }
}
</style>
