package repository

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
)

type agentRunModel struct {
	ID              int64 `gorm:"primaryKey"`
	ParentRunID     *int64
	SessionID       *int64
	TurnID          *int64
	Role            string
	Status          string
	TaskPacket      domain.AgentJSON `gorm:"column:task_packet_json;serializer:json;type:jsonb;not null"`
	CapabilityScope []string         `gorm:"column:capability_scope_json;serializer:json;type:jsonb;not null"`
	ModelKey        string
	ContextBudget   domain.AgentJSON `gorm:"column:context_budget_json;serializer:json;type:jsonb;not null"`
	ContextTraceRef string
	ResultRef       string
	ErrorMessage    string
	TraceID         string
	StartedAt       time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type agentRunContextTraceModel struct {
	ID              int64 `gorm:"primaryKey"`
	RunID           int64 `gorm:"not null"`
	TraceKind       string
	PromptVersion   string
	ModelKey        string
	Content         domain.AgentJSON `gorm:"column:content_json;serializer:json;type:jsonb;not null"`
	ContentHash     string
	RedactionStatus string
	TokenEstimate   int
	CreatedAt       time.Time
}

type agentObservationModel struct {
	ID            int64 `gorm:"primaryKey"`
	RunID         int64 `gorm:"not null"`
	CapabilityKey string
	InputSummary  string
	OutputSummary string
	Status        string
	Error         string
	ArtifactRefs  []string `gorm:"column:artifact_refs_json;serializer:json;type:jsonb;not null"`
	CreatedAt     time.Time
}

type agentArtifactModel struct {
	ID           int64 `gorm:"primaryKey"`
	RunID        int64 `gorm:"not null"`
	ArtifactType string
	ContentRef   string
	Summary      string
	SourceRefs   []string `gorm:"column:source_refs_json;serializer:json;type:jsonb;not null"`
	ContentHash  string
	CreatedAt    time.Time
}

func (agentRunModel) TableName() string             { return "agent_runs" }
func (agentRunContextTraceModel) TableName() string { return "agent_run_context_traces" }
func (agentObservationModel) TableName() string     { return "agent_observations" }
func (agentArtifactModel) TableName() string        { return "agent_artifacts" }

var agentRunUpdateColumns = []string{
	"Status",
	"TaskPacket",
	"CapabilityScope",
	"ContextBudget",
	"ContextTraceRef",
	"ResultRef",
	"ErrorMessage",
	"CompletedAt",
	"UpdatedAt",
}

func (r *AgentRepository) CreateAgentRun(ctx context.Context, run domain.AgentRun) (domain.AgentRun, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_run.create", "insert", "agent_runs")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentRunModelFromDomain(normalizeAgentRun(run))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentRun{}, opErr
	}
	return agentRunModelToDomain(model), nil
}

func (r *AgentRepository) UpdateAgentRun(ctx context.Context, run domain.AgentRun) (domain.AgentRun, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_run.update", "update", "agent_runs")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentRunModelFromDomain(normalizeAgentRun(run))
	result := r.db.WithContext(ctx).
		Model(&agentRunModel{}).
		Where("id = ?", run.ID).
		Select(agentRunUpdateColumns).
		Updates(&model)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentRun{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentRun{}, opErr
	}
	var updated agentRunModel
	if err := r.db.WithContext(ctx).Where("id = ?", run.ID).First(&updated).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentRun{}, opErr
	}
	return agentRunModelToDomain(updated), nil
}

func (r *AgentRepository) CreateAgentRunContextTrace(ctx context.Context, trace domain.AgentRunContextTrace) (domain.AgentRunContextTrace, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_run_context_trace.create", "insert", "agent_run_context_traces")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentRunContextTraceModelFromDomain(normalizeAgentRunContextTrace(trace))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentRunContextTrace{}, opErr
	}
	return agentRunContextTraceModelToDomain(model), nil
}

