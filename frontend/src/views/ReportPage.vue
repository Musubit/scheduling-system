<script setup lang="ts">
import { ref } from 'vue'
import { NTag, NButton, NEmpty } from 'naive-ui'
import { useAppStore } from '../stores/app'

const appStore = useAppStore()

// Placeholder: 快照列表将在 Phase 4 完整实现
const snapshots = ref<any[]>([])

// 轻量评分预览（微调后显示）
const livePreview = ref<{ score: number } | null>(null)

function generateReport() {
  // TODO: Phase 4 — 调用 Go 后端生成快照
}

function exportPDF() {
  window.print()
}
</script>

<template>
  <div class="report-page">
    <div class="report-header">
      <h2 class="report-title">验证报告</h2>
      <div class="report-actions">
        <n-button type="primary" @click="generateReport" :disabled="true">
          生成报告（即将上线）
        </n-button>
        <n-button @click="exportPDF" :disabled="true">
          导出 PDF
        </n-button>
      </div>
    </div>

    <!-- 快照历史列表 -->
    <div class="snapshot-list">
      <n-empty v-if="snapshots.length === 0" description="暂无排课快照，请先运行排课引擎" />
    </div>

    <!-- 轻量评分预览 -->
    <div v-if="livePreview" class="live-preview">
      <n-tag type="info">当前评分：{{ livePreview.score }}</n-tag>
    </div>
  </div>
</template>

<style scoped>
.report-page {
  padding: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.report-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.report-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--b3-theme-on-background);
  margin: 0;
}

.report-actions {
  display: flex;
  gap: 12px;
}

.snapshot-list {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
}

.live-preview {
  margin-top: 16px;
  padding: 12px;
  background: var(--b3-card-background);
  border-radius: 6px;
}
</style>
