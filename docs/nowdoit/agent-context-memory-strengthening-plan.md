# Agent 上下文记忆加强计划

状态：待审核
创建时间：2026-06-29
适用范围：Agent 短期记忆、长期记忆、上下文选择、证据投影与多轮追问

## 1. 目标

本计划用于加强当前 Agent 的上下文管理能力，使系统从“固定最近 12 条对话 + 显式用户资料 + 工具观察摘要”的 P0 实现，升级为“短期活动上下文稳定连续、长期记忆可控沉淀、历史召回由主 Agent 理解驱动、证据可追溯”的实现。

本轮不直接扩大 Agent 权限，不自动执行高风险写操作，不实现流式思考展示，不删除既有 transcript、run、observation 或 artifact 数据。

## 2. 当前核实结论

当前实现已经具备以下基础：

1. 企微和 Web 用户消息会写入 `agent_transcript_entries`。
2. 子 Agent 执行阶段默认加载同 session、同 user、当前 turn 之前的最近 12 条 `user/assistant` transcript。
3. `AuthService.BuildAgentUserContext` 会读取 `user_profiles` 并生成用户上下文，包含语言、时区、回复风格、关注主题、屏蔽主题、关注市场、关注标的、风险偏好、免打扰时间和备注。
4. 模型主动调用 `conversation.query_history` 时，可以查询历史 transcript，并写入 `agent_recall_events`。
5. 每个子 Agent 工具执行会记录 executor run、context trace、observation 和 artifact，并回填到 plan step。

当前主要不足：

1. 主 Agent 规划阶段未自动获得最近对话、当前计划、上一轮回答和关键工具结果。
2. 短期记忆固定为 12 条，没有上下文预算管理，也没有保护区。
3. 历史召回的自动判断被关闭，长期记忆不会默认参与上下文选择。
4. `agent_transcript_archive_index` 已存在，但分类仍是后端关键词规则，不符合“意图和记忆语义交给模型”的方向。
5. 多轮追问只携带 plan 摘要、step 摘要和证据引用，缺少上一轮完整回答、observation 详情和 artifact 内容投影。
6. artifact 当前主要保存摘要和引用，不能完整还原工具原始输出。
7. 用户偏好仅有显式资料读取，尚未形成“候选记忆 -> 证据链 -> 用户确认或置信提升 -> 长期记忆”的闭环。

## 3. 设计原则

1. 短期记忆服务当前任务连续性，重点保证最近对话、当前目标、当前计划、最近回答和关键工具结果不断裂。
2. 长期记忆服务稳定偏好和可追溯事实，不把全部历史直接塞入 prompt。
3. 主 Agent 负责理解是否需要近期上下文、历史召回、任务记忆、用户画像和内容证据。
4. 后端只负责结构化事实、权限校验、预算控制、数据查询、审计和持久化。
5. 用户可见回复不在业务代码中硬编码，仍由模型基于集中 prompt 生成。
6. 长期偏好不得由单次行为静默写入；高影响画像变更必须可确认、可解释、可撤销。
7. 任何上下文裁剪只能作用于模型可见投影视图，不能替代或删除原始 transcript、tool result、audit 和 artifact。

## 4. 短期记忆改造

### 4.1 活动上下文包

新增或改造统一的活动上下文结构，建议包含：

```text
ContextBundle
- system_blocks
- recent_messages
- budget_profile
- active_goal
- active_plan
- previous_assistant_answer
- key_observations
- key_artifacts
- user_constraints
- memory_blocks
- budget_report
```

职责：

1. 统一描述本轮模型实际可见上下文。
2. 保留每个 block 的来源、证据引用、更新时间、可信等级和 token 估算。
3. 支持 Web 端展示“本轮为什么带了这些上下文”。
4. 支持按主 Agent、子 Agent 和最终综合阶段区分上下文预算 profile。

### 4.2 短期保护区

每轮应优先保留：

1. 系统规则和能力边界。
2. 当前用户原始消息。
3. 当前主 Agent PlanSpec 和未完成步骤。
4. 最近对话按语义单元组成热窗口，按 token 预算、任务复杂度和相关性动态选择，不再以固定条数作为保留策略。
5. 上一轮 assistant 完整回复。
6. 最近一次关键工具 observation。
7. 最近一次关键 artifact 摘要和来源。
8. 用户刚明确表达的限制、偏好、否定反馈或授权。

### 4.3 动态预算

将固定 `Limit: 12` 改为 token 预算驱动的语义单元选择：

