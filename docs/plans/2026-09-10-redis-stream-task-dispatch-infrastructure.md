# Redis Stream 任务分发基础架构实施计划

状态：基础设施已于 2026-09-10 实施；业务任务表和 Handler 待后续开发。

## 目标

使用 Redis Streams 替换当前 RocketMQ 消息基础设施，为素材采集、素材裂变、渠道上传和渠道基建提供四条独立的任务分发通道。Redis 负责低延迟唤醒，MySQL 负责任务事实、可靠投递意图、幂等领取和故障恢复。

## 已确认的架构

```text
任务创建者
  └─ MySQL 事务：业务任务 + task_dispatch_outbox
       └─ 提交后立即尝试 XADD
            ├─ 正常路径：Redis Stream → Consumer
            └─ 失败路径：Job 补发 Outbox

Consumer
  └─ MySQL 条件领取执行权
       └─ 领取成功后 XACK
            └─ MySQL 租约管理长时间执行和宕机恢复

Job
  ├─ 补发未成功发布的 Outbox
  └─ 后续按业务任务状态恢复漏消息、过期租约和到期重试
```

四条 Stream：

```text
go-service-main:material:collection:v1
go-service-main:material:transformation:v1
go-service-main:channel:upload:v1
go-service-main:channel:infrastructure:v1
```

四个独立二进制：

```text
cmd/collection-consumer
cmd/transformation-consumer
cmd/upload-consumer
cmd/infrastructure-consumer
```

不得通过运行时参数或配置开关把一个 Consumer 二进制切换为不同 Role。

## 本次实施范围

1. 删除 RocketMQ Go SDK、Adapter、配置、Makefile 初始化逻辑和本地 Compose 服务。
2. 增加 Redis Stream Producer、Consumer、Consumer Group 初始化和 Pending 重领能力。
3. 增加共享 `task_dispatch_outbox` Store 和 Relay。
4. 任务创建事务提交后立即尝试发布；发布失败不回滚已经提交的业务任务。
5. Job 定时补发未成功发布的 Outbox，默认补偿周期一分钟。
6. 增加四个独立 Consumer Role 二进制、配置文件、健康检查和优雅关闭。
7. 未注册业务 Handler 的 Consumer 必须启动失败，不得确认未知任务。
8. 增加重复消息、条件领取、Pending 重领、Redis 重启和关闭期间不误确认的测试。
9. 新增版本化 SQL，创建 `task_dispatch_outbox` 并删除旧 `event_inbox`、`event_outbox`；不得修改可能已经执行过的历史迁移。
10. 更新 README、架构、配置、运行手册和业务开发说明。
11. 保持 Go 1.26 兼容，并在提交前运行 `make ci`。

## 明确不在本次范围

- 不创建四类业务任务表。
- 不实现素材下载、FFmpeg/FFprobe、渠道上传或渠道基建。
- 不建立统一 `media_pipeline`。
- 不实现多机共享文件或对象存储。
- 不确定各业务任务的最终租约、超时、重试次数和限流参数。
- 不声称已经完成“Redis 发布成功后数据丢失”的业务级端到端恢复；本次只保留恢复端口和测试替身，待业务任务表落地后接入。

## 可靠性边界

- `task_dispatch_outbox` 的 `PUBLISHED` 只表示 Redis 接受了 `XADD`，不表示任务已经被领取或完成。
- Redis 消息只携带稳定的投递 ID、任务类型、任务 ID 和协议版本，不携带凭据或完整业务参数。
- Redis Stream 允许重复投递；消费者必须先通过 MySQL 条件更新取得执行权。
- 成功取得 MySQL 执行权后立即 `XACK`，长任务恢复由 MySQL 租约负责。
- Outbox 找回从未成功发布的任务；业务任务扫描找回已经发布但未完成的任务。
- 本地和开发环境共用 Redis，且不依赖 Redis 持久化保证任务安全。Redis 数据丢失可能带来不超过下一次补偿扫描周期的额外延迟。
- 旧 Inbox 表可以删除，但 Inbox 幂等保证必须由业务任务的唯一键和条件领取继续提供。

## 验收

- 四个 Consumer 二进制可独立构建、配置、启动、停止和探活。
- 每个 Consumer 只订阅自己的 Stream 和 Consumer Group。
- Redis 不可用时，业务事务和 Outbox 可以提交，调用方得到“已排队”语义。
- Redis 恢复后，Job 能补发未发布 Outbox。
- 重复 Stream 消息不能获得两次 MySQL 执行权。
- Consumer 在取得执行权前退出时，消息可以从 Pending 状态被重领。
- Consumer 在取得执行权并确认消息后退出时，后续由 MySQL 租约恢复，而不是依赖 Redis PEL。
- 日志不记录消息负载、凭据、文件内容或第三方请求正文。
- 版本化 SQL 配对正确，旧表删除通过新迁移完成。
- `git diff --check` 和 `make ci` 通过；缺少 MySQL 或 Redis 环境导致的集成测试跳过必须明确报告为跳过。

## 后续业务设计

基础设施完成后，再分别设计四类业务任务的表结构、业务幂等键、输入快照、状态机、执行租约、取消、重试、限流和三方结果确认。四类任务保持独立，通过直接来源关系连接，不预设统一流水线聚合。

业务任务表落地后，还必须为 Job 增加非终态任务扫描：重新分发长时间未领取的 `PENDING`、租约过期的 `RUNNING`、到期的 `RETRY_WAIT` 和需要对账的 `UNKNOWN`。当前实现已覆盖 Outbox 未发布补偿、Redis Pending 重领和 Consumer Group 重建，但不能在没有业务任务表的情况下实现这条业务级恢复链。
