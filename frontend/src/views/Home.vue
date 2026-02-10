<template>
  <div class="home-container">
    <!-- 顶部导航栏 -->
    <el-header class="header">
      <div class="header-left">
        <el-button class="menu-btn" :icon="Menu" @click="sidebarVisible = !sidebarVisible" />
        <h3>mCloud</h3>
      </div>
      <div class="header-right">
        <el-button type="primary" :icon="Upload" @click="uploadRef?.triggerUpload()">上传</el-button>
        <el-button :icon="FolderAdd" @click="showNewFolderDialog">新建文件夹</el-button>
        <el-button :icon="Delete" @click="showRecycleBin = true">回收站</el-button>
        <el-dropdown @command="handleUserCommand">
          <span class="user-info">{{ userStore.userInfo?.nickname || userStore.userInfo?.username }}</span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="quota">存储空间</el-dropdown-item>
              <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </el-header>

    <div class="main-content">
      <!-- 左侧文件夹树 -->
      <aside class="sidebar" :class="{ visible: sidebarVisible }">
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
    <el-dialog v-model="newFolderVisible" title="新建文件夹" width="400px">
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
import { Upload, FolderAdd, Delete, Menu } from '@element-plus/icons-vue'
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
    ElMessageBox.confirm('确定退出登录？', '提示').then(() => {
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
.home-container { display: flex; flex-direction: column; height: 100vh; }
.header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 16px; border-bottom: 1px solid #e4e7ed; background: #fff;
}
.header-left { display: flex; align-items: center; gap: 12px; }
.header-left h3 { color: #409eff; margin: 0; }
.header-right { display: flex; align-items: center; gap: 8px; }
.user-info { cursor: pointer; color: #409eff; }
.menu-btn { display: none; }
.main-content { display: flex; flex: 1; overflow: hidden; }
.sidebar { width: 250px; border-right: 1px solid #e4e7ed; overflow-y: auto; background: #fafafa; padding: 8px; }
.content { flex: 1; padding: 16px; overflow-y: auto; }

@media (max-width: 768px) {
  .menu-btn { display: inline-flex; }
  .sidebar { position: fixed; left: -260px; top: 60px; bottom: 0; z-index: 100; transition: left 0.3s; }
  .sidebar.visible { left: 0; }
  .header-right .el-button span { display: none; }
}
</style>