1. 最近对话不再按 8、12、20 条等固定数量决定热窗口，而是按 `ContextBudgetProfile` 的 token 上限、任务复杂度、相关性和保护区优先级选择。
2. 语义单元应整体纳入或整体裁剪，不能对原始消息、工具结果或证据片段做任意硬截断。
3. 推荐语义单元包括：单轮 turn、连续用户/assistant 消息对、工具调用与工具结果对、history recall 结果块、observation/artifact 投影块、plan/step 投影块、用户约束块。
4. 单个语义单元超过当前预算时，应生成结构化投影视图或摘要块，并保留 `canonical_ref`、原始 fact ref、裁剪原因和可回表路径；不得改写、截断或删除原始事实。
5. 超出热窗口或模型预算时，主 Agent 应生成 `history_query_plan`，通过 `conversation.query_history` 回顾相关原文，而不是简单丢弃较早消息后凭近期上下文回答。
6. 历史查询结果进入本轮 ContextBundle 时必须保留 transcript entry 引用、召回原因、查询参数和时间边界。
7. 如果召回结果仍然过大，只对模型可见投影视图做结构化分页、摘要或分批回看；原始 transcript 不被截断、改写或删除。
8. 工具结果和 artifact 使用投影视图，保留完整原始引用。

### 4.4 主 Agent 与子 Agent 分层上下文预算

主 Agent 与子 Agent 的上下文预算必须分开计算。总任务消耗可以超过单次模型窗口，但任一模型调用都必须在自己的预算内完成。本轮上下文管理改造应先落地预算模型和投影边界，不在同一阶段强制实现完整 SubAgentTask DAG 调度。

建议引入 `ContextBudgetProfile`，至少包含：

1. `main_planning`：主 Agent 规划阶段，重点保留对话连续性、当前目标、用户约束、活动计划和必要历史召回线索。
2. `main_evaluation`：主 Agent 证据充分性评估阶段，重点保留 PlanSpec、evidence requirements、子 Agent 结构化结果、gaps 和必要回表事实。
3. `subagent_search`：搜索类子 Agent，重点保留任务边界、时间范围、来源范围、少量相关对话和网页/订阅源证据。
4. `subagent_history_recall`：历史召回类子 Agent，重点保留近期约束、历史候选、回表 transcript 原文片段和 recall reason。
5. `subagent_analysis`：分析类子 Agent，重点保留多源事实、冲突点、用户偏好和输出契约。
6. `final_synthesis`：最终综合阶段，重点保留最近对话、子 Agent 结构化结果、稳定记忆、关键 evidence refs 和输出预留。

主 Agent 预算建议：

```text
总预算：64k
最近对话热窗口：32k 默认，复杂连续任务 40k，硬上限 44k
稳定记忆：4k-6k
历史召回：6k-10k
子 Agent 结果摘要：8k-12k
计划和评估结构：4k-6k
输出和安全余量：6k-8k
```

子 Agent 预算建议：

```text
搜索子 Agent：
  最近对话 0k-4k
  任务摘要和约束 2k-4k
  搜索和网页证据 32k-44k
  输出预留 6k-8k

历史召回子 Agent：
  最近对话 4k-8k
  历史候选和回表事实 24k-36k
  输出预留 4k-6k

分析子 Agent：
  最近对话 2k-6k
  多源证据 32k-42k
  用户偏好 2k-4k
  输出预留 8k

最终综合阶段：
  最近对话 16k-24k
  子 Agent 结果 20k-28k
  稳定记忆 4k
  输出预留 8k
```

预算裁剪原则：

1. 子 Agent 不继承完整最近对话热窗口，只接收与子任务相关的用户约束、时间范围、来源范围、上游摘要和证据引用。
2. 搜索类子 Agent 优先保留网页、订阅源和来源证据，不优先保留完整会话。
3. 历史召回类子 Agent 优先保留回表后的 transcript 原文片段和召回边界。
4. 分析类子 Agent 优先保留多源事实、冲突点和用户偏好。
5. 最终综合阶段默认只读取子 Agent 结构化结果；证据不足或争议较大时，才按 `canonical_ref` 回表读取原始事实。
6. `budget_report` 必须记录预算 profile、block 预算、实际 token 估算、裁剪状态、裁剪原因、保留原因、输出预留和安全余量。
7. 上下文预算 trace 应写入 `agent_run_context_traces` 或等价审计记录，供 Web 展示“本轮为什么带这些上下文”。

## 5. 长期记忆改造

### 5.1 长期记忆相关结构

长期记忆不应理解为单独一张“长期记忆表”。本项目应把原始事实、归档索引、候选记忆、稳定记忆和用户画像分清职责，并在调用时保持证据链完整。

1. 原始事实层：transcript、tool input、tool output、run trace、audit、artifact。该层保存事实本体。
2. 归档索引层：冷热状态、memory kind、重要度、实体、关键词、证据引用。该层不是独立事实，只能结合原始事实使用，用于快速定位证据。
3. 候选记忆层：模型从对话和行为证据中提取的待审核偏好、事实、决策和任务习惯。
4. 稳定记忆层：经过用户确认、明确表达或多证据支持后形成的用户程序偏好结论。
5. 用户画像层：显式偏好、长期兴趣、负反馈、风险偏好、回复风格和通知偏好。

