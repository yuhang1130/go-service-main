# 业务开发指南

本指南描述如何在不破坏 Role 和基础设施边界的前提下增加一个业务 Feature。

## 1. 建立 Feature

业务代码放在 `internal/features/<feature>`，先定义领域语言和用例，不从数据库表或 HTTP 接口反推业务模型。

```text
internal/features/order/
├── domain/
│   ├── order.go
│   └── errors.go
└── application/
    ├── create_order.go
    └── create_order_test.go
```

`domain` 只表达业务状态和规则；`application` 负责流程编排，并定义所需 Repository、Clock、ID、Event Publisher 等端口。

## 2. 实现 Adapter

根据业务实际入口和依赖实现 Adapter：

- HTTP：请求/响应 DTO、鉴权、参数校验和错误映射。
- MySQL：GORM 持久化模型、查询和 application 端口实现。
- Redis：缓存、租约或幂等辅助；缓存不能成为业务事实的唯一来源。
- Task Dispatch：业务任务与 Redis Stream 消息之间的协议映射。
- Scheduler：只负责触发短任务，长流程必须持久化执行状态。

GORM 模型、HTTP DTO 和领域实体是三种不同的数据边界，不应复用同一个 struct。

## 3. 在 Role 中装配

只修改需要该 Feature 的组合根，并按 Feature 拆分装配文件：

- `internal/bootstrap/api_<feature>.go`：创建 HTTP Handler 和 API 用例，由 `api.go` 调用。
- `internal/bootstrap/job_<feature>.go`：提供发现、补偿或清理任务，由 `job.go` 注册。
- `internal/bootstrap/<role>_<feature>.go`：在采集、裂变、上传或基建 Consumer 的固定注册表中显式注册 Task Handler。

不要在 `cmd/*` 中写业务逻辑，也不要通过运行时参数把不同 Role 合并成一个万能进程。

## 4. 数据和任务分发

新增表或索引时必须增加成对的版本化 SQL 文件。操作人员审核后按版本顺序人工执行 `.up.sql`；应用启动不会自动修改数据库结构，也不得引入自动迁移工具。

需要可靠创建异步任务时，由 Application 用例定义任务 Repository、Outbox 和事务端口，业务任务和 `task_dispatch_outbox` 在同一 MySQL 事务中提交。事务提交后可以立即尝试发布，Job 在事务外补发未发布记录。

Redis Stream 消息只携带 `dispatch_id`、`task_type`、`task_id` 和版本。Consumer Handler 必须先通过 MySQL 条件更新取得任务执行权；取得后确认 Stream 消息，再在数据库事务外调用文件系统、FFmpeg 或第三方平台。重复消息必须返回忽略结果，不能重复执行外部副作用。

任务类型和版本是跨进程契约，必须使用稳定常量注册；改变已有语义时应新增版本，不要静默覆盖旧版本。

## 5. 测试与验收

每个 Feature 至少覆盖：

- 领域不变量和状态流转单元测试；
- Application 用例成功、冲突、重复调用和依赖失败测试；
- MySQL Adapter 集成测试；
- HTTP 或消息协议契约测试；
- API、Job 和四个 Consumer Role 独立构建。

本地验收命令：

```bash
make dev-up
# 按 docs/runbook.md 人工审核并逐个执行所需 migrations/*.up.sql
make ci
```

若某项依赖未启动而导致集成测试跳过，需要在交付说明中明确标记，不能把跳过当成通过。
