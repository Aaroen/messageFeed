# messageFeed 实施文档

**最后更新**：2026-06-17

---

## 📊 实施进度总览

| 阶段 | 名称 | 状态 | 完成度 | 开始日期 | 完成日期 |
|------|------|------|--------|----------|----------|
| 阶段一 | 基础设施搭建 | ✅ 完成 | 100% | 2026-06-12 | 2026-06-13 |
| 阶段二 | 订阅源与 Feed 闭环 | 🚧 进行中 | 80% | 2026-06-13 | - |
| 阶段三 | 日志、错误追踪与链路观测 | 🚧 进行中 | 85% | 2026-06-17 | - |
| 阶段四 | 源目录与导入 | ⏸️ 未开始 | 0% | - | - |
| 阶段五 | 自动化与推荐 | ⏸️ 未开始 | 0% | - | - |
| 阶段六 | AI 摘要与通知 | ⏸️ 未开始 | 0% | - | - |
| 阶段七 | 自然语言控制 | ⏸️ 未开始 | 0% | - | - |
| 阶段八 | 金融监控 | ⏸️ 未开始 | 0% | - | - |
| 阶段九 | 工程化增强 | ⏸️ 未开始 | 0% | - | - |
| 阶段十 | 来源扩展与分布式升级验证 | ⏸️ 未开始 | 0% | - | - |

**图例**：✅ 已完成 | 🚧 进行中 | ⏸️ 未开始 | ❌ 已取消 | ⚠️ 有问题

---

## 1. 实施目标

以 `messageFeed` 作为 `Go_Pro` 首个完整项目，先完成本地单节点可运行、可部署、可观测、可验收的最小闭环，并通过 Tailscale 提供简单远程访问，再逐步扩展 AI 摘要、微信通知、自然语言设置控制、金融市场监控、多来源采集和分布式部署能力。

当前第一部分只交付本地单节点部署，默认 `DEPLOYMENT_MODE=single_node`。该配置只表示部署拓扑，不表示监听范围；局域网或 Tailscale 访问由 `BIND_ADDR` 和 `PUBLIC_BASE_URL` 配置决定。分布式部署仅保留接口与运行时边界，包括节点标识、部署模式配置、就绪检查、任务锁接口、通知幂等键和无状态 API 约束。

## 2. 项目目录与职责规则

项目目录应从第一阶段开始建立清晰边界。除非某个新目录能够对应明确职责、稳定依赖方向和可独立验收的业务能力，否则不应新增目录。优先在既有模块内增加文件或子包，避免形成 `common`、`utils`、`misc`、`temp`、`helper` 等边界不清的目录。

### 2.1 目标目录结构

```text
messageFeed/
├── cmd/
│   └── api/
├── internal/
│   ├── config/
│   ├── domain/
│   ├── repository/
│   ├── service/
│   ├── handler/
│   ├── catalog/
│   ├── importer/
│   ├── fetcher/
│   ├── recommender/
│   ├── scheduler/
│   ├── control/
│   ├── market/
│   ├── alert/
│   ├── llm/
│   ├── notifier/
│   └── runtime/
├── api/
├── web/
├── migrations/
├── deploy/
├── test/
├── docs/
├── go.mod
├── go.sum
└── Makefile
```

### 2.2 顶层目录职责

| 目录 | 职责 | 新增规则 |
| --- | --- | --- |
| `cmd/api` | HTTP 服务入口、依赖装配、进程生命周期管理 | 一个可执行程序对应一个 `cmd/<name>`；不得在 `main.go` 写业务逻辑 |
| `internal` | 只供本项目使用的业务代码 | 所有业务实现默认进入 `internal`，不得为尚未复用的代码创建 `pkg` |
| `api` | OpenAPI、proto 或外部契约文件 | 只存契约和生成入口，不存 handler 实现 |
| `web` | Vue 3 前端工程（独立仓库或子目录） | 完整前端项目，包含 src/、public/、vite.config.ts 等，通过 API 与后端通信 |
| `migrations` | PostgreSQL 正式迁移文件 | 使用 `golang-migrate/migrate` 执行；`up` 用于升级，`down` 仅用于显式回滚；不维护第二套测试 schema |
| `deploy` | Dockerfile、Docker Compose、Cloudflare、Prometheus、Grafana 等部署材料 | 路径配置使用相对路径；本地部署和后续分布式部署配置分层放置 |
| `test` | 集成测试、E2E 测试和测试夹具 | 只放正式测试资产，不放一次性调试脚本 |
| `docs` | 需求、架构、实施等少量长期文档 | 不新增单点说明文档；新增文档前优先合并到现有三类文档 |

### 2.3 `internal` 模块职责

| 模块 | 允许放置内容 | 不允许放置内容 |
| --- | --- | --- |
| `config` | 配置结构、默认值、环境变量解析、配置校验 | 业务决策、数据库访问、HTTP handler |
| `domain` | 实体、枚举、值对象、领域错误、业务常量 | GORM 查询、第三方 SDK 调用、HTTP 响应模型 |
| `repository` | PostgreSQL 访问、事务封装、查询对象 | 业务编排、外部 API 抓取、通知发送 |
| `service` | 用例编排、事务边界、跨模块协调 | Gin 参数绑定、SQL 细节、第三方 SDK 细节 |
| `handler` | Gin 路由、请求绑定、响应渲染、错误映射 | 业务规则、数据库事务、模型提示词 |
| `catalog` | 推荐源目录、分类、健康状态、源搜索 | 用户订阅源主逻辑、RSS 抓取实现 |
| `importer` | OPML 解析、URL 批量导入、目录批量订阅流程 | Feed 抓取解析、AI 摘要 |
| `fetcher` | RSS、Atom、JSON Feed 抓取、解析、规范化 | 调度策略、用户阅读状态 |
| `recommender` | 推荐 Feed 候选池、排序、推荐原因、未订阅来源标注和用户反馈 | 抓取外部来源、自动订阅来源、隐藏未订阅来源身份 |
| `scheduler` | 周期任务、重试、任务锁调用、任务指标 | 具体业务规则实现、HTTP 入口 |
| `control` | 自然语言设置控制、意图解析、变更计划、确认策略、执行编排和审计 | 直接写数据库、绕过 service 执行设置变更、通用远程命令执行 |
| `market` | 金融标的、行情源、行情快照、市场日历、指标计算 | 通知发送、AI 文本生成 |
| `alert` | 内容告警、金融告警、规则评估、冷却时间、幂等键 | 行情拉取、微信 SDK 调用 |
| `llm` | 模型客户端、提示词、结构化输出、token 与耗时记录 | 业务实体持久化、通知通道实现 |
| `notifier` | ntfy、企业微信、公众号等通知通道适配 | 是否触发通知的业务判断 |
| `runtime` | 节点标识、部署模式、就绪状态、任务锁接口 | 具体 Feed、金融、摘要业务逻辑 |

Redis 不进入第一阶段目录和运行依赖。后续如引入 Redis，应通过 `runtime`、`scheduler` 或明确的基础设施接口承载缓存、任务队列、限流、短期状态或分布式锁实现，不应让业务 service 直接依赖 Redis 客户端。

### 2.4 文件放置规则

