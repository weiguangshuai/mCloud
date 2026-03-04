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
    <input ref="resumeFileInput" type="file" hidden @change="handleResumeFileSelect" />

    <div v-if="isDragOver" class="drag-overlay">释放文件以上传</div>

    <section class="transfer-panel" :class="{ collapsed: panelCollapsed }">
      <header class="transfer-header">
        <div class="transfer-title">传输列表</div>
        <div class="transfer-actions">
          <el-button text size="small" :loading="loadingTasks" @click="refreshTasks">刷新</el-button>
          <el-button text size="small" @click="panelCollapsed = !panelCollapsed">
            {{ panelCollapsed ? '展开' : '收起' }}
          </el-button>
        </div>
      </header>

      <div v-show="!panelCollapsed" class="transfer-body">
        <div v-if="displayTasks.length === 0" class="transfer-empty">暂无传输任务</div>

        <div v-for="task in displayTasks" :key="task.key" class="transfer-item">
          <div class="transfer-name" :title="task.fileName">{{ task.fileName }}</div>
          <div class="transfer-meta">{{ task.statusText }}</div>
          <el-progress
            :percentage="task.progress"
            :status="task.status === 'failed' || task.status === 'error' ? 'exception' : task.status === 'completed' ? 'success' : ''"
          />
          <div class="transfer-item-actions">
            <el-button
              v-if="canContinue(task)"
              text
              size="small"
              type="primary"
              :loading="task.busy"
              @click="continueTask(task)"
            >
              继续
            </el-button>
            <el-button
              v-if="canCancel(task)"
              text
              size="small"
              :loading="task.busy"
              @click="cancelTaskItem(task)"
            >
              取消
            </el-button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import SparkMD5 from 'spark-md5'
import {
  uploadFile,
  initChunkedUpload,
  queryUploadTask,
  uploadChunk,
  completeUpload,
  listUploadTasks,
  getUploadTaskDetail,
  cancelUploadTask,
} from '../api/file'

const CHUNK_SIZE = 5 * 1024 * 1024
const CHUNK_THRESHOLD = 5 * 1024 * 1024
const MAX_CHUNK_RETRIES = 3
const CHUNK_RETRY_DELAY_MS = 1000

const HANDLE_DB_NAME = 'mcloud_upload_handles'
const HANDLE_STORE_NAME = 'handles'

const props = defineProps({ folderId: { type: Number, default: 0 } })
const emit = defineEmits(['uploaded'])

const fileInput = ref(null)
const resumeFileInput = ref(null)
const dropZone = ref(null)
const isDragOver = ref(false)
const loadingTasks = ref(false)
const panelCollapsed = ref(false)
const tasks = ref([])

const runtimeFiles = new Map()
const pendingResume = {
  uploadId: '',
  resolve: null,
}

const displayTasks = computed(() => tasks.value)

function supportsNativePicker() {
  return typeof window !== 'undefined' && typeof window.showOpenFilePicker === 'function'
}

function supportsHandleStorage() {
  return typeof window !== 'undefined' && supportsNativePicker() && typeof indexedDB !== 'undefined'
}

function nowISO() {
  return new Date().toISOString()
}

function buildStatusText(status, uploaded, total) {
  if (status === 'completed') return '上传完成'
  if (status === 'failed') return '上传失败'
  if (status === 'paused') return '已暂停'
  if (status === 'pending') return '等待上传'
  if (total > 0) return `分片上传中 (${uploaded}/${total})`
  return '上传中'
}

function addLocalTask(fileName) {
  const key = `local-${Date.now()}-${Math.random().toString(16).slice(2)}`
  const task = {
    key,
    uploadId: '',
    fileName,
    fileSize: 0,
    totalChunks: 0,
    uploadedChunksCount: 0,
    uploadedSize: 0,
    fileMD5: '',
    status: 'uploading',
    statusText: '等待中...',
    progress: 0,
    busy: false,
    updatedAt: nowISO(),
  }
  tasks.value.unshift(task)
  return task
}

function updateTask(task, patch) {
  const idx = tasks.value.findIndex((item) => item.key === task.key)
  if (idx < 0) return
  tasks.value[idx] = {
    ...tasks.value[idx],
    ...patch,
    updatedAt: nowISO(),
  }
}

