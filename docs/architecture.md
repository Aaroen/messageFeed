# messageFeed 技术架构文档

## 1. 架构原则

项目采用单体模块化架构，不在初期拆分微服务。核心原因是当前运行环境为 WSL 本机，主要目标是完成业务闭环、数据建模、异步任务、AI 调用和通知链路。

部署策略采用“当前本地单节点，接口预留分布式升级”的方式。当前开发部署使用 Docker Compose、Caddy 统一入口、Vite dev server、Go API dev 容器和 Cloudflare Tunnel；外部只通过 `https://aroen.eu.cc` 进入，宿主机仅保留 `127.0.0.1:8443` 本机入口，局域网与 Tailscale 直连关闭。代码结构、配置和任务执行模型需要避免绑定单机假设，以便后续接入 Cloudflare Load Balancer 和多节点运行。`DEPLOYMENT_MODE` 只描述部署拓扑，第一阶段默认值为 `single_node`；服务监听范围由入口网关和端口绑定决定，公开访问基址由 `PUBLIC_BASE_URL` 决定。

服务内部遵循 `handler -> service -> repository` 分层。`*gin.Context` 仅保留在 handler 层，业务层统一使用 `context.Context`。

第二阶段开始将 HTTP 路由层统一迁移到 Gin。阶段一已经存在的 `/`、`/healthz`、`/readyz`、`/metrics` 和 `/api/runtime/node` 必须保持路径和响应语义兼容；新增业务 API 统一挂载在 `/api/v1` 路由组下。Gin 中间件只处理 request id、访问日志、错误映射、响应格式和跨域等横切能力，不承载订阅、抓取、阅读状态等业务规则。

## 2. 技术选型

| 职责 | 选型 |
| --- | --- |
| HTTP 框架 | Gin |
| 数据库 | PostgreSQL |
| Redis | 第一阶段不引入；后续作为缓存、任务队列、限流、短期状态或分布式锁的可选实现 |
| ORM | GORM |
| 迁移 | `golang-migrate/migrate` + 正式 SQL `up/down` 迁移文件 |
| Web 界面 | 采用前后端分离架构：Go 提供 RESTful API（含 OpenAPI 文档），Vue 3 + Vite 独立前端工程，支持后续扩展移动端 |
| RSS 解析 | gofeed |
| 定时任务 | gocron |
| 金融行情 | 自定义 `MarketDataProvider` 接口，按市场接入 Yahoo Finance、Finnhub、Polygon、Tushare、AkShare 等 |
| 技术指标 | MVP 先手写简单指标，后续评估 `go-talib` |
| AI 调用 | 自定义 `LLMClient` 接口，兼容 OpenAI API、Ollama 和 OpenAI-compatible 服务 |
| 通知 | ntfy、企业微信机器人、企业微信自建应用、微信公众号 |
| 日志 | log/slog |
| 指标 | prometheus/client_golang |
| 链路追踪 | OpenTelemetry，HTTP 入口使用 Gin instrumentation，后续 service/repository/fetcher 等关键边界手动补充 span |
| 日志存储 | 本地开发先使用 stdout/stderr + Docker `json-file`；完整观测阶段接入 Loki 或等价日志查询系统 |
| 错误追踪 | 统一错误模型 + request id + trace id + panic recovery；后续生产环境可评估 Sentry 或等价错误聚合系统 |
| 契约 | OpenAPI |
| 当前部署 | Docker Compose + Caddy + Cloudflare Tunnel |
| 当前入口 | `https://aroen.eu.cc`、`https://localhost:8443` |
| 后续入口 | Cloudflare Load Balancer 或多 Tunnel 多节点 |

### 2.1 可观测性设计

当前阶段已经具备基础可观测能力：`log/slog` 输出结构化日志、Gin 中间件生成 `X-Request-ID`、`/metrics` 暴露 Prometheus 指标、`/readyz` 检查数据库和迁移状态。但这只能满足本地开发和基础排障，还不构成完整的日志存储、错误追踪和请求链路追踪系统。

最小业务闭环完成后，必须立即进入完整观测系统建设。推荐目标形态如下：

```text
HTTP request
  -> Gin RequestID / OpenTelemetry middleware
  -> handler
  -> service
  -> repository / fetcher / notifier / llm
  -> structured JSON slog
  -> stdout/stderr
  -> Docker logging / collector
  -> Loki / Tempo / Prometheus / Grafana
```

日志设计：

- 应用日志保持输出到 stdout/stderr，避免业务进程直接管理日志文件和轮转。
- 本地开发继续使用 Docker `json-file` driver，所有 Compose 服务统一设置 `max-size=10m` 和 `max-file=3`，控制容器原始日志磁盘占用。
- 完整观测阶段新增 Loki 或等价日志系统，日志采集器负责读取容器日志并写入日志存储；本地 Loki 保留期为 168 小时，并启用 compactor retention 删除，避免日志长期累积。
- `/healthz` 和 `/metrics` 的成功请求属于健康检查与指标采集噪声，访问日志降为 debug；失败请求仍保留日志，便于排查监控链路故障。
- `slog` 输出格式从 text 调整为 JSON，固定字段包括 `service`、`environment`、`node_id`、`request_id`、`trace_id`、`span_id`、`method`、`path`、`status`、`duration_ms`、`operation`、`error`。

链路追踪设计：

- 请求入口生成或继承 `X-Request-ID`，同时接入 OpenTelemetry trace。
- request id 用于用户反馈和人工排查；trace id 用于跨 handler、service、repository、外部调用的链路定位。
- `request_id`、`trace_id` 和 `span_id` 必须写入 `context.Context`，业务层不得依赖 `*gin.Context` 获取这些字段。
- HTTP handler、RSS 抓取、数据库访问、AI 调用、通知发送和周期任务应逐步建立 span。

错误追踪设计：

- 业务层使用统一错误类型表达业务码、用户可读消息、内部原因和操作名。
- service/repository/fetcher/notifier/llm 返回错误时使用 `%w` 保留错误链。
- handler 层统一将错误映射为 HTTP 状态码、业务错误码、用户可读消息、`request_id` 和可选 `trace_id`。
- panic 由 Recovery 中间件捕获，记录 request id、trace id、method、path、panic 摘要，并返回统一 500 响应。
- 后续如接入 Sentry，只作为错误聚合增强，不替代日志、指标和 trace 的基础链路。

