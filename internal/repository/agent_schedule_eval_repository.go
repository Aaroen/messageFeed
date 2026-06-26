package repository

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultAgentScheduledTaskClaimLimit = 20
	maxAgentScheduledTaskClaimLimit     = 100
	defaultAgentScheduledTaskListLimit  = 50
	maxAgentScheduledTaskListLimit      = 100
	defaultAgentEvalCaseListLimit       = 100
	maxAgentEvalCaseListLimit           = 200
	defaultAgentEvalRunListLimit        = 20
	maxAgentEvalRunListLimit            = 100
)

type agentScheduledTaskModel struct {
	ID                   int64 `gorm:"primaryKey"`
	UserID               int64 `gorm:"not null"`
	SessionID            *int64
	TurnID               *int64
	PlanID               *int64
	SourceRunID          *int64
	Status               string
	TaskType             string
	Goal                 string
	TargetChannel        string
	TargetRef            string
	ExecutionWindowStart *time.Time
	ExecutionWindowEnd   *time.Time
	ScheduledAt          time.Time
	DeliverAt            *time.Time
	FreshnessPolicy      string
	AllowedCapabilities  []string         `gorm:"column:allowed_capabilities_json;serializer:json;type:jsonb;not null"`
	ModelPolicy          domain.AgentJSON `gorm:"column:model_policy_json;serializer:json;type:jsonb;not null"`
	FailurePolicy        domain.AgentJSON `gorm:"column:failure_policy_json;serializer:json;type:jsonb;not null"`
	Payload              domain.AgentJSON `gorm:"column:payload_json;serializer:json;type:jsonb;not null"`
	AttemptCount         int
	MaxAttempts          int
	LockedBy             string
	LockedAt             *time.Time
	LastError            string
	NextRunAt            *time.Time
	CompletedAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type agentEvalCaseModel struct {
	ID               int64 `gorm:"primaryKey"`
	CaseKey          string
	Name             string
	Category         string
	Description      string
	Input            domain.AgentJSON `gorm:"column:input_json;serializer:json;type:jsonb;not null"`
	ExpectedBehavior string
	SafetyTags       []string `gorm:"column:safety_tags_json;serializer:json;type:jsonb;not null"`
	Enabled          bool
	Metadata         domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type agentEvalRunModel struct {
	ID           int64 `gorm:"primaryKey"`
	UserID       *int64
	Trigger      string `gorm:"column:trigger_source"`
	Status       string
	ModelKey     string
	CaseCount    int
	PassedCount  int
	FailedCount  int
	Metrics      domain.AgentJSON `gorm:"column:metrics_json;serializer:json;type:jsonb;not null"`
	StartedAt    *time.Time
	CompletedAt  *time.Time
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type agentEvalResultModel struct {
	ID            int64 `gorm:"primaryKey"`
	RunID         int64 `gorm:"not null"`
	CaseID        int64 `gorm:"not null"`
	Status        string
	Score         float64
	Input         domain.AgentJSON `gorm:"column:input_json;serializer:json;type:jsonb;not null"`
	Expected      string
	Actual        string
	FailureReason string
	Metrics       domain.AgentJSON `gorm:"column:metrics_json;serializer:json;type:jsonb;not null"`
	EvidenceRefs  []string         `gorm:"column:evidence_refs_json;serializer:json;type:jsonb;not null"`
	CreatedAt     time.Time
}

func (agentScheduledTaskModel) TableName() string { return "agent_scheduled_tasks" }
func (agentEvalCaseModel) TableName() string      { return "agent_eval_cases" }
func (agentEvalRunModel) TableName() string       { return "agent_eval_runs" }
func (agentEvalResultModel) TableName() string    { return "agent_eval_results" }

func (r *AgentRepository) CreateAgentScheduledTask(ctx context.Context, task domain.AgentScheduledTask) (domain.AgentScheduledTask, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_scheduled_task.create", "insert", "agent_scheduled_tasks")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentScheduledTaskModelFromDomain(normalizeAgentScheduledTask(task))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentScheduledTask{}, opErr
	}
	return agentScheduledTaskModelToDomain(model), nil
}

func (r *AgentRepository) GetAgentScheduledTask(ctx context.Context, userID int64, taskID int64) (domain.AgentScheduledTask, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_scheduled_task.get", "select", "agent_scheduled_tasks")
	var opErr error
	defer func() { finish(opErr) }()

	var model agentScheduledTaskModel
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", taskID, userID).First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentScheduledTask{}, opErr
	}
	return agentScheduledTaskModelToDomain(model), nil
}

func (r *AgentRepository) ListAgentScheduledTasks(ctx context.Context, options domain.AgentScheduledTaskListOptions) ([]domain.AgentScheduledTask, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_scheduled_task.list", "select", "agent_scheduled_tasks")
	var opErr error
	defer func() { finish(opErr) }()

	options = normalizeAgentScheduledTaskListOptions(options)
	query := r.db.WithContext(ctx).Model(&agentScheduledTaskModel{}).Where("user_id = ?", options.UserID)
	if options.Status.Valid() {
		query = query.Where("status = ?", string(options.Status))
	}
	var models []agentScheduledTaskModel
	if err := query.Order("created_at DESC, id DESC").Limit(options.Limit).Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	tasks := make([]domain.AgentScheduledTask, 0, len(models))
	for _, model := range models {
		tasks = append(tasks, agentScheduledTaskModelToDomain(model))
	}
	return tasks, nil
}

func (r *AgentRepository) ListAgentScheduledTasksByRefs(ctx context.Context, userID int64, planID int64, turnID int64, runID int64, limit int) ([]domain.AgentScheduledTask, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_scheduled_task.list_by_refs", "select", "agent_scheduled_tasks")
	var opErr error
	defer func() { finish(opErr) }()

	if limit < 1 {
		limit = defaultAgentScheduledTaskListLimit
	}
	if limit > maxAgentScheduledTaskListLimit {
		limit = maxAgentScheduledTaskListLimit
	}
	query := r.db.WithContext(ctx).Model(&agentScheduledTaskModel{}).Where("user_id = ?", userID)
	if planID > 0 || turnID > 0 || runID > 0 {
		refQuery := r.db.Where("1 = 0")
		if planID > 0 {
			refQuery = refQuery.Or("plan_id = ?", planID)
		}
		if turnID > 0 {
			refQuery = refQuery.Or("turn_id = ?", turnID)
		}
		if runID > 0 {
			refQuery = refQuery.Or("source_run_id = ?", runID)
		}
		query = query.Where(refQuery)
	}
	var models []agentScheduledTaskModel
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	tasks := make([]domain.AgentScheduledTask, 0, len(models))
	for _, model := range models {
		tasks = append(tasks, agentScheduledTaskModelToDomain(model))
	}
	return tasks, nil
}

func (r *AgentRepository) ClaimDueAgentScheduledTasks(ctx context.Context, input domain.AgentScheduledTaskClaimInput) ([]domain.AgentScheduledTask, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_scheduled_task.claim_due", "update", "agent_scheduled_tasks")
	var opErr error
	defer func() { finish(opErr) }()

	input = normalizeAgentScheduledTaskClaimInput(input)
	var models []agentScheduledTaskModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []int64
		if err := tx.WithContext(ctx).
			Model(&agentScheduledTaskModel{}).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND scheduled_at <= ?", string(domain.AgentScheduledTaskStatusQueued), input.Now).
			Order("scheduled_at ASC, id ASC").
			Limit(input.Limit).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		updates := map[string]any{
			"status":        string(domain.AgentScheduledTaskStatusRunning),
			"locked_by":     input.WorkerID,
			"locked_at":     input.Now,
			"attempt_count": gorm.Expr("attempt_count + ?", 1),
			"updated_at":    input.Now,
		}
		if err := tx.WithContext(ctx).Model(&agentScheduledTaskModel{}).Where("id IN ?", ids).Updates(updates).Error; err != nil {
			return err
		}
		return tx.WithContext(ctx).Where("id IN ?", ids).Order("scheduled_at ASC, id ASC").Find(&models).Error
	})
	if err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	tasks := make([]domain.AgentScheduledTask, 0, len(models))
	for _, model := range models {
		tasks = append(tasks, agentScheduledTaskModelToDomain(model))
	}
	return tasks, nil
}

