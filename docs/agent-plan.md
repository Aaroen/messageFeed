# messageFeed AI Agent 阶段重组计划

**最后更新**：2026-06-19

## 1. 背景与目标

原阶段五到阶段八分别覆盖自动化与推荐、AI 摘要与通知、自然语言设置控制、金融监控。随着产品目标扩展，这些能力不应继续作为彼此割裂的功能线推进，而应重组为统一的 `messageFeed AI Agent` 体系。

重组后的目标是：构建一个基于本项目数据、API、service 层、观测系统和通知通道的受控智能 Agent。该 Agent 可以代理本项目内的智能操作，包括内容推荐、源发现与订阅管理、主动网络采集、摘要生成、提醒推送、金融事件分析、用户偏好建模和设置控制。

该 Agent 的能力边界是“项目内受控智能操作”，不是通用无限工具执行平台。模型不得直接写数据库，不得绕过权限、确认、幂等和审计流程。所有执行动作必须通过已注册能力和既有 service 接口完成。

## 2. 重组后阶段定义

| 新阶段 | 名称 | 目标 | 原阶段映射 |
| --- | --- | --- | --- |
| 阶段五 | Agent 基础设施与 AI 源 | 建立 Agent 能力注册、计划、执行、审计、AI 内部源和风险控制 | 原阶段五、六、七的基础部分 |
| 阶段六 | 主动采集与内容理解 Agent | 支持无 RSS 信息源的网络最新信息获取、网页监控、搜索型采集和内容理解 | 原阶段五、十的一部分 |
| 阶段七 | 推荐、摘要与通知 Agent | 实现个性化推荐、日报、周报、热点事件分析、企业微信和 ntfy 推送 | 原阶段五、六 |
| 阶段八 | 金融与跨领域分析 Agent | 金融行情、资讯、主动网络研究与 AI 分析联动，生成可推送的风险提示 | 原阶段八 |

阶段九继续承担工程化增强，包括 OpenAPI 契约、集成测试、E2E 测试、Dashboard、部署配置和契约校验。阶段十继续承担来源扩展与分布式升级验证。

## 3. 核心概念

### 3.1 项目级 AI Agent

项目级 AI Agent 是 `messageFeed` 的智能编排层。它面向用户自然语言请求、自动化任务和系统事件，负责生成结构化计划、调用受控工具、产出 AI 分析内容并保存审计记录。

典型能力包括：

- 根据用户兴趣推荐内容。
- 根据自然语言请求搜索合适来源并代为订阅。
- 停用低价值来源、调整标签、调整权重和抓取周期。
- 生成日报、周报、专题摘要和热点事件分析。
- 对没有 RSS 的网页进行主动采集、监控和变化分析。
- 通过企业微信、ntfy 等通道发送提醒。
- 监控金融标的并结合资讯解释行情波动。
- 生成 Agent 执行报告和可追溯操作记录。

### 3.2 AI 源

AI 源是系统内置的虚拟订阅源，建议命名为 `messageFeed AI`。它不是外部 RSS，而是 Agent 生成内容的统一展示入口。

AI 源可展示以下内容：

- 每日摘要。
- 每周摘要。
- 热点事件分析。
- 主动网络研究报告。
- 非 RSS 网页变化报告。
- 推荐内容包。
- 金融市场分析。
- 来源健康报告。
- Agent 操作报告。
- 用户偏好更新建议。

AI 源应复用现有阅读系统。推荐实现方式是在 `sources` 中创建特殊来源：

```text
sources
- type = ai_agent
- name = messageFeed AI
- url = internal://messagefeed/ai
- normalized_url = internal://messagefeed/ai
- status = active
```

Agent 生成内容作为普通 `items` 写入：

```text
items
- source_id = messageFeed AI 的 source_id
- title = 今日技术日报
- url = internal://messagefeed/ai/items/{id}
- content_text = AI 生成正文
- content_hash = 用于去重
- published_at = 生成时间
```

必要时新增 `ai_generated_items` 保存生成元数据：

```text
ai_generated_items
- item_id
- generation_type
- prompt_version
- model
- input_window_start
- input_window_end
- input_item_ids
- input_snapshot_ids
- token_input
- token_output
- cost_estimate
- confidence
- risk_level
```

### 3.3 主动网络采集

主动网络采集用于补足没有 RSS、Atom、JSON Feed 或稳定 API 的来源。该能力应定义为“受控网络研究与监控”，而不是无限制浏览器自动化。

适用对象包括：

