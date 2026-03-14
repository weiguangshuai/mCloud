<template>
  <el-dialog
    v-model="dialogVisible"
    :title="file?.original_name"
    fullscreen
    append-to-body
    class="preview-dialog"
    :show-close="true"
    :close-on-click-modal="true"
  >
    <div class="preview-container" @click="dialogVisible = false">
      <div v-if="loading" class="preview-loading">
        <el-icon class="is-loading" :size="32"><Loading /></el-icon>
        <span>加载中...</span>
      </div>
      <img
        v-else-if="previewSrc"
        :src="previewSrc"
        class="preview-image"
        @click.stop
      />
      <div v-else class="preview-error">
        <el-icon :size="48"><PictureFilled /></el-icon>
        <span>加载失败</span>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Loading, PictureFilled } from '@element-plus/icons-vue'
import { fetchPreviewBlob } from '../api/file'

const props = defineProps({
  visible: { type: Boolean, default: false },
  file: { type: Object, default: null },
})
const emit = defineEmits(['update:visible'])

const previewSrc = ref('')
const loading = ref(false)

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

function revokePreviewObjectURL() {
  if (previewSrc.value) {
    URL.revokeObjectURL(previewSrc.value)
    previewSrc.value = ''
  }
}

async function loadPreview() {
  revokePreviewObjectURL()
  if (!props.file) return

  loading.value = true
  try {
    const blob = await fetchPreviewBlob(props.file.id)
    previewSrc.value = URL.createObjectURL(blob)
  } catch (e) {
    previewSrc.value = ''
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.visible, props.file?.id],
  ([visible]) => {
    if (visible) {
      loadPreview()
    } else {
      revokePreviewObjectURL()
    }
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  revokePreviewObjectURL()
})
</script>

<style scoped>
.preview-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: calc(100vh - 80px);
  cursor: pointer;
  background-color: var(--bg-primary);
}

.preview-loading,
.preview-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--text-muted);
  font-size: 14px;
}

.preview-loading .el-icon {
  animation: rotate 1s linear infinite;
}

@keyframes rotate {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.preview-image {
  max-width: 100%;
  max-height: calc(100vh - 120px);
  object-fit: contain;
  cursor: default;
  border-radius: var(--radius-sm);
  box-shadow: var(--shadow-md);
}
</style>

<style>
/* 全局样式，用于覆盖 dialog */
.preview-dialog {
  background-color: rgba(0, 0, 0, 0.9) !important;
}

.preview-dialog .el-dialog__header {
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
  padding: 12px 16px;
}

.preview-dialog .el-dialog__title {
  font-family: var(--font-title);
  font-size: 14px;
  font-weight: 500;
  letter-spacing: 1px;
  color: var(--text-primary);
}

.preview-dialog .el-dialog__headerbtn {
  top: 12px;
}

.preview-dialog .el-dialog__headerbtn .el-dialog__close {
  color: var(--text-secondary);
}

.preview-dialog .el-dialog__headerbtn:hover .el-dialog__close {
  color: var(--accent-primary);
}

.preview-dialog .el-dialog__body {
  padding: 0;
  background-color: transparent;
}
</style>
