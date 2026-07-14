	<script setup lang="ts">
import { reactive, ref, computed, onMounted } from 'vue'
import { NSwitch, NButton, NInput, NDatePicker, NModal, NForm, NFormItem, NSpace, NRadioGroup, NRadio } from 'naive-ui'
	import { useAppStore } from '../stores/app'
	import LockedTimeGrid from '../components/scheduling/LockedTimeGrid.vue'

const appStore = useAppStore()

// Basic settings
const settings = reactive({
  autoSave: true,
  realtimeCheck: true,
})

const saved = localStorage.getItem('scheduling-settings')
if (saved) {
  try { Object.assign(settings, JSON.parse(saved)) } catch {}
}

// ===== Semester Management =====
// v0.5.5 修订：结构化字段（AcademicYear + Term + StartDate + Status），
// 显示名称由后端 DisplayName() 提供，前端不再拼 name。
const TERM_FIRST = 'FIRST'
const TERM_SECOND = 'SECOND'
const STATUS_ACTIVE = 'active'
const STATUS_PLANNED = 'planned'
const STATUS_ARCHIVED = 'archived'

interface SemesterData {
  ID: number
  academicYear: string
  term: string
  startDate: string   // ISO string from Go time.Time
  endDate: string
  status: string
}

const semesters = ref<SemesterData[]>([])
const showSemesterModal = ref(false)
const editingSemester = ref<SemesterData | null>(null)
const semesterForm = reactive({
  academicYear: '',
  term: TERM_FIRST,
  startDate: '',      // YYYY-MM-DD
  isActive: false,
})

// NDatePicker value ↔ YYYY-MM-DD string
const semesterDateVal = computed({
  get: () => semesterForm.startDate ? new Date(semesterForm.startDate).getTime() : null,
  set: (ts: number | null) => {
    if (ts) {
      const d = new Date(ts)
      semesterForm.startDate = `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`
    } else {
      semesterForm.startDate = ''
    }
  },
})

// 显示名称：{academicYear}{Term第X学期}，与后端 Semester.DisplayName() 对齐。
function displayName(sem: SemesterData): string {
  const label = sem.term === TERM_SECOND ? '第二学期' : '第一学期'
  return `${sem.academicYear}${label}`
}

// 学期第一天：Go time.Time 序列化后是 ISO 字符串；只截 YYYY-MM-DD 展示。
function formatDate(iso: string): string {
  if (!iso) return ''
  const idx = iso.indexOf('T')
  return idx > 0 ? iso.slice(0, idx) : iso
}

async function loadSemesters() {
  try {
      const { GetSemesters } = await import('../../bindings/scheduling-system/backend/services/resourceservice')
    const result = await GetSemesters()
    semesters.value = (result || []) as SemesterData[]
  } catch (e) {
    console.warn('Failed to load semesters:', e)
  }
}

function resetForm() {
  semesterForm.academicYear = defaultNextAcademicYear()
  semesterForm.term = defaultNextTerm()
  semesterForm.startDate = ''
  semesterForm.isActive = false
}

// 根据当前月份推荐"下一个学期"，与后端 nextUpcomingSemester 逻辑一致。
function defaultNextTerm(): string {
  const m = new Date().getMonth() + 1 // 1-12
  // 3-8 月 → 下学期是秋季（FIRST）；9-次年 2 月 → 下学期是春季（SECOND）
  return (m >= 3 && m <= 8) ? TERM_FIRST : TERM_SECOND
}
function defaultNextAcademicYear(): string {
  const now = new Date()
  const y = now.getFullYear()
  const m = now.getMonth() + 1
  if (m >= 3 && m <= 8) return `${y}-${y + 1}`         // 秋季 = 当年-次年
  if (m >= 9) return `${y}-${y + 1}`                    // 9-12 → 次年春季 = 当年-次年
  return `${y - 1}-${y}`                                // 1-2 月 → 次年春季 = 上年-当年
}

function openNewSemester() {
  editingSemester.value = null
  resetForm()
  showSemesterModal.value = true
}

function openEditSemester(sem: SemesterData) {
  editingSemester.value = sem
  semesterForm.academicYear = sem.academicYear || ''
  semesterForm.term = sem.term || TERM_FIRST
  semesterForm.startDate = formatDate(sem.startDate)
  semesterForm.isActive = sem.status === STATUS_ACTIVE
  showSemesterModal.value = true
}

