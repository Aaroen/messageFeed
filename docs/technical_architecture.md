# Signal Feed 技术架构文档

## 1. 架构原则

项目采用单体模块化架构，不在初期拆分微服务。核心原因是当前运行环境为 WSL 本机，主要目标是完成业务闭环、数据建模、异步任务、AI 调用和通知链路。

部署策略采用“当前本地单节点，接口预留分布式升级”的方式。第一阶段只要求 `docker-compose` 本地部署和 Tailscale 远程访问；代码结构、配置和任务执行模型需要避免绑定单机假设，以便后续接入 Cloudflare Tunnel、Cloudflare Load Balancer 和多节点运行。

服务内部遵循 `handler -> service -> repository` 分层。`*gin.Context` 仅保留在 handler 层，业务层统一使用 `context.Context`。

## 2. 技术选型

| 职责 | 选型 |
| --- | --- |
| HTTP 框架 | Gin |
| 数据库 | PostgreSQL |
| ORM | GORM |
| 迁移 | 正式 SQL 迁移文件，后续可接入 `golang-migrate/migrate` |
| RSS 解析 | gofeed |
| 定时任务 | gocron |
| 金融行情 | 自定义 `MarketDataProvider` 接口，按市场接入 Yahoo Finance、Finnhub、Polygon、Tushare、AkShare 等 |
| 技术指标 | MVP 先手写简单指标，后续评估 `go-talib` |
| AI 调用 | 自定义 `LLMClient` 接口，兼容 OpenAI API、Ollama 和 OpenAI-compatible 服务 |
| 通知 | ntfy、企业微信机器人、企业微信自建应用、微信公众号 |
| 日志 | log/slog |
| 指标 | prometheus/client_golang |
| 契约 | OpenAPI |
| 当前部署 | docker-compose + Tailscale |
| 后续入口 | Cloudflare Tunnel 或 Cloudflare Load Balancer |

## 3. 目录结构

```text
pro01_signal_feed/
├── cmd/api/main.go
├── internal/config/
├── internal/domain/
├── internal/repository/
├── internal/service/
├── internal/handler/
├── internal/catalog/
├── internal/importer/
├── internal/fetcher/
├── internal/scheduler/
├── internal/market/
├── internal/alert/
├── internal/llm/
├── internal/notifier/
├── internal/runtime/
├── api/
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
| `handler` | Gin 路由、参数绑定、响应渲染和错误映射 |
| `catalog` | 管理推荐源目录、源分类、源健康状态和源搜索 |
| `importer` | 处理 OPML、URL 批量导入和目录批量订阅 |
| `fetcher` | 抓取 RSS、Atom、JSON Feed，并规范化条目 |
| `scheduler` | 编排周期抓取、日报生成、失败重试和通知任务 |
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
| `user_item_states` | 用户阅读状态，包含已读、收藏、隐藏 |
| `interest_rules` | 兴趣规则，包含关键词、标签、权重、启用状态和匹配范围 |
| `summaries` | AI 摘要，包含日报、专题摘要、重大事件摘要和模型调用信息 |
| `notification_channels` | 通知通道，包含通道类型、启用状态和配置引用 |
| `notification_recipients` | 通知接收目标，包含微信 openid、企业微信 user_id、群机器人目标或 ntfy topic |
| `notifications` | 通知记录，包含触发原因、通道、接收目标、状态和失败原因 |
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
- `source_catalog_entries(source_origin, source_key)`
- `market_instruments(market, symbol)`
- `market_quotes(instrument_id, provider, quote_time)`
- `market_alert_events(rule_id, dedupe_key)`

分布式升级预留：

- `notifications` 应增加业务幂等键，例如 `dedupe_key`，避免日报或重大事件重复推送。
- 自动抓取、日报和推送任务应通过 `TaskLocker` 接口执行互斥。第一阶段可以使用 PostgreSQL advisory lock 或单节点实现，后续多节点部署时切换为共享数据库锁。
- 配置层预留 `APP_NODE_ID`、`DEPLOYMENT_MODE`、`PUBLIC_BASE_URL`、`BIND_ADDR` 和 `TRUSTED_PROXY_CIDRS`。

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

AI Agent 不直接参与行情拉取和阈值判断。确定性规则命中后，服务将行情快照、规则、近期相关资讯和必要上下文交给 `LLMClient` 生成解释性文本，再由 `notifier` 发送。

## 6. API 草案

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/healthz` | 存活检查 |
| `GET` | `/readyz` | 依赖就绪检查 |
| `GET` | `/metrics` | Prometheus 指标 |
| `GET` | `/api/runtime/node` | 查询当前节点标识、部署模式和公开访问基址 |
| `POST` | `/api/market/instruments` | 新增金融标的 |
| `GET` | `/api/market/instruments` | 查询金融标的 |
| `POST` | `/api/market/watchlists` | 新增关注标的 |
| `GET` | `/api/market/watchlists` | 查询关注列表 |
| `GET` | `/api/market/quotes` | 查询最新行情快照 |
| `POST` | `/api/market/alert-rules` | 新增金融告警规则 |
| `GET` | `/api/market/alert-rules` | 查询金融告警规则 |
| `GET` | `/api/market/alert-events` | 查询金融告警事件 |
| `POST` | `/api/market/alert-rules/{id}/test` | 使用最近行情测试规则与通知 |
| `POST` | `/api/sources` | 新增订阅源 |
| `GET` | `/api/sources` | 查询订阅源 |
| `PATCH` | `/api/sources/{id}` | 更新订阅源 |
| `POST` | `/api/sources/{id}/fetch` | 手动抓取 |
| `GET` | `/api/source-catalogs` | 查询推荐源 |
| `GET` | `/api/source-catalogs/search` | 搜索推荐源 |
| `POST` | `/api/sources/import/opml` | OPML 导入 |
| `POST` | `/api/sources/import/catalog` | 从推荐源批量订阅 |
| `GET` | `/api/items` | 查询 Feed |
| `GET` | `/api/items/{id}` | 查询条目详情 |
| `POST` | `/api/items/{id}/read` | 标记已读 |
| `POST` | `/api/items/{id}/favorite` | 标记收藏 |
| `POST` | `/api/summaries/daily` | 手动触发日报 |
| `GET` | `/api/summaries` | 查询摘要 |
| `POST` | `/api/notification-channels` | 新增通知通道 |
| `GET` | `/api/notification-channels` | 查询通知通道 |
| `POST` | `/api/notification-channels/{id}/test` | 测试通知 |

