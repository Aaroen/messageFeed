package repository

import (
	"context"
	"messagefeed/internal/domain"
	"time"

	"gorm.io/gorm/clause"
)

const (
	defaultAgentAuditLogListLimit = 50
	maxAgentAuditLogListLimit     = 100
)

type agentNotificationPreferenceModel struct {
	UserID                       int64 `gorm:"primaryKey"`
	ProcessNotificationsEnabled  bool
	FinalReportsEnabled          bool
	FailureNotificationsEnabled  bool
	RecoveryNotificationsEnabled bool
	MaxConcurrentTasks           int
	MaxQueuedTasks               int
	AutoRecoveryEnabled          bool
	QualityHandoffThreshold      float64
	HandoffOnFailure             bool
	HandoffOnPermission          bool
	HandoffOnBudget              bool
	CapabilityPolicy             domain.AgentJSON `gorm:"column:capability_policy_json;serializer:json;type:jsonb;not null"`
	DailyTaskQuota               int
	DailyExternalCallQuota       int
	DailyCapabilityCallQuota     int
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

func (agentNotificationPreferenceModel) TableName() string {
	return "agent_notification_preferences"
}

func (r *AgentRepository) GetAgentNotificationPreference(ctx context.Context, userID int64) (domain.AgentNotificationPreference, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_notification_preference.get", "select", "agent_notification_preferences")
	var opErr error
	defer func() { finish(opErr) }()

	var model agentNotificationPreferenceModel
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentNotificationPreference{}, opErr
	}
	return agentNotificationPreferenceModelToDomain(model), nil
}

func (r *AgentRepository) UpsertAgentNotificationPreference(ctx context.Context, preference domain.AgentNotificationPreference) (domain.AgentNotificationPreference, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_notification_preference.upsert", "upsert", "agent_notification_preferences")
	var opErr error
	defer func() { finish(opErr) }()

	preference = normalizeAgentNotificationPreference(preference)
	model := agentNotificationPreferenceModelFromDomain(preference)
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"process_notifications_enabled":  model.ProcessNotificationsEnabled,
				"final_reports_enabled":          model.FinalReportsEnabled,
				"failure_notifications_enabled":  model.FailureNotificationsEnabled,
				"recovery_notifications_enabled": model.RecoveryNotificationsEnabled,
				"max_concurrent_tasks":           model.MaxConcurrentTasks,
				"max_queued_tasks":               model.MaxQueuedTasks,
				"auto_recovery_enabled":          model.AutoRecoveryEnabled,
				"quality_handoff_threshold":      model.QualityHandoffThreshold,
				"handoff_on_failure":             model.HandoffOnFailure,
				"handoff_on_permission":          model.HandoffOnPermission,
				"handoff_on_budget":              model.HandoffOnBudget,
				"capability_policy_json":         clause.Expr{SQL: "?", Vars: []any{model.CapabilityPolicy}},
				"daily_task_quota":               model.DailyTaskQuota,
				"daily_external_call_quota":      model.DailyExternalCallQuota,
				"daily_capability_call_quota":    model.DailyCapabilityCallQuota,
				"updated_at":                     model.UpdatedAt,
			}),
		}).
		Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentNotificationPreference{}, opErr
	}
	return r.GetAgentNotificationPreference(ctx, preference.UserID)
}

func (r *AgentRepository) ListAuditLogsByRefs(ctx context.Context, options domain.AgentAuditLogListOptions) ([]domain.AgentAuditLog, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.audit.list_by_refs", "select", "agent_audit_logs")
	var opErr error
	defer func() { finish(opErr) }()

	options = normalizeAgentAuditLogListOptions(options)
	query := r.db.WithContext(ctx).Model(&agentAuditLogModel{}).Where("user_id = ?", options.UserID)
	if options.SessionID > 0 {
		query = query.Where("session_id = ?", options.SessionID)
	}
	if options.TurnID > 0 {
		query = query.Where("turn_id = ?", options.TurnID)
	}
	var models []agentAuditLogModel
	if err := query.Order("created_at DESC, id DESC").Limit(options.Limit).Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	logs := make([]domain.AgentAuditLog, 0, len(models))
	for _, model := range models {
		logs = append(logs, agentAuditLogModelToDomain(model))
	}
	return logs, nil
}

