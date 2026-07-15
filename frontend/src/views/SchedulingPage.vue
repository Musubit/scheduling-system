<script setup lang="ts">
import { computed, ref, onUnmounted, watch } from 'vue'
import { useSchedulingStore } from '../stores/scheduling'
import { useAppStore } from '../stores/app'
import { useUiStore } from '../stores/ui'
import { useScheduleStore } from '../stores/schedule'
import { useResourceStore } from '../stores/resource'
import { NButton, NSelect, NCheckbox, NProgress, NSteps, NStep, NCollapse, NCollapseItem, NSlider, NInput, NModal, NSpace, useDialog, useMessage } from 'naive-ui'
import { DEPARTMENTS } from '../types'
import { fuzzyFilterFn } from '../utils/fuzzyFilter'

const store = useSchedulingStore()
const appStore = useAppStore()
const uiStore = useUiStore()
const scheduleStore = useScheduleStore()
const resourceStore = useResourceStore()
const dialog = useDialog()
const message = useMessage()

// 根因 → 资源管理跳转
function rootCauseAction(cause: string): { tab: 'classroom' | 'teacher'; label: string } | null {
  if (!cause) return null
  if (cause.includes('教室')) return { tab: 'classroom', label: '→ 去添加教室' }
  if (cause.includes('教师')) return { tab: 'teacher', label: '→ 去调整教师' }
  return null // 求解器兜底原因，无直达操作
}

function goToResource(tab: 'classroom' | 'teacher') {
  resourceStore.setActiveTab(tab)
  appStore.navigateTo('resource', tab === 'classroom' ? '教室管理' : '教师管理')
}

const scopeOptions = [
  { label: '全校所有院系', value: '全校所有院系' },
  ...DEPARTMENTS.map(d => ({ label: d.name, value: d.name })),
]

const scoreColor = computed(() => {
  const s = store.result?.score
  if (s == null) return 'var(--b3-theme-on-surface)'
  if (s >= 80) return 'var(--b3-theme-success)'
  if (s >= 60) return 'var(--b3-theme-warning)'
  return 'var(--b3-theme-error)'
})

// PerCategoryMax — single source: computed by Go ScoreSchedule, returned in scoreDetail
const categoryMax = computed(() => store.result?.scoreDetail?.perCategoryMax || 25)

// Get the actual max for a specific category (weighted max if available, else perCategoryMax)
function getCategoryMax(field: string): number {
  const maxes = store.result?.scoreDetail?.categoryMaxes
  if (maxes && maxes[field] != null && maxes[field] > 0) return maxes[field]
  return categoryMax.value
}

// Config panel collapse state
const configCollapsed = ref(false)

watch(() => store.isRunning, (val) => {
  if (val) configCollapsed.value = true
})

function toggleConfigPanel() {
  configCollapsed.value = !configCollapsed.value
}

const isConstraintEnabled = (key: string) => {
  if (store.config.mode === 'TIME_ONLY_SCHEDULING' && key === 'low_floor_preference') {
    return false
  }
  return store.config.constraints.includes(key)
}

// v0.5.5 P0 M3: TIME_ONLY 模式下不产生教室分配，教室冲突恒为 0。
// 冲突汇总/详情/硬约束验证均对此模式做感知，避免误报"教室冲突"。
const isTimeOnlyMode = computed(() => store.config.mode === 'TIME_ONLY_SCHEDULING')

const totalConflicts = computed(() => {
  if (!store.result) return 0
  const roomC = isTimeOnlyMode.value ? 0 : store.result.roomConflicts
  return store.result.teacherConflicts + roomC + store.result.classConflicts
})

// 排课结果决策辅助 — 帮用户决定：接受 / 修补 / 重跑 / 放弃
const scheduleStatus = computed(() => {
  if (store.isRunning) {
    return { type: 'info' as const, text: `正在排课...（${store.currentStage || '初始化'}）` }
  }
  const r = store.result
  if (!r) return null

  if (r.error) {
    return { type: 'error' as const, text: r.error }
  }

  const tasks = r.tasksScheduled ?? 0
  const total = r.totalCourses ?? 0
  const unplaced = r.unplacedTasks ?? []
  const conflicts = totalConflicts.value
  const hardPassed = conflicts === 0

  // 一行结论：事实陈述，不下判断
  let conclusion = ''
  if (total > 0 && tasks < total) {
    conclusion = `${total} 门课中 ${tasks} 门已排入，${total - tasks} 门未能排入。`
  } else {
    conclusion = `${tasks} 门课全部排入。`
  }
  if (hardPassed) {
    conclusion += '硬约束全部通过。'
  } else {
    conclusion += `存在 ${conflicts} 处硬约束冲突。`
  }

  return { type: hardPassed && tasks === total ? 'success' as const : 'warning' as const, conclusion, unplaced, conflicts }
})