async function saveSemester() {
  // 简单校验：学年格式 YYYY-YYYY；学期第一天必填。
  if (!/^\d{4}-\d{4}$/.test(semesterForm.academicYear.trim())) {
    alert('学年格式应为 YYYY-YYYY，例如 2026-2027')
    return
  }
  if (!semesterForm.startDate) {
    alert('请选择学期第一天')
    return
  }
  try {
    const { CreateSemester, UpdateSemester } = await import('../../bindings/scheduling-system/backend/services/resourceservice')
    const status = semesterForm.isActive ? STATUS_ACTIVE :
                   (editingSemester.value?.status === STATUS_ARCHIVED ? STATUS_ARCHIVED : STATUS_PLANNED)
    // Go time.Time 接受 RFC3339；补 T00:00:00Z 让 UTC 中午前后不会漂移日期。
    const startIso = `${semesterForm.startDate}T00:00:00Z`
    // endDate 传 Go time.Time 的零值 —— 空串会被 Wails 参数解析器拒绝
    // （"cannot parse "" as "2006""）；后端 normalizeSemester 检测 IsZero()
    // 后自动补 EndDate = StartDate + 18 周 - 1 天。
    const zeroTime = '0001-01-01T00:00:00Z'
    const data = {
      ID: editingSemester.value?.ID || 0,
      academicYear: semesterForm.academicYear.trim(),
      term: semesterForm.term,
      startDate: startIso,
      endDate: zeroTime,      // 触发后端自动补
      status,
    } as any
    if (editingSemester.value) {
      await UpdateSemester(data)
    } else {
      await CreateSemester(data)
    }
    await loadSemesters()
    await appStore.loadSemesters()  // refresh toolbar dropdown
    showSemesterModal.value = false
  } catch (e: any) {
    console.warn('Failed to save semester:', e)
    alert('保存失败：' + (e?.message || e))
  }
}

async function deleteSemester(id: number) {
  if (!confirm('确定要删除该学期吗？')) return
  try {
    const { DeleteSemester } = await import('../../bindings/scheduling-system/backend/services/resourceservice')
    await DeleteSemester(id)
    await loadSemesters()
    await appStore.loadSemesters()  // refresh toolbar dropdown
  } catch (e) {
    console.warn('Failed to delete semester:', e)
  }
}

onMounted(() => {
  loadSemesters()
})

// ===== Backup / Restore =====
async function handleBackup() {
  try {
    const { Dialogs } = await import('@wailsio/runtime')
    const filePath = await (Dialogs as any).SaveFile({
      title: '选择备份保存位置',
      defaultFilename: `scheduling-backup-${new Date().toISOString().slice(0, 10)}.db`,
      filters: [{ display: '数据库文件', pattern: '*.db' }],
    })
    if (!filePath) return
    const { BackupDatabase } = await import('../../bindings/scheduling-system/backend/services/resourceservice')
    await BackupDatabase(filePath)
  } catch { /* silent */ }
}

async function handleRestore() {
  try {
    const { Dialogs } = await import('@wailsio/runtime')
    const filePath = await (Dialogs as any).OpenFile({
      title: '选择数据库备份文件',
      filters: [{ display: '数据库文件', pattern: '*.db' }],
    })
    if (!filePath) return
    if (!confirm('导入将覆盖当前所有数据，确定继续？')) return
    const { RestoreDatabase } = await import('../../bindings/scheduling-system/backend/services/resourceservice')
    await RestoreDatabase(filePath)
  } catch { /* silent */ }
}
</script>

