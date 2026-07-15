# 资源管理导入系统优化计划

## 背景
当前导入系统存在三个核心问题：
1. 教学任务导入逐行调 API（200行=200次RPC）
2. 其他实体也是逐条调用
3. 没有导入预览，用户无法确认

## 任务

### Task 1: 教学任务批量导入（前端修复）
**文件:** `frontend/src/views\ResourcePage.vue`
**改动:** `handleFileChange` 中的 teachingTask 分支，把所有行收集后一次性传给 `ImportTeachingTasks`
**当前代码:**
```typescript
for (let i = 0; i < dataRows.length; i++) {
  const row = [courseCode, teacherCode, classCodes.join(','), ...]
  const imported = await TS.ImportTeachingTasks(appStore.currentSemesterId || 0, [row]) // ← 每行一次
}
```
**目标代码:**
```typescript
const allRows: string[][] = []
for (const dataRow of dataRows) {
  // ... parse row ...
  allRows.push([courseCode, teacherCode, classCodes.join(','), ...])
}
const [importedCount, errs] = await TS.ImportTeachingTasks(appStore.currentSemesterId || 0, allRows) // ← 一次
```

### Task 2: 其他实体批量导入（前端并行化）
**文件:** `frontend/src/views\ResourcePage.vue`
**改动:** teacher/classroom/course/class 的导入从串行改为 `Promise.allSettled` 并行
**当前代码:**
```typescript
for (let i = 0; i < dataRows.length; i++) {
  await callCreate(tab, item) // ← 串行逐条
}
```
**目标代码:**
```typescript
const results = await Promise.allSettled(dataRows.map(row => callCreate(tab, mapRow(...))))
// 收集成功/失败数
```

### Task 3: 导入预览（前端新增）
**文件:** `frontend/src/views\ResourcePage.vue`
**改动:** 添加预览步骤——解析 Excel 后先展示表格，用户确认后再执行导入
**UI:** 用 NDataTable 展示解析结果，底部显示"确认导入 (N条)" / "取消"按钮
**状态管理:** 添加 `previewData` ref 和 `showPreview` ref

### Task 4: 完整错误列表（前端优化）
**文件:** `frontend/src/views\ResourcePage.vue`
**改动:** 导入完成后用 NDataTable 展示所有错误行（而非只显示前5行）

## 约束
- 不改动后端 API（ImportTeachingTasks 已支持批量）
- 保持现有模板格式不变
- 保持 `buildSchema`/`mapRow`/`isMetaRow` 函数不变
