<template>
  <div class="breadcrumb-container">
    <el-breadcrumb separator="/">
      <el-breadcrumb-item v-for="(item, idx) in items" :key="item.id">
        <a
          v-if="idx < items.length - 1"
          href="#"
          class="breadcrumb-link"
          @click.prevent="navigate(item, idx)"
        >
          <el-icon v-if="idx === 0"><HomeFilled /></el-icon>
          <span>{{ item.name }}</span>
        </a>
        <span v-else class="breadcrumb-current">
          <el-icon v-if="idx === 0"><HomeFilled /></el-icon>
          <span>{{ item.name }}</span>
        </span>
      </el-breadcrumb-item>
    </el-breadcrumb>
  </div>
</template>

<script setup>
import { HomeFilled } from '@element-plus/icons-vue'

const props = defineProps({ items: { type: Array, default: () => [] } })
const emit = defineEmits(['navigate'])

function navigate(item, idx) {
  const breadcrumbs = props.items.slice(0, idx + 1)
  emit('navigate', { id: item.id, breadcrumbs })
}
</script>

<style scoped>
.breadcrumb-container {
  margin-bottom: 16px;
  padding: 12px 16px;
  background-color: var(--bg-secondary);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
}

:deep(.el-breadcrumb) {
  font-size: 13px;
}

:deep(.el-breadcrumb__item) {
  display: flex;
  align-items: center;
}

:deep(.el-breadcrumb__inner) {
  color: var(--text-secondary) !important;
  font-weight: 400;
}

:deep(.el-breadcrumb__separator) {
  color: var(--text-muted) !important;
}

.breadcrumb-link {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--text-secondary) !important;
  transition: color var(--transition-fast);
}

.breadcrumb-link:hover {
  color: var(--accent-primary) !important;
}

.breadcrumb-current {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--text-primary) !important;
  font-weight: 500;
}

.breadcrumb-link .el-icon,
.breadcrumb-current .el-icon {
  font-size: 14px;
}
</style>
