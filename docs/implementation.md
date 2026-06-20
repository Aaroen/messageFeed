# messageFeed 实施文档

**最后更新**：2026-06-20

---

## 实施进度总览

| 阶段 | 名称 | 状态 | 完成度 | 开始日期 | 完成日期 |
|------|------|------|--------|----------|----------|
| 阶段一 | 基础设施搭建 | 已完成 | 100% | 2026-06-12 | 2026-06-13 |
| 阶段二 | 订阅源与 Feed 闭环 | 进行中 | 90% | 2026-06-13 | - |
| 阶段三 | 日志、错误追踪与链路观测 | 进行中 | 90% | 2026-06-17 | - |
| 阶段四 | 源目录与导入 | 进行中 | 55% | 2026-06-19 | - |
| 阶段五 | Agent 基础设施与 AI 源 | 原型进行中 | 15% | 2026-06-19 | - |
| 阶段六 | 主动采集与内容理解 Agent | 未开始 | 0% | - | - |
| 阶段七 | 推荐、摘要与通知 Agent | 未开始 | 0% | - | - |
| 阶段八 | 金融与跨领域分析 Agent | 未开始 | 0% | - | - |
| 阶段九 | 工程化增强 | 未开始 | 0% | - | - |
| 阶段十 | 来源扩展与分布式升级验证 | 未开始 | 0% | - | - |

当前状态基于 2026-06-20 代码审阅、本地部署与 Cloudflare Tunnel 验证：开发态通过 `messageFeed-make cloudflare` 启动，`messageFeed-status` 显示 `development + cloudflare`，`https://localhost:8443/healthz` 与 `https://aroen.eu.cc/healthz` 均返回成功。阶段四与原推荐 Feed 原型存在前置实现，但阶段五到阶段八已经重组为统一的 AI Agent 体系，详细方案见 `docs/agent-plan.md` 和 `docs/financial-agent-plan.md`。

---

## 1. 实施目标

以 `messageFeed` 作为 `Go_Pro` 首个完整项目，先完成本地单节点可运行、可部署、可观测、可验收的最小闭环，并通过 Cloudflare Tunnel 提供受控域名访问，再逐步扩展 `messageFeed AI Agent`、AI 内部源、主动网络采集、阅读行为画像、微信通知、金融市场监控和分布式部署能力。

当前第一部分交付本地单节点部署，默认 `DEPLOYMENT_MODE=single_node`。该配置只表示部署拓扑，不表示监听范围；当前外部访问只通过 Cloudflare Tunnel 域名 `https://aroen.eu.cc`，本机访问为 `https://localhost:8443`，局域网 IP 与 Tailscale IP 直连关闭。分布式部署仅保留接口与运行时边界，包括节点标识、部署模式配置、就绪检查、任务锁接口、通知幂等键和无状态 API 约束。

### 当前部署与开发调试基线

当前开发部署已经收敛为“单一 HTTPS 入口 + Docker 内部 API/Web 服务 + Cloudflare Tunnel 域名访问”的准部署结构。

| 项目 | 当前值 |
| --- | --- |
| 开发域名 | `https://aroen.eu.cc` |
| 本机入口 | `https://localhost:8443` |
| 宿主机网关监听 | `127.0.0.1:8443` |
| API 直连 | 宿主机仅 `127.0.0.1:60001`；开发态 `api-dev:60001` 仅 Docker 网络内访问 |
| Web dev server | `web-dev:5173` 仅 Docker 网络内访问 |
| 数据库 | `127.0.0.1:5432` |
| 观测组件 | Grafana、Prometheus、Loki、Tempo、OTel Collector 均绑定本机回环地址 |
| 局域网直连 | 关闭 |
| Tailscale 直连 | 关闭 |
| 外部入口 | Cloudflare Tunnel -> `https://gateway-dev:8443` |
| 开机自启 | `messagefeed-dev.service`，执行 `/usr/local/bin/messageFeed-make cloudflare` |

Cloudflare 到本地源站的 TLS 不依赖 Cloudflare 控制台中的 `No TLS Verify`。本地部署生成 `gateway-dev` 专用证书和本地 CA，Caddy 使用该证书服务 `https://gateway-dev:8443`，`cloudflared` 通过挂载的 CA bundle 验证源站证书。证书目录 `deploy/caddy/certs/` 和 token 文件 `key` 均不得进入版本控制。

当前命令约定：

```text
messageFeed-make cloudflare      启动开发态和 Cloudflare Tunnel
messageFeed-make reload-api      重启开发态 Go API
messageFeed-make reload-web      重启 Vite 开发服务
messageFeed-make reload-gateway  重启 Caddy 网关
messageFeed-make logs            查看开发态日志
messageFeed-make stop            停止开发态入口和 Tunnel
messageFeed-start                启动非开发模式
messageFeed-status               查询部署模式、入口地址、健康检查、容器和监听端口
```

开发测试流程：

