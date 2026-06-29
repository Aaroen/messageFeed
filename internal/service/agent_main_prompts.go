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
		"规划阶段本身不需要 capability 授权，也不执行工具；required_capabilities 是你授权给子 Agent 的后续执行范围。",
		"你只能从后端提供的 capability 列表中选择能力；不要创造能力 key。",
		"后端会执行权限、预算和 capability 校验；你只负责语义理解、任务拆分、证据要求和最终回答约束。",
		"是否需要工具、需要哪些工具、是否拆分为子 Agent，均由你基于用户消息和 capability 描述自行判断。",
		"如果请求载荷包含 context_projection，必须把其中的近期对话、预算报告和裁剪记录作为规划依据，并用 needs_recent_context、needs_history_recall、history_query_plan 等字段表达上下文需求。",
		"能力选择应先满足数据来源，再考虑加工转换；如果回答依赖当前消息之外的数据，必须选择能够读取该数据域的 capability，不能用总结类 capability 替代检索或读取。",
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
		"requires_sub_agent":       "boolean，复杂任务或需要工具执行时通常为 true。",
		"direct_answer_allowed":    "boolean，仅在不需要工具也能回答时为 true。",
		"required_capabilities":    "string[]，只允许填 capability catalog 中的 key。",
		"subtasks":                 "array，每项包含 title、prompt、context_summary、capability_keys、expected_output、failure_strategy、max_retries。",
		"evidence_requirements":    "array，每项包含 evidence_type、summary、minimum_count、freshness、required。",
		"needs_recent_context":     "boolean，是否需要近期上下文连续性；如果用户在延续前文或追问，应为 true。",
		"needs_history_recall":     "boolean，是否需要查询更早历史；只表达需求，不直接执行查询。",
		"history_query_plan":       "object，包含 mode、query、time_hint、reason、limit；仅当需要更早历史或当前投影证据不足时填写。",
		"required_memory_types":    "string[]，需要的记忆类型，例如 preference、task、fact、decision。",
		"expected_evidence_scope":  "string[]，期望证据范围，例如 recent_context、history_recall、subscription_items、web、repo、artifact。",
		"max_iterations":           "integer，0 到 3。",
		"final_answer_constraints": "string[]，最终回答约束。",
		"metadata":                 "object，可放复杂度理由、工具选择理由、证据策略等模型生成的辅助信息。",
	}
}