func (r *AgentRepository) CreateAgentObservation(ctx context.Context, observation domain.AgentObservation) (domain.AgentObservation, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_observation.create", "insert", "agent_observations")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentObservationModelFromDomain(normalizeAgentObservation(observation))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentObservation{}, opErr
	}
	persisted := agentObservationModelToDomain(model)
	r.indexObservationFact(ctx, persisted)
	return persisted, nil
}

func (r *AgentRepository) CreateAgentArtifact(ctx context.Context, artifact domain.AgentArtifact) (domain.AgentArtifact, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_artifact.create", "insert", "agent_artifacts")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentArtifactModelFromDomain(normalizeAgentArtifact(artifact))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentArtifact{}, opErr
	}
	persisted := agentArtifactModelToDomain(model)
	r.indexArtifactFact(ctx, persisted)
	return persisted, nil
}

func (r *AgentRepository) ListAgentRunsByTurn(ctx context.Context, userID int64, turnID int64) ([]domain.AgentRun, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_run.list_by_turn", "select", "agent_runs")
	var opErr error
	defer func() { finish(opErr) }()

	var models []agentRunModel
	if err := r.userScopedAgentRuns(ctx, userID).
		Where("agent_runs.turn_id = ?", turnID).
		Order("agent_runs.created_at ASC, agent_runs.id ASC").
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	runs := make([]domain.AgentRun, 0, len(models))
	for _, model := range models {
		runs = append(runs, agentRunModelToDomain(model))
	}
	return runs, nil
}

func (r *AgentRepository) GetAgentRunDetail(ctx context.Context, userID int64, runID int64) (domain.AgentRun, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_run.get_detail", "select", "agent_runs")
	var opErr error
	defer func() { finish(opErr) }()

	var model agentRunModel
	if err := r.userScopedAgentRuns(ctx, userID).
		Where("agent_runs.id = ?", runID).
		First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentRun{}, opErr
	}
	run := agentRunModelToDomain(model)

	var childModels []agentRunModel
	if err := r.db.WithContext(ctx).
		Where("parent_run_id = ?", run.ID).
		Order("created_at ASC, id ASC").
		Find(&childModels).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentRun{}, opErr
	}
	for _, child := range childModels {
		run.ChildRuns = append(run.ChildRuns, agentRunModelToDomain(child))
	}

	var traceModels []agentRunContextTraceModel
	if err := r.db.WithContext(ctx).
		Where("run_id = ?", run.ID).
		Order("created_at ASC, id ASC").
		Find(&traceModels).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentRun{}, opErr
	}
	for _, trace := range traceModels {
		run.ContextTraces = append(run.ContextTraces, agentRunContextTraceModelToDomain(trace))
	}

	var observationModels []agentObservationModel
	if err := r.db.WithContext(ctx).
		Where("run_id = ?", run.ID).
		Order("created_at ASC, id ASC").
		Find(&observationModels).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentRun{}, opErr
	}
	for _, observation := range observationModels {
		run.Observations = append(run.Observations, agentObservationModelToDomain(observation))
	}

	var artifactModels []agentArtifactModel
	if err := r.db.WithContext(ctx).
		Where("run_id = ?", run.ID).
		Order("created_at ASC, id ASC").
		Find(&artifactModels).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentRun{}, opErr
	}
	for _, artifact := range artifactModels {
		run.Artifacts = append(run.Artifacts, agentArtifactModelToDomain(artifact))
	}
	return run, nil
}

func (r *AgentRepository) userScopedAgentRuns(ctx context.Context, userID int64) *gorm.DB {
	return r.db.WithContext(ctx).
		Model(&agentRunModel{}).
		Joins("LEFT JOIN agent_turns ON agent_turns.id = agent_runs.turn_id").
		Joins("LEFT JOIN agent_sessions ON agent_sessions.id = agent_runs.session_id").
		Where("(agent_turns.user_id = ? OR agent_sessions.user_id = ?)", userID, userID).
		Select("agent_runs.*")
}

