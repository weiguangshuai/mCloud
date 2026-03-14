<template>
  <div id="app">
    <router-view />
  </div>
</template>

<script setup>
// 根组件 - 后续会添加全局布局
</script>

<style>
/* Google Fonts - Noto Sans SC for Japanese minimalist style */
@import url('https://fonts.googleapis.com/css2?family=Noto+Sans+SC:wght@300;400;500;600;700&display=swap');

/* CSS 变量系统 - 日式极简风格 */
:root {
  /* 背景色 - 白色系 */
  --bg-primary: #FAFAFA;
  --bg-secondary: #FFFFFF;
  --bg-tertiary: #F5F5F5;

  /* 边框色 */
  --border-color: #E2E8F0;
  --border-light: #EDF2F7;

  /* 强调色 - 深灰主色 */
  --accent-primary: #2D3748;
  --accent-hover: #4A5568;
  --accent-active: #1A202C;

  /* 选中状态 - 深灰色配白字 */
  --selected-bg: #2D3748;
  --selected-hover: #4A5568;

  /* 文字色 */
  --text-primary: #1A202C;
  --text-secondary: #718096;
  --text-muted: #A0AEC0;

  /* 字体 */
  --font-title: 'Noto Sans SC', -apple-system, BlinkMacSystemFont, sans-serif;
  --font-body: 'Noto Sans SC', -apple-system, BlinkMacSystemFont, sans-serif;

  /* 圆角 */
  --radius-sm: 4px;
  --radius-md: 8px;
  --radius-lg: 16px;

  /* 过渡 */
  --transition-fast: 150ms ease;
  --transition-normal: 250ms ease;

  /* 阴影 - 柔和日式风格 */
  --shadow-soft: 0 2px 8px rgba(0, 0, 0, 0.04);
  --shadow-md: 0 4px 12px rgba(0, 0, 0, 0.08);
  --shadow-lg: 0 8px 24px rgba(0, 0, 0, 0.12);
}

/* 全局样式重置 */
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

html, body, #app {
  height: 100%;
  font-family: var(--font-body);
  background-color: var(--bg-primary);
  color: var(--text-primary);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

/* 滚动条样式 */
::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}

::-webkit-scrollbar-track {
  background: transparent;
}

::-webkit-scrollbar-thumb {
  background: var(--border-color);
  border-radius: 3px;
}

::-webkit-scrollbar-thumb:hover {
  background: var(--text-muted);
}

/* 链接样式 */
a {
  color: var(--accent-primary);
  text-decoration: none;
  transition: color var(--transition-fast);
}

a:hover {
  color: var(--accent-hover);
}

/* 标题字体 */
h1, h2, h3, h4, h5, h6 {
  font-family: var(--font-title);
  font-weight: 600;
  letter-spacing: 0.5px;
}

/* Element Plus 组件样式覆盖 - 日式极简风格 */
.el-button--primary {
  --el-button-bg-color: var(--accent-primary);
  --el-button-border-color: var(--accent-primary);
  --el-button-hover-bg-color: var(--accent-hover);
  --el-button-hover-border-color: var(--accent-hover);
  --el-button-active-bg-color: var(--accent-active);
  --el-button-active-border-color: var(--accent-active);
}

.el-input__wrapper {
  background-color: var(--bg-secondary) !important;
  box-shadow: none !important;
  border: 1px solid var(--border-color) !important;
  border-radius: var(--radius-md) !important;
  transition: all var(--transition-fast) !important;
}

.el-input__wrapper:hover {
  border-color: var(--accent-hover) !important;
}

.el-input__wrapper.is-focus {
  border-color: var(--accent-primary) !important;
  box-shadow: 0 0 0 2px rgba(45, 55, 72, 0.1) !important;
}

.el-input__inner {
  color: var(--text-primary) !important;
}

.el-input__inner::placeholder {
  color: var(--text-muted) !important;
}

.el-select .el-input__wrapper {
  background-color: var(--bg-secondary) !important;
  border-radius: var(--radius-md) !important;
}

.el-select-dropdown {
  background-color: var(--bg-secondary) !important;
  border: 1px solid var(--border-color) !important;
  border-radius: var(--radius-md) !important;
  box-shadow: var(--shadow-md) !important;
}

.el-select-dropdown__item {
  color: var(--text-secondary) !important;
}

.el-select-dropdown__item:hover,
.el-select-dropdown__item.hover {
  background-color: var(--bg-tertiary) !important;
}

.el-select-dropdown__item.selected {
  color: var(--accent-primary) !important;
}

