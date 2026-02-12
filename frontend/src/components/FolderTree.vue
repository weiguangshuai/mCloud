<template>
  <div class="folder-tree">
    <el-tree
      ref="treeRef"
      :data="treeData"
      :props="treeProps"
      node-key="id"
      lazy
      :load="loadNode"
      highlight-current
      :expand-on-click-node="false"
      @node-click="handleNodeClick"
    >
      <template #default="{ data }">
        <div class="node-content">
          <el-icon><Folder /></el-icon>
          <span>{{ data.name }}</span>
        </div>
      </template>
    </el-tree>
  </div>
</template>

<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { Folder } from '@element-plus/icons-vue'
import { listFolders } from '../api/folder'
import { useUserStore } from '../store'

const emit = defineEmits(['select'])
const userStore = useUserStore()
const treeRef = ref(null)
const treeData = ref([])

const treeProps = {
  label: 'name',
  children: 'children',
  isLeaf: 'isLeaf',
}

const rootId = computed(() => userStore.userInfo?.root_folder_id || 0)

function buildRootNode() {
  return [
    {
      id: rootId.value,
      name: '全部文件',
      isRoot: true,
      isLeaf: false,
    },
  ]
}

async function loadNode(node, resolve) {
  if (node.level === 0) {
    resolve(treeData.value)
    return
  }

  try {
    const res = await listFolders(node.data.id)
    const children = (res.data || []).map((folder) => ({
      id: folder.id,
      name: folder.name,
      isLeaf: false,
    }))
    resolve(children)
  } catch (e) {
    resolve([])
  }
}

function handleNodeClick(data, node) {
  const breadcrumbs = []
  let current = node
  while (current && current.level > 0) {
    const itemName = current.level === 1 ? '根目录' : current.data.name
    breadcrumbs.unshift({ id: current.data.id, name: itemName })
    current = current.parent
  }

  if (breadcrumbs.length === 0) {
    breadcrumbs.push({ id: rootId.value, name: '根目录' })
  }

  emit('select', { id: data.id, breadcrumbs })
}

watch(
  () => rootId.value,
  async () => {
    treeData.value = buildRootNode()
    await nextTick()
    treeRef.value?.setCurrentKey(userStore.currentFolderID || rootId.value)
  },
  { immediate: true }
)

watch(
  () => userStore.currentFolderID,
  (folderId) => {
    treeRef.value?.setCurrentKey(folderId)
  }
)
</script>

<style scoped>
.folder-tree {
  padding: 4px;
}
.node-content {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
