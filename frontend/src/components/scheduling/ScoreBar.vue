<script setup lang="ts">
import { NProgress } from 'naive-ui'

defineProps<{
  label: string
  score: number
  maxScore: number
  height?: number
  showValue?: boolean
  colorFn?: (pct: number) => string
}>()
</script>

<template>
  <div class="score-bar-item">
    <span class="score-bar-label">{{ label }}</span>
    <n-progress
      type="line"
      :percentage="Math.min(100, (score / maxScore) * 100)"
      :height="height || 8"
      :border-radius="4"
      :show-indicator="false"
      :color="colorFn ? colorFn(score) : undefined"
    />
    <span v-if="showValue" class="score-bar-value">{{ score.toFixed(0) }}/{{ maxScore }}</span>
  </div>
</template>

<style scoped>
.score-bar-item {
  display: flex;
  align-items: center;
  gap: 8px;
}
.score-bar-label {
  width: 70px;
  font-size: 13px;
  flex-shrink: 0;
}
.score-bar-value {
  width: 40px;
  text-align: right;
  font-size: 13px;
  font-weight: 600;
}
</style>
