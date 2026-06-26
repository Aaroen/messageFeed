# messageFeed AI Agent 阶段重组计划

**最后更新**：2026-06-25

## 1. 背景与目标

原阶段五到阶段八分别覆盖自动化与推荐、AI 摘要与通知、自然语言设置控制、金融监控。随着产品目标扩展，这些能力不应继续作为彼此割裂的功能线推进，而应重组为统一的 `messageFeed AI Agent` 体系。

重组后的目标是：构建一个基于本项目数据、API、service 层、观测系统和通知通道的受控智能 Agent。该 Agent 可以代理本项目内的智能操作，包括内容推荐、源发现与订阅管理、主动网络采集、摘要生成、提醒推送、金融事件分析、用户偏好建模和设置控制。

该 Agent 的能力边界是“项目内受控智能操作”，不是通用无限工具执行平台。模型不得直接写数据库，不得绕过权限、确认、幂等和审计流程。所有执行动作必须通过已注册能力和既有 service 接口完成。

## 2. 参考项目技术路线与本项目取舍

本项目参考 `Eino`、`LangChainGo`、`ai_agent_scaffold_go`、`Hermes Agent`、`OpenClaw`、`Claude Code` 和 `OpenAI Codex`。这些项目分别代表了三类路线：

1. Go Agent 框架路线：以 Eino、LangChainGo、ai_agent_scaffold_go 为代表，重点是组件抽象、Runner、Tool、Memory、workflow 和 HTTP 运行时。
2. 产品级 Agent 运行时路线：以 Hermes Agent、OpenClaw 为代表，重点是会话持久化、多通道入口、工具治理、记忆、压缩、调度和安全边界。
3. 代码型 Agent 运行时路线：以 Claude Code、OpenAI Codex 为代表，重点是 session/turn、thread store、权限策略、工具延迟加载、上下文压缩和执行审计。

`messageFeed` 不应直接照搬任何一种路线。其核心目标是个人信息聚合、订阅管理、推荐摘要、主动采集、通知推送和金融分析，具备明确业务边界、稳定数据库模型和可审计执行要求。因此最终路线应是“领域受控 Agent Runtime”：吸收成熟项目的运行时结构、上下文治理和能力治理方法，但执行面收敛到本项目已有 Go service、PostgreSQL 主存储和 Web 产品边界内。

### 2.1 Eino：组件化 Agent 与图编排路线

Eino 的技术路线是 Go 原生 LLM 应用框架。其核心抽象包括 `ChatModel`、`Tool`、`Retriever`、`ChatTemplate`、ADK Agent、Runner、compose graph、workflow、callback、checkpoint 和 interrupt/resume。它支持 `ChatModelAgent` 自动处理工具调用循环，也支持 `DeepAgent` 将复杂任务拆解给子 Agent，并通过 compose graph 将确定性流程包装为工具。

可吸收设计：

- 组件接口要稳定。`messageFeed` 应定义 `LLMClient`、`AgentCapability`、`MemoryProvider`、`RecallTool`、`Planner`、`Executor` 等小接口，而不是让业务层直接依赖某个模型 SDK。
- 确定性业务流程可以包装为 capability。例如“搜索候选源 -> 校验健康状态 -> 生成订阅建议”应作为后端流程，由 Agent 决定是否调用，而不是让模型逐步拼 SQL 或直接改库。
- callback 思路可用于观测。在 Agent 的开始、计划生成、策略判定、工具执行、召回、压缩和结束位置写入 trace、指标和审计。
- interrupt/resume 思路适合用户确认。中高风险计划进入暂停状态，用户确认后从 plan checkpoint 恢复。

不直接采用项：

- 不以通用 `DeepAgent` 多子 Agent 自主协作作为早期默认模式。原因是本项目早期更需要稳定、可审计和可测试的领域动作。
- 不让图编排成为业务主模型。业务流程仍优先由 service 实现，Agent 只是智能编排入口。

映射到本项目：

```text
Eino Runner          -> AgentTurnRunner
Eino Tool            -> AgentCapability
Eino GraphTool       -> 受控业务流程 capability
Eino Callback        -> AgentAuditLogger + OpenTelemetry span
Eino checkpoint      -> agent_plans / agent_plan_steps / agent_turns
Eino interrupt       -> PolicyEngine(prompt) + 用户确认恢复
```

### 2.2 LangChainGo：Agent / Executor / Tool / Memory 路线

LangChainGo 的技术路线是将 Agent 决策、工具、执行器和 Memory 拆成相对清晰的接口。其 `Executor` 循环会调用 `Agent.Plan`，得到 action 或 finish，再按工具名执行工具并追加 observation；同时通过 `MaxIterations` 限制循环，避免 Agent 无限运行。Memory 提供 buffer、window、summary 等不同会话记忆形态。

可吸收设计：

- Agent 执行必须有最大步数。`messageFeed` 的 `acquisition_loop` 应设置明确的最大轮次、最大工具调用数、最大外部请求数和最大成本。
- 工具执行结果应进入结构化 observation。每次 capability 调用都应保存输入摘要、输出摘要、状态差异、错误和 trace id。
- Memory 需要按用途分层。短期窗口、摘要记忆、用户画像、内容事实、任务审计不能混为一个聊天历史数组。
- `context.Context` 应贯穿所有 Agent、service、repository 和外部 provider 调用，支持取消、超时和 trace 传播。

不直接采用项：

- 不采用字符串 ReAct 解析作为核心协议。本项目应优先使用结构化 JSON schema、显式 plan/step/capability 表和后端校验。
- 不把 Memory 只理解为 chat history。用户画像、阅读行为、网页快照、AI 源报告和金融行情都是更关键的长期记忆。

映射到本项目：

```text
Agent.Plan           -> AgentInterpreter + AgentPlanner
Executor             -> AgentTurnRunner + AgentExecutor
Tool                 -> AgentCapability
Observation          -> agent_plan_steps.result_summary / agent_audit_logs
MaxIterations        -> execution_budget.max_steps / max_tool_calls
Memory               -> MemoryProvider 聚合层
```

### 2.3 ai_agent_scaffold_go：DDD、端口适配器与配置装配路线

`ai_agent_scaffold_go` 的技术路线是 Go 后端 Agent 脚手架。它使用 Gin、配置驱动 Agent、Armory 装配链、Runner、Registry、SessionStore、MCP SSE 工具调用、OpenAI-compatible adapter 和 DDD 分层。其启动链路由配置表加载 Agent，再按 `Root -> AiAPI -> ChatModel -> Agent -> Workflow -> Runner` 装配运行时对象，并通过 HTTP API 暴露会话和聊天接口。

可吸收设计：

- 使用端口与适配器隔离外部依赖。模型、搜索、网页抓取、通知、行情 provider 和向量检索都应通过接口注入。
- 使用 Registry 管理可运行对象。`AgentCapabilityRegistry`、`AgentSessionManager` 和 `AgentEvalRegistry` 应有清晰入口。
- HTTP 层只处理入参和出参，Agent 的计划、权限、执行和审计不能写在 handler 中。
- OpenAI-compatible adapter 可以作为第一版模型协议，后续再扩展 Ollama 或其他 provider。

不直接采用项：

- 不以 YAML 配置作为本项目 capability 的主事实来源。原因是本项目能力与用户、权限、风险、service binding 和审计高度相关，主数据应进入 PostgreSQL。
- 不引入 MySQL/Redis 作为早期 Agent 主依赖。本项目仍以 PostgreSQL 为主存储，Redis 仅作为后续可选缓存、队列、限流或分布式锁。

映射到本项目：

```text
Armory 装配链        -> AgentBootstrap / CapabilityBootstrap
Runner Registry      -> AgentSessionManager + CapabilityRegistry
SessionStore         -> agent_sessions / agent_turns / transcript 表
OpenAI adapter       -> internal/llm OpenAI-compatible provider
MCP ToolRouter       -> 后置可选，不作为阶段五必要能力
```

### 2.4 Hermes Agent：多通道入口、持久记忆与审批路线

Hermes Agent 的技术路线是产品级个人 Agent。它提供 CLI/TUI、多消息网关、工具网关、持久记忆、技能系统、上下文压缩、会话搜索、定时任务、审批机制、终端后端和子 Agent 并行。其重要启发不是“让本项目也成为通用桌面 Agent”，而是其闭环设计：Agent 能从多个入口接收任务，跨会话保留用户模型，在合适时机调度任务，并对高风险操作进行审批。

可吸收设计：

- 多通道入口应抽象为消息入口，而不是绑定某个通知实现。本项目先支持 Web、企业微信、ntfy，后续可扩展公众号或其他渠道。
- 记忆需要主动沉淀。用户画像、通知偏好、长期主题、减少推荐主题应从行为证据中提升，但必须保留证据和用户可编辑能力。
- 审批应是一等流程。新增订阅、批量停用、创建提醒、提高通知频率、创建金融告警等必须有 plan、影响摘要、确认状态和审计记录。
- 定时任务应与 Agent 会话隔离。日报、网页监控、金融告警等系统触发任务需要独立 session/turn，不应污染用户当前会话。

不直接采用项：

- 不引入终端执行、云 VM、Docker/SSH/Singularity 等代码执行后端。
- 不把 skill 自学习作为早期能力。早期应优先稳定 subscription、summary、notification、acquisition 和 financial capability。

映射到本项目：

```text
Messaging Gateway    -> notifier + inbound_channel 抽象
Persistent memory    -> user_interest_profiles / tags / evidence
Command approval     -> PolicyEngine(prompt) + agent_approvals
Cron scheduling      -> scheduler + isolated agent_turns
Session search       -> conversation.query_history / ai_source.search / item.search
```

### 2.5 OpenClaw：Gateway、session、transcript 与 compaction 路线

OpenClaw 的技术路线是 Gateway 拥有运行时事实来源。它将 session store 与 transcript 分层：session store 维护小而可变的会话元数据，transcript JSONL 保存追加式对话、工具调用和压缩摘要。它还设计了 sessionKey、sessionId、写锁、维护清理、自动 compaction、overflow recovery、reserveTokens、keepRecentTokens、工具调用与工具结果成对分块、pre-compaction memory flush 和 silent housekeeping。

可吸收设计：

