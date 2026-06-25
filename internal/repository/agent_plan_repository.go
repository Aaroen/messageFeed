package repository

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type agentPlanModel struct {
	ID                 int64 `gorm:"primaryKey"`
	UserID             int64 `gorm:"not null"`
	SessionID          *int64
	TurnID             *int64
	ControllerRunID    *int64
	Status             string
	Goal               string
	Summary            string
	ImpactSummary      string
	RiskLevel          string
	ConfirmationPolicy string
	AllowedScopes      []string `gorm:"column:allowed_scopes_json;serializer:json;type:jsonb;not null"`
	DedupeKey          string   `gorm:"column:dedupe_key"`
	PolicyDecision     string
	PolicyReason       string
	ExpiresAt          *time.Time
	ApprovedAt         *time.Time
	RejectedAt         *time.Time
	CompletedAt        *time.Time
	FailedAt           *time.Time
	ErrorMessage       string
	Metadata           domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type agentPlanStepModel struct {
	ID              int64 `gorm:"primaryKey"`
	PlanID          int64 `gorm:"not null"`
	StepOrder       int
	Status          string
	CapabilityKey   string
	CapabilityScope []string `gorm:"column:capability_scope_json;serializer:json;type:jsonb;not null"`
	Title           string
	InputSummary    string
	OutputSummary   string
	ExpectedOutput  string
	FailureStrategy string
	ExecutorRunID   *int64
	ObservationRef  string
	ArtifactRefs    []string `gorm:"column:artifact_refs_json;serializer:json;type:jsonb;not null"`
	ErrorMessage    string
	RetryCount      int
	MaxRetries      int
	LastRetryAt     *time.Time
	RetryReason     string
	RetryMetadata   domain.AgentJSON `gorm:"column:retry_metadata_json;serializer:json;type:jsonb;not null"`
	StartedAt       *time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type agentCapabilityAuditLogModel struct {
	ID            int64 `gorm:"primaryKey"`
	UserID        int64 `gorm:"not null"`
	SessionID     *int64
	TurnID        *int64
	RunID         *int64
	PlanID        *int64
	PlanStepID    *int64
	CapabilityKey string
	Decision      string
	Reason        string
	InputSummary  string
	OutputSummary string
	Status        string
	ErrorMessage  string
	SourceRefs    []string         `gorm:"column:source_refs_json;serializer:json;type:jsonb;not null"`
	Metadata      domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt     time.Time
}

func (agentPlanModel) TableName() string               { return "agent_plans" }
func (agentPlanStepModel) TableName() string           { return "agent_plan_steps" }
func (agentCapabilityAuditLogModel) TableName() string { return "agent_capability_audit_logs" }

func (r *AgentRepository) CreateAgentPlan(ctx context.Context, plan domain.AgentPlan, steps []domain.AgentPlanStep) (domain.AgentPlan, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_plan.create", "insert", "agent_plans")
	var opErr error
	defer func() { finish(opErr) }()

	plan = normalizeAgentPlan(plan)
	model := agentPlanModelFromDomain(plan)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		create := tx.Create(&model)
		if create.Error != nil {
			return mapRepositoryError(create.Error)
		}
		for index, step := range steps {
			step = normalizeAgentPlanStep(step)
			step.PlanID = model.ID
			if step.StepOrder < 1 {
				step.StepOrder = index + 1
			}
			stepModel := agentPlanStepModelFromDomain(step)
			if err := tx.Create(&stepModel).Error; err != nil {
				return mapRepositoryError(err)
			}
		}
		return nil
	})
	if err != nil {
		opErr = err
		return domain.AgentPlan{}, err
	}
	return r.GetAgentPlan(ctx, model.UserID, model.ID)
}

func (r *AgentRepository) ListAgentPlans(ctx context.Context, userID int64, sessionID int64, turnID int64, limit int) ([]domain.AgentPlan, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_plan.list", "select", "agent_plans")
	var opErr error
	defer func() { finish(opErr) }()

	if limit < 1 || limit > 100 {
		limit = 50
	}
	query := r.db.WithContext(ctx).Model(&agentPlanModel{}).Where("user_id = ?", userID)
	if sessionID > 0 {
		query = query.Where("session_id = ?", sessionID)
	}
	if turnID > 0 {
		query = query.Where("turn_id = ?", turnID)
	}
	var models []agentPlanModel
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	plans := make([]domain.AgentPlan, 0, len(models))
	for _, model := range models {
		plans = append(plans, agentPlanModelToDomain(model))
	}
	return plans, nil
}