部署组件建议：

- Prometheus：继续抓取 `/metrics`，负责指标查询和告警规则数据源。
- Loki：存储和查询结构化日志。
- Tempo：存储 OpenTelemetry trace。
- Grafana：统一查看日志、指标、trace 和核心 Dashboard。
- OpenTelemetry Collector：作为 traces、metrics、logs 的统一采集和转发入口；本地阶段可先只接 traces。
- 本地保留策略：Prometheus TSDB 保留 7 天，Loki 日志保留 168 小时，Tempo trace 保留 24 小时；长期生产环境需要按磁盘容量、采样率和查询需求重新制定保留期。

## 3. 目录结构

```text
messageFeed/
├── cmd/api/main.go
├── internal/config/
├── internal/domain/
├── internal/repository/
├── internal/service/
├── internal/handler/
├── internal/catalog/
├── internal/importer/
├── internal/fetcher/
├── internal/recommender/
├── internal/scheduler/
├── internal/agent/
├── internal/acquisition/
├── internal/profile/
├── internal/control/
├── internal/market/
├── internal/alert/
├── internal/llm/
├── internal/notifier/
├── internal/runtime/
├── api/
├── web/
├── migrations/
├── deploy/
├── test/
├── go.mod
└── Makefile
```

## 4. 模块职责

| 模块 | 职责 |
| --- | --- |
| `config` | 加载环境变量、默认值、数据库、模型、通知和抓取配置 |
| `domain` | 定义实体、枚举、领域错误和业务常量 |
| `repository` | 封装 PostgreSQL 访问和事务 |
| `service` | 编排订阅、抓取、导入、摘要、通知等用例 |
| `handler` | Gin 路由、中间件、参数绑定、响应渲染和错误映射 |
| `catalog` | 管理推荐源目录、源分类、源健康状态和源搜索 |
| `importer` | 处理 OPML、URL 批量导入和目录批量订阅 |
| `fetcher` | 抓取 RSS、Atom、JSON Feed，并规范化条目 |
| `recommender` | 管理推荐 Feed 候选池、排序、推荐原因、未订阅来源标注和用户反馈 |
| `scheduler` | 编排周期抓取、日报生成、失败重试和通知任务 |
| `agent` | 管理项目级 AI Agent，包括能力注册、意图解析、计划生成、风险校验、执行编排和审计 |
| `acquisition` | 管理主动网络采集，包括静态网页抽取、网页变化监控、搜索型采集、快照和来源评估 |
| `profile` | 管理阅读行为事件、用户兴趣标签、短期兴趣、长期偏好和负反馈模型 |
| `control` | 保留自然语言设置控制兼容边界；后续设置控制能力归入 `agent` 的能力注册和执行框架 |
| `market` | 管理金融标的、行情源、行情快照、市场日历和技术指标 |
| `alert` | 管理内容告警、金融告警、规则评估、冷却时间和幂等触发 |
| `llm` | 抽象模型调用、token 统计、结构化摘要和错误记录 |
| `notifier` | 抽象 ntfy、微信和后续通知通道 |
| `runtime` | 管理节点标识、部署模式、就绪状态、任务锁和后续分布式运行接口 |

## 5. 核心数据模型

| 表 | 说明 |
| --- | --- |
| `sources` | 用户订阅源，包含名称、类型、URL、抓取周期、状态、标签、权重、`user_id` |
| `source_catalog_entries` | 内置候选源，包含名称、URL、站点、分类、热度、语言、来源出处、健康状态 |
| `source_import_jobs` | 导入任务，记录导入类型、状态、成功数量、失败数量和错误明细 |
| `items` | 抓取条目，包含标题、URL、规范化 URL、摘要、正文片段、发布时间、哈希、来源 |
| `ai_generated_items` | AI 源条目元数据，记录生成类型、模型、提示词版本、输入范围、token、成本估算和风险等级 |
| `web_acquisition_tasks` | 主动网络采集任务，记录搜索、网页抽取、网页监控的目标、计划、状态和风险等级 |
| `web_snapshots` | 主动采集快照，记录 URL、标题、正文 hash、抓取时间、HTTP 状态、抽取方法和失败原因 |
| `user_item_states` | 用户阅读状态，包含已读、收藏、隐藏、首次打开、最近打开、打开次数、最大阅读进度和累计主动停留时间 |
| `user_item_interaction_events` | 用户阅读行为事件，记录曝光、打开、阅读进度、点击原文、收藏、隐藏、不感兴趣和减少类似推荐等行为 |
| `user_interest_profiles` | 用户兴趣画像摘要，记录画像版本、摘要文本、更新时间和置信度 |
| `user_interest_tags` | 用户兴趣标签，记录显式偏好、隐式偏好、短期兴趣、长期兴趣和负反馈权重 |
| `user_interest_evidence` | 用户画像证据，记录标签形成依据、关联条目、来源、事件和分数变化 |
| `feed_view_preferences` | 用户 Web 阅读模式偏好，记录时间线或推荐 Feed 的最近选择 |
| `feed_recommendations` | 推荐 Feed 记录，包含用户、条目、推荐分数、推荐原因、来源订阅状态和曝光状态 |
| `recommendation_feedback` | 推荐反馈，包含隐藏、减少类似推荐、不感兴趣、订阅来源等反馈类型 |
| `interest_rules` | 兴趣规则，包含关键词、标签、权重、启用状态和匹配范围 |
| `summaries` | AI 摘要，包含日报、专题摘要、重大事件摘要和模型调用信息 |
| `notification_channels` | 通知通道，包含通道类型、启用状态和配置引用 |
| `notification_recipients` | 通知接收目标，包含微信 openid、企业微信 user_id、群机器人目标或 ntfy topic |
| `notifications` | 通知记录，包含触发原因、通道、接收目标、状态和失败原因 |
| `agent_commands` | Agent 自然语言命令和系统事件入口，记录原始输入、解析状态、模型版本和用户上下文 |
| `agent_capabilities` | Agent 可调用能力清单，记录目标类型、允许动作、风险等级、确认策略和 service 绑定 |
| `agent_plans` | Agent 结构化计划，记录计划状态、风险等级、影响摘要、确认策略、幂等键和回滚摘要 |
| `agent_plan_steps` | Agent 计划步骤，记录目标对象、操作类型、参数摘要、执行状态和错误 |
| `agent_audit_logs` | Agent 审计日志，记录用户确认、执行结果、模型输出、操作者、request id 和 trace id |
| `agent_sessions` | Agent 会话，记录用户、模型、上下文窗口、状态、开始时间和结束时间 |
| `agent_turns` | Agent 执行轮次，记录 session 内一次用户输入或系统触发的状态、模型、开始结束时间和错误 |
| `agent_transcript_entries` | Agent transcript 条目，记录用户、模型、工具、系统、摘要和自定义事件等上下文输入输出 |
| `agent_context_windows` | Agent 上下文窗口，记录窗口序号、压缩原因、压缩前后 token、摘要和创建时间 |
| `agent_context_segments` | Agent 上下文语义块，记录语义类型、起止 transcript、摘要、关键词、实体、来源引用和归档引用 |
| `agent_context_archives` | Agent 上下文归档，记录原文位置、内容 hash、摘要、存储类型和创建时间 |
| `agent_recall_events` | Agent 回忆事件，记录召回查询、召回引用、召回原因和使用位置 |
| `agent_memory_promotions` | Agent 记忆提升记录，记录从会话、阅读行为、AI 源或归档提升到画像或长期记忆的候选与确认状态 |
| `agent_eval_cases` | Agent 评测用例，记录输入状态、预期计划、预期工具调用、预期状态变更、禁止行为和评分规则 |
| `agent_eval_runs` | Agent 评测批次，记录模型、提示词版本、代码版本、开始结束时间、总分、成本和耗时 |
| `agent_eval_results` | Agent 单用例评测结果，记录实际意图、计划、工具调用、状态差异、指标得分、失败原因和人工复核状态 |
| `control_commands` | 用户自然语言设置指令，包含原始文本、解析状态、模型版本和用户上下文引用 |
| `control_capabilities` | 可被自然语言控制的设置能力清单，包含能力名称、目标类型、风险等级、确认策略和回滚支持 |
| `control_change_plans` | 设置变更计划，包含计划状态、风险等级、确认策略、影响摘要和回滚摘要 |
| `control_change_steps` | 变更计划步骤，记录目标对象、操作类型、变更前后差异、执行状态和错误 |
| `control_audit_logs` | 设置控制审计日志，记录用户确认、执行结果、模型输出、操作者和时间 |
| `market_instruments` | 金融标的，包含 symbol、市场、交易所、名称、类型、币种和启用状态 |
| `market_data_providers` | 行情数据源配置，包含 provider、覆盖市场、速率限制、延迟级别和启用状态 |
| `market_quotes` | 行情快照，包含当前价、前收价、涨跌幅、成交量、行情时间和数据源 |
| `market_watchlists` | 用户关注列表，记录用户与金融标的的关注关系 |
| `market_alert_rules` | 金融告警规则，记录规则类型、阈值、冷却时间、启用状态和通知目标 |
| `market_alert_events` | 金融告警事件，记录触发快照、规则、AI 解读、幂等键和发送状态 |

