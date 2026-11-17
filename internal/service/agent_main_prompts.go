package service

import (
	"messagefeed/internal/domain"
	"strings"
)

// mainAgentPlanSpecSystemPrompt 集中维护主 Agent 规划阶段的系统提示词。
// 该提示词只描述职责边界和输出格式，不包含面向具体用户语句的关键词分流规则。
func mainAgentPlanSpecSystemPrompt() string {
	return strings.Join([]string{
		"你是 messageFeed 的主 Agent 规划器。",
		"你的任务是理解用户消息，判断复杂度，决定是否需要子 Agent，生成结构化 PlanSpec。",
		"规划阶段不执行工具；required_capabilities 是后续执行所需的能力范围，主 Agent 可以直接调用工具。",
		"你只能从后端提供的 capability 列表中选择能力；不要创造能力 key。",
		"后端会执行权限、预算和 capability 校验；你只负责语义理解、任务拆分、证据要求和最终回答约束。",
		"普通问答、单次检索、历史召回、读取后转换格式以及有前后依赖的步骤，由主 Agent 连续执行，requires_sub_agent=false。",
		"只有 complex 任务包含至少两个可以独立完成的工作单元、合计至少三种不同能力，并且并行处理可以减少等待时间时，才允许 requires_sub_agent=true。每个独立工作单元必须填写 independent=true、独立提示词和明确产物，并通过 delegation_reason 说明并行收益；核心分析和最终决策由主 Agent 完成。",
		"子任务执行期间必须等待结果；只有用户停止、明确执行失败或者整体执行期限到达时才能终止。",
		"如果请求载荷包含 context_projection，必须把其中的近期对话、预算报告和裁剪记录作为规划依据，并用 needs_recent_context、needs_history_recall、history_query_plan 等字段表达上下文需求。",
		"能力选择应先满足数据来源，再考虑加工转换；如果回答依赖当前消息之外的数据，必须选择能够读取该数据域的 capability，不能用总结类 capability 替代检索或读取。",
		"需要长期记忆或 RAG 依据时，通常同时选择 conversation.query_history 和 memory.fact_recall：前者读取原始聊天，后者读取长期事实索引。",
		"如果用户要求最新、今天、现在、当前热点、市场收盘、比赛结果等实时事实，且没有明确要求只基于历史或不要联网，应把 web.search 作为历史召回不足时的外部证据兜底。",
		"如果用户明确要求只基于历史、仅用记忆、不要联网或禁止外部搜索，不要选择 web.search，并在 metadata 中说明 web_fallback_policy=skipped。",
		"输出必须是严格 JSON 对象，不要使用 Markdown、代码块、解释文字或额外字段。",
	}, "\n")
}

// mainAgentPlanSpecReplyConstraints 约束最终回答的用户可见边界。
// 这些约束不参与意图判断，只避免把内部治理数据暴露到企业微信等用户入口。
func mainAgentPlanSpecReplyConstraints() []string {
	return []string{
		"最终回答面向用户，不输出执行治理字段。",
		"执行细节留在 Web 详情页。",
	}
}

// mainAgentPlanSpecSchemaHint 描述模型必须返回的结构化字段。
// 字段说明用于稳定 JSON 结构，具体任务类型和能力选择仍由模型根据上下文生成。
func mainAgentPlanSpecSchemaHint() domain.AgentJSON {
	return domain.AgentJSON{
		"goal":                     "string，用户原始目标或归一化目标。",
		"intent":                   "string，主 Agent 对用户真实意图的理解。",
		"task_type":                "string，模型归纳的任务类型标签。",
		"complexity":               "simple | standard | complex。",
		"requires_sub_agent":       "boolean，默认 false；仅满足独立工作单元和并行收益条件时为 true。",
		"delegation_reason":        "string，说明独立工作单元之间的边界和减少等待时间的依据；无需委派时为空。",
		"direct_answer_allowed":    "boolean，仅在不需要工具也能回答时为 true。",
		"required_capabilities":    "string[]，只允许填 capability catalog 中的 key。",
		"subtasks":                 "array，每项包含 title、prompt、independent、context_summary、capability_keys、expected_output、failure_strategy、max_retries；independent 仅在不依赖其他子任务结果时为 true。",
		"evidence_requirements":    "array，每项包含 evidence_type、summary、minimum_count、freshness、required。",
		"needs_recent_context":     "boolean，是否需要近期上下文连续性；如果用户在延续前文或追问，应为 true。",
		"needs_history_recall":     "boolean，是否需要查询更早历史；只表达需求，不直接执行查询。",
		"history_query_plan":       "object，包含 mode、query、time_hint、reason、limit；仅当需要更早历史或当前投影证据不足时填写。",
		"required_memory_types":    "string[]，需要的记忆类型，例如 preference、task、fact、decision。",
		"expected_evidence_scope":  "string[]，期望证据范围，例如 recent_context、history_recall、subscription_items、web、repo、artifact。",
		"max_iterations":           "integer，0 到 3。",
		"final_answer_constraints": "string[]，最终回答约束。",
		"metadata":                 "object，可放复杂度理由、工具选择理由、证据策略等模型生成的辅助信息；实时事实但不联网时写入 web_fallback_policy 及原因。",
	}
}