- session 元数据与 transcript 原文应分层。`agent_sessions` 保存可查询状态，`agent_transcript_entries` 保存可重建上下文的追加式历史。
- 短期上下文裁剪必须保护工具调用对。assistant tool call 与 tool result 不能拆开，否则后续上下文会出现不可解释的工具状态。
- 长期聊天不采用摘要替换。旧消息只从热上下文移出，并通过 transcript 归档索引支持后续查询。
- 重要偏好、计划和事实可以进入长期记忆候选，但候选必须保留 transcript 原文引用，不能由摘要替代证据。
- session 清理必须有保留策略。长期运行后，transcript、归档、评测结果和 AI 源内容都需要保留边界或归档策略。

不直接采用项：

- 不使用本地 JSONL 文件作为本项目主事实来源。`messageFeed` 已经以 PostgreSQL 为主存储，session、turn、transcript、archive 和 recall event 都应入库。
- 不采用 OpenClaw 的通用插件 runtime 作为早期目标。阶段五应先实现内置 capability。

映射到本项目：

```text
sessions.json        -> agent_sessions
transcript JSONL     -> agent_transcript_entries
history index        -> agent_transcript_archive_index
pre-compaction flush -> agent_memory_promotions
memory search        -> conversation.query_history / profile.explain / ai_source.search
session cleanup      -> retention policy + archive index maintenance
```

### 2.6 Claude Code：延迟工具发现与上下文折叠路线

Claude Code 参考项目中最值得吸收的是 ToolSearch 与 Context Collapse。ToolSearch 将工具分为 core tools 与 deferred tools。core tools 常驻模型上下文，deferred tools 不直接进入 tools 数组，而是通过搜索工具发现，再通过代理执行工具调用，从而保持工具数组稳定、降低 token 常量开销并减少 prompt cache 失效。Context Collapse 则把上下文接近上限或出现请求过大错误时的消息折叠作为恢复机制，并保留历史摘要与投影视图。

可吸收设计：

- capability 暴露必须分层。少量低风险、高频能力进入 `core`；大多数能力进入 `deferred`；敏感能力进入 `hidden`。
- capability schema 应保持稳定。已发现的 deferred capability 不应动态注入完整工具数组，而应通过 `capability.execute` 代理执行。
- 搜索能力需要多模式。支持按 capability_key 精确选择、按关键词发现、按目标类型过滤、按风险等级过滤。
- 上下文折叠要区分普通维护和溢出恢复。普通维护按阈值触发；溢出恢复是错误恢复路径，必须写入审计。

不直接采用项：

- 不依赖私有模型 API 的 tool reference 或 provider 专有特性。应使用本项目自建 capability search 和普通结构化工具调用。
- 不把 deferred capability 设计为任意外部工具。它必须绑定 service 或受控 adapter。

映射到本项目：

```text
Core Tools           -> core capability
Deferred Tools       -> deferred capability
SearchExtraTools     -> capability.search
ExecuteExtraTool     -> capability.execute
Context Collapse     -> AgentContextManager.Compact
Overflow recovery    -> context_overflow_recovery turn event
```

### 2.7 OpenAI Codex：ThreadStore、Session/Turn 与权限策略路线

OpenAI Codex 的技术路线是将线程历史、元数据、会话运行和执行策略拆出清晰边界。`ThreadStore` 负责追加 canonical history 和更新 metadata，`LiveThread` 负责 active session 的持久化协调，`core/session` 创建或恢复线程，不关心底层存储是本地文件还是其他实现。Codex 还将 session source、sub-agent source、turn、multi-agent mode、工具路由和 compact prompt 分层处理。

可吸收设计：

- history append 与 metadata update 必须分开。追加 transcript 不应隐式推断和修改会话元数据，元数据变化应由上层显式写入。
- session 与 turn 是运行时基本单位。用户输入、系统触发、定时任务、通知回调都应形成 turn，并关联状态、模型、成本、错误和审计。
- 权限策略应落到可执行决策。最终不是 `low/medium/high`，而是 `allow`、`prompt`、`forbidden`。
- compact prompt 应以交接摘要为目标，保留当前进展、关键决策、约束、剩余步骤和引用，而不是泛化总结。

不直接采用项：

- 不 fork Codex，也不迁移其 Rust 运行时。Codex 服务代码执行型 CLI Agent，其 TUI、shell、sandbox、patch、diff、云任务、插件和 MCP 体系与本项目信息聚合主线不匹配。
- 不把代码执行权限模型完整迁入本项目。`messageFeed` 的高风险主要是订阅变更、通知发送、画像写入、主动采集和金融告警。

映射到本项目：

```text
ThreadStore          -> AgentTranscriptStore
LiveThread           -> ActiveAgentSession
append_items         -> AppendTranscriptEntries
update_metadata      -> UpdateSessionMetadata
Session / Turn       -> agent_sessions / agent_turns
allow/prompt/forbidden -> PolicyEngine decision
compact prompt       -> context handoff summary template
```

### 2.8 opencode：会话入队、上下文 epoch、工具注册与输出边界

opencode V2 的可借鉴点是将 prompt admission、可见历史提升、Context Epoch、自动 compaction、工具注册和工具结果 settlement 明确拆开。工具定义有输入输出 codec，执行接收稳定 invocation context，注册有作用域和覆盖规则，输出先保留完整结构化结果，再对模型可见部分做裁剪。

可吸收设计：

- 用户输入先入 durable inbox，再在安全边界提升为模型可见消息。企业微信回调也应先落库，再由 worker 创建或恢复 turn。
- 系统上下文应有版本化快照。模型可见的系统提示词、当前时间、模型信息、能力边界和用户快照需要记录版本，不能每次隐式漂移。
- 工具注册与执行必须稳定。能力被 advertised 后，本轮执行应绑定当时的 capability 版本；如果后续能力被替换，旧调用应按 stale call 处理。
- 工具输出应完整保存，模型可见结果可以裁剪。网页正文、搜索结果和仓库文件列表应保留原始引用，发送给模型的是预算内 projection。

映射到本项目：

```text
PromptAdmitted      -> agent_inbound_messages / agent_commands
Context Epoch       -> prompt_version + memory_snapshot_version + capability_version
ToolRegistry        -> AgentCapabilityRegistry
Tool settlement     -> Observation + agent_audit_logs
Output bounding     -> ContextBudgetManager.TrimToolObservation
```

### 2.9 A2A：Task 生命周期、Agent Card 与外部协作边界

A2A 的关键价值是区分普通 `Message` 与可跟踪 `Task`。简单交互可以直接返回消息；复杂或长时任务应创建 Task，并进入 working、input-required、auth-required、completed、failed、canceled 等状态。Task 终态不可重启，后续修订应创建新 Task 并引用旧 Task。Agent Card 则提供能力发现、认证和服务端点描述。

可吸收设计：

- `AgentRun` 应具备标准 task 状态，而不仅是成功或失败。
- 需要用户补充信息时应进入 `input_required`，等待用户继续，而不是将其记录为普通失败。
- 复杂任务的产物应成为 artifact 或 result record，可被后续任务引用。
- A2A 可作为后续外部 Agent 协作协议边界；阶段五不默认开放外部 Agent 服务。

映射到本项目：

```text
A2A Task            -> AgentRun
contextId           -> 企微长期 session / Web 用户会话
taskId              -> agent_runs.id
input-required      -> PolicyEngine(prompt) / RunLoop.AskUser
artifact            -> AI 源报告 / web_snapshot / execution_result
Agent Card          -> 后续 capability registry 对外只读视图
```

### 2.10 LangGraph 与 AutoGen：状态图、中断恢复和团队编排

LangGraph 的可借鉴点是 state graph、checkpoint、interrupt/resume 和 retry policy。AutoGen 的可借鉴点是消息驱动 runtime、agent/team 注册、group chat manager、termination condition 和运行队列。它们都说明复杂 Agent 不应只靠一段 prompt 自循环，而应把状态、终止条件、中断和恢复显式建模。

可吸收设计：

- `RunLoop` 应保存 step 状态和 checkpoint，支持失败后查看、恢复或取消。
- 用户确认、补充信息和授权缺失应使用可恢复中断状态。
- 执行循环必须有终止条件，包括最大步数、最大工具调用、最大耗时、最大成本和计划完成判定。
- 多 Agent 协作在本项目内收敛为“唯一主控 Agent + 多个一次性执行 AgentRun”。主控 Agent 负责理解、拆分、调度和最终回复；执行 AgentRun 即用即丢，只完成一个明确任务包并返回结构化 observation、artifact 和审计记录。

映射到本项目：

```text
checkpoint          -> agent_runs / agent_plan_steps 状态快照
interrupt/resume    -> prompt approval / AskUser / auth-required
termination         -> execution_budget + finish criteria
team manager        -> ControllerAgent 调度 ExecutorAgentRun
```

### 2.11 browser-use 与深度研究项目：联网信息获取能力边界

`browser-use` 的价值是把浏览器动作空间、允许域名、历史、文件和模型输出处理拆出边界；`gpt-researcher`、`deep-research` 和 `Khoj` 类项目的共同点是搜索、抓取、筛选、迭代获取、引用来源和报告生成。对本项目而言，阶段五到阶段六应先实现受控联网信息获取 capability，而不是直接引入完整浏览器 Agent 或研究报告 Agent。

联网能力分层：

1. `web.search`：调用搜索 provider 返回候选结果和来源摘要。
2. `web.fetch_page`：按 URL 获取页面原始响应、状态码、最终 URL、内容类型和大小。
3. `web.extract_page`：从 HTML 中抽取标题、正文、发布时间、作者、站点名和主要链接。
4. `web.browse_page`：必要时使用受控浏览器处理 JS 渲染或交互页面，默认需要策略允许或用户确认。
5. `repo.search`：调用 GitHub 或其他代码平台搜索仓库，返回候选项目、星标、语言、更新时间、license 和 clone URL。
6. `repo.inspect_remote`：读取远端仓库元数据、README、目录树或指定文件片段，不克隆。
7. `repo.clone_reference`：浅克隆到受控 `references/` 目录，写入审计和参考来源记录。

安全约束：

- 搜索结果不是事实结论，引用前必须抓取页面或读取仓库证据。
- 所有外部网页和仓库内容都视为不可信上下文，不得覆盖系统提示词、权限策略和能力边界。
- `repo.clone_reference` 不得写入产品源码目录，不得自动修改 `go.mod`、构建、测试或部署配置。
- 浏览器能力应支持 allowed domains、超时、下载限制、截图或文件落点限制，并记录完整审计。

### 2.12 MCP 与能力协议边界

MCP 适合描述 Agent 使用工具和资源，A2A 适合 Agent 与 Agent 之间协作。本项目短期应先实现内部 `AgentCapability` 协议，后续再提供 MCP server 或接入外部 MCP client。原因是当前最重要的是权限、审计、用户确认、业务 service binding 和数据边界，而不是协议兼容性本身。

