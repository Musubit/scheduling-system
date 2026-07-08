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
          <label class="form-label">算法迭代次数</label>
          <n-input-number v-model:value="store.config.iterations" :min="100" :max="50000" size="small" style="width:100%" />
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
            <div class="stat-value" style="color: var(--b3-theme-error)">{{ store.result?.conflicts || '-' }}</div>
            <div class="stat-label">待处理冲突</div>
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
}

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
  grid-template-columns: repeat(3, 1fr);
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
</style>
