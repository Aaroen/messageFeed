# 主 Agent 与子 Agent 调用重构计划

## 1. 目标

本计划用于把当前 Agent 执行链路从“主 Agent 生成计划后，由单个 runner 持有全部 capability scope 自行调用工具”重构为“主 Agent 规划、后端校验、子 Agent 独立执行、主 Agent 评估与迭代”的结构。

重构目标：

1. 主 Agent 负责理解、规划、拆分、提示词合成、证据充分性评估和最终回答。
2. 子 Agent 负责在明确任务包、独立上下文、独立预算和受限工具范围内完成具体子任务。
3. 子任务按依赖关系形成 DAG，可并行的并行执行，有依赖的串行执行。
4. 子 Agent 返回结构化结果，主 Agent 不直接消费全部工具原文，而是基于摘要、证据引用和必要回表内容做评估。
5. Web 详情页可以清晰展示“主 Agent -> 子 Agent 任务包 -> 工具行为 -> 子 Agent 产物 -> 主 Agent 评估 -> 最终回复”的完整流水线。

## 2. 当前实现核实

当前实现已经具备以下基础：

1. 主 Agent 规划阶段由模型生成 `PlanSpec`，后端只做结构校验和 capability 注册表校验。
2. `PlanSpec` 包含 `requires_sub_agent`、`subtasks`、`evidence_requirements`、`max_iterations` 等字段。
3. `Planner.BuildFromSpec` 会把 `PlanSpec` 转为 `AgentPlan` 和 `AgentPlanStep`。
4. `TurnRunner` 支持 OpenAI-compatible tool calls，并通过 MCP tool descriptor 暴露工具。
5. 工具调用会记录 executor run、context trace、observation 和 artifact。
6. Web 端已有流水线展示基础，可以展示主 Agent 规划、步骤、观察和产物。

当前主要不足：

1. 子 Agent 还不是独立执行单元。`subtask.prompt` 和 `context_summary` 主要进入 plan metadata，没有真正作为独立执行输入。
2. 执行阶段将 `plan.AllowedScopes` 一次性交给同一个 `TurnRunner`，由模型自行选择工具调用顺序。
3. plan step 当前主要按 capability 展开，而不是按“一个子 Agent 使用多个工具完成一个子目标”建模。
4. 多个子任务没有独立上下文窗口、独立预算、独立工具范围和独立返回契约。
5. `evidence_requirements` 和 `max_iterations` 当前更多用于记录，没有形成主 Agent 证据评估后继续派发的闭环。
6. 子 Agent 返回缺少统一结构，例如 `findings`、`evidence_refs`、`gaps`、`confidence` 和 `suggested_next_actions`。

## 3. 目标调用链

目标流程：

```text
用户消息
  -> 写入 transcript
  -> 主 Agent 构建规划上下文
  -> 主 Agent 输出 PlanSpec
  -> 后端校验权限、预算、capability 和依赖关系
  -> 将 PlanSpec 转为 SubAgentTask DAG
  -> ContextBuilder 为每个子任务构建独立 ContextBundle
  -> SubAgentRunner 执行无依赖或依赖已满足的子任务
  -> 工具调用写入 run、trace、observation、artifact
  -> 子 Agent 返回 SubAgentResult
  -> 主 Agent 评估证据充分性
  -> 证据不足时继续派发补充子任务或历史召回
  -> 证据充分后生成最终回答
  -> 生成候选记忆和索引更新任务
```

该流程中，主 Agent 与子 Agent 的上下文预算分开计算。总任务消耗可以超过单次 64k，但任一模型调用都必须在自己的预算内完成。

## 4. 子 Agent 执行模型

### 4.1 SubAgentTask

建议引入子任务执行包：

```text
SubAgentTask
- id
- plan_id
- step_id
- parent_task_id
- role
- task_type
- title
- goal
- instruction
- context_summary
- capability_scope
- evidence_requirements
- output_contract
- depends_on
- parallel_group
- budget
- retry_policy
- risk_level
- created_by
```

字段说明：

1. `id`：子任务稳定标识，用于依赖、审计和 Web 展示。
2. `role`：子 Agent 执行角色，例如 searcher、reader、analyst、synthesizer、memory_recaller。
3. `goal`：子任务目标，只描述本子任务要完成的事情。
4. `instruction`：主 Agent 合成的执行提示词。
5. `capability_scope`：本子 Agent 允许使用的工具集合。
6. `depends_on`：前置子任务 ID 列表。
7. `parallel_group`：同组且无依赖的任务可并行。
8. `budget`：该子 Agent 独立上下文和工具预算。
9. `output_contract`：要求子 Agent 返回的结构化 JSON 契约。