关键唯一约束：

- `sources(user_id, normalized_url)`
- `items(source_id, normalized_url)`
- `items(source_id, raw_guid)`，当源提供稳定 GUID 时启用
- `ai_generated_items(item_id)`
- `web_snapshots(task_id, content_hash, fetched_at)`
- `feed_view_preferences(user_id)`
- `feed_recommendations(user_id, item_id, recommendation_batch)`
- `source_catalog_entries(source_origin, source_key)`
- `user_interest_tags(user_id, tag, category)`
- `agent_plans(user_id, dedupe_key)`
- `agent_plan_steps(plan_id, step_order)`
- `agent_turns(session_id, started_at)`
- `agent_context_windows(session_id, window_number)`
- `agent_context_segments(session_id, start_entry_id, end_entry_id)`
- `agent_context_archives(content_hash)`
- `agent_eval_cases(case_key)`
- `agent_eval_results(run_id, case_id)`
- `control_change_plans(user_id, dedupe_key)`
- `control_change_steps(plan_id, step_order)`
- `market_instruments(market, symbol)`
- `market_quotes(instrument_id, provider, quote_time)`
- `market_alert_events(rule_id, dedupe_key)`

分布式升级预留：

- `notifications` 应增加业务幂等键，例如 `dedupe_key`，避免日报或重大事件重复推送。
- 自动抓取、日报和推送任务应通过 `TaskLocker` 接口执行互斥。第一阶段可以使用 PostgreSQL advisory lock 或单节点实现，后续多节点部署时切换为共享数据库锁；如引入 Redis，也只能作为 `TaskLocker`、短期缓存、限流或任务队列的替代实现，不改变业务层接口。
- 配置层预留 `APP_NODE_ID`、`DEPLOYMENT_MODE`、`PUBLIC_BASE_URL`、`BIND_ADDR` 和 `TRUSTED_PROXY_CIDRS`。

存储与缓存边界：

- PostgreSQL 是系统主存储，负责订阅、条目、阅读状态、阅读行为事件、AI 源生成元数据、主动采集任务与快照、用户兴趣画像、导入任务、推荐记录、通知记录、Agent 计划与审计、设置控制审计、行情快照和告警事件等持久数据。
- Redis 不作为主存储，不承担审计、业务幂等、关系查询或最终持久化职责。
- 后续可定义 `CacheStore`、`RateLimiter`、`TaskQueue` 和 `TaskLocker` 等接口，并分别提供 PostgreSQL、进程内或 Redis 实现；service 层不得直接依赖 Redis 客户端。

金融行情接口抽象：

```text
MarketDataProvider
├── Quote(ctx, instrument) -> MarketQuote
├── BatchQuotes(ctx, instruments) -> []MarketQuote
├── ProviderStatus(ctx) -> MarketProviderStatus
└── Capabilities() -> ProviderCapabilities

MarketAlertEngine
├── Evaluate(ctx, quote, rules) -> []MarketAlertEvent
└── BuildDedupeKey(rule, quote) -> string
```

AI Agent 不直接参与行情拉取和阈值判断。确定性规则命中后，服务将行情快照、规则、近期相关资讯、主动网络采集结果和必要上下文交给 `LLMClient` 生成解释性文本，写入 `messageFeed AI` 源，并按通知策略交给 `notifier` 发送。详细金融方案见 `docs/financial-agent-plan.md`。

