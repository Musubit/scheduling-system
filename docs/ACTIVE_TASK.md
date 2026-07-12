# ACTIVE_TASK.md — 当前活跃任务

> **最后更新**: 2026-07-13
> **当前分支**: `main` (v0.5.2 已发布)
> **下一版本**: v0.5.3 — URMF 设计就绪，待实施

---

## 当前状态

v0.5.2 已发布（tag v0.5.2, commit d443109）。v0.5.3 URMF 设计文档 R3 已冻结，进入实施准备阶段。

---

## 下一步行动

### 🔴 P0: v0.5.3 实施（按 P1-P7 阶段）

**前置条件**（✅ 全部满足）：
- [x] R3 设计文档冻结
- [x] 实施计划完成（7 阶段）
- [x] 测试矩阵完成（89 个测试）
- [x] ADR-0006 已记录

**实施顺序**：

| Phase | 内容 | 预估 | 前置 |
|-------|------|------|------|
| P1 | 数据模型 (models) | 0.5 天 | 无 |
| P2 | ResourceMatcher 核心 | 0.5 天 | P1 |
| P3 | SA 接入 | 1 天 | P2 |
| P4 | OR-Tools 接入 | 0.5 天 | P2 |
| P5 | MoveService 接入 | 0.5 天 | P2 |
| P6 | 前端 UI | 1 天 | P1 |
| P7 | 全量测试 | 0.5 天 | P3+P4+P5+P6 |

**关键约束**：
- 每阶段独立 commit，可单独 revert
- P3 完成后需对比 v0.5.2 评分基线（±0.5）
- P4 完成后验证 OR-Tools payload 一致性
- 不一次性做完整个 v0.5.3

### 🟡 P1: H3 调整后保存快照（v0.4.0 遗留）

v0.5.3 完成后处理。

---

## 设计文档索引

| 文档 | 路径 | 状态 |
|------|------|------|
| URMF 设计 R3 | `docs/design/v0.5.3-resource-matching-framework.md` | ✅ 冻结 |
| 实施计划 | `docs/design/v0.5.3-implementation-plan.md` | ✅ 完成 |
| 测试矩阵 | `docs/testing/v0.5.3-resource-tests.md` | ✅ 完成 |
| ADR-0006 | `docs/adr/0006-unified-resource-matching-framework.md` | ✅ 已采纳 |