func (r *AgentRepository) GetAgentPlan(ctx context.Context, userID int64, planID int64) (domain.AgentPlan, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_plan.get", "select", "agent_plans")
	var opErr error
	defer func() { finish(opErr) }()

	var model agentPlanModel
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", planID, userID).First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentPlan{}, opErr
	}
	plan := agentPlanModelToDomain(model)

	var stepModels []agentPlanStepModel
	if err := r.db.WithContext(ctx).Where("plan_id = ?", plan.ID).Order("step_order ASC, id ASC").Find(&stepModels).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentPlan{}, opErr
	}
	for _, stepModel := range stepModels {
		plan.Steps = append(plan.Steps, agentPlanStepModelToDomain(stepModel))
	}

	var approvalModels []agentApprovalModel
	if err := r.db.WithContext(ctx).Where("plan_id = ?", plan.ID).Order("created_at DESC, id DESC").Find(&approvalModels).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentPlan{}, opErr
	}
	for _, approvalModel := range approvalModels {
		plan.Approvals = append(plan.Approvals, agentApprovalModelToDomain(approvalModel))
	}
	return plan, nil
}

func (r *AgentRepository) UpdateAgentPlanStatus(ctx context.Context, userID int64, planID int64, status domain.AgentPlanStatus, now time.Time, errorMessage string) (domain.AgentPlan, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_plan.update_status", "update", "agent_plans")
	var opErr error
	defer func() { finish(opErr) }()

	if !status.Valid() {
		opErr = domain.ErrInvalidInput
		return domain.AgentPlan{}, opErr
	}
	updates := map[string]any{
		"status":        string(status),
		"error_message": strings.TrimSpace(errorMessage),
		"updated_at":    now.UTC(),
	}
	switch status {
	case domain.AgentPlanStatusApproved:
		updates["approved_at"] = now.UTC()
	case domain.AgentPlanStatusRejected:
		updates["rejected_at"] = now.UTC()
	case domain.AgentPlanStatusCompleted:
		updates["completed_at"] = now.UTC()
	case domain.AgentPlanStatusFailed:
		updates["failed_at"] = now.UTC()
	}
	result := r.db.WithContext(ctx).Model(&agentPlanModel{}).Where("id = ? AND user_id = ?", planID, userID).Updates(updates)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentPlan{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentPlan{}, opErr
	}
	return r.GetAgentPlan(ctx, userID, planID)
}

func (r *AgentRepository) UpdateAgentPlanMetadata(ctx context.Context, userID int64, planID int64, metadata domain.AgentJSON, now time.Time) (domain.AgentPlan, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_plan.update_metadata", "update", "agent_plans")
	var opErr error
	defer func() { finish(opErr) }()

	if metadata == nil {
		metadata = domain.AgentJSON{}
	}
	result := r.db.WithContext(ctx).Model(&agentPlanModel{}).Where("id = ? AND user_id = ?", planID, userID).Updates(map[string]any{
		"metadata_json": clause.Expr{SQL: "?", Vars: []any{metadata}},
		"updated_at":    now.UTC(),
	})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentPlan{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentPlan{}, opErr
	}
	return r.GetAgentPlan(ctx, userID, planID)
}

func (r *AgentRepository) UpdateAgentPlanStepStatus(ctx context.Context, userID int64, step domain.AgentPlanStep) (domain.AgentPlanStep, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_plan_step.update_status", "update", "agent_plan_steps")
	var opErr error
	defer func() { finish(opErr) }()

	step = normalizeAgentPlanStep(step)
	result := r.db.WithContext(ctx).
		Model(&agentPlanStepModel{}).
		Where("agent_plan_steps.id = ? AND EXISTS (SELECT 1 FROM agent_plans WHERE agent_plans.id = agent_plan_steps.plan_id AND agent_plans.user_id = ?)", step.ID, userID).
		Updates(map[string]any{
			"status":          string(step.Status),
			"output_summary":  step.OutputSummary,
			"executor_run_id": int64Pointer(step.ExecutorRunID),
			"observation_ref": step.ObservationRef,
			"artifact_refs_json": clause.Expr{
				SQL:  "?",
				Vars: []any{step.ArtifactRefs},
			},
			"error_message": step.ErrorMessage,
			"retry_count":   step.RetryCount,
			"max_retries":   step.MaxRetries,
			"last_retry_at": step.LastRetryAt,
			"retry_reason":  step.RetryReason,
			"retry_metadata_json": clause.Expr{
				SQL:  "?",
				Vars: []any{step.RetryMetadata},
			},
			"started_at":   step.StartedAt,
			"completed_at": step.CompletedAt,
			"updated_at":   time.Now().UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentPlanStep{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentPlanStep{}, opErr
	}
	var model agentPlanStepModel
	if err := r.db.WithContext(ctx).Where("id = ?", step.ID).First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentPlanStep{}, opErr
	}
	return agentPlanStepModelToDomain(model), nil
}

func (r *AgentRepository) CreateAgentCapabilityAuditLog(ctx context.Context, log domain.AgentCapabilityAuditLog) (domain.AgentCapabilityAuditLog, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_capability_audit.create", "insert", "agent_capability_audit_logs")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentCapabilityAuditLogModelFromDomain(normalizeAgentCapabilityAuditLog(log))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentCapabilityAuditLog{}, opErr
	}
	return agentCapabilityAuditLogModelToDomain(model), nil
}