### 4.2 SubAgentResult

子 Agent 返回统一结构：

```text
SubAgentResult
- task_id
- status
- summary
- findings
- evidence_refs
- source_refs
- artifact_refs
- observation_refs
- gaps
- confidence
- errors
- retry_count
- suggested_next_actions
- token_usage_estimate
```

字段说明：

1. `summary`：给主 Agent 的简短结论，不面向最终用户直接输出。
2. `findings`：结构化事实点，每个事实点必须带来源或证据引用。
3. `evidence_refs`：统一使用 `canonical_ref`，后续可回表读取原始事实。
4. `gaps`：本子任务发现的证据缺口。
5. `confidence`：子 Agent 对本任务完成度的自评，供主 Agent 评估参考。
6. `suggested_next_actions`：建议主 Agent 是否补派任务、扩大来源或降级回答。

## 5. 并行与串行策略

子 Agent 不应固定串行，也不应全部并行。推荐采用 DAG 执行：

```text
depends_on 为空且 parallel_group 相同 -> 可并行
depends_on 非空 -> 等待依赖完成
需要使用上一步结果 -> 串行
涉及写操作、确认或高风险能力 -> 串行
最终 synthesis -> 等待关键依赖完成
```

第一版建议：

1. 默认并发上限为 3。
2. 外部联网子 Agent 并发上限为 2。
3. 写操作子 Agent 不并行，必须确认后串行。
4. 最终综合子 Agent 或主 Agent final pass 必须等待关键依赖完成。
5. 任一子 Agent 失败时，不直接终止全局任务；先由主 Agent 判断是否可补派、降级或终止。

示例：

```text
港股消息检索       depends_on=[]
美股消息检索       depends_on=[]
A 股消息检索       depends_on=[]
机构观点检索       depends_on=[]
宏观基本面整理     depends_on=[]
综合分析           depends_on=[港股消息检索, 美股消息检索, A 股消息检索, 机构观点检索, 宏观基本面整理]
最终回复           depends_on=[综合分析]
```

## 6. 主 Agent 合成提示词

### 6.1 选择原则

主 Agent 不应把完整上下文复制给每个子 Agent，而应为每个子任务生成最小充分任务包。

必须放入：

1. 用户原始目标。
2. 子任务目标。
3. 子任务边界。
4. 时间范围。
5. 来源范围或数据范围。
6. 用户明确约束。
7. 与该子任务相关的稳定记忆。
8. capability scope。
9. 输出契约。
10. 失败处理策略。

按需放入：

1. 最近对话中的相关约束。
2. 历史召回结果。
3. 上游子任务结果摘要。
4. 上游子任务证据引用。
5. 已有 artifact 引用。

默认不放入：

1. 完整最近对话。
2. 其他子任务无关细节。
3. 全部工具原始输出。
4. Web 展示用治理字段。
5. 内部审计字段。
6. 无关失败日志。

### 6.2 提示词结构

建议子 Agent 提示词结构：

```text
角色：
你是本轮任务中的一个受限子 Agent。

子任务：
{{subtask.goal}}

任务边界：
{{subtask.boundary}}

可用工具：
{{capability_scope}}

必须遵守的用户约束：
{{user_constraints}}

相关上下文：
{{context_blocks}}

证据要求：
{{evidence_requirements}}

输出契约：
{{output_contract}}

失败策略：
如果证据不足，返回 gaps，不要编造。
```

示例：

```text
你负责完成子任务：检索周五收盘以来港股市场的消息面和基本面信息。

任务边界：
只处理港股，不分析美股和 A 股。
优先使用权威、多样来源。
需要覆盖指数、资金流向、行业板块、重要公司、政策和机构观点。
结果必须包含来源、时间、摘要和 evidence_refs。
不要生成最终投资结论，只返回结构化研究结果。

可用工具：
web.search
web.extract_page
feed.query_recent_items

输出：
{
  "summary": "",
  "findings": [],
  "sources": [],
  "gaps": [],
  "confidence": 0.0
}
```

## 7. 上下文预算

主 Agent 与子 Agent 分开预算。

### 7.1 主 Agent

主 Agent 预算重点是对话连续性、计划和评估：

