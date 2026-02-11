<template>
  <div
    ref="dropZone"
    class="upload-wrapper"
    @dragover.prevent="onDragOver"
    @dragleave.prevent="onDragLeave"
    @drop.prevent="onDrop"
    :class="{ 'drag-over': isDragOver }"
  >
    <input ref="fileInput" type="file" multiple hidden @change="handleFileSelect" />

    <!-- 拖拽提示 -->
    <div v-if="isDragOver" class="drag-overlay">释放文件以上传</div>

    <!-- 上传进度对话框 -->
    <el-dialog v-model="dialogVisible" title="上传文件" :close-on-click-modal="false" width="500px">
      <div v-for="(task, idx) in uploadTasks" :key="idx" class="upload-item">
        <div class="upload-name">{{ task.name }}</div>
        <div class="upload-status-text">{{ task.statusText }}</div>
        <el-progress
          :percentage="task.progress"
          :status="task.status === 'error' ? 'exception' : task.status === 'done' ? 'success' : ''"
        />
      </div>
      <template #footer>
        <el-button @click="dialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import SparkMD5 from 'spark-md5'
import {
  uploadFile,
  initChunkedUpload,
  uploadChunk,
  completeUpload,
  getUploadStatus,
} from '../api/file'

const CHUNK_SIZE = 5 * 1024 * 1024 // 5MB
const CHUNK_THRESHOLD = 5 * 1024 * 1024 // 大于5MB使用分片上传

const props = defineProps({ folderId: { type: Number, default: 0 } })
const emit = defineEmits(['uploaded'])
const fileInput = ref(null)
const dropZone = ref(null)
const dialogVisible = ref(false)
const uploadTasks = ref([])
const isDragOver = ref(false)

function triggerUpload() {
  fileInput.value?.click()
}

// 拖拽事件
function onDragOver() {
  isDragOver.value = true
}
function onDragLeave() {
  isDragOver.value = false
}
function onDrop(e) {
  isDragOver.value = false
  const files = Array.from(e.dataTransfer.files)
  if (files.length) processFiles(files)
}

function handleFileSelect(e) {
  const files = Array.from(e.target.files)
  if (files.length) processFiles(files)
  fileInput.value.value = ''
}

async function processFiles(files) {
  dialogVisible.value = true
  const startIdx = uploadTasks.value.length
  uploadTasks.value.push(
    ...files.map((f) => ({ name: f.name, progress: 0, status: '', statusText: '等待中...' }))
  )

  for (let i = 0; i < files.length; i++) {
    const taskIdx = startIdx + i
    const file = files[i]

    try {
      if (file.size > CHUNK_THRESHOLD) {
        await chunkedUpload(file, taskIdx)
      } else {
        await simpleUpload(file, taskIdx)
      }
      uploadTasks.value[taskIdx].status = 'done'
      uploadTasks.value[taskIdx].progress = 100
      uploadTasks.value[taskIdx].statusText = '上传完成'
    } catch (err) {
      uploadTasks.value[taskIdx].status = 'error'
      const msg = err?.response?.data?.message || '上传失败'
      uploadTasks.value[taskIdx].statusText = msg
      ElMessage.error(`${file.name}: ${msg}`)
    }
  }

  emit('uploaded')
}

// 小文件简单上传
async function simpleUpload(file, taskIdx) {
  uploadTasks.value[taskIdx].statusText = '上传中...'
  const formData = new FormData()
  formData.append('file', file)
  formData.append('folder_id', props.folderId)
  await uploadFile(formData, (e) => {
    if (e.total) {
      uploadTasks.value[taskIdx].progress = Math.round((e.loaded / e.total) * 100)
    }
  })
}

