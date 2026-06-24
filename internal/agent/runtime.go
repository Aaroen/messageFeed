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
		Description: "按关键词、时间和角色读取当前用户企微长期会话的历史聊天原文。",
		Mode:        CapabilityModeCore,
		Risk:        CapabilityRiskLow,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]any{
					"type":        "string",
					"description": "用于匹配历史聊天原文的关键词。若用户在询问整体偏好或过往决定，可使用偏好、关注、决定等词。",
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
				"before_entry_id": map[string]any{
					"type":        "integer",
					"description": "只查询该 transcript entry 之前的历史。",
				},
			},
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
