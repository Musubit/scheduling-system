<script setup lang="ts">
import { ref, h, onMounted, computed } from 'vue'
import { NButton, NDataTable, NCard, NSpace, NInput, NForm, NFormItem, NEmpty, NTag, NPopconfirm } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { DEPARTMENTS } from '../types'
import { GetDepartments, CreateDepartment, UpdateDepartment, DeleteDepartment } from '../../bindings/scheduling-system/backend/services/resourceservice'

// Wails auto-generates the Department model after the Go model is added.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Department = any

const message = useMessage()

// 当前院系列表
const departments = ref<Department[]>([])
// 初始快照（不含 gorm 内部字段，只比较 code+name）
const initialCodes = ref<Set<string>>(new Set())

const loading = ref(false)
const saving = ref(false)

// 新增表单
const newCode = ref('')
const newName = ref('')

const hasUnsavedChanges = computed(() => {
  const current = new Set(departments.value.map((d: Department) => `${d.code}|${d.name}`))
  if (current.size !== initialCodes.value.size) return true
  for (const k of current) {
    if (!initialCodes.value.has(k)) return true
  }
  return false
})

async function loadDepartments() {
  loading.value = true
  let loaded: Department[] = []
  try {
    const data = await GetDepartments()
    if (data && data.length > 0) {
      loaded = data
    }
  } catch (e) {
    console.warn('读取院系列表失败，使用默认院系:', e)
  }
  if (loaded.length === 0) {
    loaded = JSON.parse(JSON.stringify(DEPARTMENTS)) as Department[]
  }
  departments.value = loaded
  initialCodes.value = new Set(loaded.map((d: Department) => `${d.code}|${d.name}`))
  loading.value = false
}

function removeDepartment(id: number, name: string) {
  if (departments.value.length <= 1) {
    message.warning('至少需保留一个院系，无法删除')
    return
  }
  departments.value = departments.value.filter((d: Department) => d.id !== id)
}

function addDepartment() {
  const code = newCode.value.trim()
  const name = newName.value.trim()

  if (!code) {
    message.warning('院系代码不能为空')
    return
  }
  if (!name) {
    message.warning('院系名称不能为空')
    return
  }
  if (departments.value.some((d: Department) => d.code === code)) {
    message.warning(`院系代码「${code}」已存在`)
    return
  }

  // id=0 表示新增，保存时按新增处理
  departments.value = [...departments.value, { id: 0, code, name }]
  newCode.value = ''
  newName.value = ''
  message.success('已添加到列表（点击「保存设置」后才会持久化）')
}

async function saveDepartments() {
  if (departments.value.length === 0) {
    message.warning('院系列表不能为空')
    return
  }
  saving.value = true
  try {
    // 用 code 集合重建初始快照（从数据库读到的数据没有 gorm id=0 的行）
    const original: Department[] = JSON.parse(JSON.stringify(departments.value))
    const currentCodes = new Set(departments.value.map((d: Department) => d.code))

    // 删除已移除的
    for (const dept of original) {
      if (dept.id !== 0 && !currentCodes.has(dept.code)) {
        await DeleteDepartment(dept.id)
      }
    }
    // 插入新增 / 更新已修改的
    for (const dept of departments.value) {
      if (dept.id === 0) {
        await CreateDepartment({ code: dept.code, name: dept.name })
      } else {
        const orig = original.find((d: Department) => d.code === dept.code)
        if (!orig || orig.name !== dept.name) {
          await UpdateDepartment({ id: dept.id, code: dept.code, name: dept.name })
        }
      }
    }
    // 重新加载以获取真实 ID（新生效的 id）
    const data = await GetDepartments()
    departments.value = data || []
    initialCodes.value = new Set(departments.value.map((d: Department) => `${d.code}|${d.name}`))
    message.success('院系设置已保存')
  } catch (e) {
    console.error('保存院系设置失败:', e)
    message.error('保存失败，请重试')
  } finally {
    saving.value = false
  }
}

const columns = [
  {
    title: '院系名称',
    key: 'name',
    render: (row: Department) => h(NTag, { type: 'info', bordered: false }, { default: () => row.name }),
  },
  {
    title: '院系代码',
    key: 'code',
    render: (row: Department) => h('code', { style: 'font-family: monospace; color: var(--b3-theme-on-surface);' }, row.code),
  },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    render: (row: Department) =>
      h(
        NPopconfirm,
        { onPositiveClick: () => removeDepartment(row.id, row.name) },
        {
          trigger: () =>
            h(
              NButton,
              { size: 'small', type: 'error', disabled: departments.value.length <= 1 },
              { default: () => '删除' }
            ),
          default: () => `确定删除「${row.name}」吗？`,
        }
      ),
  },
]

onMounted(() => {
  loadDepartments()
})
</script>

<template>
  <div class="system-management">
    <div class="page-header">
      <h2 class="page-title">院系管理</h2>
      <n-space align="center">
        <n-tag v-if="hasUnsavedChanges" type="warning" :bordered="false">有未保存改动</n-tag>
        <n-button type="primary" :loading="saving" :disabled="!hasUnsavedChanges" @click="saveDepartments">
          保存设置
        </n-button>
      </n-space>
    </div>

    <!-- 现有院系列表 -->
    <n-card title="现有院系" class="section-card" :bordered="true">
      <n-data-table
        :columns="columns"
        :data="departments"
        :row-key="(row: Department) => String(row.id)"
        :loading="loading"
        size="small"
        :pagination="false"
      >
        <template #empty>
          <n-empty description="暂无院系数据" />
        </template>
      </n-data-table>
      <div class="hint">共 {{ departments.length }} 个院系，删除时至少保留 1 个。</div>
    </n-card>

    <!-- 新增院系 -->
    <n-card title="新增院系" class="section-card" :bordered="true">
      <n-form inline :label-width="80" @submit.prevent>
        <n-form-item label="院系代码" required>
          <n-input
            v-model:value="newCode"
            placeholder="如 cs"
            style="width: 160px"
            @keydown.enter.prevent="addDepartment"
          />
        </n-form-item>
        <n-form-item label="院系名称" required>
          <n-input
            v-model:value="newName"
            placeholder="如 计算机学院"
            style="width: 220px"
            @keydown.enter.prevent="addDepartment"
          />
        </n-form-item>
        <n-form-item>
          <n-button type="primary" @click="addDepartment">添加</n-button>
        </n-form-item>
      </n-form>
      <div class="hint">新增后仅进入本地列表，点击「保存设置」才会持久化。</div>
    </n-card>
  </div>
</template>

<style scoped>
.system-management { max-width: 760px; padding: 16px; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.page-title { font-size: 18px; font-weight: 600; color: var(--b3-theme-on-background); margin: 0; }
.section-card { margin-bottom: 16px; }
.hint { font-size: 12px; color: var(--b3-theme-on-surface-light); margin-top: 12px; }
</style>
