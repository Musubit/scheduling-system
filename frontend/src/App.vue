<script setup lang="ts">
import { computed, watch } from 'vue'
import { NConfigProvider, darkTheme } from 'naive-ui'
import { useAppStore } from './stores/app'
import AppSidebar from './components/layout/AppSidebar.vue'
import AppToolbar from './components/layout/AppToolbar.vue'
import AppDrawer from './components/layout/AppDrawer.vue'

import SchedulePage from './views/SchedulePage.vue'
import ResourcePage from './views/ResourcePage.vue'
import SchedulingPage from './views/SchedulingPage.vue'
import ConflictPage from './views/ConflictPage.vue'
import SettingsPage from './views/SettingsPage.vue'

import type { PageId } from './types'

const appStore = useAppStore()

// Naive UI 主题适配
const isDark = computed(() => appStore.theme === 'dark')

// Naive UI themeOverrides — 映射子午配色
const themeOverrides = computed(() => ({
  common: {
    primaryColor: '#3575f0',
    primaryColorHover: '#5b8af7',
    primaryColorPressed: '#2b62d0',
    primaryColorSuppl: '#ff9200',
    bodyColor: isDark.value ? '#1e1e1e' : '#ffffff',
    cardColor: isDark.value ? '#262626' : '#f6f6f6',
    modalColor: isDark.value ? '#1e1e1e' : '#ffffff',
    popoverColor: isDark.value ? '#262626' : '#ffffff',
    textColorBase: isDark.value ? '#e0e0e0' : '#222222',
    textColor1: isDark.value ? '#e0e0e0' : '#222222',
    textColor2: isDark.value ? '#a0a4a8' : '#5f6368',
    textColor3: isDark.value ? 'rgba(160,164,168,0.68)' : 'rgba(95,99,104,0.68)',
    borderColor: isDark.value ? '#3a3a3a' : '#e0e0e0',
    borderRadius: '6px',
    fontFamily: 'BlinkMacSystemFont, Helvetica, "PingFang SC", "Luxi Sans", "DejaVu Sans", arial, "Microsoft Yahei", "Hiragino Sans GB", "Source Han Sans SC", sans-serif',
    fontSize: '14px',
    scrollbarColor: isDark.value ? 'rgba(255,255,255,0.2)' : 'rgba(0,0,0,0.2)',
    scrollbarColorHover: isDark.value ? 'rgba(255,255,255,0.35)' : 'rgba(0,0,0,0.35)',
  },
}))

// 页面组件映射
const pageComponents: Record<PageId, any> = {
  schedule: SchedulePage,
  resource: ResourcePage,
  scheduling: SchedulingPage,
  conflict: ConflictPage,
  settings: SettingsPage,
}

const currentPageComponent = computed(() => pageComponents[appStore.currentPage])

// 初始化主题
watch(() => appStore.theme, (val) => {
  document.documentElement.setAttribute('data-theme', val)
}, { immediate: true })
</script>

<template>
  <n-config-provider :theme="isDark ? darkTheme : null" :theme-overrides="themeOverrides">
    <div class="app-layout" :data-theme="appStore.theme">
      <AppSidebar />
      <main class="main-content">
        <AppToolbar />
        <div class="page-container">
          <component :is="currentPageComponent" />
        </div>
      </main>
      <AppDrawer />
    </div>
  </n-config-provider>
</template>

<style scoped>
.main-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: var(--b3-body-background);
}

.page-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 20px;
  background: var(--b3-body-background);
}
</style>
