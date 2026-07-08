<script setup lang="ts">
import { useScheduleStore } from '../stores/schedule'
import WeekView from '../components/schedule/WeekView.vue'
import TimelineView from '../components/schedule/TimelineView.vue'
import MonthView from '../components/schedule/MonthView.vue'

const scheduleStore = useScheduleStore()
</script>

<template>
  <div class="schedule-page">
    <div class="schedule-header">
      <div class="schedule-title">
        <template v-if="scheduleStore.currentView === 'week'">
          2025-2026 第二学期 · 第 {{ scheduleStore.currentWeek }} 周
        </template>
        <template v-else-if="scheduleStore.currentView === 'timeline'">
          时间线视图 · 第 {{ scheduleStore.currentWeek }} 周
        </template>
        <template v-else>
          {{ scheduleStore.currentYear }} 年 {{ scheduleStore.currentMonth }} 月
        </template>
      </div>
      <div class="schedule-meta">
        <span class="stat-badge">已排 {{ scheduleStore.totalCourses || '...' }} 门课</span>
      </div>
    </div>

    <WeekView v-if="scheduleStore.currentView === 'week'" />
    <TimelineView v-else-if="scheduleStore.currentView === 'timeline'" />
    <MonthView v-else />
  </div>
</template>

<style scoped>
.schedule-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.schedule-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-shrink: 0;
}

.schedule-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
}

.schedule-meta {
  display: flex;
  align-items: center;
  gap: 10px;
}

.stat-badge {
  font-size: 12px;
  color: var(--b3-theme-success);
  background: var(--b3-card-success-background);
  padding: 2px 10px;
  border-radius: var(--b3-border-radius-s);
}
</style>
