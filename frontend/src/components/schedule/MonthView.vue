<script setup lang="ts">
import { computed } from 'vue'
import { useScheduleStore } from '../../stores/schedule'
import { DAY_NAMES } from '../../types'

const scheduleStore = useScheduleStore()

const dayNames = ['一', '二', '三', '四', '五', '六', '日']

const year = computed(() => scheduleStore.currentYear)
const month = computed(() => scheduleStore.currentMonth)
const daysInMonth = computed(() => new Date(year.value, month.value, 0).getDate())
const firstDayOfWeek = computed(() => {
  const d = new Date(year.value, month.value - 1, 1).getDay()
  return d === 0 ? 6 : d - 1
})

	const calendarDays = computed(() => {
	  const days: { date: number; isCurrentMonth: boolean; isToday: boolean; dow: number; courses: { name: string; dept: string }[] }[] = []
	  const prevMonthDays = new Date(year.value, month.value - 1, 0).getDate()
	  for (let i = firstDayOfWeek.value - 1; i >= 0; i--) {
	    days.push({ date: prevMonthDays - i, isCurrentMonth: false, isToday: false, dow: -1, courses: [] })
	  }
	  const today = new Date()
	  for (let d = 1; d <= daysInMonth.value; d++) {
	    const dow = (firstDayOfWeek.value + d - 1) % 7
	    const dt = new Date(year.value, month.value - 1, d)
	    const dayEntries = scheduleStore.displayEntries.filter(e => e.dayOfWeek === dow)
	    days.push({
	      date: d, isCurrentMonth: true,
	      isToday: dt.toDateString() === today.toDateString(),
	      dow,
	      courses: dayEntries.slice(0, 3).map(e => ({ name: e.course?.name || '', dept: e.course?.dept || 'cs' })),
	    })
	  }
  const remaining = 7 - (days.length % 7)
  if (remaining < 7) {
    for (let i = 1; i <= remaining; i++) {
      days.push({ date: i, isCurrentMonth: false, isToday: false, dow: -1, courses: [] })
    }
  }
  return days
})
</script>

<template>
  <div class="month-view">
    <div class="month-grid">
      <div v-for="d in dayNames" :key="d" class="month-header-cell">周{{ d }}</div>
      <div v-for="(day, idx) in calendarDays" :key="idx" class="month-cell" :class="{ 'other-month': !day.isCurrentMonth, 'is-today': day.isToday }">
        <div class="date-num">{{ day.date }}</div>
        <div v-if="day.isCurrentMonth" class="month-events">
          <div v-for="(c, ci) in day.courses" :key="ci" class="month-event" :class="'mo-' + c.dept">{{ c.name }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.month-view { flex: 1; overflow-y: auto; min-height: 0; }
.month-grid { display: grid; grid-template-columns: repeat(7, 1fr); gap: 1px; background: var(--b3-border-color); border: 1px solid var(--b3-border-color); border-radius: var(--b3-border-radius); overflow: hidden; min-height: 100%; }
.month-header-cell { background: var(--b3-theme-surface); padding: 8px; text-align: center; font-size: 12px; font-weight: 500; color: var(--b3-theme-on-surface); }
.month-cell { background: var(--b3-theme-background); min-height: 80px; padding: 4px 6px; }
.month-cell.other-month { background: var(--b3-theme-surface); opacity: 0.4; }
.month-cell.is-today { background: var(--b3-theme-primary-lightest); }
.month-cell.is-today .date-num { color: var(--b3-theme-primary); font-weight: 700; }
.date-num { font-size: 12px; color: var(--b3-theme-on-surface); margin-bottom: 4px; }
.month-events { display: flex; flex-direction: column; gap: 3px; }
.month-event { font-size: 10px; padding: 2px 4px; border-radius: 2px; border-left: 2px solid; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; line-height: 1.5; }
.mo-mech { background: var(--course-mech-bg); border-left-color: var(--course-mech-border); }
.mo-elec { background: var(--course-elec-bg); border-left-color: var(--course-elec-border); }
.mo-mate { background: var(--course-mate-bg); border-left-color: var(--course-mate-border); }
.mo-bio { background: var(--course-bio-bg); border-left-color: var(--course-bio-border); }
.mo-civil { background: var(--course-civil-bg); border-left-color: var(--course-civil-border); }
.mo-cs { background: var(--course-cs-bg); border-left-color: var(--course-cs-border); }
.mo-art { background: var(--course-art-bg); border-left-color: var(--course-art-border); }
.mo-design { background: var(--course-design-bg); border-left-color: var(--course-design-border); }
.mo-econ { background: var(--course-econ-bg); border-left-color: var(--course-econ-border); }
.mo-eng { background: var(--course-eng-bg); border-left-color: var(--course-eng-border); }
.mo-sci { background: var(--course-sci-bg); border-left-color: var(--course-sci-border); }
.mo-marx { background: var(--course-marx-bg); border-left-color: var(--course-marx-border); }
.mo-voc { background: var(--course-voc-bg); border-left-color: var(--course-voc-border); }
.mo-intl { background: var(--course-intl-bg); border-left-color: var(--course-intl-border); }
.mo-pe { background: var(--course-pe-bg); border-left-color: var(--course-pe-border); }
.mo-cont { background: var(--course-cont-bg); border-left-color: var(--course-cont-border); }
.mo-innov { background: var(--course-innov-bg); border-left-color: var(--course-innov-border); }
.mo-engtech { background: var(--course-engtech-bg); border-left-color: var(--course-engtech-border); }
.mo-detroit { background: var(--course-detroit-bg); border-left-color: var(--course-detroit-border); }
</style>