- HTTP 入参和响应结构放在 `handler` 内，除非它们同时是外部契约的一部分。
- 数据库模型可以放在 `repository` 或独立 persistence 文件中，但不得污染 `domain` 的业务实体。
- 领域实体只表达业务含义，不直接携带 Gin、GORM、SQL、JSON API 绑定细节。
- 第三方 SDK 只能出现在适配层，例如 `fetcher`、`llm`、`notifier`、`market/provider`。
- Web 页面只调用 HTTP API，不直接读取数据库或复刻业务规则。
- 提示词模板属于 `llm`，但摘要生成的业务选择逻辑属于 `service`。
- 通知是否触发属于 `service` 或 `alert`，通知如何发送属于 `notifier`。
- 定时任务只负责编排和触发，不直接写复杂业务规则。
- 测试夹具进入 `test`，迁移测试必须复用 `migrations`。

### 2.5 新增目录审批规则

新增目录前必须满足以下条件：

1. 该目录代表一个稳定职责，而不是临时实现细节。
2. 至少有两个文件会长期归属该职责；若只有一个文件，优先放入现有模块。
3. 该目录的上游和下游依赖方向可以用一句话说明。
4. 该目录不会与现有模块职责重叠。
5. 文档中的目录职责表需要同步更新。

禁止新增以下类型目录：

- `common`、`utils`、`helpers`、`misc`、`shared`：除非能进一步拆成明确职责模块。
- `tmp`、`debug`、`scratch`：调试资产不进入正式项目结构。
- `models`：容易混合领域实体、数据库模型和响应结构，应按职责放入 `domain`、`repository` 或 `handler`。
- `clients`：第三方客户端应放入具体适配模块，例如 `llm`、`notifier`、`market/provider`。

### 2.6 依赖方向规则

依赖方向应保持单向：

```text
handler -> service -> repository
service -> fetcher/catalog/importer/recommender/llm/notifier/control/market/alert/runtime
recommender -> repository/domain
control -> service/llm/runtime
scheduler -> service/runtime
repository -> domain
adapter modules -> domain
```

约束如下：

- `repository` 不依赖 `service`、`handler`、`scheduler`。
- `domain` 不依赖任何基础设施模块。
- `handler` 不直接调用 `repository`。
- `web` 不直接调用数据库，只通过 HTTP API 与服务交互。
- `scheduler` 不直接调用第三方 SDK，应通过 service 或明确适配模块。
- `control` 不直接调用 `repository`，设置变更必须通过既有 service 接口执行。
- `notifier` 不调用 `llm`；需要 AI 文本时由 `service` 先生成，再交给 `notifier`。
- `alert` 可以评估规则和生成事件，不负责发送通知。

### 2.7 文件命名规则

- 业务文件按职责命名，例如 `source_service.go`、`source_repository.go`、`daily_summary_service.go`。
- 接口文件优先放在调用方所在模块，避免为了接口创建抽象目录。
- 测试文件与被测文件同目录，集成测试或 E2E 测试进入 `test`。
- 迁移文件使用递增版本号、语义名称和 `up/down` 后缀，例如 `000002_add_sources_items.up.sql` 与 `000002_add_sources_items.down.sql`。
- 配置示例可以使用 `.env.example`，不得提交真实密钥。

## 3. 阶段路线