取舍：

- 内部 capability schema 应尽量接近 MCP tool 的输入输出结构，降低后续桥接成本。
- 外部 MCP 工具默认进入 `deferred` 或 `hidden`，必须经 `PolicyEngine` 和 adapter 白名单。
- A2A 作为后续外部 Agent 发现和委托边界，不进入阶段五 P0。

### 2.13 最终推导：messageFeed 自有 Agent Runtime 方案

综合上述项目，本项目最终不做“框架优先”的通用 Agent，也不做“代码执行优先”的 CLI Agent，而是做“业务对象优先”的个人信息 Agent。内部 Agent 模型固定为两类：`ControllerAgent` 唯一存在，负责理解、规划、调度、确认和最终回复；`ExecutorAgentRun` 可无限创建、即用即丢、上下文隔离，负责执行一个明确任务包并返回结构化结果。能力仍由同一个 `AgentCapabilityRegistry` 统一注册，主控只为每个执行 AgentRun 下发本次可见的 capability scope。

核心设计如下：

```text
企微消息 / Web 命令 / 系统事件 / 定时任务
  -> AgentTrigger 判断是否需要唤起 Agent
  -> AgentRunManager 创建 controller AgentRun
  -> TaskPacketBuilder 从原始入口构造任务包
  -> ModelRouter 为 ControllerAgent 选择规划模型
  -> ContextBuilder 构建 controller 上下文，只注入系统提示词、任务包、用户快照、能力边界和必要召回片段
  -> ControllerAgent 生成执行计划和若干 executor task
  -> PolicyEngine 输出 allow / prompt / forbidden
  -> AgentRunManager 按需创建一个或多个 executor AgentRun
  -> ExecutorAgentRun 使用被授予的 capability scope 执行单一任务
  -> AgentExecutor 调用 service-bound capability 并生成 observation / artifact
  -> ControllerAgent 汇总结果、判断继续、重规划、追问或结束
  -> ContextBudgetManager 在每次模型调用前后管理 token、工具结果裁剪和上下文投影视图
  -> Finalizer 生成对话回复、AI 源报告或通知投递
  -> AgentAuditLogger 记录 controller/executor 全部上下文、审计、trace、成本、模型选择和状态差异
```

自有方案的五个平面：

| 平面 | 模块 | 设计依据 | 本项目取舍 |
| --- | --- | --- | --- |
| 运行时平面 | `ControllerAgent`、`ExecutorAgentRun`、`AgentRunManager`、`AgentTranscriptStore` | Codex、OpenClaw、A2A、AutoGen | PostgreSQL 作为 session/turn/run/transcript/context trace 主事实来源 |
| 能力平面 | `AgentCapabilityRegistry`、`CapabilitySearch`、`CapabilityExecutor` | Claude Code、Eino、LangChainGo | `core/deferred/hidden` 分层，能力必须绑定 service |
| 记忆平面 | `MemoryProvider`、`ProfileMemoryProvider`、`ArchiveStore`、`RecallTool` | Hermes、OpenClaw、LangChainGo | 用户画像是长期记忆底座，但保留原始证据和事实层 |
| 策略平面 | `PolicyEngine`、`ApprovalService`、`RiskClassifier` | Codex、Hermes、Claude Code | 风险等级只参与判定，最终决策为 `allow/prompt/forbidden` |
| 评估平面 | `AgentEvalHarness`、trace、状态断言、人工复核 | OpenClaw QA、LangSmith 类方法 | 先实现项目内评测表和命令行评测，再考虑外部平台 |

新增运行约束：

- 每次真正唤起 Agent 执行任务时创建新的 controller `AgentRun`，默认不沿用企微长期聊天窗口。企微长期对话只作为入口、transcript 主事实源和历史查询资料库。
- `ControllerAgent` 是唯一主控；`ExecutorAgentRun` 可按任务无限创建、即用即丢、可并发执行，但必须有 `parent_run_id` 指向 controller run。
- `ExecutorAgentRun` 初始上下文是空执行上下文加结构化任务包，不继承 controller 的完整对话消息，也不继承其他 executor 的过程消息。需要历史时只能通过已授权 capability 查询。
- 每个 executor 的完整模型可见上下文必须可追溯，包括任务包、系统提示词版本、模型、capability scope、工具 schema 摘要、输入消息投影视图、工具调用、observation、artifact、token 估算、裁剪记录和最终输出。密钥、token、数据库 DSN 等敏感值不得进入上下文快照。
- 强模型负责复杂任务规划、能力选择、任务拆解、失败重规划和长上下文判断；普通模型负责简单任务执行、固定提醒、摘要和格式化；轻量模型负责分类、标签、索引和低风险判断。
- 系统必须维护模型元数据，包括 provider、角色、上下文窗口、最大输出、工具调用支持、结构化输出支持、成本、延迟等级、可靠性和启用状态。模型路由不得依赖硬编码模型名。
- 执行前必须进行 `ContextFitEstimate`，估算工具次数、工具结果大小、模型轮次、输入输出 token 和是否可在单上下文完成。无法单上下文完成时，应拆分任务、降级范围、转后台多 run 或向用户确认。
- 上下文压缩只作用于模型可见投影视图，不能替代 `agent_transcript_entries` 原文。工具调用与工具结果不得被拆开；如工具结果被裁剪，必须保留结构化 observation 和原始数据引用。

自有方案的执行模式：

1. `plan_once`：默认模式。适用于订阅管理、通知设置、偏好修改、日报任务创建等可明确建模的操作。
2. `acquisition_loop`：有限信息获取循环。适用于联网信息获取、网页变化监控、热点事件事实收集和金融异动事实核验，必须限制轮次、外部请求数、token、成本和召回预算。
3. `scheduled_turn`：定时任务模式。适用于日报、周报、网页监控、行情监控和通知冷却检查，与用户当前对话隔离。
4. `review_only`：只生成建议不执行。适用于不确定来源、批量订阅建议、金融解释和高风险画像变更。

自有方案的关键约束：

- 模型只能生成意图、计划、参数摘要、解释文本和候选证据，不能直接写数据库。
- 所有状态变更必须通过 capability 调用既有 service 完成。
- 所有 capability 都必须有输入 schema、输出 schema、风险等级、确认策略、幂等键和审计事件。
- `agent.schedule_task` 只保存定时契约，包括目标、执行窗口、投递时间、新鲜度策略、允许能力、模型策略和失败策略；到点后创建新的隔离 `AgentRun`，不得另起一套定时执行逻辑。
- 长期画像写入必须有证据链、置信度、来源和可编辑能力。
- 主动网络采集必须保留 URL、时间、hash、抽取方法和可信度，搜索结果不能直接作为事实。
- 金融分析必须区分事实数据、模型推断和风险提示，且不输出确定性投资建议。

### 2.9 阶段五内部落地顺序

阶段五不应一次性实现完整 Agent。结合当前企业微信管理台只开放“自建应用”的实际条件，当前优先级调整为“企业微信自建应用接收消息 API 作为对话入口，主动通知后置”：

1. 建立企业微信自建应用接收消息回调入口，完成 URL 验证、签名校验、AES 解密、XML 消息标准化和消息幂等。
2. 建立 `external_accounts`、`agent_inbound_messages`、`agent_sessions`、`agent_turns`、`agent_transcript_entries` 和 `agent_audit_logs`，先支撑可追溯对话，不先扩展完整计划执行系统。
3. 实现 `AgentSessionManager`、`AgentTurnRunner` 和 transcript append，保证同一 session 内 active turn 串行。
4. 实现最小只读 Agent Runner，先支持最近资讯查询、指定来源最新条目查询、当前消息摘要或简短问答。
5. 短回答可以使用企业微信自建应用被动回复；模型耗时较长时先入库并快速返回，再通过同一自建应用的 `message/send` 向用户异步回复，避免阻塞企业微信回调。
6. 建立企业微信自建应用 `access_token` 缓存与 `message/send` 基础适配，但该能力在 P0 仅作为对话回复出口，不作为主动通知系统。
7. 实现最小 `CapabilityRegistry` 和 `PolicyEngine`，只读能力为 `allow`，订阅变更、通知配置、画像写入和金融告警为 `prompt` 或 `forbidden`。
8. 实现 `messageFeed AI` 内部源，允许 Agent 写入一条非对话类执行报告；普通企微聊天只进入 transcript 和 audit。
9. 实现 `MemoryProvider` 第一版，只读取显式偏好、近期阅读摘要、最近企微聊天窗口、当前会话目标和 capability 边界。
10. 在对话 MVP 稳定后，再补齐 `agent_capabilities`、`agent_plans`、`agent_plan_steps`、审批恢复、历史聊天查询、归档索引和 eval case。

### 2.10 企业微信自建应用与 AI 入口接入要求

官方文档核对范围包括企业微信开发者中心的 `获取access_token`、`发送应用消息`、`回调配置`、自建应用 `接收消息与事件`、`被动回复消息格式`，以及智能机器人 `接收消息`、`被动回复消息`、`回调和回复的加解密方案`、`智能机器人长连接`。结合当前管理台无智能机器人入口的条件，对本项目的约束如下：

1. 企业微信自建应用接收消息 API 是当前阶段五 P0 默认对话入口。用户在企业微信内向应用发送消息后，企业微信将加密 XML 推送到应用接收消息 URL。P0 只接文本消息，其余消息类型先记录为不支持。
2. 自建应用回调 URL 验证需要处理 `msg_signature`、`timestamp`、`nonce`、`echostr`，校验签名后解密 `echostr`，并在 1 秒内返回明文，响应不得带引号、BOM 或换行。
3. 自建应用业务回调 POST 包含 `ToUserName`、`AgentID`、`Encrypt`。解密后可获得 `FromUserName`、`MsgType`、`Content`、`MsgId`、`AgentID` 等字段。`MsgId` 可作为优先幂等键；无 `MsgId` 的事件使用 `provider + msg_signature + timestamp + nonce` 或解密 payload hash 兜底。
4. 企业微信服务器在 5 秒内收不到响应会断开并重试，总共重试三次。P0 应先验签解密、入库、创建 turn 并快速响应，模型调用由 worker 异步执行。
5. 自建应用获取 `access_token` 使用 `GET https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=ID&corpsecret=SECRET`。`access_token` 默认有效期通常为 7200 秒，最长至少预留 512 字节存储，需要按应用维度缓存并处理提前失效。
6. 自建应用发送消息使用 `POST https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=ACCESS_TOKEN`。文本消息要求 `touser`、`toparty`、`totag` 不能同时为空，`agentid` 必填，`text.content` 不超过 2048 字节，超过会截断。
7. 自建应用发送接口会对部分无效接收人返回 `invaliduser`、`invalidparty`、`invalidtag`、`unlicenseduser`，常见原因是接收人不在应用可见范围内或缺少基础接口许可。发送审计必须保存这些返回字段。
8. 自建应用需要配置应用可见范围，用户不在可见范围内时无法稳定收发应用消息。当前无正式用户系统时，可先将 `FromUserName` 映射到默认系统用户，后续再迁移为真实账号绑定。
9. 网页授权及 JS-SDK 可信域名可用于用户身份确认、账号绑定、设置页和高风险操作确认，但不替代聊天消息回调。用户在聊天窗口向应用发消息时，只有接收消息 API 会触发后端回调。
10. 如果后续企业微信管理台开放智能机器人 API 模式，可将智能机器人短连接回调作为新增 `channel/wechatwork/aibot` 适配；它更适合群聊 `@机器人` 和机器人原生交互，但不是当前 P0 前提。
11. 智能机器人长连接适用于无公网 IP、高实时性或希望免处理回调加解密的场景。长连接需要 BotID、长连接专用 Secret、心跳、断线重连和单连接主备策略，因此只作为后续增强。