### 5.2 原始事实与归档索引

原始事实和归档索引是一组绑定结构，不是两个互相替代的记忆来源。

原始事实回答“当时实际发生了什么”。例如：

1. 用户原始消息。
2. assistant 最终回复。
3. 工具调用参数。
4. 工具返回结果。
5. 网页抓取内容。
6. observation、artifact、audit log、plan 和 step 状态。

归档索引回答“后续如何快速找到这些原始事实”。例如：

1. fact 类型。
2. fact id。
3. memory kind。
4. topics、keywords、entities。
5. importance。
6. archive status。
7. content hash。
8. indexed_at、last_accessed_at、access_count。

检索时必须先通过归档索引获得候选 fact ref，再回表读取原始事实。模型最终看到的证据应来自原始事实片段和引用，而不是只来自索引字段。

目标查询流程：

```text
主 Agent 生成 history_query_plan
  -> 查询归档索引
  -> 得到候选 fact refs
  -> 按相关性、重要度、时间、可信度排序
  -> 根据 fact_type 回表读取原始事实
  -> 生成本轮 ContextBundle 的证据投影视图
  -> 写入 recall event
```

### 5.3 当前原始事实编号现状

当前实现采用“各原始事实表各自编号”的方式：

1. `agent_transcript_entries.id`：用户消息、assistant 回复、system/tool transcript。
2. `agent_turns.id`：一轮用户输入处理。
3. `agent_runs.id`：controller run 或 executor run。
4. `agent_run_context_traces.id`：run 的上下文 trace。
5. `agent_observations.id`：工具观察。
6. `agent_artifacts.id`：工具产物摘要。
7. `agent_plans.id` 和 `agent_plan_steps.id`：计划和步骤。

当前 evidence ref 通过字符串拼接引用事实，例如：

```text
agent_transcript_entry:123
agent_plan:5
agent_turn:59
agent_plan_step:6
agent_run:12
agent_observation:31
agent_observations/31
agent_artifact:41
agent_artifacts/41
item:3595
web_search:https://example.com/path
```

该机制对单表追踪和执行链路复盘基本可用，但不利于后续统一检索：

1. 不同表的自增 id 会重复，必须依赖字符串前缀判断事实类型。
2. evidence ref 同时存在冒号和路径两种写法，例如 `agent_observation:1` 和 `agent_observations/1`。
3. `artifact_refs_json`、`source_refs_json` 等 JSON 引用没有数据库外键约束。
4. 当前归档索引主要绑定 transcript，不能统一索引 artifact、observation、web snapshot、item、plan 等事实。
5. `web_search:URL` 使用 URL 作为引用，不如 `web_snapshot:id` 稳定。

### 5.4 统一事实引用建议

后续不替换现有主键，而是在现有主键之上建立统一事实引用规范。

建议规范：

```text
fact_type:fact_id
```

示例：

```text
transcript:123
turn:59
plan:5
plan_step:6
run:12
observation:31
artifact:41
item:3595
web_snapshot:88
```

建议新增或改造通用事实索引：

```text
agent_fact_archive_index
- fact_type
- fact_id
- canonical_ref
- user_id
- session_id
- turn_id
- memory_kind
- topics
- keywords
- entities
- importance
- confidence
- content_hash
- index_model
- index_prompt_version
- index_status
- indexed_at
- last_accessed_at
- access_count
```

其中 `canonical_ref` 是后续统一对外展示和模型证据引用的稳定格式。旧的 `agent_transcript_entry:123`、`agent_observations/31` 等引用可以保留兼容，但新流程应优先写入统一 `canonical_ref`。

### 5.5 索引字段选择

关键词、主题、实体和重要度不应由后端硬编码词表决定。推荐流程：

```text
原始事实写入
  -> 创建索引任务
  -> 模型或轻量抽取器输出结构化索引字段
  -> 后端做枚举、长度、数量、证据引用和敏感信息校验
  -> 写入归档索引
```

模型输出建议：

```json
{
  "memory_kind": "preference",
  "topics": ["市场分析", "回复风格"],
  "keywords": ["结论", "依据", "风险", "不要执行过程"],
  "entities": ["港股", "美股", "A股"],
  "time_refs": [],
  "importance": 82,
  "should_recall_for": ["市场分析", "多轮追问", "回复格式"],
  "summary_for_index": "用户要求市场分析回复直接给结论、依据和风险，不展示执行过程。",
  "confidence": 0.91
}
```

后端只做工程校验和标准化：

1. 去空值、去重。
2. 限制关键词数量和单个关键词长度。
3. 校验 `memory_kind` 枚举。
4. 校验 `importance` 和 `confidence` 范围。
5. 绑定 `fact_type`、`fact_id` 和 `canonical_ref`。
6. 记录模型版本、prompt 版本和 content hash。
7. 敏感信息脱敏。
8. 失败时标记 `index_status=failed`，不影响原始事实保存。

