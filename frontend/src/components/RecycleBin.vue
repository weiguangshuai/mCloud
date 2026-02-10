<template>
  <el-drawer v-model="drawerVisible" title="回收站" size="400px">
    <div v-if="items.length === 0" class="empty">
      <el-empty description="回收站为空" />
    </div>
    <div v-for="item in items" :key="item.id" class="recycle-item">
      <div class="item-info">
        <el-icon><Document v-if="item.original_type === 'file'" /><Folder v-else /></el-icon>
        <span>{{ item.original_name }}</span>
      </div>
      <div class="item-actions">
        <el-button size="small" type="primary" text @click="restore(item.id)">恢复</el-button>
        <el-button size="small" type="danger" text @click="permanentDel(item.id)">删除</el-button>
      </div>
    </div>
    <template #footer>
      <el-button type="danger" @click="emptyAll" :disabled="items.length === 0">清空回收站</el-button>
    </template>
  </el-drawer>
</template>

<script setup>
import { ref, watch, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Document, Folder } from '@element-plus/icons-vue'
import { listRecycleBin, restoreItem, permanentDelete, emptyRecycleBin } from '../api/recycleBin'

const props = defineProps({ visible: { type: Boolean, default: false } })
const emit = defineEmits(['update:visible', 'restored'])

const drawerVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const items = ref([])

async function loadItems() {
  try {
    const res = await listRecycleBin({ page: 1, page_size: 50 })
    items.value = res.data?.items || []
  } catch (e) {}
}

async function restore(id) {
  try {
    await restoreItem(id)
    ElMessage.success('恢复成功')
    loadItems()
    emit('restored')
  } catch (e) {}
}

async function permanentDel(id) {
  try {
    await ElMessageBox.confirm('永久删除后无法恢复，确定？', '确认')
    await permanentDelete(id)
    ElMessage.success('已永久删除')
    loadItems()
  } catch (e) {}
}

async function emptyAll() {
  try {
    await ElMessageBox.confirm('确定清空回收站？此操作不可恢复', '确认')
    await emptyRecycleBin()
    ElMessage.success('回收站已清空')
    items.value = []
  } catch (e) {}
}

watch(() => props.visible, (val) => { if (val) loadItems() })
</script>

<style scoped>
.recycle-item {
  display: flex; justify-content: space-between; align-items: center;
  padding: 8px 0; border-bottom: 1px solid #f0f0f0;
}
.item-info { display: flex; align-items: center; gap: 8px; flex: 1; overflow: hidden; }
.item-info span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.empty { padding: 40px 0; }
</style>