```text
总预算：64k
最近对话热窗口：32k 默认，复杂连续任务 40k，硬上限 44k
稳定记忆：4k-6k
历史召回：6k-10k
子 Agent 结果摘要：8k-12k
计划和评估结构：4k-6k
输出和安全余量：6k-8k
```

### 7.2 子 Agent

子 Agent 预算重点是任务证据密度：

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

预算原则：

1. 子 Agent 不继承完整热窗口。
2. 搜索类子 Agent 优先保留网页和来源证据。
3. 分析类子 Agent 优先保留多源事实和冲突点。
4. 最终综合阶段只读取子 Agent 结构化结果，必要时按 `canonical_ref` 回表取证。

## 8. 主 Agent 评估与迭代

子 Agent 完成后，主 Agent 不应立即生成最终回答，而应先执行证据评估：

```text
输入：
- 原始用户目标
- PlanSpec
- SubAgentResult 列表
- evidence_requirements
- gaps
- source diversity
- freshness
- conflict report

输出：
- sufficient: true/false
- missing_evidence
- conflict_points
- followup_subtasks
- final_answer_allowed
```

评估规则：

1. 证据覆盖不足时，补派子任务。
2. 来源单一时，补派多来源检索任务。
3. 时间范围不完整时，补派时间范围检索任务。
4. 观点冲突时，补派冲突核查任务。
5. 达到最大迭代次数仍不足时，生成保守回答并明确证据边界。

## 9. 数据和持久化调整

第一版可以不立即新增大量表，但需要形成清晰领域对象。

建议新增或扩展：

```text
agent_subtasks
- id
- plan_id
- step_id
- parent_task_id
- task_key
- title
- goal
- instruction
- capability_scope
- depends_on
- parallel_group
- status
- budget_json
- output_contract_json
- result_json
- created_at
- updated_at

agent_subtask_events
- id
- subtask_id
- event_type
- status
- content_json
- created_at
```

如果第一阶段不新增表，可以先把 `SubAgentTask` 和 `SubAgentResult` 写入：

1. `agent_runs.task_packet`
2. `agent_run_context_traces.content`
3. `agent_plan_steps.retry_metadata.sub_agent`
4. `agent_plan.metadata.subagent_results`

但长期建议使用独立表，避免 `agent_plan_steps` 继续承载过多语义。

## 10. 文件职责建议

### 10.1 Agent 核心层

`internal/agent/subagent_task.go`

职责：

1. 定义 `SubAgentTask`、`SubAgentResult`、`SubAgentOutputContract`。
2. 定义 DAG 依赖、预算和状态枚举。
3. 提供结构校验函数。

`internal/agent/subagent_scheduler.go`

职责：

1. 根据 `depends_on` 和 `parallel_group` 计算可执行批次。
2. 控制并发上限。
3. 处理失败后的阻塞、跳过和补派策略。

`internal/agent/subagent_prompts.go`

职责：

1. 集中维护子 Agent 执行提示词。
2. 集中维护子 Agent 结果 JSON 契约。
3. 禁止业务流程代码散落提示词。

`internal/agent/runner.go`

职责调整：

1. 保留底层工具调用和 LLM tool loop。
2. 支持接收 `SubAgentTask` 任务包。
3. 不再把整个 plan 的 allowed scopes 当作一个大任务执行。

### 10.2 Service 编排层

`internal/service/agent_turn_pipeline.go`

职责调整：

1. 从“一次调用 `turnRunner.Run`”改为“创建并调度多个 SubAgentTask”。
2. 维护主 Agent 评估循环。
3. 汇总子 Agent 结果并生成最终回复。

`internal/service/agent_main_planner.go`

职责调整：

1. 扩展 PlanSpec schema，支持 `subtask.id`、`depends_on`、`parallel_group`、`role`、`output_contract`。
2. 规划阶段加入主 Agent 决策上下文。
3. 输出子 Agent 任务拆分依据。

`internal/service/agent_subagent_executor.go`

职责：

1. 将 `SubAgentTask` 转换为 runner 输入。
2. 为每个子 Agent 构造独立 ContextBundle。
3. 记录 subtask run、trace、observation、artifact 和 result。

`internal/service/agent_subagent_evaluator.go`

职责：

1. 调用主 Agent 评估子 Agent 结果。
2. 判断是否补派、降级、终止或进入最终回答。
3. 记录评估 trace。

### 10.3 Web 层

Agent 详情页应展示：

