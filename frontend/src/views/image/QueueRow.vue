<template>
  <li ref="root" class="row">
    <div
      class="thumb"
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
    <button class="row__remove" type="button" :disabled="busy" @click="emit('remove', item.path)">
      移除
    </button>
  </li>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ItemStatus } from '../../lib/image'
import { formatBytes } from './format'
import type { QueueItem } from './types'

const props = defineProps<{ item: QueueItem; busy: boolean }>()
const emit = defineEmits<{
  (e: 'inspect', path: string): void
  (e: 'remove', path: string): void
}>()

const root = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

const statusLabel = computed(() => {
  const result = props.item.result
  if (result) {
    switch (result.status) {
      case ItemStatus.Success:
        return '成功'
      case ItemStatus.Failed:
        return '失败'
      case ItemStatus.Skipped:
        return '已跳过'
      case ItemStatus.Canceled:
        return '已取消'
    }
  }
  if (props.item.inspecting) return '检查中'
  if (props.item.error) return '失败'
  if (props.item.info) return '可处理'
  return '待检查'
})

const statusClass = computed(() => {
  const result = props.item.result
  if (result) {
    switch (result.status) {
      case ItemStatus.Success:
        return 'row__status--ok'
      case ItemStatus.Failed:
        return 'row__status--failed'
      case ItemStatus.Skipped:
        return 'row__status--skipped'
      case ItemStatus.Canceled:
        return 'row__status--canceled'
    }
  }
  if (props.item.error) return 'row__status--failed'
  if (props.item.inspecting) return 'row__status--busy'
  if (props.item.info) return 'row__status--ok'
  return ''
})

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
  if (props.item.result?.status === ItemStatus.Skipped) {
    return props.item.result.message || '已跳过'
  }
  return props.item.info?.warning ?? ''
})

const dimensions = computed(() => {
  if (props.item.info) {
    return `${props.item.info.width} × ${props.item.info.height}`
  }
  const result = props.item.result
  if (result?.status === ItemStatus.Success && result.width > 0) {
    return `${result.width} × ${result.height}`
  }
  return ''
})

const outputSize = computed(() => {
  const result = props.item.result
  if (result?.status !== ItemStatus.Success) return ''
  return formatBytes(result.output_bytes)
})

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
  background-color: var(--bg-raised);
  background-image:
    linear-gradient(45deg, var(--border-subtle) 25%, transparent 25%),
    linear-gradient(-45deg, var(--border-subtle) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, var(--border-subtle) 75%),
    linear-gradient(-45deg, transparent 75%, var(--border-subtle) 75%);
  background-size: 8px 8px;
  background-position: 0 0, 0 4px, 4px -4px, -4px 0;
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

.row__status--busy {
  color: var(--text-2);
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
