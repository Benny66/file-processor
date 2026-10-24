<template>
  <Teleport to="body">
    <div v-if="open" class="dl__overlay" @click.self="emit('close')">
      <div ref="panel" class="dl" role="dialog" aria-label="保存图片">
        <header class="dl__head">
          <h2 class="dl__title">保存图片</h2>
          <button class="dl__close" type="button" aria-label="关闭保存" @click="emit('close')">×</button>
        </header>

        <div class="dl__body">
          <div class="dl__row">
            <span class="dl__label">保存到</span>
            <button
              class="dl__dir"
              type="button"
              :title="directory || '选择保存的文件夹'"
              @click="chooseDirectory"
            >
              {{ directory || '选择文件夹…' }}
            </button>
            <button class="dl__change" type="button" @click="chooseDirectory">更改位置</button>
          </div>
          <label class="dl__row dl__row--check">
            <input v-model="keepStructure" type="checkbox" class="dl__checkbox" />
            <span>保留原来的文件夹层级</span>
          </label>
          <div class="dl__row">
            <span class="dl__label">文件名</span>
            <input
              v-model="pattern"
              class="dl__input"
              type="text"
              placeholder="{name}-out"
              maxlength="64"
            />
          </div>
          <p class="dl__example" :title="example">{{ example }}</p>
          <p v-if="error" class="dl__error" role="alert">{{ error }}</p>
        </div>

        <footer class="dl__foot">
          <span class="dl__count">共 {{ count }} 张</span>
          <span class="dl__spacer" />
          <button class="dl__btn dl__btn--primary" type="button" :disabled="busy || !directory" @click="confirmDownload">
            {{ busy ? '正在保存…' : '保存' }}
          </button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, toRef, watch } from 'vue'
import {
  DownloadItems,
  GetDownloadOptions,
  RenderOutputSample,
  SelectDirectory,
} from '../../../wailsjs/go/main/App'
import { useModalFocus } from '../../composables/useModalFocus'
import { invoke, type AppError } from '../../lib/invoke'
import { logInfo, logWarn } from '../../lib/log'

// 下载选项框（D-58：单张与全部下载共用）：目录（记忆值）+ 保留层级 + 文件名格式。
// 命名示例走 Go 的 RenderOutputSample——与真实保存同一套渲染逻辑。
const props = defineProps<{
  open: boolean
  /** 下载分组：batchKey → 待保存的源路径（重试合并后可能跨批次）。 */
  groups: { batchKey: string; paths: string[] }[]
  /** 命名示例用的样例文件名。 */
  sampleName: string
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'downloaded', payload: { okCount: number; failCount: number; directory: string }): void
}>()

const panel = ref<HTMLElement | null>(null)
useModalFocus(toRef(props, 'open'), panel, () => emit('close'))

const directory = ref('')
const keepStructure = ref(false)
const pattern = ref('')
const busy = ref(false)
const error = ref('')
const example = ref('')

const count = computed(() => props.groups.reduce((sum, group) => sum + group.paths.length, 0))

let exampleToken = 0
let exampleTimer: ReturnType<typeof setTimeout> | null = null
async function refreshExample() {
  const token = ++exampleToken
  try {
    const rendered = await invoke(
      () => RenderOutputSample(pattern.value, props.sampleName, 'keep'),
      'RenderOutputSample',
    )
    if (token === exampleToken) example.value = `${props.sampleName} → ${rendered}（后缀按实际格式）`
  } catch {
    if (token === exampleToken) example.value = '文件名格式不对（要包含 {name} 或 {n}）'
  }
}
watch([pattern, () => props.sampleName], () => {
  if (!props.open) return
  if (exampleTimer) clearTimeout(exampleTimer)
  exampleTimer = setTimeout(() => {
    exampleTimer = null
    void refreshExample()
  }, 200)
})

// 打开时载入记忆的选项（目录为空时 Go 已回退到系统「下载」文件夹）。
watch(
  () => props.open,
  async (open) => {
    if (!open) return
    error.value = ''
    try {
      const opts = await invoke(() => GetDownloadOptions(), 'GetDownloadOptions')
      directory.value = opts.directory ?? ''
      keepStructure.value = !!opts.keep_structure
      pattern.value = opts.pattern ?? ''
    } catch (e) {
      logWarn('读取下载选项失败', e)
    }
    void refreshExample()
  },
)

