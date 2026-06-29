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

### 4.2 短期保护区

每轮应优先保留：

1. 系统规则和能力边界。
2. 当前用户原始消息。
3. 当前主 Agent PlanSpec 和未完成步骤。
4. 最近 8 到 20 条 `user/assistant` 对话，按预算动态调整。
5. 上一轮 assistant 完整回复。
6. 最近一次关键工具 observation。
7. 最近一次关键 artifact 摘要和来源。
8. 用户刚明确表达的限制、偏好、否定反馈或授权。

### 4.3 动态预算

将固定 `Limit: 12` 改为预算驱动：

1. 默认最近对话基础窗口为 12 条。
2. 简单任务可下降到 8 条。
3. 多轮追问、延续任务、分析任务可上升到 20 条。
4. 超出预算时按消息边界裁剪最旧消息，不截断单条语义。
5. 工具结果和 artifact 使用投影视图，保留完整原始引用。

## 5. 长期记忆改造

### 5.1 长期记忆分层

长期记忆分为五层：

1. 原始事实层：transcript、tool input、tool output、run trace、audit、artifact。
2. 归档索引层：冷热状态、memory kind、重要度、实体、关键词、证据引用。
3. 候选记忆层：模型从对话和行为证据中提取的偏好、事实、决策和任务习惯。
4. 稳定记忆层：经过用户确认或多证据支持后进入长期可召回记忆。
5. 用户画像层：显式偏好、长期兴趣、负反馈、风险偏好、回复风格和通知偏好。

### 5.2 模型驱动的记忆候选

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

### 5.3 用户偏好沉淀

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
  -> 稳定记忆进入后续可召回层
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

### 7.3 Repository 和 Domain 层

`internal/domain/agent_channel.go`

职责：

1. 补齐 transcript archive、memory kind、recall event 相关领域字段时保持类型稳定。
2. 如果引入候选记忆对象，可优先新增独立 domain 文件，避免继续扩大单文件。

`internal/repository/agent_repository.go`

职责：

1. 保留 transcript 原文和 archive index。
2. 将当前关键词式 `classifyTranscriptMemory` 收敛为临时 fallback。
3. 后续由模型生成的记忆候选更新 archive index。

可能新增迁移：

1. `agent_memory_candidates`：保存模型提取的候选记忆、证据、置信度、确认状态。
2. `agent_memory_blocks`：保存稳定长期记忆投影视图。
3. `agent_memory_events`：保存候选生成、确认、拒绝、撤销和更新事件。

是否新增表需要审核确认。第一阶段可以优先复用 `agent_transcript_archive_index` 和 `agent_recall_events`，降低改动面。

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
2. 让主 Agent 规划阶段获得必要的最近上下文投影。
3. 将最近 12 条固定窗口改为预算驱动窗口。
4. 多轮追问补充上一轮回答、observation 和 artifact 摘要。
5. Web 展示本轮上下文包。

验收：

1. “继续刚才任务”“刚才结论依据是什么”“收红吗”能够引用上一轮结果和证据。
2. Web 能看到本轮加载了哪些短期上下文。
3. 单元测试覆盖短期窗口、保护区和追问上下文。

### 阶段二：长期历史召回

1. PlanSpec 增加历史召回计划字段。
2. 主 Agent 判断是否需要 `conversation.query_history`。
3. 后端按模型计划执行受控查询。
4. 记录 recall event，并在 Web 展示召回原因、参数和结果。

验收：

1. “我之前说过什么偏好”必须触发历史查询。
2. 历史查询结果有 transcript entry 引用。
3. 查询不依赖后端关键词规则。

### 阶段三：记忆候选和用户确认

1. 模型从 turn 结果中生成候选记忆。
2. 后端校验证据和风险等级。
3. 低风险候选进入可应用状态。
4. 高风险候选进入用户确认流程。
5. Web 展示候选、证据、确认、拒绝和撤销入口。

验收：

1. 明确偏好能生成候选。
2. 高风险画像写入不会静默生效。
3. 每条候选都有证据引用和状态流转。

### 阶段四：完整证据投影

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
   - 动态短期窗口。
   - 多轮追问 payload。
   - 历史召回计划解析。
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

## 10. 审核点

需要确认以下事项后再进入实现：

1. 第一阶段是否允许新增 `ContextBundle` 类型，还是只扩展现有 `ContextSnapshot`。
2. 长期记忆第一版是否新增表，还是先复用 `agent_transcript_archive_index`。
3. 用户偏好候选是否先只支持显式偏好，不处理隐式阅读行为。
4. Web 是否需要新增“记忆候选”独立页面，还是先放在 Agent 计划详情页。
5. artifact 完整内容是否先存数据库字段，还是仅保存对象引用和原始来源定位。

