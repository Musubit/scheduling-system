<script setup lang="ts">
import { computed, ref, onUnmounted } from 'vue'
import { DAY_NAMES, PERIODS } from '../../types'
import type { ScheduleEntry } from '../../types'
import { useScheduleStore } from '../../stores/schedule'
import { courseColorStyle } from '../../utils/courseColor'
import { allocateLanes } from '../../utils/laneAllocator'

const scheduleStore = useScheduleStore()

// =============================================================================
// 模块级常量：PERIODS 解析 + 休息带检测 + 轨道百分比映射
// 这些只依赖 PERIODS（模块常量），不依赖任何响应式数据，组件创建时算一次。
// =============================================================================

const DAY_BASE = 8 * 60 // 以 08:00 作为当天分钟 0 点

interface PInfo { idx: number; startAbs: number; endAbs: number }
const P_INFOS: PInfo[] = PERIODS.map((p, i) => {
  const [s, e] = p.time.split('\n')
  const [sh, sm] = s.split(':').map(Number)
  const [eh, em] = e.split(':').map(Number)
  return { idx: i, startAbs: sh * 60 + sm, endAbs: eh * 60 + em }
})

const BREAK_GAP_THRESHOLD = 30 // 分钟
const BREAK_PCT = 3.4           // 每段休息带在轨道上的视觉宽度（%）

interface Seg { id: string; type: 'class' | 'break'; absStart: number; absEnd: number; lo: number; hi: number; label?: string }

const CLASS_BLOCKS: { absStart: number; absEnd: number }[] = []
const BREAK_DEFS: { absStart: number; absEnd: number; label: string }[] = []

{
  const breakLabels = ['午休', '晚饭']
  let blockStart = P_INFOS[0].startAbs
  let prevEnd = P_INFOS[0].endAbs
  let bi = 0
  for (let i = 1; i < P_INFOS.length; i++) {
    const gap = P_INFOS[i].startAbs - prevEnd
    if (gap >= BREAK_GAP_THRESHOLD) {
      CLASS_BLOCKS.push({ absStart: blockStart, absEnd: prevEnd })
      BREAK_DEFS.push({ absStart: prevEnd, absEnd: P_INFOS[i].startAbs, label: breakLabels[bi] ?? '休息' })
      bi++
      blockStart = P_INFOS[i].startAbs
    }
    prevEnd = P_INFOS[i].endAbs
  }
  CLASS_BLOCKS.push({ absStart: blockStart, absEnd: prevEnd })
}

const TOTAL_CLASS_MIN = CLASS_BLOCKS.reduce((a, b) => a + (b.absEnd - b.absStart), 0)
const CLASS_PCT_TOTAL = 100 - BREAK_DEFS.length * BREAK_PCT

const SEGMENTS: Seg[] = []
{
  let cursor = 0
  for (let i = 0; i < CLASS_BLOCKS.length; i++) {
    const b = CLASS_BLOCKS[i]
    const w = (b.absEnd - b.absStart) / TOTAL_CLASS_MIN * CLASS_PCT_TOTAL
    SEGMENTS.push({ id: 'c' + i, type: 'class', absStart: b.absStart, absEnd: b.absEnd, lo: cursor, hi: cursor + w })
    cursor += w
    if (i < BREAK_DEFS.length) {
      const d = BREAK_DEFS[i]
      SEGMENTS.push({ id: 'b' + i, type: 'break', absStart: d.absStart, absEnd: d.absEnd, lo: cursor, hi: cursor + BREAK_PCT, label: d.label })
      cursor += BREAK_PCT
    }
  }
}

// 真实时钟分钟 -> 轨道百分比（缓存 + 防御性检查）
const ABS_TO_PCT_CACHE = new Map<number, number>()

