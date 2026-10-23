import { nextTick, onMounted, onUnmounted, watch, type Ref } from 'vue'

// 对话框共用行为：Esc 关闭、打开时把应用背景设为 inert、焦点进入对话框并在关闭后归还。
export function useModalFocus(
  open: Ref<boolean>,
  panel: Ref<HTMLElement | null>,
  close: () => void,
): void {
  let lastFocused: HTMLElement | null = null

  function onKeydown(event: KeyboardEvent) {
    if (!open.value || event.key !== 'Escape') return
    event.preventDefault()
    close()
  }

  watch(open, (next) => {
    const root = document.getElementById('app')
    if (next) {
      lastFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null
      root?.setAttribute('inert', '')
      void nextTick(() => panel.value?.focus())
      return
    }
    root?.removeAttribute('inert')
    lastFocused?.focus()
    lastFocused = null
  })

  onMounted(() => window.addEventListener('keydown', onKeydown))
  onUnmounted(() => {
    window.removeEventListener('keydown', onKeydown)
    document.getElementById('app')?.removeAttribute('inert')
  })
}
