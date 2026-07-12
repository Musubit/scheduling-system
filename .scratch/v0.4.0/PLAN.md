# v0.4.0 - Experience & Extensibility

## 版本定位

在 Stable Core 冻结的前提下，消除 v0.3 遗留技术债，提升用户体验，
增强系统可扩展性。不修改评分算法、快照核心字段、ScoringContext 接口。

---

## Epic 总览

| Epic | 优先级 | 内容 | 影响 |
|------|--------|------|------|
| A - Technical Debt | P0 | 消除 5 项遗留重复代码 | 内部质量，用户无感 |
| B - Global State & Navigation | P0 | 全局学期上下文 + 导航统一 | 用户可见 |
| C - Snapshot Management | P1 | 快照重命名 + Trigger 扩展 | 用户可见 |
| D - Constraint System | P1 | 约束配置 UI 优化 + 扩展预留 | 用户可见 |
| E - Settings & Data | P2 | 设置页增强 + 时间配置 | 用户可见 |
| F - Scheduling UX | P2 | 排课页面交互优化 | 用户可见 |

---

## Epic B - Global State & Navigation (P0)

### 问题

当前系统存在多处独立的学期状态，互不同步：

| 页面 | 学期来源 | 类型 | 问题 |
|------|----------|------|------|
| AppToolbar 下拉 | `appStore.semesterFilter` | string | 仅 schedule 页显示 |
| ResourcePage | `GetActiveSemester()` 一次性 | local ref | 始终锁定 active 学期，无切换 |
| SchedulingPage | `store.selectedSemesterId` | number | 独立状态，与全局不同步 |
| ReportPage | `appStore.semesterFilter` | string | 继承全局，无独立切换 |
| HistoryComparePage | `appStore.semesterFilter` | string | 同上 |

用户切换学期后不同页面显示不同学期数据，造成"教学任务未按学期隔离"的误解。

### 设计目标

建立 **全局唯一 Current Semester**，遵循 Single Source of Truth：

```
appStore.currentSemesterId (number)  ← 唯一来源
         │
    ┌────┴────┬────────┬──────────┬──────────────┐
    ▼         ▼        ▼          ▼              ▼
 SchedulePage  ResourcePage  SchedulingPage  ReportPage  HistoryComparePage
```

刷新逻辑封装在 Store 内（`setCurrentSemester` 方法），不在 App.vue 集中增加 watch。

### B1: appStore 统一学期状态

**现状：** `appStore` 有 `semesterFilter`（string）和 `semesters`（列表），无 numeric ID。各页面各自调 `GetActiveSemester()` 获取 ID。

**方案：**

```typescript
// stores/app.ts
const currentSemesterId = ref<number>(0)
const currentSemesterName = computed(() =>
  semesters.value.find(s => s.ID === currentSemesterId.value)?.name || ''
)
const currentSemester = computed(() =>
  semesters.value.find(s => s.ID === currentSemesterId.value) || null
)
```

- `initSemester()` 用 `GetActiveSemester()` 设置 `currentSemesterId`（仅初始化）
- 保留 `semesterFilter`（string）作为兼容别名，内部同步 `currentSemesterName`
- 新增 `setCurrentSemester(id: number)` 方法，内部触发相关 store 刷新

**文件：** `frontend/src/stores/app.ts`

### B2: 工具栏统一学期切换器

**现状：** `AppToolbar.vue` 学期下拉 `v-if="currentPage === 'schedule'"`，仅课表页显示。

**方案：**
- 移除 `v-if` 条件，所有页面均显示
- 下拉绑定 `appStore.currentSemesterId`（number），options 来自 `appStore.semesters`
- 切换时调用 `appStore.setCurrentSemester(id)`

**文件：** `frontend/src/components/layout/AppToolbar.vue`

### B3: Store 内封装刷新逻辑

**现状：** `App.vue` 的 `watch(semesterFilter)` 仅刷新 `scheduleStore.loadSchedule`。

**方案：** 在 `appStore.setCurrentSemester(id)` 内部统一触发刷新：