## 7. 外部参考边界

- `OpenClaw`：参考插件化通道、WeCom/Weixin 通道目录和 Agent 工具生态，不直接移植其运行时。
- `Hermes Agent`：参考 WeCom、Weixin、home channel、定时任务和消息网关抽象。
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
- 日志统一使用 `log/slog`。
- 密钥不进入源码、测试数据和镜像层。
- 外部依赖测试优先使用真实依赖或 testcontainers。
- 第一阶段即暴露健康检查、就绪检查和指标端点。
- 金融行情和 AI 解读必须保留“不构成投资建议”的产品边界。
- 行情数据必须记录 provider 和 quote_time，避免用户误判实时性。

## 9. 部署设计

### 当前阶段

当前阶段采用本地单节点部署：

- 服务运行在 WSL 本机。
- PostgreSQL 由本地 `docker-compose` 提供。
- API 监听地址通过 `BIND_ADDR` 配置，默认可以是 `127.0.0.1:8080`；需要 Tailscale 访问时可改为 `0.0.0.0:8080` 或绑定 Tailscale IP。
- 远程设备通过 Tailscale 地址访问服务。
- `/readyz` 检查数据库连接、迁移状态和必要配置，不检查非关键外部服务。

### 后续升级路径

后续分布式部署不改变业务模块，只替换入口层和运行时配置：

- Cloudflare Tunnel：用于将某台或多台私有网络机器暴露到域名。
- Cloudflare Load Balancer：用于按 `/readyz` 将流量打到健康节点。
- 多节点 API：所有节点共享 PostgreSQL，API 层保持无状态。
- 定时任务：多个节点可以启动 scheduler，但任务执行前必须通过共享任务锁。
- 通知发送：通过 `dedupe_key` 与发送记录避免重复推送。

### 金融市场监控链路

金融监控采用“拉取行情 -> 保存快照 -> 规则评估 -> AI 解读 -> 通知发送”的链路：

1. `scheduler` 根据交易日历和关注列表触发行情轮询。
2. `market` 通过 `MarketDataProvider` 获取最新行情快照。
3. `alert` 使用确定性规则计算是否触发告警。
4. 命中规则后生成 `market_alert_events`，并用 `dedupe_key` 控制重复发送。
5. `llm` 基于行情快照、阈值、相关资讯和风险提示生成短文本。
6. `notifier` 将告警发送到微信、ntfy 等通道。

该链路不包含自动下单，也不把 AI 输出作为交易建议。
