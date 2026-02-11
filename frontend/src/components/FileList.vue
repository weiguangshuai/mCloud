<template>
  <div class="file-list-container">
    <!-- 工具栏 -->
    <div class="toolbar" v-if="!loading && (folders.length > 0 || files.length > 0 || selectedItems.length > 0)">
      <div class="toolbar-left">
        <el-checkbox v-model="selectAll" @change="toggleSelectAll" :indeterminate="isIndeterminate">全选</el-checkbox>
        <template v-if="selectedItems.length > 0">
          <el-button size="small" type="danger" @click="handleBatchDelete">删除 ({{ selectedItems.length }})</el-button>
          <el-button size="small" @click="showBatchMoveDialog">移动 ({{ selectedItems.length }})</el-button>
        </template>
      </div>
      <div class="toolbar-right">
        <el-button-group>
          <el-button size="small" :type="viewMode === 'grid' ? 'primary' : ''" @click="viewMode = 'grid'">
            <el-icon><Grid /></el-icon>
          </el-button>
          <el-button size="small" :type="viewMode === 'list' ? 'primary' : ''" @click="viewMode = 'list'">
            <el-icon><List /></el-icon>
          </el-button>
        </el-button-group>
      </div>
    </div>

    <div v-if="loading" class="loading"><el-icon class="is-loading"><Loading /></el-icon> 加载中...</div>

    <!-- 网格视图 -->
    <div v-if="viewMode === 'grid'" class="grid-view">
      <div v-for="folder in folders" :key="'f-' + folder.id" class="file-item folder"
        @click="enterFolder(folder)" @contextmenu.prevent="showFolderMenu($event, folder)">
        <el-icon :size="48" color="#f0c040"><Folder /></el-icon>
        <span class="file-name">{{ folder.name }}</span>
      </div>
      <div v-for="file in files" :key="'file-' + file.id" class="file-item"
        :class="{ selected: isSelected('file', file.id) }"
        @click.exact="handleFileClick(file)" @click.ctrl="toggleSelect('file', file.id)"
        @contextmenu.prevent="showFileMenu($event, file)">
        <el-checkbox class="item-checkbox" :model-value="isSelected('file', file.id)"
          @change="toggleSelect('file', file.id)" @click.stop />
        <img v-if="getFileObj(file).is_image" :src="getThumbnailSrc(file.id)" class="thumbnail" loading="lazy" />
        <el-icon v-else :size="48" color="#909399"><Document /></el-icon>
        <span class="file-name">{{ file.original_name }}</span>
        <span class="file-size">{{ formatSize(getFileObj(file).file_size) }}</span>
      </div>
    </div>

    <!-- 列表视图 -->
    <el-table v-if="viewMode === 'list' && !loading" :data="listViewData" @row-contextmenu="onRowContextMenu"
      @row-click="onRowClick" style="width: 100%" :row-class-name="rowClassName">
      <el-table-column width="40">
        <template #header>
          <el-checkbox v-model="selectAll" @change="toggleSelectAll" :indeterminate="isIndeterminate" />
        </template>
        <template #default="{ row }">
          <el-checkbox v-if="row._type === 'file'" :model-value="isSelected('file', row.id)"
            @change="toggleSelect('file', row.id)" @click.stop />
        </template>
      </el-table-column>
      <el-table-column label="名称" min-width="200">
        <template #default="{ row }">
          <div class="list-name-cell">
            <el-icon v-if="row._type === 'folder'" :size="20" color="#f0c040"><Folder /></el-icon>
            <img v-else-if="row._isImage" :src="getThumbnailSrc(row.id)" class="list-thumb" />
            <el-icon v-else :size="20" color="#909399"><Document /></el-icon>
            <span>{{ row._type === 'folder' ? row.name : row.original_name }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="大小" width="100">
        <template #default="{ row }">
          {{ row._type === 'file' ? formatSize(row._fileSize) : '-' }}
        </template>
      </el-table-column>
      <el-table-column label="修改时间" width="170">
        <template #default="{ row }">
          {{ formatDate(row.created_at || row.CreatedAt) }}
        </template>
      </el-table-column>
    </el-table>

    <div v-if="!loading && folders.length === 0 && files.length === 0" class="empty">
      <el-empty description="暂无文件" />
    </div>

    <!-- 分页 -->
    <el-pagination v-if="pagination.total > pagination.page_size"
      :current-page="pagination.page" :page-size="pagination.page_size" :total="pagination.total"
      layout="prev, pager, next" @current-change="changePage" class="pagination" />

    <!-- 右键菜单 -->
    <div v-if="contextMenu.visible" class="context-menu"
      :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }">
      <div v-if="contextMenu.type === 'file'" class="menu-item" @click="handleDownload">下载</div>
      <div class="menu-item" @click="handleRename">重命名</div>
      <div v-if="contextMenu.type === 'file'" class="menu-item" @click="showMoveDialog">移动</div>
      <div class="menu-item menu-item-danger" @click="handleDelete">删除</div>
    </div>

    <!-- 移动文件对话框 -->
    <el-dialog v-model="moveDialogVisible" title="移动到" width="400px">
      <el-tree
        :data="folderTreeData"
        :props="{ label: 'name', children: 'children' }"
        node-key="id"
        highlight-current
        @node-click="onMoveTargetSelect"
      />
      <template #footer>
        <el-button @click="moveDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmMove" :disabled="moveTargetId === null">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { Folder, Document, Loading, Grid, List } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  listFiles, deleteFile, renameFile, moveFile, batchDeleteFiles, batchMoveFiles,
  getThumbnailUrl, getDownloadUrl, getAuthHeaders,
} from '../api/file'
import { listFolders, deleteFolder, renameFolder } from '../api/folder'
import { useUserStore } from '../store'