### 2.11 端到端接入、对话与任务执行方案

企业微信自建应用只承担“入站消息入口”和“对话回复出口”，不承担业务决策。Agent 只承担理解、计划、解释和受控工具调用编排，不直接写数据库、不直接发送主动通知。所有真实状态变更必须由后端 service 完成。

接入链路：

```text
企业微信自建应用
  -> GET 回调 URL 验证
  -> POST 接收加密 XML
  -> channel/wechatwork 验签、解密、解析和标准化
  -> agent_inbound_messages 幂等落库
  -> external_accounts 解析外部用户
  -> agent_sessions 创建或恢复会话
  -> agent_turns 创建本轮输入
  -> AgentTurnRunner 异步处理
  -> transcript / audit / trace 记录
  -> 被动回复或 message/send 返回企业微信
```

对话运行时：

1. `handler` 只负责 HTTP 参数、响应和错误映射，不承载企业微信协议细节。
2. `channel/wechatwork` 负责 URL 验证、`msg_signature` 校验、AES 解密、XML 解析、消息标准化和 `MsgId` 幂等键生成。
3. `AgentSessionManager` 依据 `provider=wechat_work_app`、`external_user_id` 和当前系统用户创建或恢复 session。
4. `AgentTurnRunner` 将用户输入、系统提示、必要上下文和能力边界组装为 turn，保证同一 session 内 active turn 串行。
5. `AgentContextManager` 只读取必要事实：最近条目、来源、显式偏好、AI 源报告摘要和 capability 边界。
6. 短响应可走企业微信被动回复；可能超过回调时限的响应先快速返回成功，再通过 `message/send` 异步发送。

任务执行分层：

| 层级 | 阶段 | 处理方式 | 示例 |
| --- | --- | --- | --- |
| 只读回答 | P0 | `allow`，可直接执行已注册只读 capability | 查询最近资讯、查询某来源最新条目、摘要当前输入 |
| 低风险建议 | P0-P1 | 生成计划和解释，不立即改变状态 | 推荐可订阅来源、建议降低低价值来源权重 |
| 需确认变更 | P1 | `prompt`，通过 Web 确认页、网页授权/JS-SDK 身份确认或企业微信确认流程后执行 | 新增订阅、停用来源、调整抓取周期、创建提醒规则 |
| 禁止执行 | 长期默认 | `forbidden`，只给出拒绝原因或安全替代方案 | 泄露密钥、绕过访问限制、默认永久删除、未授权通知目标 |

P0 capability 最小集合：

1. `feed.query_recent_items`：按用户读取最近条目。
2. `source.query_latest_items`：按来源读取最新条目。
3. `content.summarize_text`：对用户输入或条目摘要做简短总结。
4. `agent.write_transcript`：写入对话记录、模型回复和错误。
5. `agent.write_audit`：写入权限决策、工具调用摘要、耗时和回复发送结果。

P1 之后再开放可改变状态的 capability，例如 `source.subscribe`、`source.disable`、`source.update_fetch_interval`、`alert_rule.create`、`notification.configure` 和 `market_alert.create`。这些能力必须具备输入 schema、风险等级、幂等键、回滚说明、审计事件和确认策略。

### 2.12 前端授权界面、用户系统与授权对象校验

前端授权界面用于“绑定身份”和“确认高风险计划”，不用于替代企业微信聊天消息回调。用户系统的目标不是第一阶段实现完整多租户，而是让企业微信用户、Web 登录用户、Agent session、计划和审批都落到同一个可审计 `user_id`，避免模型或前端把操作执行到错误对象上。

用户系统最小模型：

```text
users
- id
- display_name
- role: owner/member
- status: active/disabled
- created_at
- updated_at

user_sessions
- id
- user_id
- session_token_hash
- user_agent
- ip_hash
- expires_at
- revoked_at

auth_oauth_states
- id
- state_hash
- purpose: bind/confirm/login
- user_id
- redirect_path
- expires_at
- consumed_at

external_accounts
- provider: wechat_work_app
- corp_id
- agent_id
- external_user_id
- user_id
- binding_status: pending/active/disabled
- verified_at
- last_seen_at

agent_approvals
- id
- plan_id
- user_id
- requested_by_session_id
- requested_by_external_account_id
- approval_channel: web/wechat_work
- approval_token_hash
- status: pending/approved/rejected/expired
- expires_at
- decided_at
```

授权对象校验规则：

1. 前端不得提交任意 `user_id` 来决定操作归属。Web API 的 `user_id` 来自后端 session，企业微信消息的 `user_id` 来自 `external_accounts`。
2. `external_accounts` 绑定必须同时校验 `provider`、`corp_id`、`agent_id` 和 `external_user_id`，避免不同企业或不同应用下的同名用户混淆。
3. `agent_plans.user_id`、`agent_approvals.user_id`、目标资源的 `user_id` 必须一致，否则 `PolicyEngine` 判定为 `forbidden`。
4. 审批链接必须绑定 `approval_id`、`plan_id`、`user_id`、过期时间和一次性 token；前端只能展示计划摘要，是否允许批准由后端重新校验。
5. 企业微信确认流程必须要求 `external_accounts.user_id = agent_approvals.user_id`；Web 确认流程必须要求 `user_sessions.user_id = agent_approvals.user_id`。
6. 单用户过渡期可以通过配置指定默认 owner，但该默认 owner 只能在认证服务或账号映射服务中使用，不能写入 `channel/wechatwork` 协议适配层。

前端授权界面联动：

```text
企业微信消息触发需确认计划
  -> Agent 创建 agent_plans 和 agent_approvals
  -> message/send 返回确认链接
  -> 用户打开 /agent/approvals/:id
  -> 前端调用 /api/v1/auth/me
  -> 未登录或未绑定时跳转企业微信网页授权
  -> 后端用 OAuth code 换取企业微信 UserID
  -> external_accounts 校验并创建 Web session
  -> 前端展示计划、风险、影响范围和拒绝选项
  -> 用户批准或拒绝
  -> 后端二次校验 user_id、plan_id、approval token 和计划状态
  -> AgentExecutor 调用 service 执行或记录拒绝
```

前端页面建议：

1. `/auth/login`：开发期 owner 登录或后续正式登录入口。
2. `/auth/bindings`：查看企业微信绑定状态、最近验证时间和禁用绑定。
3. `/auth/wechat-work/callback`：企业微信网页授权回跳页，只处理 code/state 交换，不展示业务操作。
4. `/agent/approvals/:id`：展示待确认计划、影响范围、风险等级、执行对象、过期时间、批准和拒绝按钮。
5. `/agent/approvals/:id/result`：展示执行结果、失败原因、审计引用和可回滚状态。

