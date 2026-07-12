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

    <!-- 全局学期选择 -->
    <n-select
      :value="appStore.currentSemesterId"
      @update:value="appStore.setCurrentSemester($event)"
      :options="appStore.semesters.map(s => ({ label: s.name, value: s.ID }))"
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
