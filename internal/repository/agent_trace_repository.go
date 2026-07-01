package repository

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

type agentTraceEventModel struct {
	ID            int64 `gorm:"primaryKey"`
	RequestID     string
	TraceID       string
	SpanID        string
	UserID        *int64
	SessionID     *int64
	TurnID        *int64
	PlanID        *int64
	RunID         *int64
	ParentRunID   *int64
	StepID        *int64
	EventKind     string
	EventName     string
	Status        string
	StartedAt     time.Time
	FinishedAt    *time.Time
	DurationMS    int64
	ModelKey      string
	CapabilityKey string
	ToolName      string
	JobID         string
	ArtifactRefs  []string `gorm:"column:artifact_refs_json;serializer:json;type:jsonb;not null"`
	SourceRefs    []string `gorm:"column:source_refs_json;serializer:json;type:jsonb;not null"`
	InputSummary  string
	OutputSummary string
	ErrorCode     string
	ErrorMessage  string
	Metadata      domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt     time.Time
}

func (agentTraceEventModel) TableName() string { return "agent_trace_events" }

func (r *AgentRepository) CreateAgentTraceEvent(ctx context.Context, event domain.AgentTraceEvent) (domain.AgentTraceEvent, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_trace_event.create", "insert", "agent_trace_events")
	var opErr error
	defer func() { finish(opErr) }()

	event = normalizeAgentTraceEvent(event)
	if !event.EventKind.Valid() || !event.Status.Valid() {
		opErr = domain.ErrInvalidInput
		return domain.AgentTraceEvent{}, opErr
	}
	model := agentTraceEventModelFromDomain(event)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentTraceEvent{}, opErr
	}
	return agentTraceEventModelToDomain(model), nil
}

func (r *AgentRepository) ListAgentTraceEvents(ctx context.Context, options domain.AgentTraceEventListOptions) ([]domain.AgentTraceEvent, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_trace_event.list", "select", "agent_trace_events")
	var opErr error
	defer func() { finish(opErr) }()

	query := r.db.WithContext(ctx).Model(&agentTraceEventModel{})
	if options.UserID > 0 {
		query = query.Where("user_id = ?", options.UserID)
	}
	if strings.TrimSpace(options.RequestID) != "" {
		query = query.Where("request_id = ?", strings.TrimSpace(options.RequestID))
	}
	if strings.TrimSpace(options.TraceID) != "" {
		query = query.Where("trace_id = ?", strings.TrimSpace(options.TraceID))
	}
	if options.SessionID > 0 {
		query = query.Where("session_id = ?", options.SessionID)
	}
	if options.TurnID > 0 {
		query = query.Where("turn_id = ?", options.TurnID)
	}
	if options.PlanID > 0 {
		query = query.Where("plan_id = ?", options.PlanID)
	}
	if options.RunID > 0 {
		query = query.Where("(run_id = ? OR parent_run_id = ?)", options.RunID, options.RunID)
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	var models []agentTraceEventModel
	if err := query.Order("created_at ASC, id ASC").Limit(limit).Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	events := make([]domain.AgentTraceEvent, 0, len(models))
	for _, model := range models {
		events = append(events, agentTraceEventModelToDomain(model))
	}
	return events, nil
}

func normalizeAgentTraceEvent(event domain.AgentTraceEvent) domain.AgentTraceEvent {
	event.RequestID = strings.TrimSpace(event.RequestID)
	event.TraceID = strings.TrimSpace(event.TraceID)
	event.SpanID = strings.TrimSpace(event.SpanID)
	event.EventName = strings.TrimSpace(event.EventName)
	event.ModelKey = strings.TrimSpace(event.ModelKey)
	event.CapabilityKey = strings.TrimSpace(event.CapabilityKey)
	event.ToolName = strings.TrimSpace(event.ToolName)
	event.JobID = strings.TrimSpace(event.JobID)
	event.InputSummary = strings.TrimSpace(event.InputSummary)
	event.OutputSummary = strings.TrimSpace(event.OutputSummary)
	event.ErrorCode = strings.TrimSpace(event.ErrorCode)
	event.ErrorMessage = strings.TrimSpace(event.ErrorMessage)
	if event.StartedAt.IsZero() {
		event.StartedAt = time.Now().UTC()
	}
	if event.DurationMS < 0 {
		event.DurationMS = 0
	}
	if event.Metadata == nil {
		event.Metadata = domain.AgentJSON{}
	}
	return event
}

func agentTraceEventModelFromDomain(event domain.AgentTraceEvent) agentTraceEventModel {
	return agentTraceEventModel{
		ID:            event.ID,
		RequestID:     event.RequestID,
		TraceID:       event.TraceID,
		SpanID:        event.SpanID,
		UserID:        int64Pointer(event.UserID),
		SessionID:     int64Pointer(event.SessionID),
		TurnID:        int64Pointer(event.TurnID),
		PlanID:        int64Pointer(event.PlanID),
		RunID:         int64Pointer(event.RunID),
		ParentRunID:   int64Pointer(event.ParentRunID),
		StepID:        int64Pointer(event.StepID),
		EventKind:     string(event.EventKind),
		EventName:     event.EventName,
		Status:        string(event.Status),
		StartedAt:     event.StartedAt,
		FinishedAt:    event.FinishedAt,
		DurationMS:    event.DurationMS,
		ModelKey:      event.ModelKey,
		CapabilityKey: event.CapabilityKey,
		ToolName:      event.ToolName,
		JobID:         event.JobID,
		ArtifactRefs:  cloneStringSlice(event.ArtifactRefs),
		SourceRefs:    cloneStringSlice(event.SourceRefs),
		InputSummary:  event.InputSummary,
		OutputSummary: event.OutputSummary,
		ErrorCode:     event.ErrorCode,
		ErrorMessage:  event.ErrorMessage,
		Metadata:      cloneAgentJSON(event.Metadata),
		CreatedAt:     event.CreatedAt,
	}
}

func agentTraceEventModelToDomain(model agentTraceEventModel) domain.AgentTraceEvent {
	return domain.AgentTraceEvent{
		ID:            model.ID,
		RequestID:     model.RequestID,
		TraceID:       model.TraceID,
		SpanID:        model.SpanID,
		UserID:        int64Value(model.UserID),
		SessionID:     int64Value(model.SessionID),
		TurnID:        int64Value(model.TurnID),
		PlanID:        int64Value(model.PlanID),
		RunID:         int64Value(model.RunID),
		ParentRunID:   int64Value(model.ParentRunID),
		StepID:        int64Value(model.StepID),
		EventKind:     domain.AgentTraceEventKind(model.EventKind),
		EventName:     model.EventName,
		Status:        domain.AgentTraceEventStatus(model.Status),
		StartedAt:     model.StartedAt,
		FinishedAt:    model.FinishedAt,
		DurationMS:    model.DurationMS,
		ModelKey:      model.ModelKey,
		CapabilityKey: model.CapabilityKey,
		ToolName:      model.ToolName,
		JobID:         model.JobID,
		ArtifactRefs:  cloneStringSlice(model.ArtifactRefs),
		SourceRefs:    cloneStringSlice(model.SourceRefs),
		InputSummary:  model.InputSummary,
		OutputSummary: model.OutputSummary,
		ErrorCode:     model.ErrorCode,
		ErrorMessage:  model.ErrorMessage,
		Metadata:      cloneAgentJSON(model.Metadata),
		CreatedAt:     model.CreatedAt,
	}
}
