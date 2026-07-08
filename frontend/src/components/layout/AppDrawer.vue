<script setup lang="ts">
import { ref, computed } from 'vue'
import { NDrawer, NDrawerContent, NButton, NInput, NSelect, NSpace, NTag } from 'naive-ui'
import type { ScheduleEntry } from '../../types'
import { DAY_NAMES } from '../../types'

const isOpen = ref(false)
const selectedEntry = ref<ScheduleEntry | null>(null)
const isEditing = ref(false)
const editForm = ref<Record<string, any>>({})
const isSaving = ref(false)
const conflictMessage = ref('')

// All entries from the store for conflict checking
let allEntries: ScheduleEntry[] = []

function setAllEntries(entries: ScheduleEntry[]) {
  allEntries = entries
}

const dayOptions = DAY_NAMES.map((name, i) => ({ value: i, label: name }))
const periodOptions = [
  { value: 0, label: '第1-2节 (08:20-09:55)' },
  { value: 2, label: '第3-4节 (10:15-11:50)' },
  { value: 4, label: '第5-6节 (14:00-15:35)' },
  { value: 6, label: '第7-8节 (15:55-17:30)' },
  { value: 8, label: '第9-10节 (18:30-20:05)' },
]

function openDrawer(entry: ScheduleEntry) {
  selectedEntry.value = entry
  isOpen.value = true
  isEditing.value = false
  conflictMessage.value = ''
}

function closeDrawer() {
  isOpen.value = false
  conflictMessage.value = ''
}

function startEdit() {
  if (!selectedEntry.value) return
  editForm.value = {
    courseName: selectedEntry.value.course?.name || '',
    teacherName: selectedEntry.value.teacher?.name || '',
    roomName: selectedEntry.value.classroom?.name || '',
    dayOfWeek: selectedEntry.value.dayOfWeek,
    startPeriod: selectedEntry.value.startPeriod,
    span: selectedEntry.value.span,
  }
  isEditing.value = true
  conflictMessage.value = ''
}

function cancelEdit() {
  isEditing.value = false
  conflictMessage.value = ''
}

const hasChanges = computed(() => {
  if (!selectedEntry.value) return false
  const e = selectedEntry.value
  return editForm.value.dayOfWeek !== e.dayOfWeek ||
    editForm.value.startPeriod !== e.startPeriod ||
    editForm.value.roomName !== (e.classroom?.name || '') ||
    editForm.value.teacherName !== (e.teacher?.name || '')
})

function checkConflicts(): string | null {
  if (!selectedEntry.value) return null
  const e = selectedEntry.value
  const newDay = editForm.value.dayOfWeek ?? e.dayOfWeek
  const newStart = editForm.value.startPeriod ?? e.startPeriod
  const newSpan = editForm.value.span ?? e.span

  // Check against all other entries
  for (const other of allEntries) {
    if (other.ID === e.ID) continue // skip self
    if (other.semester !== e.semester) continue

    const periodsOverlap = (s1: number, sp1: number, s2: number, sp2: number) => {
      const e1 = s1 + sp1, e2 = s2 + sp2
      return s1 < e2 && s2 < e1
    }

    if (!periodsOverlap(newStart, newSpan, other.startPeriod, other.span)) continue
    if (other.dayOfWeek !== newDay) continue

    // Teacher conflict
    if (other.teacherId === e.teacherId) {
      return `教师 ${e.teacher?.name} 在周${newDay + 1}该时段已有课程「${other.course?.name}」`
    }

    // Room conflict
    const newRoomName = editForm.value.roomName || e.classroom?.name
    if (other.classroom?.name === newRoomName) {
      return `教室 ${newRoomName} 在周${newDay + 1}该时段已被「${other.course?.name}」占用`
    }

    // Class group conflict
    if (e.classGroupId && other.classGroupId === e.classGroupId) {
      return `班级 ${e.classGroup?.name} 在周${newDay + 1}该时段已有课程「${other.course?.name}」`
    }
  }

  return null
}

async function saveChanges() {
  if (!selectedEntry.value) return

  // Validate conflicts
  const conflict = checkConflicts()
  if (conflict) {
    conflictMessage.value = conflict
    return
  }

  isSaving.value = true
  try {
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
    console.warn('Save failed:', e)
  }
  // Update local entry optimistically
  if (selectedEntry.value.course) selectedEntry.value.course.name = editForm.value.courseName
  if (selectedEntry.value.teacher) selectedEntry.value.teacher.name = editForm.value.teacherName
  if (selectedEntry.value.classroom) selectedEntry.value.classroom.name = editForm.value.roomName
  selectedEntry.value.dayOfWeek = editForm.value.dayOfWeek
  selectedEntry.value.startPeriod = editForm.value.startPeriod
  selectedEntry.value.span = editForm.value.span
  isEditing.value = false
  isSaving.value = false
  conflictMessage.value = ''
}

defineExpose({ openDrawer, closeDrawer, setAllEntries })
</script>

<template>
  <n-drawer v-model:show="isOpen" :width="400" placement="right" :show-mask="false">
    <n-drawer-content title="课程详情" closable>
      <template v-if="selectedEntry">
        <!-- View mode -->
        <template v-if="!isEditing">
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
            <div class="detail-row"><span class="dl">上课时间</span><span class="dv">周{{ selectedEntry.dayOfWeek + 1 }} 第{{ selectedEntry.startPeriod + 1 }}-{{ selectedEntry.startPeriod + selectedEntry.span }}节</span></div>
            <div class="detail-row"><span class="dl">教学周</span><span class="dv">{{ selectedEntry.weeks || '1-16' }}周</span></div>
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
            <div class="form-row">
              <label class="fl">星期</label>
              <n-select v-model:value="editForm.dayOfWeek" :options="dayOptions" size="small" style="flex:1" />
            </div>
            <div class="form-row">
              <label class="fl">节次</label>
              <n-select v-model:value="editForm.startPeriod" :options="periodOptions" size="small" style="flex:1" />
            </div>
          </div>

          <!-- Conflict warning -->
          <div v-if="conflictMessage" class="conflict-warning">
            ⚠️ {{ conflictMessage }}
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
            <n-button size="small" type="primary" :loading="isSaving" :disabled="!!conflictMessage" @click="saveChanges()">保存修改</n-button>
          </template>
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
.form-row { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.fl { width: 64px; font-size: 13px; color: var(--b3-theme-on-surface); flex-shrink: 0; }
.conflict-warning {
  background: #fff3e0;
  border: 1px solid #ff9800;
  border-radius: 6px;
  padding: 10px 12px;
  font-size: 13px;
  color: #e65100;
  margin-top: 12px;
}
</style>