function upsertServerTask(data) {
  const key = data.upload_id
  const idx = tasks.value.findIndex((item) => item.key === key)
  const existing = idx >= 0 ? tasks.value[idx] : null
  const totalChunks = Number(data.total_chunks || 0)
  const uploadedCount = Number(data.uploaded_chunks_count || 0)
  const progress = totalChunks > 0 ? Math.min(100, Math.round((uploadedCount / totalChunks) * 100)) : 0

  const merged = {
    key,
    uploadId: data.upload_id,
    fileName: data.file_name,
    fileSize: Number(data.file_size || 0),
    totalChunks,
    uploadedChunksCount: uploadedCount,
    uploadedSize: Number(data.uploaded_size || 0),
    fileMD5: existing?.fileMD5 || '',
    status: data.status || 'uploading',
    statusText: buildStatusText(data.status, uploadedCount, totalChunks),
    progress: data.status === 'completed' ? 100 : progress,
    busy: existing?.busy || false,
    updatedAt: data.updated_at || nowISO(),
    completedAt: data.completed_at || null,
    expiresAt: data.expires_at || null,
    lastError: data.last_error || '',
  }

  if (idx >= 0) {
    tasks.value[idx] = merged
  } else {
    tasks.value.unshift(merged)
  }
  return merged
}

function removeTaskByUploadId(uploadId) {
  tasks.value = tasks.value.filter((item) => item.uploadId !== uploadId)
}

async function refreshTasks() {
  loadingTasks.value = true
  try {
    const res = await listUploadTasks()
    const serverTasks = Array.isArray(res.data) ? res.data : []
    const existingLocal = tasks.value.filter((item) => !item.uploadId)
    tasks.value = existingLocal
    serverTasks.forEach((task) => upsertServerTask(task))
  } finally {
    loadingTasks.value = false
  }
}

async function triggerUpload() {
  if (supportsNativePicker()) {
    try {
      const handles = await window.showOpenFilePicker({ multiple: true })
      if (!handles || handles.length === 0) return
      const items = []
      for (const handle of handles) {
        const file = await handle.getFile()
        items.push({ file, handle })
      }
      await processFiles(items)
      return
    } catch (error) {
      if (error?.name === 'AbortError') return
    }
  }
  fileInput.value?.click()
}

function onDragOver() {
  isDragOver.value = true
}

function onDragLeave() {
  isDragOver.value = false
}

async function onDrop(event) {
  isDragOver.value = false
  const files = Array.from(event.dataTransfer.files || [])
  if (!files.length) return
  await processFiles(files.map((file) => ({ file })))
}

async function handleFileSelect(event) {
  const files = Array.from(event.target.files || [])
  fileInput.value.value = ''
  if (!files.length) return
  await processFiles(files.map((file) => ({ file })))
}

function handleResumeFileSelect(event) {
  const file = event.target.files?.[0] || null
  resumeFileInput.value.value = ''
  if (pendingResume.resolve) {
    pendingResume.resolve(file)
    pendingResume.resolve = null
    pendingResume.uploadId = ''
  }
}

async function processFiles(items) {
  panelCollapsed.value = false
  for (const item of items) {
    const file = item.file
    if (!file) continue
    try {
      if (file.size > CHUNK_THRESHOLD) {
        await startChunkedUpload(file, item.handle || null)
      } else {
        await startSimpleUpload(file)
      }
      emit('uploaded')
    } catch (error) {
      const msg = error?.response?.data?.message || error?.message || '上传失败'
      ElMessage.error(`${file.name}: ${msg}`)
    }
  }
  await refreshTasks()
}

async function startSimpleUpload(file) {
  const task = addLocalTask(file.name)
  updateTask(task, { statusText: '上传中...' })

  const formData = new FormData()
  formData.append('file', file)
  formData.append('folder_id', props.folderId)

  await uploadFile(formData, (event) => {
    if (!event.total) return
    updateTask(task, {
      progress: Math.round((event.loaded / event.total) * 100),
    })
  })

  updateTask(task, {
    status: 'completed',
    statusText: '上传完成',
    progress: 100,
  })
}