项目级 Agent 接口抽象：

```text
AgentCapabilityRegistry
├── Register(capability) -> error
├── List(ctx, userScope) -> []AgentCapability
├── Match(ctx, intent) -> []AgentCapability
└── Search(ctx, query, scope) -> []AgentCapability

AgentSessionManager
├── CreateSession(ctx, user, scope) -> AgentSession
├── ResumeSession(ctx, sessionID) -> AgentSession
├── StartTurn(ctx, session, input) -> AgentTurn
├── CompleteTurn(ctx, turn, result) -> error
└── CancelTurn(ctx, turn, reason) -> error

AgentInterpreter
├── Interpret(ctx, command, scope) -> AgentIntent
└── BuildClarifyingQuestion(ctx, ambiguity) -> AgentQuestion

AgentPlanner
├── BuildPlan(ctx, intent) -> AgentPlan
├── ValidatePlan(ctx, plan) -> PlanValidationResult
└── EstimateImpact(ctx, plan) -> PlanImpact

AgentExecutor
├── Execute(ctx, approvedPlan) -> AgentExecutionResult
└── Rollback(ctx, plan) -> AgentRollbackResult

AgentAuditLogger
├── RecordCommand(ctx, command) -> error
├── RecordPlan(ctx, plan) -> error
├── RecordApproval(ctx, approval) -> error
└── RecordStepResult(ctx, result) -> error

AgentContextManager
├── BuildSnapshot(ctx, user, taskScope) -> AgentContextSnapshot
├── EstimatePressure(ctx, snapshot) -> ContextPressure
├── Compact(ctx, session, policy) -> ContextCompactionResult
└── Recall(ctx, query, scope) -> []RecallResult
```

所有 Agent 可执行能力必须注册到 `AgentCapabilityRegistry`。模型只能生成意图、计划、说明文本和工具参数摘要；实际执行必须由 `AgentExecutor` 调用已注册能力和既有 service 接口完成。能力暴露分为 `core`、`deferred`、`hidden`：少量核心能力默认进入模型上下文，延迟能力通过 `capability.search` 发现后再暴露 schema，隐藏能力只能由后端策略或已确认计划内部调用。`AgentContextManager` 负责组装冻结记忆快照、估算上下文压力、按完整语义块归档历史、生成压缩摘要并通过回忆工具取回历史证据。完整 Agent 方案见 `docs/agent-plan.md`。

本项目不 fork `OpenAI Codex`，也不迁移其 Rust 运行时。Codex 与 Claude Code 只作为架构参考：吸收 `Session / Turn`、工具路由、延迟工具发现、上下文窗口压缩、线程持久化、召回预算和 `allow / prompt / forbidden` 权限决策思想；具体实现应保持在 Go 后端、PostgreSQL 主存储和既有 service 边界内。

主动网络采集接口抽象：

```text
WebAcquisitionProvider
├── FetchPage(ctx, target) -> WebSnapshot
├── Monitor(ctx, task) -> WebChangeResult
└── Capabilities() -> AcquisitionCapabilities

SearchProvider
├── Search(ctx, query, options) -> []SearchResult
└── ProviderStatus(ctx) -> SearchProviderStatus

PageExtractor
├── Extract(ctx, rawPage) -> ExtractedPage
└── Method() -> string
```

搜索结果不能直接视为事实，必须经过抓取、去重、来源评估和快照记录后才能进入摘要、推荐或通知。

阅读行为和画像接口抽象：

```text
InteractionRecorder
├── Record(ctx, event) -> error
└── Aggregate(ctx, userID, itemID) -> UserItemState

InterestProfileBuilder
├── UpdateFromEvent(ctx, event) -> error
├── SuggestProfileChanges(ctx, userID) -> []InterestProfileChange
└── ExplainTag(ctx, userID, tag) -> []InterestEvidence
```

用户画像必须区分显式偏好、隐式偏好、短期兴趣、长期兴趣和负反馈。长期偏好不应由模型基于单次行为静默写入。

自然语言设置控制接口抽象：

```text
ControlInterpreter
├── Interpret(ctx, command, scope) -> ControlIntent
└── BuildClarifyingQuestion(ctx, ambiguity) -> ControlQuestion

ChangePlanner
├── BuildPlan(ctx, intent) -> ControlChangePlan
├── ValidatePlan(ctx, plan) -> PlanValidationResult
└── EstimateImpact(ctx, plan) -> PlanImpact

ControlExecutor
├── Execute(ctx, approvedPlan) -> ControlExecutionResult
└── Rollback(ctx, plan) -> ControlRollbackResult

ControlCapabilityRegistry
├── Register(capability) -> error
├── List(ctx, userScope) -> []ControlCapability
└── Match(ctx, intent) -> []ControlCapability
```

`control` 是早期自然语言设置控制边界，后续应逐步收敛到 `agent` 的能力注册、计划和执行框架中。模型只负责自然语言理解、候选计划生成和说明文本生成。设置变更必须通过 `ControlExecutor` 或 `AgentExecutor` 调用既有 `service` 接口完成，不允许模型直接写数据库、直接调用 repository 或绕过权限校验。

所有用户可配置能力都应注册到 `AgentCapabilityRegistry` 或兼容的 `ControlCapabilityRegistry`。新增业务设置时，如果未声明控制能力、风险等级、确认策略和回滚方式，则默认不得由自然语言控制面自动执行。

## 6. API 草案

