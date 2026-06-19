# messageFeed 金融分析 Agent 专项计划

**最后更新**：2026-06-19

## 1. 目标与边界

金融分析 Agent 是 `messageFeed AI Agent` 的专项能力，用于把金融行情、财经资讯、主动网络采集和 AI 分析联动起来，为用户提供关注标的监控、规则告警、市场事件解释和通知推送。

该能力只做信息监控、分析和提醒，不提供确定性买卖建议，不接入自动交易，不接入券商账户，不承诺收益，也不承担高频行情或低延迟交易职责。所有金融分析内容必须明确标注“不构成投资建议”。

## 2. 核心场景

第一阶段优先覆盖：

1. 用户添加指数、股票、ETF 等关注标的。
2. 用户配置涨跌幅、价格穿越、成交量异常等确定性规则。
3. 系统定时拉取行情快照并保存。
4. 规则触发后，Agent 汇总行情数据、近期财经资讯、政策公告、市场评论和主动网络采集结果。
5. AI 生成简短解释，写入 `messageFeed AI` 源。
6. 高优先级事件通过企业微信或 ntfy 推送。
7. 通知记录、告警事件、AI 分析依据和幂等键可追踪。

示例自然语言请求：

```text
帮我监控沪深 300，当日涨跌幅绝对值超过 2% 时提醒我。
纳指大跌时，把相关新闻和可能原因整理给我。
最近金融提醒太频繁，把冷却时间调整到 6 小时。
每天收盘后给我生成一份 A 股和美股主要指数摘要。
```

## 3. 能力分层

金融分析 Agent 应分为四层。

### 3.1 行情数据层

职责：

- 管理金融标的。
- 管理行情数据源。
- 拉取或订阅行情快照。
- 记录价格、前收价、涨跌幅、成交量、行情时间、数据源和延迟属性。

候选数据源：

- A 股与中文市场：Tushare、AkShare HTTP 化服务、东方财富、新浪财经。
- 全球指数与 ETF：Yahoo Finance、Alpha Vantage、Finnhub、Polygon 或其他 OpenAPI 兼容行情服务。

所有 provider 必须记录：

- 覆盖市场。
- 授权状态。
- 是否实时。
- 典型延迟。
- 速率限制。
- 字段完整性。
- 最近健康状态。
- 失败重试策略。

### 3.2 确定性告警层

职责：

- 读取行情快照。
- 评估用户配置的规则。
- 生成告警事件。
- 使用冷却时间和幂等键避免重复触发。

MVP 规则类型：

```text
day_change_pct_abs_gte
day_change_pct_gte
day_change_pct_lte
price_cross_above
price_cross_below
volume_ratio_gte
```

AI 不参与是否触发告警的基础判断。AI 只在确定性规则已经触发后参与解释和摘要。

### 3.3 资讯与主动采集层

职责：

- 从已订阅财经来源中检索相关内容。
- 从推荐源目录和主动网络采集任务中补充无 RSS 信息。
- 对政策公告、监管公告、央行新闻、公司公告和市场评论进行网页监控。
- 在行情异常时按关键词执行一次主动网络研究。

采集来源示例：

- 央行、证监会、交易所、财政部、统计局等官方公告。
- SEC、Federal Reserve、ECB、IMF、World Bank 等国际机构。
- 主流财经媒体 RSS 或网页。
- 用户指定的财经观察页面。
- 行情相关公司、ETF、指数发布页面。

搜索结果不能直接作为事实依据，必须先进入抓取、去重、来源评估和引用记录流程。

### 3.4 AI 分析与通知层

职责：

- 汇总行情快照、规则、相关资讯、主动采集结果和用户偏好。
- 生成市场波动解释。
- 写入 `messageFeed AI` 源。
- 通过企业微信、ntfy 等通道发送高优先级提醒。
- 保存模型、提示词版本、token、耗时、输入来源和推送状态。

AI 输出必须区分：

- 事实数据：行情价格、涨跌幅、成交量、公告标题、发布时间、来源 URL。
- 模型推断：可能背景、影响路径、后续关注点。
- 风险提示：不构成投资建议。

