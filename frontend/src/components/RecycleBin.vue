<template>
  <el-drawer v-model="drawerVisible" title="回收站" size="400px" class="dark-drawer">
    <template #header>
      <div class="drawer-header">
        <el-icon><Delete /></el-icon>
        <span>回收站</span>
      </div>
    </template>

    <div v-if="items.length === 0" class="empty">
      <el-empty description="回收站为空">
        <template #image>
          <el-icon :size="48" color="var(--text-muted)"><DeleteFilled /></el-icon>
        </template>
      </el-empty>
    </div>
    <div v-else class="recycle-list">
      <div v-for="item in items" :key="item.id" class="recycle-item">
        <div class="item-info">
          <el-icon class="item-icon">
            <Document v-if="item.original_type === 'file'" />
            <Folder v-else />
          </el-icon>
          <div class="item-detail">
            <span class="item-name">{{ item.original_name }}</span>
            <span class="item-time">{{ formatDate(item.deleted_at) }}</span>
          </div>
        </div>
        <div class="item-actions">
          <el-button size="small" type="primary" text @click="restore(item.id)">恢复</el-button>
          <el-button size="small" type="danger" text @click="permanentDel(item.id)">删除</el-button>
        </div>
      </div>
    </div>
    <template #footer>
      <el-button type="danger" @click="emptyAll" :disabled="items.length === 0">
        <el-icon><Delete /></el-icon>
        清空回收站
      </el-button>
    </template>
  </el-drawer>
</template>

<script setup>
import { ref, watch, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Document, Folder, Delete, DeleteFilled } from '@element-plus/icons-vue'
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

function formatDate(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

watch(() => props.visible, (val) => { if (val) loadItems() })
</script>

<style scoped>
.drawer-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: var(--font-title);
  font-size: 14px;
  font-weight: 500;
  letter-spacing: 1px;
  color: var(--text-primary);
}

.recycle-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.recycle-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  border-radius: var(--radius-sm);
  background-color: var(--bg-primary);
  border: 1px solid var(--border-color);
  transition: all var(--transition-fast);
}

.recycle-item:hover {
  border-color: var(--border-light);
}

.item-info {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  overflow: hidden;
}

.item-icon {
  font-size: 20px;
  color: var(--text-muted);
}

.item-detail {
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;
}

.item-name {
  font-size: 13px;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-time {
  font-size: 11px;
  color: var(--text-muted);
}

.item-actions {
  display: flex;
  gap: 4px;
}

.empty {
  padding: 40px 0;
}

/* Drawer 样式覆盖 */
:deep(.dark-drawer) {
  background-color: var(--bg-secondary) !important;
}

:deep(.dark-drawer .el-drawer__header) {
  border-bottom: 1px solid var(--border-color);
  margin-bottom: 0;
  padding: 16px;
}

:deep(.dark-drawer .el-drawer__body) {
  padding: 12px 16px;
  background-color: var(--bg-secondary);
}

:deep(.dark-drawer .el-drawer__footer) {
  border-top: 1px solid var(--border-color);
  padding: 12px 16px;
}
</style>