健康检查、就绪检查、指标和运行时节点信息保持未版本化路径。除这些基础端点外，业务 API 均使用 `/api/v1` 前缀。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/healthz` | 存活检查 |
| `GET` | `/readyz` | 依赖就绪检查 |
| `GET` | `/metrics` | Prometheus 指标 |
| `GET` | `/api/runtime/node` | 查询当前节点标识、部署模式和公开访问基址 |
| `POST` | `/api/v1/agent/commands` | 提交项目级 Agent 自然语言命令并生成计划 |
| `GET` | `/api/v1/agent/plans/{id}` | 查询 Agent 计划 |
| `POST` | `/api/v1/agent/plans/{id}/approve` | 确认并执行 Agent 计划 |
| `POST` | `/api/v1/agent/plans/{id}/reject` | 拒绝 Agent 计划 |
| `POST` | `/api/v1/agent/plans/{id}/rollback` | 回滚可回滚的 Agent 计划 |
| `GET` | `/api/v1/agent/audit-logs` | 查询 Agent 审计记录 |
| `GET` | `/api/v1/agent/capabilities` | 查询 Agent 可用能力 |
| `GET` | `/api/v1/agent/capabilities/search` | 按自然语言、目标类型或动作检索延迟能力 |
| `GET` | `/api/v1/agent/sessions/{id}/turns` | 查询 Agent 会话执行轮次 |
| `POST` | `/api/v1/agent/sessions/{id}/turns/{turn_id}/cancel` | 取消正在执行的 Agent turn |
| `GET` | `/api/v1/agent/sessions/{id}/context` | 查询 Agent 会话当前上下文快照、使用率、保护区和压缩摘要 |
| `GET` | `/api/v1/agent/sessions/{id}/context-windows` | 查询 Agent 会话上下文窗口和压缩历史 |
| `POST` | `/api/v1/agent/sessions/{id}/context/compact` | 对指定 Agent 会话触发一次上下文压缩评估 |
| `GET` | `/api/v1/agent/sessions/{id}/archives` | 查询 Agent 会话语义归档块和摘要索引 |
| `GET` | `/api/v1/agent/archives/{id}` | 查询单个上下文归档块的原文引用和元数据 |
| `GET` | `/api/v1/agent/archives/{id}/preview` | 查询单个上下文归档块的短预览 |
| `POST` | `/api/v1/agent/sessions/{id}/recall` | 按任务、关键词、实体、时间或引用执行受控回忆 |
| `GET` | `/api/v1/agent/sessions/{id}/recall-events` | 查询 Agent 会话记忆召回记录 |
| `GET` | `/api/v1/agent/memory-promotions` | 查询候选记忆提升记录 |
| `POST` | `/api/v1/agent/memory-promotions/{id}/approve` | 确认将候选证据提升为画像或长期记忆 |
| `POST` | `/api/v1/agent/memory-promotions/{id}/reject` | 拒绝候选记忆提升 |
| `GET` | `/api/v1/agent/eval/cases` | 查询 Agent 评测用例 |
| `POST` | `/api/v1/agent/eval/runs` | 创建 Agent 评测批次 |
| `GET` | `/api/v1/agent/eval/runs/{id}` | 查询 Agent 评测批次结果 |
| `GET` | `/api/v1/agent/eval/runs/{id}/results` | 查询 Agent 单用例评测结果 |
| `POST` | `/api/v1/agent/ai-items` | 手动生成 AI 源条目，例如日报、周报或专题报告 |
| `POST` | `/api/v1/acquisition/tasks` | 创建主动网络采集任务 |
| `GET` | `/api/v1/acquisition/tasks` | 查询主动网络采集任务 |
| `POST` | `/api/v1/acquisition/tasks/{id}/run` | 手动执行一次主动采集 |
| `GET` | `/api/v1/acquisition/snapshots` | 查询网页采集快照 |
| `POST` | `/api/v1/items/{id}/interactions` | 记录条目阅读行为事件 |
| `GET` | `/api/v1/profile/interests` | 查询用户兴趣画像和标签 |
| `PATCH` | `/api/v1/profile/interests/{tag}` | 调整用户兴趣标签权重或状态 |
| `GET` | `/api/v1/profile/interests/{tag}/evidence` | 查询兴趣标签形成依据 |
| `POST` | `/api/v1/control/commands` | 提交自然语言设置指令并生成变更计划 |
| `GET` | `/api/v1/control/change-plans/{id}` | 查询设置变更计划 |
| `POST` | `/api/v1/control/change-plans/{id}/approve` | 确认并执行设置变更计划 |
| `POST` | `/api/v1/control/change-plans/{id}/reject` | 拒绝设置变更计划 |
| `POST` | `/api/v1/control/change-plans/{id}/rollback` | 回滚可回滚的设置变更 |
| `GET` | `/api/v1/control/audit-logs` | 查询自然语言设置控制审计记录 |
| `POST` | `/api/v1/market/instruments` | 新增金融标的 |
| `GET` | `/api/v1/market/instruments` | 查询金融标的 |
| `POST` | `/api/v1/market/watchlists` | 新增关注标的 |
| `GET` | `/api/v1/market/watchlists` | 查询关注列表 |
| `GET` | `/api/v1/market/quotes` | 查询最新行情快照 |
| `POST` | `/api/v1/market/alert-rules` | 新增金融告警规则 |
| `GET` | `/api/v1/market/alert-rules` | 查询金融告警规则 |
| `GET` | `/api/v1/market/alert-events` | 查询金融告警事件 |
| `POST` | `/api/v1/market/alert-rules/{id}/test` | 使用最近行情测试规则与通知 |
| `POST` | `/api/v1/sources` | 新增订阅源 |
| `GET` | `/api/v1/sources` | 查询订阅源 |
| `PATCH` | `/api/v1/sources/{id}` | 更新订阅源 |
| `POST` | `/api/v1/sources/{id}/fetch` | 手动抓取 |
| `GET` | `/api/v1/source-catalogs` | 查询推荐源 |
| `GET` | `/api/v1/source-catalogs/search` | 搜索推荐源 |
| `POST` | `/api/v1/sources/import/opml` | OPML 导入 |
| `POST` | `/api/v1/sources/import/catalog` | 从推荐源批量订阅 |
| `GET` | `/api/v1/feed/timeline` | 按时间线查询已订阅来源条目 |
| `GET` | `/api/v1/feed/recommendations` | 查询推荐 Feed，包含已订阅和未订阅候选来源条目 |
| `PUT` | `/api/v1/feed/view-mode` | 保存用户 Web 阅读模式偏好 |
| `POST` | `/api/v1/feed/recommendations/{id}/feedback` | 提交推荐反馈，例如不感兴趣、隐藏、减少类似推荐 |
| `GET` | `/api/v1/items` | 查询 Feed |
| `GET` | `/api/v1/items/{id}` | 查询条目详情 |
| `POST` | `/api/v1/items/{id}/read` | 标记已读 |
| `POST` | `/api/v1/items/{id}/favorite` | 标记收藏 |
| `POST` | `/api/v1/items/{id}/hide` | 隐藏条目 |
| `POST` | `/api/v1/summaries/daily` | 手动触发日报 |
| `GET` | `/api/v1/summaries` | 查询摘要 |
| `POST` | `/api/v1/notification-channels` | 新增通知通道 |
| `GET` | `/api/v1/notification-channels` | 查询通知通道 |
| `POST` | `/api/v1/notification-channels/{id}/test` | 测试通知 |

## 7. 外部参考边界

Agent 相关参考项目只作为架构来源，不作为代码迁移来源。详细分析见 `docs/agent-plan.md`，架构层保留以下边界：

| 参考项目 | 技术路线 | 本项目吸收点 | 本项目边界 |
| --- | --- | --- | --- |
| `Eino` | Go 组件化 Agent、ADK、Runner、Tool、workflow graph、callback、checkpoint | 将确定性业务流程包装为 capability，Agent 通过 Runner 执行，关键节点接入 trace 和审计 | 不以通用 DeepAgent 自主协作为早期默认模式 |
| `LangChainGo` | `Agent -> Executor -> Tool -> Observation`，Memory buffer/window/summary | 执行循环必须有最大步数，工具结果进入结构化 observation，Memory 分层管理 | 不采用字符串 ReAct 解析作为核心协议 |
| `ai_agent_scaffold_go` | DDD、端口适配器、配置驱动装配、HTTP Agent 运行时 | 使用 Registry、SessionStore、OpenAI-compatible adapter 和清晰端口边界 | capability 主事实来源进入 PostgreSQL，不以 YAML 为主 |
| `Hermes Agent` | 多通道个人 Agent、工具网关、持久记忆、审批、调度 | 抽象 Web、企业微信、ntfy 等入口；建立审批、持久用户画像和定时任务隔离 | 不引入终端执行、云 VM 和技能自学习作为早期能力 |
| `OpenClaw` | Gateway 运行时、session store、transcript、compaction、pre-compaction memory flush | session 元数据和 transcript 分层，压缩时保护工具调用对，建立归档与召回 | 不使用本地 JSONL 作为主事实来源 |
| `Claude Code` | ToolSearch、Context Collapse、核心工具和延迟工具分层 | capability 分为 `core/deferred/hidden`，通过 `capability.search` 和 `capability.execute` 降低上下文成本 | 不依赖 provider 私有 tool reference |
| `OpenAI Codex` | ThreadStore、LiveThread、Session/Turn、权限策略、compact handoff | history append 与 metadata update 分离，turn 作为运行单元，权限决策落到 `allow/prompt/forbidden` | 不 fork、不迁移 Rust 代码、不引入代码执行型 CLI 能力 |

由以上参考推导出的本项目 Agent 架构是“领域受控 Agent Runtime”：运行时、能力、记忆、策略和评估五个平面都在 Go 后端内实现，PostgreSQL 是 session、turn、transcript、capability、plan、audit、profile 和 eval 的主存储；模型只生成意图、计划、参数摘要和解释文本，所有实际变更必须通过已注册 capability 调用既有 service。

- `Folo`：参考源发现、源分类、onboarding feed、订阅组织和 AI 阅读体验。
- `Miniflux`：参考 RSS 抓取、阅读状态和订阅管理。
- `RSSHub`：作为非标准来源桥接，不作为 MVP 的硬依赖。
- `QuantConnect LEAN`：参考证券、数据源、市场日历、回测和实盘引擎的边界，但本项目不引入完整交易引擎。
- `AkShare`、`Tushare`、`Yahoo Finance`、`Finnhub`、`Polygon`、`Alpha Vantage`：参考金融数据源能力、覆盖市场和速率限制，以 `MarketDataProvider` 方式接入。
- `TradingView Alert`、`Grafana Alerting`：参考规则、冷却时间、通知模板和告警历史模型。
- `go-talib`：作为后续技术指标计算的 Go 侧候选库。

## 8. 工程约束

- service、repository 方法首参数为 `context.Context`。
- 错误链使用 `%w` 包装，由 handler 统一映射 HTTP 响应。
- 日志统一使用 `log/slog`，最小业务闭环后调整为 JSON 输出并接入日志存储。
- request id、trace id 和 span id 必须通过 `context.Context` 向下游传递；业务层不得从 Gin 上下文读取观测字段。
- 完整观测阶段必须接入 OpenTelemetry 链路追踪，并让日志字段与 trace 关联。
- 密钥不进入源码、测试数据和镜像层。
- 外部依赖测试优先使用真实依赖或 testcontainers。
- 第一阶段即暴露健康检查、就绪检查和指标端点。
- 金融行情和 AI 解读必须保留“不构成投资建议”的产品边界。
- 行情数据必须记录 provider 和 quote_time，避免用户误判实时性。
- Agent 和自然语言设置控制必须保存计划、用户确认、执行结果和审计日志。
- 模型不得接收密钥、token、Webhook URL 等敏感明文，不得直接修改数据库。
- Agent 执行策略必须落到 `allow`、`prompt`、`forbidden`，风险等级仅作为策略输入。
- 同一 Agent session 同时只能存在一个 active turn；新输入应排队、取消当前 turn 或创建新 session，不得并发修改同一任务状态。
- Agent 能力 schema 不应全部长期进入 prompt；默认只暴露核心能力，延迟能力通过能力搜索工具按需暴露。
- Agent 上下文压缩必须保留原文归档引用；高风险操作不得只依赖摘要执行。
- Agent 召回内容必须标注来源、时间和可信等级，且不得覆盖系统规则、权限策略和能力边界；全文召回应有单轮预算、使用位置和召回原因记录。
- Agent 评测必须保存输入状态、实际计划、工具调用、状态差异、评分结果和审计引用；安全对抗评测应作为 Agent 能力变更的回归门禁。
- 主动网络采集必须保存来源 URL、抓取时间、内容 hash、抽取方法和失败原因；搜索结果不得直接作为事实使用。
- 阅读行为采集必须有明确推荐、统计或 Agent 决策用途；不得采集鼠标轨迹、键盘轨迹、剪贴板和浏览器外部行为。
- 用户画像必须可解释、可编辑、可回滚；长期偏好不得由单次行为静默确认。

## 9. 部署设计

### 9.1 当前阶段

当前阶段采用本地单节点部署：

- 服务运行在 WSL 本机。
- PostgreSQL 由本地 Docker Compose 提供。
- 数据库 schema 由一次性 `migrate/migrate` 服务执行 pending `up` 迁移，PostgreSQL 官方镜像不扫描项目 `migrations` 目录。
- 默认部署拓扑为 `DEPLOYMENT_MODE=single_node`，表示服务、调度器和数据库处于同一单节点部署边界内，不表示只能本机访问。
- 开发态 API 与 Web 服务只暴露在 Docker 网络内，由 Caddy `gateway-dev` 统一反向代理。
- 宿主机统一入口为 `127.0.0.1:8443`，不再监听 `0.0.0.0:8443`；局域网 IP 和 Tailscale IP 不能直接访问服务。
- Cloudflare Tunnel 通过 Docker 内部地址 `https://gateway-dev:8443` 访问 Caddy，公网域名为 `https://aroen.eu.cc`。
- Cloudflare 到本地源站的 TLS 使用本地 CA 签发的 `gateway-dev` 证书，`cloudflared` 通过挂载的 CA bundle 验证该证书，不依赖 Cloudflare 面板中的 `No TLS Verify`。
- `PUBLIC_BASE_URL` 在 Cloudflare 开发模式下配置为 `https://aroen.eu.cc`。
- `/readyz` 检查数据库连接、`schema_migrations` 迁移状态和必要配置，不检查非关键外部服务。

