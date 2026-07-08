<script setup lang="ts">
import { NButton, NSelect, NInput } from 'naive-ui'
import { useAppStore } from '../../stores/app'
import { DEPARTMENTS } from '../../types'

const appStore = useAppStore()

const deptOptions = [
  { label: '全部院系', value: '全部院系' },
  ...DEPARTMENTS.map(d => ({ label: d.name, value: d.name })),
]

const semesterOptions = [
  { label: '2025-2026 第二学期', value: '2025-2026 第二学期' },
  { label: '2025-2026 第一学期', value: '2025-2026 第一学期' },
  { label: '2024-2025 第二学期', value: '2024-2025 第二学期' },
]

</script>

<template>
  <header class="toolbar">
    <div class="breadcrumb">
      <span>{{ appStore.pageTitle }}</span>
      <span class="breadcrumb-sep">›</span>
      <span class="breadcrumb-current">{{ appStore.breadcrumbPath[1] || '' }}</span>
    </div>
    <div class="toolbar-spacer"></div>

    <!-- 搜索 -->
    <n-input
      v-model:value="appStore.searchQuery"
      placeholder="搜索课程、教师、教室..."
      clearable
      size="small"
      style="width: 200px"
    />

    <!-- 院系筛选 -->
    <n-select
      v-model:value="appStore.deptFilter"
      :options="deptOptions"
      size="small"
      style="width: 160px"
    />

    <!-- 学期选择 -->
    <n-select
      v-model:value="appStore.semesterFilter"
      :options="semesterOptions"
      size="small"
      style="width: 170px"
    />

    <!-- 主题切换 -->
    <n-button
      size="small"
      circle
      @click="appStore.toggleTheme()"
      :title="appStore.theme === 'dark' ? '切换到浅色主题' : '切换到深色主题'"
    >
      {{ appStore.theme === 'dark' ? '☀️' : '🌙' }}
    </n-button>
  </header>
</template>

<style scoped>
.toolbar {
  height: var(--toolbar-height);
  min-height: var(--toolbar-height);
  background: var(--b3-theme-background);
  border-bottom: 1px solid var(--b3-border-color);
  display: flex;
  align-items: center;
  padding: 0 16px;
  gap: 12px;
  z-index: 5;
}

.breadcrumb {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--b3-theme-on-surface);
  flex-shrink: 0;
}

.breadcrumb-sep {
  opacity: 0.5;
}

.breadcrumb-current {
  color: var(--b3-theme-on-background);
  font-weight: 500;
}

.toolbar-spacer {
  flex: 1;
}
</style>