func (r *AgentRepository) UpdateAgentScheduledTask(ctx context.Context, task domain.AgentScheduledTask) (domain.AgentScheduledTask, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_scheduled_task.update", "update", "agent_scheduled_tasks")
	var opErr error
	defer func() { finish(opErr) }()

	task = normalizeAgentScheduledTask(task)
	model := agentScheduledTaskModelFromDomain(task)
	result := r.db.WithContext(ctx).
		Model(&agentScheduledTaskModel{}).
		Where("id = ? AND user_id = ?", task.ID, task.UserID).
		Select("SourceRunID", "Status", "Goal", "TargetChannel", "TargetRef", "ExecutionWindowStart", "ExecutionWindowEnd", "ScheduledAt", "DeliverAt", "FreshnessPolicy", "AllowedCapabilities", "ModelPolicy", "FailurePolicy", "Payload", "AttemptCount", "MaxAttempts", "LockedBy", "LockedAt", "LastError", "NextRunAt", "CompletedAt").
		Updates(&model)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentScheduledTask{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentScheduledTask{}, opErr
	}
	return r.GetAgentScheduledTask(ctx, task.UserID, task.ID)
}

func (r *AgentRepository) CreateAgentEvalCase(ctx context.Context, evalCase domain.AgentEvalCase) (domain.AgentEvalCase, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_eval_case.create", "insert", "agent_eval_cases")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentEvalCaseModelFromDomain(normalizeAgentEvalCase(evalCase))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentEvalCase{}, opErr
	}
	return agentEvalCaseModelToDomain(model), nil
}