后端 API 建议：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/v1/auth/me` | 返回当前 Web 会话用户、绑定状态和权限范围 |
| `POST` | `/api/v1/auth/logout` | 注销当前 Web 会话 |
| `GET` | `/api/v1/auth/wechat-work/oauth-url` | 生成带 state 的企业微信网页授权 URL |
| `GET` | `/api/v1/auth/wechat-work/callback` | 接收 OAuth code，换取企业微信 UserID 并建立或绑定会话 |
| `GET` | `/api/v1/auth/bindings` | 查询当前用户外部账号绑定 |
| `PATCH` | `/api/v1/auth/bindings/{id}` | 禁用或恢复绑定，不做物理删除 |
| `GET` | `/api/v1/agent/approvals/{id}` | 查询待确认计划详情 |
| `POST` | `/api/v1/agent/approvals/{id}/approve` | 批准并触发执行 |
| `POST` | `/api/v1/agent/approvals/{id}/reject` | 拒绝计划并记录原因 |

## 3. 重组后阶段定义

| 新阶段 | 名称 | 目标 | 原阶段映射 |
| --- | --- | --- | --- |
| 阶段五 | 企业微信对话入口 Agent MVP 与 AI 源 | 先打通企业微信自建应用接收消息入口、session/turn、只读 Runner、审计和 AI 内部源，再补齐能力注册、计划、执行和风险控制 | 原阶段五、六、七的基础部分 |
| 阶段六 | 主动采集与内容理解 Agent | 支持无 RSS 信息源的网络最新信息获取、网页监控、搜索型采集和内容理解 | 原阶段五、十的一部分 |
| 阶段七 | 推荐、摘要与通知 Agent | 实现个性化推荐、日报、周报、热点事件分析、企业微信和 ntfy 推送 | 原阶段五、六 |
| 阶段八 | 金融与跨领域分析 Agent | 金融行情、资讯、主动网络信息获取与 AI 分析联动，生成可推送的风险提示 | 原阶段八 |

阶段九继续承担工程化增强，包括 OpenAPI 契约、集成测试、E2E 测试、Dashboard、部署配置和契约校验。阶段十继续承担来源扩展与分布式升级验证。

## 4. 核心概念

### 4.1 项目级 AI Agent

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

### 4.2 AI 源

AI 源是系统内置的虚拟订阅源，建议命名为 `messageFeed AI`。它不是外部 RSS，而是 Agent 生成内容的统一展示入口。

AI 源可展示以下内容：

- 每日摘要。
- 每周摘要。
- 热点事件分析。
- 主动网络信息获取报告。
- 非 RSS 网页变化报告。
- 推荐内容包。
- 金融市场分析。
- 来源健康报告。
- Agent 执行报告。
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

### 4.3 主动网络采集

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

### 4.4 用户行为与偏好模型

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

### 4.5 记忆分层

本项目不应把“记忆”简化为聊天历史，也不应把全部历史直接塞入 prompt。企业微信 P0 中用户只有一个对话入口，因此系统内部应把 `user_id + provider + corp_id + agent_id + external_user_id` 映射为一个长期 Agent session。该 session 保存完整 transcript，但每次模型调用只自动注入短期聊天窗口；更早聊天通过历史查询能力按需取回原文。

推荐记忆层次：

1. 会话记忆：当前用户目标、最近对话、澄清问题、待确认计划和当前任务状态。生命周期短，只服务当前 Agent run，默认来自同一长期 session 的最近若干条 transcript。
2. 任务记忆：Agent 命令、计划、步骤、checkpoint、失败原因、恢复数据和审计结果。对应 `agent_commands`、`agent_plans`、`agent_plan_steps` 和 `agent_audit_logs`。
3. 行为证据记忆：曝光、打开、阅读进度、点击原文、收藏、隐藏、不感兴趣和减少类似推荐。该层是原始证据，不直接等同于偏好。
4. 用户画像记忆：显式偏好、隐式偏好、短期兴趣、长期兴趣、负反馈、通知偏好和风险偏好。该层是 Agent 默认读取的底层长期记忆。
5. 内容事实记忆：`items`、`web_snapshots`、AI 源条目、金融行情快照和相关来源引用。该层保存事实来源和可追溯证据。
6. 程序性记忆：能力注册、风险等级、确认策略、通知通道、来源健康状态和采集方式。该层告诉 Agent “可以做什么”和“必须怎样做”。

记忆注入原则：

- 企微聊天历史必须完整保存到 `agent_transcript_entries`，不得用摘要替换、改写或删除原文。
- 模型默认只接收最近聊天窗口，例如最近 8 到 12 条 `user/assistant` 消息；当前 turn 的用户消息单独追加，历史查询必须排除当前 turn，避免重复输入。
- 长期聊天记忆不通过摘要长期注入模型，而是通过 `conversation.query_history` 查询原文片段。
- 原始阅读事件不直接进入模型上下文，应先聚合为可解释画像、近期兴趣或推荐证据。
- Agent 执行失败不应污染用户偏好，只能进入任务记忆和审计记忆。
- 主动采集事实和模型推断必须分开保存。事实可以作为证据记忆，模型分析应作为 AI 源内容和生成元数据保存。
- 用户画像可进入长期记忆，但必须具备证据、置信度、更新时间和用户可编辑能力。
- 长期偏好不得由单次行为静默写入，必须来自多次证据或用户确认。

建议抽象：

```text
MemoryProvider
├── Load(ctx, scope) -> MemoryBlock
└── Explain(ctx, memory_ref) -> []MemoryEvidence

MemoryBlock
- name
- priority
- content
- token_hint
- evidence_refs
- updated_at
- trust_level
```

第一版 MemoryProvider：

- `ProfileMemoryProvider`：读取用户画像、显式偏好、短期兴趣、长期兴趣和负反馈。
- `ConversationMemoryProvider`：读取同一长期企微 session 最近聊天窗口，并返回保留角色的 `user/assistant` 消息，不把聊天历史拼入 system prompt。
- `RecentInteractionProvider`：读取近期阅读、收藏、隐藏、不感兴趣和点击原文统计摘要。
- `TaskMemoryProvider`：读取当前 Agent 计划、历史执行结果和待确认步骤。
- `ContentMemoryProvider`：读取相关条目、AI 源报告、网页快照和金融分析依据。
- `CapabilityMemoryProvider`：读取当前可用能力、风险等级、确认策略和执行边界。
- `NotificationMemoryProvider`：读取通知通道、免打扰时间、冷却时间和推送偏好。

### 4.6 上下文管理与归档回忆

本项目的上下文目标不是让模型真实拥有无限窗口，而是建立“有限活动上下文 + 可查询历史原文 + 可追溯检索索引”的工程机制。归档在本项目中只表示冷热分层、检索索引和访问策略，不表示删除、压缩或摘要替换。

上下文分层：

```text
固定上下文
  系统规则、Agent 边界、安全策略、工具使用规则。

活动上下文
  当前用户目标、当前计划、待确认问题、最近若干轮对话、最近工具结果。

可检索记忆
  历史聊天原文、用户画像、历史任务、AI 源报告、阅读行为、网页快照、金融分析、订阅变更。

归档索引
  transcript 冷热层级、类型、重要度、关键词、索引状态、访问次数和最近访问时间。

事实源
  完整 transcript、工具输入输出、模型输出、审计日志和来源引用。事实源必须原样保留。
```

上下文压力策略：

1. 每次 turn 只自动注入固定预算内的短期聊天窗口和业务上下文。
2. 当短期聊天窗口超出预算时，按消息边界裁剪最旧消息，不按 token 边界截断单条语义。
3. 更早聊天从热上下文移出，保留在 transcript 中，并通过归档索引提高后续查询效率。
4. 模型需要回忆历史时，必须通过只读历史查询能力取回原文片段，而不是依赖长期摘要。

不可压缩保护区：

- 系统规则和安全边界。
- 当前用户目标。
- 当前 Agent 计划和未完成步骤。
- 待确认事项和澄清问题。
- 最近 8 到 12 条 `user/assistant` 聊天消息，具体数量按模型窗口和任务复杂度调整。
- 最近一次关键工具调用结果。
- 用户刚刚明确表达的偏好、限制、否定反馈或授权。

归档索引类型：

```text
user_goal
clarification
agent_plan
plan_step
tool_call
tool_result
decision
research_context
preference_signal
execution_result
notification_result
financial_context
```

归档索引必须保留：

- transcript entry 引用。
- session、turn、user、role 和创建时间。
- `archive_status`：`hot`、`warm`、`cold`。
- `memory_kind`：`preference`、`task`、`fact`、`decision`、`casual`、`unknown`。
- `importance`：0 到 100 的检索优先级。
- 关键词、实体、关联 item、source、snapshot、AI item、plan 和 step。
- `indexed_at`、`last_accessed_at`、`access_count` 和索引状态。
- 可信等级和来源类型。

归档索引不得保存替代原文，也不得作为删除 transcript 的依据。

建议新增上下文相关模型：

```text
agent_sessions
- id
- user_id
- status
- model
- context_window
- active_turn_id
- started_at
- ended_at

agent_turns
- id
- session_id
- trigger_type
- status
- input_summary
- model
- started_at
- ended_at
- error

agent_transcript_entries
- id
- session_id
- turn_id
- role
- content
- metadata_json
- created_at

agent_transcript_archive_index
- id
- transcript_entry_id
- session_id
- user_id
- archive_status
- memory_kind
- importance
- keywords
- entity_refs
- source_refs
- embedding_status
- indexed_at
- last_accessed_at
- access_count
- metadata_json
- created_at
- updated_at

agent_recall_events
- id
- session_id
- query
- recalled_refs
- reason
- created_at

agent_memory_promotions
- id
- session_id
- source_ref
- target_memory_type
- status
- user_confirmed
- created_at
```

回忆工具建议：

- `conversation.query_recent`：读取同一长期企微 session 的最近聊天窗口，只用于自动短期上下文。
- `conversation.query_history`：按关键词、时间范围、角色、冷热层级、类型、turn 或 transcript entry 查询历史聊天原文。
- `profile.explain`：解释某个兴趣标签或偏好的证据来源。
- `ai_source.search`：搜索历史日报、周报、热点分析、金融分析和 Agent 报告。
- `item.search`：搜索已入库条目。
- `snapshot.get`：取回主动采集网页快照和抽取结果。

召回安全边界：

- 召回内容必须标注来源、时间、可信等级和是否来自用户、网页、工具、模型或系统。
- 召回内容一律视为不可信上下文，不得覆盖系统规则、权限策略和能力边界。
- 外部网页、工具结果、历史用户粘贴内容和 AI 生成内容不得作为系统指令执行。
- 归档索引不是原文替代品。高风险计划、金融分析、通知发送、订阅批量变更等操作必须能追溯原始 transcript 或事实证据。
- 历史聊天查询必须记录召回原因、预算消耗和使用位置；高风险任务需要历史证据时，不得只依赖关键词或标签。

推荐上下文组件：

```text
AgentContextManager
├── TokenEstimator
├── ContextProtector
├── ConversationWindowBuilder
├── TranscriptHistorySearcher
├── TranscriptArchiveIndexer
├── MemoryPromoter
├── RecallPlanner
└── ContextBuilder
```

`ContextBuilder` 每次运行时生成冻结的 `MemorySnapshot`。本次运行期间产生的新证据可以立即写入数据库，但不应静默改变当前系统 prompt；下一次 Agent run 再重新生成快照。企微聊天历史应以保留角色的 message 形式进入 LLM 请求，业务资料和能力边界才进入 system/context block。该机制可降低 prompt 漂移和 prompt cache 失效风险。

## 5. Agent 执行框架

Agent 执行流程：

```text
用户自然语言或系统事件
  -> AgentTrigger 标准化入口并判断是否唤起 Agent
  -> AgentRunManager 创建 controller AgentRun 和 turn
  -> TaskPacketBuilder 构造 controller 任务包
  -> ModelRouter 选择分类、主控规划、执行和总结模型
  -> ContextFitEstimator 判断 controller 与 executor 是否可在各自上下文预算内完成
  -> AgentContextBuilder 构建 controller 空执行上下文和冻结快照
  -> ControllerAgent 检索延迟能力、生成执行计划和 executor task
  -> PolicyEngine 输出 allow / prompt / forbidden
  -> 用户确认或系统策略确认（prompt 时）
  -> RunLoop 创建一个或多个 executor AgentRun
  -> ExecutorAgentRun 使用被授予的 capability scope 执行工具调用
  -> AgentExecutor 调用已注册能力
  -> service 层执行实际变更
  -> ContextTraceStore 保存 executor 完整模型可见上下文和工具上下文
  -> ControllerAgent 接收 observation / artifact，继续、重规划、追问或失败
  -> Finalizer 生成对话回复、AI 源条目或通知
  -> 保存 transcript、审计、指标、模型选择、token 使用、上下文快照和 trace
