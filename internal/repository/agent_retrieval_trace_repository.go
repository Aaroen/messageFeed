package repository

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

type agentRecallTraceModel struct {
	ID                   int64 `gorm:"primaryKey"`
	RequestID            string
	TraceID              string
	UserID               *int64
	SessionID            *int64
	TurnID               *int64
	RunID                *int64
	PlanID               *int64
	Mode                 string
	QueryText            string `gorm:"column:query_text"`
	NeedsHistoryRecall   bool
	HistoryQueryPlan     domain.AgentJSON `gorm:"column:history_query_plan_json;serializer:json;type:jsonb;not null"`
	FullTextAttempted    bool
	FullTextCount        int
	FullTextMS           int64
	EmbeddingAttempted   bool
	EmbeddingModel       string
	EmbeddingDimension   int
	EmbeddingMS          int64
	EmbeddingStatus      string
	EmbeddingError       string
	VectorAttempted      bool
	VectorCandidateCount int
	VectorMS             int64
	RelationAttempted    bool
	RelationCount        int
	RelationMS           int64
	FinalHitCount        int
	FinalSources         domain.AgentJSON `gorm:"column:final_sources_json;serializer:json;type:jsonb;not null"`
	FallbackReason       string
	TotalMS              int64
	Status               string
	ErrorMessage         string
	CreatedAt            time.Time
}

type agentEmbeddingTraceModel struct {
	ID                 int64 `gorm:"primaryKey"`
	JobID              string
	RequestID          string
	TraceID            string
	UserID             *int64
	CanonicalRef       string
	EmbeddingModel     string
	EmbeddingDimension int
	InputChars         int
	ContentHash        string
	Status             string
	DurationMS         int64
	ErrorMessage       string
	RetryCount         int
	Metadata           domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt          time.Time
}

func (agentRecallTraceModel) TableName() string    { return "agent_recall_traces" }
func (agentEmbeddingTraceModel) TableName() string { return "agent_embedding_traces" }

func (r *AgentRepository) CreateAgentRecallTrace(ctx context.Context, trace domain.AgentRecallTrace) (domain.AgentRecallTrace, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_recall_trace.create", "insert", "agent_recall_traces")
	var opErr error
	defer func() { finish(opErr) }()

	trace = normalizeAgentRecallTrace(trace)
	if !trace.Status.Valid() {
		opErr = domain.ErrInvalidInput
		return domain.AgentRecallTrace{}, opErr
	}
	model := agentRecallTraceModelFromDomain(trace)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentRecallTrace{}, opErr
	}
	return agentRecallTraceModelToDomain(model), nil
}

func (r *AgentRepository) CreateAgentEmbeddingTrace(ctx context.Context, trace domain.AgentEmbeddingTrace) (domain.AgentEmbeddingTrace, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_embedding_trace.create", "insert", "agent_embedding_traces")
	var opErr error
	defer func() { finish(opErr) }()

	trace = normalizeAgentEmbeddingTrace(trace)
	if !trace.Status.Valid() {
		opErr = domain.ErrInvalidInput
		return domain.AgentEmbeddingTrace{}, opErr
	}
	model := agentEmbeddingTraceModelFromDomain(trace)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentEmbeddingTrace{}, opErr
	}
	return agentEmbeddingTraceModelToDomain(model), nil
}