- 官方网站新闻页。
- 产品更新页和 Changelog。
- 政策公告、监管公告和公司公告。
- 研究机构、实验室和基金会官网。
- 财经事件页面、市场评论和公告页面。
- 搜索引擎新闻结果。
- 指定关键词的最新网络信息。
- 用户指定网页的变化监控。

建议扩展来源类型：

```text
source_type
- rss
- atom
- json_feed
- api
- web_page
- web_search
- web_monitor
- ai_agent
- bridge
```

主动采集分层：

1. 静态网页抓取：HTTP 获取 HTML，提取标题、正文、发布时间和链接。
2. 网页变化监控：保存页面快照、正文 hash、结构 hash，发现变化后生成条目。
3. 搜索型采集：根据主题或 Agent 计划调用搜索服务，获取候选 URL 后再抓取正文。
4. 渲染型采集：对依赖 JavaScript 的页面使用受控 headless browser，后置实现。
5. 登录态或高限制采集：单独设计，不进入早期闭环。

主动采集结果必须保存来源 URL、抓取时间、抽取方式、内容 hash、失败原因和稳定性评价。搜索结果不能直接视为事实，必须经过抓取、去重和来源评估后才能进入分析。

### 3.4 用户行为与偏好模型

Agent 需要理解用户兴趣，但不应做黑箱广告画像。用户画像应服务于推荐、摘要、提醒和设置控制，并且必须可解释、可编辑、可回滚。

阅读行为建议分两层记录：

```text
user_item_states
- user_id
- item_id
- is_read
- read_at
- is_favorite
- favorited_at
- is_hidden
- hidden_at
- first_opened_at
- last_opened_at
- open_count
- max_scroll_ratio
- total_active_dwell_ms
```

```text
user_item_interaction_events
- user_id
- item_id
- event_type
- occurred_at
- source_context
- view_mode
- dwell_ms
- scroll_ratio
- recommendation_id
- metadata
```

优先采集事件：

- `impression`：条目在列表中曝光。
- `open_detail`：打开详情页。
- `read_progress`：阅读进度更新。
- `mark_read` / `mark_unread`：已读状态变化。
- `favorite` / `unfavorite`：收藏状态变化。
- `hide` / `unhide`：隐藏状态变化。
- `open_original`：点击阅读原文。
- `feedback_positive`：明确正反馈。
- `feedback_negative`：明确负反馈。
- `reduce_similar`：减少类似推荐。

停留时间只应统计页面可见、窗口聚焦且用户未切到后台的主动停留时间。单次停留时间应设置上限，避免用户打开页面后离开导致误判。

用户画像建议后期新增：

```text
user_interest_profiles
- user_id
- profile_version
- updated_at
- summary_text
- confidence
```

```text
user_interest_tags
- user_id
- tag
- category
- weight
- source
- confidence
- last_evidence_at
- decay_rate
```

```text
user_interest_evidence
- user_id
- tag
- evidence_type
- item_id
- source_id
- event_id
- score_delta
- created_at
```

画像标签分为显式偏好、隐式偏好、短期兴趣、长期兴趣和负反馈。长期画像不应由模型单次静默修改，应基于多次证据或用户确认。

## 4. Agent 执行框架

Agent 执行流程：

```text
用户自然语言或系统事件
  -> AgentInterpreter 解析意图
  -> AgentPlanner 生成结构化计划
  -> PolicyEngine 做风险与权限校验
  -> 用户确认（必要时）
  -> AgentExecutor 调用已注册能力
  -> service 层执行实际变更
  -> 生成 AI 源条目或通知
  -> 保存审计、指标和 trace
```

核心模块：

```text
AgentCapabilityRegistry
- Register(capability)
- List(userScope)
- Match(intent)
```

```text
AgentInterpreter
- Interpret(command, context) -> AgentIntent
- BuildClarifyingQuestion(ambiguity) -> AgentQuestion
```

```text
AgentPlanner
- BuildPlan(intent) -> AgentPlan
- ValidatePlan(plan) -> PlanValidationResult
- EstimateImpact(plan) -> PlanImpact
```

```text
AgentExecutor
- Execute(approvedPlan) -> AgentExecutionResult
- Rollback(plan) -> AgentRollbackResult
```

```text
AgentAuditLogger
- RecordCommand
- RecordPlan
- RecordApproval
- RecordStepResult
- RecordModelOutput
```

能力注册项至少包含：

```text
agent_capabilities
- capability_key
- target_type
- allowed_actions
- risk_level
- confirmation_policy
- rollback_supported
- service_binding
- enabled
```

风险分级建议：