async function chooseDirectory() {
  try {
    const dir = await invoke(() => SelectDirectory('选择保存的文件夹'), 'SelectDirectory')
    if (dir) directory.value = dir
  } catch (e) {
    error.value = (e as AppError).message
  }
}

async function confirmDownload() {
  if (busy.value || !directory.value || props.groups.length === 0) return
  busy.value = true
  error.value = ''
  let okCount = 0
  let failCount = 0
  try {
    for (const group of props.groups) {
      const results = await invoke(
        () =>
          DownloadItems(group.batchKey, group.paths, {
            directory: directory.value,
            keep_structure: keepStructure.value,
            pattern: pattern.value,
          }),
        'DownloadItems',
      )
      for (const result of results) {
        if (result.ok) okCount++
        else failCount++
      }
    }
    logInfo('下载完成', { ok: okCount, fail: failCount, dir: directory.value })
    emit('downloaded', { okCount, failCount, directory: directory.value })
  } catch (e) {
    error.value = (e as AppError).message
    logWarn('下载失败', e)
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.dl__overlay {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-6);
  background: color-mix(in oklab, var(--bg-base) 72%, transparent);
}

.dl {
  display: flex;
  flex-direction: column;
  width: min(520px, 100%);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-raised);
  box-shadow: var(--shadow-lg);
  overflow: hidden;
}

.dl__head {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-3) var(--space-4);
  border-bottom: 1px solid var(--border-subtle);
}

.dl__title {
  margin: 0;
  font-size: var(--font-size-md);
  color: var(--text-1);
}

.dl__close {
  border: none;
  background: transparent;
  color: var(--text-3);
  font-size: var(--font-size-lg);
  line-height: 1;
  cursor: pointer;
  padding: var(--space-1);
  border-radius: var(--radius-sm);
}

.dl__close:hover {
  color: var(--text-1);
}

.dl__body {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-4);
}

.dl__row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.dl__row--check {
  cursor: pointer;
  user-select: none;
  color: var(--text-2);
  font-size: var(--font-size-sm);
}

.dl__checkbox {
  width: 14px;
  height: 14px;
  accent-color: var(--color-primary);
  cursor: pointer;
}

.dl__label {
  flex: none;
  color: var(--text-2);
  font-size: var(--font-size-sm);
}

.dl__dir {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
  border: 1px solid var(--border-subtle);
  background: var(--bg-base);
  color: var(--text-1);
  font-family: inherit;
  font-size: var(--font-size-sm);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-md);
  cursor: pointer;
}

.dl__dir:hover {
  border-color: var(--color-primary-border);
}

.dl__change {
  flex: none;
  border: none;
  background: transparent;
  color: var(--color-primary);
  font-family: inherit;
  font-size: var(--font-size-sm);
  cursor: pointer;
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
}

.dl__change:hover {
  background: var(--bg-hover);
}

.dl__input {
  flex: 1;
  min-width: 0;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--bg-base);
  color: var(--text-1);
  padding: var(--space-2) var(--space-3);
  font-family: inherit;
  font-size: var(--font-size-sm);
}

.dl__example {
  margin: 0;
  color: var(--text-3);
  font-size: var(--font-size-xs);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dl__error {
  margin: 0;
  color: var(--color-danger);
  font-size: var(--font-size-sm);
}

.dl__foot {
  flex: none;
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  border-top: 1px solid var(--border-subtle);
}

.dl__count {
  color: var(--text-3);
  font-size: var(--font-size-sm);
}

.dl__spacer {
  flex: 1;
}

.dl__btn {
  border: 1px solid var(--border-subtle);
  background: var(--bg-raised);
  color: var(--text-2);
  padding: var(--space-2) var(--space-4);
  border-radius: var(--radius-md);
  font-family: inherit;
  font-size: var(--font-size-sm);
  cursor: pointer;
}

.dl__btn--primary {
  border-color: var(--color-primary);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.dl__btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
