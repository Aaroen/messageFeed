package service

import (
	"messagefeed/internal/domain"
	"strings"
)

// agentFollowupIntentSystemPrompt 集中维护多轮消息的模型决策规则。
// 后端只提供候选计划和状态事实，不用固定词表判断用户是在停止、追问还是开启新任务。
func agentFollowupIntentSystemPrompt() string {
	return strings.Join([]string{
		"你是 messageFeed 主 Agent 的多轮消息决策器。",
		"你的任务是判断当前用户消息和已有计划之间的关系。",
		"只根据语义和 payload 中的计划状态判断 intent，不要输出解释文本。",
		"如果用户在发起一个独立的新目标，intent 必须为 new_task。",
		"如果用户要求终止或取消当前活动计划，intent 为 stop。",
		"如果用户是在询问已有计划的结果、依据、进度或失败原因，intent 为 followup_question。",
		"如果用户要求重试失败计划，intent 为 retry。",
		"如果用户只是想修改当前活动计划的约束，intent 为 append_constraints；后端会把旧活动计划终止并让当前消息重新规划。",
		"如果用户明确要求基于已有结果继续派生一个新任务，intent 为 derive_task。",
		"输出必须是严格 JSON 对象。",
	}, "\n")
}

// agentFollowupIntentSchemaHint 描述多轮决策模型必须返回的结构。
func agentFollowupIntentSchemaHint() domain.AgentJSON {
	return domain.AgentJSON{
		"intent":     "string，必须是 allowed_intents 中的一个。",
		"confidence": "number，0 到 1。",
		"reason":     "string，简要说明语义判断依据，供 Web 审计查看。",
	}
}

// agentFollowupAnswerSystemPrompt 集中维护多轮追问的用户可见答复规则。
// 服务层只传入已有计划、步骤、证据和用户追问事实，具体如何解释和纠错由模型完成。
func agentFollowupAnswerSystemPrompt() string {
	return strings.Join([]string{
		"你是 messageFeed 主 Agent 的多轮追问答疑器。",
		"你的任务是基于 payload 中的既有任务结果，直接回答用户本轮追问。",
		"如果用户指出前文事实错误，要先核对 payload 中的结果和证据摘要；确有不一致时应明确承认并给出更正。",
		"如果 payload.freshness.status 为 stale，不要把历史结果当作当前事实，只能说明已有结果已不适合作为当前依据，并指出需要刷新检索。",
		"如果 payload 证据不足以回答，要说明证据不足，并指出需要重新检索或补充的事实类型。",
		"不要把计划编号、状态枚举、审计字段、内部 evidence ref、工具名列表当作正文重点；只有用户明确要求流程详情时才简要提到进度地址。",
		"输出必须是自然中文纯文本，不使用 Markdown 标题、项目符号、表格、代码块或链接说明模板。",
	}, "\n")
}
