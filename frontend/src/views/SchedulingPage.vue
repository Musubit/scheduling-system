<script setup lang="ts">
import { computed } from 'vue'
import { useSchedulingStore } from '../stores/scheduling'
import { NButton, NSelect, NInputNumber, NCheckbox, NProgress, NTag, NSteps, NStep } from 'naive-ui'
import { DEPARTMENTS } from '../types'

const store = useSchedulingStore()

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

const semesterOptions = [
  { label: '2025-2026 第二学期', value: '2025-2026 第二学期' },
]

const scoreColor = computed(() => {
  const s = store.result?.score
  if (s == null) return 'var(--b3-theme-on-surface)'
  if (s >= 80) return 'var(--b3-theme-success)'
  if (s >= 60) return 'var(--b3-theme-warning)'
  return 'var(--b3-theme-error)'
})

// Per-category max score depends on how many constraints are enabled
const categoryMax = computed(() => {
  const count = store.config.constraints.length || 4
  return Math.round(100 / count * 100) / 100
})

// Whether a specific constraint category is enabled
const isConstraintEnabled = (key: string) => store.config.constraints.includes(key)
</script>

<template>
  <div class="scheduling-page">
    <!-- Quick guide -->
    <div class="quick-guide" v-if="!store.isRunning && store.progress === 0">
      💡 <strong>操作流程：</strong>左侧配参数 → 点「开始自动排课」→ 等算法完成 → 自动跳转课表查看 → 如有冲突去「冲突检测」处理
    </div>

    <div class="scheduling-layout">
      <!-- 左侧：配置面板 -->
      <div class="config-panel">
        <h3 class="panel-title">排课参数配置</h3>

        <div class="form-group">
          <label class="form-label">排课范围</label>
          <n-select v-model:value="store.config.scope" :options="scopeOptions" size="small" />
        </div>

        <div class="form-group">
          <label class="form-label">排课学期</label>
          <n-select v-model:value="store.config.semester" :options="semesterOptions" size="small" />
        </div>

        <div class="form-group">
          <label class="form-label">优先级策略</label>
          <n-select
            v-model:value="store.config.strategy"
            :options="store.strategyOptions.map(s => ({ label: s.label, value: s.value }))"
            size="small"
          />
        </div>

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

        <div class="form-group">
          <label class="form-label">求解时间</label>
          <n-select
            v-model:value="store.timePreset"
            :options="store.timePresets"
            size="small"
          />
          <span class="form-hint">时间越长，排课质量越高，但等待更久</span>
        </div>

        <n-button
          type="primary"
          block
          :loading="store.isRunning"
          @click="store.startScheduling()"
          style="margin-top: 12px"
        >
          {{ store.isRunning ? '排课中...' : '开始自动排课' }}
        </n-button>
        <n-button
          v-if="store.isRunning"
          block
          @click="store.stopScheduling()"
          style="margin-top: 8px"
        >
          停止排课
        </n-button>
      </div>

      <!-- 右侧：结果面板 -->
      <div class="result-panel">
        <h3 class="panel-title">排课进度与结果</h3>

        <!-- Step indicator -->
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
            <div class="stat-value">{{ store.result?.scheduled || '-' }}</div>
            <div class="stat-label">已排课程</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">{{ store.result?.utilization ? (store.result.utilization * 100).toFixed(1) + '%' : '-' }}</div>
            <div class="stat-label">教室利用率</div>
          </div>
          <div class="stat-card">
            <div class="stat-value" style="color: var(--b3-theme-error)">{{ store.result?.conflicts ?? '-' }}</div>
            <div class="stat-label">待处理冲突</div>
          </div>
          <div class="stat-card">
            <div class="stat-value" :style="{ color: scoreColor }">{{ store.result?.score != null ? store.result.score + '分' : '-' }}</div>
            <div class="stat-label">综合评分</div>
          </div>
        </div>

        <!-- 评分明细 -->
        <div class="score-breakdown" v-if="store.result?.scoreDetail">
          <h4 class="breakdown-title">评分明细（软约束）</h4>
          <div class="breakdown-items">
            <div class="breakdown-item" v-if="isConstraintEnabled('teacher_preference')">
              <span class="breakdown-label">教师偏好满足度</span>
              <n-progress :percentage="store.result.scoreDetail.teacherPref / categoryMax * 100" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ store.result.scoreDetail.teacherPref }}/{{ categoryMax }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('course_dispersed')">
              <span class="breakdown-label">课程间隔均匀度</span>
              <n-progress :percentage="store.result.scoreDetail.courseSpacing / categoryMax * 100" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ store.result.scoreDetail.courseSpacing }}/{{ categoryMax }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('teacher_days_limit')">
              <span class="breakdown-label">教师到校天数</span>
              <n-progress :percentage="store.result.scoreDetail.teacherDays / categoryMax * 100" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ store.result.scoreDetail.teacherDays }}/{{ categoryMax }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('low_floor_preference')">
              <span class="breakdown-label">优先低楼层</span>
              <n-progress :percentage="store.result.scoreDetail.lowFloorPref / categoryMax * 100" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ store.result.scoreDetail.lowFloorPref }}/{{ categoryMax }}</span>
            </div>
            <div class="breakdown-item" v-if="isConstraintEnabled('avoid_saturday') || isConstraintEnabled('avoid_sunday')">
              <span class="breakdown-label">周末避让</span>
              <n-progress :percentage="(store.result.scoreDetail.weekendAvoid || 0) / categoryMax * 100" :height="8" :border-radius="4" :show-indicator="false" />
              <span class="breakdown-value">{{ (store.result.scoreDetail.weekendAvoid || 0).toFixed(0) }}/{{ categoryMax }}</span>
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

.checkbox-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
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
