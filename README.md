# go-service-main

一个可以直接承载业务代码的 Go 服务工程模板。仓库使用单个 `go.mod`，同时提供 API、Job、Consumer 三个可独立编译和部署的 Role，并共享配置、日志、数据库、缓存、消息和健康检查等基础设施。

这不是代码生成器。创建业务仓库后，直接在 `internal/features` 下增加 Feature，并在对应的 `internal/bootstrap` 组合根中完成显式装配。

## 工程结构

```text
.
├── cmd/
│   ├── api/                    # HTTP API 进程入口
│   ├── job/                    # 定时扫描、发现和补偿进程入口
│   └── consumer/               # RocketMQ 消费进程入口
├── internal/
│   ├── bootstrap/              # 各 Role 的依赖装配和生命周期
│   ├── features/               # 业务领域规则和应用用例
│   ├── adapters/               # HTTP、MySQL、Redis、MQ、调度器适配器
│   └── foundation/             # 配置、日志、健康检查、事务等共享基础设施
├── configs/                    # 各 Role 独立配置
├── migrations/                 # 人工执行的版本化 SQL，唯一数据库结构来源
├── api/openapi.yaml            # API 契约
├── deployments/                # 本地依赖与通用镜像构建
└── docs/                       # 架构、配置、运行手册和 ADR
```

依赖方向：

```text
cmd -> bootstrap -> adapters -> application -> domain
                         \-> foundation
```

`domain` 和 `application` 不依赖 Gin、GORM、Redis、RocketMQ 等基础设施 SDK。不同 Role 可以复用同一个 Feature，但不会因此共享进程生命周期。

## 环境要求

- Go 1.26+
- Docker 与 Docker Compose（本地 MySQL、Redis、RocketMQ）
- MySQL 命令行客户端或数据库管理平台（人工执行 SQL）

## 快速开始

```bash
cp .env.example .env
set -a
source .env
set +a

make dev-up
make sql-list
# 审核后，按列表顺序逐个执行 migrations/*.up.sql，详见 docs/runbook.md
make dev-api
```

本地首次初始化管理端时运行 `make init-admin`，按提示输入用户名和至少 8 位的密码。脚本通过临时 API 创建并绑定 ROOT 角色，成功后自动停止临时进程，不会把密码写入 `.env`、命令参数或日志。只有 `sys_user` 为空时才会创建该超级管理员。生产环境仍应通过部署平台 Secret 一次性注入 `APP_IDENTITY_BOOTSTRAP_USER` 与 `APP_IDENTITY_BOOTSTRAP_PASSWORD`。若需要通过管理端新增用户，还需设置 `APP_IDENTITY_DEFAULT_PASSWORD`，新用户登录后应立即修改密码。

`make dev-api` 使用固定版本的 Air 监听 `cmd/api`、`internal` 和 `configs` 下的 Go/YAML 文件，变更后重新编译并以 `SIGINT` 优雅重启 API。首次运行会下载 Air；它只用于本地开发，不会进入服务的 `go.mod` 或生产镜像。不需要热更新时仍可使用 `make run-api`。

测试文件和数据库 SQL 不触发自动重启；建表和结构变更由操作人员审核后逐个执行，应用不会自动修改数据库结构。

`make dev-up` 会创建本地 RocketMQ Topic。API 启动后可通过验证码接口确认业务服务正常响应：

```bash
curl -i http://127.0.0.1:8080/api/v1/auth/captcha
```

单租户后台能力包括登录、用户、角色、菜单、部门、字典、参数配置、通知公告、操作日志、文件上传、SSE 实时消息以及用户 Excel 导入导出。身份模块使用本服务自有的 Redis 可撤销会话；除验证码、登录、刷新令牌和随机文件内容地址外，接口均需 Bearer Token，并按稳定权限标识校验管理操作。通知富文本在入库前会清理可执行标签和危险链接。操作日志异步、尽力写入，保存操作人当时的昵称快照、路由、方法、状态、耗时和终端元数据，不保存请求体、响应体或查询字符串；统计口径是操作次数与去重操作人数，不是页面 PV/UV，也不能作为合规审计流水。

文件适配器通过 `APP_FILE_STORAGE_TYPE` 选择 `local`、`s3` 或 `aliyun_oss`。本地模式默认写入 `.tmp/uploads`；S3 模式兼容 AWS S3、MinIO、RustFS、Cloudflare R2 等服务；阿里云模式使用 OSS。单文件不超过全局 HTTP Body 上限（默认 2 MiB），仅允许常见图片、文档和压缩包扩展名，并会校验文件头识别出的 MIME 类型是否与扩展名匹配。单文件、最多 20 个文件的批量上传和专用图片上传均返回 `/api/v1/files/content/<opaque-key>`，内容由 API 从当前存储后端读取。生产多副本必须使用共享对象存储或共享持久化卷。

API 登录后会建立 `/api/v1/sse/connect` 长连接。字典新增、修改、删除和通知发布、撤回会通过 Redis Pub/Sub 跨 API 实例分发；指定用户通知只推给目标账号。SSE 只承担可丢失的实时提示，数据库通知列表才是事实来源；每个连接使用有界发送队列，慢客户端队列满后会被断开，不能阻塞其他用户。通知遵循“草稿可编辑、草稿可发布、已发布可撤回、撤回后不可重新发布”的单向生命周期。在线人数用 Redis 带过期时间的实例级在线标记按用户去重，异常退出的实例最迟在 45 秒后从统计中剔除。反向代理必须关闭该路径的响应缓冲并允许长连接。

