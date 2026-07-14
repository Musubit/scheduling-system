<script setup lang="ts">
import { computed, ref, provide, onMounted, defineAsyncComponent } from 'vue'
import { NConfigProvider, NDialogProvider, NMessageProvider } from 'naive-ui'
import { useAppStore } from './stores/app'
import { useScheduleStore } from './stores/schedule'
import { useResourceStore } from './stores/resource'
import AppSidebar from './components/layout/AppSidebar.vue'
import AppToolbar from './components/layout/AppToolbar.vue'
import AppDrawer from './components/layout/AppDrawer.vue'

import SchedulePage from './views/SchedulePage.vue'
import ResourcePage from './views/ResourcePage.vue'
import SchedulingPage from './views/SchedulingPage.vue'

// Lazy-load non-primary pages for smaller initial bundle
const ReportPage = defineAsyncComponent(() => import('./views/ReportPage.vue'))
const SettingsPage = defineAsyncComponent(() => import('./views/SettingsPage.vue'))
const HistoryComparePage = defineAsyncComponent(() => import('./views/HistoryComparePage.vue'))
const SystemManagementPage = defineAsyncComponent(() => import('./views/SystemManagementPage.vue'))
const ScheduleCenterPage = defineAsyncComponent(() => import('./views/ScheduleCenterPage.vue'))

import type { PageId } from './types'

const appStore = useAppStore()
const scheduleStore = useScheduleStore()
const resourceStore = useResourceStore()

const appLoading = ref(true)

// Load data from Go backend on startup
onMounted(async () => {
  try {
    await appStore.loadSemesters()
    if (!appStore.currentSemesterId && appStore.semesters.length > 0) {
      appStore.setCurrentSemester(appStore.semesters[0].ID)
    }
    await Promise.all([
      resourceStore.loadAll(),
      scheduleStore.loadSchedule(appStore.currentSemesterId),
    ])
  } catch (err) {
    console.error('App init failed:', err)
  } finally {
    appLoading.value = false
  }
})


// Drawer ref — shared via provide/inject so child components can open it
const drawerRef = ref<InstanceType<typeof AppDrawer>>()
provide('drawerRef', drawerRef)

// Naive UI themeOverrides — light theme only
const themeOverrides = {
  common: {
    primaryColor: '#3575f0',
    primaryColorHover: '#5b8af7',
    primaryColorPressed: '#2b62d0',
    primaryColorSuppl: '#8ab4f8',
    bodyColor: '#ffffff',
    cardColor: '#f6f6f6',
    modalColor: '#ffffff',
    popoverColor: '#ffffff',
    textColorBase: '#222222',
    textColor1: '#222222',
    textColor2: '#5f6368',
    textColor3: 'rgba(95,99,104,0.68)',
    borderColor: '#e0e0e0',
    borderRadius: '6px',
    fontFamily: 'BlinkMacSystemFont, Helvetica, "PingFang SC", "Luxi Sans", "DejaVu Sans", arial, "Microsoft Yahei", "Hiragino Sans GB", "Source Han Sans SC", sans-serif',
    fontSize: '14px',
    scrollbarColor: 'rgba(0,0,0,0.2)',
    scrollbarColorHover: 'rgba(0,0,0,0.35)',
  },
}

// 页面组件映射
	const pageComponents: Record<PageId, any> = {
	  schedule: SchedulePage,
	  resource: ResourcePage,
	  scheduling: SchedulingPage,
	  report: ReportPage,
	  settings: SettingsPage,
	  history: HistoryComparePage,
	  system: SystemManagementPage,
	  'schedule-center': ScheduleCenterPage,
	}

const currentPageComponent = computed(() => pageComponents[appStore.currentPage])
</script>

<template>
  <n-config-provider :theme-overrides="themeOverrides">
    <n-dialog-provider>
    <n-message-provider>
    <div class="app-layout">
      <AppSidebar />
      <main class="main-content">
        <AppToolbar />
        <div class="page-container">
          <component :is="currentPageComponent" />
        </div>
      </main>
      <AppDrawer ref="drawerRef" />
      <!-- Global loading overlay for initial data fetch -->
      <div v-if="appLoading" class="app-loading-overlay">
        <div class="app-loading-spinner"></div>
        <div class="app-loading-text">加载中...</div>
      </div>
    </div>
    </n-message-provider>
    </n-dialog-provider>
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

/* Global loading overlay */
.app-loading-overlay {
  position: fixed; inset: 0; z-index: 99999;
  background: var(--b3-body-background);
  display: flex; flex-direction: column;
  align-items: center; justify-content: center;
  gap: 16px;
}
.app-loading-spinner {
  width: 36px; height: 36px;
  border: 3px solid var(--b3-border-color);
  border-top-color: var(--b3-theme-primary);
  border-radius: 50%;
  animation: appLoadingSpin 0.8s linear infinite;
}
@keyframes appLoadingSpin {
  to { transform: rotate(360deg); }
}
.app-loading-text {
  font-size: 14px; color: var(--b3-theme-on-surface);
}
</style>