// Watch for pending navigation after scheduling completes
const stopWatchNav = watch(() => uiStore.pendingScheduleNav, (val) => {
  if (val) {
    dialog.info({
      title: '排课完成',
      content: '排课已完成，是否跳转到课表查看结果？',
      positiveText: '查看课表',
      negativeText: '留在本页',
      onPositiveClick: () => {
        uiStore.clearScheduleNav()
        appStore.navigateTo('schedule', '周视图')
      },
      onNegativeClick: () => {
        uiStore.clearScheduleNav()
      },
      onClose: () => {
        uiStore.clearScheduleNav()
      },
    })
  }
})

const stopWatchError = watch(() => uiStore.pendingScheduleError, (msg) => {
  if (msg) {
    dialog.error({
      title: '排课失败',
      content: msg,
      positiveText: '知道了',
      onPositiveClick: () => {
        uiStore.clearScheduleError()
      },
      onClose: () => {
        uiStore.clearScheduleError()
      },
    })
  }
})

onUnmounted(() => {
  stopWatchNav()
  stopWatchError()
})

// Constraint weight labels for sliders
const weightLabels: Record<string, string> = {
  teacher_preference: '教师偏好时段',
  course_dispersed: '课程分散度',
  teacher_days_limit: '教师到校天数（按各自MaxDays）',
  low_floor_preference: '优先低楼层',
  pe_preferred_periods: '体育课时段',
  avoid_saturday: '避开周六',
  avoid_sunday: '避开周日',
}

// ===== H3: 保存快照 =====
// 手动调整过课表(scheduleStore.dirtyMoveCount > 0)或跑完排课(store.result 非空)时,
// 用户可以点"保存快照"生成一条命名的历史版本，方便后续对比。
const showSnapshotModal = ref(false)
const snapshotName = ref('')
const isSavingSnapshot = ref(false)

function openSnapshotModal() {
  const ts = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  snapshotName.value = `手动快照 ${ts.getMonth()+1}-${pad(ts.getDate())} ${pad(ts.getHours())}:${pad(ts.getMinutes())}`
  showSnapshotModal.value = true
}

async function saveManualSnapshot() {
  if (!appStore.currentSemesterId) {
    message.error('请先在学期管理选择一个当前学期')
    return
  }
  isSavingSnapshot.value = true
  try {
    const { CreateManualSnapshot, RenameSnapshot } =
      await import('../../bindings/scheduling-system/backend/services/snapshotservice')
    const snap = await CreateManualSnapshot(appStore.currentSemesterId)
    // 用户填的名称通过 RenameSnapshot 落地(后端 CreateManualSnapshot 只写 DefaultSnapshotName)
    if (snap?.ID && snapshotName.value.trim() && snapshotName.value.trim() !== snap.name) {
      try { await RenameSnapshot(snap.ID, snapshotName.value.trim()) } catch { /* 命名失败不阻塞 */ }
    }
    scheduleStore.clearDirty()
    showSnapshotModal.value = false
    message.success(`快照已保存：${snapshotName.value.trim() || snap?.name || '(默认名)'}`)
  } catch (e: any) {
    message.error('保存快照失败：' + (e?.message || e))
  } finally {
    isSavingSnapshot.value = false
  }
}
</script>