func normalizeAgentRun(run domain.AgentRun) domain.AgentRun {
	run.ModelKey = strings.TrimSpace(run.ModelKey)
	run.ContextTraceRef = strings.TrimSpace(run.ContextTraceRef)
	run.ResultRef = strings.TrimSpace(run.ResultRef)
	run.ErrorMessage = strings.TrimSpace(run.ErrorMessage)
	run.TraceID = strings.TrimSpace(run.TraceID)
	if !run.Role.Valid() {
		run.Role = domain.AgentRunRoleExecutor
	}
	if !run.Status.Valid() {
		run.Status = domain.AgentRunStatusRunning
	}
	if run.TaskPacket == nil {
		run.TaskPacket = domain.AgentJSON{}
	}
	if run.ContextBudget == nil {
		run.ContextBudget = domain.AgentJSON{}
	}
	return run
}

func normalizeAgentRunContextTrace(trace domain.AgentRunContextTrace) domain.AgentRunContextTrace {
	trace.TraceKind = strings.TrimSpace(trace.TraceKind)
	trace.PromptVersion = strings.TrimSpace(trace.PromptVersion)
	trace.ModelKey = strings.TrimSpace(trace.ModelKey)
	trace.ContentHash = strings.TrimSpace(trace.ContentHash)
	trace.RedactionStatus = strings.TrimSpace(trace.RedactionStatus)
	if trace.RedactionStatus == "" {
		trace.RedactionStatus = "redacted"
	}
	if trace.TokenEstimate < 0 {
		trace.TokenEstimate = 0
	}
	if trace.Content == nil {
		trace.Content = domain.AgentJSON{}
	}
	return trace
}

func normalizeAgentObservation(observation domain.AgentObservation) domain.AgentObservation {
	observation.CapabilityKey = strings.TrimSpace(observation.CapabilityKey)
	observation.InputSummary = strings.TrimSpace(observation.InputSummary)
	observation.OutputSummary = strings.TrimSpace(observation.OutputSummary)
	observation.Status = strings.TrimSpace(observation.Status)
	observation.Error = strings.TrimSpace(observation.Error)
	return observation
}

func normalizeAgentArtifact(artifact domain.AgentArtifact) domain.AgentArtifact {
	artifact.ArtifactType = strings.TrimSpace(artifact.ArtifactType)
	artifact.ContentRef = strings.TrimSpace(artifact.ContentRef)
	artifact.Summary = strings.TrimSpace(artifact.Summary)
	artifact.ContentHash = strings.TrimSpace(artifact.ContentHash)
	return artifact
}

func agentRunModelFromDomain(run domain.AgentRun) agentRunModel {
	return agentRunModel{
		ID:              run.ID,
		ParentRunID:     int64Pointer(run.ParentRunID),
		SessionID:       int64Pointer(run.SessionID),
		TurnID:          int64Pointer(run.TurnID),
		Role:            string(run.Role),
		Status:          string(run.Status),
		TaskPacket:      cloneAgentJSON(run.TaskPacket),
		CapabilityScope: cloneStringSlice(run.CapabilityScope),
		ModelKey:        run.ModelKey,
		ContextBudget:   cloneAgentJSON(run.ContextBudget),
		ContextTraceRef: run.ContextTraceRef,
		ResultRef:       run.ResultRef,
		ErrorMessage:    run.ErrorMessage,
		TraceID:         run.TraceID,
		StartedAt:       run.StartedAt,
		CompletedAt:     run.CompletedAt,
		CreatedAt:       run.CreatedAt,
		UpdatedAt:       run.UpdatedAt,
	}
}