func normalizeAgentRecallTrace(trace domain.AgentRecallTrace) domain.AgentRecallTrace {
	trace.RequestID = strings.TrimSpace(trace.RequestID)
	trace.TraceID = strings.TrimSpace(trace.TraceID)
	trace.QueryText = strings.TrimSpace(trace.QueryText)
	trace.EmbeddingModel = strings.TrimSpace(trace.EmbeddingModel)
	trace.EmbeddingStatus = strings.TrimSpace(trace.EmbeddingStatus)
	trace.EmbeddingError = strings.TrimSpace(trace.EmbeddingError)
	trace.FallbackReason = strings.TrimSpace(trace.FallbackReason)
	trace.ErrorMessage = strings.TrimSpace(trace.ErrorMessage)
	if !trace.Mode.Valid() {
		trace.Mode = domain.AgentFactRecallModeHybrid
	}
	if !trace.Status.Valid() {
		trace.Status = domain.AgentRecallTraceSucceeded
	}
	if trace.HistoryQueryPlan == nil {
		trace.HistoryQueryPlan = domain.AgentJSON{}
	}
	if trace.FinalSources == nil {
		trace.FinalSources = domain.AgentJSON{}
	}
	if trace.CreatedAt.IsZero() {
		trace.CreatedAt = time.Now().UTC()
	}
	if trace.TotalMS < 0 {
		trace.TotalMS = 0
	}
	return trace
}

func normalizeAgentEmbeddingTrace(trace domain.AgentEmbeddingTrace) domain.AgentEmbeddingTrace {
	trace.JobID = strings.TrimSpace(trace.JobID)
	trace.RequestID = strings.TrimSpace(trace.RequestID)
	trace.TraceID = strings.TrimSpace(trace.TraceID)
	trace.CanonicalRef = strings.TrimSpace(trace.CanonicalRef)
	trace.EmbeddingModel = strings.TrimSpace(trace.EmbeddingModel)
	trace.ContentHash = strings.TrimSpace(trace.ContentHash)
	trace.ErrorMessage = strings.TrimSpace(trace.ErrorMessage)
	if !trace.Status.Valid() {
		trace.Status = domain.AgentEmbeddingTraceSucceeded
	}
	if trace.Metadata == nil {
		trace.Metadata = domain.AgentJSON{}
	}
	if trace.CreatedAt.IsZero() {
		trace.CreatedAt = time.Now().UTC()
	}
	if trace.DurationMS < 0 {
		trace.DurationMS = 0
	}
	return trace
}

func agentRecallTraceModelFromDomain(trace domain.AgentRecallTrace) agentRecallTraceModel {
	return agentRecallTraceModel{
		ID:                   trace.ID,
		RequestID:            trace.RequestID,
		TraceID:              trace.TraceID,
		UserID:               int64Pointer(trace.UserID),
		SessionID:            int64Pointer(trace.SessionID),
		TurnID:               int64Pointer(trace.TurnID),
		RunID:                int64Pointer(trace.RunID),
		PlanID:               int64Pointer(trace.PlanID),
		Mode:                 string(trace.Mode),
		QueryText:            trace.QueryText,
		NeedsHistoryRecall:   trace.NeedsHistoryRecall,
		HistoryQueryPlan:     cloneAgentJSON(trace.HistoryQueryPlan),
		FullTextAttempted:    trace.FullTextAttempted,
		FullTextCount:        trace.FullTextCount,
		FullTextMS:           trace.FullTextMS,
		EmbeddingAttempted:   trace.EmbeddingAttempted,
		EmbeddingModel:       trace.EmbeddingModel,
		EmbeddingDimension:   trace.EmbeddingDimension,
		EmbeddingMS:          trace.EmbeddingMS,
		EmbeddingStatus:      trace.EmbeddingStatus,
		EmbeddingError:       trace.EmbeddingError,
		VectorAttempted:      trace.VectorAttempted,
		VectorCandidateCount: trace.VectorCandidateCount,
		VectorMS:             trace.VectorMS,
		RelationAttempted:    trace.RelationAttempted,
		RelationCount:        trace.RelationCount,
		RelationMS:           trace.RelationMS,
		FinalHitCount:        trace.FinalHitCount,
		FinalSources:         cloneAgentJSON(trace.FinalSources),
		FallbackReason:       trace.FallbackReason,
		TotalMS:              trace.TotalMS,
		Status:               string(trace.Status),
		ErrorMessage:         trace.ErrorMessage,
		CreatedAt:            trace.CreatedAt,
	}
}

