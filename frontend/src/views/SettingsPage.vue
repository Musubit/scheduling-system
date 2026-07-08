<script setup lang="ts">
import { reactive, watch, ref } from 'vue'
import { NSwitch, NButton, NInputNumber, NSelect, NSpace, NTag } from 'naive-ui'
import { useAppStore } from '../stores/app'
import { DAY_NAMES } from '../types'

const appStore = useAppStore()

// Settings reactive object
const settings = reactive({
  autoSave: true,
  realtimeCheck: true,
  iterations: 5000,
  maxDailyHours: 8,
  bufferMinutes: 10,
})

// Load from localStorage
const saved = localStorage.getItem('scheduling-settings')
if (saved) {
  try { Object.assign(settings, JSON.parse(saved)) } catch {}
}

// Auto-save to localStorage on change
watch(settings, (val) => {
  localStorage.setItem('scheduling-settings', JSON.stringify(val))
}, { deep: true })

function resetDefaults() {
  Object.assign(settings, {
    autoSave: true,
    realtimeCheck: true,
    iterations: 5000,
    maxDailyHours: 8,
    bufferMinutes: 10,
  })
  alert('已恢复默认设置')
}

// ===== Locked Time Slots =====
interface LockedSlot {
  dayOfWeek: number
  startPeriod: number
  span: number
}

const lockedSlots = ref<LockedSlot[]>([])
const newSlot = reactive<LockedSlot>({ dayOfWeek: 3, startPeriod: 4, span: 2 })

// Load locked slots from backend
async function loadLockedSlots() {
  try {
    const { GetSettings } = await import('../../bindings/scheduling-system/services/resourceservice')
    // Read from Setting table via resource service (simplified)
    // For now, load from localStorage
    const saved = localStorage.getItem('locked-time-slots')
    if (saved) {
      try { lockedSlots.value = JSON.parse(saved) } catch {}
    }
  } catch {}
}

function addLockedSlot() {
  lockedSlots.value.push({ ...newSlot })
  saveLockedSlots()
}

function removeLockedSlot(index: number) {
  lockedSlots.value.splice(index, 1)
  saveLockedSlots()
}

async function saveLockedSlots() {
  localStorage.setItem('locked-time-slots', JSON.stringify(lockedSlots.value))
  // Persist to backend Setting table
  try {
    const { SaveSetting } = await import('../../bindings/scheduling-system/services/resourceservice')
    await SaveSetting('locked_time_slots', JSON.stringify(lockedSlots.value))
  } catch {
    console.warn('Failed to save locked slots to backend, using localStorage only')
  }
}

loadLockedSlots()

const periodLabels = [
  { value: 0, label: '第1节 (08:20)' },
  { value: 2, label: '第3节 (10:15)' },
  { value: 4, label: '第5节 (14:00)' },
  { value: 6, label: '第7节 (15:55)' },
  { value: 8, label: '第9节 (18:30)' },
]

const dayOptions = DAY_NAMES.map((name, i) => ({ value: i, label: name }))

const spanOptions = [
  { value: 1, label: '1节' },
  { value: 2, label: '2节' },
  { value: 3, label: '3节' },
]
</script>

<template>
  <div class="settings-page">
    <h2 class="page-title">系统设置</h2>

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

    <div class="settings-section">
      <h3 class="section-title">排课参数</h3>
      <div class="setting-item">
        <div>
          <div class="setting-label">默认算法迭代次数</div>
          <div class="setting-desc">自动排课时的遗传算法迭代轮数</div>
        </div>
        <n-input-number v-model:value="settings.iterations" :min="100" :max="50000" size="small" style="width:120px" />
      </div>
      <div class="setting-item">
        <div>
          <div class="setting-label">每日最大课时</div>
          <div class="setting-desc">单个班级每天最多安排的课时数</div>
        </div>
        <n-input-number v-model:value="settings.maxDailyHours" :min="4" :max="12" size="small" style="width:120px" />
      </div>
      <div class="setting-item">
        <div>
          <div class="setting-label">教室缓冲时间</div>
          <div class="setting-desc">两节课之间教室的间隔分钟数</div>
        </div>
        <n-select
          v-model:value="settings.bufferMinutes"
          :options="[{label:'0分钟',value:0},{label:'10分钟',value:10},{label:'15分钟',value:15},{label:'20分钟',value:20}]"
          size="small"
          style="width:120px"
        />
      </div>
    </div>

    <div class="settings-section">
      <h3 class="section-title">锁定时间段</h3>
      <div class="setting-desc" style="margin-bottom:12px">设置全校不可排课的时间段（如周四下午全院会议），排课引擎将跳过这些时段。</div>
      
      <!-- Existing locked slots -->
      <div v-for="(slot, idx) in lockedSlots" :key="idx" class="setting-item">
        <div>
          <span class="setting-label">{{ DAY_NAMES[slot.dayOfWeek] }} {{ slot.startPeriod === 0 ? '1-2' : slot.startPeriod === 2 ? '3-4' : slot.startPeriod === 4 ? '5-6' : slot.startPeriod === 6 ? '7-8' : '9-11' }}节</span>
        </div>
        <n-button size="tiny" type="error" text @click="removeLockedSlot(idx)">移除</n-button>
      </div>
      
      <!-- Add new locked slot -->
      <div class="setting-item" style="border-top:1px dashed var(--b3-border-color); padding-top:12px;">
        <n-space align="center" :wrap="false">
          <n-select v-model:value="newSlot.dayOfWeek" :options="dayOptions" size="tiny" style="width:80px" />
          <n-select v-model:value="newSlot.startPeriod" :options="periodLabels" size="tiny" style="width:130px" />
          <n-select v-model:value="newSlot.span" :options="spanOptions" size="tiny" style="width:70px" />
          <n-button size="tiny" type="primary" @click="addLockedSlot">添加</n-button>
        </n-space>
      </div>
    </div>

    <div class="settings-section">
      <h3 class="section-title">数据管理</h3>
      <div class="setting-item">
        <div>
          <div class="setting-label">数据库位置</div>
          <div class="setting-desc">SQLite 数据库文件存储路径</div>
        </div>
        <span class="setting-value">~/scheduling/data.db</span>
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
      <div class="setting-item">
        <div>
          <div class="setting-label">恢复默认设置</div>
          <div class="setting-desc">将所有设置恢复为出厂默认值</div>
        </div>
        <n-button size="small" @click="resetDefaults">恢复默认</n-button>
      </div>
    </div>
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
