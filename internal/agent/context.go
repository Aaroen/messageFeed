package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type UserContextProvider interface {
	BuildUserContextBlock(ctx context.Context, userID int64) (ContextBlock, error)
}

type CapabilityExecutor interface {
	Execute(ctx context.Context, input CapabilityExecuteInput) (CapabilityExecuteResult, error)
}

type CapabilityExecuteInput struct {
	Capability Capability
	UserID     int64
	SessionID  int64
	TurnID     int64
	Message    string
}

type CapabilityExecuteResult struct {
	Blocks      []ContextBlock
	Observation CapabilityObservation
}

type DefaultContextBuilder struct {
	registry            *CapabilityRegistry
	policy              *PolicyEngine
	userContextProvider UserContextProvider
	executor            CapabilityExecutor
	capabilityKeys      []string
	now                 func() time.Time
}

type DefaultContextBuilderOptions struct {
	Registry            *CapabilityRegistry
	Policy              *PolicyEngine
	UserContextProvider UserContextProvider
	Executor            CapabilityExecutor
	CapabilityKeys      []string
	Now                 func() time.Time
}

func NewDefaultContextBuilder(options DefaultContextBuilderOptions) *DefaultContextBuilder {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	registry := options.Registry
	if registry == nil {
		registry = NewP0CapabilityRegistry()
	}
	policy := options.Policy
	if policy == nil {
		policy = NewPolicyEngine()
	}
	capabilityKeys := append([]string(nil), options.CapabilityKeys...)
	if len(capabilityKeys) == 0 {
		capabilityKeys = []string{"feed.query_recent_items", "source.query_latest_items"}
	}
	return &DefaultContextBuilder{
		registry:            registry,
		policy:              policy,
		userContextProvider: options.UserContextProvider,
		executor:            options.Executor,
		capabilityKeys:      capabilityKeys,
		now:                 now,
	}
}

func (b *DefaultContextBuilder) Build(ctx context.Context, input ContextBuildInput) (ContextSnapshot, error) {
	snapshot := ContextSnapshot{
		Blocks:       make([]ContextBlock, 0, len(b.capabilityKeys)+2),
		Observations: make([]CapabilityObservation, 0, len(b.capabilityKeys)+1),
	}
	if b == nil {
		return snapshot, nil
	}

	if b.userContextProvider != nil {
		block, err := b.userContextProvider.BuildUserContextBlock(ctx, input.UserID)
		if err != nil {
			return snapshot, err
		}
		if strings.TrimSpace(block.Content) != "" {
			if block.Name == "" {
				block.Name = "用户上下文"
			}
			if block.GeneratedAt.IsZero() {
				block.GeneratedAt = b.now().UTC()
			}
			snapshot.Blocks = append(snapshot.Blocks, block)
			snapshot.Observations = append(snapshot.Observations, CapabilityObservation{
				Capability: "user.context",
				Decision:   string(PolicyDecisionAllow),
				Status:     "succeeded",
				Summary:    "loaded user context",
			})
		}
	}

	for _, key := range b.capabilityKeys {
		capability, ok := b.registry.Get(key)
		if !ok {
			snapshot.Observations = append(snapshot.Observations, CapabilityObservation{
				Capability: key,
				Decision:   string(PolicyDecisionForbidden),
				Status:     "skipped",
				Summary:    "capability is not registered",
			})
			continue
		}
		decision := b.policy.Decide(ctx, PolicyInput{Capability: capability, UserID: input.UserID})
		if decision.Decision != PolicyDecisionAllow {
			snapshot.Observations = append(snapshot.Observations, CapabilityObservation{
				Capability: capability.Key,
				Decision:   string(decision.Decision),
				Status:     "blocked",
				Summary:    decision.Reason,
			})
			continue
		}
		if b.executor == nil {
			snapshot.Observations = append(snapshot.Observations, CapabilityObservation{
				Capability: capability.Key,
				Decision:   string(decision.Decision),
				Status:     "skipped",
				Summary:    "capability executor is unavailable",
			})
			continue
		}
		result, err := b.executor.Execute(ctx, CapabilityExecuteInput{
			Capability: capability,
			UserID:     input.UserID,
			SessionID:  input.SessionID,
			TurnID:     input.TurnID,
			Message:    input.MessageText,
		})
		if err != nil {
			return snapshot, fmt.Errorf("%s: %w", capability.Key, err)
		}
		observation := result.Observation
		if observation.Capability == "" {
			observation.Capability = capability.Key
		}
		if observation.Decision == "" {
			observation.Decision = string(decision.Decision)
		}
		if observation.Status == "" {
			observation.Status = "succeeded"
		}
		snapshot.Observations = append(snapshot.Observations, observation)
		for _, block := range result.Blocks {
			if strings.TrimSpace(block.Content) == "" {
				continue
			}
			if block.CapabilityKey == "" {
				block.CapabilityKey = capability.Key
			}
			if block.GeneratedAt.IsZero() {
				block.GeneratedAt = b.now().UTC()
			}
			snapshot.Blocks = append(snapshot.Blocks, block)
		}
	}

	snapshot.Blocks = append(snapshot.Blocks, ContextBlock{
		Name:          "可用能力边界",
		CapabilityKey: "capability.list_available",
		Content:       b.capabilityBoundaryText(),
		ItemCount:     len(b.capabilityKeys),
		GeneratedAt:   b.now().UTC(),
		TrustLevel:    "system",
	})
	return snapshot, nil
}

func (b *DefaultContextBuilder) capabilityBoundaryText() string {
	if b == nil || b.registry == nil {
		return "当前没有可用能力。"
	}
	var builder strings.Builder
	builder.WriteString("当前只允许使用只读能力。")
	for _, key := range b.capabilityKeys {
		capability, ok := b.registry.Get(key)
		if !ok {
			continue
		}
		builder.WriteString("\n")
		builder.WriteString(capability.Key)
		builder.WriteString("：")
		builder.WriteString(capability.Description)
	}
	return builder.String()
}