检索时应多路召回：

1. 结构化过滤：user_id、session_id、memory_kind、时间范围。
2. 关键词召回：topics、keywords、entities。
3. 全文召回：原始事实全文索引或 trigram。
4. 语义召回：后续可加入 embedding。
5. 排序：相关性、重要度、时间新鲜度、访问频次和可信度。
6. 回表：根据 `canonical_ref` 读取原始事实。

索引字段是派生数据，允许重建。重建索引不得改写原始事实。

### 5.6 候选记忆

候选记忆是从原始事实中提取出来、可能值得沉淀为稳定记忆的待审核结论。它不是原始事实，不是归档索引，也还不是稳定记忆。

候选的作用是防止系统把每一句话都直接变成长期记忆。候选是缓冲层、审核层和置信度层。

候选应包含：

1. 候选内容。
2. 类型：回复风格、任务习惯、内容偏好、风险偏好、禁忌项等。
3. 证据引用：transcript、plan、artifact、observation。
4. 置信度。
5. 是否需要用户确认。
6. 风险等级。
7. 状态：pending、approved、rejected、promoted、expired。

示例：

原始事实：

```text
用户说：“以后回复别给我一堆执行过程，直接说结论、依据和风险。”
```

归档索引：

```text
memory_kind=preference
keywords=["回复风格", "执行过程", "结论", "依据", "风险"]
canonical_ref=transcript:123
```

候选记忆：

```text
用户偏好最终回复直接呈现结论、依据和风险，不展示执行过程细节。
```

稳定记忆：

```text
给该用户回复市场分析类任务时，默认输出结论、依据和风险，不展示执行治理细节。
```

### 5.7 稳定记忆

稳定记忆是用户的程序偏好结论，不是原始事实，也不是索引摘要。

稳定记忆用于后续 Agent 默认遵循，例如：

1. 回复格式偏好。
2. 分析任务默认关注维度。
3. 禁止展示的内容类型。
4. 通知偏好。
5. 风险偏好。
6. 工具使用或证据展示习惯。

稳定记忆必须具备：

1. 内容。
2. 类型。
3. 证据引用。
4. 置信度。
5. 是否用户确认。
6. 更新时间。
7. 可编辑、可撤销状态。

高影响稳定记忆必须经过用户确认；低风险稳定记忆也必须保留证据链和撤销能力。

### 5.8 模型驱动的记忆候选

后续不再由后端关键词规则判断 transcript 的记忆类型。应由模型基于集中 prompt 输出结构化候选：

```json
{
  "memory_candidates": [
    {
      "kind": "preference|task|fact|decision|casual|unknown",
      "content": "",
      "evidence_refs": [],
      "confidence": 0.0,
      "requires_confirmation": false,
      "reason": ""
    }
  ]
}
```

后端只校验：

1. kind 是否合法。
2. evidence_refs 是否存在。
3. confidence 是否在范围内。
4. 是否涉及高风险画像写入。
5. 是否需要用户确认。

### 5.9 用户偏好沉淀

显式偏好处理：

1. 用户明确说“以后默认短回复”“不要再推荐某类内容”等，可生成候选记忆。
2. 低风险回复风格类偏好可以进入待应用状态。
3. 影响推荐、通知、金融风险偏好、订阅策略的偏好必须进入确认状态。

隐式偏好处理：

1. 阅读、收藏、隐藏、不感兴趣、点击原文等行为只作为证据。
2. 多次证据聚合后才生成候选。
3. 不能由单次行为直接写入长期偏好。

## 6. 主要调用链

目标调用链：

```text
用户消息
  -> 写入 transcript
  -> 主 Agent 构建规划上下文
  -> 主 Agent 输出 PlanSpec、上下文需求、历史召回计划、记忆需求
  -> 后端校验权限、预算、能力范围
  -> ContextBuilder 构造 ContextBundle
  -> 子 Agent 基于 ContextBundle 和 capability scope 执行工具
  -> 工具结果写入 run、trace、observation、artifact
  -> 主 Agent 评估证据是否充分
  -> 不充分则按预算重规划或继续召回
  -> 生成最终回复
  -> 生成记忆候选
  -> 需要确认的候选进入确认流程
  -> 稳定记忆进入后续可默认遵循的用户程序偏好层
```

## 7. 拟修改文件和职责

### 7.1 Agent 核心层

`internal/agent/context.go`

职责：

1. 扩展 `ContextSnapshot` 或引入 `ContextBundle`。
2. 支持短期保护区、动态对话窗口、memory block 和预算报告。
3. 删除或废弃当前 `ClassifyHistoryNeed`、`ShouldQueryConversationHistory` 的固定返回逻辑。
4. 不再用后端关键词判断历史需求。

`internal/agent/runner.go`

职责：