func normalizeAgentNotificationPreference(preference domain.AgentNotificationPreference) domain.AgentNotificationPreference {
	if preference.MaxConcurrentTasks < 1 {
		preference.MaxConcurrentTasks = 2
	}
	if preference.MaxConcurrentTasks > 20 {
		preference.MaxConcurrentTasks = 20
	}
	if preference.MaxQueuedTasks < 1 {
		preference.MaxQueuedTasks = 20
	}
	if preference.MaxQueuedTasks > 200 {
		preference.MaxQueuedTasks = 200
	}
	if preference.QualityHandoffThreshold <= 0 {
		preference.QualityHandoffThreshold = 0.65
	}
	if preference.QualityHandoffThreshold > 1 {
		preference.QualityHandoffThreshold = 1
	}
	if preference.CapabilityPolicy == nil {
		preference.CapabilityPolicy = domain.AgentJSON{}
	}
	if preference.DailyTaskQuota < 1 {
		preference.DailyTaskQuota = 50
	}
	if preference.DailyTaskQuota > 10000 {
		preference.DailyTaskQuota = 10000
	}
	if preference.DailyExternalCallQuota < 1 {
		preference.DailyExternalCallQuota = 200
	}
	if preference.DailyExternalCallQuota > 100000 {
		preference.DailyExternalCallQuota = 100000
	}
	if preference.DailyCapabilityCallQuota < 1 {
		preference.DailyCapabilityCallQuota = 500
	}
	if preference.DailyCapabilityCallQuota > 100000 {
		preference.DailyCapabilityCallQuota = 100000
	}
	if preference.CreatedAt.IsZero() {
		preference.CreatedAt = time.Now().UTC()
	}
	if preference.UpdatedAt.IsZero() {
		preference.UpdatedAt = preference.CreatedAt
	}
	return preference
}

func normalizeAgentAuditLogListOptions(options domain.AgentAuditLogListOptions) domain.AgentAuditLogListOptions {
	if options.Limit < 1 {
		options.Limit = defaultAgentAuditLogListLimit
	}
	if options.Limit > maxAgentAuditLogListLimit {
		options.Limit = maxAgentAuditLogListLimit
	}
	return options
}

func agentNotificationPreferenceModelFromDomain(preference domain.AgentNotificationPreference) agentNotificationPreferenceModel {
	return agentNotificationPreferenceModel{
		UserID:                       preference.UserID,
		ProcessNotificationsEnabled:  preference.ProcessNotificationsEnabled,
		FinalReportsEnabled:          preference.FinalReportsEnabled,
		FailureNotificationsEnabled:  preference.FailureNotificationsEnabled,
		RecoveryNotificationsEnabled: preference.RecoveryNotificationsEnabled,
		MaxConcurrentTasks:           preference.MaxConcurrentTasks,
		MaxQueuedTasks:               preference.MaxQueuedTasks,
		AutoRecoveryEnabled:          preference.AutoRecoveryEnabled,
		QualityHandoffThreshold:      preference.QualityHandoffThreshold,
		HandoffOnFailure:             preference.HandoffOnFailure,
		HandoffOnPermission:          preference.HandoffOnPermission,
		HandoffOnBudget:              preference.HandoffOnBudget,
		CapabilityPolicy:             cloneRepositoryAgentJSON(preference.CapabilityPolicy),
		DailyTaskQuota:               preference.DailyTaskQuota,
		DailyExternalCallQuota:       preference.DailyExternalCallQuota,
		DailyCapabilityCallQuota:     preference.DailyCapabilityCallQuota,
		CreatedAt:                    preference.CreatedAt,
		UpdatedAt:                    preference.UpdatedAt,
	}
}

func agentNotificationPreferenceModelToDomain(model agentNotificationPreferenceModel) domain.AgentNotificationPreference {
	return domain.AgentNotificationPreference{
		UserID:                       model.UserID,
		ProcessNotificationsEnabled:  model.ProcessNotificationsEnabled,
		FinalReportsEnabled:          model.FinalReportsEnabled,
		FailureNotificationsEnabled:  model.FailureNotificationsEnabled,
		RecoveryNotificationsEnabled: model.RecoveryNotificationsEnabled,
		MaxConcurrentTasks:           model.MaxConcurrentTasks,
		MaxQueuedTasks:               model.MaxQueuedTasks,
		AutoRecoveryEnabled:          model.AutoRecoveryEnabled,
		QualityHandoffThreshold:      model.QualityHandoffThreshold,
		HandoffOnFailure:             model.HandoffOnFailure,
		HandoffOnPermission:          model.HandoffOnPermission,
		HandoffOnBudget:              model.HandoffOnBudget,
		CapabilityPolicy:             cloneRepositoryAgentJSON(model.CapabilityPolicy),
		DailyTaskQuota:               model.DailyTaskQuota,
		DailyExternalCallQuota:       model.DailyExternalCallQuota,
		DailyCapabilityCallQuota:     model.DailyCapabilityCallQuota,
		CreatedAt:                    model.CreatedAt,
		UpdatedAt:                    model.UpdatedAt,
	}
}

func cloneRepositoryAgentJSON(input domain.AgentJSON) domain.AgentJSON {
	if input == nil {
		return domain.AgentJSON{}
	}
	output := make(domain.AgentJSON, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
