package agent

import (
	"context"
	"strings"
)

type CapabilityMode string

const (
	CapabilityModeCore     CapabilityMode = "core"
	CapabilityModeDeferred CapabilityMode = "deferred"
	CapabilityModeHidden   CapabilityMode = "hidden"
)

type CapabilityRisk string

const (
	CapabilityRiskLow    CapabilityRisk = "low"
	CapabilityRiskMedium CapabilityRisk = "medium"
	CapabilityRiskHigh   CapabilityRisk = "high"
)

type PolicyDecision string

const (
	PolicyDecisionAllow     PolicyDecision = "allow"
	PolicyDecisionPrompt    PolicyDecision = "prompt"
	PolicyDecisionForbidden PolicyDecision = "forbidden"
)

type Capability struct {
	Key         string
	Name        string
	Description string
	Mode        CapabilityMode
	Risk        CapabilityRisk
	Mutates     bool
	Parameters  map[string]any
}

type CapabilityRegistry struct {
	byKey map[string]Capability
}

func NewP0CapabilityRegistry() *CapabilityRegistry {
	registry := &CapabilityRegistry{byKey: map[string]Capability{}}
	registry.Register(Capability{
		Key:         "feed.query_recent_items",
		Name:        "查询最近资讯",
		Description: "读取当前用户最近订阅条目。",
		Mode:        CapabilityModeCore,
		Risk:        CapabilityRiskLow,
	})
	registry.Register(Capability{
		Key:         "source.query_latest_items",
		Name:        "查询来源最新条目",
		Description: "按来源名称或来源 ID 读取最新条目。",
		Mode:        CapabilityModeCore,
		Risk:        CapabilityRiskLow,
	})
	registry.Register(Capability{
		Key:         "conversation.query_history",
		Name:        "查询历史聊天",
		Description: "按关键词、时间、角色或会话边界读取当前用户企微长期会话的历史聊天原文。",
		Mode:        CapabilityModeCore,
		Risk:        CapabilityRiskLow,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"description": "查询模式。search 为关键词或普通历史查询；time_range 为按时间范围查询；earliest 用于查询当前 session 最早聊天；latest 用于查询当前 session 最新聊天。",
					"enum":        []string{"search", "time_range", "earliest", "latest"},
				},
				"query": map[string]any{
					"type":        "string",
					"description": "自然语言查询或关键词。earliest/latest 模式通常留空。",
				},
				"time_hint": map[string]any{
					"type":        "string",
					"description": "自然语言时间表达，例如昨天、今天上午、上周、2026-06-23 晚上。仅在需要时间过滤时填写。",
				},
				"role": map[string]any{
					"type":        "string",
					"description": "可选角色过滤，允许 user 或 assistant。",
					"enum":        []string{"user", "assistant"},
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "返回历史原文条数，默认 8，最大 20。",
					"minimum":     1,
					"maximum":     20,
				},
			},
		},
	})
	registry.Register(Capability{
		Key:         "agent.schedule_message",
		Name:        "定时发送消息",
		Description: "创建当前企微用户的定时提醒或定时发送消息任务。模型负责把自然语言时间归一化为 scheduled_at；后端只做校验和写入。默认需要用户明确确认后才能创建。",
		Mode:        CapabilityModeDeferred,
		Risk:        CapabilityRiskMedium,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_type": map[string]any{
					"type":        "string",
					"description": "任务类型，reminder 为提醒，send_message 为定时发送消息。",
					"enum":        []string{"reminder", "send_message"},
				},
				"content": map[string]any{
					"type":        "string",
					"description": "到时发送给当前企微用户的文本内容。",
				},
				"scheduled_at": map[string]any{
					"type":        "string",
					"description": "模型归一化后的明确发送时间，优先使用 RFC3339，例如 2026-06-24T21:55:00+08:00。创建任务时优先填写该字段。",
				},
				"time_hint": map[string]any{
					"type":        "string",
					"description": "原始自然语言时间表达，仅作为辅助证据，例如今天晚上9点55、明天上午9点。",
				},
				"time_zone": map[string]any{
					"type":        "string",
					"description": "用于解释 scheduled_at 或 time_hint 的时区，默认 Asia/Shanghai。",
				},
				"importance": map[string]any{
					"type":        "string",
					"description": "重要性。",
					"enum":        []string{"normal", "high"},
				},
				"confirmed": map[string]any{
					"type":        "boolean",
					"description": "仅当用户已经明确确认创建该定时任务时才为 true。用户回复确认上一轮待创建提醒时，必须重新调用本工具并设为 true。",
				},
			},
			"required": []string{"task_type", "content"},
		},
	})
	registry.Register(Capability{
		Key:         "content.summarize_text",
		Name:        "总结文本",
		Description: "对用户输入或条目摘要生成简短总结。",
		Mode:        CapabilityModeCore,
		Risk:        CapabilityRiskLow,
	})
	registry.Register(Capability{
		Key:         "agent.write_transcript",
		Name:        "写入对话记录",
		Description: "写入本轮 transcript。",
		Mode:        CapabilityModeHidden,
		Risk:        CapabilityRiskLow,
	})
	registry.Register(Capability{
		Key:         "agent.write_audit",
		Name:        "写入审计记录",
		Description: "写入本轮审计事件。",
		Mode:        CapabilityModeHidden,
		Risk:        CapabilityRiskLow,
	})
	return registry
}

func (r *CapabilityRegistry) Register(capability Capability) {
	if r == nil {
		return
	}
	key := strings.TrimSpace(capability.Key)
	if key == "" {
		return
	}
	if r.byKey == nil {
		r.byKey = map[string]Capability{}
	}
	capability.Key = key
	r.byKey[key] = capability
}

func (r *CapabilityRegistry) Get(key string) (Capability, bool) {
	if r == nil || r.byKey == nil {
		return Capability{}, false
	}
	capability, ok := r.byKey[strings.TrimSpace(key)]
	return capability, ok
}

func (r *CapabilityRegistry) List() []Capability {
	if r == nil || len(r.byKey) == 0 {
		return nil
	}
	capabilities := make([]Capability, 0, len(r.byKey))
	for _, capability := range r.byKey {
		capabilities = append(capabilities, capability)
	}
	return capabilities
}

type PolicyEngine struct{}

func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{}
}

type PolicyInput struct {
	Capability Capability
	UserID     int64
}

type PolicyResult struct {
	Decision PolicyDecision
	Reason   string
}

func (e *PolicyEngine) Decide(_ context.Context, input PolicyInput) PolicyResult {
	if input.UserID < 1 {
		return PolicyResult{Decision: PolicyDecisionForbidden, Reason: "missing authenticated user"}
	}
	if strings.TrimSpace(input.Capability.Key) == "" {
		return PolicyResult{Decision: PolicyDecisionForbidden, Reason: "capability is not registered"}
	}
	if input.Capability.Mutates {
		return PolicyResult{Decision: PolicyDecisionPrompt, Reason: "state-changing capability requires approval"}
	}
	if input.Capability.Risk == CapabilityRiskHigh {
		return PolicyResult{Decision: PolicyDecisionPrompt, Reason: "high risk capability requires approval"}
	}
	return PolicyResult{Decision: PolicyDecisionAllow, Reason: "read-only P0 capability"}
}
