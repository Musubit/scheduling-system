<script setup lang="ts">
import { useScheduleStore } from '../../stores/schedule'

const scheduleStore = useScheduleStore()

const dayNames = ['一', '二', '三', '四', '五', '六', '日']

// Calculate first day of month offset (2026年3月 starts on Sunday = index 6)
const startDay = 6
const daysInMonth = 31
</script>

<template>
  <div class="month-view">
    <div class="month-grid">
      <div v-for="d in dayNames" :key="d" class="month-header-cell">周{{ d }}</div>

      <!-- Previous month padding -->
      <div v-for="i in startDay" :key="'prev-' + i" class="month-cell other-month">
        <div class="date-num">{{ 28 - startDay + 1 + i - 1 }}</div>
      </div>

      <!-- Current month -->
      <div
        v-for="d in daysInMonth"
        :key="d"
        class="month-cell"
        :class="{ today: d === 24 }"
      >
        <div class="date-num">{{ d }}</div>
      </div>

      <!-- Next month padding -->
      <div
        v-for="i in (6 - ((startDay + daysInMonth - 1) % 7))"
        :key="'next-' + i"
        class="month-cell other-month"
      >
        <div class="date-num">{{ i }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.month-view {
  flex: 1;
  overflow: auto;
}

.month-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 1px;
  background: var(--b3-border-color);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius);
  overflow: hidden;
}

.month-header-cell {
  background: var(--b3-theme-surface);
  padding: 8px;
  text-align: center;
  font-size: 12px;
  font-weight: 500;
  color: var(--b3-theme-on-surface);
}

.month-cell {
  background: var(--b3-theme-background);
  min-height: 100px;
  padding: 6px;
}

.month-cell.other-month {
  background: var(--b3-theme-surface);
  opacity: 0.5;
}

.month-cell.today {
  background: var(--b3-theme-primary-lightest);
}

.month-cell.today .date-num {
  color: var(--b3-theme-primary);
  font-weight: 700;
}

.date-num {
  font-size: 12px;
  color: var(--b3-theme-on-surface);
  margin-bottom: 4px;
}
</style>