async function startChunkedUpload(file, handle) {
  const task = addLocalTask(file.name)
  updateTask(task, { statusText: '计算文件指纹...' })

  const md5 = await calculateMD5(file, (progress) => {
    updateTask(task, { progress: Math.round(progress * 20) })
  })

  updateTask(task, { fileMD5: md5 })

  const queryRes = await queryUploadTask({
    file_name: file.name,
    file_size: file.size,
    file_md5: md5,
    folder_id: props.folderId,
  })

  let uploadId = ''
  let totalChunks = 0
  let uploadedChunks = []
  let chunkSize = CHUNK_SIZE

  if (queryRes.data?.resumable) {
    uploadId = queryRes.data.upload_id
    totalChunks = Number(queryRes.data.total_chunks || 0)
    uploadedChunks = Array.isArray(queryRes.data.uploaded_chunks) ? queryRes.data.uploaded_chunks : []
    updateTask(task, { statusText: '检测到断点，准备续传...' })
  } else {
    const initRes = await initChunkedUpload({
      file_name: file.name,
      file_size: file.size,
      file_md5: md5,
      folder_id: props.folderId,
    })

    if (initRes.data?.status === 'instant_upload') {
      updateTask(task, { status: 'completed', statusText: '秒传完成', progress: 100 })
      return
    }

    uploadId = initRes.data.upload_id
    totalChunks = Number(initRes.data.total_chunks || 0)
    chunkSize = Number(initRes.data.chunk_size || CHUNK_SIZE)
    uploadedChunks = []
  }

  const serverTask = upsertServerTask({
    upload_id: uploadId,
    file_name: file.name,
    file_size: file.size,
    total_chunks: totalChunks,
    uploaded_chunks_count: uploadedChunks.length,
    uploaded_size: 0,
    status: 'uploading',
    updated_at: nowISO(),
  })
  updateTask(serverTask, { fileMD5: md5, busy: true, statusText: '分片上传中...' })

  runtimeFiles.set(uploadId, file)
  if (handle) {
    await saveUploadHandle(uploadId, handle)
  }

  try {
    await uploadChunks(uploadId, file, totalChunks, chunkSize, new Set(uploadedChunks), serverTask)
    updateTask(serverTask, { statusText: '合并文件...', progress: 95 })
    await completeUpload({ upload_id: uploadId })
    updateTask(serverTask, {
      status: 'completed',
      statusText: '上传完成',
      progress: 100,
      uploadedChunksCount: totalChunks,
      uploadedSize: file.size,
      busy: false,
    })
    await deleteUploadHandle(uploadId)
  } catch (error) {
    updateTask(serverTask, {
      status: 'failed',
      statusText: error?.response?.data?.message || error?.message || '上传失败',
      busy: false,
    })
    throw error
  }
}

async function uploadChunks(uploadId, file, totalChunks, chunkSize, uploadedSet, task) {
  let completedChunks = uploadedSet.size
  for (let idx = 0; idx < totalChunks; idx++) {
    if (uploadedSet.has(idx)) continue

    const start = idx * chunkSize
    const end = Math.min(start + chunkSize, file.size)
    const formData = new FormData()
    formData.append('upload_id', uploadId)
    formData.append('chunk_index', idx)
    formData.append('chunk', file.slice(start, end))

    updateTask(task, {
      status: 'uploading',
      statusText: `分片上传中... (${idx + 1}/${totalChunks})`,
    })

    await uploadChunkWithRetry(formData, uploadId, idx, (event) => {
      if (!event.total || totalChunks <= 0) return
      const currentChunkProgress = event.loaded / event.total
      const overall = (completedChunks + currentChunkProgress) / totalChunks
      updateTask(task, {
        progress: Math.round(20 + overall * 70),
      })
    })

    completedChunks++
    uploadedSet.add(idx)
    const uploadedSize = Math.min(file.size, Math.round((completedChunks / totalChunks) * file.size))
    updateTask(task, {
      uploadedChunksCount: completedChunks,
      uploadedSize,
      progress: Math.round(20 + (completedChunks / totalChunks) * 70),
      statusText: `分片上传中... (${completedChunks}/${totalChunks})`,
    })
  }
}