<template>
  <div class="scheduling-page">
    <!-- Quick guide -->
    <div class="quick-guide" v-if="!store.isRunning && store.progress === 0">
      💡 <strong>操作流程：</strong>选择范围 → 选择约束预设 → 点「开始自动排课」→ 查看课表结果
    </div>

    <div class="scheduling-layout" :class="{ collapsed: configCollapsed }">
      <!-- 左侧：配置面板 -->
      <div class="config-panel" :class="{ collapsed: configCollapsed }">
        <div class="config-header">
          <h3 class="panel-title">排课参数配置</h3>
          <button class="collapse-btn" @click="toggleConfigPanel" title="收起配置面板">◀</button>
        </div>

        <!-- 学期选择 -->
        <div class="form-group">
          <label class="form-label">排课学期</label>
          <n-select
            :value="appStore.currentSemesterId"
            @update:value="appStore.setCurrentSemester($event)"
            :options="appStore.semesterSelectOptions"
            size="small"
            :consistent-menu-width="true"
          />
          <span class="form-hint">选择要排课的学期，教学任务按学期隔离</span>
        </div>

        <div class="form-group">
          <label class="form-label">排课范围</label>
          <n-select v-model:value="store.config.scope" :options="scopeOptions" filterable :filter="fuzzyFilterFn" size="small" />
        </div>

        <div class="form-group">
          <label class="form-label">排课模式</label>
          <n-checkbox
            :checked="store.config.mode === 'TIME_ONLY_SCHEDULING'"
            @update:checked="(checked: boolean) => { store.config.mode = checked ? 'TIME_ONLY_SCHEDULING' : 'FULL_SCHEDULING' }"
          >
            关闭教室场地分配（仅排上课时间）
          </n-checkbox>
          <span class="form-hint">启用后将忽略教室容量/占用分配，仅计算课程时间安排</span>
        </div>

        <!-- 约束预设 -->
        <div class="form-group">
          <label class="form-label">约束方案</label>
          <n-select
            v-model:value="store.activePreset"
            :options="store.CONSTRAINT_PRESETS.map(p => ({ label: p.label, value: p.name }))"
            size="small"
            @update:value="store.applyPreset"
          />
          <span class="form-hint">切换方案自动调整约束权重</span>
        </div>

        <!-- 约束开关 -->
        <div class="form-group">
          <label class="form-label">约束条件</label>
          <div class="checkbox-group">
            <n-checkbox
              v-for="opt in store.constraintOptions"
              :key="opt.key"
              :checked="store.config.constraints.includes(opt.key)"
              :disabled="isTimeOnlyMode && opt.key === 'low_floor_preference'"
              size="small"
              @update:checked="store.toggleConstraint(opt.key)"
            >
              {{ opt.label }}
              <span v-if="isTimeOnlyMode && opt.key === 'low_floor_preference'" class="constraint-disabled-hint">
                (TIME_ONLY 模式不评估)
              </span>
            </n-checkbox>
          </div>
        </div>

        <!-- 约束权重调整 -->
        <n-collapse class="advanced-collapse">
          <n-collapse-item title="约束权重调整（高级）" name="advanced">
            <!-- 约束权重滑块 -->
            <div class="form-group" v-for="opt in store.constraintOptions" :key="'w-'+opt.key">
              <label class="form-label" style="font-size:12px">{{ weightLabels[opt.key] || opt.label }}</label>
              <n-slider
                v-model:value="store.constraintWeights[opt.key]"
                :min="0"
                :max="100"
                :step="5"
                :disabled="!store.config.constraints.includes(opt.key)"
              />
            </div>

            <!-- 求解引擎 -->
            <div class="form-group">
              <label class="form-label">求解引擎</label>
              <n-select
                v-model:value="store.engine"
                :options="store.engineOptions"
                size="small"
              />
              <span class="form-hint">智能模式自动选择最优引擎，OR-Tools不可用时自动降级</span>
            </div>
          </n-collapse-item>
        </n-collapse>

        <n-button
          type="primary"
          block
          size="large"
          :loading="store.isRunning"
          @click="store.startScheduling()"
          style="margin-top: 16px"
        >
          {{ store.isRunning ? '排课中...' : '开始自动排课' }}
        </n-button>
      </div>

      <!-- 右侧：结果面板 -->
      <div class="result-panel">
        <div class="result-header">
          <h3 class="panel-title">排课进度与结果</h3>
          <div class="mode-badge" :class="isTimeOnlyMode ? 'mode-time-only' : 'mode-full'" v-if="store.result">
            {{ isTimeOnlyMode ? '⏱ 仅时间模式' : '🏫 完整模式（时间+教室）' }}
          </div>
          <n-button
            size="tiny"
            v-if="store.result || scheduleStore.dirtyMoveCount > 0"
            @click="openSnapshotModal"
            class="save-snapshot-btn"
          >
            📸 保存快照
            <span v-if="scheduleStore.dirtyMoveCount > 0" class="dirty-badge">
              +{{ scheduleStore.dirtyMoveCount }}
            </span>
          </n-button>
          <button v-if="configCollapsed" class="expand-btn" @click="toggleConfigPanel">▶ 展开配置</button>
        </div>

        <n-steps :current="store.progressHistory.length" size="small" style="margin-bottom: 16px;">
          <n-step
            v-for="(p, idx) in store.progressHistory"
            :key="idx"
            :title="p.stage"
            :description="p.progress + '%'"
          />
          <n-step v-if="store.progressHistory.length === 0" title="就绪" description="等待开始" />
        </n-steps>

        <div class="progress-section">
          <div class="progress-label">
            <span class="stage-badge" v-if="store.currentStage">{{ store.currentStage }}</span>
            {{ store.progressText }}
          </div>
          <n-progress
            :percentage="store.progress"
            :indicator-placement="'inside'"
            :height="24"
            :border-radius="4"
          />
        </div>

        <div class="stats-row">
          <div class="stat-card">
            <div class="stat-value">
              <template v-if="store.result">{{ store.result.tasksScheduled }}<span class="stat-sub"> / {{ store.result.totalCourses }}</span></template>
              <template v-else>-</template>
            </div>
            <div class="stat-label">已排任务 / 总任务</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">{{ store.result?.scheduled || '-' }}</div>
            <div class="stat-label">排课条目总数</div>
          </div>
          <div class="stat-card">
            <div class="stat-value" :style="{ color: totalConflicts > 0 ? 'var(--b3-theme-error)' : 'var(--b3-theme-success)' }">
              {{ store.result ? totalConflicts : '-' }}
            </div>
            <div class="stat-label">待处理冲突</div>
            <div class="stat-detail" v-if="store.result && totalConflicts > 0">
              <span>教师 {{ store.result.teacherConflicts }}</span>
              <span v-if="!isTimeOnlyMode">教室 {{ store.result.roomConflicts }}</span>
              <span>班级 {{ store.result.classConflicts }}</span>
            </div>
          </div>
          <div class="stat-card">
            <div class="stat-value" :style="{ color: scoreColor }">{{ store.result?.score != null ? store.result.score.toFixed(1) + '分' : '-' }}</div>
            <div class="stat-label">综合评分</div>
          </div>
        </div>

        <!-- 排课决策辅助 -->
        <div v-if="scheduleStatus" class="decision-support" :class="'ds-' + scheduleStatus.type">
          <div class="ds-conclusion">{{ scheduleStatus.conclusion }}</div>
          <div v-if="scheduleStatus.unplaced && scheduleStatus.unplaced.length > 0" class="ds-problems">
            <div v-for="u in scheduleStatus.unplaced" :key="u.taskId" class="ds-problem">
              <span class="ds-course">{{ u.courseName }}</span>
              <span class="ds-teacher">{{ u.teacherName }}</span>
              <span v-if="u.className" class="ds-class">{{ u.className }}</span>
              <span class="ds-cause">
                {{ u.rootCause }}
                <n-button
                  v-if="rootCauseAction(u.rootCause)"
                  size="tiny"
                  text
                  type="primary"
                  @click="goToResource(rootCauseAction(u.rootCause)!.tab)"
                  style="margin-left: 8px;"
                >{{ rootCauseAction(u.rootCause)!.label }}</n-button>
              </span>
            </div>
            <div class="ds-hint">修改数据后回到此页面重新排课即可。</div>
          </div>
        </div>

        <!-- 评分明细 -->
        <div class="score-breakdown" v-if="store.result?.scoreDetail">
          <h4 class="breakdown-title">评分明细（软约束）</h4>
          <div class="breakdown-items">
            <div class="breakdown-item" v-if="isConstraintEnabled('teacher_preference')">
              <span class="breakdown-label">教师偏好满足度</span>
              <n-progress :percentage="Math.round(store.result.scoreDetail.teacherPref / getCategoryMax('teacherPref') * 1000) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ store.result.scoreDetail.teacherPref.toFixed(2) }}/{{ getCategoryMax('teacherPref') }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('course_dispersed')">
              <span class="breakdown-label">课程间隔均匀度</span>
              <n-progress :percentage="Math.round(store.result.scoreDetail.courseSpacing / getCategoryMax('courseSpacing') * 1000) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ store.result.scoreDetail.courseSpacing.toFixed(2) }}/{{ getCategoryMax('courseSpacing') }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('teacher_days_limit')">
              <span class="breakdown-label">教师到校天数</span>
              <n-progress :percentage="Math.round(store.result.scoreDetail.teacherDays / getCategoryMax('teacherDays') * 1000) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ store.result.scoreDetail.teacherDays.toFixed(2) }}/{{ getCategoryMax('teacherDays') }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('low_floor_preference')">
              <span class="breakdown-label">优先低楼层</span>
              <n-progress :percentage="Math.round(store.result.scoreDetail.lowFloorPref / getCategoryMax('lowFloorPref') * 1000) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ store.result.scoreDetail.lowFloorPref.toFixed(2) }}/{{ getCategoryMax('lowFloorPref') }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('avoid_saturday') || isConstraintEnabled('avoid_sunday')">
              <span class="breakdown-label">周末避让</span>
              <n-progress :percentage="Math.round(((store.result.scoreDetail.weekendAvoid || 0) / getCategoryMax('weekendAvoid') * 1000)) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ (store.result.scoreDetail.weekendAvoid || 0).toFixed(2) }}/{{ getCategoryMax('weekendAvoid') }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('pe_preferred_periods')">
              <span class="breakdown-label">体育课时段</span>
              <n-progress :percentage="Math.round(((store.result.scoreDetail.pePeriodPref || 0) / getCategoryMax('pePeriodPref') * 1000)) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ (store.result.scoreDetail.pePeriodPref || 0).toFixed(2) }}/{{ getCategoryMax('pePeriodPref') }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('student_fatigue')">
              <span class="breakdown-label">学生疲劳度</span>
              <n-progress :percentage="Math.round(((store.result.scoreDetail.studentFatigue || 0) / getCategoryMax('studentFatigue') * 1000)) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ (store.result.scoreDetail.studentFatigue || 0).toFixed(2) }}/{{ getCategoryMax('studentFatigue') }}</span>
            </div>
          </div>
        </div>

        <!-- 硬约束验证 -->
        <div class="score-breakdown" v-if="store.result">
          <h4 class="breakdown-title">硬约束验证</h4>
          <div class="verify-items">
            <div class="verify-item">
              <span class="verify-icon" :style="{ color: store.result.conflicts === 0 ? 'var(--b3-theme-success)' : 'var(--b3-theme-error)' }">
                {{ store.result.conflicts === 0 ? '✅' : '❌' }}
              </span>
              <span class="verify-label">教师时间冲突</span>
              <span class="verify-result">{{ store.result.conflicts === 0 ? '通过' : '发现冲突' }}</span>
            </div>
            <div class="verify-item" v-if="!isTimeOnlyMode">
              <span class="verify-icon" :style="{ color: store.result.conflicts === 0 ? 'var(--b3-theme-success)' : 'var(--b3-theme-error)' }">
                {{ store.result.conflicts === 0 ? '✅' : '❌' }}
              </span>
              <span class="verify-label">教室占用冲突</span>
              <span class="verify-result">{{ store.result.conflicts === 0 ? '通过' : '发现冲突' }}</span>
            </div>
            <div class="verify-item">
              <span class="verify-icon" :style="{ color: store.result.conflicts === 0 ? 'var(--b3-theme-success)' : 'var(--b3-theme-error)' }">
                {{ store.result.conflicts === 0 ? '✅' : '❌' }}
              </span>
              <span class="verify-label">班级时间冲突</span>
              <span class="verify-result">{{ store.result.conflicts === 0 ? '通过' : '发现冲突' }}</span>
            </div>
            <div class="verify-item">
              <span class="verify-icon" style="color: var(--b3-theme-success)">✅</span>
              <span class="verify-label">锁定时间段规避</span>
              <span class="verify-result">已规避</span>
            </div>
          </div>
        </div>

        <!-- 日志 -->
        <div class="log-section" v-if="store.logs.length > 0">
          <h4 class="log-title">排课日志</h4>
          <div class="log-content">
            <div v-for="(log, i) in store.logs" :key="i" class="log-line">{{ log }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- H3: 保存快照弹窗 -->
    <n-modal v-model:show="showSnapshotModal" preset="card" title="保存快照" style="width: 420px;">
      <div class="setting-desc" style="margin-bottom:12px">
        <span v-if="scheduleStore.dirtyMoveCount > 0">
          自上次快照后有 {{ scheduleStore.dirtyMoveCount }} 处手动调整。
        </span>
        <span v-else>
          将把当前学期的排课结果保存为一条快照，方便后续在"历史对比"中查看。
        </span>
      </div>
      <n-input
        v-model:value="snapshotName"
        placeholder="快照名称（可留空使用默认）"
        maxlength="80"
        show-count
      />
      <template #footer>
        <n-space justify="end">
          <n-button @click="showSnapshotModal = false" :disabled="isSavingSnapshot">取消</n-button>
          <n-button type="primary" :loading="isSavingSnapshot" @click="saveManualSnapshot">保存</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.scheduling-page { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.quick-guide { font-size: 13px; color: var(--b3-theme-on-surface); background: var(--b3-theme-primary-lightest); padding: 10px 16px; border-radius: var(--b3-border-radius-s); margin-bottom: 16px; border-left: 3px solid var(--b3-theme-primary); }

.scheduling-layout {
  flex: 1;
  display: flex;
  gap: 20px;
  min-height: 0;
  transition: gap 0.25s ease;
}

.scheduling-layout.collapsed {
  gap: 0;
}

.panel-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin-bottom: 16px;
}

.config-header,
.result-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.config-header .panel-title,
.result-header .panel-title {
  margin-bottom: 0;
}

/* v0.5.5 P0 M3: mode badge in result header */
.mode-badge {
  font-size: 12px;
  padding: 3px 10px;
  border-radius: 12px;
  font-weight: 500;
  margin-left: auto;
  margin-right: 12px;
}
.mode-badge.mode-full {
  background: var(--b3-theme-primary-lightest);
  color: var(--b3-theme-primary);
  border: 1px solid var(--b3-theme-primary-light);
}
.mode-badge.mode-time-only {
  background: var(--b3-theme-warning-lightest, #fff4e6);
  color: var(--b3-theme-warning, #d97706);
  border: 1px solid var(--b3-theme-warning-light, #fbbf24);
}
.constraint-disabled-hint {
  font-size: 11px;
  color: var(--b3-theme-on-surface-light);
  margin-left: 4px;
}

/* v0.5.5 H3: 保存快照按钮 + dirty badge */
.save-snapshot-btn {
  margin-right: 8px;
}
.dirty-badge {
  background: var(--b3-theme-error);
  color: white;
  border-radius: 8px;
  padding: 1px 6px;
  font-size: 10px;
  margin-left: 4px;
  font-weight: 600;
}
.setting-desc {
  font-size: 12px;
  color: var(--b3-theme-on-surface-light);
}

.collapse-btn {
  background: transparent;
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius-s);
  cursor: pointer;
  color: var(--b3-theme-on-surface);
  padding: 2px 8px;
  font-size: 12px;
  line-height: 1.4;
  transition: background 0.15s, color 0.15s;
}

.collapse-btn:hover {
  background: var(--b3-theme-primary-lightest);
  color: var(--b3-theme-primary);
}

.expand-btn {
  background: transparent;
  border: 1px solid var(--b3-theme-primary);
  border-radius: var(--b3-border-radius-s);
  cursor: pointer;
  color: var(--b3-theme-primary);
  padding: 4px 12px;
  font-size: 12px;
  font-weight: 500;
  transition: background 0.15s;
}

.expand-btn:hover {
  background: var(--b3-theme-primary-lightest);
}

.config-panel {
  width: 320px;
  min-width: 0;
  flex-shrink: 0;
  background: var(--b3-theme-surface);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius);
  padding: 20px;
  overflow-y: auto;
  transition: width 0.25s ease, opacity 0.2s ease, padding 0.2s ease, border 0.2s ease;
}

.config-panel.collapsed {
  width: 0;
  padding: 0;
  opacity: 0;
  overflow: hidden;
  border: none;
  pointer-events: none;
}

.result-panel {
  flex: 1;
  min-width: 0;
  background: var(--b3-theme-surface);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius);
  padding: 20px;
  overflow-y: auto;
}

.form-group {
  margin-bottom: 14px;
}

.form-label {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: var(--b3-theme-on-surface);
  margin-bottom: 6px;
}

.form-hint {
  display: block;
  font-size: 11px;
  color: var(--b3-theme-on-surface-light);
  margin-top: 4px;
}

.semester-badge {
  display: inline-block;
  background: var(--b3-theme-primary-lightest);
  color: var(--b3-theme-primary);
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 13px;
  font-weight: 500;
}

.checkbox-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.advanced-collapse {
  margin-top: 16px;
}

.progress-section {
  margin-bottom: 20px;
}

.progress-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--b3-theme-primary);
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.stage-badge {
  font-size: 12px;
  font-weight: 700;
  padding: 2px 10px;
  border-radius: 4px;
  background: var(--b3-theme-primary);
  color: #fff;
}