func agentRunModelToDomain(model agentRunModel) domain.AgentRun {
	return domain.AgentRun{
		ID:              model.ID,
		ParentRunID:     int64Value(model.ParentRunID),
		SessionID:       int64Value(model.SessionID),
		TurnID:          int64Value(model.TurnID),
		Role:            domain.AgentRunRole(model.Role),
		Status:          domain.AgentRunStatus(model.Status),
		TaskPacket:      cloneAgentJSON(model.TaskPacket),
		CapabilityScope: cloneStringSlice(model.CapabilityScope),
		ModelKey:        model.ModelKey,
		ContextBudget:   cloneAgentJSON(model.ContextBudget),
		ContextTraceRef: model.ContextTraceRef,
		ResultRef:       model.ResultRef,
		ErrorMessage:    model.ErrorMessage,
		TraceID:         model.TraceID,
		StartedAt:       model.StartedAt,
		CompletedAt:     model.CompletedAt,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}
}

func agentRunContextTraceModelFromDomain(trace domain.AgentRunContextTrace) agentRunContextTraceModel {
	return agentRunContextTraceModel{
		ID:              trace.ID,
		RunID:           trace.RunID,
		TraceKind:       trace.TraceKind,
		PromptVersion:   trace.PromptVersion,
		ModelKey:        trace.ModelKey,
		Content:         cloneAgentJSON(trace.Content),
		ContentHash:     trace.ContentHash,
		RedactionStatus: trace.RedactionStatus,
		TokenEstimate:   trace.TokenEstimate,
		CreatedAt:       trace.CreatedAt,
	}
}

func agentRunContextTraceModelToDomain(model agentRunContextTraceModel) domain.AgentRunContextTrace {
	return domain.AgentRunContextTrace{
		ID:              model.ID,
		RunID:           model.RunID,
		TraceKind:       model.TraceKind,
		PromptVersion:   model.PromptVersion,
		ModelKey:        model.ModelKey,
		Content:         cloneAgentJSON(model.Content),
		ContentHash:     model.ContentHash,
		RedactionStatus: model.RedactionStatus,
		TokenEstimate:   model.TokenEstimate,
		CreatedAt:       model.CreatedAt,
	}
}

func agentObservationModelFromDomain(observation domain.AgentObservation) agentObservationModel {
	return agentObservationModel{
		ID:            observation.ID,
		RunID:         observation.RunID,
		CapabilityKey: observation.CapabilityKey,
		InputSummary:  observation.InputSummary,
		OutputSummary: observation.OutputSummary,
		Status:        observation.Status,
		Error:         observation.Error,
		ArtifactRefs:  cloneStringSlice(observation.ArtifactRefs),
		CreatedAt:     observation.CreatedAt,
	}
}

func agentObservationModelToDomain(model agentObservationModel) domain.AgentObservation {
	return domain.AgentObservation{
		ID:            model.ID,
		RunID:         model.RunID,
		CapabilityKey: model.CapabilityKey,
		InputSummary:  model.InputSummary,
		OutputSummary: model.OutputSummary,
		Status:        model.Status,
		Error:         model.Error,
		ArtifactRefs:  cloneStringSlice(model.ArtifactRefs),
		CreatedAt:     model.CreatedAt,
	}
}

func agentArtifactModelFromDomain(artifact domain.AgentArtifact) agentArtifactModel {
	return agentArtifactModel{
		ID:           artifact.ID,
		RunID:        artifact.RunID,
		ArtifactType: artifact.ArtifactType,
		ContentRef:   artifact.ContentRef,
		Summary:      artifact.Summary,
		SourceRefs:   cloneStringSlice(artifact.SourceRefs),
		ContentHash:  artifact.ContentHash,
		CreatedAt:    artifact.CreatedAt,
	}
}

func agentArtifactModelToDomain(model agentArtifactModel) domain.AgentArtifact {
	return domain.AgentArtifact{
		ID:           model.ID,
		RunID:        model.RunID,
		ArtifactType: model.ArtifactType,
		ContentRef:   model.ContentRef,
		Summary:      model.Summary,
		SourceRefs:   cloneStringSlice(model.SourceRefs),
		ContentHash:  model.ContentHash,
		CreatedAt:    model.CreatedAt,
	}
}