- `low`：生成推荐、生成摘要、查询源目录、生成订阅建议。
- `medium`：新增订阅、调整标签、调整来源权重、创建低频提醒。
- `high`：批量停用来源、提高通知频率、修改通知接收目标、创建金融告警。
- `critical`：永久删除、暴露敏感配置、绕过访问限制。默认禁止或必须二次确认。

## 5. 阶段五：Agent 基础设施与 AI 源

目标是先建立可控执行底座，而不是直接堆叠具体智能功能。

实施内容：

1. 新增 Agent 核心领域对象：命令、意图、计划、步骤、执行结果、审计日志。
2. 建立 `AgentCapabilityRegistry`，所有 Agent 可执行能力必须注册。
3. 建立 `AgentTool` 抽象，每个工具只能调用既有 service。
4. 建立计划生成、计划校验、影响评估、确认策略和执行器。
5. 为用户创建默认 AI 源 `messageFeed AI`。
6. 将 Agent 生成的日报、报告、执行结果写入 AI 源。
7. 在 Web 中展示 AI 源，与普通来源共用阅读状态、收藏、隐藏和详情页。
8. 接入 observability，记录 request id、trace id、模型调用、执行步骤和错误链。

阶段五验收标准：

- 用户可以提交自然语言命令并得到结构化计划。
- 低风险计划可以生成建议但不必立即执行。
- 中高风险计划必须等待用户确认。
- Agent 可以写入一条 `messageFeed AI` 源内容。
- Agent 执行结果具备审计记录。
- 模型不能直接访问数据库写接口。

## 6. 阶段六：主动采集与内容理解 Agent

目标是支持非 RSS 信息源的主动获取、抽取、去重和分析。

实施内容：

1. 定义 `WebAcquisitionProvider`、`SearchProvider`、`PageExtractor` 和 `SnapshotStore`。
2. 新增 `web_acquisition_tasks`，记录搜索、网页抽取和网页监控任务。
3. 新增 `web_snapshots`，保存 URL、标题、正文、hash、HTTP 状态、抽取方法和抓取时间。
4. 支持静态网页抓取和正文抽取。
5. 支持网页变化监控，发现变化后生成普通条目或 AI 源报告。
6. 支持搜索型采集，先获取候选 URL，再抓取正文和评估来源。
7. 对采集内容建立去重、可信度评估和来源稳定性记录。
8. 将事实来源与模型推断分开保存和展示。

建议数据模型：

```text
web_acquisition_tasks
- user_id
- task_type
- query
- target_url
- schedule
- status
- risk_level
- last_run_at
- next_run_at
```

```text
web_snapshots
- task_id
- url
- title
- content_text
- content_hash
- fetched_at
- http_status
- extraction_method
- failure_reason
```

阶段六验收标准：

- 可以创建一个网页监控任务。
- 可以抓取一个无 RSS 页面并抽取正文。
- 页面变化后可以生成条目或 AI 源报告。
- 可以按关键词执行一次主动网络研究并生成 AI 源报告。
- 所有主动采集结果保留 URL、时间、hash 和抽取方式。

## 7. 阶段七：推荐、摘要与通知 Agent

目标是形成“内容理解 -> AI 源沉淀 -> 主动提醒”的闭环。

实施内容：

1. 建立持久化推荐候选池和推荐记录。
2. 建立 `interest_rules`、`feed_recommendations`、`recommendation_feedback`。
3. 使用阅读行为、来源权重、标签、语言、收藏、隐藏和停留时间形成基础评分。
4. 生成推荐原因，区分已订阅来源和未订阅候选来源。
5. 支持日报、周报、专题摘要和热点事件分析。
6. 生成内容写入 `messageFeed AI` 源。
7. 支持企业微信、ntfy 和后续微信通道推送。
8. 建立通知冷却、免打扰时间、幂等键、失败重试和通知历史。
9. 用户可以用自然语言调整摘要范围、推送频率和通知偏好。

推荐信号建议：

```text
强正反馈：收藏、点击原文、多次打开、读完、主动订阅来源
中正反馈：打开详情、停留较长、滚动超过 70%
弱正反馈：列表曝光后停留、同类内容连续打开
强负反馈：隐藏、不感兴趣、减少类似推荐
弱负反馈：多次曝光但长期不打开
```

阶段七验收标准：

- 推荐 Feed 可以稳定混合已订阅和未订阅内容。
- 推荐条目具有推荐原因。
- 用户可以反馈“不感兴趣”和“减少类似推荐”。
- 系统可以生成日报并写入 AI 源。
- 系统可以通过企业微信或 ntfy 发送摘要提醒。
- 通知具备幂等键、冷却时间和失败记录。

