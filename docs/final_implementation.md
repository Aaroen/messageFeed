# Signal Feed 最终实施文档

## 1. 实施目标

以 `pro01_signal_feed` 作为 `Go_Pro` 首个完整项目，先完成本地可运行、可部署、可观测、可验收的最小闭环，并通过 Tailscale 提供简单远程访问，再逐步扩展 AI 摘要、微信通知、金融市场监控、多来源采集和分布式部署能力。

分布式部署在第一阶段只做接口预留，不作为交付目标。预留内容包括节点标识、部署模式配置、就绪检查、任务锁接口、通知幂等键和无状态 API 约束。

## 2. 阶段路线

### 阶段一：本地工程基线与 Tailscale 访问

交付目标：

- 建立项目骨架。
- 完成配置加载、结构化日志、统一错误、基础路由。
- 接入 PostgreSQL 与 GORM。
- 提供正式迁移文件。
- 提供 `/healthz`、`/readyz`、`/metrics`。
- 提供最小 `docker-compose` 和 `make verify`。
- 支持通过配置设置 `BIND_ADDR`、`PUBLIC_BASE_URL`、`APP_NODE_ID` 和 `DEPLOYMENT_MODE`。
- 服务默认本地运行，可通过 Tailscale IP 或 MagicDNS 在私有网络内访问。
- 增加 `runtime` 基础接口，用于返回当前节点标识、部署模式和公开访问基址。

验收标准：

- `docker-compose up` 可以启动服务和 PostgreSQL。
- `/healthz` 返回成功。
- `/readyz` 能检查数据库连接。
- Tailscale 网络内的设备可以访问 API。
- `DEPLOYMENT_MODE=local` 时无需 Cloudflare 依赖即可运行。
- `make verify` 可以执行构建和基础测试。

### 阶段二：订阅源与 Feed 闭环

交付目标：

- 实现订阅源 CRUD。
- 实现 RSS 手动抓取。
- 实现条目去重入库。
- 实现 Feed 列表和条目详情。
- 实现已读、收藏、隐藏状态。

验收标准：

- 可以新增 RSS 源。
- 可以手动触发抓取。
- 重复抓取不会重复入库。
- 可以查询 Feed 列表和详情。

### 阶段三：源目录与导入

交付目标：

- 建立 `source_catalog_entries`。
- 导入一批初始推荐源。
- 支持推荐源分类、关键词、语言和热度查询。
- 支持 OPML 导入。
- 支持 URL 批量导入。
- 支持从推荐源目录批量订阅。

验收标准：

- 可以通过 API 搜索推荐源。
- 可以导入 OPML，并返回成功与失败明细。
- 可以从推荐源目录批量创建订阅。
- 失败源不会阻断其他源导入。

### 阶段四：自动化与兴趣规则

交付目标：

- 接入 `gocron`。
- 按源抓取周期执行自动抓取。
- 记录抓取耗时、失败原因、最近抓取时间和下次抓取时间。
- 实现关键词、标签、来源权重和语言评分。
- 增加基础重试策略。

验收标准：

- 启用的源可以按周期自动抓取。
- 失败抓取会记录错误并可后续重试。
- Feed 查询可以按兴趣评分排序或过滤。

### 阶段五：AI 摘要与通知

交付目标：

- 定义并实现 `LLMClient`。
- 实现日报摘要。
- 实现重大事件判断提示词。
- 接入 `ntfy`。
- 接入微信单向通知，优先企业微信机器人或微信公众号测试号。
- 记录模型、token、耗时、通道、接收目标、状态和失败原因。

验收标准：

- 可以手动生成一次日报摘要。
- 可以发送 `ntfy` 测试通知。
- 可以发送微信测试通知。
- 通知失败可被查询并定位原因。

### 阶段六：金融市场监控与 AI 告警

交付目标：

- 增加 `market` 与 `alert` 模块。
- 定义 `MarketDataProvider` 和 `MarketAlertEngine` 接口。
- 支持新增指数、股票、ETF 等金融标的。
- 支持关注列表和金融告警规则。
- 支持定时拉取行情快照并计算当日涨跌幅。
- 支持当日涨跌幅、价格穿越、成交量异常等基础规则。
- 规则命中后调用 AI 生成解释性文本，并通过微信或 ntfy 通知。
- 金融通知必须记录行情快照、规则阈值、AI 解读、幂等键和发送状态。

验收标准：

