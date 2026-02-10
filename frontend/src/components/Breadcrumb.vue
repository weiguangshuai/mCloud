<template>
  <el-breadcrumb separator="/">
    <el-breadcrumb-item v-for="(item, idx) in items" :key="item.id">
      <a v-if="idx < items.length - 1" href="#" @click.prevent="navigate(item, idx)">{{ item.name }}</a>
      <span v-else>{{ item.name }}</span>
    </el-breadcrumb-item>
  </el-breadcrumb>
</template>

<script setup>
const props = defineProps({ items: { type: Array, default: () => [] } })
const emit = defineEmits(['navigate'])

function navigate(item, idx) {
  const breadcrumbs = props.items.slice(0, idx + 1)
  emit('navigate', { id: item.id, breadcrumbs })
}
</script>

<style scoped>
.el-breadcrumb { margin-bottom: 16px; }
</style>