### 9.2 数据库迁移策略

数据库迁移采用版本化 SQL 文件和 `golang-migrate/migrate`。本地常规启动入口为 `make compose-up`，其顺序为启动 PostgreSQL、显式执行 pending `up` 迁移、再启动 API。空卷首次初始化时，直接 `docker compose up -d --build` 也能通过一次性 migrate 服务完成初始化；已有数据卷的后续迁移应使用 `make compose-up` 或 `make migrate-up`。API 启动和 `/readyz` 只检查迁移状态，不执行 schema 修改。

后续多节点部署继续沿用该工具，不计划迁移到 Flyway、Liquibase 或 Goose。生产或准生产发布时，迁移应作为独立发布步骤在 API 滚动更新前执行：先备份数据库，再执行 `migrate up`，确认 `schema_migrations` 非 dirty 且版本符合预期，最后启动或更新 API 节点。`down` 文件保留为显式回滚资产，常规故障处理优先采用备份恢复或前向修复。

### 9.3 后续升级路径

后续分布式部署不改变业务模块，只替换入口层和运行时配置：

- Cloudflare Tunnel：用于将某台或多台私有网络机器暴露到域名。
- Cloudflare Load Balancer：用于按 `/readyz` 将流量打到健康节点。
- 多节点 API：所有节点共享 PostgreSQL，API 层保持无状态。
- 定时任务：多个节点可以启动 scheduler，但任务执行前必须通过共享任务锁。
- Redis：后续可作为缓存、队列、限流或任务锁实现加入，但不能替代 PostgreSQL 主存储。
- 通知发送：通过 `dedupe_key` 与发送记录避免重复推送。