func (r *AgentRepository) UpsertAgentEvalCase(ctx context.Context, evalCase domain.AgentEvalCase) (domain.AgentEvalCase, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_eval_case.upsert", "upsert", "agent_eval_cases")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentEvalCaseModelFromDomain(normalizeAgentEvalCase(evalCase))
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "case_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"category",
			"description",
			"input_json",
			"expected_behavior",
			"safety_tags_json",
			"enabled",
			"metadata_json",
			"updated_at",
		}),
	}).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentEvalCase{}, opErr
	}
	var stored agentEvalCaseModel
	if err := r.db.WithContext(ctx).Where("case_key = ?", model.CaseKey).First(&stored).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentEvalCase{}, opErr
	}
	return agentEvalCaseModelToDomain(stored), nil
}

func (r *AgentRepository) ListAgentEvalCases(ctx context.Context, options domain.AgentEvalCaseListOptions) ([]domain.AgentEvalCase, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_eval_case.list", "select", "agent_eval_cases")
	var opErr error
	defer func() { finish(opErr) }()

	options = normalizeAgentEvalCaseListOptions(options)
	query := r.db.WithContext(ctx).Model(&agentEvalCaseModel{})
	if options.EnabledOnly {
		query = query.Where("enabled = ?", true)
	}
	if options.Category != "" {
		query = query.Where("category = ?", options.Category)
	}
	var models []agentEvalCaseModel
	if err := query.Order("category ASC, id ASC").Limit(options.Limit).Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	cases := make([]domain.AgentEvalCase, 0, len(models))
	for _, model := range models {
		cases = append(cases, agentEvalCaseModelToDomain(model))
	}
	return cases, nil
}

func (r *AgentRepository) CreateAgentEvalRun(ctx context.Context, run domain.AgentEvalRun) (domain.AgentEvalRun, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_eval_run.create", "insert", "agent_eval_runs")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentEvalRunModelFromDomain(normalizeAgentEvalRun(run))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentEvalRun{}, opErr
	}
	return agentEvalRunModelToDomain(model), nil
}

func (r *AgentRepository) UpdateAgentEvalRun(ctx context.Context, run domain.AgentEvalRun) (domain.AgentEvalRun, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_eval_run.update", "update", "agent_eval_runs")
	var opErr error
	defer func() { finish(opErr) }()

	run = normalizeAgentEvalRun(run)
	model := agentEvalRunModelFromDomain(run)
	result := r.db.WithContext(ctx).Model(&agentEvalRunModel{}).Where("id = ?", run.ID).Select(
		"Status",
		"ModelKey",
		"CaseCount",
		"PassedCount",
		"FailedCount",
		"Metrics",
		"StartedAt",
		"CompletedAt",
		"ErrorMessage",
		"UpdatedAt",
	).Updates(&model)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentEvalRun{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentEvalRun{}, opErr
	}
	return r.GetAgentEvalRunDetail(ctx, run.ID)
}

