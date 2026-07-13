# DECISION_LOG.md — 关键决策记录

> **最后更新**: 2026-07-13

---

## 2026-07-13 — v0.5.5 Phase 1 Semester 领域 FK 化

**决策**: 将 Semester 从"三重用途字符串 Name"重构为结构化字段（AcademicYear + Term + StartDate:time.Time + EndDate + Status）；`ScheduleEntry.Semester` / `ScheduleSnapshot.Semester` 由字符串迁移为 `SemesterID uint` FK

**理由**: 旧模型 `Name` 同时承担显示名、唯一约束、查找键三个职责——改名即破坏全链路；`IsActive` 布尔无法表达"按时间自动判定当前学期"；`StartDate string` 前端被迫伪计算日期

**范围**: 仅 Phase 1（模型 + FK + Seed + Bindings）。AcademicTerm / TeachingWeek / Holiday / Solver 感知 / 前端 `s.name`/`s.isActive` 清理**均延后**至 Phase 2 / Phase 3

**兼容性**: 破坏性 schema 变更；无生产数据，首次升级删除旧 db 重建

---

## 2026-07-13 — v0.5.4 删除 TeachingTask 自动合并推断

**决策**: 移除 `teaching_task_service.go` 中基于 `(courseId, teacherId, semesterId)` 的自动合班推断（-139 行），完全依赖 `TeachingTaskClass` 显式关联

**理由**: 自动推断在合班场景下产生误合并；显式关联更清晰、更可预测；配合 ADR-0004（TeachingTask 实体化）意图

**影响**: 前端 `ResourcePage.vue` 移除 -94 行相关 UI；教学任务创建/编辑仅呈现显式多班关联

---

## 2026-07-13 — v0.5.4 Seed 幂等性修复

**决策**: `SeedData` 从 `Count + Create` 迁移到 `FirstOrCreate`

**理由**: `Count + Create` 存在 GORM 错误状态传播风险——第一次 Create 因唯一约束失败后，同一 session 的后续 Count 也会返回错误，导致 seed 中途放弃；`FirstOrCreate` 是原子操作，天然幂等

**详见**: `docs/release/SEED_IDEMPOTENCY_FIX_REPORT.md`

---

## 2026-07-12 — v0.5.3 统一资源匹配框架 (URMF, ADR-0006)

**决策**: 建立 `ResourceMatcher` 纯函数核心，SA / OR-Tools / MoveService 三处统一走 `AllowedRooms(task, course, classrooms)`

**理由**: 原先三处独立判断教室类型，if-else 散落且大小写不一致；URMF 让教室能力（Type + Equipment）与课程需求（派生 RequiredRoomType + RequiredEquipment）在单一决策点匹配

**实现**: `ee93fb4` P2（核心）→ `01ef918` P3（SA 接入）→ `86d6fbf` P4（OR-Tools/MoveService 接入）→ `b2d722b`（`Classroom.RoomType` 冲突修复，复用现有 `Classroom.Type`）

**副产品**: OR-Tools payload 中 `AllowedRoomIDs` 由 Go 侧计算好，Python solver 只读列表——彻底消除 Go/Python 匹配规则不同步风险

---

## 2026-07-10 — v0.5.2 评分统一语义

**决策**: 引入 `Completeness` + `FinalTotal` 字段，排课完成度作为评分惩罚因子

**理由**: 防止"少排少错"——未排满的课表因冲突少而得分虚高

**公式**: `factor = ratio × (0.5 + 0.5 × ratio)`，ratio=0.5 → factor=0.375

**兼容性**: 新字段可选，旧代码不读不写无影响；`Total` 保留原 7 项之和语义

---

## 2026-07-10 — v0.5.2 SA delta-score 缓存

**决策**: 在 SA 热路径维护增量评分缓存，避免每次邻域评估全量重扫

**实现**: `sa_scorecache.go` + `sa_scorecache_apply.go` + `sa_scorecache_reduce.go`
- `applyEntry(sign, entry, isSports)` 对称 ±1 更新所有计数器
- `scoreFromCache()` 从缓存还原完整 ScoreBreakdown
- 正确性由 `TestDeltaScoreMatchesFullScore` 黄金测试保障

**结果**: SA 迭代速度 29,830 → 236,000+ iter/s（8x 加速）

---

## 2026-07-10 — v0.5.2 uint64 occupancy keys

**决策**: roomOcc/teacherOcc/classOcc 从 `map[string]bool` 改为 `map[uint64]bool`

**实现**: `occKey(day, period, id)` 将三个坐标打包为单个 uint64

**理由**: uint64 键的哈希和比较比字符串快，减少 GC 压力

---

## 2026-07-10 — v0.5.1 灵活课时跨度

**决策**: 支持课时跨度 1/2/3 节（原先仅支持 2 节）

**实现**: `session_plan.go` 作为单一真实来源，`solver.py` 镜像同步

**优先级**: task.PreferredSpan > courseHours 派生 > MaxHoursPerWeek 钳制

---

## 2026-07-10 — v0.4.0 Stable Core 冻结

**决策**: 冻结评分核心（ScoreSchedule / ScoringContext / ScoreBreakdown / ScheduleSnapshot / ScheduleVersion / TeachingTaskClass）

**规则**: 可追加字段，不可修改已有字段语义

**影响**: v0.5.2 的扩展均以可选字段方式追加，保持向后兼容

---

## 2026-07-09 — v0.4.0 全局学期状态

**决策**: 建立 `appStore.currentSemesterId` 为全局唯一学期来源（SSOT）

**影响**: SchedulePage / ResourcePage / SchedulingPage / ReportPage / HistoryComparePage 统一消费

---

## 2026-07-08 — v0.4.0 DB 接口化 (ADR-0003)

**决策**: 定义 `database.DB` 接口，构造函数注入到每个服务

**替代方案**: 包级全局 `*gorm.DB` 变量（已废弃）

---

## 2026-07-08 — v0.4.0 TeachingTask 实体 (ADR-0004)

**决策**: 引入 TeachingTask + TeachingTaskClass 显式关联，替代院系模糊匹配

**影响**: 消除 `deptMap` / `reverseDeptMap` 硬编码，支持合班和跨院系排课

---

## 2026-07-07 — 多引擎架构 (ADR-0005)

**决策**: SA 为主引擎，OR-Tools 为可选增强，uv 管理 Python 依赖

**降级策略**: OR-Tools 不可用/超时 → 无缝切换 SA，用户无感

---

## 2026-07-07 — 快照模式 (ADR-0002)

**决策**: 排课/微调后生成持久化快照，支持历史对比

**触发**: 自动（排课后）+ 手动（微调后用户主动）

---

## 2026-07-07 — SA 作为主求解器 (ADR-0001)

**决策**: 采用模拟退火，放弃 OR-Tools 作为主引擎

**理由**: OR-Tools 体积 150MB+、MSVC/MinGW ABI 不兼容、Go 绑定不成熟
