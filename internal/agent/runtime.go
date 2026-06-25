package agent

import (
	"context"
	"sort"
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
	Key            string
	Name           string
	Description    string
	Mode           CapabilityMode
	Risk           CapabilityRisk
	DataDomain     string
	Mutates        bool
	ExternalAccess bool
	Schedulable    bool
	Reusable       bool
	Parameters     map[string]any
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
		DataDomain:  "local_feed",
		Reusable:    true,
	})
	registry.Register(Capability{
		Key:         "source.query_latest_items",
		Name:        "查询来源最新条目",
		Description: "按来源名称或来源 ID 读取最新条目。",
		Mode:        CapabilityModeCore,
		Risk:        CapabilityRiskLow,
		DataDomain:  "local_feed",
		Reusable:    true,
	})
	registry.Register(Capability{
		Key:         "conversation.query_history",
		Name:        "查询历史聊天",
		Description: "按关键词、时间、角色或会话边界读取当前用户企微长期会话的历史聊天原文。",
		Mode:        CapabilityModeCore,
		Risk:        CapabilityRiskLow,
		DataDomain:  "long_term_memory",
		Reusable:    true,
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
		Description: "兼容旧入口：创建当前企微用户的定时提醒或定时发送消息任务。新任务应优先使用 agent.schedule_task。",
		Mode:        CapabilityModeDeferred,
		Risk:        CapabilityRiskMedium,
		DataDomain:  "notification",
		Mutates:     true,
		Schedulable: true,
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
		Key:         "agent.schedule_task",
		Name:        "创建定时 Agent 任务",
		Description: "创建当前用户的定时 Agent 任务。任务到点后应复用 controller run 闭环执行，可用于提醒、定时检索、定时总结和定时汇报。默认需要用户明确确认。",
		Mode:        CapabilityModeDeferred,
		Risk:        CapabilityRiskMedium,
		DataDomain:  "scheduled_task",
		Mutates:     true,
		Schedulable: true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_type": map[string]any{
					"type":        "string",
					"description": "任务类型，例如 reminder、send_message、digest、research 或 agent_task。",
				},
				"goal": map[string]any{
					"type":        "string",
					"description": "到点后 controller run 需要完成的目标。",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "兼容提醒类任务的用户可见内容；如果 goal 为空，可作为 goal 使用。",
				},
				"scheduled_at": map[string]any{
					"type":        "string",
					"description": "模型归一化后的明确触发时间，优先使用 RFC3339。",
				},
				"time_hint": map[string]any{
					"type":        "string",
					"description": "原始自然语言时间表达，仅作为辅助证据。",
				},
				"time_zone": map[string]any{
					"type":        "string",
					"description": "用于解释 scheduled_at 或 time_hint 的时区，默认 Asia/Shanghai。",
				},
				"target_channel": map[string]any{
					"type":        "string",
					"description": "结果投递通道，默认 wechat_work_app。",
				},
				"freshness_policy": map[string]any{
					"type":        "string",
					"description": "到点执行时的信息新鲜度策略，默认 latest_at_run。",
				},
				"allowed_capabilities": map[string]any{
					"type":        "array",
					"description": "到点任务允许使用的 capability key 列表。",
					"items":       map[string]any{"type": "string"},
				},
				"confirmed": map[string]any{
					"type":        "boolean",
					"description": "仅当用户已经明确确认创建该定时任务时才为 true。",
				},
			},
			"required": []string{"task_type", "goal"},
		},
	})
	registry.Register(Capability{
		Key:            "web.search",
		Name:           "搜索网页",
		Description:    "根据查询词返回候选网页，输出来源、抓取时间和摘要。",
		Mode:           CapabilityModeDeferred,
		Risk:           CapabilityRiskLow,
		DataDomain:     "external_web",
		ExternalAccess: true,
		Reusable:       true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "搜索关键词或自然语言查询。",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "候选结果数量，默认 5，最大 8。",
					"minimum":     1,
					"maximum":     8,
				},
			},
			"required": []string{"query"},
		},
	})
	registry.Register(Capability{
		Key:            "web.fetch_page",
		Name:           "抓取网页",
		Description:    "按 URL 获取网页响应元数据和受限大小的正文片段。",
		Mode:           CapabilityModeDeferred,
		Risk:           CapabilityRiskLow,
		DataDomain:     "external_web",
		ExternalAccess: true,
		Reusable:       true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "需要抓取的 http 或 https URL。",
				},
			},
			"required": []string{"url"},
		},
	})
	registry.Register(Capability{
		Key:            "web.extract_page",
		Name:           "抽取网页正文",
		Description:    "按 URL 抓取页面并抽取标题、正文摘要、主要链接和来源信息。",
		Mode:           CapabilityModeDeferred,
		Risk:           CapabilityRiskLow,
		DataDomain:     "external_web",
		ExternalAccess: true,
		Reusable:       true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "需要抽取正文的 http 或 https URL。",
				},
			},
			"required": []string{"url"},
		},
	})
	registry.Register(Capability{
		Key:            "repo.search",
		Name:           "搜索参考仓库",
		Description:    "根据查询词返回远端仓库候选，输出仓库 URL、描述、语言、许可和更新时间。",
		Mode:           CapabilityModeDeferred,
		Risk:           CapabilityRiskLow,
		DataDomain:     "external_repo",
		ExternalAccess: true,
		Reusable:       true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "仓库搜索关键词。",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "候选仓库数量，默认 5，最大 8。",
					"minimum":     1,
					"maximum":     8,
				},
			},
			"required": []string{"query"},
		},
	})
	registry.Register(Capability{
		Key:            "repo.inspect_remote",
		Name:           "检查远端仓库",
		Description:    "只读检查 GitHub 远端仓库 README、license、默认分支和基础元数据，不克隆到本地。",
		Mode:           CapabilityModeDeferred,
		Risk:           CapabilityRiskLow,
		DataDomain:     "external_repo",
		ExternalAccess: true,
		Reusable:       true,
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo": map[string]any{
					"type":        "string",
					"description": "GitHub 仓库 URL 或 owner/repo。",
				},
			},
			"required": []string{"repo"},
		},
	})
	registry.Register(Capability{
		Key:         "content.summarize_text",
		Name:        "总结文本",
		Description: "对用户输入或条目摘要生成简短总结。",
		Mode:        CapabilityModeCore,
		Risk:        CapabilityRiskLow,
		DataDomain:  "derived_content",
		Reusable:    true,
	})
	registry.Register(Capability{
		Key:         "agent.write_transcript",
		Name:        "写入对话记录",
		Description: "写入本轮 transcript。",
		Mode:        CapabilityModeHidden,
		Risk:        CapabilityRiskLow,
		DataDomain:  "agent_internal",
	})
	registry.Register(Capability{
		Key:         "agent.write_audit",
		Name:        "写入审计记录",
		Description: "写入本轮审计事件。",
		Mode:        CapabilityModeHidden,
		Risk:        CapabilityRiskLow,
		DataDomain:  "agent_internal",
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
	sort.Slice(capabilities, func(i, j int) bool {
		return capabilities[i].Key < capabilities[j].Key
	})
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
	if input.Capability.Schedulable {
		return PolicyResult{Decision: PolicyDecisionPrompt, Reason: "scheduled capability requires approval"}
	}
	if input.Capability.Risk == CapabilityRiskHigh {
		return PolicyResult{Decision: PolicyDecisionPrompt, Reason: "high risk capability requires approval"}
	}
	if input.Capability.ExternalAccess {
		return PolicyResult{Decision: PolicyDecisionAllow, Reason: "external read-only capability with bounded fetch policy"}
	}
	return PolicyResult{Decision: PolicyDecisionAllow, Reason: "read-only capability"}
}