func (r *AgentRepository) ListAgentEvalRuns(ctx context.Context, options domain.AgentEvalRunListOptions) ([]domain.AgentEvalRun, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_eval_run.list", "select", "agent_eval_runs")
	var opErr error
	defer func() { finish(opErr) }()

	options = normalizeAgentEvalRunListOptions(options)
	query := r.db.WithContext(ctx).Model(&agentEvalRunModel{})
	if options.UserID > 0 {
		query = query.Where("user_id = ?", options.UserID)
	}
	var models []agentEvalRunModel
	if err := query.Order("created_at DESC, id DESC").Limit(options.Limit).Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	runs := make([]domain.AgentEvalRun, 0, len(models))
	for _, model := range models {
		runs = append(runs, agentEvalRunModelToDomain(model))
	}
	return runs, nil
}

func (r *AgentRepository) CreateAgentEvalResult(ctx context.Context, result domain.AgentEvalResult) (domain.AgentEvalResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_eval_result.create", "insert", "agent_eval_results")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentEvalResultModelFromDomain(normalizeAgentEvalResult(result))
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model).Error; err != nil {
			return err
		}
		return recalculateAgentEvalRunStats(ctx, tx, model.RunID)
	})
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentEvalResult{}, opErr
	}
	return agentEvalResultModelToDomain(model), nil
}

func (r *AgentRepository) GetAgentEvalRunDetail(ctx context.Context, runID int64) (domain.AgentEvalRun, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_eval_run.get_detail", "select", "agent_eval_runs")
	var opErr error
	defer func() { finish(opErr) }()

	var model agentEvalRunModel
	if err := r.db.WithContext(ctx).Where("id = ?", runID).First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentEvalRun{}, opErr
	}
	run := agentEvalRunModelToDomain(model)
	var resultModels []agentEvalResultModel
	if err := r.db.WithContext(ctx).Where("run_id = ?", runID).Order("id ASC").Find(&resultModels).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentEvalRun{}, opErr
	}
	for _, resultModel := range resultModels {
		run.Results = append(run.Results, agentEvalResultModelToDomain(resultModel))
	}
	return run, nil
}

func recalculateAgentEvalRunStats(ctx context.Context, tx *gorm.DB, runID int64) error {
	var rows []agentEvalResultModel
	if err := tx.WithContext(ctx).Where("run_id = ?", runID).Find(&rows).Error; err != nil {
		return err
	}
	caseCount := len(rows)
	passedCount := 0
	failedCount := 0
	for _, row := range rows {
		switch domain.AgentEvalResultStatus(row.Status) {
		case domain.AgentEvalResultStatusPassed:
			passedCount++
		case domain.AgentEvalResultStatusFailed, domain.AgentEvalResultStatusError:
			failedCount++
		}
	}
	status := string(domain.AgentEvalRunStatusRunning)
	if caseCount > 0 {
		status = string(domain.AgentEvalRunStatusCompleted)
		if failedCount > 0 {
			status = string(domain.AgentEvalRunStatusFailed)
		}
	}
	return tx.WithContext(ctx).Model(&agentEvalRunModel{}).Where("id = ?", runID).Updates(map[string]any{
		"status":       status,
		"case_count":   caseCount,
		"passed_count": passedCount,
		"failed_count": failedCount,
		"updated_at":   time.Now().UTC(),
	}).Error
}

