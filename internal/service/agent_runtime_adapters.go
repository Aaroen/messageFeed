package service

import (
	"context"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"strconv"
	"strings"
	"time"
)

type agentUserContextBlockProvider struct {
	provider AgentUserContextProvider
	now      func() time.Time
}

func (p agentUserContextBlockProvider) BuildUserContextBlock(ctx context.Context, userID int64) (agent.ContextBlock, error) {
	if p.provider == nil {
		return agent.ContextBlock{}, nil
	}
	result, err := p.provider.BuildAgentUserContext(ctx, userID)
	if err != nil {
		return agent.ContextBlock{}, err
	}
	content := strings.TrimSpace(result.Prompt.PlainText)
	if content == "" {
		return agent.ContextBlock{}, nil
	}
	now := time.Now
	if p.now != nil {
		now = p.now
	}
	return agent.ContextBlock{
		Name:          "用户上下文",
		CapabilityKey: "user.profile.read",
		Content:       content,
		ItemCount:     1,
		GeneratedAt:   now().UTC(),
		TrustLevel:    "user_profile",
	}, nil
}

type agentP0CapabilityExecutor struct {
	recentItems    AgentRecentItemsProvider
	sourceProvider AgentSourceProvider
	now            func() time.Time
}

func (e agentP0CapabilityExecutor) Execute(ctx context.Context, input agent.CapabilityExecuteInput) (agent.CapabilityExecuteResult, error) {
	switch input.Capability.Key {
	case "feed.query_recent_items":
		return e.queryRecentItems(ctx, input)
	case "source.query_latest_items":
		return e.querySourceLatestItems(ctx, input)
	default:
		return agent.CapabilityExecuteResult{
			Observation: agent.CapabilityObservation{
				Capability: input.Capability.Key,
				Decision:   string(agent.PolicyDecisionForbidden),
				Status:     "skipped",
				Summary:    "capability executor does not support this capability",
			},
		}, nil
	}
}

func (e agentP0CapabilityExecutor) queryRecentItems(ctx context.Context, input agent.CapabilityExecuteInput) (agent.CapabilityExecuteResult, error) {
	observation := agent.CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(agent.PolicyDecisionAllow),
	}
	if e.recentItems == nil {
		observation.Status = "skipped"
		observation.Summary = "recent items provider is unavailable"
		return agent.CapabilityExecuteResult{Observation: observation}, nil
	}
	result, err := e.recentItems.ListItems(ctx, ListItemsInput{
		UserID:        input.UserID,
		Limit:         5,
		Offset:        0,
		IncludeHidden: false,
		Order:         string(domain.ItemSortOrderDesc),
	})
	if err != nil {
		return agent.CapabilityExecuteResult{}, err
	}
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("loaded %d recent items", len(result.Items))
	return agent.CapabilityExecuteResult{
		Blocks: []agent.ContextBlock{
			{
				Name:          "最近条目",
				CapabilityKey: input.Capability.Key,
				Content:       formatRecentItemsBlock(result.Items),
				ItemCount:     len(result.Items),
				GeneratedAt:   e.currentTime(),
				TrustLevel:    "database",
			},
		},
		Observation: observation,
	}, nil
}

func (e agentP0CapabilityExecutor) querySourceLatestItems(ctx context.Context, input agent.CapabilityExecuteInput) (agent.CapabilityExecuteResult, error) {
	observation := agent.CapabilityObservation{
		Capability: input.Capability.Key,
		Decision:   string(agent.PolicyDecisionAllow),
	}
	if e.sourceProvider == nil || e.recentItems == nil {
		observation.Status = "skipped"
		observation.Summary = "source or item provider is unavailable"
		return agent.CapabilityExecuteResult{Observation: observation}, nil
	}
	source, found, err := e.matchSourceByText(ctx, input.UserID, input.Message)
	if err != nil {
		return agent.CapabilityExecuteResult{}, err
	}
	if !found {
		observation.Status = "skipped"
		observation.Summary = "no source name matched user input"
		return agent.CapabilityExecuteResult{Observation: observation}, nil
	}
	result, err := e.recentItems.ListItems(ctx, ListItemsInput{
		UserID:        input.UserID,
		SourceID:      source.ID,
		Limit:         3,
		Offset:        0,
		IncludeHidden: false,
		Order:         string(domain.ItemSortOrderDesc),
	})
	if err != nil {
		return agent.CapabilityExecuteResult{}, err
	}
	observation.Status = "succeeded"
	observation.Summary = fmt.Sprintf("loaded %d latest items for source %s", len(result.Items), source.Name)
	return agent.CapabilityExecuteResult{
		Blocks: []agent.ContextBlock{
			{
				Name:          "匹配来源最新条目",
				CapabilityKey: input.Capability.Key,
				Content:       formatSourceLatestItemsBlock(source, result.Items),
				ItemCount:     len(result.Items),
				GeneratedAt:   e.currentTime(),
				TrustLevel:    "database",
			},
		},
		Observation: observation,
	}, nil
}

func (e agentP0CapabilityExecutor) matchSourceByText(ctx context.Context, userID int64, text string) (domain.Source, bool, error) {
	sources, err := e.sourceProvider.ListSources(ctx, userID)
	if err != nil {
		return domain.Source{}, false, err
	}
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return domain.Source{}, false, nil
	}
	for _, source := range sources {
		name := strings.ToLower(strings.TrimSpace(source.Name))
		if name != "" && strings.Contains(text, name) {
			return source, true, nil
		}
	}
	return domain.Source{}, false, nil
}

func (e agentP0CapabilityExecutor) currentTime() time.Time {
	if e.now != nil {
		return e.now().UTC()
	}
	return time.Now().UTC()
}

func formatRecentItemsBlock(items []domain.Item) string {
	if len(items) == 0 {
		return "暂无可用条目。"
	}
	var builder strings.Builder
	for i, item := range items {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(strconv.Itoa(i + 1))
		builder.WriteString(". ")
		builder.WriteString(item.Title)
		if item.SourceName != "" {
			builder.WriteString("（")
			builder.WriteString(item.SourceName)
			builder.WriteString("）")
		}
		if item.Summary != "" {
			builder.WriteString("：")
			builder.WriteString(truncateError(item.Summary, 160))
		}
	}
	return builder.String()
}

func formatSourceLatestItemsBlock(source domain.Source, items []domain.Item) string {
	var builder strings.Builder
	builder.WriteString(source.Name)
	builder.WriteString("：")
	if len(items) == 0 {
		builder.WriteString("暂无可用条目。")
		return builder.String()
	}
	for i, item := range items {
		builder.WriteString("\n")
		builder.WriteString(strconv.Itoa(i + 1))
		builder.WriteString(". ")
		builder.WriteString(item.Title)
	}
	return builder.String()
}
