# 课表 PDF 导出优化 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking。

**Goal:** 让教师/教务人员能一键导出全年级或全教师的课表 PDF，而非手动逐个切换再导出。

**Architecture:** 在现有 `exportSchedulePDF()` 基础上增加批量模式：遍历所有教师/班级，对每个对象切换视角、截图、生成 PDF 页，最终合并为一个多页 PDF 文件。同时添加 `@media print` 样式让用户可以直接 Ctrl+P 打印。

**Tech Stack:** html2canvas, jsPDF (已有), Vue3 Composition API, Naive UI

## Global Constraints

- 不引入新的 npm 依赖（html2canvas + jsPDF 已有）
- 批量导出时复用现有 `exportSchedulePDF` 的截图逻辑
- 导出过程中显示进度，可取消
- 文件名格式：`课表_教师_{日期}.pdf` 或 `课表_班级_{日期}.pdf`

---

### Task 1: 提取截图核心函数

**Files:**
- Modify: `frontend/src/views/SchedulePage.vue:204-310`

**Interfaces:**
- Produces: `captureSchedulePage(title: string): Promise<HTMLCanvasElement>` — 截图当前课表视图并返回 canvas

- [ ] **Step 1: 提取 captureSchedulePage 函数**

把 `exportSchedulePDF` 中的截图逻辑提取为独立函数：

```typescript
/**
 * 截图当前课表视图，返回 canvas。
 * 调用前确保课表已渲染完成。
 */
async function captureSchedulePage(title: string): Promise<HTMLCanvasElement> {
  const grid = document.querySelector('.schedule-grid') as HTMLElement
  if (!grid) throw new Error('课表网格未加载')

  const EXPORT_W = 1400
  const container = document.createElement('div')
  const b3Vars: Record<string, string> = {
    '--b3-theme-background': '#ffffff',
    '--b3-theme-surface': '#f6f6f6',
    '--b3-theme-on-surface': '#333333',
    '--b3-theme-on-background': '#222222',
    '--b3-theme-on-surface-light': '#999999',
    '--b3-border-color': '#e0e0e0',
    '--b3-border-radius': '6px',
    '--b3-border-radius-s': '4px',
    '--b3-theme-primary': '#3575f0',
    '--b3-theme-primary-light': '#5b8af7',
    '--b3-theme-primary-lightest': '#e8f0fe',
    '--b3-theme-error': '#e53935',
  }
  container.style.cssText = `position:fixed;left:-30000px;top:0;width:${EXPORT_W}px;background:#fff;padding:14px 18px 10px;font-family:"Microsoft YaHei","PingFang SC",sans-serif;color:#222;`
  for (const [k, v] of Object.entries(b3Vars)) container.style.setProperty(k, v)

  const dateStr = new Date().toLocaleString()
  const titleEl = document.createElement('div')
  titleEl.style.cssText = 'font-size:20px;font-weight:700;margin-bottom:4px;line-height:1.4;'
  titleEl.textContent = title
  container.appendChild(titleEl)
  const subEl = document.createElement('div')
  subEl.style.cssText = 'font-size:12px;color:#888;margin-bottom:12px;'
  subEl.textContent = `第${scheduleStore.currentWeek}周　生成时间：${dateStr}`
  container.appendChild(subEl)

  const gridClone = grid.cloneNode(true) as HTMLElement
  gridClone.style.setProperty('overflow', 'visible')
  gridClone.style.setProperty('height', 'auto')
  gridClone.style.setProperty('flex', 'none')
  gridClone.querySelector('.week-toolbar')?.remove()
  container.appendChild(gridClone)
  document.body.appendChild(container)

  try {
    return await html2canvas(container, { scale: 3, backgroundColor: '#ffffff' })
  } finally {
    document.body.removeChild(container)
  }
}
```

- [ ] **Step 2: 重构 exportSchedulePDF 使用 captureSchedulePage**

```typescript
async function exportSchedulePDF() {
  exporting.value = true
  try {
    if (scheduleStore.displayEntries.length === 0) {
      const hasSelection = scheduleStore.selectedTeacherId || scheduleStore.selectedClassId
      window.alert(hasSelection ? '当前对象暂无课程' : '当前没有可导出的课表，请先选择教师或班级')
      return
    }

    let title = '课表'
    if (scheduleStore.perspective === 'teacher' && scheduleStore.selectedTeacherId) {
      const t = scheduleStore.displayEntries[0]?.teacher
      title = `${t?.name || '教师'} 课表`
    } else if (scheduleStore.perspective === 'class' && scheduleStore.selectedClassId) {
      title = '班级课表'
    }

    const canvas = await captureSchedulePage(title)
    saveCanvasAsPDF(canvas, title)
  } catch (err: any) {
    window.alert('导出PDF失败：' + (err?.message || err))
  } finally {
    exporting.value = false
  }
}
```

- [ ] **Step 3: 提取 saveCanvasAsPDF 函数**

```typescript
function saveCanvasAsPDF(canvas: HTMLCanvasElement, title: string) {
  const pdf = new jsPDF('l', 'mm', 'a4')
  const pageW = pdf.internal.pageSize.getWidth()
  const pageH = pdf.internal.pageSize.getHeight()
  const imgW = pageW - 14
  const imgH = (canvas.height * imgW) / canvas.width
  const imgData = canvas.toDataURL('image/png')

  let heightLeft = imgH
  let position = 7
  pdf.addImage(imgData, 'PNG', 7, position, imgW, imgH)
  heightLeft -= (pageH - position)
  while (heightLeft > 0) {
    position -= pageH
    pdf.addPage()
    pdf.addImage(imgData, 'PNG', 7, position, imgW, imgH)
    heightLeft -= pageH
  }

  const fileDate = new Date().toISOString().slice(0, 10)
  const hash6 = Math.random().toString(16).slice(2, 8)
  pdf.save(`${title}_${fileDate}_${hash6}.pdf`)
}
```

- [ ] **Step 4: 验证**

- 选择一个教师 → 导出 PDF → 确认文件正常打开、内容正确
- 选择一个班级 → 导出 PDF → 确认文件正常打开、内容正确

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/SchedulePage.vue
git commit -m "refactor: 提取 captureSchedulePage/saveCanvasAsPDF 复用函数"
```