func normalizeAgentScheduledTask(task domain.AgentScheduledTask) domain.AgentScheduledTask {
	task.TaskType = strings.TrimSpace(task.TaskType)
	task.Goal = strings.TrimSpace(task.Goal)
	task.TargetChannel = strings.TrimSpace(task.TargetChannel)
	task.TargetRef = strings.TrimSpace(task.TargetRef)
	task.FreshnessPolicy = strings.TrimSpace(task.FreshnessPolicy)
	task.LockedBy = strings.TrimSpace(task.LockedBy)
	task.LastError = strings.TrimSpace(task.LastError)
	if !task.Status.Valid() {
		task.Status = domain.AgentScheduledTaskStatusQueued
	}
	if task.TaskType == "" {
		task.TaskType = "agent_task"
	}
	if task.FreshnessPolicy == "" {
		task.FreshnessPolicy = "latest_at_run"
	}
	if task.MaxAttempts < 1 {
		task.MaxAttempts = 3
	}
	if task.ScheduledAt.IsZero() {
		task.ScheduledAt = time.Now().UTC()
	}
	if task.AllowedCapabilities == nil {
		task.AllowedCapabilities = []string{}
	}
	if task.ModelPolicy == nil {
		task.ModelPolicy = domain.AgentJSON{}
	}
	if task.FailurePolicy == nil {
		task.FailurePolicy = domain.AgentJSON{}
	}
	if task.Payload == nil {
		task.Payload = domain.AgentJSON{}
	}
	return task
}

func normalizeAgentScheduledTaskClaimInput(input domain.AgentScheduledTaskClaimInput) domain.AgentScheduledTaskClaimInput {
	input.WorkerID = strings.TrimSpace(input.WorkerID)
	if input.WorkerID == "" {
		input.WorkerID = "agent-scheduler"
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	} else {
		input.Now = input.Now.UTC()
	}
	if input.Limit < 1 {
		input.Limit = defaultAgentScheduledTaskClaimLimit
	}
	if input.Limit > maxAgentScheduledTaskClaimLimit {
		input.Limit = maxAgentScheduledTaskClaimLimit
	}
	return input
}

func normalizeAgentScheduledTaskListOptions(options domain.AgentScheduledTaskListOptions) domain.AgentScheduledTaskListOptions {
	if options.Limit < 1 {
		options.Limit = defaultAgentScheduledTaskListLimit
	}
	if options.Limit > maxAgentScheduledTaskListLimit {
		options.Limit = maxAgentScheduledTaskListLimit
	}
	return options
}

func normalizeAgentEvalCaseListOptions(options domain.AgentEvalCaseListOptions) domain.AgentEvalCaseListOptions {
	options.Category = strings.TrimSpace(options.Category)
	if options.Limit < 1 {
		options.Limit = defaultAgentEvalCaseListLimit
	}
	if options.Limit > maxAgentEvalCaseListLimit {
		options.Limit = maxAgentEvalCaseListLimit
	}
	return options
}

func normalizeAgentEvalRunListOptions(options domain.AgentEvalRunListOptions) domain.AgentEvalRunListOptions {
	if options.Limit < 1 {
		options.Limit = defaultAgentEvalRunListLimit
	}
	if options.Limit > maxAgentEvalRunListLimit {
		options.Limit = maxAgentEvalRunListLimit
	}
	return options
}

func normalizeAgentEvalCase(evalCase domain.AgentEvalCase) domain.AgentEvalCase {
	evalCase.CaseKey = strings.TrimSpace(evalCase.CaseKey)
	evalCase.Name = strings.TrimSpace(evalCase.Name)
	evalCase.Category = strings.TrimSpace(evalCase.Category)
	evalCase.Description = strings.TrimSpace(evalCase.Description)
	evalCase.ExpectedBehavior = strings.TrimSpace(evalCase.ExpectedBehavior)
	if evalCase.Input == nil {
		evalCase.Input = domain.AgentJSON{}
	}
	if evalCase.SafetyTags == nil {
		evalCase.SafetyTags = []string{}
	}
	if evalCase.Metadata == nil {
		evalCase.Metadata = domain.AgentJSON{}
	}
	return evalCase
}

func normalizeAgentEvalRun(run domain.AgentEvalRun) domain.AgentEvalRun {
	run.Trigger = strings.TrimSpace(run.Trigger)
	run.ModelKey = strings.TrimSpace(run.ModelKey)
	run.ErrorMessage = strings.TrimSpace(run.ErrorMessage)
	if !run.Status.Valid() {
		run.Status = domain.AgentEvalRunStatusQueued
	}
	if run.Metrics == nil {
		run.Metrics = domain.AgentJSON{}
	}
	return run
}