- 可以新增一个指数标的。
- 可以配置“当日涨跌幅绝对值大于等于 2%”规则。
- 可以拉取行情并保存快照。
- 当规则命中时，可以生成 AI 解读。
- 可以通过微信或 ntfy 发送金融告警。
- 同一规则在冷却时间内不会重复发送。

### 阶段七：工程化增强

交付目标：

- 增加 OpenAPI 契约。
- 增加集成测试。
- 增加关键任务 Prometheus 指标。
- 完善 `docker-compose`，纳入可选 ntfy 和微信回调测试桩。
- 增加 Grafana Dashboard 草案。

验收标准：

- `make verify` 覆盖单元测试、集成测试、构建和契约检查。
- 指标能展示抓取次数、抓取失败、摘要耗时、行情拉取成功率、告警触发次数和通知成功率。

### 阶段八：来源扩展与分布式升级验证

交付目标：

- 接入 RSSHub 作为非标准来源桥接。
- 增加 YouTube 频道订阅支持。
- 扩充推荐源目录。
- 评估 X、网页变化和 Agent 型来源。
- 抽象 `SourceConnector`。
- 验证 Cloudflare Tunnel 或 Cloudflare Load Balancer 入口方案。
- 验证备用节点连接同一 PostgreSQL 后可以承接 API 流量。
- 验证自动任务通过共享任务锁避免重复执行。
- 验证多节点运行时金融告警不会重复发送。

验收标准：

- YouTube 频道 RSS 可以作为普通源订阅。
- RSSHub 路由源可以被记录为桥接源。
- 新来源不破坏原有 RSS 抓取链路。
- 关闭一个 API 节点后，健康入口可以将访问切换到仍可用节点。
- 多节点同时运行 scheduler 时不会重复发送同一份日报。
- 多节点同时运行金融行情轮询时不会重复发送同一条金融告警。

## 3. 优先级裁剪

必须优先完成：

- 工程基线。
- Tailscale 简单远程访问。
- RSS 手动抓取。
- 去重入库。
- Feed 查询。
- OPML 导入。
- 日报摘要。
- 微信单向通知。
- 指数行情监控与阈值告警。

可以延后：

- Cloudflare 入口和多节点故障转移。
- 通用 Agent。
- X 深度接入。
- 浏览器自动化采集。
- 独立向量数据库。
- 多用户权限体系。
- WebPush 和移动端原生推送。
- 自动交易和券商账户接入。
- 完整量化回测系统。

## 4. 参考项目阅读顺序

1. `references/miniflux_v2`、`references/gofeed`、`references/rsshub`：RSS 主链路。
2. `references/rssnext_folo`：源目录、推荐源、订阅体验。
3. `references/hermes_agent`、`references/openclaw`：微信通道与 Agent 消息网关。
4. `references/gocron`、`references/river`、`references/asynq`：调度与异步任务。
5. `references/openai_go`、`references/eino`、`references/eino_ext`：AI 调用和编排。
6. `references/ntfy`、`references/gotify_server`：推送服务。
7. QuantConnect LEAN、AkShare、Tushare、Yahoo Finance、Finnhub、Polygon、Alpha Vantage、TradingView Alert、Grafana Alerting：金融行情、告警和数据源设计参考。

## 5. 关键实现注意事项

- 不直接复制 Folo 的源数据为正式内置数据，先记录为候选来源并核查许可。
- 微信凭证必须通过环境变量或外部配置注入。
- 个人微信桥接仅作实验，不进入第一版验收。
- 抓取任务必须设置超时，避免阻塞调度器。
- 摘要任务必须记录 token、耗时和错误，便于成本分析。
- 通知发送必须幂等，避免日报重复推送。
- API 层不得依赖本机内存保存业务状态，避免后续多节点部署时产生状态不一致。
- 第一阶段的任务锁可以采用单节点实现，但接口必须保留，后续切换为 PostgreSQL advisory lock 或任务表锁。
- 第一阶段不要引入 Redis，后台队列优先后续评估 River。
- 金融行情必须记录数据源、行情时间和延迟属性，不把延迟行情展示为实时行情。
- 金融告警必须具备冷却时间和幂等键，避免行情快速波动时连续刷屏。
- AI 金融解读必须包含风险提示，不输出确定性买卖建议。

## 6. 最小验收命令

```bash
docker-compose up
make verify
```

最终项目应在冷启动后通过 `/healthz` 与 `/readyz`，可以在 Tailscale 网络内访问，并能完成“新增订阅源 -> 抓取 -> 查询 Feed -> 生成摘要 -> 发送通知”和“新增金融标的 -> 拉取行情 -> 规则命中 -> AI 解读 -> 微信通知”的闭环。