function absToPct(absMin: number): number {
  const cached = ABS_TO_PCT_CACHE.get(absMin)
  if (cached !== undefined) return cached

  for (const s of SEGMENTS) {
    const inRange = s.type === 'break'
      ? (absMin >= s.absStart && absMin < s.absEnd)
      : (absMin >= s.absStart && absMin <= s.absEnd)
    if (inRange) {
      const result = s.type === 'break' ? s.lo : s.lo + ((absMin - s.absStart) / (s.absEnd - s.absStart || 1)) * (s.hi - s.lo)
      ABS_TO_PCT_CACHE.set(absMin, result)
      return result
    }
  }
  const result = absMin <= SEGMENTS[0].absStart ? SEGMENTS[0].lo : SEGMENTS[SEGMENTS.length - 1].hi
  ABS_TO_PCT_CACHE.set(absMin, result)
  return result
}

// 时间轴刻度：每节课起点
const AXIS_TICKS = P_INFOS.map((p, i) => ({
  label: PERIODS[i].time.split('\n')[0],
  pct: absToPct(p.startAbs),
  idx: i + 1,
}))

// 休息带边界刻度（过滤掉与 AXIS_TICKS 重复的标签，如 14:00 / 18:30）
const AXIS_LABEL_SET = new Set(AXIS_TICKS.map(t => t.label))
const BREAK_TICKS = BREAK_DEFS.flatMap(d => [
  { label: formatAbs(d.absStart), pct: absToPct(d.absStart), type: 'break-start' as const },
  { label: formatAbs(d.absEnd), pct: absToPct(d.absEnd), type: 'break-end' as const },
]).filter(bt => !AXIS_LABEL_SET.has(bt.label))

function formatAbs(min: number): string {
  const h = Math.floor(min / 60)
  const m = min % 60
  return `${h}:${m.toString().padStart(2, '0')}`
}

// =============================================================================
// 响应式数据
// =============================================================================

// 按天分组的课程索引（computed，displayEntries 变化时重算一次）
const dayCourses = computed(() => {
  const map = new Map<number, ReturnType<typeof allocateLanes>>()
  for (let d = 0; d < 7; d++) {
    const entries = scheduleStore.displayEntries.filter(e => e.dayOfWeek === d)
    map.set(d, allocateLanes(entries))
  }
  return map
})

// 当前时间红线位置（ref + setInterval，因为 new Date() 不是响应式依赖）
const nowLinePct = ref<number | null>(null)

function updateNowLine() {
  const now = new Date()
  const min = now.getHours() * 60 + now.getMinutes()
  const pct = absToPct(min)
  nowLinePct.value = (pct >= 0 && pct <= 100) ? pct : null
}
updateNowLine()
const nowLineTimer = setInterval(updateNowLine, 60000) // 每分钟更新

// 组件卸载时清理定时器
onUnmounted(() => clearInterval(nowLineTimer))

// 选中的课程（用于高亮同课程）
const hoveredCourseId = ref<number | null>(null)

// =============================================================================
// 事件样式 + 交互
// =============================================================================

function eventStyle(e: ScheduleEntry, lane: number, totalLanes: number) {
  // 防御性检查：越界 startPeriod 时发出警告
  const startIdx = e.startPeriod
  const endIdx = e.startPeriod + e.span - 1
  if (startIdx < 0 || startIdx >= P_INFOS.length || endIdx < 0 || endIdx >= P_INFOS.length) {
    console.warn(`[TimelineView] Entry ${e.ID} has out-of-range startPeriod=${e.startPeriod}, span=${e.span}`)
    return { display: 'none' }
  }

  const startMin = P_INFOS[startIdx].startAbs
  const endMin = P_INFOS[endIdx].endAbs

  const left = absToPct(startMin)
  const width = absToPct(endMin) - left

  const laneHeight = 100 / totalLanes
  const top = lane * laneHeight

  return {
    left: left + '%',
    width: width + '%',
    top: top + '%',
    height: (laneHeight - 2) + '%',
  }
}

// 打开课程详情（复用 AppDrawer）
function openCourseDetail(e: ScheduleEntry) {
  // 通过全局事件触发 AppDrawer 打开（避免跨组件依赖）
  window.dispatchEvent(new CustomEvent('timeline:open-course', { detail: e }))
}

