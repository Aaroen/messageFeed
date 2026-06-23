package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/observability"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

type AlertRuleStore interface {
	ListEnabledByUser(ctx context.Context, userID int64) ([]domain.AlertRule, error)
}

type AlertCandidateStore interface {
	Create(ctx context.Context, candidate domain.AlertCandidate) (domain.AlertCandidate, error)
}

type AlertRuleService struct {
	ruleStore      AlertRuleStore
	candidateStore AlertCandidateStore
	now            func() time.Time
}

type AlertRuleServiceOption func(*AlertRuleService)

func WithAlertRuleNow(now func() time.Time) AlertRuleServiceOption {
	return func(service *AlertRuleService) {
		if now != nil {
			service.now = now
		}
	}
}

func NewAlertRuleService(ruleStore AlertRuleStore, candidateStore AlertCandidateStore, options ...AlertRuleServiceOption) *AlertRuleService {
	service := &AlertRuleService{
		ruleStore:      ruleStore,
		candidateStore: candidateStore,
		now:            time.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type ProcessItemEventInput struct {
	Event domain.ItemEvent
}

type ProcessItemEventResult struct {
	RuleCount      int
	CandidateCount int
	Candidates     []domain.AlertCandidate
}

func (s *AlertRuleService) ProcessItemEvent(ctx context.Context, input ProcessItemEventInput) (ProcessItemEventResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.alert_rule.process_item_event",
		attribute.Int64("user.id", input.Event.UserID),
		attribute.Int64("item_event.id", input.Event.ID),
		attribute.String("item_event.type", string(input.Event.EventType)),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.ruleStore == nil || s.candidateStore == nil {
		opErr = fmt.Errorf("alert rule service is not configured")
		return ProcessItemEventResult{}, opErr
	}
	if input.Event.EventType != domain.ItemEventTypeItemCreated {
		return ProcessItemEventResult{}, nil
	}
	if input.Event.UserID < 1 {
		opErr = fmt.Errorf("%w: user id must be positive", domain.ErrInvalidInput)
		return ProcessItemEventResult{}, opErr
	}

	rules, err := s.ruleStore.ListEnabledByUser(ctx, input.Event.UserID)
	if err != nil {
		opErr = err
		return ProcessItemEventResult{}, opErr
	}

	result := ProcessItemEventResult{
		RuleCount:  len(rules),
		Candidates: make([]domain.AlertCandidate, 0),
	}
	for _, rule := range rules {
		matched, reasons := alertRuleMatchesItemEvent(rule, input.Event, s.now().UTC())
		if !matched {
			continue
		}
		candidate := domain.AlertCandidate{
			UserID:         input.Event.UserID,
			RuleID:         rule.ID,
			ItemEventID:    input.Event.ID,
			SourceID:       input.Event.SourceID,
			ItemID:         input.Event.ItemID,
			Status:         alertCandidateStatusForRule(rule),
			MatchedReasons: reasons,
			DedupeKey:      alertCandidateDedupeKey(rule.ID, input.Event.ItemID),
		}
		created, err := s.candidateStore.Create(ctx, candidate)
		if err != nil {
			opErr = err
			return ProcessItemEventResult{}, opErr
		}
		result.CandidateCount++
		result.Candidates = append(result.Candidates, created)
	}
	span.SetAttributes(
		attribute.Int("alert_rule.count", result.RuleCount),
		attribute.Int("alert_candidate.created", result.CandidateCount),
	)
	return result, nil
}

func alertRuleMatchesItemEvent(rule domain.AlertRule, event domain.ItemEvent, now time.Time) (bool, []string) {
	if !rule.Enabled {
		return false, nil
	}
	if rule.CooldownSeconds > 0 && rule.LastTriggeredAt != nil && now.Sub(*rule.LastTriggeredAt) < time.Duration(rule.CooldownSeconds)*time.Second {
		return false, nil
	}

	switch rule.Scope {
	case domain.AlertRuleScopeGlobal:
		return true, []string{"global rule"}
	case domain.AlertRuleScopeSource:
		if containsInt64(rule.Condition.SourceIDs, event.SourceID) {
			return true, []string{"source matched"}
		}
	case domain.AlertRuleScopeKeyword:
		text := strings.ToLower(itemEventSearchText(event))
		for _, keyword := range rule.Condition.Keywords {
			keyword = strings.ToLower(strings.TrimSpace(keyword))
			if keyword != "" && strings.Contains(text, keyword) {
				return true, []string{"keyword matched: " + keyword}
			}
		}
	case domain.AlertRuleScopeCategory:
		category := strings.ToLower(strings.TrimSpace(stringPayloadValue(event.Payload, "category")))
		if category != "" && containsStringFold(rule.Condition.Categories, category) {
			return true, []string{"category matched: " + category}
		}
	case domain.AlertRuleScopeTag:
		for _, tag := range stringSlicePayloadValue(event.Payload, "tags") {
			if containsStringFold(rule.Condition.Tags, tag) {
				return true, []string{"tag matched: " + strings.ToLower(tag)}
			}
		}
	case domain.AlertRuleScopeTicker:
		for _, ticker := range stringSlicePayloadValue(event.Payload, "tickers") {
			if containsStringFold(rule.Condition.Tickers, ticker) {
				return true, []string{"ticker matched: " + strings.ToUpper(ticker)}
			}
		}
	}
	return false, nil
}

func alertCandidateStatusForRule(rule domain.AlertRule) domain.AlertCandidateStatus {
	if rule.AIRequired {
		return domain.AlertCandidateStatusPendingAnalysis
	}
	return domain.AlertCandidateStatusReady
}

func alertCandidateDedupeKey(ruleID int64, itemID int64) string {
	return "alert_candidate:" + strconv.FormatInt(ruleID, 10) + ":" + strconv.FormatInt(itemID, 10)
}

func itemEventSearchText(event domain.ItemEvent) string {
	values := []string{
		stringPayloadValue(event.Payload, "title"),
		stringPayloadValue(event.Payload, "url"),
		stringPayloadValue(event.Payload, "normalized_url"),
	}
	return strings.Join(values, " ")
}

func stringPayloadValue(payload domain.ItemEventPayload, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func stringSlicePayloadValue(payload domain.ItemEventPayload, key string) []string {
	if payload == nil {
		return nil
	}
	value, ok := payload[key]
	if !ok {
		return nil
	}
	if values, ok := value.([]string); ok {
		return values
	}
	if values, ok := value.([]any); ok {
		result := make([]string, 0, len(values))
		for _, item := range values {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	}
	return nil
}

func containsInt64(values []int64, target int64) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsStringFold(values []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) == target {
			return true
		}
	}
	return false
}