.el-dialog {
  background-color: var(--bg-secondary) !important;
  border: 1px solid var(--border-color) !important;
  border-radius: var(--radius-lg) !important;
  box-shadow: var(--shadow-lg) !important;
}

.el-dialog__header {
  border-bottom: 1px solid var(--border-color);
}

.el-dialog__title {
  color: var(--text-primary) !important;
  font-weight: 500;
}

.el-dialog__body {
  color: var(--text-secondary) !important;
}

.el-message-box {
  background-color: var(--bg-secondary) !important;
  border: 1px solid var(--border-color) !important;
  border-radius: var(--radius-lg) !important;
  box-shadow: var(--shadow-lg) !important;
}

.el-message-box__title {
  color: var(--text-primary) !important;
}

.el-message-box__content {
  color: var(--text-secondary) !important;
}

.el-dropdown-menu {
  background-color: var(--bg-secondary) !important;
  border: 1px solid var(--border-color) !important;
  border-radius: var(--radius-md) !important;
  box-shadow: var(--shadow-md) !important;
}

.el-dropdown-menu__item {
  color: var(--text-secondary) !important;
}

.el-dropdown-menu__item:hover {
  background-color: var(--bg-tertiary) !important;
  color: var(--text-primary) !important;
}

.el-context-menu {
  background-color: var(--bg-secondary) !important;
  border: 1px solid var(--border-color) !important;
  border-radius: var(--radius-md) !important;
}

.el-context-menu__item {
  color: var(--text-secondary) !important;
}

.el-context-menu__item:hover {
  background-color: var(--bg-tertiary) !important;
  color: var(--text-primary) !important;
}

.el-menu {
  background-color: transparent !important;
  border-right: none !important;
}

.el-menu-item {
  color: var(--text-secondary) !important;
  border-radius: var(--radius-sm) !important;
  margin: 2px 8px !important;
}

.el-menu-item:hover,
.el-menu-item.is-active {
  background-color: var(--selected-bg) !important;
  color: #FFFFFF !important;
}

.el-breadcrumb__inner {
  color: var(--text-secondary) !important;
}

.el-breadcrumb__inner.is-link:hover {
  color: var(--accent-primary) !important;
}

.el-breadcrumb__separator {
  color: var(--text-muted) !important;
}

.el-table {
  background-color: transparent !important;
  color: var(--text-secondary) !important;
}

.el-table tr {
  background-color: var(--bg-secondary) !important;
}

.el-table th.el-table__cell {
  background-color: var(--bg-tertiary) !important;
  color: var(--text-primary) !important;
  font-weight: 500;
}

.el-table td.el-table__cell {
  border-bottom-color: var(--border-color) !important;
}

.el-table--enable-row-hover .el-table__body tr:hover > td.el-table__cell {
  background-color: var(--bg-tertiary) !important;
}

.el-pagination {
  --el-pagination-bg-color: var(--bg-tertiary);
  --el-pagination-text-color: var(--text-secondary);
  --el-pagination-button-bg-color: var(--bg-secondary);
  --el-pagination-hover-color: var(--accent-primary);
}

.el-pagination button:disabled {
  background-color: var(--bg-tertiary) !important;
}

.el-pager li {
  background-color: var(--bg-secondary) !important;
  color: var(--text-secondary) !important;
  border-radius: var(--radius-sm) !important;
}

.el-pager li:hover {
  color: var(--accent-primary) !important;
}

.el-pager li.is-active {
  background-color: var(--selected-bg) !important;
  color: #FFFFFF !important;
}

.el-tag {
  --el-tag-bg-color: var(--bg-tertiary);
  --el-tag-border-color: var(--border-color);
  --el-tag-text-color: var(--text-secondary);
  border-radius: var(--radius-sm) !important;
}

.el-tag--success {
  --el-tag-bg-color: rgba(72, 187, 120, 0.15);
  --el-tag-border-color: rgba(72, 187, 120, 0.3);
  --el-tag-text-color: #48BB78;
}

.el-tag--warning {
  --el-tag-bg-color: rgba(237, 137, 54, 0.15);
  --el-tag-border-color: rgba(237, 137, 54, 0.3);
  --el-tag-text-color: #ED8936;
}

.el-tag--danger {
  --el-tag-bg-color: rgba(245, 101, 101, 0.15);
  --el-tag-border-color: rgba(245, 101, 101, 0.3);
  --el-tag-text-color: #F56565;
}

.el-empty__description {
  color: var(--text-muted) !important;
}

.el-loading-mask {
  background-color: rgba(250, 250, 250, 0.9) !important;
}

.el-loading-spinner .circular {
  stroke: var(--accent-primary) !important;
}

.el-loading-text {
  color: var(--text-secondary) !important;
}
</style>
