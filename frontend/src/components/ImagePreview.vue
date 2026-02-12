<template>
  <el-dialog v-model="dialogVisible" :title="file?.original_name" fullscreen append-to-body>
    <div class="preview-container" @click="dialogVisible = false">
      <img v-if="previewSrc" :src="previewSrc" class="preview-image" @click.stop />
    </div>
  </el-dialog>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { fetchPreviewBlob } from '../api/file'

const props = defineProps({
  visible: { type: Boolean, default: false },
  file: { type: Object, default: null },
})
const emit = defineEmits(['update:visible'])

const previewSrc = ref('')

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

  try {
    const blob = await fetchPreviewBlob(props.file.id)
    previewSrc.value = URL.createObjectURL(blob)
  } catch (e) {
    previewSrc.value = ''
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
  min-height: 60vh;
  cursor: pointer;
}
.preview-image {
  max-width: 100%;
  max-height: 80vh;
  object-fit: contain;
  cursor: default;
}
</style>
