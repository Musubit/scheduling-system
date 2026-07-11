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

// =============================================================================
// 教学时间轴视觉权重系统
// 每节课 = 基本单元(1.0)，各类间隙按相对视觉权重压缩
// =============================================================================

const PERIOD_WEIGHT      = 1.00 // 课时基本单元
const GAP_SHORT_WEIGHT   = 0.10 // ≤10 分钟短课间
const GAP_LONG_WEIGHT    = 0.30 // >10, ≤30 分钟长课间
const BREAK_LUNCH_WEIGHT = 0.60 // 午休
const BREAK_DINNER_WEIGHT = 0.40 // 晚饭

const GAP_SHORT_MAX = 10  // 短课间上限（分钟）
const GAP_LONG_MAX  = 30  // 长课间上限（分钟）

interface Seg { id: string; type: 'class' | 'break' | 'gap'; absStart: number; absEnd: number; lo: number; hi: number; label?: string }

function buildSegments(): Seg[] {
  interface FlatItem { absStart: number; absEnd: number; type: Seg['type']; weight: number; label?: string }
  
  const items: FlatItem[] = []

  for (let i = 0; i < PERIODS.length; i++) {
    const p = P_INFOS[i]
    // 课时
    items.push({ absStart: p.startAbs, absEnd: p.endAbs, type: 'class', weight: PERIOD_WEIGHT })

    if (i >= PERIODS.length - 1) break

    const gapStart = p.endAbs
    const gapEnd   = P_INFOS[i + 1].startAbs
    const dur      = gapEnd - gapStart

    if (dur <= GAP_SHORT_MAX) {
      items.push({ absStart: gapStart, absEnd: gapEnd, type: 'gap', weight: GAP_SHORT_WEIGHT })
    } else if (dur <= GAP_LONG_MAX) {
      items.push({ absStart: gapStart, absEnd: gapEnd, type: 'gap', weight: GAP_LONG_WEIGHT })
    } else if (gapStart < 14 * 60) {
      // 午休（14:00 之前的大间隙）
      items.push({ absStart: gapStart, absEnd: gapEnd, type: 'break', weight: BREAK_LUNCH_WEIGHT, label: '午休' })
    } else {
      // 晚饭
      items.push({ absStart: gapStart, absEnd: gapEnd, type: 'break', weight: BREAK_DINNER_WEIGHT, label: '晚饭' })
    }
  }

  const totalWeight = items.reduce((sum, it) => sum + it.weight, 0)
  const segments: Seg[] = []
  let cursor = 0
  let classIdx = 0, breakIdx = 0, gapIdx = 0

  for (const it of items) {
    const w = (it.weight / totalWeight) * 100
    const id = it.type === 'class' ? `c${classIdx++}` : it.type === 'break' ? `b${breakIdx++}` : `g${gapIdx++}`
    segments.push({
      id, type: it.type,
      absStart: it.absStart, absEnd: it.absEnd,
      lo: cursor, hi: cursor + w,
      label: it.label,
    })
    cursor += w
  }

  return segments
}

const SEGMENTS: Seg[] = buildSegments()

// Header 只渲染课程段背景（不渲染 break 段，避免色条侵入节次标签区）
const CLASS_SEGMENTS = SEGMENTS.filter(s => s.type === 'class')

// 真实时钟分钟 -> 轨道百分比（缓存，所有 segment 统一线性插值）
const ABS_TO_PCT_CACHE = new Map<number, number>()

function absToPct(absMin: number): number {
  const cached = ABS_TO_PCT_CACHE.get(absMin)
  if (cached !== undefined) return cached

  for (const s of SEGMENTS) {
    if (absMin >= s.absStart && absMin <= s.absEnd) {
      const result = s.lo + ((absMin - s.absStart) / (s.absEnd - s.absStart || 1)) * (s.hi - s.lo)
      ABS_TO_PCT_CACHE.set(absMin, result)
      return result
    }
  }
  const result = absMin <= SEGMENTS[0].absStart ? SEGMENTS[0].lo : SEGMENTS[SEGMENTS.length - 1].hi
  ABS_TO_PCT_CACHE.set(absMin, result)
  return result
}

// 节次起始刻度（track 网格线也用）
const AXIS_TICKS = P_INFOS.map((p, i) => ({
  label: PERIODS[i].time.split('\n')[0],
  pct: absToPct(p.startAbs),
  idx: i + 1,
}))

// 统一时间轴刻度：节次起点 + 午休/晚饭标记 + 最后节次结束时间
const HEADER_TICKS = [
  ...AXIS_TICKS.map(t => ({ label: t.label, pct: t.pct })),
  { label: '11:50', pct: absToPct(11 * 60 + 50) },
  { label: '17:30', pct: absToPct(17 * 60 + 30) },
  { label: PERIODS[PERIODS.length - 1].time.split('\n')[1], pct: absToPct(P_INFOS[P_INFOS.length - 1].endAbs) },
].sort((a, b) => a.pct - b.pct)

// Track 休息带边界虚线（从 break 类型 segment 的视觉边界推导）
const BREAK_LINES = SEGMENTS
  .filter(s => s.type === 'break')
  .flatMap(s => [s.lo, s.hi])

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
            v-for="seg in CLASS_SEGMENTS"
            :key="seg.id"
            class="tl-seg"
            :class="seg.type"
            :style="{ left: seg.lo + '%', width: (seg.hi - seg.lo) + '%' }"
          ></div>
          <div v-for="(t, ti) in HEADER_TICKS" :key="ti" class="tl-hour" :style="{ left: t.pct + '%' }">{{ t.label }}</div>
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
          <div v-for="(bp, bi) in BREAK_LINES" :key="'btl-' + bi" class="tl-break-line" :style="{ left: bp + '%' }"></div>
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

/* 行 */
.tl-row {
  display: flex;
  align-items: stretch;
  flex: 1;
  min-height: 0;
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
  box-sizing: border-box;
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