## 8. 阶段八：金融与跨领域分析 Agent

金融分析 Agent 使用独立专项计划维护，详见 `docs/financial-agent-plan.md`。

本总纲仅保留阶段八的集成目标：

- 将金融行情、财经资讯、主动网络研究和 AI 分析联动。
- 规则判断保持确定性，AI 不参与基础阈值判断。
- 规则命中后生成 `messageFeed AI` 源分析条目。
- 高优先级事件通过企业微信或 ntfy 推送。
- 金融分析必须标注“不构成投资建议”。
- 不接入自动交易、券商账户和确定性买卖建议。

阶段八最小验收闭环：

- 可以新增一个指数或 ETF 关注标的。
- 可以配置当日涨跌幅阈值规则。
- 可以拉取行情快照并触发规则。
- 规则命中后生成 AI 源分析条目。
- 可以通过企业微信或 ntfy 发送金融告警。
- 同一规则在冷却时间内不会重复发送。

## 9. Web 产品形态

Web 侧应逐步形成以下入口：

```text
订阅
推荐
messageFeed AI
来源管理
Agent 任务
我的偏好
设置
```

AI 源页面：

- 展示日报、周报、热点分析、主动网络研究、金融分析和 Agent 报告。
- 支持按生成类型筛选。
- 支持收藏、隐藏、阅读原文、查看依据。
- 展示输入来源、关联条目、模型、生成时间和推送状态。

Agent 任务页面：

- 展示自然语言命令历史。
- 展示待确认计划。
- 展示执行中任务、失败任务和可重试任务。
- 展示主动网络采集任务。
- 展示调度任务和下一次执行时间。

我的偏好页面：

- 展示显式关注主题。
- 展示短期兴趣和长期兴趣。
- 展示减少推荐的主题。
- 支持调整权重、删除标签、固定偏好和清空隐式画像。
- 支持查看某个标签的形成原因。

## 10. 安全、权限与治理约束

必须遵守：

- 模型不直接写数据库。
- 模型不直接访问密钥、token、Webhook URL 和数据库 DSN。
- 所有执行动作必须通过能力注册和 service 接口。
- 高风险操作必须确认。
- 删除类自然语言默认解释为停用或归档，永久删除必须二次确认。
- 主动网络采集必须保留来源、时间、hash、抽取方法和失败原因。
- 搜索结果不能直接视为事实。
- 重要分析必须区分事实来源和模型推断。
- 指标 label 不使用高基数字段。
- trace attribute 不写入大正文、完整提示词或敏感配置。
- 登录态采集、绕过访问限制、规避反爬等能力不进入早期实现。

## 11. 推荐落地顺序

在进入本计划前，仍应先完成以下基础事项：

1. 收尾阶段二 Web 闭环：已读、收藏、隐藏、筛选、分页和阅读模式偏好。
2. 补齐 `api/openapi.yaml` 中已实现接口。
3. 完成阶段三 Compose 观测验收。

随后按以下顺序推进：

1. Agent 基础表、能力注册、计划、执行、审计。
2. `messageFeed AI` 内部源和 AI 生成内容入库。
3. 订阅管理 Agent：源搜索、源推荐、订阅、停用、标签和权重调整。
4. 主动网络采集：静态网页抽取、网页变化监控、搜索型采集。
5. 阅读行为事件和基础用户画像。
6. 推荐候选池、推荐原因和反馈闭环。
7. 日报、周报、热点分析和 AI 源内容生成。
8. 企业微信、ntfy 通知和通知审计。
9. 金融监控和跨领域分析。
10. 工程化增强、集成测试、E2E 测试和 Dashboard 迭代。

## 12. 最小可验收闭环

最小 Agent 闭环建议定义为：

```text
用户输入自然语言：
“帮我关注 Go、AI infra 和宏观金融，每天早上生成摘要，有重大事件通过企微提醒。”

系统生成计划：
- 搜索并建议订阅相关官方源和高质量来源。
- 创建 messageFeed AI 日报任务。
- 创建重大事件提醒规则。
- 配置企业微信通知通道。
- 保存用户显式偏好标签。

用户确认后：
- 订阅来源。
- 创建调度任务。
- 生成一条 Agent 操作报告写入 messageFeed AI。
- 后续按计划生成日报、热点分析和提醒。
```

该闭环完成后，项目将从 RSS 阅读器扩展为受控的个人信息 Agent 系统。普通来源负责稳定输入，主动网络采集补足无 RSS 信息源，`messageFeed AI` 负责沉淀分析和执行结果，通知系统负责把高价值内容送达用户。
