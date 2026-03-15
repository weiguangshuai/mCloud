<template>
  <div class="home-container">
    <!-- 顶部导航栏 -->
    <el-header class="header">
      <div class="header-left">
        <el-button class="menu-btn" :icon="Menu" @click="sidebarVisible = !sidebarVisible" />
        <div class="logo">
          <el-icon :size="24"><Cloudy /></el-icon>
          <span class="logo-text">MCLOUD</span>
        </div>
      </div>
      <div class="header-right">
        <el-button type="primary" :icon="Upload" @click="uploadRef?.triggerUpload()">
          <span class="btn-text">上传</span>
        </el-button>
        <el-button :icon="FolderAdd" @click="showNewFolderDialog">
          <span class="btn-text">新建</span>
        </el-button>
        <el-button :icon="Delete" @click="showRecycleBin = true">
          <span class="btn-text">回收站</span>
        </el-button>
        <el-dropdown @command="handleUserCommand">
          <div class="user-info">
            <el-icon :size="18"><User /></el-icon>
            <span class="username">{{ userStore.userInfo?.nickname || userStore.userInfo?.username }}</span>
            <el-icon :size="14"><ArrowDown /></el-icon>
          </div>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="quota">
                <el-icon><Box /></el-icon>
                存储空间
              </el-dropdown-item>
              <el-dropdown-item command="logout" divided>
                <el-icon><SwitchButton /></el-icon>
                退出登录
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </el-header>

    <div class="main-content">
      <!-- 左侧文件夹树 -->
      <aside class="sidebar" :class="{ visible: sidebarVisible }">
        <div class="sidebar-header">
          <span class="sidebar-title">文件</span>
        </div>
        <FolderTree @select="handleFolderSelect" />
      </aside>

      <!-- 中间文件列表 -->
      <main class="content">
        <Breadcrumb :items="userStore.breadcrumbs" @navigate="navigateToFolder" />
        <FileList
          :folder-id="userStore.currentFolderID"
          ref="fileListRef"
          @preview="handlePreview"
        />
      </main>
    </div>

    <!-- 文件上传组件 -->
    <FileUpload ref="uploadRef" :folder-id="userStore.currentFolderID" @uploaded="refreshFileList" />

    <!-- 图片预览 -->
    <ImagePreview v-model:visible="previewVisible" :file="previewFile" />

    <!-- 回收站 -->
    <RecycleBin v-model:visible="showRecycleBin" @restored="refreshFileList" />

    <!-- 新建文件夹对话框 -->
    <el-dialog v-model="newFolderVisible" title="新建文件夹" width="400px" class="dark-dialog">
      <el-input v-model="newFolderName" placeholder="请输入文件夹名称" @keyup.enter="createNewFolder" />
      <template #footer>
        <el-button @click="newFolderVisible = false">取消</el-button>
        <el-button type="primary" @click="createNewFolder">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Upload, FolderAdd, Delete, Menu, Cloudy, User, ArrowDown, Box, SwitchButton } from '@element-plus/icons-vue'
import { useUserStore } from '../store'
import { getProfile } from '../api/auth'
import { createFolder } from '../api/folder'
import { removeToken } from '../utils/auth'
import FolderTree from '../components/FolderTree.vue'
import FileList from '../components/FileList.vue'
import FileUpload from '../components/FileUpload.vue'
import ImagePreview from '../components/ImagePreview.vue'
import Breadcrumb from '../components/Breadcrumb.vue'
import RecycleBin from '../components/RecycleBin.vue'

const router = useRouter()
const userStore = useUserStore()
const fileListRef = ref(null)
const uploadRef = ref(null)
const sidebarVisible = ref(true)
const previewVisible = ref(false)
const previewFile = ref(null)
const showRecycleBin = ref(false)
const newFolderVisible = ref(false)
const newFolderName = ref('')

// 加载用户信息
async function loadUser() {
  try {
    const res = await getProfile()
    userStore.setUser(res.data)
    userStore.resetToRoot()
  } catch (e) {
    router.push('/login')
  }
}
loadUser()

function handleFolderSelect(folder) {
  userStore.setCurrentFolder(folder.id, folder.breadcrumbs)
}

function navigateToFolder(folder) {
  userStore.setCurrentFolder(folder.id, folder.breadcrumbs)
}