1. 接收 ContextBundle。
2. 将最近对话、用户上下文、计划上下文、关键工具结果按预算组装成模型消息。
3. 保留同 turn 工具结果进入模型的现有机制。
4. 记录实际模型可见上下文投影视图。

`internal/agent/planner.go`

职责：

1. 扩展 PlanSpec，增加上下文需求字段。
2. 支持 `needs_recent_context`、`needs_history_recall`、`history_query_plan`、`required_memory_types`、`expected_evidence_scope`。
3. 后端 planner 只做结构校验和能力校验，不做关键词意图判断。

### 7.2 Service 编排层

`internal/service/agent_main_planner.go`

职责：

1. 主 Agent 规划请求加入必要的短期上下文摘要或投影视图。
2. 模型输出上下文需求和历史召回计划。
3. 规划失败、格式修复和模型重试仍走统一 prompt。

`internal/service/agent_turn_pipeline.go`

职责：

1. 将主 Agent 的上下文需求传递给 runner。
2. 在证据不足时支持主 Agent 评估后继续召回或重规划。
3. 保证 plan step、observation、artifact 绑定完整。

`internal/service/agent_runtime_adapters.go`

职责：

1. 保留 `conversation.query_history` 作为受控工具。
2. 支持模型传入的 history query plan。
3. 移除替模型做语义判断的关键词分支。
4. 工具参数清洗、权限边界和预算限制仍保留。

`internal/service/agent_multiturn_flow.go`

职责：

1. 多轮追问 payload 增加上一轮 assistant 完整回复。
2. 增加 source plan 的关键 observation、artifact 摘要、来源 URL、抓取时间和结构化结果。
3. 对证据不足的追问，引导主 Agent 重新召回，而不是仅基于 plan summary 保守回答。

`internal/service/agent_run_recording_executor.go`

职责：

1. 保留现有 run、trace、observation、artifact 记录。
2. 增加完整工具输出的可追溯引用。
3. artifact summary 仍可裁剪，但必须保留完整内容可查引用或原始数据定位方式。

`internal/service/agent_main_prompts.go`

职责：

1. 集中管理上下文选择、历史召回、记忆候选、偏好确认和回复风格相关 prompt。
2. 禁止业务流程代码中散落用户可见回复文案。
3. 增加索引字段抽取 prompt。
4. 增加候选记忆抽取 prompt。
5. 增加稳定记忆提升和用户确认 prompt。
6. 所有索引、候选、稳定记忆的模型输出均要求结构化 JSON。

### 7.3 Repository 和 Domain 层

`internal/domain/agent_channel.go`

职责：

1. 补齐 transcript archive、memory kind、recall event 相关领域字段时保持类型稳定。
2. 保留现有 transcript 相关类型。
3. 如果引入候选记忆对象、稳定记忆对象或通用 fact index，可优先新增独立 domain 文件，避免继续扩大单文件。

`internal/repository/agent_repository.go`

职责：

1. 保留 transcript 原文和现有 archive index。
2. 将当前关键词式 `classifyTranscriptMemory` 收敛为临时 fallback。
3. 后续由模型生成的结构化索引字段更新 archive index。
4. 增加统一 `canonical_ref` 的解析、标准化和回表读取能力。
5. 支持从通用 fact index 查询 transcript、artifact、observation、web snapshot、item、plan 和 step 等事实。

可能新增迁移：

1. `agent_fact_archive_index`：保存通用事实索引、统一 canonical ref、模型抽取字段、索引状态和访问统计。
2. `agent_memory_candidates`：保存模型提取的候选记忆、证据、置信度、确认状态。
3. `agent_memory_blocks`：保存稳定用户程序偏好结论。
4. `agent_memory_events`：保存候选生成、确认、拒绝、撤销和更新事件。

是否新增表需要审核确认。第一阶段可以优先复用 `agent_transcript_archive_index` 和 `agent_recall_events`，但需要明确其只能覆盖 transcript，不能满足跨事实类型统一检索的最终目标。

### 7.4 Web 展示层

`web/src/api/agent.ts`

职责：

1. 补充 ContextBundle、MemoryBlock、RecallEvent、MemoryCandidate 类型。
2. 保持 run detail、plan detail 和 progress API 类型一致。

Agent 计划详情页相关 Vue 文件

职责：

1. 在现有流水线中展示本轮实际上下文包。
2. 展示短期对话窗口、长期召回、记忆候选、证据引用和裁剪记录。
3. 子 Agent 仍默认收起，展开后显示工具调用、观察、产物、错误和重试记录。

## 8. 实施顺序

### 阶段一：短期上下文连续性

1. 增加 ContextBundle 数据结构。
2. 增加 `ContextBudgetProfile` 和 `budget_report`，区分主 Agent、子 Agent 和最终综合阶段。
3. 让主 Agent 规划阶段获得必要的最近上下文投影。
4. 将最近 12 条固定窗口改为 token 预算驱动的语义单元窗口。
5. 多轮追问补充上一轮回答、observation 和 artifact 摘要。
6. 子 Agent 上下文投影不继承完整热窗口，只携带任务相关约束、证据和引用。
7. Web 展示本轮上下文包、预算 profile、裁剪记录和保留原因。