.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 20px;
}

.stat-card {
  background: var(--b3-theme-background);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius-s);
  padding: 12px;
  text-align: center;
}

.stat-value {
  font-size: 22px;
  font-weight: 700;
  color: var(--b3-theme-primary);
}

.stat-label {
  font-size: 11px;
  color: var(--b3-theme-on-surface-light);
  margin-top: 4px;
}

.stat-sub {
  font-size: 14px;
  font-weight: 400;
  opacity: 0.5;
}

.stat-detail {
  display: flex;
  gap: 8px;
  margin-top: 4px;
  font-size: 11px;
  color: var(--b3-theme-on-surface-light);
}

.stat-detail span {
  white-space: nowrap;
}

.decision-support {
  padding: 12px 16px;
  border-radius: var(--b3-border-radius);
  margin-bottom: 16px;
  border: 1px solid;
}
.ds-success { background: var(--b3-theme-success-lightest, #e8f5e9); border-color: var(--b3-theme-success-light, #a5d6a7); }
.ds-warning { background: var(--b3-theme-warning-lightest, #fff8e1); border-color: var(--b3-theme-warning-light, #ffe082); }
.ds-error   { background: var(--b3-theme-error-lightest, #fce4ec);   border-color: var(--b3-theme-error-light, #ef9a9a); }
.ds-info    { background: var(--b3-theme-primary-lightest, #e3f2fd); border-color: var(--b3-theme-primary-light, #90caf9); }

.ds-conclusion {
  font-size: 14px;
  font-weight: 500;
  color: var(--b3-theme-on-background);
  margin-bottom: 8px;
}

.ds-problems {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.ds-problem {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 13px;
  color: var(--b3-theme-on-surface);
  padding: 6px 10px;
  background: var(--b3-theme-surface);
  border-radius: 4px;
}

.ds-course {
  font-weight: 600;
  color: var(--b3-theme-on-background);
  min-width: 80px;
}

.ds-teacher {
  color: var(--b3-theme-on-surface);
  min-width: 50px;
}

.ds-class {
  color: var(--b3-theme-on-surface-light);
  font-size: 12px;
}

.ds-cause {
  color: var(--b3-theme-error);
  font-size: 12px;
  margin-left: auto;
}

.ds-hint {
  font-size: 12px;
  color: var(--b3-theme-on-surface-light);
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed var(--b3-border-color);
}

.log-section {
  margin-top: 16px;
}

.log-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin-bottom: 8px;
}

.log-content {
  background: var(--b3-theme-background);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius-s);
  padding: 10px;
  max-height: 200px;
  overflow-y: auto;
  font-family: monospace;
  font-size: 12px;
  line-height: 1.8;
}

.log-line {
  color: var(--b3-theme-on-surface);
}

.score-breakdown {
  margin-top: 12px;
  padding: 12px;
  background: var(--b3-theme-background);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius-s);
}

.breakdown-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin-bottom: 10px;
}

.breakdown-items {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.breakdown-item {
  display: grid;
  grid-template-columns: 120px 1fr 56px;
  align-items: center;
  gap: 8px;
}

.breakdown-label {
  font-size: 11px;
  color: var(--b3-theme-on-surface);
}

.breakdown-value {
  font-size: 11px;
  font-weight: 600;
  color: var(--b3-theme-primary);
  text-align: right;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.verify-items {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.verify-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.verify-icon {
  width: 20px;
  text-align: center;
}

.verify-label {
  flex: 1;
  color: var(--b3-theme-on-surface);
}

.verify-result {
  font-weight: 500;
  color: var(--b3-theme-on-background);
}
</style>