1. 修改前端代码后，通过 `https://localhost:8443` 或 `https://aroen.eu.cc` 调试，Vite HMR 负责热更新。
2. 修改后端 Go 代码后，执行 `messageFeed-make reload-api`。
3. 修改网关配置后，执行 `messageFeed-make reload-gateway`。
4. 状态核查使用 `messageFeed-status`。
5. 基础健康检查使用 `curl -sk https://localhost:8443/healthz` 和 `curl -sk https://aroen.eu.cc/healthz`。
6. 完整验证继续使用 `go test ./...` 与 `cd web && npm run build`。

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
│   ├── agent/
│   ├── acquisition/
│   ├── profile/
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
| `agent` | 项目级 AI Agent 的能力注册、意图解析、计划生成、风险校验、执行编排和审计 | 直接写数据库、绕过 service 执行变更、通用无限工具执行 |
| `acquisition` | 主动网络采集、静态网页抽取、网页变化监控、搜索型采集、快照和来源评估 | 登录态采集、绕过访问限制、规避反爬、无限制浏览器自动化 |
| `profile` | 阅读行为事件、用户兴趣标签、短期兴趣、长期偏好、负反馈和画像解释 | 无明确用途的高频行为采集、不可解释黑箱画像 |
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
service -> agent/acquisition/profile
recommender -> repository/domain
agent -> service/llm/runtime
acquisition -> repository/domain
profile -> repository/domain
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
- `agent` 不直接调用 `repository`，所有执行动作必须通过已注册能力和既有 service 接口完成。
- `acquisition` 可以保存采集快照，但搜索结果必须经抓取、去重和来源评估后才能进入摘要、推荐或通知。
- `profile` 不采集鼠标轨迹、键盘轨迹、剪贴板和浏览器外部行为，长期偏好必须具备多次证据或用户确认。
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
| 阶段五 | Agent 基础设施与 AI 源 | [查看细节](#phase-5) |
| 阶段六 | 主动采集与内容理解 Agent | [查看细节](#phase-6) |
| 阶段七 | 推荐、摘要与通知 Agent | [查看细节](#phase-7) |
| 阶段八 | 金融与跨领域分析 Agent | [查看细节](#phase-8) |
| 阶段九 | 工程化增强 | [查看细节](#phase-9) |
| 阶段十 | 来源扩展与分布式升级验证 | [查看细节](#phase-10) |

## 4. 详细实施过程

### <a id="phase-1"></a>阶段一：基础设施搭建

**状态**：已完成 | **完成时间**：2026-06-13 | **完成度**：100%

#### 实施进度清单

**项目骨架（已完成）**
- [x] 初始化 Go 模块
- [x] 创建目录结构
- [x] 配置 .gitignore

**配置系统（已完成）**
- [x] 实现 internal/config 模块
- [x] 环境变量解析
- [x] 配置校验和默认值
- [x] 单元测试

**HTTP 服务（已完成）**
- [x] 基础 HTTP 服务器
- [x] GET / (服务信息)
- [x] GET /healthz (存活检查)
- [x] GET /readyz (就绪检查，含数据库)
- [x] GET /metrics (Prometheus 指标)
- [x] GET /api/runtime/node (节点信息)
- [x] 请求日志中间件

**数据库集成（已完成）**
- [x] internal/db 模块
- [x] 连接池配置
- [x] 健康检查
- [x] migrations/000001_init_schema.up.sql
- [x] migrations/000001_init_schema.down.sql
- [x] `golang-migrate/migrate` 版本化迁移
- [x] Docker Compose 独立迁移服务

**可观测性（已完成）**
- [x] log/slog 结构化日志
- [x] internal/metrics 模块
- [x] HTTP 请求指标
- [x] 数据库连接池指标

**构建与部署（已完成）**
- [x] Makefile (fmt, vet, test, build, verify)
- [x] Dockerfile (多阶段构建)
- [x] docker-compose.yml
- [x] .env.example
- [x] Compose `dev` profile：`api-dev`、`web-dev`、`gateway-dev`
- [x] Cloudflare Tunnel profile：`cloudflared`
- [x] Caddy 统一入口：本机 `127.0.0.1:8443`，域名经 Tunnel 访问
- [x] systemd 开机自启：`messagefeed-dev.service`
- [x] 系统命令：`messageFeed-make`、`messageFeed-start`、`messageFeed-status`

**文档（已完成）**
- [x] docs/requirements.md
- [x] docs/architecture.md
- [x] docs/implementation.md
- [x] 前后端架构章节

#### 验收结果
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
- 本地默认使用 `DEPLOYMENT_MODE=single_node`。当前远程访问通过 Cloudflare Tunnel 域名完成，局域网 IP 与 Tailscale IP 直连入口关闭；宿主机统一入口只绑定 `127.0.0.1:8443`。
- PostgreSQL 作为第一阶段唯一主存储；Redis 不进入第一阶段 Docker Compose 必需组件，但预留后续缓存、队列、限流和任务锁接口。

实施步骤：

1. 创建 `cmd/api`、`internal/config`、`internal/handler`、`internal/runtime`、`internal/repository`、`deploy`、`api` 等基础目录。
2. 在配置层定义数据库、HTTP、运行时、日志和后续外部服务的配置结构；密钥只从环境变量或外部配置注入。
3. 建立基础 HTTP 路由，统一错误响应结构和 request id 日志字段；Gin 路由迁移进入第二阶段执行。
4. 建立数据库连接、版本化迁移执行入口和 `/readyz` 依赖检查；迁移由 Compose/Makefile 显式执行，API 启动时只检查迁移状态，不自行修改 schema。
5. 增加 Prometheus 指标注册，至少覆盖请求量、请求耗时、健康状态和数据库连接状态。
6. 在 Docker Compose 中纳入服务本体、PostgreSQL 和一次性迁移服务，不在第一阶段引入 Redis；与缓存、队列、限流和任务锁相关的能力通过接口预留。
7. 开发态通过 Caddy 统一入口访问页面和 API；Cloudflare Tunnel 将 `aroen.eu.cc` 转发到 Docker 内部 `gateway-dev:8443`，`PUBLIC_BASE_URL` 配置为 `https://aroen.eu.cc`。

验收标准：

- `make compose-up` 可以启动 PostgreSQL、完成 pending `up` 迁移并启动服务；空卷首次初始化时直接 `docker compose up -d --build` 也应成立。
- `/healthz` 返回成功。
- `/readyz` 能检查数据库连接和 `schema_migrations` 迁移状态。
- `/metrics` 可以被 Prometheus 格式读取。
- Cloudflare 域名 `https://aroen.eu.cc/healthz` 可以访问 API 健康检查；局域网 IP 和 Tailscale IP 直连不可访问统一入口。
- `make verify` 可以执行格式检查、构建和基础测试。
- `/api/runtime/node` 能返回 `deployment_mode=single_node`、节点标识、监听配置和公开访问基址。

风险控制：

- API 层不得依赖本机内存保存业务状态。
- 第一阶段任务锁可以是单节点或 PostgreSQL 实现，但接口必须保留，后续允许替换为 PostgreSQL advisory lock、任务表锁或 Redis 锁。
- `/readyz` 不检查非关键外部服务，避免微信、AI 或行情源短暂不可用导致服务被错误摘除。
- Redis 不作为主存储或审计存储；业务幂等、持久状态和可追溯记录必须落入 PostgreSQL。
- PostgreSQL 官方镜像的 `/docker-entrypoint-initdb.d` 只用于空库初始化，不承载项目迁移；不得将包含 `.down.sql` 的完整 `migrations` 目录挂载到该路径。
- 共享环境中已经执行过的迁移文件不得修改，只能追加更高版本迁移；生产回滚优先采用备份恢复或前向修复，`down` 仅用于明确授权的受控回滚。

### <a id="phase-2"></a>阶段二：订阅源与 Feed 闭环

**状态**：进行中 | **开始时间**：2026-06-13 | **完成度**：90%

#### 实施进度清单

**路由层迁移（已完成）**
- [x] 引入 Gin 依赖
- [x] 将现有 `net/http` 路由迁移到 Gin
- [x] 保留 `/`、`/healthz`、`/readyz`、`/metrics` 和 `/api/runtime/node` 的既有行为
- [x] 建立 `/api/v1` 业务路由组
- [x] 建立 request id、访问日志、统一响应和错误映射中间件
- [x] 迁移现有 handler 测试

**数据库设计（已完成）**
- [x] 创建 `migrations/000002_add_sources_items.up.sql`
- [x] 创建 `migrations/000002_add_sources_items.down.sql`
- [x] 定义 `sources` 表
- [x] 定义 `items` 表
- [x] 定义 `user_item_states` 表
- [x] 定义 `feed_view_preferences` 表
- [x] 添加索引、唯一约束、检查约束和更新时间触发器
- [x] 通过 Docker Compose 在空数据库上执行 `000001 -> 000002` 迁移验证

**领域模型（已完成）**
- [x] `internal/domain/source.go`
- [x] `internal/domain/item.go`
- [x] `internal/domain/user_item_state.go`
- [x] `internal/domain/feed_view_preference.go`
- [x] 枚举和领域错误定义

**Repository 层（已完成）**
- [x] `internal/repository/source_repository.go` (Create, Get, List, Update, UpdateFetchResult)
- [x] `internal/repository/source_repository_test.go`
- [x] `internal/repository/item_repository.go` (UpsertMany, ListByUser, GetByIDForUser；列表和详情联表返回来源名称与阅读状态)
- [x] `internal/repository/item_repository_test.go`
- [x] `internal/repository/user_item_state_repository.go` (MarkRead, Favorite, Hide；使用 `ON CONFLICT(user_id,item_id)` 原子 upsert，避免并发首次写入触发唯一约束冲突)
- [x] `internal/repository/feed_view_preference_repository.go` (Get, Upsert)

**Fetcher 模块（已完成）**
- [x] `internal/fetcher/feed_fetcher.go`
- [x] 集成 `gofeed`
- [x] HTTP 抓取（超时、重定向、大小限制）
- [x] URL 规范化
- [x] 条目字段映射和内容 hash
- [x] 错误处理
- [x] `internal/fetcher/feed_fetcher_test.go`

**Service 层（已完成）**
- [x] `internal/service/source_service.go` (Create, List, Update, TriggerFetch)
- [x] `internal/service/source_service_test.go`
- [x] `internal/service/timeline_service.go` (ListItems, GetItem；支持已读、收藏、隐藏和来源过滤参数)
- [x] `internal/service/timeline_service_test.go`
- [x] `internal/service/item_service.go` (MarkRead, Favorite, Hide)
- [x] `internal/service/feed_view_service.go` (GetMode, SaveMode；不存在用户偏好时返回默认 `timeline`)

**Handler 层（已完成）**
- [x] `internal/handler/router.go` (Gin 路由注册和中间件装配)
- [x] `internal/handler/response.go` (统一响应格式和错误映射)
- [x] `internal/handler/source_handler.go` (POST, GET, PATCH /api/v1/sources, POST /api/v1/sources/:id/fetch)
- [x] `internal/handler/item_handler.go` (GET /api/v1/items, GET /api/v1/items/:id, GET /api/v1/feed/timeline)
- [x] `internal/handler/item_handler.go` 标记操作 (POST /api/v1/items/:id/read, /favorite, /hide；空请求体默认置为 true，请求体可显式传 false 取消状态)
- [x] `internal/handler/feed_view_handler.go` (GET, PUT /api/v1/feed/view-mode)
- [x] `internal/handler/middleware.go` CORS 中间件，允许本地 Vite 开发源跨域调用 API
- [x] 统一响应格式

**OpenAPI 文档（进行中）**
- [x] `api/openapi.yaml` 最小前端契约：条目列表、条目详情、订阅源列表、阅读模式偏好
- [ ] 后端接口稳定后安装 swaggo/swag 或接入契约生成/校验流程
- [ ] 添加完整 OpenAPI 注解或补齐完整手写契约
- [ ] 配置 Swagger UI

**前端初始化（已完成）**
- [x] 初始化 Vue 3 + Vite 项目 (web/)
- [x] 安装 Arco Design Vue
- [x] 配置 Vue Router, Pinia, Axios
- [x] 配置 TypeScript
- [x] 配置 Vite 监听容器内部 `0.0.0.0:5173`，由 Caddy 统一入口转发访问；宿主机不直接暴露 `5173`
- [x] 建立无开发信息的 AppShell、覆盖式导航和基础页面入口

**前端页面（进行中）**
- [x] 订阅源管理页基础版 (`/sources`)：支持源目录搜索、目录启停、URL 批量导入、OPML 导入和启用后手动抓取
- [x] 时间线页面基础版 (`/subscriptions`，`/timeline` 当前重定向)：支持订阅条目列表、下拉刷新和来源阅读器入口
- [x] 推荐页面原型 (`/recommendations`)：接入当前推荐 Feed 原型接口
- [x] 覆盖式条目详情阅读器：支持条目详情加载、正文 `srcdoc` 展示、阅读原文和阅读进度
- [ ] 独立 `/items/:id` 详情路由
- [ ] 时间线筛选、分页加载和阅读模式偏好与前端 UI 的完整绑定

**前端组件（进行中）**
- [x] `SubscriptionSourcesView`：承载源目录列表、订阅启停、导入入口和抓取动作
- [x] `SubscriptionFeedView`：承载订阅/推荐/来源三类条目列表和下拉刷新
- [x] `AppShell` 覆盖式导航、来源阅读器和条目详情阅读器
- [ ] `ActionBar`：已读、收藏、隐藏等条目状态操作尚未在 Web 界面打通
- [ ] `SourceForm`：新增/编辑抽屉表单尚未形成完整交互；当前通过目录、URL 和 OPML 导入创建来源

**前端验证与集成测试（进行中）**
- [x] `web` 目录下 `vue-tsc --noEmit` 通过
- [ ] 前后端联调验收
- [ ] 端到端测试

**后端验证（已完成）**
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
- [x] 可以通过 Web 界面完成订阅源基础管理，包括目录启停、URL 导入、OPML 导入和手动抓取
- [x] 可以通过 API 手动触发抓取
- [x] 重复抓取不会重复入库
- [x] API 可以返回时间线模式条目，并在列表和详情中包含来源名称与阅读状态
- [x] Web 界面显示时间线模式
- [x] API 可以标记已读、收藏、隐藏，并支持取消状态
- [x] API 可以保存和读取 Web 阅读模式偏好
- [x] API 已提供阶段二前端最小 OpenAPI 契约
- [ ] Web 界面可以标记已读、收藏、隐藏
- [ ] Web 界面支持时间线筛选、分页加载和阅读模式偏好持久化
- [ ] `api/openapi.yaml` 补齐创建、更新、抓取、导入、推荐和状态操作等已实现接口
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

开发态准部署入口：

- 当前已完成开发态准部署入口，不再依赖手动分别启动 `docker compose`、后端 API、`vite preview` 和临时 HTTPS 证书。
- 当前开发拓扑为：浏览器访问 `https://localhost:8443` 或 `https://aroen.eu.cc`；Caddy 作为统一入口；`/api`、`/healthz`、`/readyz`、`/metrics` 转发到 `api-dev:60001`；其余页面转发到 `web-dev:5173`，保留 Vite HMR。
- Compose `dev` profile 已纳入 PostgreSQL、迁移、`api-dev`、`web-dev` 和 `gateway-dev`；Cloudflare 访问通过 `cloudflare` profile 中的 `cloudflared` 提供。
- 宿主机只暴露本机回环入口 `127.0.0.1:8443`，不直接暴露 `5173`、开发态 `60001`、局域网 IP 或 Tailscale IP。
- Cloudflare Tunnel 远程路由为 `aroen.eu.cc -> https://gateway-dev:8443`。本地通过自签 CA 和专用 `gateway-dev` 证书完成源站 TLS 校验，不依赖 Cloudflare 控制台的 `No TLS Verify`。
- 本阶段不要求每次前端改动都执行生产构建；开发态使用 Vite dev server，最终生产态再切换为静态 `web/dist`。

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

- [x] 可以通过 API 新增 RSS 源并手动触发抓取。
- [x] 重复抓取不会重复入库。
- [x] API 可以返回时间线模式条目，按时间倒序展示。
- [x] API 条目列表和详情已包含来源名称与用户阅读状态。
- [x] API 可以标记已读、收藏和隐藏，并支持取消状态。
- [x] API 可以读取和保存阅读模式偏好。
- [x] Web 界面已支持订阅源基础管理、手动抓取、时间线展示和条目详情阅读。
- [ ] Web 界面尚未支持已读、收藏和隐藏等条目状态操作。
- [ ] Web 界面尚未完整支持筛选、分页加载和阅读模式偏好持久化。
- [x] API 已提供阶段二前端最小 OpenAPI 契约；完整契约、契约校验和 Swagger UI 后置补充。
- [x] 阶段一已有健康检查、就绪检查、指标和运行时节点端点在迁移到 Gin 后保持兼容。

风险控制：

- Gin 迁移不得改变已有基础端点的响应语义，避免破坏 Docker healthcheck、Prometheus 抓取和 Tailscale 访问验证。
- Gin 中间件只承载横切关注点，不在中间件中写订阅、抓取和阅读状态业务规则。
- 抓取任务必须设置超时，避免阻塞请求或调度器。
- 外部源返回异常编码、异常 MIME、空 feed 或重复 GUID 时必须有可诊断错误。
- 抓取器只允许 `http` 和 `https` URL，并限制重定向次数和响应体大小，避免外部输入长期占用资源。

### <a id="phase-3"></a>阶段三：日志、错误追踪与链路观测

**状态**：进行中 | **完成度**：90%

**触发条件**：阶段二完成“订阅源 -> 手动抓取 -> 去重入库 -> 时间线展示 -> 阅读状态”最小业务闭环后立即启动。该阶段完成前，不进入源目录、推荐 Feed、AI 摘要、通知或金融监控的主体开发。

当前代码已经前置实现阶段三主体能力；后续仍需完成完整 Compose 观测环境的端到端验收。

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

当前实现状态：

- [x] `internal/observability` 已提供结构化 JSON logger、OpenTelemetry tracer provider、span 辅助函数和 trace/span id 读取能力。
- [x] Gin 路由已接入 request id、CORS、Recovery、访问日志和 OpenTelemetry middleware。
- [x] repository、service、fetcher 等已实现关键 span 与基础指标。
- [x] Docker Compose 已纳入 Prometheus、Grafana、Loki、Promtail、Tempo 和 OpenTelemetry Collector。
- [ ] 仍需用完整 Compose 环境做一次端到端验收，确认同一请求可通过 `request_id` 和 `trace_id` 串联日志、指标和 trace。
- [ ] 统一错误类型仍可继续抽象，以便后续 AI、通知、金融和自然语言控制模块复用相同错误模型。

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

**状态**：进行中 | **完成度**：55%

实施范围：

- 建立推荐源目录、OPML 导入、URL 批量导入和从目录批量订阅。
- 源目录借鉴 Folo 的 discover sources 与 onboarding feed，但必须记录来源出处和许可状态。

当前实现状态：

- [x] `migrations/000003_add_source_catalog_imports.*.sql` 已建立 `source_catalog_entries` 和 `source_import_jobs` 表。
- [x] `migrations/000003` 至 `000006` 已写入并持续扩充官方源目录，覆盖 AI、开发、云、数据库、财经、新闻和中文来源等分类。
- [x] `internal/domain/source_catalog.go`、`internal/repository/source_catalog_repository.go` 已实现源目录领域对象、列表查询和按 ID 查询。
- [x] `internal/service/source_service.go` 已实现目录导入、URL 批量导入、重复来源复用和停用来源恢复。
- [x] `internal/handler/source_handler.go` 已注册 `/api/v1/source-catalogs`、`/api/v1/sources/import/catalog`、`/api/v1/sources/import/urls` 和 `/api/v1/sources/import/opml`。
- [x] OPML 导入已支持 2 MiB 文件大小限制、递归读取 outline 中的 `xmlUrl`，并复用 URL 批量导入流程。
- [x] `web/src/views/SubscriptionSourcesView.vue` 已接入源目录搜索、目录启停、URL 导入、OPML 导入和启用后抓取。
- [ ] `source_import_jobs` 目前主要是 schema 预留，尚未由 service/repository 持久化每次导入任务及错误明细。
- [ ] 源目录仍缺少自动健康检查、许可状态治理、热度字段和最近校验时间更新流程。
- [ ] 源目录 API 目前以关键词和分类过滤为主，语言、国家、健康状态等过滤维度仍需补齐。
- [ ] `api/openapi.yaml` 尚未覆盖源目录、目录导入、URL 导入和 OPML 导入接口。

实施步骤：

1. [x] 建立 `source_catalog_entries` 和 `source_import_jobs`。
2. [x] 为候选源记录名称、URL、站点、分类、语言、来源出处、健康状态和最近校验时间等基础字段。
3. [x] 实现目录查询、关键词搜索、分类过滤和从目录批量订阅。
4. [x] 实现 OPML 解析，输出成功和失败明细。
5. [x] 实现 URL 批量导入，每个 URL 独立校验，单个失败不影响整体任务。
6. [ ] 建立导入任务持久化流程，保留原始错误摘要，便于用户修正源地址。
7. [ ] 补齐许可状态、热度、语言过滤、健康状态过滤和源健康检查任务。

验收标准：

- [x] 可以通过 API 搜索推荐源。
- [x] 可以导入 OPML，并返回成功与失败明细。
- [x] 可以从推荐源目录批量创建订阅。
- [x] 失败源不会阻断其他源导入。
- [ ] 可以查询导入任务状态和历史错误明细。
- [ ] 推荐源具备可追踪的许可状态、健康状态和最近校验记录。

风险控制：

- 不直接复制第三方源数据为正式内置数据，先作为候选来源并核查许可。
- 目录源需要定期健康检查，避免大量不可用源影响用户体验。

### <a id="phase-5"></a>阶段五：Agent 基础设施与 AI 源

**状态**：原型进行中 | **完成度**：15%

阶段五将原“自动化、推荐、摘要、自然语言控制”的基础能力统一为项目级 AI Agent 底座。详细设计见 `docs/agent-plan.md`。

当前实现状态：

- [x] `GET /api/v1/feed/recommendations` 推荐 Feed 原型已存在，可作为后续推荐 Agent 的候选能力输入。
- [x] Web 已具备 `/recommendations` 推荐入口和推荐来源订阅启停基础交互。
- [ ] 尚未建立 `agent` 模块、能力注册、结构化计划、风险校验、确认策略、执行器和审计表。
- [ ] 尚未建立 `messageFeed AI` 内部源和 AI 生成条目元数据。
- [ ] 尚未将自然语言设置控制统一纳入 Agent 能力注册框架。
- [ ] 尚未建立 Agent 上下文管理、冻结记忆快照、语义分块归档和按需回忆机制。

实施步骤：

1. 建立 `agent_commands`、`agent_capabilities`、`agent_plans`、`agent_plan_steps` 和 `agent_audit_logs`。
2. 定义 `AgentCapabilityRegistry`、`AgentInterpreter`、`AgentPlanner`、`AgentExecutor` 和 `AgentAuditLogger`。
3. 定义能力风险等级和确认策略：低风险可建议或自动执行，中高风险必须确认。
4. 建立 `AgentContextManager`、`MemoryProvider`、`ContextBuilder` 和冻结 `MemorySnapshot`。
5. 建立上下文压力评估、不可压缩保护区、语义分块、归档摘要和按需回忆基础能力。
6. 创建每个用户默认的 `messageFeed AI` 内部源，来源类型为 `ai_agent`。
7. 将 Agent 生成的日报、周报、热点分析、主动网络研究报告、金融分析和执行结果写入 AI 源。
8. Web 支持展示 AI 源，与普通来源共用列表、详情、已读、收藏和隐藏能力。
9. Agent 执行过程接入 request id、trace id、结构化日志、指标、上下文压缩、记忆召回和审计记录。

验收标准：

- 用户可以提交自然语言命令并得到结构化 Agent 计划。
- Agent 能力必须经过注册才能执行。
- 中高风险计划必须等待用户确认。
- Agent 可以写入一条 `messageFeed AI` 源内容。
- Agent 执行结果具备可查询审计记录。
- Agent 可以生成一次冻结的用户画像记忆快照。
- 当上下文达到压缩阈值时，可以按完整语义块归档历史并保留摘要索引。
- Agent 可以通过回忆工具取回历史归档、AI 源报告或画像证据。
- 模型不能直接访问数据库写接口。

风险控制：

- 模型只生成意图、计划、说明文本和工具参数摘要。
- 实际执行必须由 `AgentExecutor` 调用既有 service 接口完成。
- 删除类自然语言默认解释为停用或归档；永久删除必须二次确认。
- 密钥、token、Webhook URL 和数据库 DSN 不进入模型上下文。
- 召回内容必须标注来源、时间和可信等级，且不得覆盖系统规则、权限策略和能力边界。

### <a id="phase-6"></a>阶段六：主动采集与内容理解 Agent

阶段六用于补足无 RSS、Atom、JSON Feed 或稳定 API 的信息源。

实施范围：

- 静态网页抓取和正文抽取。
- 网页变化监控和快照 hash。
- 搜索型采集和候选 URL 抓取。
- 来源可信度、稳定性和失败原因记录。
- 将重要变化写入普通条目或 `messageFeed AI` 源报告。

实施步骤：

1. 建立 `web_acquisition_tasks` 和 `web_snapshots`。
2. 定义 `WebAcquisitionProvider`、`SearchProvider`、`PageExtractor` 和 `SnapshotStore`。
3. 实现静态网页抓取，抽取标题、正文、发布时间和链接。
4. 实现网页变化监控，记录正文 hash、结构 hash 和抓取状态。
5. 实现搜索型采集，搜索结果必须先抓取、去重和来源评估。
6. 将主动采集结果与现有 `items`、AI 源和推荐候选池打通。
7. 为主动采集任务建立调度、失败重试和限流策略。

验收标准：

- 可以创建一个网页监控任务。
- 可以抓取一个无 RSS 页面并抽取正文。
- 页面变化后可以生成普通条目或 AI 源报告。
- 可以按关键词执行一次主动网络研究并生成 AI 源报告。
- 所有主动采集结果保留 URL、抓取时间、hash、抽取方法和失败原因。

风险控制：

- 搜索结果不能直接视为事实。
- 登录态采集、绕过访问限制和规避反爬不进入早期实现。
- 主动采集不得无限并发，必须具备超时、大小限制和重试边界。

### <a id="phase-7"></a>阶段七：推荐、摘要与通知 Agent

阶段七将推荐、摘要、通知和用户偏好学习合并为“内容理解 -> AI 源沉淀 -> 主动提醒”的闭环。

实施范围：

- 持久化推荐候选池、推荐原因和推荐反馈。
- 阅读行为事件与用户兴趣画像。
- 用户画像作为 Agent 底层长期记忆的冻结快照。
- 日报、周报、专题摘要和热点事件分析。
- 企业微信、`ntfy` 和后续微信通道推送。
- 通知冷却、免打扰、幂等键、失败重试和通知历史。

实施步骤：

1. 建立 `user_item_interaction_events`，扩展 `user_item_states` 的打开次数、阅读进度和主动停留时间。
2. 建立 `user_interest_profiles`、`user_interest_tags` 和 `user_interest_evidence`。
3. 建立 `feed_recommendations` 和 `recommendation_feedback` 的完整持久化闭环。
4. 记录推荐原因，区分已订阅来源和未订阅候选来源。
5. 基于阅读行为、来源权重、标签、语言、收藏、隐藏和停留时间形成初步评分。
6. 将用户画像、近期兴趣、负反馈和通知偏好作为推荐、摘要和通知 Agent 的长期记忆输入。
7. 生成日报、周报、专题摘要和热点事件分析，并写入 `messageFeed AI` 源。
8. 在 `notifier` 中抽象企业微信机器人、企业微信自建应用、`ntfy` 和后续微信通道。
9. 通知记录必须保存通道、接收目标、触发原因、状态、失败原因、模型、token、耗时和 `dedupe_key`。

验收标准：

- 推荐 Feed 可以稳定混合已订阅和未订阅内容。
- 推荐条目具有推荐原因。
- 用户可以反馈“不感兴趣”和“减少类似推荐”。
- 系统可以记录阅读行为事件并更新可解释兴趣标签。
- 系统可以生成日报并写入 AI 源。
- 系统可以通过企业微信或 `ntfy` 发送摘要提醒。
- 通知具备幂等键、冷却时间和失败记录。
- 用户画像可以解释关键推荐、摘要选择和通知触发依据。

风险控制：

- 停留时间只能统计页面可见且窗口聚焦的主动停留时间。
- 用户画像必须可解释、可编辑、可回滚。
- 原始阅读事件不直接进入模型上下文，应先聚合为画像、近期兴趣或推荐证据。
- 个人微信桥接仅作实验，不进入第一版验收。
- 摘要任务必须记录 token、耗时和错误，便于成本分析。

### <a id="phase-8"></a>阶段八：金融与跨领域分析 Agent

金融分析使用独立专项计划维护，详见 `docs/financial-agent-plan.md`。本阶段在实施文档中只保留集成目标和最小闭环。

实施范围：

- 建立金融标的、行情源、行情快照、关注列表、规则和告警事件。
- 规则判断保持确定性，AI 不参与基础阈值判断。
- 规则命中后，Agent 汇总行情、近期财经资讯、主动网络采集结果和用户关注主题。
- AI 生成市场波动解释，写入 `messageFeed AI` 源。
- 高优先级事件通过企业微信或 `ntfy` 推送。

实施步骤：

1. 建立 `market_instruments`、`market_data_providers`、`market_quotes`、`market_watchlists`、`market_alert_rules` 和 `market_alert_events`。
2. 定义 `MarketDataProvider` 和 `MarketAlertEngine`。
3. 第一版选择一个低成本 provider 完成行情链路验证。
4. 告警命中后生成 `market_alert_events`，通过 `dedupe_key` 和冷却时间避免重复发送。
5. 金融分析写入 AI 源，并记录行情快照、触发规则、相关资讯、模型、提示词版本和 token。
6. 金融通知复用 `notifier`，发送内容包含标的、当前价、涨跌幅、触发阈值、行情时间、数据源、AI 简述和“不构成投资建议”提示。

验收标准：

- 可以新增一个指数或 ETF 关注标的。
- 可以配置当日涨跌幅阈值规则。
- 可以拉取行情快照并触发规则。
- 规则命中后生成 AI 源分析条目。
- 可以通过企业微信或 `ntfy` 发送金融告警。
- 同一规则在冷却时间内不会重复发送。

风险控制：

- 金融分析必须标注“不构成投资建议”。
- AI 不输出确定性买入、卖出、加仓、减仓建议。
- AI 不参与基础告警触发判断。
- 不接入自动交易、券商账户和下单能力。

### <a id="phase-9"></a>阶段九：工程化增强

实施范围：

- 在阶段三完整观测系统的基础上，增加 OpenAPI 契约、集成测试、业务指标扩展、Dashboard 迭代和更完整的部署配置。
- 阶段二的开发态准部署入口已收敛为可长期运行的部署结构：Compose `dev` profile、Caddy 统一入口、Cloudflare Tunnel 域名访问和 systemd 开机自启已经落地。阶段九继续补齐契约检查、集成测试、生产静态前端入口和多节点验证。

实施步骤：

1. 将当前 API 固化为 OpenAPI 文档，并在 `make verify` 中增加契约检查。
2. 增加基于真实 PostgreSQL 的集成测试，优先覆盖源导入、抓取去重、摘要记录、通知记录、自然语言设置控制和金融告警。
3. 完善 Docker Compose，纳入可选 ntfy 和 Redis；Prometheus、Grafana、Loki、Tempo、OpenTelemetry Collector 沿用阶段三观测 profile 并按业务需要扩展。
4. 扩展核心业务指标：摘要耗时、控制计划成功率、通知成功率、行情拉取成功率、告警触发次数。
5. 迭代 Grafana Dashboard，按采集、摘要、设置控制、通知、行情和告警分类展示。
6. 已增加 Caddy 统一入口服务：开发态将页面请求转发到 `web-dev:5173`，将 `/api`、`/healthz`、`/readyz`、`/metrics` 转发到内部 `api-dev:60001`；生产态后续切换为直接服务 `web/dist`。
7. 已拆分 Compose profile：`dev` 包含 Web 热更新服务和开发网关，`cloudflare` 包含 Tunnel，观测组件保持独立服务并仅绑定本机回环地址；后续继续整理 `core` 与 `observability` 的长期边界。
8. 已增加本机自恢复方案：`messagefeed-dev.service` 开机自启并执行 `/usr/local/bin/messageFeed-make cloudflare`，确保机器重启后开发态与 Tunnel 自动恢复。
9. 已将外部访问收敛到单一 HTTPS 域名 `https://aroen.eu.cc`，并关闭局域网 IP 与 Tailscale IP 直连入口；不再依赖 `https://100.x.x.x:5173` 或 `https://192.168.x.x:8443`。
10. API、Web 开发服务、数据库和观测组件均不直接对外暴露；浏览器只通过 Caddy 统一入口和 Cloudflare Tunnel 访问页面与 API。

验收标准：

- `make verify` 覆盖格式检查、单元测试、集成测试、构建和契约检查。
- 指标能在阶段三基础上继续展示摘要耗时、控制计划成功率、行情拉取成功率、告警触发次数和通知成功率。
- `make compose-up` 后可访问服务、数据库和可选观测组件。
- 开发态可以通过 `https://localhost:8443` 和 `https://aroen.eu.cc` 访问前端和 API，前端改动可以热更新，不需要手动复制构建产物。
- 生产态可以通过同一统一入口服务访问静态前端和 API，外部只暴露 HTTPS 入口，不直接暴露 `5173` 或 `60001`。

风险控制：

- 测试应复用正式迁移文件，不维护第二套测试 schema。
- Dashboard 不应依赖固定本机绝对路径。
- PostgreSQL 数据卷不得因重建入口服务或前端开发服务而丢失；迁移前应保留备份或可恢复快照。
- 统一入口与 HTTPS 证书配置不得将本地私钥提交到仓库；本地证书目录应保持在忽略列表中。

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
8. 通过 `notifications.dedupe_key`、`agent_plans.dedupe_key`、`control_change_plans.dedupe_key` 和 `market_alert_events.dedupe_key` 验证日报、Agent 计划、设置控制和金融告警不会重复执行。

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

当前近期推进顺序（2026-06-20）：

1. 收尾阶段二 Web 闭环：补齐已读、收藏、隐藏操作，完善筛选、分页加载和阅读模式偏好持久化。
2. 补齐 `api/openapi.yaml`：覆盖创建、更新、抓取、导入、推荐和条目状态操作等已实现接口。
3. 完成阶段三观测验收：通过 Compose 验证 `request_id`、`trace_id`、日志、指标和 trace 的查询链路。
4. 推进阶段四验收缺口：持久化导入任务，补齐源健康检查、许可状态、语言/健康过滤和错误明细查询。
5. 进入阶段五 Agent 基础设施：建立能力注册、结构化计划、风险校验、确认策略、执行器、审计日志、上下文管理和冻结记忆快照。
6. 建立 Agent 上下文归档与回忆基础能力：补齐语义分块、压缩阈值、摘要索引、归档引用、回忆工具和记忆提升确认。
7. 建立 `messageFeed AI` 内部源：将日报、周报、热点分析、主动网络研究报告、金融分析和 Agent 操作报告统一写入 AI 源。
8. 推进阶段六主动采集：先实现静态网页抽取、网页变化监控和搜索结果抓取评估，再接入 AI 源报告。
9. 推进阶段七推荐、摘要与通知：补齐阅读行为事件、用户画像、推荐原因、反馈闭环、摘要生成和企业微信或 `ntfy` 推送。
10. 推进阶段八金融专项：按 `docs/financial-agent-plan.md` 完成关注标的、行情快照、确定性规则、AI 解读和通知闭环。

必须优先完成：

- 工程基线。
- Tailscale 简单远程访问。
- RSS 手动抓取。
- 去重入库。
- Feed 查询。
- Web 时间线模式。
- 日志、错误追踪和链路观测系统。
- Agent 基础设施与审计。
- Agent 上下文管理、冻结记忆快照、语义归档和回忆工具。
- `messageFeed AI` 内部源。
- 主动网络采集最小闭环。
- 阅读行为事件与基础用户画像。
- 推荐 Feed 模式。
- OPML 导入。
- 日报摘要。
- 微信单向通知。
- 自然语言设置控制向 Agent 能力注册框架收敛。
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