---

### Task 2: 添加批量 PDF 导出功能

**Files:**
- Modify: `frontend/src/views/SchedulePage.vue` — 添加 `batchExportPDF` 函数和菜单项
- Modify: `frontend/src/stores/schedule.ts` — 确认 entries 包含所有教师/班级数据

**Interfaces:**
- Consumes: `captureSchedulePage(title)` from Task 1
- Consumes: `saveCanvasAsPDF(canvas, title)` from Task 1
- Consumes: `scheduleStore.entries` — 全量课表数据
- Consumes: `scheduleStore.perspective` / `selectedTeacherId` / `selectedClassId`

- [ ] **Step 1: 添加批量导出函数**

```typescript
const batchExporting = ref(false)
const batchProgress = ref({ current: 0, total: 0, label: '' })

async function batchExportPDF(mode: 'teacher' | 'class') {
  // 收集所有不重复的教师/班级 ID
  const entries = scheduleStore.entries
  if (entries.length === 0) {
    window.alert('当前没有课表数据')
    return
  }

  const idMap = new Map<number, string>()
  for (const e of entries) {
    if (mode === 'teacher' && e.teacher) {
      idMap.set(e.teacherId, e.teacher.name)
    } else if (mode === 'class') {
      const tt = (e as any).teachingTask
      if (tt?.classes) {
        for (const c of tt.classes) {
          const cgId = c.classGroupId || c.ClassGroupID
          const cgName = c.classGroup?.name || c.ClassGroup?.name || `班级#${cgId}`
          if (cgId) idMap.set(cgId, cgName)
        }
      }
      if (e.classGroupId) {
        idMap.set(e.classGroupId, (e as any).classGroup?.name || `班级#${e.classGroupId}`)
      }
    }
  }

  if (idMap.size === 0) {
    window.alert(mode === 'teacher' ? '没有教师数据' : '没有班级数据')
    return
  }

  // 确认
  const label = mode === 'teacher' ? '教师' : '班级'
  if (!window.confirm(`将导出 ${idMap.size} 个${label}的课表 PDF，是否继续？`)) return

  batchExporting.value = true
  batchProgress.value = { current: 0, total: idMap.size, label: '' }

  const pdf = new jsPDF('l', 'mm', 'a4')
  const pageW = pdf.internal.pageSize.getWidth()
  const pageH = pdf.internal.pageSize.getHeight()
  let firstPage = true

  try {
    for (const [id, name] of idMap) {
      batchProgress.value = { current: batchProgress.value.current + 1, total: idMap.size, label: name }

      // 切换视角
      scheduleStore.setPerspective(mode)
      if (mode === 'teacher') {
        scheduleStore.selectedTeacherId = id
      } else {
        scheduleStore.selectedClassId = id
      }

      // 等待渲染
      await new Promise(r => setTimeout(r, 300))

      // 检查是否有数据
      if (scheduleStore.displayEntries.length === 0) continue

      // 截图
      const canvas = await captureSchedulePage(`${name} 课表`)

      // 添加到 PDF
      const imgW = pageW - 14
      const imgH = (canvas.height * imgW) / canvas.width
      const imgData = canvas.toDataURL('image/png')

      if (!firstPage) pdf.addPage()
      firstPage = false

      // 如果单页放不下，分页
      let heightLeft = imgH
      let position = 7
      pdf.addImage(imgData, 'PNG', 7, position, imgW, imgH)
      heightLeft -= (pageH - position)
      while (heightLeft > 0) {
        position -= pageH
        pdf.addPage()
        pdf.addImage(imgData, 'PNG', 7, position, imgW, imgH)
        heightLeft -= pageH
      }
    }

    const fileDate = new Date().toISOString().slice(0, 10)
    pdf.save(`课表_${label}_${fileDate}.pdf`)
  } catch (err: any) {
    window.alert('批量导出失败：' + (err?.message || err))
  } finally {
    batchExporting.value = false
    batchProgress.value = { current: 0, total: 0, label: '' }
    // 恢复原来的视角选择
    scheduleStore.setPerspective(scheduleStore.perspective)
  }
}
```

- [ ] **Step 2: 更新导出菜单**

```typescript
const combinedExportOptions: any[] = [
  { type: 'group', key: 'excel-header', label: 'Excel（数据）' },
  ...exportOptions.map(o => ({ key: 'excel:' + o.key, label: '　' + o.label })),
  { type: 'divider', key: 'div1' },
  { type: 'group', key: 'pdf-header', label: 'PDF' },
  { key: 'pdf', label: '　当前课表' },
  { key: 'pdf-batch:teacher', label: '　全部教师（批量）' },
  { key: 'pdf-batch:class', label: '　全部班级（批量）' },
]

