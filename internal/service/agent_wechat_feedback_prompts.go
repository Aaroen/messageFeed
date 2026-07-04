package service

import (
	"encoding/json"
	"messagefeed/internal/domain"
	"strings"
)

// agentWeChatFeedbackSystemPrompt 统一维护企业微信短消息的生成规则。
// 这里不做意图分类，只约束语气、长度和不能泄露内部执行字段。
func agentWeChatFeedbackSystemPrompt() string {
	return strings.Join([]string{
		"你是 messageFeed 主 Agent 的企业微信短消息生成器。",
		"你的任务是把当前 Agent 阶段、错误或结果状态，改写成自然、简短、像微信聊天一样的中文回复。",
		"只输出发给用户的一小段文本，不要输出 JSON、Markdown 标题、列表、代码块或解释。",
		"进度消息最多两句，语气直接、日常，不要夸张，不要营销。",
		"错误消息必须说明失败阶段和用户能理解的原因；如果 payload.error 非空，不允许只说处理失败。",
		"如果 timed_out=true，必须明确说本轮处理超时。",
		"如果 thinking_timeout=true 或 error_type=thinking_timeout，必须明确说明这是模型思考阶段超时。",
		"如果 stage=accepted，只表达已收到并开始处理，不要暗示任务已经完成。",
		"如果 stage=button_callback，要把按钮动作处理结果改写成用户能理解的话，不要照抄 handler、control_type 或英文内部摘要。",
		"不要暴露状态锚点、权限、预算、质量评分、成本、trace、run_id、plan_id、内部错误栈或开发实现细节。",
		"如果 payload 提供 progress_url 或 approval_url，可以在必要时自然包含对应 URL。",
	}, "\n")
}

// agentWeChatFeedbackPayloadSchema 只用于提示模型理解字段含义。
// 字段来自后端已经确认的运行状态，不替代主 Agent 的任务规划。
func agentWeChatFeedbackPayloadSchema() domain.AgentJSON {
	return domain.AgentJSON{
		"stage":            "当前消息场景，例如 accepted、started、subagent_stage_completed、failed。",
		"user_message":     "用户原始消息，可为空。",
		"goal":             "主 Agent 理解后的任务目标，可为空。",
		"summary":          "计划摘要或阶段摘要，可为空。",
		"status":           "当前计划状态。",
		"step":             "当前子 Agent 阶段摘要，可为空。",
		"error":            "错误摘要，可为空。",
		"error_type":       "错误类型，例如 thinking_timeout、timeout 或 error。",
		"timed_out":        "是否由处理超时触发。",
		"thinking_timeout": "是否由模型思考阶段超时触发。",
		"progress_url":     "Web 详情地址，可为空。",
		"approval_url":     "需要用户确认时的审批地址，可为空。",
		"control":          "按钮回调或显式控制结果，可为空；包含 action_key、type、status、summary、changed、scheduled_task_status 等字段。",
	}
}

// agentWeChatFeedbackUserPrompt 生成模型输入提示。
// 这属于模型契约提示词，不承载具体业务意图或固定回复文案。
func agentWeChatFeedbackUserPrompt(payload []byte) string {
	body, err := json.Marshal(domain.AgentJSON{
		"instruction": "请根据 payload 生成一条企业微信短回复。",
		"payload":     json.RawMessage(payload),
	})
	if err != nil {
		return string(payload)
	}
	return string(body)
}
