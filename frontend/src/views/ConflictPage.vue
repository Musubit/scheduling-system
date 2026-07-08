<script setup lang="ts">
import { ref } from 'vue'
import { NTag, NButton, NSpace } from 'naive-ui'

interface ConflictItem {
  id: number
  type: string
  description: string
  detail: string
  severity: 'error' | 'warning'
}

const conflicts = ref<ConflictItem[]>([
  {
    id: 1,
    type: '教师时间冲突',
    description: '王建国教授在周一第1-2节同时段有两门课程',
    detail: '高等数学(A301) vs 线性代数(B205)',
    severity: 'error',
  },
  {
    id: 2,
    type: '教室容量冲突',
    description: '计算机组成原理选课120人，分配教室A301仅80座',
    detail: '周三第3-4节 · A301',
    severity: 'error',
  },
  {
    id: 3,
    type: '教室占用冲突',
    description: 'C502教室在周四第5-6节被两门课程同时占用',
    detail: '数据结构 vs 操作系统 · C502',
    severity: 'warning',
  },
])

const selectedId = ref(1)
</script>

<template>
  <div class="conflict-page">
    <div class="conflict-layout">
      <!-- 冲突列表 -->
      <div class="conflict-list">
        <div class="list-header">
          <span>冲突列表</span>
          <n-tag type="error" size="small">{{ conflicts.length }} 项</n-tag>
        </div>
        <div
          v-for="c in conflicts"
          :key="c.id"
          class="conflict-item"
          :class="{ active: selectedId === c.id }"
          @click="selectedId = c.id"
        >
          <div class="conflict-type">
            <n-tag :type="c.severity === 'error' ? 'error' : 'warning'" size="tiny">✕</n-tag>
            {{ c.type }}
          </div>
          <div class="conflict-desc">{{ c.description }}</div>
          <div class="conflict-detail">{{ c.detail }}</div>
        </div>
      </div>

      <!-- 冲突详情 -->
      <div class="conflict-detail-panel">
        <div class="detail-header">
          <h3>冲突详情</h3>
          <n-button size="small" text>📍 定位到课表</n-button>
        </div>

        <template v-if="selectedId === 1">
          <div class="detail-grid">
            <div class="detail-item"><span class="dl">冲突教师</span><span class="dv">王建国 教授</span></div>
            <div class="detail-item"><span class="dl">所属院系</span><span class="dv">数学与统计学院</span></div>
            <div class="detail-item"><span class="dl">冲突课程 A</span><span class="dv">高等数学 · 周一 1-2节 · A301</span></div>
            <div class="detail-item"><span class="dl">冲突课程 B</span><span class="dv">线性代数 · 周一 1-2节 · B205</span></div>
          </div>
        </template>

        <div class="resolve-section">
          <h4>解决方案</h4>
          <div class="resolve-options">
            <div class="resolve-option"><span class="resolve-radio"></span>将「线性代数」调至其他时段</div>
            <div class="resolve-option"><span class="resolve-radio"></span>更换「线性代数」授课教师</div>
            <div class="resolve-option"><span class="resolve-radio"></span>合并为合班授课</div>
          </div>
          <div class="resolve-actions">
            <n-button size="small">暂不处理</n-button>
            <n-button size="small" type="primary">确认解决</n-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.conflict-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.conflict-layout {
  flex: 1;
  display: grid;
  grid-template-columns: 300px 1fr;
  gap: 20px;
  min-height: 0;
}

.conflict-list {
  background: var(--b3-theme-surface);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius);
  overflow-y: auto;
}

.list-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  font-size: 13px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  border-bottom: 1px solid var(--b3-border-color);
}

.conflict-item {
  padding: 12px 16px;
  border-bottom: 1px solid var(--b3-border-color);
  cursor: pointer;
  transition: background 0.15s;
}

.conflict-item:hover {
  background: var(--b3-list-hover);
}

.conflict-item.active {
  background: var(--b3-theme-primary-lightest);
  border-left: 3px solid var(--b3-theme-primary);
}

.conflict-type {
  font-size: 13px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
}

.conflict-desc {
  font-size: 12px;
  color: var(--b3-theme-on-surface);
  margin-bottom: 2px;
}

.conflict-detail {
  font-size: 11px;
  color: var(--b3-theme-on-surface-light);
}

.conflict-detail-panel {
  background: var(--b3-theme-surface);
  border: 1px solid var(--b3-border-color);
  border-radius: var(--b3-border-radius);
  padding: 20px;
  overflow-y: auto;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.detail-header h3 {
  font-size: 14px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
}

.detail-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-bottom: 20px;
}

.detail-item {
  padding: 8px 0;
}

.dl {
  display: block;
  font-size: 11px;
  color: var(--b3-theme-on-surface-light);
  margin-bottom: 2px;
}

.dv {
  font-size: 13px;
  color: var(--b3-theme-on-background);
  font-weight: 500;
}

.resolve-section {
  border-top: 1px solid var(--b3-border-color);
  padding-top: 16px;
}

.resolve-section h4 {
  font-size: 13px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin-bottom: 12px;
}

.resolve-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 14px;
}

.resolve-option {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--b3-theme-on-surface);
  cursor: pointer;
}

.resolve-radio {
  width: 14px;
  height: 14px;
  border: 2px solid var(--b3-border-color);
  border-radius: 50%;
  flex-shrink: 0;
}

.resolve-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