```

核心模块：

```text
AgentCapabilityRegistry
- Register(capability)
- List(userScope)
- Match(intent)
- Search(query, scope)
```

```text
AgentSessionManager
- CreateSession(user, scope) -> AgentSession
- ResumeSession(session_id) -> AgentSession
- StartTurn(session, input) -> AgentTurn
- CompleteTurn(turn, result) -> error
- CancelTurn(turn, reason) -> error
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
- BuildExecutorTasks(plan) -> []ExecutorTask
```

```text
AgentExecutor
- Execute(task, capabilityScope) -> AgentExecutionResult
- Rollback(plan) -> AgentRollbackResult
```

```text
AgentAuditLogger
- RecordCommand
- RecordPlan
- RecordRunContext
- RecordApproval
- RecordStepResult
- RecordModelOutput
```

```text
AgentContextManager
- BuildSnapshot(user, task_scope) -> AgentContextSnapshot
- EstimatePressure(snapshot) -> ContextPressure
- Compact(session, policy) -> ContextCompactionResult
- Recall(query, scope) -> []RecallResult
```

```text
AgentRunManager
- CreateControllerRun(trigger, taskPacket) -> AgentRun
- CreateExecutorRun(parentRun, executorTask, capabilityScope) -> AgentRun
- ClaimRun(run_id, worker) -> AgentRun
- MarkRunning(run) -> error
- CompleteRun(run, result) -> error
- FailRun(run, reason) -> error
- CancelRun(run, reason) -> error
```

```text
TaskPacketBuilder
- BuildFromWechatTurn(message, session) -> TaskPacket
- BuildFromWebCommand(command) -> TaskPacket
- BuildFromScheduledTask(task) -> TaskPacket
- ExtractRelevantContextHints(packet) -> []ContextHint
```

```text
ModelRouter
- ClassifyTask(packet) -> TaskComplexity
- SelectPlannerModel(complexity, constraints) -> ModelRef
- SelectExecutorModel(plan, constraints) -> ModelRef
- SelectAuxModel(task) -> ModelRef
- GetModelMetadata(model_id) -> ModelMetadata
```

```text
ContextFitEstimator
- Estimate(packet, capabilities, models) -> ContextFitEstimate
- CanSingleContextComplete(estimate) -> bool
- RecommendDecomposition(estimate) -> TaskDecomposition
```

```text
ContextBudgetManager
- EstimateRequest(messages, tools, model) -> TokenEstimate
- PrepareProjection(snapshot, budget) -> ContextProjection
- CompressProjection(projection, policy) -> ContextCompactionResult
- TrimToolObservation(observation, budget) -> ObservationProjection
- GuardToolCallPairs(messages) -> []ContextMessage
```

```text
RunLoop
- RunController(plan, context, modelPolicy) -> AgentRunResult
- RunExecutor(task, context, capabilityScope, modelPolicy) -> AgentRunResult
- ExecuteExecutorTask(task) -> Observation
- ContinueOrFinish(state, observation) -> LoopDecision
- Replan(state, reason) -> AgentPlan
- AskUser(question) -> AgentRunResult
- Fail(reason) -> AgentRunResult
```

```text
ContextTraceStore
- SaveRunInputProjection(run, projection) -> ContextTraceRef
- SaveModelRequest(run, request) -> ContextTraceRef
- SaveModelResponse(run, response) -> ContextTraceRef
- SaveToolSchemaProjection(run, tools) -> ContextTraceRef
- SaveToolObservation(run, observation) -> ContextTraceRef
- SaveTrimRecord(run, trimRecord) -> ContextTraceRef
```

最小 Agent Runtime 建议在 Go 后端内实现：

```text
internal/agent
├── ControllerAgent
├── ExecutorAgent
├── AgentRunManager
├── TaskPacketBuilder
├── ModelRouter
├── ContextFitEstimator
├── SessionManager
├── TurnRunner
├── CapabilityRegistry
├── CapabilitySearch
├── Planner
├── PolicyEngine
├── RunLoop
├── Executor
├── ContextManager
├── ContextBudgetManager
├── ContextTraceStore
├── MemoryProvider
├── ArchiveStore
├── RecallTool
└── AuditLogger
```

`AgentRun` 与长期企微 session 的关系：

- 企微 session 负责连续对话、账号映射和完整 transcript。
- controller `AgentRun` 负责一次用户请求、系统事件或定时触发的总体调度。
- executor `AgentRun` 负责一个明确的子任务，必须有 `parent_run_id`，执行结束后上下文销毁，但记录持久保留。
- `AgentRun` 可以引用原企微 session、turn 和 transcript entry，但不默认把长期聊天窗口注入模型。
- 执行结果回写到原 session 的 transcript 和 audit；需要主动投递时通过通知系统或企业微信回复出口发送。

`agent_runs` 建议字段：

```text
agent_runs
- id
- parent_run_id
- session_id
- turn_id
- role: controller / executor
- status: pending / running / input_required / auth_required / completed / failed / canceled
- task_packet_json
- capability_scope_json
- model_key
- context_budget_json
- context_trace_ref
- result_ref
- trace_id
- started_at
- completed_at
- created_at
```

执行 Agent 上下文追溯建议字段：

```text
agent_run_context_traces
- id
- run_id
- trace_kind: input_projection / model_request / model_response / tool_schema / tool_call / observation / trim_record
- prompt_version
- model_key
- content_json
- content_hash
- redaction_status
- token_estimate
- created_at
```

追溯原则：

- 保存的是模型实际可见的上下文投影视图，而不是事后重新拼接的摘要。
- 大正文、网页原文、仓库内容和附件可以保存为 `content_ref`，但 trace 中必须保留 hash、来源、裁剪位置和模型可见片段。
- 如果因为安全策略脱敏，必须记录 `redaction_status` 和脱敏原因。
- controller 只接收 executor 的结构化 observation 和 artifact 引用，不直接吞入 executor 的完整过程上下文。

模型元数据建议：

```text
agent_models
- model_key
- provider
- model_name
- role: strong / normal / cheap
- context_window_tokens
- max_output_tokens
- supports_tool_call
- supports_json_schema
- supports_streaming
- input_cost_per_1k
- output_cost_per_1k
- latency_class
- reliability_score
- enabled
- updated_at
```

能力注册项至少包含：

```text
agent_capabilities
- capability_key
- target_type
- allowed_actions
- risk_level
- decision_policy
- confirmation_policy
- rollback_supported
- service_binding
- exposure_mode
- search_text
- input_schema
- output_schema
- supports_parallel
- requires_user_interaction
- enabled
```

联网信息获取 capability 建议：

| capability | 暴露模式 | 风险 | 默认决策 | 作用 |
| --- | --- | --- | --- | --- |
| `web.search` | `core` 或 `deferred` | `low` | `allow` | 根据关键词、时间、语言和站点约束返回候选网页 |
| `web.fetch_page` | `deferred` | `low` | `allow` | 获取指定 URL 的响应元数据和原始内容引用 |
| `web.extract_page` | `deferred` | `low` | `allow` | 抽取正文、标题、发布时间、作者、站点名和主要链接 |
| `web.browse_page` | `deferred` | `medium` | `prompt` | 使用受控浏览器处理 JS 渲染、交互页面或登录态页面 |
| `repo.search` | `core` 或 `deferred` | `low` | `allow` | 搜索 GitHub 等平台的参考仓库候选 |
| `repo.inspect_remote` | `deferred` | `low` | `allow` | 不克隆仓库，读取 README、目录树、license 和指定文件片段 |
| `repo.clone_reference` | `deferred` | `medium` | `prompt` | 浅克隆参考项目到受控 `references/` 目录 |

`repo.clone_reference` 的执行结果只写入参考资料目录和审计记录，不得自动触发依赖安装、构建、测试、部署或源码 import。若用户明确要求立即克隆指定公开仓库，Web 管理端可将该计划降为一次性 `allow`，但仍必须记录目标目录、远端 URL、commit、目录规范化结果和执行人。

能力暴露模式：

- `core`：模型默认可见，适合少量高频、低风险、基础查询和计划类能力。
- `deferred`：默认只进入搜索索引，模型需要先通过 `capability.search` 获取 schema 后才能执行。
- `hidden`：不对模型直接暴露，只能由后端策略或已确认计划内部调用。

风险分级建议：

- `low`：生成推荐、生成摘要、查询源目录、生成订阅建议。
- `medium`：新增订阅、调整标签、调整来源权重、创建低频提醒。
- `high`：批量停用来源、提高通知频率、修改通知接收目标、创建金融告警。
- `critical`：永久删除、暴露敏感配置、绕过访问限制。默认禁止或必须二次确认。

执行决策建议：

- `allow`：低风险查询、摘要草稿、推荐候选生成、只读证据检索。
- `prompt`：新增订阅、发送通知、创建规则、批量变更、读取完整历史归档。
- `forbidden`：绕过访问限制、泄露密钥、未授权通知目标、默认永久删除和未注册能力执行。

## 6. 阶段五：企业微信对话入口 Agent MVP 与 AI 源

目标是先建立可审计对话入口和受控执行底座，而不是直接堆叠具体智能功能。阶段五 P0 以企业微信自建应用接收消息 API 为默认入口，主动通知和自动设置变更后置。

实施内容：

1. 新增企业微信自建应用接收消息回调入口，完成 URL 验证、签名校验、AES 解密、XML 解析和消息标准化。
2. 新增 `external_accounts` 和 `agent_inbound_messages`，将企业微信应用用户与系统用户映射，并使用 `MsgId` 或回调签名组合作为幂等键。
3. 新增 Agent 核心领域对象：session、turn、transcript、执行结果和审计日志；计划、步骤和审批可在 P0 之后补齐。
4. 建立 `AgentSessionManager` 和 `AgentTurnRunner`，保证同一 session 内 active turn 串行执行，并支持取消、恢复和失败记录。
5. 实现最小只读 Runner：最近资讯、指定来源最新条目、摘要或简短问答。
6. 短回答可通过自建应用被动回复返回企业微信；长回答通过 `message/send` 异步发送，`message/send` 暂不作为主动通知系统。
7. 建立 `AgentCapabilityRegistry`，所有 Agent 可执行能力必须注册。
8. 建立 `AgentTool` 抽象，每个工具只能调用既有 service。
9. 建立能力暴露模式和 `capability.search`：少量核心能力常驻，其他能力延迟检索。
10. 建立计划生成、计划校验、影响评估、确认策略和执行器。
11. 建立 `allow`、`prompt`、`forbidden` 决策策略，风险等级只作为策略输入。
12. 建立 `AgentContextManager`、`ConversationMemoryProvider`、`MemoryProvider`、`ContextBuilder` 和 `MemorySnapshot`。
13. 建立企微短期聊天窗口、历史聊天原文查询、transcript 冷热归档索引和召回预算约束。
14. 为用户创建默认 AI 源 `messageFeed AI`。
15. 将日报、报告、执行结果等非即时对话内容写入 AI 源；企微普通聊天记录只进入 transcript，不进入订阅或推荐主页条目。
16. 在 Web 中展示 AI 源，与普通来源共用阅读状态、收藏、隐藏和详情页。
17. 接入 observability，记录 request id、trace id、模型调用、执行步骤、历史查询、记忆召回和错误链。

阶段五验收标准：

- 企业微信后台可以成功保存自建应用接收消息 URL 配置。
- 用户可以通过企业微信自建应用发送文本消息并收到系统回复。
- 重复回调不会重复创建 turn，`MsgId` 或回调签名幂等可通过数据库约束验证。
- 企业微信消息、Agent turn、transcript、audit、request id、trace id 和回复发送结果可以关联查询。
- P0 Runner 只能执行只读查询、摘要或问答，不执行订阅新增、删除、通知配置和金融告警变更。
- 用户可以通过 Web 或企业微信提交自然语言命令并得到结构化计划。
- Agent 可以创建 session 和 turn，并限制同一 session 同时只有一个 active turn。
- 低风险计划可以生成建议但不必立即执行。
- 中高风险计划必须等待用户确认。
- 能力可以按 `core`、`deferred`、`hidden` 暴露，延迟能力需要先检索再执行。
- Agent 可以写入一条非对话类 `messageFeed AI` 源内容。
- Agent 执行结果具备审计记录。
- Agent 可以生成一次冻结的用户画像记忆快照。
- Agent 可以自动注入同一企微长期 session 的最近聊天窗口，且不会重复注入当前 turn。
- Agent 可以通过 `conversation.query_history` 按关键词、时间和角色取回历史聊天原文或画像证据。
- 模型不能直接访问数据库写接口。

## 7. 阶段六：主动采集与内容理解 Agent

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
- 可以按关键词执行一次主动网络信息获取并生成 AI 源报告。
- 所有主动采集结果保留 URL、时间、hash 和抽取方式。

## 8. 阶段七：推荐、摘要与通知 Agent

目标是形成“内容理解 -> AI 源沉淀 -> 主动提醒”的闭环。

实施内容：

1. 建立持久化推荐候选池和推荐记录。
2. 建立 `interest_rules`、`feed_recommendations`、`recommendation_feedback`。
3. 使用阅读行为、来源权重、标签、语言、收藏、隐藏和停留时间形成基础评分。
4. 生成推荐原因，区分已订阅来源和未订阅候选来源。
5. 基于用户画像构建推荐、摘要和通知的长期记忆输入。
6. 支持日报、周报、专题摘要和热点事件分析。
7. 生成内容写入 `messageFeed AI` 源。
8. 支持企业微信、ntfy 和后续微信通道推送。
9. 建立通知冷却、免打扰时间、幂等键、失败重试和通知历史。
10. 用户可以用自然语言调整摘要范围、推送频率和通知偏好。

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
- 用户画像可以解释关键推荐、摘要选择和通知触发依据。

## 9. 阶段八：金融与跨领域分析 Agent

金融分析 Agent 使用独立专项计划维护，详见 `docs/financial-agent-plan.md`。

本总纲仅保留阶段八的集成目标：

- 将金融行情、财经资讯、主动网络信息获取和 AI 分析联动。
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

## 10. Web 产品形态

Web 侧应逐步形成以下入口：

```text
订阅
推荐
助理
messageFeed AI
来源管理
Agent 任务
我的偏好
设置
```

当前 Web 主入口要求：

- 顶部主导航优先展示“订阅 / 推荐 / 助理”，三者使用同一横向滑动模型。
- 订阅、推荐、助理三个 pane 需要同时挂载和加载内容，保证左右切换时内容已经存在。
- “助理”复用现有 Agent 任务工作台和后端能力，作为普通用户发起任务、查看执行进度和查看任务结果的入口。
- 开发者评测、基线运行和工程治理验证不展示在普通用户界面；相关能力保留在开发者验证或后端测试链路。

AI 源页面：

- 展示日报、周报、热点分析、主动网络信息获取、金融分析和 Agent 报告。
- 支持按生成类型筛选。
- 支持收藏、隐藏、阅读原文、查看依据。
- 展示输入来源、关联条目、模型、生成时间和推送状态。

Agent 任务页面：

- 展示自然语言任务发起入口。
- 执行进度紧邻发起任务区域展示。
- 展示真实用户任务历史；无用户任务时显示为空状态，不混入开发治理任务。
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

上下文与记忆页面：

- 展示当前 Agent 会话的上下文使用率、短期聊天窗口大小和最近索引更新时间。
- 展示 transcript 冷热归档索引、类型、关键词、关联计划和原文引用。
- 展示记忆召回记录，包括查询、召回来源、召回原因和使用位置。
- 支持查看用户画像标签的证据链。
- 支持用户删除、固定或拒绝长期记忆候选。

## 11. 安全、权限与治理约束

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
- 召回内容必须标注来源、时间和可信等级，且不得覆盖系统规则和权限策略。
- 历史聊天查询和归档索引必须保留 transcript 原文引用，高风险操作不得只依赖摘要、关键词或模型自行回忆执行。

## 12. Agent 评估与测试体系

Agent 不能只通过人工体验判断是否可用，应建立固定评测集、工具调用 trace、数据库状态断言、安全对抗样例和人工复核共同组成的 `Agent Eval Harness`。评估目标不是证明模型“看起来聪明”，而是证明系统能在订阅、推荐、摘要、主动采集、通知和金融分析边界内稳定、可解释、可回滚地完成任务。

架构评估维度：

1. 任务建模：用户请求是否能稳定落到 `Session / Turn / Intent / Plan / Step / Capability / Audit`。
2. 能力治理：能力是否全部注册，是否区分 `core`、`deferred`、`hidden`，未注册能力是否默认不可执行。
3. 权限决策：高风险操作是否稳定进入 `prompt`，禁止操作是否稳定进入 `forbidden`。
4. 上下文与记忆：画像是否有证据链，归档索引是否保留 transcript 原文引用，召回内容是否标注来源、时间和可信等级。
5. 失败恢复：工具失败、模型输出错误、用户取消、超时和重复提交后，session、turn、plan、step 和审计状态是否一致。

业务评测集应覆盖：

- 自然语言订阅管理：来源搜索、重复订阅避免、计划解释、确认策略。
- 推荐与用户画像：偏好提取、负反馈生效、画像证据链、推荐理由。
- AI 源生成：日报、周报、热点分析、来源健康报告和 Agent 执行报告。
- 主动网络采集：网页变化监控、关键词研究、正文抽取、去重和来源可信度。
- 通知与提醒：误报、漏报、冷却时间、通知目标权限和发送审计。
- 金融分析：规则触发、数据源标注、风险提示和避免确定性投资建议。
- 上下文与记忆：归档召回准确率、摘要漂移、长期任务连续性和证据覆盖率。
- 安全对抗：prompt injection、敏感信息泄露、越权工具调用、未授权通知目标和默认永久删除。

建议评测用例结构：

```text
agent_eval_cases
- user_input
- initial_user_profile
- existing_sources
- existing_items
- available_capabilities
- expected_intent
- expected_plan
- expected_policy_decision
- expected_tool_calls
- expected_state_change
- expected_ai_item
- forbidden_behaviors
- scoring_rules
```

评测执行链路：

```text
EvalCase
  -> 构造隔离数据库状态和能力清单
  -> 运行 Agent turn
  -> 捕获 transcript、plan、tool calls、state diff、AI item 和 audit logs
  -> 自动规则评分
  -> LLM-as-judge 或人工复核仅用于文本质量、事实一致性和解释质量
  -> 生成 EvalRun 报告并参与回归门禁
