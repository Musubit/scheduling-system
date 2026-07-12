<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { DAY_NAMES, PERIODS } from '../../types'
import type { LockedTimeSlot } from '../../types'
import { DEFAULT_LOCKED } from '../../stores/scheduling'

const lockedSlots = ref<LockedTimeSlot[]>([])

const emit = defineEmits<{
  change: [slots: LockedTimeSlot[]]
}>()

function loadLockedSlots() {
  try {
    const saved = localStorage.getItem('locked-time-slots')
    if (saved) {
      const parsed = JSON.parse(saved)
      if (Array.isArray(parsed)) {
        lockedSlots.value = parsed
        return
      }
    }
    lockedSlots.value = [...DEFAULT_LOCKED]
  } catch {
    lockedSlots.value = [...DEFAULT_LOCKED]
  }
}

function saveLockedSlots() {
  localStorage.setItem('locked-time-slots', JSON.stringify(lockedSlots.value))
  import('../../../bindings/scheduling-system/backend/services/resourceservice').then(({ SaveSetting }) => {
    SaveSetting('locked_time_slots', JSON.stringify(lockedSlots.value)).catch((err: any) => {
      console.warn('[LockedTimeGrid] 保存到数据库失败:', err)
    })
  })
  emit('change', lockedSlots.value)
}

function isLocked(day: number, period: number): boolean {
  return lockedSlots.value.some(ls =>
    ls.dayOfWeek === day &&
    period >= ls.startPeriod &&
    period < ls.startPeriod + ls.span
  )
}

function toggleCell(day: number, period: number) {
  const startPeriod = period % 2 === 0 ? period : period - 1
  const span = 2
  // Match by day+startPeriod only (the span may differ: original DEFAULT_LOCKED has span=4)
  const existingIdx = lockedSlots.value.findIndex(ls =>
    ls.dayOfWeek === day && ls.startPeriod === startPeriod
  )
  if (existingIdx >= 0) {
    lockedSlots.value.splice(existingIdx, 1)
  } else {
    lockedSlots.value.push({ dayOfWeek: day, startPeriod, span })
  }
  saveLockedSlots()
}

onMounted(loadLockedSlots)
</script>

<template>
  <div class="locked-grid">
    <!-- Header row: corner + 7 day labels -->
    <div class="lg-corner"></div>
    <div v-for="(name, di) in DAY_NAMES" :key="di" class="lg-col-label">{{ name }}</div>

    <!-- Data rows -->
    <template v-for="(p) in PERIODS" :key="p.num">
      <div class="lg-row-label">第{{ p.num }}节</div>
      <div
        v-for="(_, di) in 7"
        :key="'c' + p.num + '-' + di"
        class="lg-cell"
        :class="{ locked: isLocked(di, p.num - 1) }"
        @click="toggleCell(di, p.num - 1)"
        :title="(isLocked(di, p.num - 1) ? '解锁' : '锁定') + ' ' + DAY_NAMES[di] + ' 第' + p.num + '节'"
      ></div>
    </template>
  </div>
</template>

<style scoped>
.locked-grid {
  display: grid;
  grid-template-columns: 60px repeat(7, 1fr);
  gap: 2px;
  user-select: none;
  width: 100%;
}

.lg-corner {
  grid-row: 1;
  grid-column: 1;
}

.lg-col-label {
  grid-row: 1;
  font-size: 11px;
  color: var(--b3-text-color-2);
  font-weight: 600;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
}

.lg-row-label {
  font-size: 11px;
  color: var(--b3-text-color-2);
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
}

.lg-cell {
  min-height: 36px;
  border-radius: 3px;
  border: 1px solid var(--b3-border-color);
  background: var(--b3-theme-surface-light);
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s;
}

.lg-cell:hover {
  border-color: var(--b3-theme-primary);
  background: var(--b3-theme-primary-lightest);
}

.lg-cell.locked {
  background: rgba(244, 67, 54, 0.25);
  border-color: rgba(244, 67, 54, 0.4);
}

.lg-cell.locked:hover {
  background: rgba(244, 67, 54, 0.35);
}
</style>
