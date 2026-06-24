# 已实现内容归档

**归档日期**：2026-06-24

本文归档 `docs/implementation.md` 中已经实现或已经形成稳定基线的内容。主实施文档只保留后续待执行事项。

## 生产与开发基线

- 已形成生产结构：单一 HTTPS 入口、生产静态 Web 服务、Go API、Cloudflare Tunnel 域名访问。
- 本机入口为 `https://localhost:8443`，生产域名为 `https://aroen.eu.cc`。
- 生产 Web 由 Caddy 服务 `web/dist`，开发态 Vite 仅通过 `messageFeed-make` 显式启动。
- 已提供 `messageFeed-start`、`messageFeed-make`、`messageFeed-status` 等系统命令。
- Docker Compose 已包含生产服务、开发 profile、Cloudflare Tunnel profile、Caddy 网关和观测组件。

## 阶段一：基础设施搭建

已完成：

- Go 模块、目录结构、`.gitignore`、配置模块。
- HTTP 基础服务：`/`、`/healthz`、`/readyz`、`/metrics`、`/api/runtime/node`。
- PostgreSQL 连接池、健康检查、迁移框架和 Docker Compose 迁移服务。
- 结构化日志、基础指标、Makefile、Dockerfile、生产/开发 Compose 服务。
- Caddy 统一入口、Cloudflare Tunnel、systemd 开机自启和部署脚本。
- 基础文档：需求、架构、实施文档。

## 阶段二：订阅源与 Feed 闭环

已完成：

- Gin 路由迁移、`/api/v1` 路由组、中间件、统一响应和错误映射。
- `sources`、`items`、`user_item_states`、`feed_view_preferences` 等表和迁移。
- source、item、user item state、feed view preference 的 domain、repository、service 和主要测试。
- RSS/Atom 抓取、URL 规范化、内容 hash、重复抓取去重。
- 订阅源、条目列表、条目详情、时间线、阅读状态、阅读模式偏好的 API。
- Vue 3 + Vite + Arco Design Vue 前端基础工程。
- 订阅源管理、源目录搜索、URL/OPML 导入、手动抓取、时间线列表、推荐页原型、条目详情阅读器。
- 最小 OpenAPI 契约。

仍未完成的阶段二事项保留在主实施文档。

## 阶段三：日志、错误追踪与链路观测

已完成：

- `internal/observability` 统一初始化 logger、request id、trace id、span id 和 tracer provider。
- JSON 结构化日志，固定 `service`、`environment`、`node_id`、`request_id`、`trace_id`、`span_id` 等字段。
- Gin request id、CORS、Recovery、访问日志和 OpenTelemetry middleware。
- repository、service、fetcher 的主要 span 和基础指标。
- Prometheus、Grafana、Loki、Promtail、Tempo、OpenTelemetry Collector 的 Compose 配置。
- Grafana Dashboard 草案和日志/指标/trace 保留策略。

仍未完成的阶段三事项保留在主实施文档。

## 阶段四：源目录与导入

已完成：

- `source_catalog_entries` 和 `source_import_jobs` 表。
- 官方源目录迁移，覆盖 AI、开发、云、数据库、财经、新闻和中文来源等分类。
- 源目录 domain、repository、service 和 handler。
- `/api/v1/source-catalogs`、目录导入、URL 导入、OPML 导入和导入任务查询接口。
- OPML 导入大小限制、递归 outline 解析、URL 批量导入复用。
- 前端源目录搜索、目录启停、URL 导入、OPML 导入和启用后抓取。
- OpenAPI 已覆盖源目录、导入和导入任务查询接口。

仍未完成的阶段四事项保留在主实施文档。

## 阶段四点五：后端治理与后台刷新解耦

已完成：

- `sources` 增加 `next_fetch_at`、`etag`、`last_modified`、`fetch_priority`。
- `source_fetch_jobs`、`source_fetch_attempts`、`item_events` 基础表。
- 抓取任务、抓取尝试、item event 的 domain、repository 和模型转换测试。
- `SourceSyncService` 支持扫描到期来源、创建 queued 抓取任务、领取并执行任务、记录 attempt 和重试。
- 批量刷新入队接口和后台抓取状态轮询。
- 前端批量刷新改为后台任务模式。
- `task_locks` 迁移和租约式任务锁。
- `alert_rules`、`alert_candidates`、`ai_analysis_jobs`、`notification_jobs`、`notification_deliveries` 基础表。
- `AlertRuleService`、`ItemEventWorkerService`、`AlertPolicyEngine`。
- `AIFeedService` 可确保 `messageFeed AI` 内部源存在，并写入今日摘要、提醒解释、来源健康报告和 Agent 执行报告。

## 阶段五已完成基础

已完成：

- 推荐 Feed 原型接口和 Web 推荐入口。
- 企业微信自建应用接收消息回调入口、验签解密、消息标准化和 `MsgId` 幂等入库。
- 最小用户系统：`users`、`user_sessions`、`auth_oauth_states`、`external_accounts`、`agent_approvals`。
- 对话 MVP 表结构：`external_accounts`、`agent_inbound_messages`、`agent_sessions`、`agent_turns`、`agent_transcript_entries`、`agent_audit_logs`。
- 只读 Agent Runner、企业微信自建应用 `message/send` 异步回复 worker。
- P0 `agent` 模块、能力注册、只读策略和基础审计。
- 系统提示词已抽象到独立文件。
- 企业微信普通对话不会写入 AI 源、订阅主页或推荐主页。

下一步阶段五不继续沿用旧单 Agent 设计，改为 `ControllerAgent` 唯一主控和一次性 `ExecutorAgentRun` 的运行时模型。新的落地计划见 `docs/nowdoit/agent-controller-executor-implementation-plan.md`。