```

核心指标：

- 任务成功率。
- 意图解析准确率。
- 工具选择准确率。
- 权限决策正确率。
- 高风险操作确认率。
- 越权操作拦截率。
- 事实引用完整率。
- 召回准确率。
- 摘要事实一致性。
- 用户画像证据覆盖率。
- 通知误报率和漏报率。
- 单任务成本和耗时。
- 回归通过率。

可参考的成熟工具与方法：

- `OpenAI Evals`：结构化评测和自动评分。
- `LangSmith`：Agent trace、工具调用、数据集和人工标注。
- `Langfuse`：生产环境 LLM observability、成本、trace、评分和反馈。
- `Braintrust`：模型、提示词和工具链版本的回归评测。
- `Promptfoo`：轻量 prompt 与工具输出回归测试。
- `RAGAS`、`DeepEval`：检索、召回、引用和事实一致性评估。
- `AgentBench`、`ToolBench`、`API-Bank`、`tau-bench`、`GAIA`：参考多轮任务、工具调用和复杂信息搜索评测思路。

本项目第一版不必引入完整外部评测平台，但应先沉淀可复用的 `agent_eval_cases`、`agent_eval_runs`、`agent_eval_results` 和命令行评测入口。后续再根据成本和团队习惯接入 Langfuse、LangSmith、Braintrust 或 Promptfoo。

## 13. 推荐落地顺序

在进入本计划前，仍应先完成以下基础事项：

1. 收尾阶段二 Web 闭环：已读、收藏、隐藏、筛选、分页和阅读模式偏好。
2. 补齐 `api/openapi.yaml` 中已实现接口。
3. 完成阶段三 Compose 观测验收。

随后按以下顺序推进：

1. 企业微信自建应用接收消息回调入口：URL 验证、签名校验、AES 解密、消息标准化和 `MsgId` 幂等。
2. 外部账号映射和 Agent 会话基础表：`external_accounts`、`agent_inbound_messages`、`agent_sessions`、`agent_turns`、`agent_transcript_entries`、`agent_audit_logs`。
3. 企业微信 Agent P0 Runner：只读查询最近资讯、指定来源最新条目、摘要或简短问答，并通过被动回复或 `message/send` 返回。
4. 企业微信自建应用基础适配：`access_token` 缓存、`message/send` 文本发送、可见范围错误记录；该能力在 P0 只作为回复出口，后续再纳入通知基础。
5. `messageFeed AI` 内部源和 AI 生成内容入库。
6. Agent 能力注册、结构化计划、执行器、审计和 `allow`、`prompt`、`forbidden` 策略。
7. `AgentContextManager`、`ConversationMemoryProvider`、`MemoryProvider`、冻结记忆快照和基础用户画像读取。
8. 企微短期聊天窗口、历史聊天原文查询、transcript 冷热归档索引和回忆工具。
9. 订阅管理 Agent：源搜索、源推荐、订阅、停用、标签和权重调整。
10. 主动网络采集：静态网页抽取、网页变化监控、搜索型采集。
11. 阅读行为事件和基础用户画像。
12. 推荐候选池、推荐原因和反馈闭环。
13. 日报、周报、热点分析和 AI 源内容生成。
14. 主动通知系统：企业微信自建应用消息、可选智能机器人主动消息能力、`ntfy`、通知审计、冷却和幂等。
15. 金融监控和跨领域分析。
16. 工程化增强、集成测试、E2E 测试和 Dashboard 迭代。

## 14. 最小可验收闭环

最小 Agent 闭环建议定义为：

```text
用户通过企业微信自建应用输入：
“最近 Go 和 AI infra 有哪些值得看的内容？”