// 课程块信息文本
function eventInfo(e: ScheduleEntry): { name: string; sub: string; periodText: string } {
  const periodText = e.span >= 2
    ? `第${e.startPeriod + 1}-${e.startPeriod + e.span}节`
    : `第${e.startPeriod + 1}节`
  return {
    name: e.course?.name || '未知课程',
    sub: `${e.teacher?.name || '未知教师'} · ${e.classroom?.name || '未知教室'}`,
    periodText,
  }
}

// 判断是否同课程高亮
function isHighlighted(e: ScheduleEntry): boolean {
  return hoveredCourseId.value !== null && e.course?.ID === hoveredCourseId.value
}
</script>

<template>
  <div class="timeline-view">
    <div v-if="scheduleStore.displayEntries.length === 0" class="empty-state">暂无排课数据</div>
    <template v-else>
      <!-- 头部：节次刻度 -->
      <div class="tl-header">
        <div class="tl-day-label">节次</div>
        <div class="tl-hours">
          <div
            v-for="seg in SEGMENTS"
            :key="seg.id"
            class="tl-seg"
            :class="seg.type"
            :style="{ left: seg.lo + '%', width: (seg.hi - seg.lo) + '%' }"
          ></div>
          <div v-for="t in AXIS_TICKS" :key="t.idx" class="tl-hour" :style="{ left: t.pct + '%' }">{{ t.label }}</div>
          <div v-for="(bt, bi) in BREAK_TICKS" :key="'bt-' + bi" class="tl-break-tick" :style="{ left: bt.pct + '%' }">
            <span class="tl-break-tick-label">{{ bt.label }}</span>
          </div>
        </div>
      </div>

      <!-- 7 天行 -->
      <div v-for="(day, di) in DAY_NAMES" :key="di" class="tl-row">
        <div class="tl-day-name">{{ day }}</div>
        <div class="tl-track">
          <!-- 休息带背景 -->
          <div
            v-for="seg in SEGMENTS"
            :key="seg.id"
            class="tl-seg"
            :class="seg.type"
            :style="{ left: seg.lo + '%', width: (seg.hi - seg.lo) + '%' }"
          >
            <span v-if="seg.type === 'break'" class="tl-break-label">{{ seg.label }}</span>
          </div>
          <!-- 网格线 -->
          <div v-for="t in AXIS_TICKS" :key="t.idx" class="tl-gridline" :style="{ left: t.pct + '%' }"></div>
          <div v-for="(bt, bi) in BREAK_TICKS" :key="'btl-' + bi" class="tl-break-line" :style="{ left: bt.pct + '%' }"></div>
          <!-- 当前时间红线 -->
          <div v-if="nowLinePct != null" class="tl-now-line" :style="{ left: nowLinePct + '%' }">
            <span class="tl-now-dot"></span>
          </div>
          <!-- 课程块 -->
          <div
            v-for="(e, ei) in dayCourses.get(di)"
            :key="ei"
            class="tl-event"
            :class="{ highlighted: isHighlighted(e) }"
            :style="{ ...eventStyle(e, e.lane, e.totalLanes), ...courseColorStyle(e.course?.ID ?? 0) }"
            :title="`${e.course?.name} · ${e.classroom?.name} · ${e.teacher?.name}`"
            @click="openCourseDetail(e)"
            @mouseenter="hoveredCourseId = e.course?.ID ?? null"
            @mouseleave="hoveredCourseId = null"
          >
            <div class="tl-event-inner">
              <span class="tl-event-name">{{ eventInfo(e).name }}</span>
              <span class="tl-event-sub">{{ eventInfo(e).sub }}</span>
              <span v-if="e.span >= 2" class="tl-event-period">{{ eventInfo(e).periodText }}</span>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.timeline-view {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  gap: 2px;
  overflow-x: auto;
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: var(--b3-theme-on-surface-light);
  font-size: 14px;
}

/* 头部 */
.tl-header {
  display: flex;
  margin-bottom: 4px;
  flex-shrink: 0;
  min-width: 600px;
}

.tl-day-label {
  width: 48px;
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--b3-theme-on-surface);
  display: flex;
  align-items: center;
  justify-content: center;
}

