<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import {
  NButton, NSelect, NDataTable, NCard, NSpace, NEmpty,
  NTag, NGrid, NGi, NStatistic, NSpin, useMessage,
} from 'naive-ui'
import { useAppStore } from '../stores/app'
import { GetSnapshots, CompareSnapshots } from '../../bindings/scheduling-system/backend/services/snapshotservice'

const appStore = useAppStore()
const message = useMessage()

// ---- State ----
const snapshots = ref<any[]>([])
const loadingSnapshots = ref(false)
const comparing = ref(false)

const snapshotA = ref<any | null>(null)
const snapshotB = ref<any | null>(null)
const result = ref<any | null>(null)

// ---- Snapshot Select Options ----
const snapshotOptions = computed(() =>
  snapshots.value.map((s: any) => ({
    label: s.name || '未命名',
    value: s,
  }))
)

function formatTime(ms: number): string {
  if (!ms && ms !== 0) return '-'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.floor(ms / 60000)}m${Math.floor((ms % 60000) / 1000)}s`
}

function formatDate(raw: string | Date): string {
  if (!raw) return '-'
  return new Date(raw).toLocaleString('zh-CN')
}

// ---- Data Loading ----
async function loadSnapshots() {
  loadingSnapshots.value = true
  try {
    const result = await GetSnapshots(appStore.currentSemesterId)
    snapshots.value = result || []
    if (snapshots.value.length === 0) {
      message.warning('暂无快照，请先去「自动排课」或「验证报告」生成快照')
    }
  } catch (e: any) {
    console.warn('Failed to load snapshots:', e)
    message.error('加载快照列表失败')
    snapshots.value = []
  } finally {
    loadingSnapshots.value = false
  }
}

async function handleCompare() {
  if (!snapshotA.value || !snapshotB.value) {
    message.warning('请先选择 A（基线）和 B（对比）快照')
    return
  }
  if (snapshotA.value.ID === snapshotB.value.ID) {
    message.warning('请选择两个不同的快照进行对比')
    return
  }
  comparing.value = true
  result.value = null
  try {
    const res = await CompareSnapshots(snapshotA.value.ID, snapshotB.value.ID)
    if (!res) {
      message.error('对比失败，返回空结果')
      return
    }
    result.value = res
  } catch (e: any) {
    console.warn('Compare failed:', e)
    message.error('对比执行失败')
  } finally {
    comparing.value = false
  }
}

// ---- Teacher Diffs Table ----
const teacherColumns = [
  {
    title: '教师',
    key: 'name',
    width: 160,
    render: (row: any) => `${row.name}（${row.code}）`,
  },
  { title: '课时变化', key: 'entryDelta', width: 100 },
  { title: '早课惩罚变化', key: 'earlyDelta', width: 120 },
  { title: '晚课惩罚变化', key: 'lateDelta', width: 120 },
  {
    title: '上课天数',
    key: 'daysActualB',
    width: 180,
    render: (row: any) =>
      `${row.daysActualA} → ${row.daysActualB}（目标 ${row.daysTarget}天）`,
  },
  { title: '平均楼层变化', key: 'avgFloorDelta', width: 120 },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render: (row: any) => {
      const statusMap: Record<string, { type: 'success' | 'error' | 'default' | 'info' | 'warning'; label: string }> = {
        improved: { type: 'success', label: '改善' },
        regressed: { type: 'error', label: '退化' },
        unchanged: { type: 'default', label: '不变' },
        added: { type: 'info', label: '新增' },
        removed: { type: 'warning', label: '移除' },
      }
      const cfg = statusMap[row.status] || { type: 'default', label: row.status }
      return h(NTag, { type: cfg.type }, { default: () => cfg.label })
    },
  },
]

const hasTeacherDiffs = computed(() =>
  result.value?.teacherDiffs && result.value.teacherDiffs.length > 0
)

// ---- Score Delta Display Helpers ----
function scoreDeltaColor(delta: number): string {
  if (delta > 0) return '#18a058'
  if (delta < 0) return '#d03050'
  return 'var(--b3-theme-on-surface)'
}

function conflictDeltaLabel(delta: number): string {
  if (delta > 0) return '冲突已解决 ✓'
  if (delta < 0) return '新增冲突 ✗'
  return '无变化'
}

function conflictDeltaType(delta: number): 'success' | 'error' | 'default' {
  if (delta > 0) return 'success'
  if (delta < 0) return 'error'
  return 'default'
}

// ---- Lifecycle ----
onMounted(() => {
  loadSnapshots()
})
</script>

<template>
  <div class="history-compare">
    <h2 class="page-title">历史课表对比</h2>

    <!-- Snapshot Selection -->
    <NCard class="section-card" title="选择快照">
      <NSpace vertical>
        <NSpace align="center" wrap>
          <span class="label">A（基线）：</span>
          <NSelect
            v-model:value="snapshotA"
            :options="snapshotOptions"
            placeholder="选择基线快照"
            :loading="loadingSnapshots"
            class="snapshot-select"
            clearable
            value-field="value"
          />
          <span class="label">B（对比）：</span>
          <NSelect
            v-model:value="snapshotB"
            :options="snapshotOptions"
            placeholder="选择对比快照"
            :loading="loadingSnapshots"
            class="snapshot-select"
            clearable
            value-field="value"
          />
          <NButton type="primary" @click="handleCompare" :loading="comparing">
            开始对比
          </NButton>
          <NButton @click="loadSnapshots" :loading="loadingSnapshots" tertiary>
            刷新
          </NButton>
        </NSpace>
      </NSpace>
    </NCard>

    <!-- No snapshots placeholder -->
    <NEmpty
      v-if="!loadingSnapshots && snapshots.length === 0 && !result"
      description="暂无快照，请先去「自动排课」或「验证报告」生成快照"
      class="empty-state"
    />

    <!-- Compare Result -->
    <template v-if="result">
      <!-- Overall Comparison -->
      <NCard class="section-card" title="总体对比">
        <NGrid :cols="3" :x-gap="16" :y-gap="16">
          <!-- Snapshot A -->
          <NGi>
            <NCard size="small" title="A（基线）" class="snapshot-card">
              <NSpace vertical size="small">
                <div class="stat-row">
                  <span class="stat-label">快照</span>
                  <span class="stat-value">{{ result.a?.name || '未命名' }}</span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">总分</span>
                  <span class="stat-value">{{ result.a?.totalScore?.toFixed(1) }}</span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">课时数</span>
                  <span class="stat-value">{{ result.a?.totalEntries }}</span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">硬约束</span>
                  <NTag :type="result.a?.hardPassed ? 'success' : 'error'" size="tiny">
                    {{ result.a?.hardPassed ? '通过' : '未通过' }}
                  </NTag>
                </div>
                <div class="stat-row">
                  <span class="stat-label">求解器</span>
                  <span class="stat-value">{{ result.a?.solver || '-' }}</span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">耗时</span>
                  <span class="stat-value">{{ formatTime(result.a?.solveTimeMs) }}</span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">创建时间</span>
                  <span class="stat-value">{{ formatDate(result.a?.CreatedAt) }}</span>
                </div>
              </NSpace>
            </NCard>
          </NGi>

          <!-- Delta -->
          <NGi>
            <NCard size="small" title="变化对比" class="snapshot-card delta-card">
              <NSpace vertical size="small">
                <div class="stat-row">
                  <span class="stat-label">总分变化</span>
                  <span class="stat-value" :style="{ color: scoreDeltaColor(result.scoreDelta) }">
                    {{ result.scoreDelta > 0 ? '+' : '' }}{{ result.scoreDelta?.toFixed(1) }}
                  </span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">课时变化</span>
                  <span class="stat-value" :style="{ color: scoreDeltaColor(result.entryDelta) }">
                    {{ result.entryDelta > 0 ? '+' : '' }}{{ result.entryDelta }}
                  </span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">冲突变化</span>
                  <NTag :type="conflictDeltaType(result.conflictDelta)" size="tiny">
                    {{ conflictDeltaLabel(result.conflictDelta) }}
                  </NTag>
                </div>
              </NSpace>
            </NCard>
          </NGi>

          <!-- Snapshot B -->
          <NGi>
            <NCard size="small" title="B（对比）" class="snapshot-card">
              <NSpace vertical size="small">
                <div class="stat-row">
                  <span class="stat-label">快照</span>
                  <span class="stat-value">{{ result.b?.name || '未命名' }}</span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">总分</span>
                  <span class="stat-value">{{ result.b?.totalScore?.toFixed(1) }}</span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">课时数</span>
                  <span class="stat-value">{{ result.b?.totalEntries }}</span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">硬约束</span>
                  <NTag :type="result.b?.hardPassed ? 'success' : 'error'" size="tiny">
                    {{ result.b?.hardPassed ? '通过' : '未通过' }}
                  </NTag>
                </div>
                <div class="stat-row">
                  <span class="stat-label">求解器</span>
                  <span class="stat-value">{{ result.b?.solver || '-' }}</span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">耗时</span>
                  <span class="stat-value">{{ formatTime(result.b?.solveTimeMs) }}</span>
                </div>
                <div class="stat-row">
                  <span class="stat-label">创建时间</span>
                  <span class="stat-value">{{ formatDate(result.b?.CreatedAt) }}</span>
                </div>
              </NSpace>
            </NCard>
          </NGi>
        </NGrid>
      </NCard>

      <!-- Teacher-Level Comparison Table -->
      <NCard class="section-card" title="教师级对比">
        <NDataTable
          v-if="hasTeacherDiffs"
          :columns="teacherColumns"
          :data="result.teacherDiffs"
          :bordered="false"
          :single-line="false"
          size="small"
          striped
        />
        <NEmpty
          v-else
          description="教师级对比数据为空，所选快照可能不存在教师级别的差异"
        />
      </NCard>
    </template>

    <!-- Loading state (before result) -->
    <NSpace v-if="comparing && !result" justify="center" class="loading-state">
      <NSpin>
        <span>正在对比中...</span>
      </NSpin>
    </NSpace>
  </div>
</template>

<style scoped>
.history-compare {
  max-width: 1100px;
}

.page-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin-bottom: 24px;
}

.section-card {
  background: var(--b3-theme-surface);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius);
  margin-bottom: 16px;
}

.snapshot-select {
  min-width: 240px;
}

.label {
  font-size: 13px;
  font-weight: 500;
  color: var(--b3-theme-on-background);
  white-space: nowrap;
}

.snapshot-card {
  height: 100%;
}

.delta-card {
  display: flex;
  align-items: center;
  justify-content: center;
}

.stat-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 0;
}

.stat-label {
  font-size: 12px;
  color: var(--b3-theme-on-surface-light);
}

.stat-value {
  font-size: 13px;
  font-weight: 500;
  color: var(--b3-theme-on-background);
}

.empty-state {
  margin-top: 48px;
}

.loading-state {
  margin-top: 48px;
}
</style>