验收：

1. “继续刚才任务”“刚才结论依据是什么”“收红吗”能够引用上一轮结果和证据。
2. Web 能看到本轮加载了哪些短期上下文。
3. Web 能看到预算 profile、block token 估算、裁剪记录和 evidence refs。
4. 子 Agent 类上下文不会自动带入完整最近对话热窗口。
5. 单元测试覆盖短期语义单元窗口、保护区、预算 profile、整体裁剪策略和追问上下文。

### 阶段二：长期历史召回

1. PlanSpec 增加历史召回计划字段。
2. 主 Agent 判断是否需要 `conversation.query_history`。
3. 后端按模型计划执行受控查询。
4. 记录 recall event，并在 Web 展示召回原因、参数和结果。
5. 将召回结果统一转换为 `canonical_ref`，再回表读取原始事实。

验收：

1. “我之前说过什么偏好”必须触发历史查询。
2. 历史查询结果有 transcript entry 引用。
3. 查询不依赖后端关键词规则。
4. Web 可展示召回 query、命中的 canonical refs、回表后的原始事实片段。

### 阶段三：通用归档索引和事实引用

1. 建立统一 `fact_type:fact_id` 引用规范。
2. 增加 legacy evidence ref 到 canonical ref 的兼容转换。
3. 建立通用 fact index 的领域对象和 repository 能力。
4. 模型抽取 topics、keywords、entities、importance、confidence 等索引字段。
5. 索引写入失败不影响原始事实保存。

验收：

1. transcript、observation、artifact、plan、step 至少具备统一 canonical ref。
2. 索引命中后必须回表读取原始事实。
3. 索引字段抽取不依赖后端固定关键词词表。
4. 旧 evidence ref 仍可兼容解析。

### 阶段四：记忆候选和用户确认

1. 模型从 turn 结果中生成候选记忆。
2. 后端校验证据和风险等级。
3. 低风险候选进入可应用状态。
4. 高风险候选进入用户确认流程。
5. Web 展示候选、证据、确认、拒绝和撤销入口。

验收：

1. 明确偏好能生成候选。
2. 高风险画像写入不会静默生效。
3. 每条候选都有证据引用和状态流转。

### 阶段五：完整证据投影

1. 工具完整输出保存为可追溯 artifact 内容引用。
2. ContextBudgetManager 只裁剪模型可见投影视图。
3. Web 支持查看 artifact 摘要、来源和完整内容定位。

验收：

1. 子 Agent 关键工具结果可以从 plan step 追溯到 run、observation、artifact。
2. 被裁剪的内容有裁剪记录和原始引用。
3. 深度分析任务可以复盘证据来源。

## 9. 测试要求

1. 单元测试：
   - ContextBundle 构建。
   - token 预算驱动的短期语义单元窗口。
   - ContextBudgetProfile 分配。
   - budget_report token 估算和语义单元整体裁剪记录。
   - 子 Agent 上下文不继承完整热窗口。
   - 多轮追问 payload。
   - 历史召回计划解析。
   - canonical ref 生成、解析和 legacy ref 兼容。
   - 索引字段抽取结果校验。
   - 记忆候选风险校验。

2. 集成测试：
   - 企微消息到最终回复完整链路。
   - Web 发起任务到计划详情展示完整链路。
   - 子 Agent run、trace、observation、artifact 可查。

3. 真实模型测试：
   - 使用 `.env` 中配置。
   - 执行真实多轮任务，不使用假网址或假工具结果。
   - 校验模型每一步输出格式符合要求。

4. 回归测试：
   - `go test ./...`
   - 前端类型检查和构建。
   - 部署后由用户进行企微和 Web 验收。

## 10. RAG 接入方案

### 10.1 接入边界

RAG 不直接替代归档索引层。归档索引仍然负责事实定位、权限过滤、审计引用和回表取证；RAG 作为索引层的增强召回能力接入，用于提升语义召回、精确匹配、跨事实关联和证据重排质量。

目标结构：

```text
原始事实层
  -> 多路索引层
  -> 召回融合层
  -> rerank 重排
  -> canonical_ref 回表读取原始事实
  -> 主 Agent 判断证据是否充分
  -> 生成回答或继续召回
```

必须保留的边界：

1. 原始事实层是唯一可信事实来源。
2. 索引、embedding、chunk summary 和 contextual text 都是派生数据，可以重建，不能替代原文。
3. 模型最终可引用的证据必须带 `canonical_ref`，并能回表定位到 transcript、observation、artifact、web snapshot、item、plan 或 step。
4. 所有召回必须先经过 `user_id`、session、权限、预算和风险边界过滤。
5. 稳定记忆仍是用户程序偏好结论，不等同于普通 RAG 文档片段。

