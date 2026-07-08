<script setup lang="ts">
import { useResourceStore } from '../stores/resource'
import { NButton, NInput, NSelect, NDataTable, NSpace, NTag } from 'naive-ui'
import { DEPARTMENTS } from '../types'
import { ref, computed, h } from 'vue'

const resourceStore = useResourceStore()

const tabOptions = [
  { key: 'teacher' as const, label: '教师' },
  { key: 'classroom' as const, label: '教室' },
  { key: 'course' as const, label: '课程' },
  { key: 'class' as const, label: '班级' },
]

// ===== 模拟数据（阶段3替换为真实数据） =====
const mockTeachers = [
  { id: 1, code: 'T001', name: '王建国', dept: '数学与统计学院', title: '教授', status: 'active', weeklyHours: 12 },
  { id: 2, code: 'T002', name: '张明远', dept: '计算机科学学院', title: '教授', status: 'active', weeklyHours: 10 },
  { id: 3, code: 'T003', name: '李伟', dept: '计算机科学学院', title: '副教授', status: 'active', weeklyHours: 14 },
]

const teacherColumns = [
  { title: '工号', key: 'code', width: 80 },
  { title: '姓名', key: 'name', width: 100 },
  { title: '院系', key: 'dept', width: 160 },
  { title: '职称', key: 'title', width: 80 },
  { title: '课时', key: 'weeklyHours', width: 70 },
  {
    title: '状态', key: 'status', width: 80,
    render: (row: any) => h(NTag, { type: row.status === 'active' ? 'success' : 'default', size: 'small' },
      { default: () => row.status === 'active' ? '在职' : '休假' }
    ),
  },
  {
    title: '操作', key: 'actions', width: 140,
    render: () => h(NSpace, { size: 'small' }, {
      default: () => [h(NButton, { size: 'tiny', text: true }, { default: () => '编辑' }), h(NButton, { size: 'tiny', text: true, type: 'error' }, { default: () => '删除' })],
    }),
  },
]

const deptOptions = [
  { label: '全部院系', value: '全部院系' },
  ...DEPARTMENTS.map(d => ({ label: d.name, value: d.name })),
]
</script>

<template>
  <div class="resource-page">
    <div class="resource-tabs">
      <n-button
        v-for="tab in tabOptions"
        :key="tab.key"
        :type="resourceStore.activeTab === tab.key ? 'primary' : 'default'"
        size="small"
        @click="resourceStore.switchTab(tab.key)"
      >
        {{ tab.label }}
      </n-button>
    </div>

    <div class="resource-toolbar">
      <n-input
        :placeholder="`搜索${tabOptions.find(t => t.key === resourceStore.activeTab)?.label || ''}...`"
        clearable
        size="small"
        style="width: 240px"
      />
      <n-select
        :options="deptOptions"
        size="small"
        style="width: 160px"
        default-value="全部院系"
      />
      <div class="spacer"></div>
      <n-button size="small" type="primary">+ 新增</n-button>
      <n-button size="small">导入</n-button>
      <n-button size="small">导出</n-button>
    </div>

    <div class="resource-table">
      <n-data-table
        v-if="resourceStore.activeTab === 'teacher'"
        :columns="teacherColumns"
        :data="mockTeachers"
        :single-line="false"
        size="small"
      />
      <div v-else class="placeholder">
        <p>{{ tabOptions.find(t => t.key === resourceStore.activeTab)?.label }}管理（阶段2实现）</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.resource-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.resource-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

.resource-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
}

.spacer {
  flex: 1;
}

.resource-table {
  flex: 1;
  overflow: auto;
}

.placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: var(--b3-theme-on-surface);
  font-size: 14px;
}
</style>
