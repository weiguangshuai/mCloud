<template>
  <el-dialog v-model="dialogVisible" :title="file?.original_name" fullscreen append-to-body>
    <div class="preview-container" @click="dialogVisible = false">
      <img v-if="file" :src="previewSrc" class="preview-image" @click.stop />
    </div>
  </el-dialog>
</template>

<script setup>
import { computed } from 'vue'
import { getPreviewUrl, getAuthHeaders } from '../api/file'

const props = defineProps({
  visible: { type: Boolean, default: false },
  file: { type: Object, default: null },
})
const emit = defineEmits(['update:visible'])

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const previewSrc = computed(() => {
  if (!props.file) return ''
  const headers = getAuthHeaders()
  const token = headers.Authorization.replace('Bearer ', '')
  return `${getPreviewUrl(props.file.id)}?token=${encodeURIComponent(token)}`
})
</script>

<style scoped>
.preview-container {
  display: flex; justify-content: center; align-items: center;
  min-height: 60vh; cursor: pointer;
}
.preview-image { max-width: 100%; max-height: 80vh; object-fit: contain; cursor: default; }
</style>
