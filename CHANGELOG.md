# Changelog

## [0.3.3] - 2026-07-11

### Removed

- **生产版移除种子数据**：`-tags production` 构建时 `SeedData` 为空操作，首次启动不再自动插入 19 教师/11 教室/35 课程/13 班级/35 教学任务/22 Demo 课表。Dev 模式种子数据照常保留。

### Fixed

- **构建系统**：修复 `go build` 裸编译缺少 Windows 资源嵌入（图标/版本信息），新增加 `build/windows/version.rc` + `windres` 构建链路

---

## [0.3.2] - 2026-07-11 — Final Stabilization

> v0.3 系列最终稳定版本。统一核心数据来源，建立 Stable Core 基线。
> 除严重 Bug 外，ScoreSchedule / ScoreBreakdown / ScoringContext / ScheduleSnapshot / Snapshot.Name 在 v0.4.x 不再修改。

### Fixed

- **评分一致性**：CreateManualSnapshot 修复 TeachingTasks 传入 nil 导致 `student_fatigue` 不参与评分的 Bug
- **显示一致性**：教师评分明细摘要重复"周"前缀（"周周一3-4节" → "周一 3-4节"）
- **静态资源**：favicon 从 325KB PNG 替换为 15KB 多分辨率 .ico，消除 404

### Added

- **ScoreBreakdown 新增字段**：`PerCategoryMax`（每分类满分）、`EnabledCategoryCount`（评分类别数），Go 后端统一计算
- **ScheduleSnapshot 新增字段**：`PerCategoryMax`、`EnabledCategoryCount`、`Name`，数据模型完善
- **Snapshot.Name 机制**：快照创建时自动命名（`自动排课 · 2026-07-11 14:30:05`），`GetSnapshots` 存量回填，前端各页面统一使用

### Changed

- **perCategoryMax 统一来源**：ReportPage 和 SchedulingPage 删除独立的 perCategoryMax 计算逻辑（共 27 行），改为读取后端计算的字段
- **历史快照命名统一**：HistoryComparePage 下拉、ReportPage 侧边栏、PDF 导出、打印头均使用 `snapshot.Name`

### Removed

- 暗色模式完全移除（#0.3.1 收尾）

---

## [0.3.1] - 2026-07-11

### Removed

- **移除暗色模式**：删除深色/浅色主题切换功能，系统仅保留亮色主题
  - 移除 `stores/app.ts` 中的 `theme` 状态、`toggleTheme()`、`setTheme()` 及 localStorage 持久化
  - 移除 `App.vue` 中的 `darkTheme` 导入、`isDark` 计算属性、`themeOverrides` 三元条件
  - 移除 `AppToolbar.vue` 中的主题切换按钮（太阳/月亮图标）
  - 移除 `SettingsPage.vue` 中的"深色模式"设置项
  - 移除 `ziwu.css` 中 `[data-theme="dark"]` 覆盖块（85行）
  - 移除 `TimelineView.vue` 中暗色主题适配 CSS

### Changed

- vendor-vue chunk 从 730 kB 缩减至 682 kB（-48 kB，移除 darkTheme）
- index.css 从 31.8 kB 缩减至 28.8 kB（-3 kB）

### Fixed

- 修复移除 `watch` 导入导致 App 白屏的回归问题

---

## [0.3.0] - 2026-07-11

### Added

- **排课质量报告增强**：总评等级（A+/A/B/C/D）、分项星级评分（★★★★★半星支持）、低于阈值的自动改善建议
- **教师负载分析**：ReportPage 新增教师负载分析卡片，展示每日分布、均衡评分、优化建议（纯后处理，不影响求解器）
- **排课过程阶段反馈**：求解过程可视化（初始化→加载任务→求解→冲突分析→保存→完成），NSteps 动态阶段列表
- **course_dispersed 周内均衡**：SC4 软约束新增日间均衡惩罚项（OR-Tools `AddMaxEquality` + SA `balancePenalty`），鼓励课程均匀分布

### Changed

- **求解器稳定性**：`lastNeighbor` 从包级变量迁移到 `schedulingContext` 字段，消除不可重入风险
- **教室类型判断统一**：新建 `roomtype.go`（`IsLabCourse` / `IsComputerCourse`），4 处大小写不一致的判断统一为公共函数
- **ReportPage 重构**：硬编码分项进度条改为 `categoryDefs` 数据驱动，支持动态扩展新约束分类

### Removed

- 移除 `containsKeyword` 死函数、`strings` 无用导入（3 个 SA 文件 + `scheduling_service.go`）
- 移除 SchedulingPage 硬编码 5 步骤和 `currentStep` 死代码

---

## [0.2.0] - 2026-07-11

### Fixed

- 修复 OR-Tools 求解器诊断输出中教室容量计算公式错误（`total_demand * periods` → `n_rooms * periods`）
- 修复 OR-Tools 求解器裸 `except:` 可能吞噬 `KeyboardInterrupt` / `SystemExit` 信号的问题
- 修复 Naive UI `primaryColorSuppl` 错误设置为橙色而非蓝色系的问题
- 修复 `App.vue` 初始化加载失败时 `appLoading` 永久为 true 导致加载覆盖层无法消失
- 修复 `ReportPage.vue` 模块级 `result` ref 与局部变量 `result` 的名称遮蔽
- 修复 `SchedulingPage.vue` watcher 在 `onMounted` 内注册、组件重新挂载时 watcher 累积泄露
- 修复时间线视图午休标签 12:00 → 11:50 对齐下课时间
- 修复时间线视图 header 布局重构、统一刻度并消除色条侵入
- 修复时间线视图 break 标签上移 + 色条 border-box 防溢出
- 修复时间线视图 — 消除午休/晚饭重复标签 + 修复红线冻结
- 修复体育馆/操场未支持无限容量和多班共用的问题
- 修复分数显示精度（ReportPage / HistoryComparePage）+ 周次越界（schedule.ts displayEntries 过滤）
- 修复 OR-Tools 崩溃、教学任务更新冲突、周次越界
- 修复教学任务编辑 bug + 完善时间参数透传
- 修复非专业课错误分配到专业教室（体育馆/实验室/机房）的问题
- 修复验证报告百分比布局问题

### Changed

- `course_dispersed` 软约束评分算法升级为日期间隔感知，惩罚连续日期聚类
- 排课诊断增强 + 教师时间网格 UI + 健壮性修复
- 侧边栏折叠功能
- 排课系统增强：多会话支持、教室类型分配、权重配置、院系表重构

### Refactor

- 死代码清理（ScoreBar.vue、dept-colors.css、未使用 import、未使用变量/ref/computed、未使用类型定义、未使用 CSS）
- 项目结构重构为 `backend/` + `scheduler/` + 功能增强
- SA 求解器拆分为 4 文件
- CSS 院系颜色去重 + ScoreBar 组件抽离
- PostOptimize 洗牌 bug 修复 + 死字段清理 + MoveService 解耦
- CP-SAT 锁定时段建模简化

### Architecture

- OR-Tools Python 微服务完整硬约束 + 软约束体系
- 模拟退火引擎替换随机漫步排课
- 快照系统（ScheduleSnapshot + SnapshotDetail）实现
- CheckMove API + MoveService 实现
- 锁定时段双来源合并 + 前端错误处理 + 单元测试
- ADR-0004 教学任务实体 + ADR-0005 多引擎架构
- 三条软约束主动优先算法（教师偏好、课程分散度、低楼层优先）
- 学期不再硬编码，统一从数据库读取激活学期
- 数据库接口化 + 命名类型规范化

---

## [0.1.0] - Unreleased

Initial development versions prior to formal versioning.