默认地址：

- 业务 API：`http://127.0.0.1:8080`
- API 健康检查：`http://127.0.0.1:9090/readyz`
- Job 健康检查：`http://127.0.0.1:9091/readyz`
- Consumer 健康检查：`http://127.0.0.1:9092/readyz`

业务模块注册事件生产和处理逻辑后，可分别启动 Job 和 Consumer 完成 Outbox 投递及 Inbox 防重消费：

```bash
make run-job
make run-consumer
```

Consumer 没有注册业务事件处理器时会以 idle 模式启动，仅暴露 management 健康检查且不连接 RocketMQ；注册至少一个事件处理器后才会创建订阅，避免空 Consumer 确认并丢弃未知消息。

## 开始写业务

以订单业务为例，新建：

```text
internal/features/order/
├── domain/                     # Order、状态流转和业务不变量
└── application/                # CreateOrder、CancelOrder 等用例及其端口
```

然后按使用场景添加 Adapter：

```text
internal/adapters/http/order/          # API DTO、参数校验和响应映射
internal/adapters/mysql/order/         # GORM 持久化模型和 Repository 实现
internal/adapters/messaging/order/     # 事件发布或消费协议映射
```

最后只在需要该业务的 Role 中装配。按 Feature 使用独立装配文件，避免把具体业务依赖堆进 Role 生命周期文件：

- API 用例：`internal/bootstrap/api_<feature>.go`
- 定时发现或补偿任务：`internal/bootstrap/job_<feature>.go`
- 事件处理器：`internal/bootstrap/consumer_<feature>.go`，并加入 `consumer_handlers.go` 的统一注册表

完整规则和检查清单见 [业务开发指南](docs/business-development.md)。

## 独立构建与部署

分别构建三个二进制：

```bash
make build-api
make build-job
make build-consumer
```

通用 Dockerfile 通过 `SERVICE` 参数选择入口：

```bash
docker build -f deployments/docker/Dockerfile --build-arg SERVICE=api -t go-service-main-api:dev .
docker build -f deployments/docker/Dockerfile --build-arg SERVICE=job -t go-service-main-job:dev .
docker build -f deployments/docker/Dockerfile --build-arg SERVICE=consumer -t go-service-main-consumer:dev .
```

三个镜像必须分别配置、发布和扩缩容。数据库变更 SQL 由操作人员在部署前独立执行，不会在任一 Role 启动时自动执行。
镜像内置 `/configs` 下的非敏感 Role YAML，运行时环境变量具有更高优先级；不要把真实密码或生产地址写入这些文件。

## 配置

配置优先级为：代码默认值 → Role YAML → `APP_*` 环境变量。每个 Role 只校验和初始化自己实际使用的 Capability：三个 Role 都需要 MySQL，API 的本地身份会话需要 Redis，Job 和 Consumer 需要 RocketMQ。

- `APP_CONFIG_FILE`：指定配置文件
- `APP_MYSQL_DSN`：应用使用的 GORM DSN
- `APP_REDIS_ADDRESS`：Redis 地址
- `APP_IDENTITY_BOOTSTRAP_USER`、`APP_IDENTITY_BOOTSTRAP_PASSWORD`：空库首次启动时创建超级管理员（必须同时配置）
- `APP_IDENTITY_DEFAULT_PASSWORD`：管理端新增用户的初始密码
- `APP_FILE_STORAGE_TYPE`：`local`、`s3` 或 `aliyun_oss`
- `APP_FILE_STORAGE_ROOT`：本地存储根目录
- `APP_FILE_STORAGE_PUBLIC_BASE_URL`：文件公开访问地址前缀；留空时返回同源相对地址
- `APP_FILE_STORAGE_MAX_FILE_BYTES`：单文件大小上限，不能超过 `APP_SERVER_MAX_BODY_BYTES`
- `APP_FILE_STORAGE_S3_*`：S3 兼容存储端点、区域、Bucket 和凭据；凭据留空时使用 AWS 默认凭据链
- `APP_FILE_STORAGE_ALIYUN_OSS_*`：阿里云 OSS 端点、Bucket 和凭据
- `APP_ROCKETMQ_ENDPOINTS`：RocketMQ Proxy gRPC 地址

真实密码、Token、AccessKey 和生产地址只能通过环境变量或部署平台 Secret 注入。人工执行 SQL 时使用独立的受限数据库账号，不复用应用账号或在命令历史中写入密码。

详见 [配置说明](docs/configuration.md) 和 [运行手册](docs/runbook.md)。

## 质量门禁

```bash
make ci
```

该命令执行格式检查、版本化 SQL 配对检查、`go vet`、Staticcheck、单元测试、可用集成测试、OpenAPI 检查和三个 Role 的构建。需要真实依赖的完整集成测试应先启动本地基础设施，并按运行手册人工执行所需 SQL：

```bash
make dev-up
# 人工审核并逐个执行 migrations/*.up.sql
make test-integration
```

## 从模板创建新业务仓库

复制本仓库后，需要将 `github.com/yuhang1130/go-service-main` 全局替换为新仓库的 Go module path，并同步修改服务名、Compose 顶层 `name`、配置中的 Topic 前缀和 Consumer Group。完成后执行：

```bash
go mod tidy
make ci
```

不要保留模板仓库的 Git 历史、测试数据或本地 `.env`。
