<script setup lang="ts">
import { NButton } from 'naive-ui'
import { useAppStore } from '../../stores/app'

const appStore = useAppStore()
</script>

<template>
  <header class="toolbar">
    <div class="breadcrumb">
      <span>{{ appStore.pageTitle }}</span>
      <span class="breadcrumb-sep">›</span>
      <span class="breadcrumb-current">{{ appStore.breadcrumbPath[1] || '' }}</span>
    </div>
    <div class="toolbar-spacer"></div>

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

.breadcrumb-sep { opacity: 0.5; }

.breadcrumb-current {
  color: var(--b3-theme-on-background);
  font-weight: 500;
}

.toolbar-spacer { flex: 1; }
</style>