<template>
  <div class="settings-page">
    <h2 class="page-title">系统设置</h2>

    <!-- 基本设置 -->
    <div class="settings-section">
      <h3 class="section-title">基本设置</h3>
      <div class="setting-item">
        <div>
          <div class="setting-label">自动保存</div>
          <div class="setting-desc">排课修改后自动保存到本地数据库</div>
        </div>
        <n-switch v-model:value="settings.autoSave" />
      </div>
      <div class="setting-item">
        <div>
          <div class="setting-label">冲突实时检测</div>
          <div class="setting-desc">拖拽调课时实时检测并高亮冲突</div>
        </div>
        <n-switch v-model:value="settings.realtimeCheck" />
      </div>
    </div>

    <!-- 时间配置 -->
    <div class="settings-section">
      <h3 class="section-title">时间配置</h3>
      <div class="setting-desc" style="margin-bottom:12px">设置全局锁定时段，排课引擎将避免在这些时段安排课程。</div>
      <locked-time-grid />
    </div>

    <!-- 学期管理 -->
    <div class="settings-section">
      <h3 class="section-title">学期管理</h3>
      <div class="setting-desc" style="margin-bottom:12px">
        管理学期信息，排课时使用当前学期。学期第一天用于日期↔星期换算；
        系统会自动推荐"下一个学期"的开学日建议值，你可以直接使用或修改。
      </div>

      <div v-if="semesters.length === 0" class="setting-desc" style="padding:12px 0;">
        暂无学期，请点击下方按钮新增。
      </div>

      <div v-for="sem in semesters" :key="sem.ID" class="setting-item">
        <div>
          <span class="setting-label">
            {{ displayName(sem) }}
            <span v-if="sem.status === STATUS_ACTIVE" style="color: var(--b3-theme-success); font-size: 11px; margin-left: 6px;">● 当前学期</span>
            <span v-else-if="sem.status === STATUS_PLANNED" style="color: var(--b3-theme-on-surface-light); font-size: 11px; margin-left: 6px;">○ 预排</span>
            <span v-else-if="sem.status === STATUS_ARCHIVED" style="color: var(--b3-theme-on-surface-light); font-size: 11px; margin-left: 6px;">□ 已归档</span>
          </span>
          <div class="setting-desc" v-if="sem.startDate">学期第一天：{{ formatDate(sem.startDate) }}</div>
        </div>
        <n-space>
          <n-button size="tiny" @click="openEditSemester(sem)">编辑</n-button>
          <n-button size="tiny" type="error" text @click="deleteSemester(sem.ID)">删除</n-button>
        </n-space>
      </div>

      <div class="setting-item" style="border-top:1px dashed var(--b3-border-color); padding-top:12px;">
        <n-button size="small" type="primary" @click="openNewSemester">+ 新增学期</n-button>
      </div>
    </div>

    <!-- 数据管理 -->
    <div class="settings-section">
      <h3 class="section-title">数据管理</h3>
      <div class="setting-item">
        <div>
          <div class="setting-label">数据库位置</div>
          <div class="setting-desc">SQLite 数据库文件存储路径</div>
        </div>
        <span class="setting-value">scheduling.db</span>
      </div>
      <div class="setting-item">
        <div>
          <div class="setting-label">备份与恢复</div>
          <div class="setting-desc">导出或导入数据库备份文件</div>
        </div>
        <n-space>
          <n-button size="small" @click="handleBackup">导出备份</n-button>
          <n-button size="small" @click="handleRestore">导入</n-button>
        </n-space>
      </div>
    </div>

    <!-- 学期编辑弹窗 -->
    <n-modal v-model:show="showSemesterModal" preset="card" :title="editingSemester ? '编辑学期' : '新增学期'" style="width: 460px;">
      <n-form label-placement="left" label-width="90">
        <n-form-item label="学年" required>
          <n-input v-model:value="semesterForm.academicYear" placeholder="如 2026-2027" />
        </n-form-item>
        <n-form-item label="学期" required>
          <n-radio-group v-model:value="semesterForm.term">
            <n-radio :value="TERM_FIRST">第一学期（秋季）</n-radio>
            <n-radio :value="TERM_SECOND">第二学期（春季）</n-radio>
          </n-radio-group>
        </n-form-item>
        <n-form-item label="学期第一天" required>
          <n-date-picker v-model:value="semesterDateVal" type="date" clearable placeholder="选择开学第一天（建议周一）" />
        </n-form-item>
        <n-form-item label="设为当前学期">
          <n-switch v-model:value="semesterForm.isActive" />
        </n-form-item>
        <div class="setting-desc" style="margin-top:-4px;">
          启用后，其它当前学期会被自动归档。学期结束日 = 第一天 + 18 周，无需手动填写。
        </div>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showSemesterModal = false">取消</n-button>
          <n-button type="primary" @click="saveSemester">保存</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.settings-page { max-width: 820px; }
.page-title { font-size: 18px; font-weight: 600; color: var(--b3-theme-on-background); margin-bottom: 24px; }
.settings-section { background: var(--b3-theme-surface); border: 1px solid var(--b3-border-color); border-radius: var(--b3-border-radius); padding: 20px; margin-bottom: 16px; }
.section-title { font-size: 14px; font-weight: 600; color: var(--b3-theme-on-background); margin-bottom: 16px; padding-bottom: 10px; border-bottom: 1px solid var(--b3-border-color); }
.setting-item { display: flex; justify-content: space-between; align-items: center; padding: 10px 0; }
.setting-item + .setting-item { border-top: 1px solid var(--b3-border-color); }
.setting-label { font-size: 13px; font-weight: 500; color: var(--b3-theme-on-background); }
.setting-desc { font-size: 12px; color: var(--b3-theme-on-surface-light); margin-top: 2px; }
.setting-value { font-size: 12px; color: var(--b3-theme-on-surface); }
</style>
