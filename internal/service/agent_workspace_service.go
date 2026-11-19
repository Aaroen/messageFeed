package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

type agentWorkspaceRepository interface {
	GetAgentWorkspace(context.Context, int64, int64, int64) (domain.AgentWorkspaceData, error)
	GetAgentWorkspaceState(context.Context, int64, int64) (domain.AgentWorkspaceData, error)
	ListAgentStreamEvents(context.Context, int64, int64, int64, int) ([]domain.AgentStreamEvent, error)
}

func (s *AgentSessionService) CancelWorkspaceTurn(ctx context.Context, auth CurrentAuth, turnID int64) (AgentTurnResponse, error) {
	if !auth.Authenticated || auth.User.ID < 1 || turnID < 1 {
		return AgentTurnResponse{}, fmt.Errorf("%w: authenticated user and turn are required", domain.ErrInvalidInput)
	}
	store, ok := s.repository.(interface {
		RequestAgentTurnCancel(context.Context, int64, int64, string, time.Time) (domain.AgentTurn, error)
	})
	if !ok {
		return AgentTurnResponse{}, fmt.Errorf("agent cancellation store is unavailable")
	}
	turn, err := store.RequestAgentTurnCancel(ctx, auth.User.ID, turnID, "用户停止执行", s.now().UTC())
	if err != nil {
		return AgentTurnResponse{}, err
	}
	return agentTurnResponse(turn), nil
}

type AgentStreamItem struct {
	RunID    int64  `json:"run_id"`
	TurnID   int64  `json:"turn_id"`
	ItemID   string `json:"item_id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	Stage    string `json:"stage"`
	Phase    string `json:"phase"`
	ModelKey string `json:"model_key"`
}

type AgentWorkspaceResponse struct {
	Session AgentSessionResponse `json:"session"`
	Turns   []AgentTurnResponse  `json:"turns"`
	Runs    []AgentActivity      `json:"runs"`
	Plans   []AgentPlanResponse  `json:"plans"`
	Items   []AgentStreamItem    `json:"items"`
	Cursor  int64                `json:"cursor"`
	HasMore bool                 `json:"has_more"`
}

type AgentActivity struct {
	ID          int64    `json:"id"`
	ParentRunID int64    `json:"parent_run_id"`
	TurnID      int64    `json:"turn_id"`
	Kind        string   `json:"kind"`
	Stage       string   `json:"stage,omitempty"`
	Capability  string   `json:"capability,omitempty"`
	Title       string   `json:"title,omitempty"`
	Status      string   `json:"status"`
	StartedAt   string   `json:"started_at"`
	CompletedAt string   `json:"completed_at,omitempty"`
	UpdatedAt   string   `json:"updated_at"`
	Results     []string `json:"results,omitempty"`
}

func PublicAgentActivity(run AgentRunResponse) AgentActivity {
	activity := AgentActivity{
		ID: run.ID, ParentRunID: run.ParentRunID, TurnID: run.TurnID, Status: run.Status,
		StartedAt: run.StartedAt, CompletedAt: run.CompletedAt, UpdatedAt: run.UpdatedAt,
	}
	activity.Stage, _ = run.TaskPacket["stage"].(string)
	activity.Capability, _ = run.TaskPacket["capability_key"].(string)
	switch {
	case run.Role == "controller":
		activity.Kind = "controller"
	case activity.Capability != "":
		activity.Kind = "tool"
	case activity.Stage != "":
		activity.Kind = "model"
	default:
		activity.Kind = "subagent"
		for _, key := range []string{"title", "goal", "task", "description"} {
			value, _ := run.TaskPacket[key].(string)
			if title := publicActivityText(value); title != "" {
				activity.Title = title
				break
			}
		}
	}
	if activity.Kind == "subagent" || activity.Kind == "tool" {
		for _, artifact := range run.Artifacts {
			if summary := publicActivityText(artifact.Summary); summary != "" {
				activity.Results = append(activity.Results, summary)
			}
		}
	}
	return activity
}

func publicActivityText(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "{") || strings.HasPrefix(value, "[") || strings.HasPrefix(value, "```") {
		return ""
	}
	return value
}

