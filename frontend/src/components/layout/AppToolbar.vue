<script setup lang="ts">
import { NButton, NSelect, NInput, NIcon } from 'naive-ui'
import { useAppStore } from '../../stores/app'
import { useScheduleStore } from '../../stores/schedule'
import { DEPARTMENTS } from '../../types'

const appStore = useAppStore()
const scheduleStore = useScheduleStore()

const deptOptions = [
  { label: '全部院系', value: '全部院系' },
  ...DEPARTMENTS.map(d => ({ label: d.name, value: d.name })),
]

const semesterOptions = [
  { label: '2025-2026 第二学期', value: '2025-2026 第二学期' },
  { label: '2025-2026 第一学期', value: '2025-2026 第一学期' },
  { label: '2024-2025 第二学期', value: '2024-2025 第二学期' },
]

const viewButtons = [
  { key: 'week' as const, label: '周', icon: '📅' },
  { key: 'timeline' as const, label: '时间线', icon: '📋' },
  { key: 'month' as const, label: '月', icon: '📆' },
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

    <!-- 视图切换（课表页显示） -->
    <div v-if="appStore.currentPage === 'schedule'" class="view-switcher">
      <n-button
        v-for="btn in viewButtons"
        :key="btn.key"
        :type="scheduleStore.currentView === btn.key ? 'primary' : 'default'"
        size="small"
        @click="scheduleStore.switchView(btn.key)"
      >
        {{ btn.icon }} {{ btn.label }}
      </n-button>
    </div>

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

.view-switcher {
  display: flex;
  gap: 4px;
}
</style>