const props = defineProps({ folderId: { type: Number, default: 0 } })
const emit = defineEmits(['preview'])
const userStore = useUserStore()

const loading = ref(false)
const folders = ref([])
const files = ref([])
const pagination = ref({ page: 1, page_size: 20, total: 0 })
const viewMode = ref('grid')
const contextMenu = ref({ visible: false, x: 0, y: 0, target: null, type: '' })

// 批量选择
const selectedItems = ref([]) // [{type: 'file', id: 1}, ...]
const selectAll = ref(false)
const isIndeterminate = computed(() =>
  selectedItems.value.length > 0 && selectedItems.value.length < files.value.length
)

// 移动对话框
const moveDialogVisible = ref(false)
const moveTargetId = ref(null)
const folderTreeData = ref([])
const moveFileIds = ref([])

// 从 File 对象获取 FileObject 属性
function getFileObj(file) {
  return file.file_object || {}
}

function getThumbnailSrc(fileId) {
  const headers = getAuthHeaders()
  return `${getThumbnailUrl(fileId)}?token=${encodeURIComponent(headers.Authorization.replace('Bearer ', ''))}`
}

// 列表视图数据
const listViewData = computed(() => {
  const folderRows = folders.value.map((f) => ({ ...f, _type: 'folder' }))
  const fileRows = files.value.map((f) => {
    const fo = getFileObj(f)
    return { ...f, _type: 'file', _isImage: fo.is_image, _fileSize: fo.file_size }
  })
  return [...folderRows, ...fileRows]
})

function rowClassName({ row }) {
  if (row._type === 'file' && isSelected('file', row.id)) return 'selected-row'
  return ''
}

async function loadFiles() {
  loading.value = true
  selectedItems.value = []
  selectAll.value = false
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
  const fo = getFileObj(file)
  if (fo.is_image) {
    emit('preview', file)
  }
}

function changePage(page) {
  pagination.value.page = page
  loadFiles()
}

// 选择
function isSelected(type, id) {
  return selectedItems.value.some((s) => s.type === type && s.id === id)
}

function toggleSelect(type, id) {
  const idx = selectedItems.value.findIndex((s) => s.type === type && s.id === id)
  if (idx >= 0) {
    selectedItems.value.splice(idx, 1)
  } else {
    selectedItems.value.push({ type, id })
  }
  selectAll.value = selectedItems.value.length === files.value.length
}

function toggleSelectAll(val) {
  if (val) {
    selectedItems.value = files.value.map((f) => ({ type: 'file', id: f.id }))
  } else {
    selectedItems.value = []
  }
}

// 右键菜单
function showFileMenu(e, file) {
  contextMenu.value = { visible: true, x: e.clientX, y: e.clientY, target: file, type: 'file' }
}
function showFolderMenu(e, folder) {
  contextMenu.value = { visible: true, x: e.clientX, y: e.clientY, target: folder, type: 'folder' }
}
function onRowContextMenu(row, col, e) {
  e.preventDefault()
  if (row._type === 'file') showFileMenu(e, row)
  else showFolderMenu(e, row)
}
function onRowClick(row) {
  if (row._type === 'folder') enterFolder(row)
  else {
    const fo = getFileObj(row)
    if (fo.is_image) emit('preview', row)
  }
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

async function handleRename() {
  const target = contextMenu.value.target
  const type = contextMenu.value.type
  const currentName = type === 'file' ? target.original_name : target.name
  contextMenu.value.visible = false

  try {
    const { value } = await ElMessageBox.prompt('输入新名称', '重命名', {
      inputValue: currentName,
      inputValidator: (v) => (v && v.trim() ? true : '名称不能为空'),
    })
    if (value.trim() === currentName) return
    if (type === 'file') {
      await renameFile(target.id, value.trim())
    } else {
      await renameFolder(target.id, { name: value.trim() })
    }
    ElMessage.success('重命名成功')
    loadFiles()
  } catch (e) {}
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
    ElMessage.success('已移到回收站')
    loadFiles()
  } catch (e) {}
}