func agentRecallTraceModelToDomain(model agentRecallTraceModel) domain.AgentRecallTrace {
	return domain.AgentRecallTrace{
		ID:                   model.ID,
		RequestID:            model.RequestID,
		TraceID:              model.TraceID,
		UserID:               int64Value(model.UserID),
		SessionID:            int64Value(model.SessionID),
		TurnID:               int64Value(model.TurnID),
		RunID:                int64Value(model.RunID),
		PlanID:               int64Value(model.PlanID),
		Mode:                 domain.AgentFactRecallMode(model.Mode),
		QueryText:            model.QueryText,
		NeedsHistoryRecall:   model.NeedsHistoryRecall,
		HistoryQueryPlan:     cloneAgentJSON(model.HistoryQueryPlan),
		FullTextAttempted:    model.FullTextAttempted,
		FullTextCount:        model.FullTextCount,
		FullTextMS:           model.FullTextMS,
		EmbeddingAttempted:   model.EmbeddingAttempted,
		EmbeddingModel:       model.EmbeddingModel,
		EmbeddingDimension:   model.EmbeddingDimension,
		EmbeddingMS:          model.EmbeddingMS,
		EmbeddingStatus:      model.EmbeddingStatus,
		EmbeddingError:       model.EmbeddingError,
		VectorAttempted:      model.VectorAttempted,
		VectorCandidateCount: model.VectorCandidateCount,
		VectorMS:             model.VectorMS,
		RelationAttempted:    model.RelationAttempted,
		RelationCount:        model.RelationCount,
		RelationMS:           model.RelationMS,
		FinalHitCount:        model.FinalHitCount,
		FinalSources:         cloneAgentJSON(model.FinalSources),
		FallbackReason:       model.FallbackReason,
		TotalMS:              model.TotalMS,
		Status:               domain.AgentRecallTraceStatus(model.Status),
		ErrorMessage:         model.ErrorMessage,
		CreatedAt:            model.CreatedAt,
	}
}

func agentEmbeddingTraceModelFromDomain(trace domain.AgentEmbeddingTrace) agentEmbeddingTraceModel {
	return agentEmbeddingTraceModel{
		ID:                 trace.ID,
		JobID:              trace.JobID,
		RequestID:          trace.RequestID,
		TraceID:            trace.TraceID,
		UserID:             int64Pointer(trace.UserID),
		CanonicalRef:       trace.CanonicalRef,
		EmbeddingModel:     trace.EmbeddingModel,
		EmbeddingDimension: trace.EmbeddingDimension,
		InputChars:         trace.InputChars,
		ContentHash:        trace.ContentHash,
		Status:             string(trace.Status),
		DurationMS:         trace.DurationMS,
		ErrorMessage:       trace.ErrorMessage,
		RetryCount:         trace.RetryCount,
		Metadata:           cloneAgentJSON(trace.Metadata),
		CreatedAt:          trace.CreatedAt,
	}
}

func agentEmbeddingTraceModelToDomain(model agentEmbeddingTraceModel) domain.AgentEmbeddingTrace {
	return domain.AgentEmbeddingTrace{
		ID:                 model.ID,
		JobID:              model.JobID,
		RequestID:          model.RequestID,
		TraceID:            model.TraceID,
		UserID:             int64Value(model.UserID),
		CanonicalRef:       model.CanonicalRef,
		EmbeddingModel:     model.EmbeddingModel,
		EmbeddingDimension: model.EmbeddingDimension,
		InputChars:         model.InputChars,
		ContentHash:        model.ContentHash,
		Status:             domain.AgentEmbeddingTraceStatus(model.Status),
		DurationMS:         model.DurationMS,
		ErrorMessage:       model.ErrorMessage,
		RetryCount:         model.RetryCount,
		Metadata:           cloneAgentJSON(model.Metadata),
		CreatedAt:          model.CreatedAt,
	}
}
