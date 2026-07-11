<script setup lang="ts">
import { ref } from 'vue'
import { NDrawer, NDrawerContent, NButton, NSpace } from 'naive-ui'
import type { ScheduleEntry } from '../../types'
import { DAY_NAMES } from '../../types'

const isOpen = ref(false)
const selectedEntry = ref<ScheduleEntry | null>(null)

// All entries from the store for conflict checking
let allEntries: ScheduleEntry[] = []

function setAllEntries(entries: ScheduleEntry[]) {
  allEntries = entries
}

function openDrawer(entry: ScheduleEntry) {
  selectedEntry.value = entry
  isOpen.value = true
}

function closeDrawer() {
  isOpen.value = false
}

defineExpose({ openDrawer, closeDrawer, setAllEntries })
</script>

<template>
  <n-drawer v-model:show="isOpen" :width="400" placement="right" :show-mask="false">
    <n-drawer-content title="课程详情" closable>
      <template v-if="selectedEntry">
        <div class="drawer-section">
          <div class="section-title">基本信息</div>
          <div class="detail-row"><span class="dl">课程名称</span><span class="dv">{{ selectedEntry.course?.name || '-' }}</span></div>
          <div class="detail-row"><span class="dl">课程编号</span><span class="dv">{{ selectedEntry.course?.code || '-' }}</span></div>
          <div class="detail-row"><span class="dl">学分</span><span class="dv">{{ selectedEntry.course?.credit || '-' }}</span></div>
          <div class="detail-row" v-if="selectedEntry.classGroup"><span class="dl">上课班级</span><span class="dv">{{ selectedEntry.classGroup?.name || '-' }}</span></div>
        </div>
        <div class="drawer-section">
          <div class="section-title">上课安排</div>
          <div class="detail-row"><span class="dl">授课教师</span><span class="dv">{{ selectedEntry.teacher?.name || '-' }}</span></div>
          <div class="detail-row"><span class="dl">上课教室</span><span class="dv">{{ selectedEntry.classroom?.name || '-' }}</span></div>
          <div class="detail-row"><span class="dl">上课时间</span><span class="dv">{{ DAY_NAMES[selectedEntry.dayOfWeek] }} 第{{ selectedEntry.startPeriod + 1 }}-{{ selectedEntry.startPeriod + selectedEntry.span }}节</span></div>
          <div class="detail-row"><span class="dl">教学周</span><span class="dv">{{ selectedEntry.weeks || '1-16' }}周</span></div>
        </div>
      </template>
      <template v-else>
        <p style="color: var(--b3-theme-on-surface)">点击课表中的课程卡片查看详情</p>
      </template>

      <template #footer>
        <n-space justify="end">
          <n-button size="small" @click="closeDrawer">关闭</n-button>
        </n-space>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
.drawer-section { margin-bottom: 20px; }
.section-title {
  font-size: 14px; font-weight: 600; color: var(--b3-theme-on-background);
  margin-bottom: 12px; padding-bottom: 8px;
  border-bottom: 1px solid var(--b3-border-color);
}
.detail-row { display: flex; justify-content: space-between; padding: 6px 0; font-size: 13px; }
.dl { color: var(--b3-theme-on-surface-light); }
.dv { color: var(--b3-theme-on-background); font-weight: 500; }
</style>