async function handleBatchDelete() {
  const fileIds = selectedItems.value.filter((s) => s.type === 'file').map((s) => s.id)
  if (!fileIds.length) return

  try {
    await ElMessageBox.confirm(`确定删除选中的 ${fileIds.length} 个文件？`, '批量删除')
    await batchDeleteFiles(fileIds)
    ElMessage.success('已移到回收站')
    loadFiles()
  } catch (e) {}
}

// 移动功能
async function loadFolderTree() {
  try {
    const res = await listFolders(0)
    folderTreeData.value = [{ id: 0, name: '根目录', children: res.data || [] }]
  } catch (e) {
    folderTreeData.value = [{ id: 0, name: '根目录', children: [] }]
  }
}

function showMoveDialog() {
  moveFileIds.value = [contextMenu.value.target.id]
  contextMenu.value.visible = false
  moveTargetId.value = null
  loadFolderTree()
  moveDialogVisible.value = true
}

function showBatchMoveDialog() {
  moveFileIds.value = selectedItems.value.filter((s) => s.type === 'file').map((s) => s.id)
  if (!moveFileIds.value.length) return
  moveTargetId.value = null
  loadFolderTree()
  moveDialogVisible.value = true
}

function onMoveTargetSelect(data) {
  moveTargetId.value = data.id
}

async function confirmMove() {
  if (moveTargetId.value === null) return
  try {
    if (moveFileIds.value.length === 1) {
      await moveFile(moveFileIds.value[0], moveTargetId.value)
    } else {
      await batchMoveFiles(moveFileIds.value, moveTargetId.value)
    }
    ElMessage.success('移动成功')
    moveDialogVisible.value = false
    loadFiles()
  } catch (e) {
    ElMessage.error('移动失败')
  }
}

function hideContextMenu() {
  contextMenu.value.visible = false
}

function formatSize(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}

function formatDate(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

watch(
  () => props.folderId,
  () => {
    pagination.value.page = 1
    loadFiles()
  },
  { immediate: true }
)

onMounted(() => document.addEventListener('click', hideContextMenu))
onUnmounted(() => document.removeEventListener('click', hideContextMenu))

defineExpose({ loadFiles })
</script>

<style scoped>
.file-list-container {
  width: 100%;
}
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  padding: 0 4px;
}
.toolbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.toolbar-right {
  display: flex;
  align-items: center;
}
.grid-view {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  position: relative;
}
.file-item {
  width: 120px;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 8px;
  border-radius: 8px;
  cursor: pointer;
  text-align: center;
  position: relative;
}
.file-item:hover {
  background: #f5f7fa;
}
.file-item.selected {
  background: #ecf5ff;
  outline: 1px solid #409eff;
}
.item-checkbox {
  position: absolute;
  top: 4px;
  left: 4px;
  opacity: 0;
}
.file-item:hover .item-checkbox,
.file-item.selected .item-checkbox {
  opacity: 1;
}
.thumbnail {
  width: 80px;
  height: 80px;
  object-fit: cover;
  border-radius: 4px;
}
.file-name {
  font-size: 12px;
  margin-top: 4px;
  word-break: break-all;
  max-height: 32px;
  overflow: hidden;
}
.file-size {
  font-size: 11px;
  color: #999;
}
.loading {
  width: 100%;
  text-align: center;
  padding: 40px;
  color: #999;
}
.empty {
  width: 100%;
}
.pagination {
  width: 100%;
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
.context-menu {
  position: fixed;
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 4px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
  z-index: 1000;
  padding: 4px 0;
}
.menu-item {
  padding: 8px 16px;
  cursor: pointer;
  font-size: 14px;
}
.menu-item:hover {
  background: #ecf5ff;
  color: #409eff;
}
.menu-item-danger:hover {
  background: #fef0f0;
  color: #f56c6c;
}
.list-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}
.list-thumb {
  width: 24px;
  height: 24px;
  object-fit: cover;
  border-radius: 2px;
}
:deep(.selected-row) {
  background-color: #ecf5ff !important;
}

@media (max-width: 768px) {
  .file-item {
    width: 90px;
  }
  .thumbnail {
    width: 60px;
    height: 60px;
  }
  .toolbar {
    flex-direction: column;
    gap: 8px;
    align-items: flex-start;
  }
}
</style>
