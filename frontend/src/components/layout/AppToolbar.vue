<script setup lang="ts">
import { NSelect } from 'naive-ui'
import { useAppStore } from '../../stores/app'
import { OpenDownloads } from '../../../bindings/scheduling-system/backend/services/resourceservice'

const appStore = useAppStore()

async function handleOpenDownloads() {
  try { await OpenDownloads() } catch { /* ignore */ }
}
</script>

<template>
  <header class="toolbar">
    <!-- 折叠侧边栏 -->
    <button
      class="toolbar-btn"
      @click="appStore.toggleSidebar()"
      title="折叠侧边栏"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <path d="M3 6h18M3 12h18M3 18h18"/>
      </svg>
    </button>

    <div class="breadcrumb">
      <span>{{ appStore.pageTitle }}</span>
      <span class="breadcrumb-sep">›</span>
      <span class="breadcrumb-current">{{ appStore.breadcrumbPath[1] || '' }}</span>
    </div>
    <div class="toolbar-spacer"></div>

    <!-- 学期选择（仅课表中心显示） -->
    <n-select
      v-if="appStore.currentPage === 'schedule'"
      v-model:value="appStore.semesterFilter"
      :options="appStore.semesterOptions"
      size="small"
      style="width: 170px"
    />

    <!-- 打开下载目录 -->
    <button
      class="toolbar-btn"
      @click="handleOpenDownloads()"
      title="打开下载目录"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2v11Z"/>
        <line x1="12" y1="11" x2="12" y2="17"/>
        <polyline points="9 14 12 17 15 14"/>
      </svg>
    </button>

    <!-- 主题切换 -->
    <button
      class="toolbar-btn"
      @click="appStore.toggleTheme()"
      :title="appStore.theme === 'dark' ? '切换到浅色主题' : '切换到深色主题'"
    >
      <!-- 太阳图标（浅色模式下显示，点击切换到深色） -->
      <svg v-if="appStore.theme === 'dark'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="5"/>
        <line x1="12" y1="1" x2="12" y2="3"/>
        <line x1="12" y1="21" x2="12" y2="23"/>
        <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/>
        <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
        <line x1="1" y1="12" x2="3" y2="12"/>
        <line x1="21" y1="12" x2="23" y2="12"/>
        <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/>
        <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
      </svg>
      <!-- 月亮图标（深色模式下显示，点击切换到浅色） -->
      <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
      </svg>
    </button>
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

.breadcrumb-sep { opacity: 0.5; }

.breadcrumb-current {
  color: var(--b3-theme-on-background);
  font-weight: 500;
}

.toolbar-spacer { flex: 1; }

.toolbar-btn {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: none;
  background: transparent;
  color: var(--b3-theme-on-surface);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  transition: background 0.15s, color 0.15s;
  flex-shrink: 0;
}

.toolbar-btn:hover {
  background: var(--b3-theme-surface-lighter);
  color: var(--b3-theme-on-background);
}

.toolbar-btn svg {
  width: 18px;
  height: 18px;
}
</style>