## 4. 数据模型建议

### 4.1 金融标的

```text
market_instruments
- id
- symbol
- market
- exchange
- name
- instrument_type
- currency
- status
- created_at
- updated_at
```

唯一约束：

```text
market_instruments(market, symbol)
```

### 4.2 行情数据源

```text
market_data_providers
- id
- provider
- covered_markets
- authorization_status
- realtime_level
- typical_delay_seconds
- rate_limit
- status
- last_checked_at
- last_error
```

### 4.3 行情快照

```text
market_quotes
- id
- instrument_id
- provider
- price
- previous_close
- day_change
- day_change_pct
- volume
- quote_time
- fetched_at
- raw_payload_hash
```

唯一约束：

```text
market_quotes(instrument_id, provider, quote_time)
```

### 4.4 关注列表

```text
market_watchlists
- id
- user_id
- instrument_id
- status
- display_order
- created_at
- updated_at
```

### 4.5 告警规则

```text
market_alert_rules
- id
- user_id
- instrument_id
- rule_type
- threshold_value
- comparison_window
- cooldown_seconds
- notification_policy
- status
- created_at
- updated_at
```

### 4.6 告警事件

```text
market_alert_events
- id
- rule_id
- instrument_id
- quote_id
- event_time
- trigger_value
- threshold_value
- dedupe_key
- cooldown_until
- ai_item_id
- notification_status
- created_at
```

唯一约束：

```text
market_alert_events(rule_id, dedupe_key)
```

### 4.7 AI 金融分析元数据

金融分析正文写入 `messageFeed AI` 源对应的 `items`，元数据写入扩展表：

```text
ai_financial_analysis
- item_id
- analysis_type
- instrument_ids
- quote_ids
- alert_event_id
- input_item_ids
- input_snapshot_ids
- model
- prompt_version
- token_input
- token_output
- cost_estimate
- confidence
- risk_disclaimer_included
- created_at
```

## 5. 服务与接口抽象

### 5.1 行情 Provider

```text
MarketDataProvider
- Quote(ctx, instrument) -> MarketQuote
- BatchQuotes(ctx, instruments) -> []MarketQuote
- ProviderStatus(ctx) -> MarketProviderStatus
- Capabilities() -> ProviderCapabilities
```

### 5.2 告警引擎

```text
MarketAlertEngine
- Evaluate(ctx, quote, rules) -> []MarketAlertEvent
- BuildDedupeKey(rule, quote) -> string
```

### 5.3 金融 Agent 编排

```text
FinancialAgent
- BuildMarketBrief(ctx, input) -> AIItem
- ExplainAlert(ctx, event) -> AIItem
- BuildDailyMarketDigest(ctx, watchlist) -> AIItem
- FindRelatedNews(ctx, instrument, timeWindow) -> []RelatedItem
```

### 5.4 通知编排

```text
FinancialNotificationService
- ShouldNotify(ctx, event) -> bool
- BuildNotification(ctx, analysisItem) -> Notification
- Send(ctx, notification) -> NotificationResult
```

## 6. API 草案

业务 API 继续使用 `/api/v1` 前缀。

```text
POST /api/v1/market/instruments
GET  /api/v1/market/instruments
POST /api/v1/market/watchlists
GET  /api/v1/market/watchlists
GET  /api/v1/market/quotes
POST /api/v1/market/alert-rules
GET  /api/v1/market/alert-rules
PATCH /api/v1/market/alert-rules/{id}
GET  /api/v1/market/alert-events
POST /api/v1/market/alert-rules/{id}/test
POST /api/v1/agent/financial/briefs
GET  /api/v1/agent/financial/analyses
```

自然语言入口仍归属 Agent：

```text
POST /api/v1/agent/commands
```

示例命令：

```json
{
  "command": "帮我监控沪深 300，当日涨跌幅绝对值超过 2% 时通过企业微信提醒"
}
```

## 7. Web 产品形态

### 7.1 金融关注页

展示：