func (r *AgentRepository) CreateAgentApproval(ctx context.Context, approval domain.AgentApproval) (domain.AgentApproval, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_approval.create", "insert", "agent_approvals")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentApprovalModelFromDomain(normalizeAgentApproval(approval))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentApproval{}, opErr
	}
	return agentApprovalModelToDomain(model), nil
}

func normalizeAgentPlan(plan domain.AgentPlan) domain.AgentPlan {
	plan.Goal = strings.TrimSpace(plan.Goal)
	plan.Summary = strings.TrimSpace(plan.Summary)
	plan.ImpactSummary = strings.TrimSpace(plan.ImpactSummary)
	plan.RiskLevel = strings.TrimSpace(plan.RiskLevel)
	plan.ConfirmationPolicy = strings.TrimSpace(plan.ConfirmationPolicy)
	plan.DedupeKey = strings.TrimSpace(plan.DedupeKey)
	plan.PolicyDecision = strings.TrimSpace(plan.PolicyDecision)
	plan.PolicyReason = strings.TrimSpace(plan.PolicyReason)
	plan.ErrorMessage = strings.TrimSpace(plan.ErrorMessage)
	if !plan.Status.Valid() {
		plan.Status = domain.AgentPlanStatusDraft
	}
	if plan.RiskLevel == "" {
		plan.RiskLevel = "low"
	}
	if plan.ConfirmationPolicy == "" {
		plan.ConfirmationPolicy = "auto"
	}
	if plan.AllowedScopes == nil {
		plan.AllowedScopes = []string{}
	}
	if plan.Metadata == nil {
		plan.Metadata = domain.AgentJSON{}
	}
	return plan
}

func normalizeAgentPlanStep(step domain.AgentPlanStep) domain.AgentPlanStep {
	step.CapabilityKey = strings.TrimSpace(step.CapabilityKey)
	step.Title = strings.TrimSpace(step.Title)
	step.InputSummary = strings.TrimSpace(step.InputSummary)
	step.OutputSummary = strings.TrimSpace(step.OutputSummary)
	step.ExpectedOutput = strings.TrimSpace(step.ExpectedOutput)
	step.FailureStrategy = strings.TrimSpace(step.FailureStrategy)
	step.ObservationRef = strings.TrimSpace(step.ObservationRef)
	step.ErrorMessage = strings.TrimSpace(step.ErrorMessage)
	if !step.Status.Valid() {
		step.Status = domain.AgentPlanStepStatusPending
	}
	if step.CapabilityScope == nil {
		step.CapabilityScope = []string{}
	}
	if step.ArtifactRefs == nil {
		step.ArtifactRefs = []string{}
	}
	if step.MaxRetries < 0 {
		step.MaxRetries = 0
	}
	if step.RetryCount < 0 {
		step.RetryCount = 0
	}
	step.RetryReason = strings.TrimSpace(step.RetryReason)
	if step.RetryMetadata == nil {
		step.RetryMetadata = domain.AgentJSON{}
	}
	return step
}

func normalizeAgentCapabilityAuditLog(log domain.AgentCapabilityAuditLog) domain.AgentCapabilityAuditLog {
	log.CapabilityKey = strings.TrimSpace(log.CapabilityKey)
	log.Decision = strings.TrimSpace(log.Decision)
	log.Reason = strings.TrimSpace(log.Reason)
	log.InputSummary = strings.TrimSpace(log.InputSummary)
	log.OutputSummary = strings.TrimSpace(log.OutputSummary)
	log.Status = strings.TrimSpace(log.Status)
	log.ErrorMessage = strings.TrimSpace(log.ErrorMessage)
	if log.SourceRefs == nil {
		log.SourceRefs = []string{}
	}
	if log.Metadata == nil {
		log.Metadata = domain.AgentJSON{}
	}
	return log
}

