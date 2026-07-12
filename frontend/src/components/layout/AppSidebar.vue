<script setup lang="ts">
import { computed } from 'vue'
import { useAppStore } from '../../stores/app'
import { useScheduleStore } from '../../stores/schedule'
import { useResourceStore } from '../../stores/resource'
import { NTooltip, NPopover } from 'naive-ui'

const appStore = useAppStore()
const scheduleStore = useScheduleStore()
const resourceStore = useResourceStore()

// Navigation data: single source from appStore.navGroups
const navGroups = computed(() => appStore.navGroups)

// 使用 store 的展开状态
function isGroupOpen(label: string): boolean {
  return appStore.expandedGroups.includes(label)
}

// 活跃的子菜单项
const activeChild = computed(() => appStore.breadcrumbPath[1] || '')

function handleNavClick(child: { page: string; label: string; scheduleView?: string; resourceTab?: string }) {
  appStore.navigateTo(child.page, child.label)
  if (child.scheduleView) scheduleStore.switchView(child.scheduleView)
  if (child.resourceTab) resourceStore.switchTab(child.resourceTab)
}
</script>

<template>
  <aside class="sidebar" :class="{ collapsed: appStore.sidebarCollapsed }">
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
        :class="{ open: isGroupOpen(group.label) && !appStore.sidebarCollapsed }"
      >
        <!-- 展开态 -->
        <template v-if="!appStore.sidebarCollapsed">
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
        </template>

        <!-- 折叠态：Tooltip + Popover -->
        <template v-else>
          <n-tooltip placement="right" :delay="500">
            <template #trigger>
              <n-popover trigger="click" placement="right-start">
                <template #trigger>
                  <div class="nav-group-label collapsed-nav-item">
                    <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
                      <path :d="group.icon" />
                    </svg>
                  </div>
                </template>
                <div class="collapse-submenu">
                  <div
                    v-for="child in group.children"
                    :key="child.label"
                    class="collapse-submenu-item"
                    @click="handleNavClick(child)"
                  >
                    {{ child.label }}
                  </div>
                </div>
              </n-popover>
            </template>
            {{ group.label }}
          </n-tooltip>
        </template>
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
  transition: width 0.25s ease, min-width 0.25s ease;
  z-index: 10;
}

/* 折叠状态 */
.sidebar.collapsed {
  width: var(--sidebar-collapsed-width);
  min-width: var(--sidebar-collapsed-width);
}

.sidebar.collapsed .sidebar-title {
  display: none;
}

.sidebar.collapsed .sidebar-header {
  justify-content: center;
  padding: 0 10px;
}

/* 折叠态导航项：仅图标居中 */
.collapsed-nav-item {
  justify-content: center !important;
  padding: 12px 8px !important;
}

.collapsed-nav-item .nav-icon {
  margin: 0 !important;
  opacity: 0.65;
}

/* 折叠态弹出子菜单 */
.collapse-submenu {
  min-width: 140px;
  padding: 4px 0;
}

.collapse-submenu-item {
  padding: 8px 16px;
  font-size: 13px;
  color: var(--b3-theme-on-surface);
  cursor: pointer;
  transition: background 0.15s;
  white-space: nowrap;
}

.collapse-submenu-item:hover {
  background: var(--b3-list-hover);
  color: var(--b3-theme-primary);
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
