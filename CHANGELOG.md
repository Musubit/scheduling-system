# Changelog

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
