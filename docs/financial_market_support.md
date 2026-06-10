# 金融市场支持调研与扩展方案

## 1. 目标

本方案为 Signal Feed 增加金融和股市相关的专业支持。目标不是构建自动交易系统，而是在现有信息聚合、AI 摘要和微信通知能力之上，扩展轻量级行情监控与告警：

- 用户可以关注指数、股票、ETF 等金融标的。
- 系统按交易时间拉取行情快照。
- 当当日涨跌幅、价格、成交量或技术指标超过阈值时，生成告警事件。
- AI Agent 根据行情快照和相关新闻生成简短解释。
- 告警通过微信、ntfy 等通道发送给用户。

## 2. 成熟方案调研

本次调研优先参考官方文档或主仓库，以避免只依据二手介绍做技术判断。主要来源包括：

- [QuantConnect LEAN](https://github.com/QuantConnect/Lean)
- [AkShare](https://akshare.akfamily.xyz/)
- [Tushare](https://tushare.pro/)
- [Alpha Vantage API](https://www.alphavantage.co/documentation/)
- [Finnhub API](https://finnhub.io/docs/api)
- [Massive/Polygon API Docs](https://massive.com/docs)
- [TradingView Alerts](https://www.tradingview.com/support/solutions/43000520149-introduction-to-tradingview-alerts/)
- [Grafana Alerting](https://grafana.com/docs/grafana/latest/alerting/)
- [Prometheus Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/)
- [go-talib](https://github.com/markcheno/go-talib)

### 2.1 量化和交易引擎

| 方案 | 成熟能力 | 对本项目的启发 | 是否直接引入 |
| --- | --- | --- | --- |
| QuantConnect LEAN | 多资产数据模型、市场日历、回测、实盘交易、算法生命周期 | 学习证券模型、市场日历、数据源抽象和事件驱动设计 | 否，体量过大且偏交易系统 |
| Backtrader、Zipline、vectorbt | 回测、指标、策略评估 | 后续可参考指标与策略测试模型 | 否，MVP 不做回测 |
| AkQuant | 面向量化研究和交易框架 | 可参考研究和回测边界 | 否，非 Go 主线 |

结论：本项目只需要“监控和告警”，不应引入完整量化交易引擎。

### 2.2 财经数据源

| 方案 | 适用市场 | 优点 | 主要限制 | 接入建议 |
| --- | --- | --- | --- | --- |
| Yahoo Finance | 全球股票、指数、ETF | 覆盖广，易用，适合原型 | 非官方接口稳定性和授权需谨慎 | 可作为国际市场 MVP 候选 |
| Alpha Vantage | 美股、外汇、加密、部分指标 | API 清晰，有免费层 | 免费层速率较低 | 适合作为稳定 API 备选 |
| Finnhub | 全球股票、新闻、基本面 | 实时和新闻能力较完整 | 免费层限制明显 | 适合金融新闻联动和美股监控 |
| Polygon | 美股、期权、外汇、加密 | 专业实时数据能力强 | 成本较高 | 后续正式实时需求评估 |
| Tushare | A 股、基金、指数、宏观 | 国内市场覆盖较广 | 积分、权限和频率限制 | A 股正式方案候选 |
| AkShare | A 股、港股、美股、宏观等 | 开源，覆盖丰富，易验证 | Python 生态，部分接口受上游页面变化影响 | 可通过独立 HTTP 服务或命令适配验证 |
| 东方财富、新浪财经 | A 股实时行情常见来源 | 低成本、实时性较好 | 非正式接口稳定性和合规风险 | 只作为个人实验源，需标注风险 |

结论：本项目应通过统一 `MarketDataProvider` 接入多数据源。MVP 可以先接一个低成本数据源，正式方案保留 Tushare、Finnhub、Polygon 等可替换路径。任何非官方或网页逆向来源不得在产品文案中表述为稳定实时数据源，必须在 provider 元数据中记录授权状态、延迟级别和接口维护风险。

### 2.3 告警产品模式

| 方案 | 成熟能力 | 对本项目的启发 |
| --- | --- | --- |
| TradingView Alert | 指标、价格、条件表达式、Webhook、频率控制 | 告警规则、冷却时间、Webhook 通知、模板化消息 |
| Grafana Alerting | 指标查询、规则评估、状态机、通知路由、静默和抑制 | 告警状态、恢复通知、通知策略、告警历史 |
| Prometheus Alertmanager | 分组、去重、静默、路由 | 金融告警也需要 `dedupe_key`、冷却时间和通知路由 |

结论：金融告警不应只是“if 命中就发送”。需要保存规则、状态、冷却时间、触发历史和幂等键。

### 2.4 Go 生态参考

| 方案 | 能力 | 用法建议 |
| --- | --- | --- |
| `piquette/finance-go` | Yahoo Finance Go 客户端 | 可作为 Yahoo Finance 接入参考，但需核查维护状态和接口稳定性 |
| `markcheno/go-talib` | Go 版技术指标库 | 后续增加均线、RSI、MACD 等指标时评估 |
| 自定义 HTTP client | 接入 Finnhub、Alpha Vantage、Polygon、Tushare HTTP API | 更适合本项目的接口驱动和可替换要求 |

## 3. 本项目扩展设计

### 3.1 新增模块

```text
internal/market/
├── provider/
├── quote/
├── calendar/
└── indicator/

internal/alert/
├── market_rules/
├── evaluator/
└── dedupe/
```

`market` 负责行情和金融标的，`alert` 负责告警规则、规则评估、冷却和幂等。

### 3.2 核心接口

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

AI Agent 不负责判断阈值是否命中。阈值判断必须是确定性逻辑，AI 只负责解释、摘要和组织通知文本。

### 3.3 数据模型

| 表 | 说明 |
| --- | --- |
| `market_instruments` | 金融标的，包含 symbol、市场、交易所、名称、类型、币种 |
| `market_data_providers` | 数据源，包含 provider、覆盖市场、速率限制、延迟级别和启用状态 |
| `market_quotes` | 行情快照，包含当前价、前收价、涨跌幅、成交量、行情时间和数据源 |
| `market_watchlists` | 用户关注列表 |
| `market_alert_rules` | 告警规则，包含规则类型、阈值、冷却时间、通知目标 |
| `market_alert_events` | 告警事件，包含触发快照、AI 解读、幂等键、发送状态 |

### 3.4 告警链路

```text
定时任务
  -> 拉取行情快照
  -> 保存 market_quotes
  -> 规则引擎评估 market_alert_rules
  -> 生成 market_alert_events
  -> 关联当天财经资讯和 Feed 内容
  -> 调用 LLMClient 生成 AI 解读
  -> notifier 发送微信或 ntfy
```

通知内容建议包含：

- 标的名称和 symbol。
- 当前价格、涨跌幅、触发阈值。
- 行情时间和数据源。
- 可能相关的资讯摘要。
- AI 解读。
- 风险提示：“该信息仅用于监控与摘要，不构成投资建议。”

## 4. MVP 建议

金融支持 MVP 应只做三件事：

1. 支持手动添加指数或股票标的。
2. 支持定时拉取最新行情并计算当日涨跌幅。
3. 支持涨跌幅阈值命中后生成 AI 解读并发送微信通知。

不进入 MVP 的能力：

- 自动交易。
- 券商账户接入。
- 高频行情。
- 完整回测。
- 复杂技术指标策略。
- 组合收益归因。

## 5. 数据源策略

第一阶段可选一个低成本行情源完成链路验证。若目标是 A 股指数，优先评估 Tushare、AkShare HTTP 化服务、东方财富或新浪财经。若目标是全球指数和 ETF，优先评估 Yahoo Finance、Alpha Vantage 或 Finnhub。

正式实现时，所有 provider 必须声明：

- 覆盖市场。
- 是否实时。
- 典型延迟。
- 速率限制。
- 授权与使用条款。
- 失败重试策略。
- 数据字段完整性。

## 6. 风险与边界

- 行情数据授权风险：免费或非官方接口可能不允许商业使用。
- 实时性风险：部分数据源是延迟行情，通知必须展示行情时间。
- 稳定性风险：网页逆向接口可能随时变化。
- 合规风险：AI 输出不得表达确定性买卖建议。
- 成本风险：实时数据和高频请求通常需要付费。
- 噪声风险：阈值过低会导致频繁通知，必须支持冷却时间和重复抑制。

## 7. 融入现有项目路线

金融市场支持应作为 RSS、摘要和通知链路稳定后的独立阶段接入。它复用现有能力：

- 复用 `scheduler` 执行行情轮询。
- 复用 `LLMClient` 生成解释。
- 复用 `notifier` 发送微信和 ntfy。
- 复用 `notifications` 保存发送状态。
- 复用 Feed 内容为 AI 解读提供新闻背景。

新增的独立能力是金融标的、行情源、行情快照和金融告警规则。

成熟方案向本项目扩展的映射关系如下：

| 成熟方案能力 | 本项目采用方式 |
| --- | --- |
| LEAN 的证券、行情和市场日历边界 | 抽象为 `market_instruments`、`market_quotes`、`MarketDataProvider` 和轻量市场日历，不引入回测与交易生命周期 |
| TradingView 的价格、技术指标和 watchlist 告警 | 抽象为 `market_alert_rules`，先支持涨跌幅、价格穿越和成交量放大，后续扩展指标条件 |
| Grafana Alerting 的规则状态、通知路由和静默机制 | 抽象为告警状态、冷却时间、通知目标和可查询历史 |
| Alertmanager 的分组、去重和路由 | 抽象为 `dedupe_key`、冷却窗口和通道选择，避免多节点或高频波动导致重复通知 |
| AkShare、Tushare、Finnhub、Alpha Vantage、Massive/Polygon 的 API 化行情能力 | 统一封装为 provider，不让业务层直接依赖具体数据源 |
| go-talib 的技术指标能力 | 作为后续指标计算候选，MVP 只保留接口边界和简单指标 |