function handleExportSelect(key: string) {
  if (key === 'pdf') {
    exportSchedulePDF()
  } else if (key === 'pdf-batch:teacher') {
    batchExportPDF('teacher')
  } else if (key === 'pdf-batch:class') {
    batchExportPDF('class')
  } else if (key.startsWith('excel:')) {
    exportSchedule(key.slice(6) as 'teacher' | 'class')
  }
}
```

- [ ] **Step 3: 添加进度提示 UI**

在导出按钮旁边添加进度提示：

```html
<span v-if="batchExporting" class="batch-progress">
  导出中 {{ batchProgress.current }}/{{ batchProgress.total }} {{ batchProgress.label }}
</span>
```

CSS:
```css
.batch-progress {
  font-size: 12px;
  color: var(--b3-theme-primary);
  margin-left: 8px;
}
```

- [ ] **Step 4: 验证**

- 点击导出 → "全部教师（批量）" → 确认弹窗 → 等待完成 → 检查 PDF 包含所有教师课表
- 点击导出 → "全部班级（批量）" → 确认弹窗 → 等待完成 → 检查 PDF 包含所有班级课表
- 某个教师无课时跳过该页，不报错
- 导出过程中显示进度

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/SchedulePage.vue
git commit -m "feat: 批量 PDF 导出 — 一键导出全部教师/班级课表"
```

