<script setup lang="ts">
import { computed, ref, onMounted, watch } from 'vue'
import { useSchedulingStore } from '../stores/scheduling'
import { useAppStore } from '../stores/app'
import { useUiStore } from '../stores/ui'
import { NButton, NSelect, NCheckbox, NProgress, NTag, NSteps, NStep, NCollapse, NCollapseItem, NSlider, useDialog } from 'naive-ui'
import { DEPARTMENTS } from '../types'
import { fuzzyFilter } from '../utils/fuzzyFilter'

const fuzzyFilterFn = fuzzyFilter as any

const store = useSchedulingStore()
const appStore = useAppStore()
const uiStore = useUiStore()
const dialog = useDialog()
const showAdvanced = ref(false)

// Step indicator
const currentStep = computed(() => {
  if (!store.isRunning && store.progress === 0) return 0
  if (store.progress < 30) return 1
  if (store.progress < 70) return 2
  if (store.progress < 100) return 3
  return 4
})

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

// Per-category max score
const categoryMax = computed(() => {
  const count = store.config.constraints.length || 4
  return Math.round(100 / count * 100) / 100
})

const isConstraintEnabled = (key: string) => store.config.constraints.includes(key)

const totalConflicts = computed(() => {
  if (!store.result) return 0
  return store.result.teacherConflicts + store.result.roomConflicts + store.result.classConflicts
})

// Watch for pending navigation after scheduling completes
onMounted(() => {
  watch(() => uiStore.pendingScheduleNav, (val) => {
    if (val) {
      dialog.info({
        title: '排课完成',
        content: '排课已完成，是否跳转到课表查看结果？',
        positiveText: '查看课表',
        negativeText: '留在本页',
        onPositiveClick: () => {
          uiStore.clearScheduleNav()
          appStore.navigateTo('schedule', '周视图课表')
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

  watch(() => uiStore.pendingScheduleError, (msg) => {
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
</script>

<template>
  <div class="scheduling-page">
    <!-- Quick guide -->
    <div class="quick-guide" v-if="!store.isRunning && store.progress === 0">
      💡 <strong>操作流程：</strong>选择范围 → 选择约束预设 → 点「开始自动排课」→ 查看课表结果
    </div>

    <div class="scheduling-layout">
      <!-- 左侧：配置面板 -->
      <div class="config-panel">
        <h3 class="panel-title">排课参数配置</h3>

        <!-- 学期选择 -->
        <div class="form-group">
          <label class="form-label">排课学期</label>
          <n-select
            v-model:value="store.selectedSemesterId"
            :options="store.semesters.map(s => ({
              label: s.name + (s.isActive ? '（当前）' : ''),
              value: s.ID
            }))"
            size="small"
            :consistent-menu-width="true"
          />
          <span class="form-hint">选择要排课的学期，教学任务按学期隔离</span>
        </div>

        <div class="form-group">
          <label class="form-label">排课范围</label>
          <n-select v-model:value="store.config.scope" :options="scopeOptions" filterable :filter="fuzzyFilterFn" size="small" />
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
              size="small"
              @update:checked="store.toggleConstraint(opt.key)"
            >
              {{ opt.label }}
            </n-checkbox>
          </div>
        </div>

        <!-- 高级设置 -->
        <n-collapse class="advanced-collapse">
          <n-collapse-item title="高级设置" name="advanced">
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
        <h3 class="panel-title">排课进度与结果</h3>

        <n-steps :current="currentStep" size="small" style="margin-bottom: 16px;">
          <n-step title="准备" description="加载资源" />
          <n-step title="清空" description="清除旧课表" />
          <n-step title="排课" description="算法分配" />
          <n-step title="检测" description="冲突扫描" />
          <n-step title="完成" description="生成课表" />
        </n-steps>

        <div class="progress-section">
          <div class="progress-label">{{ store.progressText }}</div>
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
              <span>教室 {{ store.result.roomConflicts }}</span>
              <span>班级 {{ store.result.classConflicts }}</span>
            </div>
          </div>
          <div class="stat-card">
            <div class="stat-value" :style="{ color: scoreColor }">{{ store.result?.score != null ? store.result.score.toFixed(1) + '分' : '-' }}</div>
            <div class="stat-label">综合评分</div>
          </div>
        </div>

        <!-- 评分明细 -->
        <div class="score-breakdown" v-if="store.result?.scoreDetail">
          <h4 class="breakdown-title">评分明细（软约束）</h4>
          <div class="breakdown-items">
            <div class="breakdown-item" v-if="isConstraintEnabled('teacher_preference')">
              <span class="breakdown-label">教师偏好满足度</span>
              <n-progress :percentage="Math.round(store.result.scoreDetail.teacherPref / categoryMax * 1000) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ store.result.scoreDetail.teacherPref.toFixed(1) }}/{{ categoryMax }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('course_dispersed')">
              <span class="breakdown-label">课程间隔均匀度</span>
              <n-progress :percentage="Math.round(store.result.scoreDetail.courseSpacing / categoryMax * 1000) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ store.result.scoreDetail.courseSpacing.toFixed(1) }}/{{ categoryMax }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('teacher_days_limit')">
              <span class="breakdown-label">教师到校天数</span>
              <n-progress :percentage="Math.round(store.result.scoreDetail.teacherDays / categoryMax * 1000) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ store.result.scoreDetail.teacherDays.toFixed(1) }}/{{ categoryMax }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('low_floor_preference')">
              <span class="breakdown-label">优先低楼层</span>
              <n-progress :percentage="Math.round(store.result.scoreDetail.lowFloorPref / categoryMax * 1000) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ store.result.scoreDetail.lowFloorPref.toFixed(1) }}/{{ categoryMax }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('avoid_saturday') || isConstraintEnabled('avoid_sunday')">
              <span class="breakdown-label">周末避让</span>
              <n-progress :percentage="Math.round(((store.result.scoreDetail.weekendAvoid || 0) / categoryMax * 1000)) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ (store.result.scoreDetail.weekendAvoid || 0).toFixed(1) }}/{{ categoryMax }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('pe_preferred_periods')">
              <span class="breakdown-label">体育课时段</span>
              <n-progress :percentage="Math.round(((store.result.scoreDetail.pePeriodPref || 0) / categoryMax * 1000)) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ (store.result.scoreDetail.pePeriodPref || 0).toFixed(1) }}/{{ categoryMax }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('student_fatigue')">
              <span class="breakdown-label">学生疲劳度</span>
              <n-progress :percentage="Math.round(((store.result.scoreDetail.studentFatigue || 0) / categoryMax * 1000)) / 10" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ (store.result.scoreDetail.studentFatigue || 0).toFixed(1) }}/{{ categoryMax }}</span>
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
            <div class="verify-item">
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
  </div>
</template>

<style scoped>
.scheduling-page { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.quick-guide { font-size: 13px; color: var(--b3-theme-on-surface); background: var(--b3-theme-primary-lightest); padding: 10px 16px; border-radius: var(--b3-border-radius-s); margin-bottom: 16px; border-left: 3px solid var(--b3-theme-primary); }

.scheduling-layout {
  flex: 1;
  display: grid;
  grid-template-columns: 320px 1fr;
  gap: 20px;
  min-height: 0;
}

.panel-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin-bottom: 16px;
}

.config-panel {
  background: var(--b3-theme-surface);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius);
  padding: 20px;
  overflow-y: auto;
}

.result-panel {
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
  grid-template-columns: 120px 1fr 50px;
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