async function continueTask(task) {
  if (!task.uploadId || task.busy || task.status === 'completed') return
  updateTask(task, { busy: true, statusText: '加载任务详情...' })

  try {
    const detailRes = await getUploadTaskDetail(task.uploadId)
    const detail = detailRes.data || {}

    if (detail.status === 'completed') {
      updateTask(task, { status: 'completed', statusText: '上传完成', progress: 100, busy: false })
      await refreshTasks()
      return
    }

    const source = await resolveResumeFile(task, detail)
    if (!source?.file) {
      updateTask(task, { busy: false, statusText: '未选择文件，已取消续传' })
      return
    }

    const file = source.file
    if (Number(file.size) !== Number(detail.file_size)) {
      updateTask(task, { busy: false, statusText: '文件大小不匹配，无法续传', status: 'failed' })
      ElMessage.error('选择的文件大小与任务不一致')
      return
    }

    const md5 = await calculateMD5(file, (progress) => {
      updateTask(task, { progress: Math.round(progress * 20), statusText: '校验文件指纹...' })
    })
    if (detail.file_md5 && md5 !== detail.file_md5) {
      updateTask(task, { busy: false, statusText: '文件指纹不匹配，无法续传', status: 'failed' })
      ElMessage.error('选择的文件与原任务不一致')
      return
    }

    runtimeFiles.set(task.uploadId, file)
    if (source.handle) {
      await saveUploadHandle(task.uploadId, source.handle)
    }

    const uploadedChunks = Array.isArray(detail.uploaded_chunks) ? detail.uploaded_chunks : []
    await uploadChunks(task.uploadId, file, Number(detail.total_chunks || 0), CHUNK_SIZE, new Set(uploadedChunks), task)

    updateTask(task, { statusText: '合并文件...', progress: 95 })
    await completeUpload({ upload_id: task.uploadId })

    updateTask(task, {
      status: 'completed',
      statusText: '上传完成',
      progress: 100,
      uploadedChunksCount: Number(detail.total_chunks || 0),
      uploadedSize: Number(detail.file_size || file.size),
      busy: false,
    })
    await deleteUploadHandle(task.uploadId)
    emit('uploaded')
    await refreshTasks()
  } catch (error) {
    updateTask(task, {
      busy: false,
      status: 'failed',
      statusText: error?.response?.data?.message || error?.message || '续传失败',
    })
    ElMessage.error(`续传失败: ${error?.response?.data?.message || error?.message || '未知错误'}`)
  }
}

async function resolveResumeFile(task, detail) {
  const runtime = runtimeFiles.get(task.uploadId)
  if (runtime) {
    return { file: runtime, handle: null }
  }

  if (supportsHandleStorage()) {
    const handle = await loadUploadHandle(task.uploadId)
    if (handle) {
      let permission = await handle.queryPermission({ mode: 'read' })
      if (permission !== 'granted') {
        permission = await handle.requestPermission({ mode: 'read' })
      }
      if (permission === 'granted') {
        const file = await handle.getFile()
        return { file, handle }
      }
    }
  }

  const selectedFile = await requestResumeFile(detail.upload_id || task.uploadId)
  if (!selectedFile) return null
  return { file: selectedFile, handle: null }
}

function requestResumeFile(uploadId) {
  return new Promise((resolve) => {
    pendingResume.uploadId = uploadId
    pendingResume.resolve = resolve
    resumeFileInput.value?.click()
  })
}

function canContinue(task) {
  return Boolean(task.uploadId) && task.status !== 'completed'
}

function canCancel(task) {
  return Boolean(task.uploadId) && task.status !== 'completed'
}

async function cancelTaskItem(task) {
  if (!task.uploadId || task.busy) return
  updateTask(task, { busy: true, statusText: '取消中...' })
  try {
    await cancelUploadTask(task.uploadId)
    runtimeFiles.delete(task.uploadId)
    await deleteUploadHandle(task.uploadId)
    removeTaskByUploadId(task.uploadId)
    ElMessage.success('上传任务已取消')
  } catch (error) {
    updateTask(task, {
      busy: false,
      statusText: error?.response?.data?.message || error?.message || '取消失败',
    })
  }
}