## 10. 核心业务链路

### 10.1 AI Agent 与 AI 源链路

AI Agent 采用“用户命令或系统事件 -> session/turn -> 上下文与记忆快照 -> 意图解析 -> 能力检索 -> 结构化计划 -> 策略校验 -> 用户确认 -> 受控执行 -> AI 源沉淀 -> 通知或反馈”的链路：

1. `handler` 接收自然语言命令、系统事件或调度触发，并记录 `agent_commands`。
2. `AgentSessionManager` 创建或恢复 session，`AgentTurnRunner` 创建 turn，并保证同一 session 内 active turn 串行执行。
3. `AgentContextManager` 组装冻结记忆快照，读取用户画像、当前任务、相关条目、AI 源报告、网页快照和能力边界。
4. `agent` 调用 `llm` 解析意图，优先匹配核心能力；能力不足时通过 `capability.search` 检索延迟能力。
5. `agent` 生成 `agent_plans` 和 `agent_plan_steps`，标注风险等级、确认策略、影响范围和幂等键。
6. `PolicyEngine` 将计划决策为 `allow`、`prompt` 或 `forbidden`；`prompt` 操作要求用户确认。
7. 用户确认后，`AgentExecutor` 只能调用已注册能力和既有 service 接口，不直接写数据库。
8. Agent 生成的日报、周报、热点分析、主动网络研究报告和执行结果写入 `messageFeed AI` 内部源。
9. 当上下文达到压缩阈值时，按完整语义块归档历史，保留摘要、索引、窗口编号、压缩前后 token 和原文引用。
10. 高优先级 AI 源条目可由 `notifier` 通过企业微信、`ntfy` 等通道推送。
11. 全过程写入 transcript、审计日志、上下文压缩记录、记忆召回记录、指标和 trace。

### 10.2 主动网络采集链路

主动网络采集采用“任务定义 -> 搜索或抓取 -> 快照保存 -> 正文抽取 -> 去重评估 -> 条目或 AI 源报告”的链路：

1. 用户或 Agent 创建 `web_acquisition_tasks`，任务类型包括 `search`、`monitor` 和 `page_extract`。
2. `acquisition` 通过 `SearchProvider` 获取候选 URL，或通过 `WebAcquisitionProvider` 直接抓取目标页面。
3. `PageExtractor` 抽取标题、正文、发布时间和链接。
4. `web_snapshots` 保存 URL、正文 hash、HTTP 状态、抽取方法、抓取时间和失败原因。
5. 系统按 `content_hash`、URL 和来源评估结果去重。
6. 重要变化生成普通条目或 `messageFeed AI` 源报告。
7. 搜索结果必须先抓取和评估，不能直接作为事实进入摘要或通知。

### 10.3 阅读行为与画像链路

阅读行为采用“事件采集 -> 状态聚合 -> 画像更新 -> 推荐和摘要使用”的链路：

1. Web 在条目曝光、打开详情、阅读进度、点击原文、收藏、隐藏、不感兴趣和减少类似推荐时记录行为事件。
2. `profile` 将行为事件写入 `user_item_interaction_events`，并聚合到 `user_item_states`。
3. `InterestProfileBuilder` 基于多次行为证据更新短期兴趣、长期偏好和负反馈标签。
4. 用户可以在偏好页面查看、编辑、删除、固定或清空兴趣标签。
5. `recommender`、`agent`、摘要任务和通知策略读取画像时必须保留可解释依据。