```text
主 Agent 接收任务
主 Agent 理解与规划
后端权限和预算校验
主 Agent 合成子 Agent 任务包
子 Agent DAG 执行
  - 子 Agent 默认收起
  - 展开后显示提示词、上下文、工具调用、观察、产物、错误和重试
主 Agent 证据充分性评估
最终回答
记忆候选和索引更新
```

## 11. 实施顺序

### 阶段一：执行输入修正

目标：让当前 runner 真正接收主 Agent 生成的子任务提示词。

改造点：

1. 从 plan step metadata 中读取 `sub_agent.prompt`、`context_summary` 和 `evidence_requirements`。
2. runner 输入从用户原始消息改为子任务执行包。
3. 每个 step 独立执行一次 runner。
4. 每个 step 只下发自己的 `CapabilityScope`。

验收：

1. Web 可以看到每个子 Agent 的实际提示词。
2. 子 Agent 不再拿整个 plan 的 allowed scopes。
3. 搜索任务可以拆成多个独立步骤执行。

### 阶段二：SubAgentTask 领域对象

目标：让子 Agent 从展示概念变成可调度实体。

改造点：

1. 定义 `SubAgentTask` 和 `SubAgentResult`。
2. PlanSpec 扩展 `subtask.id`、`depends_on`、`parallel_group`、`role`。
3. 将 PlanSpec 转为 DAG。
4. 每个子任务独立记录 run 和 result。

验收：

1. 同一任务可以生成多个子 Agent。
2. 子 Agent 返回结构化结果。
3. plan step 能关联 subtask、run、observation 和 artifact。

### 阶段三：DAG 调度

目标：支持并行和串行混合执行。

改造点：

1. 实现子任务依赖排序。
2. 实现并发上限。
3. 实现依赖失败后的阻塞、跳过和补派。
4. 联网任务按并发上限执行。

验收：

1. 无依赖搜索任务可以并行。
2. 综合任务等待检索任务完成。
3. 任一子任务失败不会导致后台僵尸任务。

### 阶段四：主 Agent 评估循环

目标：形成“派发 -> 执行 -> 评估 -> 补派/完成”的闭环。

改造点：

1. 新增主 Agent evidence sufficiency prompt。
2. 评估 `SubAgentResult` 是否覆盖证据要求。
3. 不足时生成补充 `SubAgentTask`。
4. 达到最大迭代次数后保守收敛。

验收：

1. 来源不足时能补充搜索。
2. 证据冲突时能补充核查。
3. 无法补足时最终回答明确证据边界。

### 阶段五：Web 流水线完善

目标：使用户能在 Web 端复盘完整主/子 Agent 流程。

改造点：

1. 展示 DAG。
2. 子 Agent 默认收起。
3. 展开后展示任务包、上下文包、工具调用、观察、产物、错误、重试和结果。
4. 展示主 Agent 评估结果和补派原因。

验收：

1. 用户可以清楚看到每个子 Agent 为什么执行、使用了什么、返回了什么。
2. 执行详情不再堆叠到页面底部。
3. 无效治理字段不作为用户重点信息展示。

## 12. 测试要求

单元测试：

1. PlanSpec 到 SubAgentTask 的转换。
2. DAG 依赖排序。
3. 并发批次计算。
4. 子任务提示词构建。
5. SubAgentResult 解析和校验。
6. 证据充分性评估结果解析。

集成测试：

1. 简单直接回答任务不创建子 Agent。
2. 单工具任务创建一个子 Agent。
3. 多来源搜索任务创建多个并行子 Agent。
4. 综合分析任务等待依赖完成。
5. 子任务失败后主 Agent 能补派或降级。

真实模型测试：

1. 使用 `.env` 中运行时模型配置。
2. 执行真实联网搜索，不使用假网址。
3. 验证每个子 Agent 返回符合结构化契约。
4. 验证主 Agent 能根据子 Agent 结果补派或最终回答。

## 13. 审核点

进入实现前需要确认：

1. 第一版是否先复用 `agent_plan_steps`，还是直接新增 `agent_subtasks` 表。
2. 子 Agent 默认并发上限是否采用 3，联网并发是否采用 2。
3. 所有子 Agent 是否统一使用 64k 预算，还是按 role 设置不同预算。
4. 主 Agent 评估循环最大次数是否仍保留 3。
5. Web 是否需要在现有计划详情页内展示 DAG，还是新增独立子任务详情页。
6. 子 Agent 的结构化结果是否必须全部写入 artifact，还是 result_json 和 artifact 双写。
7. 失败子任务是否允许用户在 Web 端单独重试。