### 10.2 多路索引层

通用 fact index 应同时支持以下检索能力：

1. 结构化索引：`user_id`、`session_id`、`turn_id`、`fact_type`、`memory_kind`、时间范围、来源、风险等级。
2. 全文索引：用于精确词、项目名、错误码、URL、股票代码、机构名和用户明确表述。
3. 语义索引：用于相似意图、近义表达、多轮追问和跨表事实回忆。
4. 上下文化索引：在 chunk 入库前补充所属会话、计划、工具、来源、时间和任务背景，避免孤立片段丢失语义。
5. 关系索引：记录 `fact -> fact`、`turn -> plan -> run -> observation -> artifact`、`user preference -> evidence refs` 的轻量关系边。

建议第一版仍以 Postgres 为主：

```text
agent_fact_archive_index
  - canonical_ref
  - fact_type
  - fact_id
  - user_id
  - session_id
  - turn_id
  - memory_kind
  - topics
  - keywords
  - entities
  - summary_for_index
  - contextual_text
  - full_text_vector
  - embedding
  - importance
  - confidence
  - source_refs
  - relation_refs
  - index_status
```

### 10.3 写入链路

事实写入后不直接进入模型长期上下文，应先进入索引和记忆治理流程：

```text
原始事实写入
  -> 生成 canonical_ref
  -> 主 Agent 或索引模型抽取 topics、entities、summary_for_index、should_recall_for
  -> 后端校验字段长度、枚举、证据引用、用户边界和敏感信息
  -> 写入结构化索引和全文索引
  -> 生成 contextual_text
  -> 生成 embedding
  -> 写入 relation_refs
  -> 需要沉淀偏好时生成 memory candidate
```

写入失败处理：

1. 原始事实保存成功后，索引失败不得回滚原始事实。
2. 索引失败应记录 `index_status=failed`、错误类型和可重建任务。
3. 后续可以通过后台任务批量重建索引、embedding 和 contextual text。

### 10.4 查询链路

用户追问、延续任务或复杂分析任务中，主 Agent 先生成结构化检索计划，后端执行受控召回：

```text
用户问题
  -> 主 Agent 输出 recall_plan
  -> 后端执行权限和预算校验
  -> 结构化过滤
  -> 全文召回
  -> 语义召回
  -> 关系扩展
  -> 合并去重
  -> rerank
  -> canonical_ref 回表读取原始事实
  -> 形成 ContextBundle evidence blocks
  -> 主 Agent 判断证据是否充分
```

召回结果必须区分：

1. `index_hit`：索引命中的派生信息，只说明为什么命中。
2. `source_fact`：回表读取的原始事实片段，是可引用证据。
3. `projection`：为本轮 prompt 压缩后的模型可见视图。

如果证据不足，主 Agent 应继续生成新的 recall_plan 或外部工具计划，而不是基于低相关索引结果直接回答。

### 10.5 与最新 RAG 技术的取舍

本项目应吸收 RAG 的召回能力，但保留当前事实治理结构。

1. Hybrid RAG：优先落地。结合全文检索、结构化过滤和语义向量，适合当前 transcript、artifact、observation 和网页事实召回。
2. Contextual Retrieval：优先落地。chunk 入索引前补充任务、来源、时间和父级事实背景，避免“片段相关但无法解释”的问题。
3. Rerank：优先落地。复杂分析任务先召回更多候选，再按用户问题重排，减少无关证据进入最终回答。
4. GraphRAG：中期借鉴。先用 Postgres 关系边表达事实关联，不立即引入重型图数据库。
5. 专用记忆产品：作为设计参考。Mem0、Zep/Graphiti 的用户记忆、session 记忆、agent 记忆、metadata filter 和审计能力可借鉴，但本项目用户数据和执行审计应优先保留在自有数据库中。

### 10.6 实施顺序补充

RAG 接入应插入到现有长期记忆阶段之后分步实施：

1. 建立 `canonical_ref` 和通用 fact index。
2. 在 Postgres 内先实现结构化过滤和全文召回。
3. 接入 pgvector 或等价向量能力，增加 embedding 字段和语义召回。
4. 增加 contextual_text 生成流程。
5. 增加 rerank 阶段和证据充分性评分。
6. 增加 relation_refs，形成轻量 GraphRAG 能力。
7. Web 展示召回链路：query、index hit、rerank、source fact、projection 和最终证据。

## 11. 参考项目

本轮已将以下项目拉取到 `../references`，用于后续设计和实现核查：

