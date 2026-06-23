package repository

import (
	"context"
	"messagefeed/internal/domain"
	"time"

	"gorm.io/gorm"
)

type AlertRuleRepository struct {
	db *gorm.DB
}

func NewAlertRuleRepository(db *gorm.DB) *AlertRuleRepository {
	return &AlertRuleRepository{db: db}
}

type alertRuleModel struct {
	ID              int64 `gorm:"primaryKey"`
	UserID          int64 `gorm:"not null"`
	Name            string
	Scope           string
	Condition       domain.AlertRuleCondition `gorm:"column:condition_json;serializer:json;type:jsonb;not null"`
	MinImportance   float64
	AIRequired      bool
	CooldownSeconds int
	Channel         string
	Enabled         bool
	LastTriggeredAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (alertRuleModel) TableName() string {
	return "alert_rules"
}

func (r *AlertRuleRepository) Create(ctx context.Context, rule domain.AlertRule) (domain.AlertRule, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.alert_rule.create", "create", "alert_rules")
	var opErr error
	defer func() { finish(opErr) }()

	model := alertRuleModelFromDomain(rule)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AlertRule{}, opErr
	}
	return alertRuleModelToDomain(model), nil
}

func (r *AlertRuleRepository) ListEnabledByUser(ctx context.Context, userID int64) ([]domain.AlertRule, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.alert_rule.list_enabled_by_user", "select", "alert_rules")
	var opErr error
	defer func() { finish(opErr) }()

	var models []alertRuleModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND enabled = true", userID).
		Order("id ASC").
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}

	rules := make([]domain.AlertRule, 0, len(models))
	for _, model := range models {
		rules = append(rules, alertRuleModelToDomain(model))
	}
	return rules, nil
}

func alertRuleModelFromDomain(rule domain.AlertRule) alertRuleModel {
	return alertRuleModel{
		ID:              rule.ID,
		UserID:          rule.UserID,
		Name:            rule.Name,
		Scope:           string(rule.Scope),
		Condition:       cloneAlertRuleCondition(rule.Condition),
		MinImportance:   rule.MinImportance,
		AIRequired:      rule.AIRequired,
		CooldownSeconds: rule.CooldownSeconds,
		Channel:         rule.Channel,
		Enabled:         rule.Enabled,
		LastTriggeredAt: rule.LastTriggeredAt,
		CreatedAt:       rule.CreatedAt,
		UpdatedAt:       rule.UpdatedAt,
	}
}

func alertRuleModelToDomain(model alertRuleModel) domain.AlertRule {
	return domain.AlertRule{
		ID:              model.ID,
		UserID:          model.UserID,
		Name:            model.Name,
		Scope:           domain.AlertRuleScope(model.Scope),
		Condition:       cloneAlertRuleCondition(model.Condition),
		MinImportance:   model.MinImportance,
		AIRequired:      model.AIRequired,
		CooldownSeconds: model.CooldownSeconds,
		Channel:         model.Channel,
		Enabled:         model.Enabled,
		LastTriggeredAt: model.LastTriggeredAt,
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}
}

func cloneAlertRuleCondition(condition domain.AlertRuleCondition) domain.AlertRuleCondition {
	return domain.AlertRuleCondition{
		SourceIDs:  append([]int64(nil), condition.SourceIDs...),
		Categories: append([]string(nil), condition.Categories...),
		Tags:       append([]string(nil), condition.Tags...),
		Keywords:   append([]string(nil), condition.Keywords...),
		Tickers:    append([]string(nil), condition.Tickers...),
	}
}
