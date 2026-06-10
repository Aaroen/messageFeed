# Signal Feed

Signal Feed 是 `Go_Pro` 下拟落地的第一个完整产品型实战项目，目标是构建一个面向个人使用的信息聚合与 AI 摘要推送系统。

系统从 RSS、YouTube、微信公众号桥接源、网页变化源和后续 Agent 型来源采集内容，经过去重、排序、兴趣匹配和大模型摘要后，通过 Feed 流、微信和其他通知通道向用户呈现。

系统同时规划金融与股市监控能力：用户可以关注指数、股票、ETF 或其他金融标的，当当日涨跌幅、价格、成交量或技术指标触发规则时，系统通过 AI Agent 生成简要解读，并经微信、ntfy 等通道发送通知。

## 项目定位

建议将 `Go_Pro` 的首个落地项目从通用 `blog_api` 调整为 `pro01_signal_feed`。

该项目覆盖以下实战能力：

- RSS、Atom、JSON Feed 抓取与解析
- 推荐源目录与 OPML 导入
- Feed 流、已读、收藏和隐藏状态
- 定时抓取与失败重试
- AI 日报摘要与重大事件判断
- 金融行情监控、阈值告警和 AI 解读
- 微信、ntfy 等通知通道
- PostgreSQL、GORM、迁移和集成测试
- 可观测性、Docker Compose 和 `make verify`

## 文档

- [需求文档](docs/requirements.md)：定义产品目标、MVP 范围、信息源、微信通知、验收标准与风险边界。
- [技术架构文档](docs/technical_architecture.md)：定义技术栈、模块边界、数据模型、接口草案、外部依赖与工程约束。
- [最终实施文档](docs/final_implementation.md)：定义阶段路线、交付物、优先级、验收命令和实施注意事项。
- [金融市场支持调研与扩展方案](docs/financial_market_support.md)：调研成熟行情、告警和量化方案，并定义本项目的金融监控扩展路径。

## 当前结论

第一阶段应聚焦本地可运行闭环：服务运行在 WSL 本机，通过 `docker-compose` 启动，并使用 Tailscale 提供简单远程访问。当前阶段不强制接入 Cloudflare Tunnel、Cloudflare Load Balancer 或多机故障转移。

架构设计需要从一开始保留分布式升级接口，包括无状态 API、`/readyz`、节点标识、部署模式配置、任务锁接口和通知幂等记录。后续可以在不重写业务层的前提下接入 Cloudflare Tunnel、Cloudflare Load Balancer 和多节点部署。

第一阶段不实现通用 Agent、X 深度接入和浏览器自动化采集。微信通知优先采用企业微信机器人、企业微信自建应用或微信公众号测试号等相对稳定路径，个人微信桥接仅作为后续实验性方案。

## 参考项目

已在 `references/` 下准备 RSS、Feed、Agent、推送、调度、搜索和 LLM 相关参考项目。新增重点参考包括：

- `references/openclaw`：参考插件化通道、WeCom/Weixin 通道注册和 Agent 工具生态。
- `references/hermes_agent`：参考 WeCom、Weixin、定时任务、通知目标配置和消息网关抽象。
- `references/rssnext_folo`：参考源发现、推荐源、onboarding feed、订阅组织和 AI 阅读器产品形态。
- `references/miniflux_v2`、`references/gofeed`、`references/rsshub`：参考 RSS 主链路与非标准来源桥接。
- 外部成熟方案：参考 QuantConnect LEAN、AkShare、Tushare、TradingView Alert、Grafana Alerting、Yahoo Finance、Finnhub、Polygon、Alpha Vantage 等方案的职责划分。

## 建议实施顺序

1. 建立本地工程基线：项目骨架、配置、日志、健康检查、指标、数据库、迁移、`docker-compose` 和 Tailscale 访问。
2. 打通 RSS 闭环：订阅源 CRUD、手动抓取、去重入库、Feed 查询。
3. 增加源目录与导入：推荐源、OPML 导入、URL 批量导入。
4. 增加自动化：周期抓取、失败重试、抓取状态和兴趣规则。
5. 增加 AI 与通知：日报摘要、重大事件判断、ntfy 和微信单向通知。
6. 增加金融市场支持：行情源适配、关注标的、阈值规则、AI 解读和微信告警。
7. 完善工程化：OpenAPI、集成测试、Dashboard 和完整 `docker-compose`。
8. 验证分布式升级路径：Cloudflare 入口、备用节点、共享 PostgreSQL、任务锁和健康检查故障转移。

详细阶段拆分见 [最终实施文档](docs/final_implementation.md)。