1. `../references/graphrag`：Microsoft GraphRAG，参考知识图谱索引、实体关系抽取、社区摘要和私有数据推理方式。
2. `../references/llama_index`：LlamaIndex，参考 Agentic RAG、工具查询引擎、检索流程编排和多步任务循环。
3. `../references/mem0`：Mem0，参考用户记忆、Agent 记忆、session 记忆、记忆增删改查、metadata filter 和审计设计。
4. `../references/graphiti`：Zep Graphiti，参考时序知识图谱、事件事实、关系演化和长期记忆检索。
5. `../references/haystack`：deepset Haystack，参考生产级 RAG pipeline、retriever、ranker、document store 和组件编排。

既有参考目录中还可继续利用：

1. `../references/pgvector_go`：参考 Go 侧 pgvector 使用方式。
2. `../references/bleve`：参考 Go 全文索引实现。
3. `../references/qdrant_go_client`：参考外部向量库 Go SDK。
4. `../references/langgraph`：参考图式 Agent workflow 和状态流转。
5. `../references/langchaingo`：参考 Go 生态 LLM、工具和检索链路封装。

## 12. 审核点

需要确认以下事项后再进入实现：

1. 第一阶段是否允许新增 `ContextBundle` 类型，还是只扩展现有 `ContextSnapshot`。
2. 长期记忆第一版是否新增表，还是先复用 `agent_transcript_archive_index`。
3. 用户偏好候选是否先只支持显式偏好，不处理隐式阅读行为。
4. Web 是否需要新增“记忆候选”独立页面，还是先放在 Agent 计划详情页。
5. artifact 完整内容是否先存数据库字段，还是仅保存对象引用和原始来源定位。
6. 是否在本轮新增 `agent_fact_archive_index`，还是先实现 canonical ref 兼容层。
7. 是否将 `web_search:URL` 统一替换为 `web_snapshot:id` 形式，还是先保留 URL 引用兼容。
8. RAG 第一版是否限定为 Postgres 内混合检索，还是允许引入外部向量库。
9. contextual_text 和 embedding 是否同步写入，还是通过后台任务异步补齐。
10. rerank 第一版使用模型重排、专用 reranker，还是先用规则化分数组合过渡。

## 13. 本轮实现步骤清单

以下清单用于后续实现时逐项勾选。每完成一项，将对应条目从 `[ ]` 更新为 `[x]`，并保留必要的测试或核验证据。

- [x] 定义或扩展 `ContextBundle`，补齐 `budget_profile`、`budget_report`、block 来源、证据引用、可信等级、token 估算和裁剪状态。
- [x] 定义 `ContextBudgetProfile` 和预算策略，覆盖 `main_planning`、`main_evaluation`、`subagent_search`、`subagent_history_recall`、`subagent_analysis`、`final_synthesis`。
- [x] 将最近 12 条固定窗口改为 token 预算驱动的语义单元窗口，设置主 Agent 热窗口 token 上限，并保证语义单元整体纳入或整体裁剪。
- [ ] 为 ContextBundle 增加短期保护区，固定保留当前用户消息、系统规则、用户明确约束、活动计划、上一轮 assistant 完整回答、关键 observation 和关键 artifact 摘要。
- [x] 让主 Agent 规划阶段接收必要的 ContextBundle 投影视图，避免只基于当前用户消息、capability catalog 和 schema 规划。
- [x] 扩展 PlanSpec 或规划 metadata，加入 `needs_recent_context`、`needs_history_recall`、`history_query_plan`、`required_memory_types` 和 `expected_evidence_scope`。
- [x] 保留 `conversation.query_history` 受控工具，按模型生成的 history query plan 执行召回，并在 ContextBundle 中记录 transcript entry 引用、召回原因、查询参数和时间边界。
- [x] 实现 canonical ref 兼容层，支持将 `agent_transcript_entry:123`、`agent_observations/31`、`agent_artifact:41` 等旧引用标准化为 `transcript:123`、`observation:31`、`artifact:41`。
- [x] 为子 Agent 类上下文投影建立预算规则，确保子 Agent 不继承完整最近对话热窗口，只接收任务相关约束、上游摘要、证据引用和必要回表事实。
- [ ] 多轮追问 payload 补充上一轮 assistant 完整回答、关键 observation 摘要、artifact 摘要、来源 URL、抓取时间和证据引用。
- [x] 在 `agent_run_context_traces` 或等价审计记录中写入 ContextBundle 投影视图、预算 profile、token 估算、裁剪记录、保留原因和 evidence refs。
- [ ] Web 计划详情页展示本轮 ContextBundle、预算 profile、短期窗口、历史召回、裁剪记录、关键 observation、关键 artifact 和 evidence refs。
- [ ] 增加单元测试覆盖 ContextBundle 构建、语义单元窗口、预算 profile、整体裁剪策略、子 Agent 不继承完整热窗口、多轮追问上下文和历史召回计划解析。
- [ ] 运行后端相关测试，并在实现完成后记录实际执行命令和结果。
- [ ] 运行前端类型检查或相关构建验证，并在实现完成后记录实际执行命令和结果。