func normalizeAgentApproval(approval domain.AgentApproval) domain.AgentApproval {
	approval.ApprovalTokenHash = strings.TrimSpace(approval.ApprovalTokenHash)
	approval.Channel = strings.TrimSpace(approval.Channel)
	approval.RequestID = strings.TrimSpace(approval.RequestID)
	approval.TraceID = strings.TrimSpace(approval.TraceID)
	if approval.Channel == "" {
		approval.Channel = "web"
	}
	if !approval.Status.Valid() {
		approval.Status = domain.AgentApprovalStatusPending
	}
	if approval.Metadata == nil {
		approval.Metadata = domain.AgentJSON{}
	}
	return approval
}

func agentPlanModelFromDomain(plan domain.AgentPlan) agentPlanModel {
	return agentPlanModel{
		ID:                 plan.ID,
		UserID:             plan.UserID,
		SessionID:          int64Pointer(plan.SessionID),
		TurnID:             int64Pointer(plan.TurnID),
		ControllerRunID:    int64Pointer(plan.ControllerRunID),
		Status:             string(plan.Status),
		Goal:               plan.Goal,
		Summary:            plan.Summary,
		ImpactSummary:      plan.ImpactSummary,
		RiskLevel:          plan.RiskLevel,
		ConfirmationPolicy: plan.ConfirmationPolicy,
		AllowedScopes:      append([]string(nil), plan.AllowedScopes...),
		DedupeKey:          plan.DedupeKey,
		PolicyDecision:     plan.PolicyDecision,
		PolicyReason:       plan.PolicyReason,
		ExpiresAt:          plan.ExpiresAt,
		ApprovedAt:         plan.ApprovedAt,
		RejectedAt:         plan.RejectedAt,
		CompletedAt:        plan.CompletedAt,
		FailedAt:           plan.FailedAt,
		ErrorMessage:       plan.ErrorMessage,
		Metadata:           cloneAgentJSON(plan.Metadata),
		CreatedAt:          plan.CreatedAt,
		UpdatedAt:          plan.UpdatedAt,
	}
}

func agentPlanModelToDomain(model agentPlanModel) domain.AgentPlan {
	return domain.AgentPlan{
		ID:                 model.ID,
		UserID:             model.UserID,
		SessionID:          int64Value(model.SessionID),
		TurnID:             int64Value(model.TurnID),
		ControllerRunID:    int64Value(model.ControllerRunID),
		Status:             domain.AgentPlanStatus(model.Status),
		Goal:               model.Goal,
		Summary:            model.Summary,
		ImpactSummary:      model.ImpactSummary,
		RiskLevel:          model.RiskLevel,
		ConfirmationPolicy: model.ConfirmationPolicy,
		AllowedScopes:      append([]string(nil), model.AllowedScopes...),
		DedupeKey:          model.DedupeKey,
		PolicyDecision:     model.PolicyDecision,
		PolicyReason:       model.PolicyReason,
		ExpiresAt:          model.ExpiresAt,
		ApprovedAt:         model.ApprovedAt,
		RejectedAt:         model.RejectedAt,
		CompletedAt:        model.CompletedAt,
		FailedAt:           model.FailedAt,
		ErrorMessage:       model.ErrorMessage,
		Metadata:           cloneAgentJSON(model.Metadata),
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
	}
}

func agentPlanStepModelFromDomain(step domain.AgentPlanStep) agentPlanStepModel {
	return agentPlanStepModel{
		ID:              step.ID,
		PlanID:          step.PlanID,
		StepOrder:       step.StepOrder,
		Status:          string(step.Status),
		CapabilityKey:   step.CapabilityKey,
		CapabilityScope: append([]string(nil), step.CapabilityScope...),
		Title:           step.Title,
		InputSummary:    step.InputSummary,
		OutputSummary:   step.OutputSummary,
		ExpectedOutput:  step.ExpectedOutput,
		FailureStrategy: step.FailureStrategy,
		ExecutorRunID:   int64Pointer(step.ExecutorRunID),
		ObservationRef:  step.ObservationRef,
		ArtifactRefs:    append([]string(nil), step.ArtifactRefs...),
		ErrorMessage:    step.ErrorMessage,
		RetryCount:      step.RetryCount,
		MaxRetries:      step.MaxRetries,
		LastRetryAt:     step.LastRetryAt,
		RetryReason:     step.RetryReason,
		RetryMetadata:   cloneAgentJSON(step.RetryMetadata),
		StartedAt:       step.StartedAt,
		CompletedAt:     step.CompletedAt,
		CreatedAt:       step.CreatedAt,
		UpdatedAt:       step.UpdatedAt,
	}
}

