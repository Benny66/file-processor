<template>
  <section class="workbench">
    <header class="workbench__head">
      <h1 class="workbench__title">图片处理</h1>
      <p class="workbench__desc">批量转换格式、压缩体积、限制尺寸。图片只在本机处理。</p>
    </header>

    <div class="dropzone" role="region" aria-label="图片导入区">
      <div class="dropzone__icon" aria-hidden="true">＋</div>
      <p class="dropzone__title">将图片或文件夹拖到这里</p>
      <p class="dropzone__hint">支持 {{ supportedFormats }}</p>
    </div>
  </section>
</template>

<script setup lang="ts">
const supportedFormats = 'JPG / PNG / WebP / BMP / TIFF / GIF（静态）'
</script>

<style scoped>
.workbench {
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: var(--space-6);
}

.workbench__head {
  flex: none;
}

.workbench__title {
  font-size: var(--font-size-xl);
  margin: 0 0 var(--space-2);
}

.workbench__desc {
  margin: 0;
  color: var(--text-2);
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
</style>
