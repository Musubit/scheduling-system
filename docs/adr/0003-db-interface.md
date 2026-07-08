# ADR-0003: DB 接口化——打破全局可变状态

## 状态

已采纳

## 上下文

项目原有 `database.DB` 包级全局 `*gorm.DB` 变量，4个服务直接读取。这是系统最大的耦合源——阻碍了 70% 代码的可测试性。每个服务的方法都依赖全局状态，无法进行单元测试。

## 决策

**定义 `database.DB` 接口，通过构造函数注入到每个服务。**

接口仅包含服务实际调用的 GORM 方法子集：`Find`, `Create`, `Save`, `Delete`, `Where`, `Preload`, `First`, `Model`, `Order`, `Count`, `Transaction`, `AutoMigrate`, `Error`。

`GormAdapter` 包装 `*gorm.DB` 实现该接口。`InitDB()` 返回适配器实例，由 `main.go` 传递给各服务构造函数。

## 理由

1. **可测试性**：每个服务可接收 mock DB 独立测试
2. **杠杆效应**：一个接口，5个服务受益（ResourceService, SchedulingService, SnapshotService, MoveService）
3. **局部性改善**：服务不再隐式依赖全局状态，构造函数明确声明依赖
4. **渐进式**：不改变 GORM 调用模式——`s.db.Find(&x)` 语义与原来一致

## 后果

- **正面**：测试面显著扩大，服务间依赖关系显式化
- **负面**：接口方法列表需与 GORM 实际调用保持同步（新增 GORM 方法需追加到接口）
- **可逆性**：如接口膨胀到难以维护，可降级为仓库模式（每实体一个接口）
