<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { NSwitch, NButton, NInput, NInputNumber, NModal, NForm, NFormItem, NSelect, NSpace, NDatePicker } from 'naive-ui'
import { useAppStore } from '../stores/app'

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
interface SemesterData {
  ID: number
  name: string
  isActive: boolean
  startDate: string
}

const semesters = ref<SemesterData[]>([])
const showSemesterModal = ref(false)
const editingSemester = ref<SemesterData | null>(null)
const semesterForm = reactive({ name: '', isActive: false, startDate: '' })

async function loadSemesters() {
  try {
      const { GetSemesters } = await import('../../bindings/scheduling-system/services/resourceservice')
    const result = await GetSemesters()
    semesters.value = (result || []) as SemesterData[]
  } catch (e) {
    console.warn('Failed to load semesters:', e)
  }
}

function openNewSemester() {
  editingSemester.value = null
  semesterForm.name = ''
  semesterForm.isActive = false
  semesterForm.startDate = ''
  showSemesterModal.value = true
}

function openEditSemester(sem: SemesterData) {
  editingSemester.value = sem
  semesterForm.name = sem.name
  semesterForm.isActive = sem.isActive
  semesterForm.startDate = sem.startDate || ''
  showSemesterModal.value = true
}

async function saveSemester() {
  try {
    const { CreateSemester, UpdateSemester } = await import('../../bindings/scheduling-system/services/resourceservice')
    const data = {
      ID: editingSemester.value?.ID || 0,
      name: semesterForm.name,
      isActive: semesterForm.isActive,
      startDate: semesterForm.startDate,
    }
    if (editingSemester.value) {
      await UpdateSemester(data)
    } else {
      await CreateSemester(data)
    }
    await loadSemesters()
    showSemesterModal.value = false
  } catch (e) {
    console.warn('Failed to save semester:', e)
  }
}

async function deleteSemester(id: number) {
  if (!confirm('确定要删除该学期吗？')) return
  try {
    const { DeleteSemester } = await import('../../bindings/scheduling-system/services/resourceservice')
    await DeleteSemester(id)
    await loadSemesters()
  } catch (e) {
    console.warn('Failed to delete semester:', e)
  }
}

onMounted(() => {
  loadSemesters()
})
</script>

<template>
  <div class="settings-page">
    <h2 class="page-title">系统设置</h2>

    <!-- 基本设置 -->
    <div class="settings-section">
      <h3 class="section-title">基本设置</h3>
      <div class="setting-item">
        <div>
          <div class="setting-label">深色模式</div>
          <div class="setting-desc">切换系统界面为深色主题</div>
        </div>
        <n-switch :value="appStore.theme === 'dark'" @update:value="appStore.toggleTheme()" />
      </div>
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

    <!-- 学期管理 -->
    <div class="settings-section">
      <h3 class="section-title">学期管理</h3>
      <div class="setting-desc" style="margin-bottom:12px">管理学期信息，排课时使用当前激活的学期。学期第一天用于确定日期-星期对应关系。</div>
      
      <div v-for="sem in semesters" :key="sem.ID" class="setting-item">
        <div>
          <span class="setting-label">
            {{ sem.name }}
            <span v-if="sem.isActive" style="color: var(--b3-theme-success); font-size: 11px; margin-left: 6px;">● 当前学期</span>
          </span>
          <div class="setting-desc" v-if="sem.startDate">学期第一天：{{ sem.startDate }}</div>
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
          <n-button size="small">导出备份</n-button>
          <n-button size="small">导入</n-button>
        </n-space>
      </div>
    </div>

    <!-- 学期编辑弹窗 -->
    <n-modal v-model:show="showSemesterModal" preset="card" :title="editingSemester ? '编辑学期' : '新增学期'" style="width: 420px;">
      <n-form label-placement="left" label-width="100">
        <n-form-item label="学期名称">
          <n-input v-model:value="semesterForm.name" placeholder="如 2025-2026 第二学期" />
        </n-form-item>
        <n-form-item label="学期第一天">
          <n-input v-model:value="semesterForm.startDate" placeholder="如 2025-09-01" />
        </n-form-item>
        <n-form-item label="设为当前学期">
          <n-switch v-model:value="semesterForm.isActive" />
        </n-form-item>
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
.settings-page { max-width: 680px; }
.page-title { font-size: 18px; font-weight: 600; color: var(--b3-theme-on-background); margin-bottom: 24px; }
.settings-section { background: var(--b3-theme-surface); border: 1px solid var(--b3-border-color); border-radius: var(--b3-border-radius); padding: 20px; margin-bottom: 16px; }
.section-title { font-size: 14px; font-weight: 600; color: var(--b3-theme-on-background); margin-bottom: 16px; padding-bottom: 10px; border-bottom: 1px solid var(--b3-border-color); }
.setting-item { display: flex; justify-content: space-between; align-items: center; padding: 10px 0; }
.setting-item + .setting-item { border-top: 1px solid var(--b3-border-color); }
.setting-label { font-size: 13px; font-weight: 500; color: var(--b3-theme-on-background); }
.setting-desc { font-size: 12px; color: var(--b3-theme-on-surface-light); margin-top: 2px; }
.setting-value { font-size: 12px; color: var(--b3-theme-on-surface); }
</style>