### 10.4 金融市场监控链路

金融监控采用“拉取行情 -> 保存快照 -> 规则评估 -> AI 解读 -> 通知发送”的链路：

1. `scheduler` 根据交易日历和关注列表触发行情轮询。
2. `market` 通过 `MarketDataProvider` 获取最新行情快照。
3. `alert` 使用确定性规则计算是否触发告警。
4. 命中规则后生成 `market_alert_events`，并用 `dedupe_key` 控制重复发送。
5. `llm` 基于行情快照、阈值、相关资讯和风险提示生成短文本。
6. `notifier` 将告警发送到微信、ntfy 等通道。

该链路不包含自动下单，也不把 AI 输出作为交易建议。

### 10.5 Web 阅读与推荐 Feed 链路

Web 阅读采用“用户选择模式 -> 查询对应 Feed -> 展示条目 -> 记录反馈”的链路：

1. Web 界面提供时间线模式和推荐 Feed 模式切换，并通过 `feed_view_preferences` 保存最近选择。
2. 时间线模式调用 `/api/v1/feed/timeline`，以用户已订阅来源为主，按 `published_at desc` 排序，缺失发布时间时使用 `fetched_at desc` 兜底。
3. 推荐 Feed 模式由 `recommender` 构建候选池，候选包括已订阅来源条目、推荐源目录条目、健康候选源条目和经过标注的桥接来源条目。
4. `recommender` 根据兴趣规则、阅读反馈、来源权重、内容新鲜度、来源健康状态和重大事件上下文计算排序。
5. 推荐结果写入 `feed_recommendations`，并保存推荐原因、分数、推荐批次和来源订阅状态。
6. Web 界面对未订阅来源必须展示“未订阅来源”标记、来源出处、健康状态和一键订阅入口。
7. 用户的隐藏、不感兴趣、减少类似推荐、订阅来源等操作写入 `recommendation_feedback`，后续推荐排序必须读取该反馈。

推荐 Feed 不能把阅读未订阅内容解释为自动订阅，也不能隐藏未订阅来源身份。

### 10.6 自然语言设置控制链路

自然语言设置控制是 AI Agent 链路中的设置变更子集，采用“用户指令 -> 意图解析 -> 变更计划 -> 校验确认 -> 受控执行 -> 审计记录”的链路：

1. `handler` 接收用户自然语言指令，并记录 `control_commands`。
2. `control` 调用 `llm` 生成结构化意图，识别目标对象、操作类型、风险等级和歧义点。
3. 若指令存在歧义或匹配多个对象，`control` 返回澄清问题，不执行变更。
4. `control` 生成 `control_change_plans` 和 `control_change_steps`，展示变更前后差异、影响范围和回滚方式。
5. `control` 或 `agent` 根据策略判断执行模式：仅建议、需要确认或委托执行。
6. 用户确认后，`ControlExecutor` 或 `AgentExecutor` 调用订阅、目录、通知、摘要、告警、金融等 service 接口执行变更。
7. 执行过程写入 `control_audit_logs`，失败步骤保留错误原因和可重试信息。

该链路只控制系统设置，不提供通用远程命令执行、浏览器自动化或未授权外部工具调用能力。

## 11. 前后端架构与多端支持

### 11.1 架构选型

采用**前后端分离 + 统一 API** 架构，支持多端扩展：

```
                 ┌─────────────────────────┐
                 │   Go API Server (后端)   │
                 │  RESTful API + OpenAPI   │
                 │  /api/v1/sources, items  │
                 └───────────┬─────────────┘
                             │
              ┌──────────────┼──────────────┐
              ↓              ↓              ↓
        ┌──────────┐   ┌─────────┐   ┌──────────┐
        │ Web 前端  │   │ iOS App │   │ Android  │
        │ (Vue 3)  │   │(SwiftUI)│   │(Compose) │
        └──────────┘   └─────────┘   └──────────┘
```

### 11.2 技术栈

**后端（Go）**：
- 框架：Gin（RESTful API）
- API 规范：OpenAPI 3.0
- 文档：Swagger UI 自动生成
- 特性：CORS 支持、JWT 认证、请求日志

**Web 前端（Vue 3）**：
- 构建工具：Vite 8
- UI 框架：Arco Design Vue（字节跳动开源，现代化设计）
- 状态管理：Pinia
- HTTP 客户端：Axios（基于 OpenAPI 自动生成）
- 路由：Vue Router 4
- TypeScript：完整类型支持

**移动端（后续）**：
- iOS：Swift + SwiftUI + Alamofire
- Android：Kotlin + Jetpack Compose + Retrofit
- API 客户端：OpenAPI Generator 自动生成

### 11.3 API 设计原则

- RESTful 风格，统一 `/api/v1` 前缀
- 返回结构：`{code, message, data, request_id}`
- 分页参数：`limit`, `offset`, `total`
- 排序参数：`sort_by`, `order`（asc/desc）
- 错误码：标准 HTTP 状态码 + 业务错误码
- 响应头：`X-Request-ID` 用于追踪

### 11.4 部署架构

**开发环境**：
```
messageFeed-make cloudflare
  ├── Cloudflare Tunnel (aroen.eu.cc -> gateway-dev:8443)
  ├── Caddy gateway-dev (127.0.0.1:8443 on host, gateway-dev:8443 in Docker network)
  ├── Go API dev (Docker network only, api-dev:60001)
  ├── Web dev (Docker network only, web-dev:5173, Vite HMR)
  ├── PostgreSQL (127.0.0.1:5432 on host)
  └── Migrate one-shot service (golang-migrate)
```

**生产环境**：
```
Cloudflare Tunnel or Load Balancer
  ↓
Caddy or nginx (统一入口)
  ├── /api/* → Go API
  └── /* → Vue 静态资源
```

### 11.5 跨域与认证

- 开发环境：浏览器访问 `https://localhost:8443` 或 `https://aroen.eu.cc`，Caddy 将 `/api`、`/healthz`、`/readyz`、`/metrics` 转发到 Go API，将页面请求转发到 Vite dev server。
- 生产环境：Caddy 或 Nginx 反向代理统一入口，同域部署，无跨域问题
- 认证：JWT Token 存储在 `localStorage`
- 请求头：`Authorization: Bearer <token>`

## 12. 运维与监控
