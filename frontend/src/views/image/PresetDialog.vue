<template>
  <Teleport to="body">
    <div v-if="open" class="preset-dialog" @click.self="emit('close')">
      <div
        ref="panel"
        class="preset-dialog__panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="preset-dialog-title"
        tabindex="-1"
      >
        <header class="preset-dialog__head">
          <div>
            <h2 id="preset-dialog-title" class="preset-dialog__title">预设管理</h2>
            <p class="preset-dialog__subtitle">把常用参数保存为预设，应用时只更新参数，不会自动开始处理。</p>
          </div>
          <button
            class="preset-dialog__close"
            type="button"
            aria-label="关闭预设管理"
            @click="emit('close')"
          >
            ×
          </button>
        </header>

        <section class="preset-dialog__save">
          <label class="preset-dialog__label" for="preset-name">预设名称</label>
          <input
            id="preset-name"
            v-model="newName"
            class="preset-dialog__input"
            type="text"
            placeholder="例如：电商主图"
            :disabled="busy"
            @keyup.enter="saveCurrent"
          />
          <button
            class="preset-dialog__button preset-dialog__button--primary"
            type="button"
            :disabled="busy"
            @click="saveCurrent"
          >
            保存当前参数
          </button>
        </section>

        <p v-if="error" class="preset-dialog__error" role="alert">{{ error }}</p>
        <p v-if="loadError" class="preset-dialog__error" role="alert">
          加载预设失败：{{ loadError }}
        </p>

        <div class="preset-dialog__body">
          <ul v-if="presets.length > 0" class="preset-list" aria-label="已保存的预设">
            <li v-for="preset in presets" :key="preset.id" class="preset-row">
              <template v-if="renamingId === preset.id">
                <input
                  v-model="renameValue"
                  class="preset-dialog__input preset-row__input"
                  type="text"
                  :aria-label="`重命名 ${preset.name}`"
                  :disabled="busy"
                  @keyup.enter="confirmRename(preset)"
                />
                <button
                  class="preset-dialog__button preset-dialog__button--primary"
                  type="button"
                  :disabled="busy"
                  @click="confirmRename(preset)"
                >
                  保存
                </button>
                <button
                  class="preset-dialog__button"
                  type="button"
                  :disabled="busy"
                  @click="cancelRename"
                >
                  取消
                </button>
              </template>
              <template v-else>
                <div class="preset-row__info">
                  <span class="preset-row__name" :title="preset.name">{{ preset.name }}</span>
                  <span class="preset-row__rules">{{ describeRules(preset) }}</span>
                </div>
                <button
                  class="preset-dialog__button"
                  type="button"
                  :disabled="busy"
                  @click="emit('apply', preset)"
                >
                  应用
                </button>
                <button
                  class="preset-dialog__button"
                  type="button"
                  :disabled="busy"
                  @click="startRename(preset)"
                >
                  重命名
                </button>
                <button
                  class="preset-dialog__button preset-dialog__button--danger"
                  type="button"
                  :disabled="busy"
                  @click="removePreset(preset)"
                >
                  删除
                </button>
              </template>
            </li>
          </ul>
          <p v-else-if="loading" class="preset-dialog__empty">加载中…</p>
          <p v-else class="preset-dialog__empty">
            暂无预设。输入名称后，可将当前参数保存为第一个预设。
          </p>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, toRef, watch } from 'vue'
import { Confirm, DeletePreset, SavePreset } from '../../../wailsjs/go/main/App'
import { model } from '../../../wailsjs/go/models'
import { useModalFocus } from '../../composables/useModalFocus'
import { invoke, type AppError } from '../../lib/invoke'
import { OutputFormat } from '../../lib/image'
import { formatBytes } from './format'
import type { Preset, PresetRules } from './types'

const props = defineProps<{
  open: boolean
  presets: Preset[]
  rules: PresetRules
  loading: boolean
  loadError: string
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'refresh'): void
  (e: 'apply', preset: Preset): void
}>()

const panel = ref<HTMLElement | null>(null)
const newName = ref('')
const renamingId = ref<number | null>(null)
const renameValue = ref('')
const busy = ref(false)
const error = ref('')

useModalFocus(
  toRef(props, 'open'),
  panel,
  () => emit('close'),
)

watch(
  () => props.open,
  (open) => {
    if (!open) return
    newName.value = ''
    renamingId.value = null
    renameValue.value = ''
    error.value = ''
  },
)

function describeRules(preset: Preset): string {
  let format = preset.output_format.toUpperCase()
  if (preset.output_format === OutputFormat.Keep) format = '保持原格式'
  if (preset.output_format === OutputFormat.Auto) format = '自动（最小体积）'
  const target = preset.target_bytes > 0 ? `目标 ${formatBytes(preset.target_bytes)}` : '不限体积'
  const size = preset.max_width > 0 ? `${preset.max_width}×${preset.max_height}` : '不限尺寸'
  return `${format} · ${target} · ${size}`
}

