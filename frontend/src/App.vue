<script setup lang="ts">
import { computed, ref, provide, watch, onMounted, defineAsyncComponent } from 'vue'
import { NConfigProvider, NDialogProvider, NMessageProvider, darkTheme } from 'naive-ui'
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

import type { PageId } from './types'

const appStore = useAppStore()
const scheduleStore = useScheduleStore()
const resourceStore = useResourceStore()

const appLoading = ref(true)

// Load data from Go backend on startup — sequential to avoid semester race
onMounted(async () => {
  try {
    // 1. Load semesters first (initSemester may have raced with Wails backend)
    await appStore.loadSemesters()
    // 2. Ensure semesterFilter is set
    if (!appStore.semesterFilter && appStore.semesterOptions.length > 0) {
      appStore.semesterFilter = appStore.semesterOptions[0].value
    }
    // 3. Now load resources and schedule with the correct semester
    await Promise.all([
      resourceStore.loadAll(),
      scheduleStore.loadSchedule(appStore.semesterFilter),
    ])
  } catch (err) {
    console.error('App init failed:', err)
  } finally {
    appLoading.value = false
  }
})

// Watch semester changes → reload schedule
watch(() => appStore.semesterFilter, (newSemester) => {
  scheduleStore.loadSchedule(newSemester)
})

// Drawer ref — shared via provide/inject so child components can open it
const drawerRef = ref<InstanceType<typeof AppDrawer>>()
provide('drawerRef', drawerRef)

// Naive UI 主题适配
const isDark = computed(() => appStore.theme === 'dark')

// Naive UI themeOverrides — 映射子午配色
const themeOverrides = computed(() => ({
  common: {
    primaryColor: '#3575f0',
    primaryColorHover: '#5b8af7',
    primaryColorPressed: '#2b62d0',
    primaryColorSuppl: '#8ab4f8',
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
	  report: ReportPage,
	  settings: SettingsPage,
	  history: HistoryComparePage,
	  system: SystemManagementPage,
	}

const currentPageComponent = computed(() => pageComponents[appStore.currentPage])

// 初始化主题
watch(() => appStore.theme, (val) => {
  document.documentElement.setAttribute('data-theme', val)
}, { immediate: true })
</script>

<template>
  <n-config-provider :theme="isDark ? darkTheme : null" :theme-overrides="themeOverrides">
    <n-dialog-provider>
    <n-message-provider>
    <div class="app-layout" :data-theme="appStore.theme">
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