// 大文件分片上传
async function chunkedUpload(file, taskIdx) {
  // 1. 计算 MD5
  uploadTasks.value[taskIdx].statusText = '计算文件指纹...'
  const md5 = await calculateMD5(file, (progress) => {
    uploadTasks.value[taskIdx].progress = Math.round(progress * 20) // MD5 占 0-20%
  })

  // 2. 初始化上传（含秒传检查）
  uploadTasks.value[taskIdx].statusText = '检查秒传...'
  const initRes = await initChunkedUpload({
    file_name: file.name,
    file_size: file.size,
    file_md5: md5,
    folder_id: props.folderId,
  })

  // 秒传成功
  if (initRes.data?.status === 'instant_upload') {
    uploadTasks.value[taskIdx].statusText = '秒传完成'
    return
  }

  const uploadId = initRes.data.upload_id
  const totalChunks = initRes.data.total_chunks
  const chunkSize = initRes.data.chunk_size || CHUNK_SIZE

  // 保存到 localStorage 以支持断点续传
  saveUploadTask(uploadId, file.name, file.size, md5, totalChunks)

  // 3. 查询已上传的分片（断点续传）
  let uploadedSet = new Set()
  try {
    const statusRes = await getUploadStatus(uploadId)
    if (statusRes.data?.uploaded_chunks) {
      statusRes.data.uploaded_chunks.forEach((idx) => uploadedSet.add(idx))
    }
  } catch (_) {}

  // 4. 逐片上传
  uploadTasks.value[taskIdx].statusText = '分片上传中...'
  for (let i = 0; i < totalChunks; i++) {
    if (uploadedSet.has(i)) continue

    const start = i * chunkSize
    const end = Math.min(start + chunkSize, file.size)
    const blob = file.slice(start, end)

    const formData = new FormData()
    formData.append('upload_id', uploadId)
    formData.append('chunk_index', i)
    formData.append('chunk', blob)

    await uploadChunk(formData)

    // 进度：20-90% 分给分片上传
    const chunkProgress = ((i + 1) / totalChunks) * 70
    uploadTasks.value[taskIdx].progress = Math.round(20 + chunkProgress)
  }

  // 5. 完成合并
  uploadTasks.value[taskIdx].statusText = '合并文件...'
  uploadTasks.value[taskIdx].progress = 90
  await completeUpload({ upload_id: uploadId })
  uploadTasks.value[taskIdx].progress = 100

  // 清除 localStorage
  removeUploadTask(uploadId)
}

// 使用 SparkMD5 计算文件 MD5
function calculateMD5(file, onProgress) {
  return new Promise((resolve, reject) => {
    const spark = new SparkMD5.ArrayBuffer()
    const reader = new FileReader()
    const totalChunks = Math.ceil(file.size / (2 * 1024 * 1024)) // 2MB 分块读取
    let currentChunk = 0

    reader.onload = (e) => {
      spark.append(e.target.result)
      currentChunk++
      if (onProgress) onProgress(currentChunk / totalChunks)
      if (currentChunk < totalChunks) {
        readNext()
      } else {
        resolve(spark.end())
      }
    }
    reader.onerror = () => reject(new Error('MD5计算失败'))

    function readNext() {
      const start = currentChunk * 2 * 1024 * 1024
      const end = Math.min(start + 2 * 1024 * 1024, file.size)
      reader.readAsArrayBuffer(file.slice(start, end))
    }
    readNext()
  })
}

// localStorage 断点续传支持
function saveUploadTask(uploadId, fileName, fileSize, md5, totalChunks) {
  const tasks = JSON.parse(localStorage.getItem('mcloud_upload_tasks') || '{}')
  tasks[uploadId] = { fileName, fileSize, md5, totalChunks, savedAt: Date.now() }
  localStorage.setItem('mcloud_upload_tasks', JSON.stringify(tasks))
}

function removeUploadTask(uploadId) {
  const tasks = JSON.parse(localStorage.getItem('mcloud_upload_tasks') || '{}')
  delete tasks[uploadId]
  localStorage.setItem('mcloud_upload_tasks', JSON.stringify(tasks))
}

// 页面加载时检查未完成的上传任务
onMounted(() => {
  const tasks = JSON.parse(localStorage.getItem('mcloud_upload_tasks') || '{}')
  const now = Date.now()
  let cleaned = false
  for (const [id, task] of Object.entries(tasks)) {
    // 超过24小时的任务自动清除
    if (now - task.savedAt > 24 * 60 * 60 * 1000) {
      delete tasks[id]
      cleaned = true
    }
  }
  if (cleaned) {
    localStorage.setItem('mcloud_upload_tasks', JSON.stringify(tasks))
  }
})

defineExpose({ triggerUpload })
</script>

<style scoped>
.upload-wrapper {
  position: relative;
}
.drag-over {
  outline: 2px dashed #409eff;
  outline-offset: -2px;
  border-radius: 4px;
}
.drag-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(64, 158, 255, 0.08);
  color: #409eff;
  font-size: 16px;
  font-weight: 500;
  z-index: 10;
  pointer-events: none;
  border-radius: 4px;
}
.upload-item {
  margin-bottom: 12px;
}
.upload-name {
  font-size: 13px;
  margin-bottom: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.upload-status-text {
  font-size: 11px;
  color: #999;
  margin-bottom: 4px;
}
</style>
