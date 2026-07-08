<script setup lang="ts">
import { DAY_NAMES } from '../../types'

const hours = ['08:00', '09:00', '10:00', '11:00', '12:00', '13:00', '14:00', '15:00', '16:00', '17:00', '18:00', '19:00', '20:00', '21:00']
</script>

<template>
  <div class="timeline-view">
    <div class="timeline-header">
      <div class="timeline-day-label"></div>
      <div class="timeline-hours">
        <div v-for="h in hours" :key="h" class="timeline-hour">{{ h }}</div>
      </div>
    </div>

    <div v-for="(day, di) in DAY_NAMES" :key="di" class="timeline-row">
      <div class="timeline-day-name">{{ day }}</div>
      <div class="timeline-track">
        <div class="timeline-placeholder">时间线视图 — 阶段2实现详细交互</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.timeline-view {
  flex: 1;
  overflow: auto;
}

.timeline-header {
  display: flex;
  margin-bottom: 8px;
}

.timeline-day-label {
  width: 56px;
  flex-shrink: 0;
}

.timeline-hours {
  display: grid;
  grid-template-columns: repeat(14, 1fr);
  flex: 1;
  gap: 1px;
}

.timeline-hour {
  font-size: 10px;
  color: var(--b3-theme-on-surface-light);
  text-align: center;
}

.timeline-row {
  display: flex;
  margin-bottom: 4px;
  align-items: center;
}

.timeline-day-name {
  width: 56px;
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--b3-theme-on-surface);
}

.timeline-track {
  flex: 1;
  height: 40px;
  background: var(--b3-theme-surface);
  border-radius: var(--b3-border-radius-s);
  border: 1px solid var(--b3-border-color);
  display: flex;
  align-items: center;
  padding: 0 12px;
}

.timeline-placeholder {
  font-size: 12px;
  color: var(--b3-theme-on-surface-light);
}
</style>
