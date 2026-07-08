<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../../stores/app'
import { useScheduleStore } from '../../stores/schedule'
import { useResourceStore } from '../../stores/resource'
import type { PageId, ScheduleView } from '../../types'

const appStore = useAppStore()
const scheduleStore = useScheduleStore()
const resourceStore = useResourceStore()

/** 侧栏导航结构 */
interface NavChild {
  label: string
  page: PageId
  scheduleView?: ScheduleView
  resourceTab?: 'teacher' | 'classroom' | 'course' | 'class'
}

interface NavGroup {
  label: string
  icon: string   // SVG path data
  children: NavChild[]
}

// 导航数据（和 store 保持同步）
const navGroups: NavGroup[] = [
  {
    label: '课表中心',
    icon: 'M3 6h18M3 12h18M3 18h12',
    children: [
      { label: '周视图课表', page: 'schedule', scheduleView: 'week' },
      { label: '时间线视图', page: 'schedule', scheduleView: 'timeline' },
      { label: '月视图', page: 'schedule', scheduleView: 'month' },
    ],
  },
  {
    label: '资源管理',
    icon: 'M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M9 3a4 4 0 0 1 0 7.75M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75',
    children: [
      { label: '教师管理', page: 'resource', resourceTab: 'teacher' },
      { label: '教室管理', page: 'resource', resourceTab: 'classroom' },
      { label: '课程管理', page: 'resource', resourceTab: 'course' },
      { label: '班级管理', page: 'resource', resourceTab: 'class' },
    ],
  },
  {
    label: '排课引擎',
    icon: 'M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z',
    children: [
      { label: '自动排课', page: 'scheduling' },
      { label: '验证报告', page: 'report' },
    ],
  },
  {
    label: '系统设置',
    icon: 'M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42',
    children: [
      { label: '基本设置', page: 'settings' },
    ],
  },
]

// 使用 store 的展开状态
function isGroupOpen(label: string): boolean {
  return appStore.expandedGroups.includes(label)
}

// 活跃的子菜单项
const activeChild = computed(() => appStore.breadcrumbPath[1] || '')

function handleNavClick(child: NavChild) {
  appStore.navigateTo(child.page, child.label)
  if (child.scheduleView) scheduleStore.switchView(child.scheduleView)
  if (child.resourceTab) resourceStore.switchTab(child.resourceTab)
}
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-header">
      <svg class="sidebar-logo" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
        <!-- 外框圆角矩形 -->
        <rect x="2" y="3" width="28" height="26" rx="5" stroke="currentColor" stroke-width="2.2" />
        <!-- 顶部横线（表头） -->
        <line x1="2" y1="11" x2="30" y2="11" stroke="currentColor" stroke-width="2" />
        <!-- 纵向分隔线 -->
        <line x1="10" y1="11" x2="10" y2="29" stroke="currentColor" stroke-width="1.2" opacity="0.5" />
        <line x1="18" y1="11" x2="18" y2="29" stroke="currentColor" stroke-width="1.2" opacity="0.5" />
        <!-- 中间横线 -->
        <line x1="10" y1="18" x2="30" y2="18" stroke="currentColor" stroke-width="1" opacity="0.35" />
        <line x1="10" y1="24" x2="30" y2="24" stroke="currentColor" stroke-width="1" opacity="0.35" />
        <!-- 左下角小方块（代表已排课） -->
        <rect x="4" y="14" width="4" height="3" rx="1" fill="currentColor" opacity="0.6" />
        <rect x="4" y="20" width="4" height="3" rx="1" fill="currentColor" opacity="0.3" />
      </svg>
      <span class="sidebar-title">高校排课系统</span>
    </div>

    <nav class="sidebar-nav">
      <div
        v-for="group in navGroups"
        :key="group.label"
        class="nav-group"
        :class="{ open: isGroupOpen(group.label) }"
      >
        <div class="nav-group-label" @click="appStore.toggleNavGroup(group.label)">
          <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
            <path :d="group.icon" />
          </svg>
          {{ group.label }}
          <svg class="nav-arrow" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="9 18 15 12 9 6" />
          </svg>
        </div>
        <div class="nav-children">
          <div
            v-for="child in group.children"
            :key="child.label"
            class="nav-child"
            :class="{ active: activeChild === child.label }"
            @click="handleNavClick(child)"
          >
            <span class="dot"></span>
            {{ child.label }}
          </div>
        </div>
      </div>
	    </nav>
	  </aside>
</template>

<style scoped>
.sidebar {
  width: var(--sidebar-width);
  min-width: var(--sidebar-width);
  background: var(--b3-theme-surface);
  border-right: 1px solid var(--b3-border-color);
  display: flex;
  flex-direction: column;
  transition: var(--b3-transition);
  z-index: 10;
}

.sidebar-header {
  height: 52px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  border-bottom: 1px solid var(--b3-border-color);
  gap: 10px;
  flex-shrink: 0;
}

.sidebar-logo {
  width: 28px;
  height: 28px;
  color: var(--b3-theme-primary);
  flex-shrink: 0;
}

.sidebar-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  white-space: nowrap;
}

.sidebar-nav {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}

.nav-group {
  margin-bottom: 4px;
}

.nav-group-label {
  display: flex;
  align-items: center;
  padding: 8px 16px;
  font-size: 13px;
  font-weight: 500;
  color: var(--b3-theme-on-surface);
  cursor: pointer;
  transition: background .15s;
  gap: 8px;
  user-select: none;
}

.nav-group-label:hover {
  background: var(--b3-list-hover);
}

.nav-group-label .nav-icon {
  width: 18px; height: 18px;
  flex-shrink: 0;
  opacity: .6;
}

.nav-group-label .nav-arrow {
  margin-left: auto;
  width: 16px; height: 16px;
  transition: transform .2s;
  opacity: .4;
}

.nav-group.open .nav-arrow {
  transform: rotate(90deg);
}

.nav-children {
  display: none;
}

.nav-group.open .nav-children {
  display: block;
}

.nav-child {
  display: flex;
  align-items: center;
  padding: 6px 16px 6px 42px;
  font-size: 13px;
  color: var(--b3-theme-on-surface);
  cursor: pointer;
  transition: background .15s;
  gap: 6px;
}

.nav-child:hover {
  background: var(--b3-list-hover);
}

.nav-child.active {
  color: var(--b3-theme-primary);
  background: var(--b3-theme-primary-lightest);
  font-weight: 500;
}

.nav-child .dot {
  width: 5px; height: 5px;
  border-radius: 50%;
  background: currentColor;
  opacity: .5;
  flex-shrink: 0;
}

.sidebar-footer {
  border-top: 1px solid var(--b3-border-color);
  padding: 10px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.user-avatar {
  width: 28px; height: 28px;
  border-radius: 50%;
  background: var(--b3-theme-primary-lighter);
  display: flex; align-items: center; justify-content: center;
  font-size: 12px; font-weight: 600;
  color: var(--b3-theme-primary);
  flex-shrink: 0;
}

.user-info {
  flex: 1; min-width: 0;
}

.user-name {
  font-size: 13px; font-weight: 500;
  color: var(--b3-theme-on-background);
}

.user-role {
  font-size: 11px;
  color: var(--b3-theme-on-surface-light);
}
</style>