```typescript
function setCurrentSemester(id: number) {
  currentSemesterId.value = id
  const name = currentSemesterName.value
  // 课表
  useScheduleStore().loadSchedule(name)
  // 教学任务
  useResourceStore().loadTeachingTasks(id)
  // 后续可扩展：Report / History 等
}
```

- 不在 `App.vue` 增加 watch
- 后续页面接入只需在此方法中增加一行

**文件：** `frontend/src/stores/app.ts`（可能需要动态 import 其他 store 避免循环依赖）

### B4: ResourcePage 改用全局学期

**现状：** `ResourcePage.vue` 自己调 `GetActiveSemester()` 获取 `activeSemester`，无 watcher。

**方案：**
- 删除本地 `activeSemester` ref
- `onMounted` 改用 `appStore.currentSemesterId`
- 教学任务 CRUD 后的刷新改用 `appStore.currentSemesterId`
- 教学任务表格增加"学期"列，显示 `appStore.currentSemesterName`

**文件：** `frontend/src/views/ResourcePage.vue`

### B5: SchedulingPage 改用全局学期

**现状：** `SchedulingPage.vue` 使用 `store.selectedSemesterId`（scheduling store 独立状态）。

**方案：**
- scheduling store 的 `selectedSemesterId` 改为 computed 代理 `appStore.currentSemesterId`
- SchedulingPage 下拉绑定 `appStore.currentSemesterId`
- 删除 scheduling store 中 `selectedSemesterId`、`activeSemesterId`、`activeSemesterName`、`semesters`、`loadActiveSemester()` 等冗余状态

**文件：** `frontend/src/stores/scheduling.ts`、`frontend/src/views/SchedulingPage.vue`

### B6: 当前学期视觉标识

**现状：** 除排课页外，无页面显示当前操作的是哪个学期。

**方案：**
- AppToolbar 学期下拉旁增加固定标签"当前学期"
- ResourcePage 教学任务区域顶部显示 `当前学期：{{ appStore.currentSemesterName }}`
- ReportPage / HistoryComparePage 已通过快照 semester 字段显示，无需额外标识

**文件：** `frontend/src/components/layout/AppToolbar.vue`、`frontend/src/views/ResourcePage.vue`

### 交互流程

```
用户在工具栏切换学期
  ↓
appStore.setCurrentSemester(id)  ← 唯一入口
  ├──> scheduleStore.loadSchedule(name)     课表刷新
  ├──> resourceStore.loadTeachingTasks(id)  教学任务刷新
  └──> schedulingStore 自动同步             排课页同步（computed 代理）
  ↓
所有页面显示新学期数据
工具栏标签更新为新学期名称
```

### 不涉及

- 后端 API 不变（`ListTeachingTasks(semesterID)` 签名不变）
- 数据库结构不变
- Stable Core 不变
- `GetActiveSemester()` 仅用于初始化默认学期

---

## Epic A - Technical Debt (P0)

### A1: Backup/Restore 提取公共函数

**现状：** `resource_service.go` 中 `BackupDatabase` 和 `RestoreDatabase` 各自重复 `os.Open` / `os.Create` / `io.Copy` 逻辑。

**方案：**
```go
// copyFile copies src to dst, creating parent dirs as needed.
func copyFile(src, dst string) error
```
`BackupDatabase` / `RestoreDatabase` 调用 `copyFile`，各自仅保留路径拼接。

**文件：** `backend/services/resource_service.go`

### A2: 导航数据单一来源

**现状：** 导航树在 `AppSidebar.vue`（含 `resourceTab`）和 `stores/app.ts`（不含 `resourceTab`）各定义一份。

**方案：**
- `stores/app.ts` 的 `navGroups` 增加 `resourceTab` 字段，成为唯一来源
- `AppSidebar.vue` 删除本地 `navGroups`，改用 `appStore.navGroups`
- NavItem 接口增加 `resourceTab?: ResourceType` 字段

**文件：** `frontend/src/stores/app.ts`、`frontend/src/components/layout/AppSidebar.vue`

### A3: DEFAULT_LOCKED 单一来源

**现状：** `WeekView.vue:19-21` 和 `stores/scheduling.ts:9-11` 各定义一份 `DEFAULT_LOCKED`。

