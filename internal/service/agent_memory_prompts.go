package service

import (
	"encoding/json"
	"messagefeed/internal/domain"
	"strings"
)

const (
	agentMemoryClassifierName = "llm_memory_classifier"
)

var agentMemoryRiskGuardTerms = []string{
	"密码",
	"口令",
	"密钥",
	"token",
	"secret",
	"api key",
	"apikey",
	"身份证",
	"银行卡",
}

func agentMemoryClassificationSystemPrompt() string {
	return strings.Join([]string{
		"你是 messageFeed 的长期记忆判定器。",
		"你的任务是基于用户消息和当前活跃主题，判断是否应沉淀为长期记忆、是否延续或切换主题、是否需要形成可嵌入的 memory chunk。",
		"所有语义判断由你完成，包括 memory_kind、importance、risk_level、topic_decision 和 consolidation_reason。",
		"后端只会执行结构校验、安全阈值、落库、异步 embedding 入队和观测记录；不要依赖后端关键词规则。",
		"仅当内容对未来对话、用户偏好、事实档案、长期任务或明确决策有持续价值时 should_capture 才为 true。",
		"涉及凭据、密钥、身份证件、银行卡或其他高敏信息时 risk_level 必须为 high，并应要求确认。",
		"输出必须是严格 JSON 对象，不要使用 Markdown、代码块、解释文字或额外字段。",
	}, "\n")
}

func agentMemoryClassificationSchemaHint() domain.AgentJSON {
	return domain.AgentJSON{
		"should_capture":        "boolean，是否应创建 memory candidate。",
		"memory_kind":           "preference | task | fact | decision | casual | unknown。",
		"confidence":            "number，0 到 1。",
		"importance":            "integer，0 到 100。",
		"risk_level":            "low | medium | high。",
		"summary":               "string，候选记忆摘要；不应包含无关闲聊。",
		"keywords":              "string[]，模型提取的主题关键词，最多 8 个。",
		"topic_decision":        "new_topic | same_topic | close_and_new | close_only | ignore。",
		"topic_title":           "string，当前或新主题标题。",
		"topic_summary":         "string，当前或新主题摘要。",
		"topic_intent":          "string，模型归纳的主题意图标签。",
		"consolidation_reason":  "high_value | topic_switch | topic_size_exceeded | context_overflow | idle | none。",
		"should_create_chunk":   "boolean，是否应创建 memory chunk 并进入 embedding 队列。",
		"chunk_title":           "string，chunk 标题。",
		"chunk_summary":         "string，chunk 摘要。",
		"chunk_content":         "string，适合检索和嵌入的完整 chunk 文本。",
		"requires_confirmation": "boolean，高风险或不确定敏感内容应为 true。",
		"reason":                "string，简短说明模型判定依据。",
	}
}

func agentMemoryClassificationUserPrompt(payload domain.AgentJSON) string {
	payload["required_schema"] = agentMemoryClassificationSchemaHint()
	payload["output_language"] = "zh-CN"
	body, _ := json.Marshal(payload)
	return string(body)
}