func agentPlanStepModelToDomain(model agentPlanStepModel) domain.AgentPlanStep {
	return domain.AgentPlanStep{
		ID:              model.ID,
		PlanID:          model.PlanID,
		StepOrder:       model.StepOrder,
		Status:          domain.AgentPlanStepStatus(model.Status),
		CapabilityKey:   model.CapabilityKey,
		CapabilityScope: append([]string(nil), model.CapabilityScope...),
		Title:           model.Title,
		InputSummary:    model.InputSummary,
		OutputSummary:   model.OutputSummary,
		ExpectedOutput:  model.ExpectedOutput,
		FailureStrategy: model.FailureStrategy,
		ExecutorRunID:   int64Value(model.ExecutorRunID),
		ObservationRef:  model.ObservationRef,
		ArtifactRefs:    append([]string(nil), model.ArtifactRefs...),
		ErrorMessage:    model.ErrorMessage,
		RetryCount:      model.RetryCount,
		MaxRetries:      model.MaxRetries,
		LastRetryAt:     model.LastRetryAt,
		RetryReason:     model.RetryReason,
		RetryMetadata:   cloneAgentJSON(model.RetryMetadata),
		StartedAt:       model.StartedAt,
		CompletedAt:     model.CompletedAt,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}
}

func agentCapabilityAuditLogModelFromDomain(log domain.AgentCapabilityAuditLog) agentCapabilityAuditLogModel {
	return agentCapabilityAuditLogModel{
		ID:            log.ID,
		UserID:        log.UserID,
		SessionID:     int64Pointer(log.SessionID),
		TurnID:        int64Pointer(log.TurnID),
		RunID:         int64Pointer(log.RunID),
		PlanID:        int64Pointer(log.PlanID),
		PlanStepID:    int64Pointer(log.PlanStepID),
		CapabilityKey: log.CapabilityKey,
		Decision:      log.Decision,
		Reason:        log.Reason,
		InputSummary:  log.InputSummary,
		OutputSummary: log.OutputSummary,
		Status:        log.Status,
		ErrorMessage:  log.ErrorMessage,
		SourceRefs:    append([]string(nil), log.SourceRefs...),
		Metadata:      cloneAgentJSON(log.Metadata),
		CreatedAt:     log.CreatedAt,
	}
}

func agentCapabilityAuditLogModelToDomain(model agentCapabilityAuditLogModel) domain.AgentCapabilityAuditLog {
	return domain.AgentCapabilityAuditLog{
		ID:            model.ID,
		UserID:        model.UserID,
		SessionID:     int64Value(model.SessionID),
		TurnID:        int64Value(model.TurnID),
		RunID:         int64Value(model.RunID),
		PlanID:        int64Value(model.PlanID),
		PlanStepID:    int64Value(model.PlanStepID),
		CapabilityKey: model.CapabilityKey,
		Decision:      model.Decision,
		Reason:        model.Reason,
		InputSummary:  model.InputSummary,
		OutputSummary: model.OutputSummary,
		Status:        model.Status,
		ErrorMessage:  model.ErrorMessage,
		SourceRefs:    append([]string(nil), model.SourceRefs...),
		Metadata:      cloneAgentJSON(model.Metadata),
		CreatedAt:     model.CreatedAt,
	}
}

func agentApprovalModelFromDomain(approval domain.AgentApproval) agentApprovalModel {
	return agentApprovalModel{
		ID:                approval.ID,
		PlanID:            approval.PlanID,
		UserID:            approval.UserID,
		ExternalAccountID: approval.ExternalAccountID,
		ApprovalTokenHash: approval.ApprovalTokenHash,
		Channel:           approval.Channel,
		Status:            string(approval.Status),
		ExpiresAt:         approval.ExpiresAt,
		DecidedAt:         approval.DecidedAt,
		RequestID:         approval.RequestID,
		TraceID:           approval.TraceID,
		Metadata:          cloneAgentJSON(approval.Metadata),
		CreatedAt:         approval.CreatedAt,
		UpdatedAt:         approval.UpdatedAt,
	}
}