- 关注标的。
- 最新价格。
- 当日涨跌幅。
- 数据源。
- 更新时间。
- 告警规则状态。
- 最近告警事件。

### 7.2 金融告警规则页

支持：

- 新增规则。
- 启用和停用规则。
- 调整阈值。
- 调整冷却时间。
- 调整通知通道。
- 测试规则。

### 7.3 AI 源中的金融分析

`messageFeed AI` 源中展示：

- 市场日报。
- 规则触发解释。
- 指数波动分析。
- 相关资讯聚合。
- 后续关注点。
- 风险提示。

每条金融分析应提供“查看依据”入口，展示行情快照、触发规则、相关资讯和主动采集来源。

## 8. 自然语言控制能力

金融相关能力必须注册到 Agent 能力清单。

低风险能力：

- 查询关注列表。
- 生成市场摘要。
- 查询最近行情。
- 生成告警规则建议。

中风险能力：

- 新增关注标的。
- 创建低频告警规则。
- 调整非通知类展示偏好。

高风险能力：

- 提高通知频率。
- 修改通知接收人。
- 创建高频告警规则。
- 批量启用多个告警。

默认需要确认的命令：

```text
新增或修改通知目标。
提高通知频率。
创建金融告警。
批量修改金融规则。
删除或停用大量规则。
```

## 9. 实施阶段

### 9.1 阶段 A：金融数据基础

目标：

- 建立金融标的、关注列表、行情 provider 和行情快照。

验收：

- 可以新增一个指数标的。
- 可以拉取一次行情快照。
- 行情数据保存到 PostgreSQL。
- `/readyz` 不依赖行情源。

### 9.2 阶段 B：确定性告警

目标：

- 建立告警规则和告警事件。

验收：

- 可以配置涨跌幅绝对值阈值规则。
- 规则命中后生成告警事件。
- 冷却时间和幂等键生效。

### 9.3 阶段 C：AI 分析入源

目标：

- 告警事件触发后生成 AI 分析，并写入 `messageFeed AI` 源。

验收：

- 分析内容包含行情数据、触发规则、相关资讯和风险提示。
- 元数据记录模型、提示词版本、token 和输入来源。

### 9.4 阶段 D：通知推送

目标：

- 通过企业微信或 ntfy 推送金融告警。

验收：

- 告警通知包含标的、价格、涨跌幅、阈值、数据源、AI 简述和风险提示。
- 同一规则在冷却期内不重复发送。
- 失败通知可查询和重试。

### 9.5 阶段 E：主动网络研究联动

目标：

- 在行情异常时主动采集相关财经资讯和官方公告。

验收：

- Agent 可以围绕触发标的和关键词进行一次网络研究。
- 研究结果写入金融分析依据。
- 搜索结果经过抓取、去重和来源评估。

## 10. 合规与风险控制

必须遵守：

- 所有金融分析必须包含“不构成投资建议”。
- AI 不输出确定性买入、卖出、加仓、减仓建议。
- AI 不参与基础告警触发判断。
- 不接入自动交易、券商账户和下单能力。
- 非官方或网页逆向数据源必须标注授权状态、稳定性和延迟属性。
- 高优先级通知必须具备冷却时间和幂等键。
- 金融提示不得承诺收益。
- 用户修改通知目标、提高通知频率和新增高频规则必须确认。

## 11. 最小验收闭环

```text
用户输入：
“帮我监控沪深 300，当日涨跌幅绝对值超过 2% 时提醒我。”

系统生成计划：
- 新增或确认沪深 300 标的。
- 创建 day_change_pct_abs_gte 规则，阈值为 2%。
- 设置冷却时间。
- 设置企业微信或 ntfy 通知策略。
- 确认是否启用 AI 解释。

用户确认后：
- 保存关注标的和规则。
- 定时拉取行情。
- 规则命中后生成告警事件。
- Agent 汇总行情和相关资讯。
- AI 分析写入 messageFeed AI。
- 通知通道发送提醒。
```

该闭环完成后，金融分析 Agent 即具备从关注、行情、规则、分析到通知的最小业务能力。
