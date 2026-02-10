<template>
  <div class="file-list">
    <div v-if="loading" class="loading"><el-icon class="is-loading"><Loading /></el-icon> 加载中...</div>

    <!-- 文件夹列表 -->
    <div v-for="folder in folders" :key="'f-' + folder.id" class="file-item folder"
      @click="enterFolder(folder)" @contextmenu.prevent="showFolderMenu($event, folder)">
      <el-icon :size="48" color="#f0c040"><Folder /></el-icon>
      <span class="file-name">{{ folder.name }}</span>
    </div>

    <!-- 文件列表 -->
    <div v-for="file in files" :key="'file-' + file.id" class="file-item"
      @click="handleFileClick(file)" @contextmenu.prevent="showFileMenu($event, file)">
      <img v-if="file.is_image" :src="getThumbnailSrc(file.id)" class="thumbnail" loading="lazy" />
      <el-icon v-else :size="48" color="#909399"><Document /></el-icon>
      <span class="file-name">{{ file.original_name }}</span>
      <span class="file-size">{{ formatSize(file.file_size) }}</span>
    </div>

    <div v-if="!loading && folders.length === 0 && files.length === 0" class="empty">
      <el-empty description="暂无文件" />
    </div>

    <!-- 分页 -->
    <el-pagination v-if="pagination.total > pagination.page_size"
      :current-page="pagination.page" :page-size="pagination.page_size" :total="pagination.total"
      layout="prev, pager, next" @current-change="changePage" class="pagination" />

    <!-- 右键菜单 -->
    <div v-if="contextMenu.visible" class="context-menu" :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }">
      <div class="menu-item" @click="handleDownload">下载</div>
      <div class="menu-item" @click="handleDelete">删除</div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { Folder, Document, Loading } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listFiles, deleteFile, getThumbnailUrl, getDownloadUrl, getAuthHeaders } from '../api/file'
import { listFolders, deleteFolder } from '../api/folder'
import { useUserStore } from '../store'

const props = defineProps({ folderId: { type: Number, default: 0 } })
const emit = defineEmits(['preview'])
const userStore = useUserStore()

const loading = ref(false)
const folders = ref([])
const files = ref([])
const pagination = ref({ page: 1, page_size: 20, total: 0 })
const contextMenu = ref({ visible: false, x: 0, y: 0, target: null, type: '' })

function getThumbnailSrc(fileId) {
  const headers = getAuthHeaders()
  return `${getThumbnailUrl(fileId)}?token=${encodeURIComponent(headers.Authorization.replace('Bearer ', ''))}`
}

async function loadFiles() {
  loading.value = true
  try {
    const [folderRes, fileRes] = await Promise.all([
      listFolders(props.folderId),
      listFiles({ folder_id: props.folderId, page: pagination.value.page, page_size: pagination.value.page_size }),
    ])
    folders.value = folderRes.data || []
    files.value = fileRes.data?.files || []
    if (fileRes.data?.pagination) {
      pagination.value = fileRes.data.pagination
    }
  } catch (e) {}
  loading.value = false
}

function enterFolder(folder) {
  const breadcrumbs = [...userStore.breadcrumbs, { id: folder.id, name: folder.name }]
  userStore.setCurrentFolder(folder.id, breadcrumbs)
}

function handleFileClick(file) {
  if (file.is_image) {
    emit('preview', file)
  }
}

function changePage(page) {
  pagination.value.page = page
  loadFiles()
}

function showFileMenu(e, file) {
  contextMenu.value = { visible: true, x: e.clientX, y: e.clientY, target: file, type: 'file' }
}

function showFolderMenu(e, folder) {
  contextMenu.value = { visible: true, x: e.clientX, y: e.clientY, target: folder, type: 'folder' }
}

function handleDownload() {
  if (contextMenu.value.type === 'file') {
    const url = getDownloadUrl(contextMenu.value.target.id)
    const headers = getAuthHeaders()
    const a = document.createElement('a')
    a.href = `${url}?token=${encodeURIComponent(headers.Authorization.replace('Bearer ', ''))}`
    a.download = contextMenu.value.target.original_name
    a.click()
  }
  contextMenu.value.visible = false
}

async function handleDelete() {
  const target = contextMenu.value.target
  const type = contextMenu.value.type
  contextMenu.value.visible = false

  try {
    await ElMessageBox.confirm(`确定删除 ${type === 'file' ? target.original_name : target.name}？`, '确认删除')
    if (type === 'file') {
      await deleteFile(target.id)
    } else {
      await deleteFolder(target.id)
    }
    ElMessage.success('已删除')
    loadFiles()
  } catch (e) {}
}

function hideContextMenu() { contextMenu.value.visible = false }

function formatSize(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}

watch(() => props.folderId, () => {
  pagination.value.page = 1
  loadFiles()
}, { immediate: true })

onMounted(() => document.addEventListener('click', hideContextMenu))
onUnmounted(() => document.removeEventListener('click', hideContextMenu))

defineExpose({ loadFiles })
</script>

<style scoped>
.file-list { display: flex; flex-wrap: wrap; gap: 12px; position: relative; }
.file-item {
  width: 120px; display: flex; flex-direction: column; align-items: center;
  padding: 12px 8px; border-radius: 8px; cursor: pointer; text-align: center;
}
.file-item:hover { background: #f5f7fa; }
.thumbnail { width: 80px; height: 80px; object-fit: cover; border-radius: 4px; }
.file-name { font-size: 12px; margin-top: 4px; word-break: break-all; max-height: 32px; overflow: hidden; }
.file-size { font-size: 11px; color: #999; }
.loading { width: 100%; text-align: center; padding: 40px; color: #999; }
.empty { width: 100%; }
.pagination { width: 100%; display: flex; justify-content: center; margin-top: 16px; }
.context-menu {
  position: fixed; background: #fff; border: 1px solid #e4e7ed; border-radius: 4px;
  box-shadow: 0 2px 12px rgba(0,0,0,0.1); z-index: 1000; padding: 4px 0;
}
.menu-item { padding: 8px 16px; cursor: pointer; font-size: 14px; }
.menu-item:hover { background: #ecf5ff; color: #409eff; }

@media (max-width: 768px) {
  .file-item { width: 90px; }
  .thumbnail { width: 60px; height: 60px; }
}
</style>