func normalizeAgentEvalResult(result domain.AgentEvalResult) domain.AgentEvalResult {
	result.Expected = strings.TrimSpace(result.Expected)
	result.Actual = strings.TrimSpace(result.Actual)
	result.FailureReason = strings.TrimSpace(result.FailureReason)
	if !result.Status.Valid() {
		result.Status = domain.AgentEvalResultStatusSkipped
	}
	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 1 {
		result.Score = 1
	}
	if result.Input == nil {
		result.Input = domain.AgentJSON{}
	}
	if result.Metrics == nil {
		result.Metrics = domain.AgentJSON{}
	}
	if result.EvidenceRefs == nil {
		result.EvidenceRefs = []string{}
	}
	return result
}

func agentScheduledTaskModelFromDomain(task domain.AgentScheduledTask) agentScheduledTaskModel {
	return agentScheduledTaskModel{
		ID:                   task.ID,
		UserID:               task.UserID,
		SessionID:            int64Pointer(task.SessionID),
		TurnID:               int64Pointer(task.TurnID),
		PlanID:               int64Pointer(task.PlanID),
		SourceRunID:          int64Pointer(task.SourceRunID),
		Status:               string(task.Status),
		TaskType:             task.TaskType,
		Goal:                 task.Goal,
		TargetChannel:        task.TargetChannel,
		TargetRef:            task.TargetRef,
		ExecutionWindowStart: task.ExecutionWindowStart,
		ExecutionWindowEnd:   task.ExecutionWindowEnd,
		ScheduledAt:          task.ScheduledAt,
		DeliverAt:            task.DeliverAt,
		FreshnessPolicy:      task.FreshnessPolicy,
		AllowedCapabilities:  cloneStringSlice(task.AllowedCapabilities),
		ModelPolicy:          cloneAgentJSON(task.ModelPolicy),
		FailurePolicy:        cloneAgentJSON(task.FailurePolicy),
		Payload:              cloneAgentJSON(task.Payload),
		AttemptCount:         task.AttemptCount,
		MaxAttempts:          task.MaxAttempts,
		LockedBy:             task.LockedBy,
		LockedAt:             task.LockedAt,
		LastError:            task.LastError,
		NextRunAt:            task.NextRunAt,
		CompletedAt:          task.CompletedAt,
		CreatedAt:            task.CreatedAt,
		UpdatedAt:            task.UpdatedAt,
	}
}

func agentScheduledTaskModelToDomain(model agentScheduledTaskModel) domain.AgentScheduledTask {
	return domain.AgentScheduledTask{
		ID:                   model.ID,
		UserID:               model.UserID,
		SessionID:            int64Value(model.SessionID),
		TurnID:               int64Value(model.TurnID),
		PlanID:               int64Value(model.PlanID),
		SourceRunID:          int64Value(model.SourceRunID),
		Status:               domain.AgentScheduledTaskStatus(model.Status),
		TaskType:             model.TaskType,
		Goal:                 model.Goal,
		TargetChannel:        model.TargetChannel,
		TargetRef:            model.TargetRef,
		ExecutionWindowStart: model.ExecutionWindowStart,
		ExecutionWindowEnd:   model.ExecutionWindowEnd,
		ScheduledAt:          model.ScheduledAt,
		DeliverAt:            model.DeliverAt,
		FreshnessPolicy:      model.FreshnessPolicy,
		AllowedCapabilities:  cloneStringSlice(model.AllowedCapabilities),
		ModelPolicy:          cloneAgentJSON(model.ModelPolicy),
		FailurePolicy:        cloneAgentJSON(model.FailurePolicy),
		Payload:              cloneAgentJSON(model.Payload),
		AttemptCount:         model.AttemptCount,
		MaxAttempts:          model.MaxAttempts,
		LockedBy:             model.LockedBy,
		LockedAt:             model.LockedAt,
		LastError:            model.LastError,
		NextRunAt:            model.NextRunAt,
		CompletedAt:          model.CompletedAt,
		CreatedAt:            model.CreatedAt,
		UpdatedAt:            model.UpdatedAt,
	}
}