**方案：**
- `stores/scheduling.ts` 保留 `DEFAULT_LOCKED` 作为唯一来源并 export
- `WeekView.vue` import 使用，删除本地定义

**文件：** `frontend/src/stores/scheduling.ts`、`frontend/src/components/schedule/WeekView.vue`

### A4: fuzzyFilter 类型优化

**现状：** 三个文件各自 `import { fuzzyFilter }` 后 `const fuzzyFilterFn = fuzzyFilter as any`。

**方案：**
- `fuzzyFilter.ts` 导出类型正确的 `fuzzyFilterFn` 适配版（签名匹配 Naive UI `Filter` 类型）
- 三个调用文件直接 import `fuzzyFilterFn`，删除本地 `as any` 转换

**文件：** `frontend/src/utils/fuzzyFilter.ts`、`SchedulePage.vue`、`SchedulingPage.vue`、`ResourcePage.vue`

### A5: ScoringService 构造统一

**现状：** 4 处混用 `(&ScoringService{}).ScoreSchedule(...)` 和 `NewScoringService().ScoreSchedule(...)`。

**方案：** 全部统一为 `NewScoringService()`。

**文件：** `backend/services/scheduling_service.go`、`backend/services/sa_neighbors.go`、`backend/services/snapshot_service.go`

---

## Epic B - Snapshot Management (P1)

### B1: 快照重命名 UI

**现状：** `ScheduleSnapshot.Name` 字段已存在（v0.3.2），后端 `DisplayName()` / `DefaultSnapshotName()` 已就绪。前端只读。

**方案：**
- ReportPage 侧边栏快照卡片增加"重命名"入口（hover 显示铅笔图标）
- 弹出 `NModal` + `NInput` 编辑 Name
- 后端新增 `SnapshotService.RenameSnapshot(id uint, name string) error`
- 前端调用后刷新列表

**交互流程：**
```
侧边栏 hover 快照卡片 -> 铅笔图标出现 -> 点击 -> 弹窗输入新名称 -> 保存 -> 列表刷新
```

**文件：** `backend/services/snapshot_service.go`、`frontend/src/views/ReportPage.vue`

**待解决：**
- 重命名后是否影响 HistoryComparePage 的下拉显示？-> 不影响，两页都读 `snap.name`
- 是否限制名称长度？-> 前端限制 100 字符（与 GORM size 一致）

### B2: Trigger 类型扩展预留

**现状：** `models.TriggerLabel()` 已用 switch 实现，支持 `auto` / `manual`。

**方案：** 本期不新增 Trigger 类型，但验证现有设计可扩展：
- `TriggerLabel()` 增加 `import` / `restore` / `copy` case（返回中文标签，无实际逻辑）
- 前端 `triggerLabel` 显示统一从后端获取（当前前端已删除 `triggerLabel` 函数，ReportPage/HistoryComparePage 直接用 `snap.name`）

**文件：** `backend/models/snapshot.go`（仅扩展 switch case）

---

## Epic C - Constraint System (P1)

### C1: 约束配置 UI 优化

**现状：** `SchedulingPage.vue` 约束开关为平铺 checkbox，权重滑块在折叠面板内。

**方案：**
- 约束列表改为卡片式布局：每个约束一行，包含开关 + 权重滑块 + 说明
- 移除折叠面板，直接展示（减少操作步骤）
- 约束分组：硬约束（只读） / 软约束（可调权重）

**布局设计：**
```
┌─────────────────────────────────────────┐
│ 约束方案: [均衡（推荐） ▼]               │
├─────────────────────────────────────────┤
│ ☑ 教师偏好时段     [━━━━●━━] 50        │
│   避免教师排在不偏好时段                 │
├─────────────────────────────────────────┤
│ ☑ 课程分散度       [━━━━●━━] 50        │
│   课程均匀分布在一周内                   │
├─────────────────────────────────────────┤
│ ☐ 体育课时段       [━━━━●━━] 50  禁用  │
│   体育课优先3-4节或7-8节                 │
└─────────────────────────────────────────┘
```

