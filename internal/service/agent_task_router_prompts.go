package service

import (
	"encoding/json"
	"messagefeed/internal/domain"
)

func agentTaskRouterSystemPrompt() string {
	return `你是主 Agent 的任务分级器。你只判断当前用户消息应进入哪种执行路径，不直接回答用户。

可选 task_type：
- quick_answer：普通问答、解释、改写、很短的总结；不需要历史召回、工具、子 Agent。
- rag_answer：用户明确要求结合历史上下文、之前记录、记忆、偏好、刚才内容或依据来源；需要历史召回，但不需要外部工具。
- deep_task：需要多步骤规划、外部工具、持续执行、审批、定时任务、联网搜索、复杂分析或不确定风险。

输出必须是一个 JSON 对象，不要 Markdown，不要解释。`
}

func agentTaskRouterUserPrompt(payload domain.AgentJSON) string {
	return marshalAgentTaskRouterPromptPayload(domain.AgentJSON{
		"task":            "classify_agent_task_route",
		"payload":         payload,
		"required_schema": agentTaskRouterSchemaHint(),
	})
}

func agentTaskRouterSchemaHint() domain.AgentJSON {
	return domain.AgentJSON{
		"task_type":               "quick_answer | rag_answer | deep_task",
		"confidence":              "0 到 1 的数字",
		"needs_history_recall":    "boolean",
		"needs_tools":             "boolean",
		"requires_sub_agent":      "boolean",
		"estimated_latency_class": "fast | normal | slow",
		"history_query":           "需要历史召回时填写简短查询；否则为空字符串",
		"reason":                  "一句中文说明，用于用户侧展示主 Agent 判断依据",
	}
}

func marshalAgentTaskRouterPromptPayload(payload domain.AgentJSON) string {
	body, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(body)
}
