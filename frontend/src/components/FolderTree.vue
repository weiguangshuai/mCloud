<template>
  <div class="folder-tree">
    <div class="tree-item root" :class="{ active: currentId === 0 }" @click="selectFolder(0, '根目录')">
      <el-icon><Folder /></el-icon>
      <span>全部文件</span>
    </div>
    <div v-for="folder in folders" :key="folder.id" class="tree-item"
      :class="{ active: currentId === folder.id }" @click="selectFolder(folder.id, folder.name)">
      <el-icon><Folder /></el-icon>
      <span>{{ folder.name }}</span>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { Folder } from '@element-plus/icons-vue'
import { listFolders } from '../api/folder'
import { useUserStore } from '../store'

const emit = defineEmits(['select'])
const userStore = useUserStore()
const folders = ref([])
const currentId = ref(0)

async function loadFolders(parentId = 0) {
  try {
    const res = await listFolders(parentId)
    folders.value = res.data || []
  } catch (e) {}
}

function selectFolder(id, name) {
  currentId.value = id
  const breadcrumbs = [{ id: 0, name: '根目录' }]
  if (id !== 0) {
    breadcrumbs.push({ id, name })
  }
  emit('select', { id, breadcrumbs })
}

watch(() => userStore.currentFolderID, (val) => {
  loadFolders(val)
}, { immediate: true })
</script>

<style scoped>
.folder-tree { padding: 4px; }
.tree-item {
  display: flex; align-items: center; gap: 8px; padding: 8px 12px;
  cursor: pointer; border-radius: 4px; margin-bottom: 2px;
}
.tree-item:hover { background: #ecf5ff; }
.tree-item.active { background: #409eff; color: #fff; }
</style>