**文件：** `frontend/src/views/SchedulingPage.vue`

### C2: 约束体系扩展预留

**现状：** 约束 key 为硬编码字符串，`constraintOptions` 在 store 中定义。

**方案：** 不新增约束，但重构 `constraintOptions` 为数据驱动：
- 每个约束定义 `key` / `label` / `description` / `group`（hard/soft）/ `defaultWeight`
- 新增约束只需在数组中添加一项 + 后端实现评分方法

**文件：** `frontend/src/stores/scheduling.ts`

---

## Epic D - Settings & Data (P2)

### D1: 设置页增强

**现状：** SettingsPage 有基本设置、学期管理、数据管理三个区块。

**方案：**
- 数据管理区块增加"清空所有数据"按钮（带二次确认）
- 学期管理增加"学期复制"功能（复制学期 + 关联教学任务）
- 基本设置增加"默认排课参数"（迭代次数、时间限制）

**文件：** `frontend/src/views/SettingsPage.vue`、`backend/services/resource_service.go`

### D2: 可用时间配置

**现状：** 锁定时段（DEFAULT_LOCKED）硬编码为周四 5-8 节，用户通过 WeekView 点击格子锁定。

**方案：**
- SettingsPage 新增"时间配置"区块
- 可视化时间网格：7天 × 10节，点击切换锁定/解锁
- 配置保存到 `settings` 表，前端从后端读取
- WeekView 和 scheduling.ts 统一从 store 读取锁定配置

**交互流程：**
```
设置页 -> 时间配置 -> 7×10 网格 -> 点击格子切换 -> 保存 -> WeekView 同步
```

**文件：** `frontend/src/views/SettingsPage.vue`、`backend/services/resource_service.go`、`frontend/src/stores/scheduling.ts`

**待解决：**
- 是否支持按学期配置不同锁定时段？-> 暂不，全局配置即可
- 午休/晚饭时段是否在此配置？-> 暂不，保持硬编码

---

## Epic E - Scheduling UX (P2)

### E1: 排课页面布局优化

**现状：** 左右分栏（配置 | 结果），配置面板占比较大。

**方案：**
- 配置面板可折叠（默认展开，排课开始后自动折叠）
- 结果区域扩展为全宽，显示更多统计信息
- 排课进度阶段反馈（v0.3.0 已实现）保留

**文件：** `frontend/src/views/SchedulingPage.vue`

### E2: 排课结果对比

**现状：** 排课完成后直接跳转课表，无法快速对比"排课前 vs 排课后"。

**方案：**
- 排课完成后结果区域显示"本次排课摘要"
- 增加"对比上次"按钮，跳转 HistoryComparePage 并自动选中最新两个快照

**文件：** `frontend/src/views/SchedulingPage.vue`、`frontend/src/views/HistoryComparePage.vue`

---

## 优先级与执行顺序

```
第一轮: Epic A (技术债清理，无 UI 风险)
  A1 -> A2 -> A3 -> A4 -> A5

第二轮: Epic B (快照管理)
  B1 -> B2

第三轮: Epic C (约束系统)
  C1 -> C2

第四轮: Epic D + E (并行)
  D1 + D2  |  E1 + E2
```

---

## 风险矩阵

| 风险 | 等级 | 缓解 |
|------|------|------|
| A2 导航数据统一可能遗漏 resourceTab 路由 | 中 | 回归测试所有导航跳转 |
| A3 锁定配置迁移可能丢失用户自定义 | 低 | localStorage 读取兼容旧格式 |
| B1 重命名后 PDF 导出标题需同步 | 低 | PDF 已使用 snap.name |
| C1 约束 UI 重构影响排课流程 | 中 | 保持约束 key 和 store 接口不变 |
| D2 时间配置改动影响锁定时段逻辑 | 中 | 充分测试 WeekView 锁定交互 |

---

## 不在 v0.4.0 范围

- 评分算法修改（Stable Core 冻结）
- 新增软约束维度（v0.5.x）
- AI 辅助排课（v0.5.x）
- 数据分析报表（v0.7.x）
- 插件化（v0.8.x）
