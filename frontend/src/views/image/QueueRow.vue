<template>
  <li ref="root" class="row">
    <div
      class="thumb"
      :class="{ 'thumb--failed': item.error, 'thumb--pending': !item.info && !item.error }"
    >
      <img v-if="item.info" :src="item.info.thumbnail" alt="" />
      <span v-else class="thumb__mark" aria-hidden="true">{{ item.error ? '!' : '…' }}</span>
    </div>

    <div class="row__main">
      <p class="row__name" :title="item.path">{{ item.name }}</p>
      <div class="row__meta">
        <span class="row__path" :title="item.path">{{ item.path }}</span>
        <template v-if="item.info">
          <span class="row__sep">·</span>
          <span class="row__dims">{{ item.info.width }} × {{ item.info.height }}</span>
          <span class="row__sep">·</span>
          <span class="row__format">{{ formatLabel(item.info.format) }}</span>
        </template>
      </div>
      <p v-if="item.error" class="row__error" :title="item.error">{{ item.error }}</p>
      <p v-else-if="warning" class="row__warning" :title="warning">{{ warning }}</p>
    </div>

    <span class="row__size">{{ formatBytes(item.size) }}</span>
    <span class="row__status" :class="statusClass">{{ statusLabel }}</span>
    <button class="row__remove" type="button" :disabled="busy" @click="emit('remove', item.path)">
      移除
    </button>
  </li>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import type { QueueItem } from './types'

const props = defineProps<{ item: QueueItem; busy: boolean }>()
const emit = defineEmits<{
  (e: 'inspect', path: string): void
  (e: 'remove', path: string): void
}>()

const root = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

const statusLabel = computed(() => {
  if (props.item.inspecting) return '检查中'
  if (props.item.error) return '失败'
  if (props.item.info) return '可处理'
  return '待检查'
})

const statusClass = computed(() => {
  if (props.item.error) return 'row__status--failed'
  if (props.item.inspecting) return 'row__status--busy'
  if (props.item.info) return 'row__status--ok'
  return ''
})

const warning = computed(() => props.item.info?.warning ?? '')

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

.row__size {
  flex: none;
  color: var(--text-2);
  font-size: var(--font-size-sm);
  font-variant-numeric: tabular-nums;
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