.tl-hours {
  position: relative;
  flex: 1;
  height: 20px;
}

.tl-hour {
  position: absolute;
  transform: translateX(-50%);
  font-size: 10px;
  color: var(--b3-theme-on-surface-light);
  white-space: nowrap;
  z-index: 2;
}

.tl-break-tick {
  position: absolute;
  top: 14px;
  transform: translateX(-50%);
  z-index: 2;
}

.tl-break-tick-label {
  font-size: 9px;
  color: var(--b3-theme-on-surface-light);
  opacity: 0.7;
}

/* 行 */
.tl-row {
  display: flex;
  align-items: stretch;
  flex: 1;
  min-height: 0;
  min-width: 600px;
}

.tl-day-name {
  width: 48px;
  flex-shrink: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--b3-theme-on-surface);
  display: flex;
  align-items: center;
  padding: 0 4px;
}

.tl-track {
  flex: 1;
  position: relative;
  background: var(--b3-theme-background);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius-s);
  min-height: 0;
  overflow: hidden;
}

/* 分段背景 */
.tl-seg {
  position: absolute;
  top: 0;
  bottom: 0;
}

.tl-seg.class {
  background: transparent;
}

.tl-seg.break {
  background: var(--b3-theme-primary-lightest);
  opacity: 0.3;
  border-left: 1px dashed var(--b3-border-color);
  border-right: 1px dashed var(--b3-border-color);
}

.tl-break-label {
  position: absolute;
  top: 4px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 9px;
  color: var(--b3-theme-on-surface-light);
  white-space: nowrap;
  opacity: 0.6;
}

/* 网格线 */
.tl-gridline {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 1px;
  background: var(--b3-border-color);
  opacity: 0.6;
  pointer-events: none;
  z-index: 1;
}

.tl-break-line {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 1px;
  border-left: 1px dashed var(--b3-theme-primary);
  opacity: 0.4;
  pointer-events: none;
  z-index: 1;
}

/* 当前时间线 */
.tl-now-line {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--b3-theme-error, #e53935);
  z-index: 3;
  pointer-events: none;
}

.tl-now-dot {
  position: absolute;
  top: -4px;
  left: 50%;
  transform: translateX(-50%);
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--b3-theme-error, #e53935);
  box-shadow: 0 0 4px rgba(229, 57, 53, 0.4);
}

/* 课程块 */
.tl-event {
  position: absolute;
  border-radius: 3px;
  border-left: 3px solid;
  padding: 2px 5px;
  display: flex;
  align-items: center;
  font-size: 11px;
  overflow: hidden;
  white-space: nowrap;
  min-width: 0;
  z-index: 2;
  cursor: pointer;
  transition: filter 0.15s, transform 0.1s;
}

.tl-event:hover {
  filter: brightness(0.95);
  transform: translateY(-1px);
  z-index: 4;
}

.tl-event.highlighted {
  filter: brightness(0.92) saturate(1.2);
  z-index: 4;
  box-shadow: 0 0 0 2px rgba(0, 0, 0, 0.1);
}

.tl-event-inner {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
  line-height: 1.3;
}

.tl-event-name {
  font-weight: 600;
  color: var(--b3-theme-on-background);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
  text-shadow: 0 1px 1px rgba(0, 0, 0, 0.08);
}

.tl-event-sub {
  opacity: 0.6;
  color: var(--b3-theme-on-surface);
  font-size: 10px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.tl-event-period {
  font-size: 9px;
  opacity: 0.5;
  color: var(--b3-theme-on-surface-light);
}

/* 暗色主题增强 */
[data-theme="dark"] .tl-event-name {
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
}

[data-theme="dark"] .tl-seg.break {
  opacity: 0.15;
}

/* 窄屏提示 */
@media (max-width: 600px) {
  .timeline-view::before {
    content: "建议横屏或使用周视图查看课表";
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    background: var(--b3-theme-surface);
    padding: 12px 20px;
    border-radius: 8px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    font-size: 13px;
    color: var(--b3-theme-on-surface);
    z-index: 100;
    white-space: nowrap;
  }
}
</style>

