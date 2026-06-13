# messageFeed 实施文档

**最后更新**：2026-06-13

---

## 📊 实施进度总览

| 阶段 | 名称 | 状态 | 完成度 | 开始日期 | 完成日期 |
|------|------|------|--------|----------|----------|
| 阶段一 | 基础设施搭建 | ✅ 完成 | 100% | 2026-06-12 | 2026-06-13 |
| 阶段二 | 订阅源与 Feed 闭环 | 🚧 进行中 | 0% | 2026-06-13 | - |
| 阶段三 | 源目录与导入 | ⏸️ 未开始 | 0% | - | - |
| 阶段四 | 自动化与推荐 | ⏸️ 未开始 | 0% | - | - |
| 阶段五 | AI 摘要与通知 | ⏸️ 未开始 | 0% | - | - |
| 阶段六 | 自然语言控制 | ⏸️ 未开始 | 0% | - | - |
| 阶段七 | 金融监控 | ⏸️ 未开始 | 0% | - | - |

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
| `migrations` | PostgreSQL 正式迁移文件 | 迁移文件必须可重复应用校验，不维护第二套测试 schema |
| `deploy` | Dockerfile、docker-compose、Cloudflare、Prometheus、Grafana 等部署材料 | 路径配置使用相对路径；本地部署和后续分布式部署配置分层放置 |
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
- 迁移文件使用递增版本号和语义名称，例如 `000001_create_sources.sql`。
- 配置示例可以使用 `.env.example`，不得提交真实密钥。

## 3. 阶段路线

