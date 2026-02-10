<template>
  <div>
    <input ref="fileInput" type="file" multiple hidden @change="handleFileSelect" />
    <!-- 上传进度对话框 -->
    <el-dialog v-model="dialogVisible" title="上传文件" :close-on-click-modal="false" width="500px">
      <div v-for="(task, idx) in uploadTasks" :key="idx" class="upload-item">
        <div class="upload-name">{{ task.name }}</div>
        <el-progress :percentage="task.progress" :status="task.status === 'error' ? 'exception' : task.status === 'done' ? 'success' : ''" />
      </div>
      <template #footer>
        <el-button @click="dialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { uploadFile } from '../api/file'

const props = defineProps({ folderId: { type: Number, default: 0 } })
const emit = defineEmits(['uploaded'])
const fileInput = ref(null)
const dialogVisible = ref(false)
const uploadTasks = ref([])

function triggerUpload() {
  fileInput.value?.click()
}

async function handleFileSelect(e) {
  const files = Array.from(e.target.files)
  if (!files.length) return

  dialogVisible.value = true
  uploadTasks.value = files.map(f => ({ name: f.name, progress: 0, status: '' }))

  for (let i = 0; i < files.length; i++) {
    const file = files[i]
    const formData = new FormData()
    formData.append('file', file)
    formData.append('folder_id', props.folderId)

    try {
      await uploadFile(formData, (e) => {
        if (e.total) {
          uploadTasks.value[i].progress = Math.round((e.loaded / e.total) * 100)
        }
      })
      uploadTasks.value[i].status = 'done'
      uploadTasks.value[i].progress = 100
    } catch (err) {
      uploadTasks.value[i].status = 'error'
      ElMessage.error(`${file.name} 上传失败`)
    }
  }

  emit('uploaded')
  fileInput.value.value = ''
}

defineExpose({ triggerUpload })
</script>

<style scoped>
.upload-item { margin-bottom: 12px; }
.upload-name { font-size: 13px; margin-bottom: 4px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