---

### Task 3: 添加打印优化样式

**Files:**
- Modify: `frontend/src/views/SchedulePage.vue` — 添加 `<style>` 中的 `@media print` 规则

- [ ] **Step 1: 添加 @media print 样式**

在 SchedulePage.vue 的 `<style>` 块末尾添加：

```css
@media print {
  /* 隐藏非课表元素 */
  .schedule-header,
  .schedule-toolbar,
  .perspective-bar,
  .week-toolbar,
  .schedule-page > .sidebar,
  .batch-progress,
  nav,
  .app-header {
    display: none !important;
  }

  /* 课表全宽 */
  .schedule-content {
    width: 100% !important;
    margin: 0 !important;
    padding: 0 !important;
  }

  .schedule-grid {
    width: 100% !important;
    overflow: visible !important;
    height: auto !important;
  }

  /* 打印友好颜色 */
  .schedule-grid {
    --b3-theme-background: #ffffff;
    --b3-theme-surface: #f6f6f6;
    background: #fff;
  }

  /* 分页避免截断 */
  .schedule-grid {
    page-break-inside: avoid;
  }

  /* 打印标题 */
  .schedule-content::before {
    content: attr(data-print-title);
    display: block;
    font-size: 18px;
    font-weight: 700;
    margin-bottom: 8px;
  }
}
```

- [ ] **Step 2: 添加 print-title data 属性**

在 `.schedule-content` 元素上添加 `data-print-title` 属性：

```html
<div class="schedule-content" :data-print-title="printTitle">
```

computed:
```typescript
const printTitle = computed(() => {
  if (scheduleStore.perspective === 'teacher' && scheduleStore.selectedTeacherId) {
    const t = scheduleStore.displayEntries[0]?.teacher
    return `${t?.name || '教师'} 课表`
  }
  if (scheduleStore.perspective === 'class' && scheduleStore.selectedClassId) {
    return '班级课表'
  }
  return '课表'
})
```

- [ ] **Step 3: 验证**

- 选择一个教师 → Ctrl+P → 预览中只显示课表，无工具栏/导航
- 选择一个班级 → Ctrl+P → 预览中只显示课表

- [ ] **Step 4: Commit**

```bash
git add frontend/src/views/SchedulePage.vue
git commit -m "feat: 打印优化 — @media print 样式，Ctrl+P 直接打印课表"
```

---

## 验收标准

### 功能验收

| 场景 | 预期结果 |
|------|---------|
| 单个教师导出 PDF | 文件正常，内容与页面一致 |
| 单个班级导出 PDF | 文件正常，内容与页面一致 |
| 批量教师导出 | 包含所有有课教师的课表，无课教师跳过 |
| 批量班级导出 | 包含所有有课班级的课表，无课班级跳过 |
| 批量导出进度 | 显示 "导出中 3/50 张三" |
| 批量导出取消 | 确认弹窗可取消 |
| Ctrl+P 打印 | 只显示课表，无导航/工具栏 |

### 质量验收

- 导出过程中不阻塞 UI（进度可更新）
- 某个对象截图失败不中断整个批量流程
- 文件名包含日期，便于管理
- 无新 npm 依赖