| 阶段 | 目标 | 详细实施 |
| --- | --- | --- |
| 阶段一 | 本地工程基线与 Tailscale 访问 | [查看细节](#phase-1) |
| 阶段二 | 订阅源与 Feed 闭环 | [查看细节](#phase-2) |
| 阶段三 | 源目录与导入 | [查看细节](#phase-3) |
| 阶段四 | 自动化、兴趣规则与推荐 Feed | [查看细节](#phase-4) |
| 阶段五 | AI 摘要与通知 | [查看细节](#phase-5) |
| 阶段六 | 自然语言设置控制 | [查看细节](#phase-6) |
| 阶段七 | 金融市场监控与 AI 告警 | [查看细节](#phase-7) |
| 阶段八 | 工程化增强 | [查看细节](#phase-8) |
| 阶段九 | 来源扩展与分布式升级验证 | [查看细节](#phase-9) |

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
- [x] Docker Compose 自动迁移

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
- [x] docker-compose up -d 一键启动
- [x] /healthz 返回成功
- [x] /readyz 包含数据库检查
- [x] /metrics 暴露指标
- [x] make verify 全部通过
- [x] 数据库自动迁移成功

---

实施范围：

- 建立 Go 服务骨架、配置加载、结构化日志、统一错误、基础路由。
- 接入 PostgreSQL、GORM 和正式 SQL 迁移文件。
- 暴露 `/healthz`、`/readyz`、`/metrics` 和 `/api/runtime/node`。
- 提供 `docker-compose`、`Makefile` 和最小 `make verify`。
- 支持 `BIND_ADDR`、`PUBLIC_BASE_URL`、`APP_NODE_ID`、`DEPLOYMENT_MODE`、`TRUSTED_PROXY_CIDRS`。
- 本地默认使用 `DEPLOYMENT_MODE=single_node`，远程访问通过 `BIND_ADDR` 暴露到局域网、Tailscale IP 或 MagicDNS 完成。
- PostgreSQL 作为第一阶段唯一主存储；Redis 不进入第一阶段 `docker-compose` 必需组件，但预留后续缓存、队列、限流和任务锁接口。

实施步骤：

1. 创建 `cmd/api`、`internal/config`、`internal/handler`、`internal/runtime`、`internal/repository`、`deploy`、`api` 等基础目录。
2. 在配置层定义数据库、HTTP、运行时、日志和后续外部服务的配置结构；密钥只从环境变量或外部配置注入。
3. 建立 Gin 路由，统一错误响应结构和 request id 日志字段。
4. 建立数据库连接、迁移执行入口和 `/readyz` 依赖检查。
5. 增加 Prometheus 指标注册，至少覆盖请求量、请求耗时、健康状态和数据库连接状态。
6. 在 `docker-compose` 中只纳入服务本体和 PostgreSQL，不在第一阶段引入 Redis；与缓存、队列、限流和任务锁相关的能力通过接口预留。
7. 在 Tailscale 场景下将 `BIND_ADDR` 配置为 `0.0.0.0:8080` 或 Tailscale IP，并将 `PUBLIC_BASE_URL` 配置为 Tailscale IP 或 MagicDNS 访问地址。

验收标准：

- `docker-compose up` 可以启动服务和 PostgreSQL。
- `/healthz` 返回成功。
- `/readyz` 能检查数据库连接和迁移状态。
- `/metrics` 可以被 Prometheus 格式读取。
- Tailscale 网络内设备可以访问 API。
- `make verify` 可以执行格式检查、构建和基础测试。
- `/api/runtime/node` 能返回 `deployment_mode=single_node`、节点标识、监听配置和公开访问基址。

风险控制：

- API 层不得依赖本机内存保存业务状态。
- 第一阶段任务锁可以是单节点或 PostgreSQL 实现，但接口必须保留，后续允许替换为 PostgreSQL advisory lock、任务表锁或 Redis 锁。
- `/readyz` 不检查非关键外部服务，避免微信、AI 或行情源短暂不可用导致服务被错误摘除。
- Redis 不作为主存储或审计存储；业务幂等、持久状态和可追溯记录必须落入 PostgreSQL。

### <a id="phase-2"></a>阶段二：订阅源与 Feed 闭环 🚧

**状态**：🚧 进行中 | **开始时间**：2026-06-13 | **完成度**：0%

#### 实施进度清单

**数据库设计** ⏸️
- [ ] 创建 migrations/000002_add_sources_items.up.sql
- [ ] 定义 sources 表
- [ ] 定义 items 表
- [ ] 定义 user_item_states 表
- [ ] 添加索引和约束
- [ ] 执行迁移验证

**领域模型** ⏸️
- [ ] internal/domain/source.go
- [ ] internal/domain/item.go
- [ ] internal/domain/user_item_state.go
- [ ] 枚举和错误定义

**Repository 层** ⏸️
- [ ] internal/repository/source_repository.go (Create, Get, List, Update, Delete)
- [ ] internal/repository/item_repository.go (Create, BatchCreate, List)
- [ ] internal/repository/user_item_state_repository.go (MarkRead, Favorite, Hide)

**Fetcher 模块** ⏸️
- [ ] internal/fetcher/fetcher.go
- [ ] 集成 gofeed
- [ ] HTTP 抓取（超时、重定向、大小限制）
- [ ] URL 规范化
- [ ] 去重逻辑
- [ ] 错误处理

**Service 层** ⏸️
- [ ] internal/service/source_service.go (CRUD + TriggerFetch)
- [ ] internal/service/feed_service.go (FetchAndStore)
- [ ] internal/service/timeline_service.go (GetTimeline)
- [ ] internal/service/item_service.go (MarkRead, Favorite, Hide)

**Handler 层** ⏸️
- [ ] internal/handler/source_handler.go (POST, GET, PATCH, DELETE /api/v1/sources)
- [ ] internal/handler/item_handler.go (GET /api/v1/items, 标记操作)
- [ ] internal/handler/feed_handler.go (GET /api/v1/feed/timeline)
- [ ] 统一响应格式

**OpenAPI 文档** ⏸️
- [ ] 安装 swaggo/swag
- [ ] 添加 OpenAPI 注解
- [ ] 生成文档
- [ ] 配置 Swagger UI

**前端初始化** ⏸️
- [ ] 初始化 Vue 3 + Vite 项目 (web/)
- [ ] 安装 Arco Design Vue
- [ ] 配置 Vue Router, Pinia, Axios
- [ ] 配置 TypeScript

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

#### 验收标准
- [ ] 可以通过 Web 界面管理订阅源
- [ ] 可以手动触发抓取
- [ ] 重复抓取不会重复入库
- [ ] Web 界面显示时间线模式
- [ ] 可以标记已读、收藏、隐藏
- [ ] API 文档可在 Swagger UI 访问

---

实施范围：

- 实现订阅源 CRUD、RSS 手动抓取、条目去重入库、Feed 查询和阅读状态。
- 建立 Web 阅读入口，先支持时间线模式。
- 第一版来源只要求标准 RSS、Atom、JSON Feed，使用 `gofeed` 解析。

实施步骤：

1. 建立 `sources`、`items`、`user_item_states` 的迁移文件和仓储接口。
2. 在 `fetcher` 中封装 HTTP 抓取、超时、重定向、内容大小限制和 `gofeed` 解析。
3. 规范化 URL，建立 `sources(user_id, normalized_url)`、`items(source_id, normalized_url)` 和可选 `items(source_id, raw_guid)` 唯一约束。
4. 实现 `POST /api/sources`、`GET /api/sources`、`PATCH /api/sources/{id}`、`POST /api/sources/{id}/fetch`。
5. 实现 `GET /api/items`、`GET /api/items/{id}`、`POST /api/items/{id}/read`、`POST /api/items/{id}/favorite`。
6. 实现 `GET /api/feed/timeline`，按 `published_at desc` 查询已订阅来源条目；缺失发布时间时使用 `fetched_at desc` 兜底。
7. 实现 `PUT /api/feed/view-mode`，保存用户最近选择的 Web 阅读模式。
8. 在 `web` 中提供时间线模式入口，支持分页、来源过滤、已读、收藏和隐藏操作。
9. 抓取结果需要记录状态、耗时、条目数量、失败原因和最近抓取时间。

实施细节：

**后端 API（Go）**：
- `POST /api/v1/sources` - 创建订阅源
- `GET /api/v1/sources` - 获取订阅源列表
- `POST /api/v1/sources/{id}/fetch` - 手动触发抓取
- `GET /api/v1/items` - 获取 Feed 条目（支持分页、排序、过滤）
- `POST /api/v1/items/{id}/mark-read` - 标记已读
- `POST /api/v1/items/{id}/favorite` - 收藏
- `POST /api/v1/items/{id}/hide` - 隐藏
- 集成 `gofeed` 解析 RSS/Atom/JSON Feed
- 基于 `source_id + normalized_url` 去重

**Web 前端（Vue 3）**：
- 路由：`/sources` 订阅源管理，`/timeline` 时间线模式
- 组件：SourceList, SourceForm, FeedTimeline, ItemCard
- 状态：Pinia store 管理订阅源和条目数据
- 交互：实时刷新、下拉加载、标记操作

**技术栈**：
- 后端：Go + gofeed + OpenAPI 注解
- 前端：Vue 3 + Vite + Arco Design Vue + Pinia + Vue Router + Axios

验收标准：

- ✅ 可以通过 Web 界面新增 RSS 源并手动触发抓取
- ✅ 重复抓取不会重复入库（后端去重逻辑验证）
- ✅ Web 界面显示时间线模式，按时间倒序展示条目
- ✅ 可以在 Web 界面标记已读、收藏和隐藏
- ✅ API 提供 OpenAPI 文档，可在 Swagger UI 中测试

风险控制：

- 抓取任务必须设置超时，避免阻塞请求或调度器。
- 外部源返回异常编码、异常 MIME、空 feed 或重复 GUID 时必须有可诊断错误。

### <a id="phase-3"></a>阶段三：源目录与导入

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

### <a id="phase-4"></a>阶段四：自动化、兴趣规则与推荐 Feed

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
8. 实现 `GET /api/feed/recommendations` 和 `POST /api/feed/recommendations/{id}/feedback`。
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

### <a id="phase-5"></a>阶段五：AI 摘要与通知

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

### <a id="phase-6"></a>阶段六：自然语言设置控制

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

### <a id="phase-7"></a>阶段七：金融市场监控与 AI 告警

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

### <a id="phase-8"></a>阶段八：工程化增强

实施范围：

- 增加 OpenAPI 契约、集成测试、关键指标、Dashboard 和更完整的部署配置。

实施步骤：

1. 将当前 API 固化为 OpenAPI 文档，并在 `make verify` 中增加契约检查。
2. 增加基于真实 PostgreSQL 的集成测试，优先覆盖源导入、抓取去重、摘要记录、通知记录、自然语言设置控制和金融告警。
3. 完善 `docker-compose`，纳入可选 ntfy、Prometheus、Grafana 和 Redis。Redis 只在缓存、队列、限流或任务锁实现已经接入时启用。
4. 增加核心指标：抓取次数、抓取失败、抓取耗时、摘要耗时、控制计划成功率、通知成功率、行情拉取成功率、告警触发次数。
5. 增加 Grafana Dashboard 草案，按采集、摘要、设置控制、通知、行情和告警分类展示。

验收标准：

- `make verify` 覆盖格式检查、单元测试、集成测试、构建和契约检查。
- 指标能展示抓取次数、抓取失败、摘要耗时、控制计划成功率、行情拉取成功率、告警触发次数和通知成功率。
- `docker-compose up` 后可访问服务、数据库和可选观测组件。

风险控制：

- 测试应复用正式迁移文件，不维护第二套测试 schema。
- Dashboard 不应依赖固定本机绝对路径。

### <a id="phase-9"></a>阶段九：来源扩展与分布式升级验证

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
docker-compose up
make verify
```

最终项目应在冷启动后通过 `/healthz` 与 `/readyz`，可以在 Tailscale 网络内访问，并能完成“新增订阅源 -> 抓取 -> Web 时间线浏览 -> 推荐 Feed 浏览 -> 生成摘要 -> 发送通知”、“自然语言指令 -> 变更计划 -> 用户确认 -> 设置调整 -> 审计记录”和“新增金融标的 -> 拉取行情 -> 规则命中 -> AI 解读 -> 微信通知”的闭环。