function isTimeoutError(error) {
  if (error?.code === 'ECONNABORTED') return true
  const message = String(error?.message || '').toLowerCase()
  return message.includes('timeout')
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function uploadChunkWithRetry(formData, uploadId, chunkIndex, onProgress) {
  let lastError = null
  const maxAttempts = MAX_CHUNK_RETRIES + 1

  for (let attempt = 0; attempt <= MAX_CHUNK_RETRIES; attempt++) {
    try {
      await uploadChunk(formData, onProgress)
      return
    } catch (error) {
      lastError = error
      const timeoutHint = isTimeoutError(error) ? ' timeout' : ''
      console.warn(
        `[upload] chunk failed${timeoutHint}: upload_id=${uploadId}, chunk=${chunkIndex}, attempt=${attempt + 1}/${maxAttempts}`,
        error
      )
      if (attempt === MAX_CHUNK_RETRIES) break
      await sleep(CHUNK_RETRY_DELAY_MS * (attempt + 1))
    }
  }

  throw lastError
}

function calculateMD5(file, onProgress) {
  return new Promise((resolve, reject) => {
    const spark = new SparkMD5.ArrayBuffer()
    const reader = new FileReader()
    const readChunkSize = 2 * 1024 * 1024
    const totalChunks = Math.ceil(file.size / readChunkSize)
    let currentChunk = 0

    reader.onload = (event) => {
      spark.append(event.target.result)
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
      const start = currentChunk * readChunkSize
      const end = Math.min(start + readChunkSize, file.size)
      reader.readAsArrayBuffer(file.slice(start, end))
    }

    readNext()
  })
}

function openHandleDB() {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(HANDLE_DB_NAME, 1)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(HANDLE_STORE_NAME)) {
        db.createObjectStore(HANDLE_STORE_NAME)
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

async function saveUploadHandle(uploadId, handle) {
  if (!supportsHandleStorage() || !uploadId || !handle) return
  const db = await openHandleDB()
  await new Promise((resolve, reject) => {
    const tx = db.transaction(HANDLE_STORE_NAME, 'readwrite')
    tx.objectStore(HANDLE_STORE_NAME).put(handle, uploadId)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
  db.close()
}

async function loadUploadHandle(uploadId) {
  if (!supportsHandleStorage() || !uploadId) return null
  const db = await openHandleDB()
  const value = await new Promise((resolve, reject) => {
    const tx = db.transaction(HANDLE_STORE_NAME, 'readonly')
    const req = tx.objectStore(HANDLE_STORE_NAME).get(uploadId)
    req.onsuccess = () => resolve(req.result || null)
    req.onerror = () => reject(req.error)
  })
  db.close()
  return value
}

async function deleteUploadHandle(uploadId) {
  if (!supportsHandleStorage() || !uploadId) return
  const db = await openHandleDB()
  await new Promise((resolve, reject) => {
    const tx = db.transaction(HANDLE_STORE_NAME, 'readwrite')
    tx.objectStore(HANDLE_STORE_NAME).delete(uploadId)
    tx.oncomplete = () => resolve()
    tx.onerror = () => reject(tx.error)
  })
  db.close()
}

onMounted(async () => {
  await refreshTasks()
  const pendingCount = tasks.value.filter((item) => item.status !== 'completed').length
  if (pendingCount > 0) {
    ElMessage.info(`检测到 ${pendingCount} 个未完成上传任务，可在传输列表中继续上传`)
  }
})

defineExpose({ triggerUpload, refreshTasks })
</script>

<style scoped>
.upload-wrapper {
  position: relative;
}

.drag-over {
  outline: 2px dashed #409eff;
  outline-offset: -2px;
  border-radius: 6px;
}

.drag-overlay {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2000;
  background: rgba(64, 158, 255, 0.12);
  color: #409eff;
  font-size: 18px;
  font-weight: 600;
}

.transfer-panel {
  position: fixed;
  right: 16px;
  bottom: 16px;
  width: min(420px, calc(100vw - 24px));
  max-height: min(70vh, 520px);
  border: 1px solid #dfe4ea;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.12);
  z-index: 1500;
  overflow: hidden;
}

.transfer-panel.collapsed {
  max-height: 52px;
}

.transfer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-bottom: 1px solid #eff3f8;
}

.transfer-title {
  font-size: 14px;
  font-weight: 600;
}

.transfer-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.transfer-body {
  max-height: calc(min(70vh, 520px) - 52px);
  overflow: auto;
  padding: 10px 12px;
}

.transfer-empty {
  color: #8b94a1;
  font-size: 13px;
  padding: 16px 0;
  text-align: center;
}

.transfer-item {
  border: 1px solid #edf1f5;
  border-radius: 10px;
  padding: 10px;
  margin-bottom: 10px;
  background: #fcfdff;
}

.transfer-item:last-child {
  margin-bottom: 0;
}

.transfer-name {
  font-size: 13px;
  font-weight: 600;
  color: #1f2d3d;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.transfer-meta {
  margin: 4px 0 8px;
  font-size: 12px;
  color: #7b8794;
}

.transfer-item-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 6px;
}

@media (max-width: 768px) {
  .transfer-panel {
    right: 8px;
    bottom: 8px;
    width: calc(100vw - 16px);
    max-height: min(60vh, 480px);
  }
}
</style>