function presetPayload(preset: Preset, name: string): model.ImagePreset {
  return new model.ImagePreset({
    id: preset.id,
    name,
    output_format: preset.output_format,
    target_bytes: preset.target_bytes,
    max_width: preset.max_width,
    max_height: preset.max_height,
  })
}

async function run(action: () => Promise<unknown>, label: string, onSuccess?: () => void) {
  error.value = ''
  busy.value = true
  try {
    await invoke(action, label)
    onSuccess?.()
    emit('refresh')
  } catch (e) {
    error.value = (e as AppError).message
  } finally {
    busy.value = false
  }
}

function saveCurrent() {
  if (busy.value) return
  const name = newName.value.trim()
  if (!name) {
    error.value = '请输入预设名称'
    return
  }
  const preset = new model.ImagePreset({ name, ...props.rules })
  void run(() => SavePreset(preset), 'SavePreset', () => {
    newName.value = ''
  })
}

function startRename(preset: Preset) {
  error.value = ''
  renamingId.value = preset.id
  renameValue.value = preset.name
}

function cancelRename() {
  renamingId.value = null
  renameValue.value = ''
}

function confirmRename(preset: Preset) {
  if (busy.value) return
  const name = renameValue.value.trim()
  if (!name) {
    error.value = '请输入预设名称'
    return
  }
  const payload = presetPayload(preset, name)
  void run(() => SavePreset(payload), 'SavePreset', () => {
    renamingId.value = null
    renameValue.value = ''
  })
}

async function removePreset(preset: Preset) {
  if (busy.value) return
  error.value = ''
  busy.value = true
  try {
    const confirmed = await invoke(
      () => Confirm('删除预设', `确定删除「${preset.name}」？此操作不可撤销。`),
      'Confirm',
    )
    if (!confirmed) return
    await invoke(() => DeletePreset(preset.id), 'DeletePreset')
    emit('refresh')
  } catch (e) {
    error.value = (e as AppError).message
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.preset-dialog {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-6);
  background: color-mix(in oklab, var(--bg-base) 72%, transparent);
}

.preset-dialog__panel {
  width: min(720px, 100%);
  height: min(620px, 100%);
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

.preset-dialog__head {
  flex: none;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
}

.preset-dialog__title {
  margin: 0;
  font-size: var(--font-size-lg);
}

.preset-dialog__subtitle {
  margin: var(--space-1) 0 0;
  color: var(--text-3);
  font-size: var(--font-size-xs);
}

.preset-dialog__close {
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

.preset-dialog__close:hover {
  background: var(--bg-hover);
  color: var(--text-1);
}

.preset-dialog__close:focus-visible,
.preset-dialog__button:focus-visible,
.preset-dialog__input:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

.preset-dialog__save {
  flex: none;
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-hover);
}

.preset-dialog__label {
  flex: none;
  color: var(--text-2);
  font-size: var(--font-size-sm);
}

.preset-dialog__input {
  flex: 1;
  min-width: 0;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-raised);
  color: var(--text-1);
  padding: var(--space-2) var(--space-3);
  font-family: inherit;
  font-size: var(--font-size-sm);
}

.preset-dialog__input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.preset-dialog__button {
  flex: none;
  border: 1px solid var(--border-subtle);
  background: var(--bg-raised);
  color: var(--text-2);
  padding: var(--space-1) var(--space-3);
  border-radius: var(--radius-md);
  font-family: inherit;
  font-size: var(--font-size-sm);
  cursor: pointer;
  transition: border-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard);
}

.preset-dialog__button:hover:not(:disabled) {
  border-color: var(--color-primary-border);
  background: var(--bg-hover);
  color: var(--text-1);
}

.preset-dialog__button--primary {
  border-color: var(--color-primary);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.preset-dialog__button--danger:hover:not(:disabled) {
  border-color: var(--color-danger);
  color: var(--color-danger);
}

.preset-dialog__button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.preset-dialog__error {
  flex: none;
  margin: 0;
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

.preset-dialog__body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
}

.preset-list {
  margin: 0;
  padding: 0;
  list-style: none;
}

.preset-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-bottom: 1px solid var(--border-subtle);
}

.preset-row:last-child {
  border-bottom: none;
}

.preset-row__input {
  flex: 1;
}

.preset-row__info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.preset-row__name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-1);
  font-size: var(--font-size-md);
}

.preset-row__rules {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-3);
  font-size: var(--font-size-xs);
}

.preset-dialog__empty {
  margin: 0;
  padding: var(--space-6);
  color: var(--text-3);
  font-size: var(--font-size-sm);
  text-align: center;
}
</style>