function handlePreview(file) {
  previewFile.value = file
  previewVisible.value = true
}

function refreshFileList() {
  fileListRef.value?.loadFiles()
}

function showNewFolderDialog() {
  newFolderName.value = ''
  newFolderVisible.value = true
}

async function createNewFolder() {
  if (!newFolderName.value.trim()) {
    ElMessage.warning('请输入文件夹名称')
    return
  }
  try {
    await createFolder({ name: newFolderName.value.trim(), parent_id: userStore.currentFolderID })
    ElMessage.success('文件夹创建成功')
    newFolderVisible.value = false
    refreshFileList()
  } catch (e) {}
}

function handleUserCommand(command) {
  if (command === 'logout') {
    ElMessageBox.confirm('确定退出登录？', '提示', {
      confirmButtonText: '确认',
      cancelButtonText: '取消',
    }).then(() => {
      removeToken()
      userStore.clearUser()
      router.push('/login')
    }).catch(() => {})
  } else if (command === 'quota') {
    ElMessage.info(`已用 ${formatSize(userStore.userInfo?.storage_used)} / ${formatSize(userStore.userInfo?.storage_quota)}`)
  }
}

function formatSize(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}
</script>

<style scoped>
.home-container {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background-color: var(--bg-primary);
}

/* 顶部导航栏 */
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  height: 60px;
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
  box-shadow: var(--shadow-soft);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--accent-primary);
}

.logo-text {
  font-family: var(--font-title);
  font-size: 18px;
  font-weight: 600;
  letter-spacing: 1px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.header-right :deep(.el-button) {
  font-family: var(--font-body);
  font-size: 14px;
  background-color: transparent;
  border: 1px solid var(--border-color);
  color: var(--text-secondary);
  border-radius: var(--radius-md);
  transition: all var(--transition-fast);
}

.header-right :deep(.el-button:hover) {
  background-color: var(--bg-tertiary);
  border-color: var(--border-color);
  color: var(--text-primary);
}

.header-right :deep(.el-button--primary) {
  background: linear-gradient(135deg, var(--accent-primary) 0%, var(--accent-hover) 100%);
  border: none;
  color: #FFFFFF;
  box-shadow: 0 2px 8px rgba(45, 55, 72, 0.2);
}

.header-right :deep(.el-button--primary:hover) {
  box-shadow: 0 4px 12px rgba(45, 55, 72, 0.25);
}

.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  color: var(--text-secondary);
  padding: 8px 14px;
  border-radius: var(--radius-md);
  transition: all var(--transition-fast);
}

.user-info:hover {
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
}

.username {
  font-size: 14px;
}

.menu-btn {
  display: none;
}

/* 主内容区 */
.main-content {
  display: flex;
  flex: 1;
  overflow: hidden;
}

/* 侧边栏 */
.sidebar {
  width: 240px;
  background-color: var(--bg-secondary);
  border-right: 1px solid var(--border-color);
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.sidebar-header {
  padding: 20px 24px;
  border-bottom: 1px solid var(--border-color);
}

.sidebar-title {
  font-family: var(--font-title);
  font-size: 13px;
  font-weight: 500;
  letter-spacing: 1px;
  color: var(--text-muted);
  text-transform: uppercase;
}

/* 内容区 */
.content {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
  background-color: var(--bg-primary);
}

/* 下拉菜单样式 */
:deep(.el-dropdown-menu__item) {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
  padding: 10px 16px;
}

:deep(.el-dropdown-menu__item .el-icon) {
  margin-right: 4px;
}

/* 响应式 */
@media (max-width: 768px) {
  .menu-btn {
    display: inline-flex;
  }

  .sidebar {
    position: fixed;
    left: -250px;
    top: 60px;
    bottom: 0;
    z-index: 100;
    transition: left var(--transition-normal);
  }

  .sidebar.visible {
    left: 0;
  }

  .header-right .btn-text {
    display: none;
  }

  .logo-text {
    display: none;
  }

  .content {
    padding: 16px;
  }
}

/* 对话框覆盖 */
:deep(.dark-dialog) {
  background-color: var(--bg-secondary) !important;
}

:deep(.dark-dialog .el-dialog__header) {
  border-bottom: 1px solid var(--border-color);
}

:deep(.dark-dialog .el-dialog__title) {
  font-family: var(--font-title);
  letter-spacing: 1px;
}
</style>