func agentEvalCaseModelFromDomain(evalCase domain.AgentEvalCase) agentEvalCaseModel {
	return agentEvalCaseModel{
		ID:               evalCase.ID,
		CaseKey:          evalCase.CaseKey,
		Name:             evalCase.Name,
		Category:         evalCase.Category,
		Description:      evalCase.Description,
		Input:            cloneAgentJSON(evalCase.Input),
		ExpectedBehavior: evalCase.ExpectedBehavior,
		SafetyTags:       cloneStringSlice(evalCase.SafetyTags),
		Enabled:          evalCase.Enabled,
		Metadata:         cloneAgentJSON(evalCase.Metadata),
		CreatedAt:        evalCase.CreatedAt,
		UpdatedAt:        evalCase.UpdatedAt,
	}
}

func agentEvalCaseModelToDomain(model agentEvalCaseModel) domain.AgentEvalCase {
	return domain.AgentEvalCase{
		ID:               model.ID,
		CaseKey:          model.CaseKey,
		Name:             model.Name,
		Category:         model.Category,
		Description:      model.Description,
		Input:            cloneAgentJSON(model.Input),
		ExpectedBehavior: model.ExpectedBehavior,
		SafetyTags:       cloneStringSlice(model.SafetyTags),
		Enabled:          model.Enabled,
		Metadata:         cloneAgentJSON(model.Metadata),
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}

func agentEvalRunModelFromDomain(run domain.AgentEvalRun) agentEvalRunModel {
	return agentEvalRunModel{
		ID:           run.ID,
		UserID:       int64Pointer(run.UserID),
		Trigger:      run.Trigger,
		Status:       string(run.Status),
		ModelKey:     run.ModelKey,
		CaseCount:    run.CaseCount,
		PassedCount:  run.PassedCount,
		FailedCount:  run.FailedCount,
		Metrics:      cloneAgentJSON(run.Metrics),
		StartedAt:    run.StartedAt,
		CompletedAt:  run.CompletedAt,
		ErrorMessage: run.ErrorMessage,
		CreatedAt:    run.CreatedAt,
		UpdatedAt:    run.UpdatedAt,
	}
}

func agentEvalRunModelToDomain(model agentEvalRunModel) domain.AgentEvalRun {
	return domain.AgentEvalRun{
		ID:           model.ID,
		UserID:       int64Value(model.UserID),
		Trigger:      model.Trigger,
		Status:       domain.AgentEvalRunStatus(model.Status),
		ModelKey:     model.ModelKey,
		CaseCount:    model.CaseCount,
		PassedCount:  model.PassedCount,
		FailedCount:  model.FailedCount,
		Metrics:      cloneAgentJSON(model.Metrics),
		StartedAt:    model.StartedAt,
		CompletedAt:  model.CompletedAt,
		ErrorMessage: model.ErrorMessage,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}
}

func agentEvalResultModelFromDomain(result domain.AgentEvalResult) agentEvalResultModel {
	return agentEvalResultModel{
		ID:            result.ID,
		RunID:         result.RunID,
		CaseID:        result.CaseID,
		Status:        string(result.Status),
		Score:         result.Score,
		Input:         cloneAgentJSON(result.Input),
		Expected:      result.Expected,
		Actual:        result.Actual,
		FailureReason: result.FailureReason,
		Metrics:       cloneAgentJSON(result.Metrics),
		EvidenceRefs:  cloneStringSlice(result.EvidenceRefs),
		CreatedAt:     result.CreatedAt,
	}
}

func agentEvalResultModelToDomain(model agentEvalResultModel) domain.AgentEvalResult {
	return domain.AgentEvalResult{
		ID:            model.ID,
		RunID:         model.RunID,
		CaseID:        model.CaseID,
		Status:        domain.AgentEvalResultStatus(model.Status),
		Score:         model.Score,
		Input:         cloneAgentJSON(model.Input),
		Expected:      model.Expected,
		Actual:        model.Actual,
		FailureReason: model.FailureReason,
		Metrics:       cloneAgentJSON(model.Metrics),
		EvidenceRefs:  cloneStringSlice(model.EvidenceRefs),
		CreatedAt:     model.CreatedAt,
	}
}