func (s *AgentSessionService) GetWorkspace(ctx context.Context, auth CurrentAuth, sessionID, beforeTurnID int64) (AgentWorkspaceResponse, error) {
	if !auth.Authenticated || auth.User.ID < 1 || sessionID < 1 || beforeTurnID < 0 {
		return AgentWorkspaceResponse{}, fmt.Errorf("%w: authenticated user and session are required", domain.ErrInvalidInput)
	}
	store, ok := s.repository.(agentWorkspaceRepository)
	if !ok {
		return AgentWorkspaceResponse{}, fmt.Errorf("agent workspace store is unavailable")
	}
	data, err := store.GetAgentWorkspace(ctx, auth.User.ID, sessionID, beforeTurnID)
	if err != nil {
		return AgentWorkspaceResponse{}, err
	}
	return agentWorkspaceResponse(data), nil
}

func (s *AgentSessionService) GetWorkspaceState(ctx context.Context, auth CurrentAuth, sessionID int64) (AgentWorkspaceResponse, error) {
	if !auth.Authenticated || auth.User.ID < 1 || sessionID < 1 {
		return AgentWorkspaceResponse{}, fmt.Errorf("%w: invalid workspace scope", domain.ErrInvalidInput)
	}
	store, ok := s.repository.(agentWorkspaceRepository)
	if !ok {
		return AgentWorkspaceResponse{}, fmt.Errorf("agent workspace store is unavailable")
	}
	data, err := store.GetAgentWorkspaceState(ctx, auth.User.ID, sessionID)
	if err != nil {
		return AgentWorkspaceResponse{}, err
	}
	return agentWorkspaceResponse(data), nil
}

func agentWorkspaceResponse(data domain.AgentWorkspaceData) AgentWorkspaceResponse {
	result := AgentWorkspaceResponse{Session: agentSessionResponse(data.Session, domain.AgentSessionStats{}, false), Turns: []AgentTurnResponse{}, Runs: []AgentActivity{}, Plans: []AgentPlanResponse{}, Items: agentStreamItems(data.Events), Cursor: data.Cursor, HasMore: data.HasMore}
	for _, turn := range data.Turns {
		result.Turns = append(result.Turns, agentTurnResponse(turn))
	}
	for _, run := range data.Runs {
		result.Runs = append(result.Runs, PublicAgentActivity(agentRunResponse(run, false)))
	}
	for _, plan := range data.Plans {
		result.Plans = append(result.Plans, agentPlanResponse(plan, false))
	}
	return result
}

func (s *AgentSessionService) GetWorkspaceEvents(ctx context.Context, auth CurrentAuth, sessionID, afterID int64) ([]domain.AgentStreamEvent, error) {
	if !auth.Authenticated || auth.User.ID < 1 || sessionID < 1 || afterID < 0 {
		return nil, fmt.Errorf("%w: invalid stream scope", domain.ErrInvalidInput)
	}
	if _, err := s.repository.GetAgentSession(ctx, auth.User.ID, sessionID); err != nil {
		return nil, err
	}
	store, ok := s.repository.(agentWorkspaceRepository)
	if !ok {
		return nil, fmt.Errorf("agent workspace store is unavailable")
	}
	return store.ListAgentStreamEvents(ctx, auth.User.ID, sessionID, afterID, 200)
}

func agentStreamItems(events []domain.AgentStreamEvent) []AgentStreamItem {
	items := []AgentStreamItem{}
	indices := map[string]int{}
	for _, event := range events {
		key := fmt.Sprintf("%d:%s:%s", event.RunID, event.Kind, event.ItemID)
		index, exists := indices[key]
		if !exists {
			index = len(items)
			indices[key] = index
			items = append(items, AgentStreamItem{RunID: event.RunID, TurnID: event.TurnID, ItemID: event.ItemID, Kind: event.Kind})
		}
		item := &items[index]
		if event.Metadata["replace"] == true {
			item.Content = event.Delta
		} else {
			item.Content += event.Delta
		}
		if event.Name != "" {
			item.Name = event.Name
		}
		if value, ok := event.Metadata["stage"].(string); ok {
			item.Stage = value
		}
		if value, ok := event.Metadata["phase"].(string); ok && value != "" {
			item.Phase = value
		}
		if value, ok := event.Metadata["model_key"].(string); ok {
			item.ModelKey = value
		}
	}
	return items
}