| 阶段 | 目标 | 详细实施 |
| --- | --- | --- |
| 阶段一 | 本地工程基线与 Tailscale 访问 | [查看细节](#phase-1) |
| 阶段二 | 订阅源与 Feed 闭环 | [查看细节](#phase-2) |
| 阶段三 | 日志、错误追踪与链路观测 | [查看细节](#phase-3) |
| 阶段四 | 源目录与导入 | [查看细节](#phase-4) |
| 阶段五 | 自动化、兴趣规则与推荐 Feed | [查看细节](#phase-5) |
| 阶段六 | AI 摘要与通知 | [查看细节](#phase-6) |
| 阶段七 | 自然语言设置控制 | [查看细节](#phase-7) |
| 阶段八 | 金融市场监控与 AI 告警 | [查看细节](#phase-8) |
| 阶段九 | 工程化增强 | [查看细节](#phase-9) |
| 阶段十 | 来源扩展与分布式升级验证 | [查看细节](#phase-10) |

## 4. 详细实施过程

### <a id="phase-1"></a>阶段一：基础设施搭建 ✅

**状态**：✅ 已完成 | **完成时间**：2026-06-13 | **完成度**：100%

#### 实施进度清单

**项目骨架** ✅
- [x] 初始化 Go 模块
- [x] 创建目录结构
- [x] 配置 .gitignore

**配置系统** ✅
- [x] 实现 internal/config 模块
- [x] 环境变量解析
- [x] 配置校验和默认值
- [x] 单元测试

**HTTP 服务** ✅
- [x] 基础 HTTP 服务器
- [x] GET / (服务信息)
- [x] GET /healthz (存活检查)
- [x] GET /readyz (就绪检查，含数据库)
- [x] GET /metrics (Prometheus 指标)
- [x] GET /api/runtime/node (节点信息)
- [x] 请求日志中间件

**数据库集成** ✅
- [x] internal/db 模块
- [x] 连接池配置
- [x] 健康检查
- [x] migrations/000001_init_schema.up.sql
- [x] migrations/000001_init_schema.down.sql
- [x] `golang-migrate/migrate` 版本化迁移
- [x] Docker Compose 独立迁移服务

**可观测性** ✅
- [x] log/slog 结构化日志
- [x] internal/metrics 模块
- [x] HTTP 请求指标
- [x] 数据库连接池指标

**构建与部署** ✅
- [x] Makefile (fmt, vet, test, build, verify)
- [x] Dockerfile (多阶段构建)
- [x] docker-compose.yml
- [x] .env.example

**文档** ✅
- [x] docs/requirements.md
- [x] docs/architecture.md
- [x] docs/implementation.md
- [x] 前后端架构章节

#### 验收结果 ✅
- [x] make compose-up 一键启动
- [x] /healthz 返回成功
- [x] /readyz 包含数据库检查
- [x] /metrics 暴露指标
- [x] make verify 全部通过
- [x] 数据库自动迁移成功

---

实施范围：

- 建立 Go 服务骨架、配置加载、结构化日志、统一错误、基础路由。
- 接入 PostgreSQL、GORM 和 `golang-migrate/migrate` 正式 SQL 迁移文件。
- 暴露 `/healthz`、`/readyz`、`/metrics` 和 `/api/runtime/node`。
- 提供 Docker Compose、`Makefile` 和最小 `make verify`。
- 支持 `BIND_ADDR`、`PUBLIC_BASE_URL`、`APP_NODE_ID`、`DEPLOYMENT_MODE`、`TRUSTED_PROXY_CIDRS`。
- 本地默认使用 `DEPLOYMENT_MODE=single_node`，远程访问通过 `BIND_ADDR` 暴露到局域网、Tailscale IP 或 MagicDNS 完成。
- PostgreSQL 作为第一阶段唯一主存储；Redis 不进入第一阶段 Docker Compose 必需组件，但预留后续缓存、队列、限流和任务锁接口。

实施步骤：

1. 创建 `cmd/api`、`internal/config`、`internal/handler`、`internal/runtime`、`internal/repository`、`deploy`、`api` 等基础目录。
2. 在配置层定义数据库、HTTP、运行时、日志和后续外部服务的配置结构；密钥只从环境变量或外部配置注入。
3. 建立基础 HTTP 路由，统一错误响应结构和 request id 日志字段；Gin 路由迁移进入第二阶段执行。
4. 建立数据库连接、版本化迁移执行入口和 `/readyz` 依赖检查；迁移由 Compose/Makefile 显式执行，API 启动时只检查迁移状态，不自行修改 schema。
5. 增加 Prometheus 指标注册，至少覆盖请求量、请求耗时、健康状态和数据库连接状态。
6. 在 Docker Compose 中纳入服务本体、PostgreSQL 和一次性迁移服务，不在第一阶段引入 Redis；与缓存、队列、限流和任务锁相关的能力通过接口预留。
7. 在 Tailscale 场景下将 `BIND_ADDR` 配置为 `0.0.0.0:8080` 或 Tailscale IP，并将 `PUBLIC_BASE_URL` 配置为 Tailscale IP 或 MagicDNS 访问地址。

验收标准：

- `make compose-up` 可以启动 PostgreSQL、完成 pending `up` 迁移并启动服务；空卷首次初始化时直接 `docker compose up -d --build` 也应成立。
- `/healthz` 返回成功。
- `/readyz` 能检查数据库连接和 `schema_migrations` 迁移状态。
- `/metrics` 可以被 Prometheus 格式读取。
- Tailscale 网络内设备可以访问 API。
- `make verify` 可以执行格式检查、构建和基础测试。
- `/api/runtime/node` 能返回 `deployment_mode=single_node`、节点标识、监听配置和公开访问基址。

风险控制：

- API 层不得依赖本机内存保存业务状态。
- 第一阶段任务锁可以是单节点或 PostgreSQL 实现，但接口必须保留，后续允许替换为 PostgreSQL advisory lock、任务表锁或 Redis 锁。
- `/readyz` 不检查非关键外部服务，避免微信、AI 或行情源短暂不可用导致服务被错误摘除。
- Redis 不作为主存储或审计存储；业务幂等、持久状态和可追溯记录必须落入 PostgreSQL。
- PostgreSQL 官方镜像的 `/docker-entrypoint-initdb.d` 只用于空库初始化，不承载项目迁移；不得将包含 `.down.sql` 的完整 `migrations` 目录挂载到该路径。
- 共享环境中已经执行过的迁移文件不得修改，只能追加更高版本迁移；生产回滚优先采用备份恢复或前向修复，`down` 仅用于明确授权的受控回滚。

### <a id="phase-2"></a>阶段二：订阅源与 Feed 闭环 🚧

**状态**：🚧 进行中 | **开始时间**：2026-06-13 | **完成度**：80%

#### 实施进度清单

**路由层迁移** ✅
- [x] 引入 Gin 依赖
- [x] 将现有 `net/http` 路由迁移到 Gin
- [x] 保留 `/`、`/healthz`、`/readyz`、`/metrics` 和 `/api/runtime/node` 的既有行为
- [x] 建立 `/api/v1` 业务路由组
- [x] 建立 request id、访问日志、统一响应和错误映射中间件
- [x] 迁移现有 handler 测试

**数据库设计** ✅
- [x] 创建 `migrations/000002_add_sources_items.up.sql`
- [x] 创建 `migrations/000002_add_sources_items.down.sql`
- [x] 定义 `sources` 表
- [x] 定义 `items` 表
- [x] 定义 `user_item_states` 表
- [x] 定义 `feed_view_preferences` 表
- [x] 添加索引、唯一约束、检查约束和更新时间触发器
- [x] 通过 Docker Compose 在空数据库上执行 `000001 -> 000002` 迁移验证

**领域模型** ✅
- [x] `internal/domain/source.go`
- [x] `internal/domain/item.go`
- [x] `internal/domain/user_item_state.go`
- [x] `internal/domain/feed_view_preference.go`
- [x] 枚举和领域错误定义

**Repository 层** ✅
- [x] `internal/repository/source_repository.go` (Create, Get, List, Update, UpdateFetchResult)
- [x] `internal/repository/source_repository_test.go`
- [x] `internal/repository/item_repository.go` (UpsertMany, ListByUser, GetByIDForUser；列表和详情联表返回来源名称与阅读状态)
- [x] `internal/repository/item_repository_test.go`
- [x] `internal/repository/user_item_state_repository.go` (MarkRead, Favorite, Hide；使用 `ON CONFLICT(user_id,item_id)` 原子 upsert，避免并发首次写入触发唯一约束冲突)
- [x] `internal/repository/feed_view_preference_repository.go` (Get, Upsert)

**Fetcher 模块** ✅
- [x] `internal/fetcher/feed_fetcher.go`
- [x] 集成 `gofeed`
- [x] HTTP 抓取（超时、重定向、大小限制）
- [x] URL 规范化
- [x] 条目字段映射和内容 hash
- [x] 错误处理
- [x] `internal/fetcher/feed_fetcher_test.go`

**Service 层** ✅
- [x] `internal/service/source_service.go` (Create, List, Update, TriggerFetch)
- [x] `internal/service/source_service_test.go`
- [x] `internal/service/timeline_service.go` (ListItems, GetItem；支持已读、收藏、隐藏和来源过滤参数)
- [x] `internal/service/timeline_service_test.go`
- [x] `internal/service/item_service.go` (MarkRead, Favorite, Hide)
- [x] `internal/service/feed_view_service.go` (GetMode, SaveMode；不存在用户偏好时返回默认 `timeline`)

**Handler 层** 🚧
- [x] `internal/handler/router.go` (Gin 路由注册和中间件装配)
- [x] `internal/handler/response.go` (统一响应格式和错误映射)
- [x] `internal/handler/source_handler.go` (POST, GET, PATCH /api/v1/sources, POST /api/v1/sources/:id/fetch)
- [x] `internal/handler/item_handler.go` (GET /api/v1/items, GET /api/v1/items/:id, GET /api/v1/feed/timeline)
- [x] `internal/handler/item_handler.go` 标记操作 (POST /api/v1/items/:id/read, /favorite, /hide；空请求体默认置为 true，请求体可显式传 false 取消状态)
- [x] `internal/handler/feed_view_handler.go` (GET, PUT /api/v1/feed/view-mode)
- [x] `internal/handler/middleware.go` CORS 中间件，允许本地 Vite 开发源跨域调用 API
- [x] 统一响应格式

**OpenAPI 文档** 🚧
- [x] `api/openapi.yaml` 最小前端契约：条目列表、条目详情、订阅源列表、阅读模式偏好
- [ ] 后端接口稳定后安装 swaggo/swag 或接入契约生成/校验流程
- [ ] 添加完整 OpenAPI 注解或补齐完整手写契约
- [ ] 配置 Swagger UI

**前端初始化** ✅
- [x] 初始化 Vue 3 + Vite 项目 (web/)
- [x] 安装 Arco Design Vue
- [x] 配置 Vue Router, Pinia, Axios
- [x] 配置 TypeScript
- [x] 配置 Vite 监听 `0.0.0.0:5173`，支持通过 Tailscale IP 访问开发服务
- [x] 建立无开发信息的 AppShell、覆盖式导航和基础页面入口

**前端页面** ⏸️
- [ ] 订阅源管理页 (/sources)
- [ ] 时间线页面 (/timeline)
- [ ] 条目详情页 (/items/:id)

**前端组件** ⏸️
- [ ] SourceList, SourceForm
- [ ] FeedTimeline, ItemCard
- [ ] ActionBar

**集成测试** ⏸️
- [ ] 前后端联调
- [ ] 端到端测试

**后端验证** ✅
- [x] `make verify`
- [x] `go test ./...`
- [x] `go vet ./...`
- [x] Docker Compose 空库迁移验证
- [x] Docker Compose API 容器健康检查
- [x] 真实 PostgreSQL 下验证 `POST /api/v1/sources`、`GET /api/v1/sources`、`PATCH /api/v1/sources/{id}`
- [x] 真实 PostgreSQL 下验证 `POST /api/v1/sources/{id}/fetch`
- [x] 重复抓取验证 `created_count=0`、`updated_count>0`，确认不会重复入库
- [x] 真实 PostgreSQL 下验证 `GET /api/v1/items`
- [x] 真实 PostgreSQL 下验证 `GET /api/v1/feed/timeline`
- [x] 真实 PostgreSQL 下验证 `POST /api/v1/items/{id}/read`、`POST /api/v1/items/{id}/favorite`、`POST /api/v1/items/{id}/hide`
- [x] 验证阅读状态首次并发写入、取消状态和不存在条目的 404 错误映射
- [x] 本地单元测试覆盖条目详情、列表状态过滤、阅读模式偏好默认值和保存行为
- [x] 真实 PostgreSQL 下验证 `GET /api/v1/items/{id}` 返回来源名称与阅读状态
- [x] 真实 PostgreSQL 下验证 `GET /api/v1/items` 的已读、隐藏和 `include_hidden` 过滤
- [x] 真实 PostgreSQL 下验证 `GET /api/v1/feed/view-mode`、`PUT /api/v1/feed/view-mode` 与非法模式 400 错误映射
- [x] 条目响应增加 `content_text`，前端可默认按纯文本展示；`content_snippet` 不作为直接 `v-html` 渲染来源

#### 验收标准
- [ ] 可以通过 Web 界面管理订阅源
- [x] 可以通过 API 手动触发抓取
- [x] 重复抓取不会重复入库
- [x] API 可以返回时间线模式条目，并在列表和详情中包含来源名称与阅读状态
- [ ] Web 界面显示时间线模式
- [x] API 可以标记已读、收藏、隐藏，并支持取消状态
- [x] API 可以保存和读取 Web 阅读模式偏好
- [x] API 已提供阶段二前端最小 OpenAPI 契约
- [ ] Web 界面可以标记已读、收藏、隐藏
- [ ] API 文档可在 Swagger UI 访问

---

实施范围：

- 将阶段一现有 `net/http` 路由迁移到 Gin，并保持健康检查、指标和运行时节点端点兼容。
- 实现订阅源 CRUD、RSS 手动抓取、条目去重入库、Feed 查询和阅读状态。
- 实现 Web 阅读模式偏好保存，阶段二只要求时间线模式可用。
- 建立 Web 阅读入口，先支持时间线模式。
- 第一版来源只要求标准 RSS、Atom、JSON Feed，使用 `gofeed` 解析。
- 阶段二不实现源目录、OPML 导入、推荐 Feed、周期调度、AI 摘要、通知和金融监控。

实施步骤：

1. 引入 Gin，将 `cmd/api` 中的 `http.NewServeMux` 路由迁移为 Gin engine；保留现有根路径、健康检查、就绪检查、指标和运行时节点端点的响应语义。
2. 在 `internal/handler` 建立路由注册入口、request id、访问日志、统一响应和错误映射；业务路由统一挂载在 `/api/v1`。
3. 建立 `sources`、`items`、`user_item_states`、`feed_view_preferences` 的迁移文件和仓储接口。
4. 在 `fetcher` 中封装 HTTP 抓取、超时、重定向、内容大小限制和 `gofeed` 解析。
5. 规范化 URL，建立 `sources(user_id, normalized_url)`、`items(source_id, normalized_url)` 和可选 `items(source_id, raw_guid)` 唯一约束；`raw_guid` 唯一约束只对非空值生效。
6. 实现 `POST /api/v1/sources`、`GET /api/v1/sources`、`PATCH /api/v1/sources/{id}`、`POST /api/v1/sources/{id}/fetch`。
7. 实现 `GET /api/v1/items`、`GET /api/v1/items/{id}`、`POST /api/v1/items/{id}/read`、`POST /api/v1/items/{id}/favorite`、`POST /api/v1/items/{id}/hide`。
8. 实现 `GET /api/v1/feed/timeline`，按 `published_at desc nulls last, fetched_at desc` 查询已订阅来源条目；条目响应直接包含 `source_name`、`is_read`、`is_favorite`、`is_hidden`、`content_text` 和对应时间戳，供前端直接渲染。
9. 实现 `GET /api/v1/feed/view-mode` 与 `PUT /api/v1/feed/view-mode`，读取并保存用户最近选择的 Web 阅读模式。
10. 在 `web` 中提供时间线模式入口，支持分页、来源过滤、已读、收藏和隐藏操作。
11. 抓取结果需要记录状态、耗时、条目数量、失败原因和最近抓取时间。
12. 先维护 `api/openapi.yaml` 中的前端最小契约；后端接口进一步稳定后，再补齐完整 OpenAPI 契约、契约校验和 Swagger UI。

实施细节：

**Gin 路由迁移（Go）**：
- `cmd/api` 只负责配置加载、依赖装配、Gin engine 创建、HTTP server 生命周期和优雅关闭。
- `internal/handler` 负责 Gin 路由注册、中间件、请求绑定、响应渲染和错误映射。
- `*gin.Context` 不进入 `service`、`repository`、`fetcher` 和 `domain`；业务层统一使用 `context.Context`。
- `/healthz`、`/readyz`、`/metrics` 和 `/api/runtime/node` 保持未版本化路径，便于监控和部署系统继续使用。
- 阶段二新增业务 API 统一使用 `/api/v1` 前缀。

**数据库模型（PostgreSQL）**：
- `sources`：记录 `user_id`、`name`、`type`、`url`、`normalized_url`、`status`、`fetch_interval_seconds`、`tags`、`weight`、最近抓取时间、最近抓取状态、最近错误、最近耗时和最近条目数量。
- `items`：记录 `source_id`、`title`、`url`、`normalized_url`、`raw_guid`、`content_hash`、`summary`、`content_snippet`、`author`、`published_at` 和 `fetched_at`。
- `user_item_states`：记录 `user_id`、`item_id`、`is_read`、`is_favorite`、`is_hidden` 及对应时间。
- `feed_view_preferences`：记录 `user_id`、最近阅读模式和更新时间，阶段二只允许 `timeline`，后续阶段扩展 `recommendations`。

**后端 API（Go）**：
- `POST /api/v1/sources` - 创建订阅源
- `GET /api/v1/sources` - 获取订阅源列表
- `PATCH /api/v1/sources/{id}` - 更新订阅源或调整启用状态
- `POST /api/v1/sources/{id}/fetch` - 手动触发抓取
- `GET /api/v1/items` - 获取 Feed 条目（支持分页、排序、来源过滤、已读过滤、收藏过滤、隐藏过滤）
- `GET /api/v1/items/{id}` - 获取条目详情
- `POST /api/v1/items/{id}/read` - 标记已读
- `POST /api/v1/items/{id}/favorite` - 收藏
- `POST /api/v1/items/{id}/hide` - 隐藏
- `GET /api/v1/feed/timeline` - 查询时间线
- `GET /api/v1/feed/view-mode` - 获取阅读模式偏好
- `PUT /api/v1/feed/view-mode` - 保存阅读模式偏好
- 集成 `gofeed` 解析 RSS/Atom/JSON Feed
- 基于 `source_id + normalized_url` 去重
- 条目响应提供 `content_text` 纯文本字段；`content_snippet` 可能包含外部来源 HTML，前端不得未经净化直接渲染

**Web 前端（Vue 3）**：
- 路由：`/sources` 订阅源管理，`/timeline` 时间线模式
- 组件：SourceList, SourceForm, FeedTimeline, ItemCard
- 状态：Pinia store 管理订阅源和条目数据
- 交互：实时刷新、下拉加载、标记操作

**阶段二前端现代化设计方案**：

参考项目已放置在 `../../references/awesome-design-md`、`../../references/impeccable` 和 `../../references/react-bits`。阶段二只吸收其设计方法，不直接复制品牌视觉或 React 代码：

- `awesome-design-md` 用作设计系统组织参考：在前端实现中沉淀颜色、字体、间距、圆角、组件状态和响应式规则，避免页面级样式各自定义。
- `impeccable` 用作质量约束参考：避免卡片嵌套、纯黑纯白、低对比文本、紫蓝渐变、玻璃拟态、无意义发光、过度居中、所有按钮同等强调和动画影响布局。
- `react-bits` 用作动效语法参考：只采用列表进入、状态切换、骨架加载和轻量反馈的思想；不引入背景特效、弹跳动画、霓虹效果和大面积文本动画。

设计定位：

- 产品类型为个人信息聚合与阅读工作台，首屏应直接进入可操作的时间线或订阅源管理，不做营销式首页。
- 视觉目标是高可读、高密度、低装饰。信息流卡片应接近邮件客户端和 RSS 阅读器的扫描效率，而不是瀑布流图片社区。
- 阶段二的推荐 Feed 入口只保留禁用态或后置入口提示，不实现推荐内容页，不影响当前最小闭环验收。

信息架构：

- 全局使用 `AppShell`：桌面端左侧主导航、顶部状态栏和主内容区；移动端收敛为顶部标题栏、内容区和底部导航。
- `/timeline` 是默认工作台视图。桌面端采用“来源筛选栏 + 条目列表 + 详情预览”的两到三栏布局；窄屏下列表与详情通过路由切换。
- `/sources` 采用表格与列表混合布局。桌面端使用表格展示来源名称、类型、状态、最近抓取、最近错误和操作；移动端使用紧凑列表。
- `/items/:id` 展示条目详情、来源、发布时间、纯文本正文、阅读原文入口和操作栏。阶段二默认使用 `content_text`，不直接渲染 `content_snippet` HTML。

核心页面设计：

- 时间线页顶部提供来源选择、状态筛选、刷新、只看未读、只看收藏、显示隐藏条目和分页加载。阅读模式切换使用分段控制，阶段二仅 `timeline` 可选。
- 条目列表使用紧凑卡片或列表项：标题、来源、时间、作者、摘要纯文本、状态标记和操作按钮必须在固定结构内稳定排布，避免 hover 或加载状态导致布局跳动。
- 条目详情页优先展示安全纯文本正文，并提供“阅读原文”按钮。若后续增加 iframe 内嵌，应作为独立标签页或预览区，并保留失败兜底。
- 订阅源管理页通过 Drawer 承载新增和编辑表单。阶段二不提供删除按钮，启停使用 Switch，抓取使用明确的图标按钮和加载态。

视觉系统：

- 基础浅色主题：页面背景 `#f6f8fb`，主表面 `#ffffff`，次级表面 `#eef3f7`，主文本 `#172033`，次级文本 `#526173`，边框 `#d8e0ea`。
- 强调色采用多语义而非单一蓝色：主操作 `#2563eb`，成功 `#0f8a5f`，警告 `#b7791f`，错误 `#c2410c`，信息辅助 `#0e7490`。
- 深色主题使用带色相的深灰蓝背景，避免纯黑；保持正文对比度优先，来源标签和时间信息不得低于可读阈值。
- 圆角以 `6px` 和 `8px` 为主，按钮、输入框、列表项和卡片保持一致；不使用大圆角玻璃卡片。
- 字体使用系统中文优先栈：`ui-sans-serif, "Noto Sans SC", "PingFang SC", "Microsoft YaHei", sans-serif`。正文不使用负字距，不用大字号标题占据工作台空间。
- 间距基于 4px 栅格。列表项、工具栏和表格行使用稳定高度，确保筛选、加载和状态切换不会改变整体布局。

组件规范：

- 基础组件继续使用 Arco Design Vue；图标按钮优先使用既有图标库，不手写 SVG。
- `SourceList`：桌面端表格，移动端列表；提供启停、抓取、编辑和错误查看。
- `SourceForm`：Drawer 表单，字段包括 URL、名称、类型、抓取间隔、标签和权重；URL 为唯一必填项。
- `FeedTimeline`：封装查询条件、分页、加载状态、空状态和错误状态。
- `ItemCard`：负责标题、元信息、摘要和状态操作。已读项降低标题权重但不降低正文到不可读。
- `ActionBar`：提供已读、收藏、隐藏、阅读原文。危险或不可逆操作不得使用主按钮样式。

交互与动效：

- 动效只服务状态理解：列表新增、筛选切换、保存成功、抓取中和骨架加载可以使用 120ms 到 180ms 的透明度或位移动画。
- 禁止使用会改变布局尺寸的动画；禁用弹跳、抖动、背景粒子、霓虹发光和持续循环动效。
- 支持 `prefers-reduced-motion`，在用户要求降低动态效果时关闭非必要过渡。
- 所有异步操作需要显式状态：加载、成功、失败、空数据、部分失败和重试入口。

响应式规则：

- `>= 1200px`：左侧导航、来源筛选栏、条目列表和详情预览可并列展示。
- `768px - 1199px`：保留左侧导航和主列表，详情通过路由或抽屉打开。
- `< 768px`：单列布局，底部导航承载时间线和来源入口，筛选使用 Drawer，操作按钮保持至少 44px 触控尺寸。

API 与安全约束：

- 前端实现前应补齐或本地声明 `POST /api/v1/sources`、`PATCH /api/v1/sources/{id}`、`POST /api/v1/sources/{id}/fetch`、`POST /api/v1/items/{id}/read`、`POST /api/v1/items/{id}/favorite` 和 `POST /api/v1/items/{id}/hide` 的类型。
- Axios 统一解析 `{code, message, data, request_id, trace_id}`，错误提示应展示用户可理解的信息，并在详情中保留 `request_id` 便于日志追踪。
- 阶段二默认不使用 `v-html`。如确需展示 HTML，必须引入 DOMPurify 或等价净化流程，并保留纯文本兜底。
- 前端不得直接访问数据库，不复刻后端去重、抓取和业务状态规则。

实现顺序：

1. 补齐 `web` 工程：Vue 3、Vite、TypeScript、Arco Design Vue、Pinia、Vue Router、Axios 和基础样式令牌。
2. 建立 `api` 客户端、统一响应解包、错误处理和请求追踪字段展示。
3. 实现 `AppShell`、主题变量、导航、加载态、空状态和错误态。
4. 实现订阅源管理页，先打通新增、编辑、启停和手动抓取。
5. 实现时间线页，支持分页、来源过滤、已读、收藏、隐藏和显示隐藏条目。
6. 实现条目详情页和操作栏，完成“订阅源 -> 抓取 -> 时间线 -> 详情 -> 阅读状态”的 Web 闭环。
7. 完成桌面与移动端目视核查，并用正式前端命令执行类型检查、构建和必要的单元测试。

**技术栈**：
- 后端：Go + Gin + gofeed + OpenAPI YAML 契约
- 前端：Vue 3 + Vite + Arco Design Vue + Pinia + Vue Router + Axios

验收标准：

- ✅ 可以通过 API 新增 RSS 源并手动触发抓取
- ✅ 重复抓取不会重复入库（后端去重逻辑验证）
- ✅ API 可以返回时间线模式条目，按时间倒序展示
- ✅ API 条目列表和详情已包含来源名称与用户阅读状态
- ✅ API 可以标记已读、收藏和隐藏，并支持取消状态
- ✅ API 可以读取和保存阅读模式偏好
- ⏳ Web 界面新增 RSS 源、手动抓取、时间线展示和标记操作仍待实现
- ✅ API 已提供阶段二前端最小 OpenAPI 契约；完整契约、契约校验和 Swagger UI 后置补充
- ✅ 阶段一已有健康检查、就绪检查、指标和运行时节点端点在迁移到 Gin 后保持兼容

风险控制：

- Gin 迁移不得改变已有基础端点的响应语义，避免破坏 Docker healthcheck、Prometheus 抓取和 Tailscale 访问验证。
- Gin 中间件只承载横切关注点，不在中间件中写订阅、抓取和阅读状态业务规则。
- 抓取任务必须设置超时，避免阻塞请求或调度器。
- 外部源返回异常编码、异常 MIME、空 feed 或重复 GUID 时必须有可诊断错误。
- 抓取器只允许 `http` 和 `https` URL，并限制重定向次数和响应体大小，避免外部输入长期占用资源。

### <a id="phase-3"></a>阶段三：日志、错误追踪与链路观测

**触发条件**：阶段二完成“订阅源 -> 手动抓取 -> 去重入库 -> 时间线展示 -> 阅读状态”最小业务闭环后立即启动。该阶段完成前，不进入源目录、推荐 Feed、AI 摘要、通知或金融监控的主体开发。

实施范围：

- 将当前基础 `log/slog` 日志升级为结构化 JSON 日志。
- 将 request id 从 Gin 上下文打通到标准 `context.Context`。
- 接入 OpenTelemetry，建立 HTTP 入口 trace，并为 service、repository、fetcher、notifier、llm、scheduler 预留 span 模式。
- 建立统一错误模型和 handler 层错误映射。
- 完善 panic recovery，确保 panic 与 request id、trace id、method、path 关联。
- 将日志存储、指标、trace 和 Dashboard 纳入 Docker Compose 可选观测组件。

实施步骤：

1. [x] 新增 `internal/observability` 模块，统一初始化 logger、request id、trace id、span id、tracer provider 和 shutdown 钩子。
2. [x] 将 `slog` 输出从 text handler 调整为 JSON handler，固定字段包括 `service`、`environment`、`node_id`、`request_id`、`trace_id`、`span_id` 和 `error`。
3. [x] 保持应用日志输出到 stdout/stderr，不让业务进程直接写日志文件；本地继续使用 Docker `json-file` 轮转，完整观测环境使用 Loki 查询日志。
4. [x] 修改 request id 中间件，将 `X-Request-ID` 写入响应头、Gin 上下文和 `context.Context`，并提供从 context 读取 request id 的辅助函数。
5. [x] 接入 OpenTelemetry Gin 中间件，新增 `OBSERVABILITY_TRACE_ENABLED`、`OTEL_SERVICE_NAME`、`OTEL_EXPORTER_OTLP_ENDPOINT`、`ENVIRONMENT` 和采样配置。
6. [x] 在 `cmd/api` 中初始化 tracer provider，并在优雅关闭时 flush exporter。
7. [x] 为当前已实现业务的 HTTP、service、repository 和 fetcher 定义 span 命名规则，例如 `service.source.trigger_fetch`、`repository.source.create`、`fetcher.feed.fetch`。
8. 建立统一错误类型，包含业务错误码、HTTP 状态码、用户可读消息、内部错误链、操作名和是否可重试。
9. [x] handler 层统一渲染错误响应，响应中包含 `code`、`message`、`request_id` 和 `trace_id`。
10. [x] Recovery 中间件捕获 panic 后记录 request id、trace id、span id、method、path 和 panic 摘要，并返回统一 500 响应。
11. [x] 完善 Prometheus 指标，增加 Feed 抓取失败、抓取耗时、外部调用耗时、数据库查询耗时和数据库连接池等待指标。
12. [x] Docker Compose 增加 `prometheus`、`grafana`、`loki`、`promtail`、`tempo`、`otel-collector` 配置。
13. [x] 建立 Grafana Dashboard 草案，展示请求量、HTTP P95 耗时、HTTP 状态、数据库连接池、抓取次数、外部调用耗时和 API 日志。
14. [x] 补充磁盘保护策略：所有 Compose 服务统一 Docker `json-file` 日志轮转，Prometheus 本地保留 7 天，Loki 保留 168 小时并启用 compactor retention 删除，Tempo trace 保留 24 小时。
15. [x] 将 `/healthz` 和 `/metrics` 的成功访问日志降为 debug，避免健康检查和 Prometheus 抓取在 info 级别持续写入 Loki；失败请求仍保留可追踪日志。

验收标准：

- 用户或前端拿到 `request_id` 后，可以在日志系统中查询该请求的入口日志、错误日志和关键下游操作。
- 一次 API 请求可以在 trace 系统中看到 HTTP 入口、handler、service、repository 或外部调用 span。
- panic、业务错误、数据库错误和外部依赖错误都有统一响应结构和服务端结构化日志。
- `/metrics` 能展示请求量、错误率、耗时和数据库连接池状态。
- 完整观测 Docker Compose 启动后，可以通过 Grafana 查询日志、指标和 trace。
- API 容器、观测组件和数据库容器均有 Docker 日志轮转；Loki、Prometheus 和 Tempo 均有本地保留窗口，避免本地验证环境无界增长。
- `make verify` 继续通过，新增观测相关单元测试覆盖 request id 上下文传播、错误映射和 recovery。

风险控制：

- 日志不得输出密钥、token、Webhook URL、数据库 DSN、AI API key 或用户敏感正文。
- 观测系统自身也会产生日志和指标，必须保留轮转和保留期；后续生产环境上线前需要根据磁盘容量、采样率、日志量和查询窗口重新评估保留策略。
- trace attribute 不得写入大正文、完整文章内容、模型提示词全文或敏感配置。
- 指标 label 不得使用高基数字段，例如原始 URL、用户输入文本、完整错误消息或 request id。
- Sentry 或其他错误聚合平台只作为后续增强，不替代日志、指标和 trace 的基础链路。

### <a id="phase-4"></a>阶段四：源目录与导入

实施范围：

- 建立推荐源目录、OPML 导入、URL 批量导入和从目录批量订阅。
- 源目录借鉴 Folo 的 discover sources 与 onboarding feed，但必须记录来源出处和许可状态。

实施步骤：

1. 建立 `source_catalog_entries` 和 `source_import_jobs`。
2. 为候选源记录名称、URL、站点、分类、语言、热度、来源出处、许可状态、健康状态和最近校验时间。
3. 实现目录查询、关键词搜索、分类过滤、语言过滤和批量订阅。
4. 实现 OPML 解析，输出成功、失败和跳过的明细。
5. 实现 URL 批量导入，每个 URL 独立校验，单个失败不影响整体任务。
6. 建立导入任务状态，保留原始错误摘要，便于用户修正源地址。

验收标准：

- 可以通过 API 搜索推荐源。
- 可以导入 OPML，并返回成功与失败明细。
- 可以从推荐源目录批量创建订阅。
- 失败源不会阻断其他源导入。

风险控制：

- 不直接复制第三方源数据为正式内置数据，先作为候选来源并核查许可。
- 目录源需要定期健康检查，避免大量不可用源影响用户体验。

### <a id="phase-5"></a>阶段五：自动化、兴趣规则与推荐 Feed

实施范围：

- 接入周期抓取、失败重试、抓取状态维护和基础兴趣评分。
- 增加推荐 Feed 模式，推荐条目可以来自已订阅来源和未订阅候选来源。
- 第一版使用 `gocron`，后续再评估 River 或其他任务队列。

实施步骤：

1. 在 `scheduler` 中定义抓取任务、日报任务、行情任务等任务类型，但本阶段只启用抓取任务。
2. 为每个源计算下次抓取时间，支持默认周期和单源自定义周期。
3. 实现失败重试策略，至少包含最大重试次数、退避间隔和最后失败原因。
4. 建立 `interest_rules`，支持关键词、来源权重、标签和语言。
5. 建立 `feed_view_preferences`、`feed_recommendations`、`recommendation_feedback`。
6. 增加 `recommender`，候选池包含已订阅来源条目、推荐源目录条目、健康候选源条目和经过标注的桥接来源条目。
7. 推荐排序至少使用兴趣规则、阅读反馈、来源权重、内容新鲜度、来源健康状态和重大事件上下文。
8. 实现 `GET /api/v1/feed/recommendations` 和 `POST /api/v1/feed/recommendations/{id}/feedback`。
9. Web 界面支持推荐 Feed 模式切换，并对未订阅来源展示来源名称、未订阅标记、推荐原因和一键订阅入口。
10. 调度执行前调用 `TaskLocker` 接口，第一阶段可用单节点实现，后续切换共享数据库锁。

验收标准：

- 启用的源可以按周期自动抓取。
- 失败抓取会记录错误并可后续重试。
- Feed 查询可以按兴趣评分排序或过滤。
- Web 界面可以切换到推荐 Feed 模式。
- 推荐 Feed 可以同时展示已订阅和未订阅来源条目。
- 未订阅来源条目会标注来源状态、推荐原因，并支持一键订阅或不感兴趣反馈。
- 重启服务后不会丢失源抓取状态。

风险控制：

- 调度任务不得无限并发，需限制同一时间的抓取数量。
- 抓取任务和 API 请求共享数据库时，需要避免长事务和大批量写入阻塞查询。
- 推荐 Feed 不得将用户阅读未订阅内容解释为自动订阅。
- 未订阅来源必须展示来源出处、健康状态和桥接风险，不得与已订阅来源混同。

### <a id="phase-6"></a>阶段六：AI 摘要与通知

实施范围：

- 定义 `LLMClient`，实现日报摘要、重大事件判断和通知发送。
- MVP 通知通道支持 `ntfy` 和微信单向通知，微信优先企业微信机器人或微信公众号测试号。

实施步骤：

1. 建立 `summaries`、`notification_channels`、`notification_recipients`、`notifications`。
2. 在 `llm` 中定义模型请求、结构化输出、token 统计、耗时和错误记录。
3. 实现日报摘要：按时间窗口选取高相关条目，输出重点条目、事件聚类和简短摘要。
4. 实现重大事件判断：对高权重命中内容进行模型判断，但通知触发必须保存明确原因。
5. 在 `notifier` 中抽象 `ntfy`、企业微信机器人、企业微信自建应用、微信公众号测试号。
6. 通知记录必须保存通道、接收目标、触发原因、状态、失败原因、模型、token、耗时和 `dedupe_key`。

验收标准：

- 可以手动生成一次日报摘要。
- 可以发送 `ntfy` 测试通知。
- 可以发送微信测试通知。
- 通知失败可被查询并定位原因。
- 同一日报或同一重大事件不会重复发送。

风险控制：

- 微信凭证必须通过环境变量或外部配置注入。
- 个人微信桥接仅作实验，不进入第一版验收。
- 摘要任务必须记录 token、耗时和错误，便于成本分析。

### <a id="phase-7"></a>阶段七：自然语言设置控制

实施范围：

- 增加 `control` 模块，支持自然语言设置指令、结构化意图、变更计划、确认策略、受控执行和审计记录。
- 支持用户通过自然语言增减订阅、调整提示事件、订阅官方号或官方文章桥接源、配置 AI 摘要提醒、调整通知偏好和修改金融告警规则。
- 覆盖全部用户可配置项；后续新增设置能力时必须同步注册控制能力、风险等级、确认策略和回滚方式。
- 默认执行模式为 `approval_required`，即模型生成计划，用户确认后执行。

实施步骤：

1. 建立 `control_commands`、`control_change_plans`、`control_change_steps`、`control_audit_logs`。
2. 建立 `control_capabilities` 或等价能力注册机制，记录每类设置的目标类型、允许操作、风险等级、确认策略和回滚支持。
3. 定义 `ControlInterpreter`，将自然语言指令解析为结构化意图、目标对象、操作类型、置信度和歧义点。
4. 定义 `ChangePlanner`，生成变更计划，展示变更前后差异、影响范围、风险等级、成本估算和回滚摘要。
5. 定义 `ControlExecutor`，只允许通过既有 service 接口执行设置变更，不允许直接写数据库或直接调用 repository。
6. 支持三种执行模式：`suggest_only`、`approval_required`、`delegated`。第一版只启用前两种，`delegated` 先保留策略接口。
7. 为高风险操作建立确认策略，包括批量停用订阅、提高通知频率、修改通知接收人、增加模型成本、修改金融告警和永久删除请求。
8. 对“删除”类自然语言默认解释为停用或归档；永久删除必须二次确认。
9. 记录模型版本、提示词版本、用户确认记录、执行结果、失败原因和审计日志。

验收标准：

- 用户输入“帮我订阅 Go 官方博客和某公众号文章，每天早上摘要提醒”后，系统可以生成结构化变更计划。
- 用户确认后，系统可以新增订阅源或候选桥接源，并创建摘要提醒设置。
- 对“关闭最近新增的所有财经提醒”等批量或歧义指令，系统会要求确认或澄清。
- 执行后的设置变更可以查询审计记录。
- 模型无法接触密钥、token、Webhook URL 等敏感明文。
- 新增用户设置项如果没有注册控制能力，不能被自然语言控制面自动执行。

风险控制：

- 模型只生成意图、计划和说明文本，不拥有直接数据库写入能力。
- 模型能力不足、对象匹配不唯一或影响范围过大时，必须向用户追问。
- 通知接收目标、金融告警和成本增加类操作默认需要用户确认。
- 该阶段不实现通用无限工具执行、浏览器自动化或任意系统命令能力。

### <a id="phase-8"></a>阶段八：金融市场监控与 AI 告警

实施范围：

- 增加 `market` 与 `alert` 模块。
- 支持指数、股票、ETF 等金融标的。
- 支持行情快照、关注列表、阈值规则、告警事件、AI 解读和微信或 ntfy 通知。

实施步骤：

1. 建立 `market_instruments`、`market_data_providers`、`market_quotes`、`market_watchlists`、`market_alert_rules`、`market_alert_events`。
2. 定义 `MarketDataProvider`，包含 `Quote`、`BatchQuotes`、`ProviderStatus` 和 `Capabilities`。
3. 第一版选择一个低成本 provider 完成链路验证。A 股指数优先评估 Tushare、AkShare HTTP 化服务、东方财富或新浪财经；全球指数和 ETF 优先评估 Alpha Vantage、Finnhub 或 Yahoo Finance 原型。
4. 行情快照必须记录当前价、前收价、涨跌幅、成交量、行情时间、数据源和延迟属性。
5. 定义 `MarketAlertEngine`，先支持 `day_change_pct_abs_gte`、`day_change_pct_gte`、`day_change_pct_lte`、`price_cross_above`、`price_cross_below`、`volume_ratio_gte`。
6. 告警命中后生成 `market_alert_events`，通过 `dedupe_key` 和冷却时间避免重复发送。
7. AI 只在确定性规则命中后生成解释文本，输入包括行情快照、规则、相关 Feed 内容和风险提示。
8. 金融通知复用 `notifier`，发送内容包含标的、当前价、涨跌幅、触发阈值、行情时间、数据源、AI 简述和“不构成投资建议”提示。

验收标准：

- 可以新增一个指数标的。
- 可以配置“当日涨跌幅绝对值大于等于 2%”规则。
- 可以拉取行情并保存快照。
- 当规则命中时，可以生成 AI 解读。
- 可以通过微信或 ntfy 发送金融告警。
- 同一规则在冷却时间内不会重复发送。

风险控制：

- 行情数据源必须记录授权状态、实时性、典型延迟和速率限制。
- 非官方或网页逆向来源只能作为个人实验或原型候选。
- AI 金融解读不得输出确定性买卖建议。
- 本项目不接入自动交易、券商账户和高频行情。

### <a id="phase-9"></a>阶段九：工程化增强

实施范围：

- 在阶段三完整观测系统的基础上，增加 OpenAPI 契约、集成测试、业务指标扩展、Dashboard 迭代和更完整的部署配置。

实施步骤：

1. 将当前 API 固化为 OpenAPI 文档，并在 `make verify` 中增加契约检查。
2. 增加基于真实 PostgreSQL 的集成测试，优先覆盖源导入、抓取去重、摘要记录、通知记录、自然语言设置控制和金融告警。
3. 完善 Docker Compose，纳入可选 ntfy 和 Redis；Prometheus、Grafana、Loki、Tempo、OpenTelemetry Collector 沿用阶段三观测 profile 并按业务需要扩展。
4. 扩展核心业务指标：摘要耗时、控制计划成功率、通知成功率、行情拉取成功率、告警触发次数。
5. 迭代 Grafana Dashboard，按采集、摘要、设置控制、通知、行情和告警分类展示。

验收标准：

- `make verify` 覆盖格式检查、单元测试、集成测试、构建和契约检查。
- 指标能在阶段三基础上继续展示摘要耗时、控制计划成功率、行情拉取成功率、告警触发次数和通知成功率。
- `make compose-up` 后可访问服务、数据库和可选观测组件。

风险控制：

- 测试应复用正式迁移文件，不维护第二套测试 schema。
- Dashboard 不应依赖固定本机绝对路径。

### <a id="phase-10"></a>阶段十：来源扩展与分布式升级验证

实施范围：

- 扩展非标准来源，并验证后续分布式部署路径。
- 分布式验证不改变业务模块，只替换入口层、运行时配置和任务锁实现。

实施步骤：

1. 抽象 `SourceConnector`，为 RSSHub、YouTube、网页变化源和后续 Agent 型来源预留统一入口。
2. 接入 RSSHub 路由时记录桥接来源、原始 URL、稳定性和潜在失效风险。
3. YouTube 优先使用频道 RSS；结构化播放量、评论和搜索后续再评估 YouTube Data API。
4. 验证 Cloudflare Tunnel 将单节点服务暴露到域名。
5. 验证 Cloudflare Load Balancer 根据 `/readyz` 将访问转发到健康节点。
6. 验证多个 API 节点共享 PostgreSQL 时 API 层保持无状态。
7. 将 `TaskLocker` 切换为 PostgreSQL advisory lock、任务表锁或 Redis 锁，验证多节点 scheduler 不重复执行。
8. 通过 `notifications.dedupe_key`、`control_change_plans.dedupe_key` 和 `market_alert_events.dedupe_key` 验证日报、设置控制和金融告警不会重复执行。

验收标准：

- YouTube 频道 RSS 可以作为普通源订阅。
- RSSHub 路由源可以被记录为桥接源。
- 新来源不破坏原有 RSS 抓取链路。
- 关闭一个 API 节点后，健康入口可以将访问切换到仍可用节点。
- 多节点同时运行 scheduler 时不会重复发送同一份日报。
- 多节点同时运行自然语言设置控制执行器时不会重复执行同一份变更计划。
- 多节点同时运行金融行情轮询时不会重复发送同一条金融告警。

风险控制：

- Cloudflare 入口只作为后续阶段，不阻塞第一阶段本地部署。
- 多节点验证必须基于共享数据库锁和幂等记录，不依赖进程内锁。

## 5. 优先级裁剪

必须优先完成：

- 工程基线。
- Tailscale 简单远程访问。
- RSS 手动抓取。
- 去重入库。
- Feed 查询。
- Web 时间线模式。
- 日志、错误追踪和链路观测系统。
- 推荐 Feed 模式。
- OPML 导入。
- 日报摘要。
- 微信单向通知。
- 自然语言设置控制。
- 指数行情监控与阈值告警。

可以延后：

- Cloudflare 入口和多节点故障转移。
- 通用无限工具执行 Agent 平台。
- X 深度接入。
- 浏览器自动化采集。
- 独立向量数据库。
- 多用户权限体系。
- WebPush 和移动端原生推送。
- 自动交易和券商账户接入。
- 完整量化回测系统。

## 6. 参考项目阅读顺序

1. `references/miniflux_v2`、`references/gofeed`、`references/rsshub`：RSS 主链路。
2. `references/rssnext_folo`：源目录、推荐源、订阅体验。
3. `references/hermes_agent`、`references/openclaw`：微信通道与 Agent 消息网关。
4. `references/gocron`、`references/river`、`references/asynq`：调度与异步任务。
5. `references/openai_go`、`references/eino`、`references/eino_ext`：AI 调用和编排。
6. `references/ntfy`、`references/gotify_server`：推送服务。
7. QuantConnect LEAN、AkShare、Tushare、Yahoo Finance、Finnhub、Polygon、Alpha Vantage、TradingView Alert、Grafana Alerting：金融行情、告警和数据源设计参考。

## 7. 最小验收命令

```bash
make compose-up
make verify
```

最终项目应在冷启动后通过 `/healthz` 与 `/readyz`，可以在 Tailscale 网络内访问，并能完成“新增订阅源 -> 抓取 -> Web 时间线浏览 -> 推荐 Feed 浏览 -> 生成摘要 -> 发送通知”、“自然语言指令 -> 变更计划 -> 用户确认 -> 设置调整 -> 审计记录”和“新增金融标的 -> 拉取行情 -> 规则命中 -> AI 解读 -> 微信通知”的闭环。