系统处理：
- 校验企业微信自建应用回调签名并解密消息。
- 使用 `MsgId` 或回调签名幂等落库为 inbound message。
- 创建或恢复 Agent session，并创建本次 turn。
- 读取最近条目、来源和必要的用户显式偏好。
- 生成简短回答并写入 transcript、audit 和 trace。
- 通过被动回复或 `message/send` 返回企业微信。

- 将非即时对话类执行报告写入 `messageFeed AI`。
- 对需要订阅、提醒、金融告警或画像写入的请求，返回待确认计划，不在 P0 自动执行。
```

该闭环完成后，项目将先具备“用户通过企业微信自建应用提问，系统可审计地回答”的 Agent 入口能力。普通来源负责稳定输入，`messageFeed AI` 负责沉淀分析和执行结果；智能机器人、主动通知、摘要推送、金融告警和更复杂的执行能力在后续阶段统一接入策略、审计和通知模型。

## 15. 当前实现对照

本节记录截至 2026-06-26 的实际实现状态，用于和主进度台账 `docs/implementation.md` 保持一致。

### 15.1 已落地能力

- Web 侧已有 Agent 任务入口、任务工作台、计划进度页、审批页、证据和回放相关页面。
- 企业微信侧已有自建应用 callback、OAuth、文本消息发送和模板卡片发送基础能力。
- 后端已有 Agent session、turn、run、plan、approval、scheduled task、eval、recovery、audit 等基础能力。
- 后端任务聚合已暴露多项企业微信双端治理摘要，包括进度卡片、模板渲染、模板发送、模板集成、试点指标、真实交互自动化等。
- 后端已新增 `wechat_web_progress_link` 聚合摘要，覆盖进度地址、地址来源、企业微信投递通道、模板状态、fallback 状态、浏览器目标和审计引用。
- 前端已声明并展示 `wechat_web_progress_link`，Web 任务工作台可以查看企业微信 Web 进度地址投递摘要和地址链接。
- 企业微信进度通知已接入真实模板卡片投递，模板失败时降级为文本 fallback，且聚合摘要读取真实审计事件。
- 企业微信最终结果汇报已接入模板卡片入口加完整文本结果的组合投递，模板失败时文本仍可发送；`wechat_final_report` 聚合摘要和 Web 工作台已展示真实投递状态。
- Web 发起任务完成后已可向用户启用的企业微信绑定投递最终报告，最终报告包含 Web 浏览器可打开的计划进度地址，并写入真实投递审计。
- 企业微信 OAuth / external account / Web session 的访问关系已完成核对：OAuth URL 生成要求 Web 登录用户；callback 使用 state 中的 `user_id` 绑定 external account 并创建 Web session；`/api/v1/auth/me` 可返回当前用户 bindings；disabled binding 会被企业微信 external account 解析拒绝。
- Web 进度页访问不依赖 URL 携带外部账号凭证，最终数据访问由 Web session 用户与 Agent 任务 owner 的归属校验决定；现有服务测试已覆盖未登录、跨用户计划进度、跨用户计划详情和跨用户调度任务进度拒绝。
- 最近一轮完整验证已通过 `go test ./...`、`go vet ./...`、`npm --prefix web run test`、`npm --prefix web run type-check` 和 `npm --prefix web run build`。

### 15.2 当前缺口

- Agent 能力、架构、上下文记忆、执行策略和评测系统已有基础实现，但尚未有证据证明本设计文档中的全部能力均完整完成。

### 15.3 架构治理要求

当前大文件规模需要作为持续治理对象：

| 文件 | 当前行数 | 治理要求 |
| --- | ---: | --- |
| `internal/service/agent_session_service.go` | 4446 | 已迁出任务列表聚合响应 DTO、任务摘要 DTO、转换函数、任务摘要状态 helper、基础治理审计快照 recorder、发布执行/日报闭环 recorder、发布窗口/外部监控 recorder、生产发布/上线交接 recorder、运行态参数/反馈闭环 recorder、放量阶段/运维处置 recorder 和审批执行/工单 SLA recorder；继续拆分剩余审计 recorder、进度构造和服务编排逻辑 |
| `internal/service/agent_workflow_governance.go` | 739 | 已明显低于此前 5000 行级别；本轮已迁出所有 `buildAgent*` 纯 builder，剩余内容主要为 admission、质量摘要、通用 helper 和 plan/domain 转换辅助 |
| `web/src/views/AgentPlanView.vue` | 3680 | 已迁出企业微信最终汇报和 Web 进度地址两个小组件；仍需继续拆分任务摘要组件、组合式状态逻辑和展示面板 |

上述文件达到数千行不应被视为理想的企业级终态。后续实现必须优先新增职责明确的小文件或组件，并在必要时逐步迁出既有逻辑。

### 15.4 当前活动文档

上一活动文档 `docs/nowdoit/agent-wechat-web-progress-link-delivery-plan.md` 已完成并归档到 `docs/nowdoit/archive/agent-wechat-web-progress-link-delivery-plan-implemented-2026-06-25.md`。

上一活动文档 `docs/nowdoit/agent-wechat-final-result-report-delivery-plan.md` 已完成并归档到 `docs/nowdoit/archive/agent-wechat-final-result-report-delivery-plan-implemented-2026-06-25.md`。

上一活动文档 `docs/nowdoit/agent-web-progress-permission-binding-governance-plan.md` 已完成并归档到 `docs/nowdoit/archive/agent-web-progress-permission-binding-governance-plan-implemented-2026-06-26.md`。完成项为：

1. 已完成 Agent 进度和计划详情 API 的用户归属校验测试。
2. 已完成企业微信 OAuth 和 external account 绑定对 Web 进度页访问的支持状态梳理。
3. 已补齐 OAuth state 归属绑定、当前用户 bindings 返回和 disabled binding 拒绝测试。
4. 已拆分前端 Agent 工作台中进度地址和最终汇报摘要展示逻辑，新增 `web/src/components/agent/AgentWeChatFinalReportSummary.vue` 和 `web/src/components/agent/AgentWeChatWebProgressLinkSummary.vue`；`npm --prefix web run type-check`、`npm --prefix web run build` 和 `npm --prefix web run test` 已通过。

上一活动文档 `docs/nowdoit/agent-session-service-aggregation-modularization-plan.md` 已完成并归档到 `docs/nowdoit/archive/agent-session-service-aggregation-modularization-plan-implemented-2026-06-26.md`。完成项为：

1. 已梳理 `agent_session_service.go` 中任务聚合响应类型和 builder 可迁移边界。
2. 已新增 `internal/service/agent_task_list_responses.go`，迁出任务列表聚合响应 DTO、任务摘要 DTO、转换函数和任务摘要状态 helper。
3. 已保持 `ListTasks` 的 JSON 字段、审计事件和前端 API 语义不变；`agent_session_service.go` 当前降至 5936 行。
4. `go test ./...` 和 `go vet ./...` 已通过。

上一活动文档 `docs/nowdoit/agent-workflow-governance-builder-modularization-plan.md` 已完成并归档到 `docs/nowdoit/archive/agent-workflow-governance-builder-modularization-plan-implemented-2026-06-26.md`。完成项为：

1. 已梳理 `agent_workflow_governance.go` 中低耦合 builder 群组。
2. 已新增 `internal/service/agent_workflow_metadata_builders.go`、`internal/service/agent_workflow_foundation_builders.go` 和 `internal/service/agent_workflow_wechat_builders.go`，迁出 metadata builder、基础聚合 builder 和企业微信组件 builder 群组。
3. 已保持 `ListTasks` 聚合结果、JSON 字段、企业微信按钮 key、fallback 文案和审计语义不变；`agent_workflow_governance.go` 当前降至 3717 行。
4. `go test ./...` 和 `go vet ./...` 已通过。

上一活动文档 `docs/nowdoit/agent-workflow-governance-release-ops-builder-modularization-plan.md` 已完成并归档到 `docs/nowdoit/archive/agent-workflow-governance-release-ops-builder-modularization-plan-implemented-2026-06-26.md`。已新增 `internal/service/agent_workflow_release_ops_builders.go`，迁出发布、运维、灰度、告警通道、上线演练、企业微信原生按钮联调、发布执行、审批、日报、监控、按钮回调闭环、发布窗口、外部监控、按钮直控、企业微信验收、生产发布、上线交接、运行态参数、监控回读、放量推荐、企业微信用户反馈、运营面板和异常自动汇报相关 50 个纯 builder，并迁入 1 个企业微信最终汇报审计读取 helper；`agent_workflow_governance.go` 当前降至 2127 行。

上一活动文档 `docs/nowdoit/agent-workflow-governance-ops-handling-builder-modularization-plan.md` 已完成并归档到 `docs/nowdoit/archive/agent-workflow-governance-ops-handling-builder-modularization-plan-implemented-2026-06-26.md`。已新增独立 `internal/service/agent_workflow_ops_handling_builders.go`，迁出 43 个运维处置、审批执行、证据闭环、双端进度和真实交互相关纯 builder；剩余 SLA 摘要和任务报表 builder 已迁入 `agent_workflow_foundation_builders.go`。`agent_workflow_governance.go` 当前降至 739 行，已不再承接 `buildAgent*` 纯 builder。

当前活动文档为 `docs/nowdoit/agent-minimal-closed-loop-delivery-plan.md`。本轮已补齐 Web 发起任务完成后向用户已绑定企业微信账号发送最终报告，最终报告包含 Web 浏览器可打开的进度地址并写入审计证据；`go test ./...` 和 `go vet ./...` 已通过。`docs/nowdoit/agent-session-service-snapshot-recorder-modularization-plan.md` 已写入的 4.8 recorder 拆分计划暂不归档、不删除，后续可恢复执行。
