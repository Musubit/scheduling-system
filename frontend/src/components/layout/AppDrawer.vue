<script setup lang="ts">
import { ref } from 'vue'
import { NDrawer, NDrawerContent, NButton, NInput, NSpace } from 'naive-ui'
import type { ScheduleEntry } from '../../types'

const isOpen = ref(false)
const selectedEntry = ref<ScheduleEntry | null>(null)
const isEditing = ref(false)
const editForm = ref<Record<string, any>>({})
const isSaving = ref(false)

function openDrawer(entry: ScheduleEntry) {
  selectedEntry.value = entry
  isOpen.value = true
  isEditing.value = false
}

function closeDrawer() {
  isOpen.value = false
}

function startEdit() {
  if (!selectedEntry.value) return
  editForm.value = {
    courseName: selectedEntry.value.course?.name || '',
    teacherName: selectedEntry.value.teacher?.name || '',
    roomName: selectedEntry.value.classroom?.name || '',
  }
  isEditing.value = true
}

function cancelEdit() {
  isEditing.value = false
}

async function saveChanges() {
  if (!selectedEntry.value) return
  isSaving.value = true
  try {
    // Call Go backend to update the schedule entry
      const { UpdateTeacher, UpdateClassroom, UpdateCourse } = await import('../../../bindings/scheduling-system/services/resourceservice')
    if (selectedEntry.value.teacher && editForm.value.teacherName) {
      await UpdateTeacher({ ...selectedEntry.value.teacher, name: editForm.value.teacherName })
    }
    if (selectedEntry.value.classroom && editForm.value.roomName) {
      await UpdateClassroom({ ...selectedEntry.value.classroom, name: editForm.value.roomName })
    }
    if (selectedEntry.value.course && editForm.value.courseName) {
      await UpdateCourse({ ...selectedEntry.value.course, name: editForm.value.courseName })
    }
    // Refresh
    const { useScheduleStore } = await import('../../stores/schedule')
    const { useAppStore } = await import('../../stores/app')
    useScheduleStore().loadSchedule(useAppStore().semesterFilter)
  } catch (e) {
    console.warn('Save failed, updating locally:', e)
  }
  // Update local entry optimistically
  if (selectedEntry.value.course) selectedEntry.value.course.name = editForm.value.courseName
  if (selectedEntry.value.teacher) selectedEntry.value.teacher.name = editForm.value.teacherName
  if (selectedEntry.value.classroom) selectedEntry.value.classroom.name = editForm.value.roomName
  isEditing.value = false
  isSaving.value = false
}

defineExpose({ openDrawer, closeDrawer })
</script>

<template>
  <n-drawer v-model:show="isOpen" :width="380" placement="right">
    <n-drawer-content title="课程详情" closable>
      <template v-if="selectedEntry">
        <!-- View mode -->
        <template v-if="!isEditing">
          <div class="drawer-section">
            <div class="section-title">基本信息</div>
            <div class="detail-row"><span class="dl">课程名称</span><span class="dv">{{ selectedEntry.course?.name || '-' }}</span></div>
            <div class="detail-row"><span class="dl">课程编号</span><span class="dv">{{ selectedEntry.course?.code || '-' }}</span></div>
            <div class="detail-row"><span class="dl">学分</span><span class="dv">{{ selectedEntry.course?.credit || '-' }}</span></div>
          </div>
          <div class="drawer-section">
            <div class="section-title">上课安排</div>
            <div class="detail-row"><span class="dl">授课教师</span><span class="dv">{{ selectedEntry.teacher?.name || '-' }}</span></div>
            <div class="detail-row"><span class="dl">上课教室</span><span class="dv">{{ selectedEntry.classroom?.name || '-' }}</span></div>
            <div class="detail-row"><span class="dl">上课时间</span><span class="dv">周{{ selectedEntry.dayOfWeek + 1 }} 第{{ selectedEntry.startPeriod + 1 }}-{{ selectedEntry.startPeriod + selectedEntry.span }}节</span></div>
          </div>
        </template>

        <!-- Edit mode -->
        <template v-else>
          <div class="drawer-section">
            <div class="section-title">编辑信息</div>
            <div class="form-row">
              <label class="fl">课程名称</label>
              <n-input v-model:value="editForm.courseName" size="small" />
            </div>
            <div class="form-row">
              <label class="fl">授课教师</label>
              <n-input v-model:value="editForm.teacherName" size="small" />
            </div>
            <div class="form-row">
              <label class="fl">上课教室</label>
              <n-input v-model:value="editForm.roomName" size="small" />
            </div>
          </div>
        </template>
      </template>
      <template v-else>
        <p style="color: var(--b3-theme-on-surface)">点击课表中的课程卡片查看详情</p>
      </template>

      <template #footer>
        <n-space justify="end">
          <template v-if="!isEditing">
            <n-button size="small" @click="closeDrawer">关闭</n-button>
            <n-button size="small" @click="startEdit()">编辑</n-button>
          </template>
          <template v-else>
            <n-button size="small" @click="cancelEdit()">取消</n-button>
            <n-button size="small" type="primary" :loading="isSaving" @click="saveChanges()">保存修改</n-button>
          </template>
        </n-space>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
.drawer-section { margin-bottom: 20px; }
.section-title { font-size: 14px; font-weight: 600; color: var(--b3-theme-on-background); margin-bottom: 12px; padding-bottom: 8px; border-bottom: 1px solid var(--b3-border-color); }
.detail-row { display: flex; justify-content: space-between; padding: 6px 0; font-size: 13px; }
.dl { color: var(--b3-theme-on-surface-light); }
.dv { color: var(--b3-theme-on-background); font-weight: 500; }
.form-row { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.fl { width: 64px; font-size: 13px; color: var(--b3-theme-on-surface); flex-shrink: 0; }
</style>
