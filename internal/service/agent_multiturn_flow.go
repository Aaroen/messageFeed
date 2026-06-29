package service

import (
	"context"
	"encoding/json"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"strconv"
	"strings"
	"time"
)

type agentFollowupIntent string

const (
	agentFollowupIntentNewTask           agentFollowupIntent = "new_task"
	agentFollowupIntentStop              agentFollowupIntent = "stop"
	agentFollowupIntentAppendConstraints agentFollowupIntent = "append_constraints"
	agentFollowupIntentRetry             agentFollowupIntent = "retry"
	agentFollowupIntentQuestion          agentFollowupIntent = "followup_question"
	agentFollowupIntentDeriveTask        agentFollowupIntent = "derive_task"
)

// handleMultiTurnMessage 处理同一会话内的停止、补充、重试和结果追问。
func (s *AgentConversationService) handleMultiTurnMessage(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
) (bool, ReceiveWeChatWorkAppMessageResult, error) {
	ctx = withAgentLLMUserID(ctx, account.UserID)
	message := strings.TrimSpace(input.TextContent)
	if s == nil || s.repository == nil || message == "" {
		return false, ReceiveWeChatWorkAppMessageResult{}, nil
	}
	candidates, err := s.selectMultiTurnPlanCandidates(ctx, account.UserID, session.ID)
	if err != nil {
		return false, ReceiveWeChatWorkAppMessageResult{}, err
	}
	if !candidates.hasAny() {
		return false, ReceiveWeChatWorkAppMessageResult{}, nil
	}
	decision := s.classifyAgentFollowupIntent(ctx, account, session, turn, input, candidates)
	intent := decision.Intent
	now := s.now().UTC()
	if candidates.ActiveFound && (intent == agentFollowupIntentNewTask || intent == agentFollowupIntentDeriveTask || intent == agentFollowupIntentAppendConstraints) {
		if err := s.supersedeActivePlanForNewTask(ctx, account.UserID, session.ID, turn.ID, candidates.Active, input); err != nil {
			return false, ReceiveWeChatWorkAppMessageResult{}, err
		}
		return false, ReceiveWeChatWorkAppMessageResult{}, nil
	}
	if intent == agentFollowupIntentNewTask || intent == agentFollowupIntentDeriveTask {
		if intent == agentFollowupIntentDeriveTask && candidates.CompletedFound {
			s.rememberDerivedParentPlan(turn.ID, candidates.Completed)
		}
		return false, ReceiveWeChatWorkAppMessageResult{}, nil
	}
	plan, found, stale := candidates.planForIntent(intent)
	if !found {
		return false, ReceiveWeChatWorkAppMessageResult{}, nil
	}
	if stale && intent == agentFollowupIntentQuestion {
		plan.Metadata = updateResultReuseMetadata(plan, message, now, true)
		updated, err := s.repository.UpdateAgentPlanMetadata(ctx, account.UserID, plan.ID, plan.Metadata, now)
		if err != nil {
			return false, ReceiveWeChatWorkAppMessageResult{}, err
		}
		s.recordMultiTurnAudit(ctx, account.UserID, session.ID, turn.ID, updated, input, "agent.plan_result_stale", "stale", message)
		reply, answerErr := s.generateMultiTurnFollowupAnswer(ctx, account, session, turn, input, updated, message)
		if answerErr != nil {
			return true, s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, updated, answerErr), nil
		}
		result, err := s.finishTurnWithReply(ctx, account, inbound, session, turn, input, reply, nil, "followup_stale")
		result.Plan = updated
		return true, result, err
	}
	if stale {
		return false, ReceiveWeChatWorkAppMessageResult{}, nil
	}
	switch intent {
	case agentFollowupIntentStop:
		if !agentPlanCanStop(plan.Status) {
			return false, ReceiveWeChatWorkAppMessageResult{}, nil
		}
		updated, _, err := s.stopExistingAgentPlan(ctx, account.UserID, plan, message)
		if err != nil {
			return false, ReceiveWeChatWorkAppMessageResult{}, err
		}
		updated.Metadata = updateMultiTurnMetadata(updated, intent, message, now)
		updated, err = s.repository.UpdateAgentPlanMetadata(ctx, account.UserID, updated.ID, updated.Metadata, now)
		if err != nil {
			return false, ReceiveWeChatWorkAppMessageResult{}, err
		}
		s.recordMultiTurnAudit(ctx, account.UserID, session.ID, turn.ID, updated, input, "agent.plan_stopped", "stopped", message)
		reply := s.generateAgentWeChatFeedbackText(ctx, agentWeChatFeedbackRequest{
			Stage:       "stopped",
			UserMessage: input.TextContent,
			Plan:        updated,
			ErrorText:   updated.ErrorMessage,
			ProgressURL: s.agentPlanURLIfAvailable(updated.ID),
		})
		result, err := s.finishTurnWithReply(ctx, account, inbound, session, turn, input, reply, nil, "stopped")
		result.Plan = updated
		return true, result, err
	case agentFollowupIntentAppendConstraints:
		return false, ReceiveWeChatWorkAppMessageResult{}, nil
	case agentFollowupIntentRetry:
		if plan.Status != domain.AgentPlanStatusFailed && plan.Status != domain.AgentPlanStatusCompleted {
			if !isActiveMultiTurnPlan(plan.Status) {
				return false, ReceiveWeChatWorkAppMessageResult{}, nil
			}
			s.recordMultiTurnAudit(ctx, account.UserID, session.ID, turn.ID, plan, input, "agent.plan_retry_requested", "skipped", message)
			reply := s.generateAgentWeChatFeedbackText(ctx, agentWeChatFeedbackRequest{
				Stage:       "retry_skipped",
				UserMessage: input.TextContent,
				Plan:        plan,
				ProgressURL: s.agentPlanURLIfAvailable(plan.ID),
			})
			result, err := s.finishTurnWithReply(ctx, account, inbound, session, turn, input, reply, nil, "retry_skipped")
			result.Plan = plan
			return true, result, err
		}
		plan.Metadata = updateMultiTurnMetadata(plan, intent, message, now)
		updated, err := s.repository.UpdateAgentPlanMetadata(ctx, account.UserID, plan.ID, plan.Metadata, now)
		if err != nil {
			return false, ReceiveWeChatWorkAppMessageResult{}, err
		}
		s.recordMultiTurnAudit(ctx, account.UserID, session.ID, turn.ID, updated, input, "agent.plan_retry_requested", "rerouted_to_new_plan", message)
		return false, ReceiveWeChatWorkAppMessageResult{}, nil
	case agentFollowupIntentQuestion:
		plan.Metadata = updateResultReuseMetadata(plan, message, now, false)
		updated, err := s.repository.UpdateAgentPlanMetadata(ctx, account.UserID, plan.ID, plan.Metadata, now)
		if err != nil {
			return false, ReceiveWeChatWorkAppMessageResult{}, err
		}
		s.recordMultiTurnAudit(ctx, account.UserID, session.ID, turn.ID, updated, input, "agent.plan_result_reused", "succeeded", message)
		reply, answerErr := s.generateMultiTurnFollowupAnswer(ctx, account, session, turn, input, updated, message)
		if answerErr != nil {
			return true, s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, updated, answerErr), nil
		}
		result, err := s.finishTurnWithReply(ctx, account, inbound, session, turn, input, reply, nil, "followup_reused")
		result.Plan = updated
		return true, result, err
	default:
		return false, ReceiveWeChatWorkAppMessageResult{}, nil
	}
}

type agentFollowupDecision struct {
	Intent     agentFollowupIntent
	Confidence float64
	Reason     string
}

type agentFollowupDecisionJSON struct {
	Intent     string  `json:"intent"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

type multiTurnPlanCandidates struct {
	Active         domain.AgentPlan
	ActiveFound    bool
	ActiveStale    bool
	Failed         domain.AgentPlan
	FailedFound    bool
	FailedStale    bool
	Completed      domain.AgentPlan
	CompletedFound bool
	CompletedStale bool
}

func (c multiTurnPlanCandidates) hasAny() bool {
	return c.ActiveFound || c.FailedFound || c.CompletedFound
}

func (c multiTurnPlanCandidates) planForIntent(intent agentFollowupIntent) (domain.AgentPlan, bool, bool) {
	switch intent {
	case agentFollowupIntentStop, agentFollowupIntentAppendConstraints:
		return c.Active, c.ActiveFound, c.ActiveStale
	case agentFollowupIntentRetry:
		if c.CompletedFound && c.FailedFound {
			if multiTurnPlanReferenceTime(c.Completed).After(multiTurnPlanReferenceTime(c.Failed)) {
				return c.Completed, true, c.CompletedStale
			}
			return c.Failed, true, c.FailedStale
		}
		if c.CompletedFound {
			return c.Completed, true, c.CompletedStale
		}
		if c.FailedFound {
			return c.Failed, true, c.FailedStale
		}
		return c.Active, c.ActiveFound, c.ActiveStale
	case agentFollowupIntentQuestion:
		if c.ActiveFound {
			return c.Active, true, c.ActiveStale
		}
		if c.CompletedFound {
			return c.Completed, true, c.CompletedStale
		}
		if c.FailedFound {
			return c.Failed, true, c.FailedStale
		}
	}
	return domain.AgentPlan{}, false, false
}

func (s *AgentConversationService) selectMultiTurnPlanCandidates(ctx context.Context, userID int64, sessionID int64) (multiTurnPlanCandidates, error) {
	plans, err := s.repository.ListAgentPlans(ctx, userID, sessionID, 0, 10)
	if err != nil {
		return multiTurnPlanCandidates{}, err
	}
	now := s.now().UTC()
	candidates := multiTurnPlanCandidates{}
	for _, plan := range plans {
		stale := isStaleMultiTurnPlan(plan, now)
		if !candidates.ActiveFound && agentPlanCanStop(plan.Status) {
			candidates.Active = plan
			candidates.ActiveFound = true
			candidates.ActiveStale = stale
			continue
		}
		if !candidates.FailedFound && plan.Status == domain.AgentPlanStatusFailed {
			candidates.Failed = plan
			candidates.FailedFound = true
			candidates.FailedStale = stale
			continue
		}
		if !candidates.CompletedFound && plan.Status == domain.AgentPlanStatusCompleted {
			candidates.Completed = plan
			candidates.CompletedFound = true
			candidates.CompletedStale = stale
		}
	}
	return candidates, nil
}

func multiTurnPlanReferenceTime(plan domain.AgentPlan) time.Time {
	reference := plan.UpdatedAt
	if reference.IsZero() {
		reference = plan.CreatedAt
	}
	if plan.CompletedAt != nil && !plan.CompletedAt.IsZero() && plan.CompletedAt.After(reference) {
		reference = *plan.CompletedAt
	}
	if plan.FailedAt != nil && !plan.FailedAt.IsZero() && plan.FailedAt.After(reference) {
		reference = *plan.FailedAt
	}
	return reference.UTC()
}

func (s *AgentConversationService) classifyAgentFollowupIntent(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	candidates multiTurnPlanCandidates,
) agentFollowupDecision {
	if s == nil || s.llmClient == nil {
		return agentFollowupDecision{Intent: agentFollowupIntentNewTask, Reason: "llm_unavailable"}
	}
	payload := domain.AgentJSON{
		"user_message": input.TextContent,
		"user_id":      account.UserID,
		"session_id":   session.ID,
		"turn_id":      turn.ID,
		"active_plan":  multiTurnPlanDecisionSummary(candidates.Active, candidates.ActiveFound),
		"failed_plan":  multiTurnPlanDecisionSummary(candidates.Failed, candidates.FailedFound),
		"completed_plan": multiTurnPlanDecisionSummary(
			candidates.Completed,
			candidates.CompletedFound,
		),
		"allowed_intents": []string{
			string(agentFollowupIntentNewTask),
			string(agentFollowupIntentStop),
			string(agentFollowupIntentAppendConstraints),
			string(agentFollowupIntentRetry),
			string(agentFollowupIntentQuestion),
			string(agentFollowupIntentDeriveTask),
		},
		"required_schema": agentFollowupIntentSchemaHint(),
	}
	body, _ := json.Marshal(payload)
	response, err := s.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: agentFollowupIntentSystemPrompt()},
			{Role: "user", Content: string(body)},
		},
		Temperature: 0.1,
		MaxTokens:   256,
	})
	if err != nil {
		s.recordMultiTurnDecisionAudit(ctx, account.UserID, session.ID, turn.ID, input, "llm_error", err.Error(), domain.AgentJSON{"payload": payload})
		return agentFollowupDecision{Intent: agentFollowupIntentNewTask, Reason: "llm_error"}
	}
	decision, parseErr := parseAgentFollowupDecision(response.Content)
	if parseErr != nil {
		s.recordMultiTurnDecisionAudit(ctx, account.UserID, session.ID, turn.ID, input, "parse_error", parseErr.Error(), domain.AgentJSON{
			"payload":      payload,
			"raw_response": safeSummary(response.Content, 1000),
			"provider":     response.Provider,
			"model":        response.Model,
		})
		return agentFollowupDecision{Intent: agentFollowupIntentNewTask, Reason: "parse_error"}
	}
	s.recordMultiTurnDecisionAudit(ctx, account.UserID, session.ID, turn.ID, input, "succeeded", decision.Reason, domain.AgentJSON{
		"intent":     string(decision.Intent),
		"confidence": decision.Confidence,
		"provider":   response.Provider,
		"model":      response.Model,
	})
	return decision
}

func parseAgentFollowupDecision(raw string) (agentFollowupDecision, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return agentFollowupDecision{}, fmt.Errorf("followup decision json object is missing")
	}
	var decoded agentFollowupDecisionJSON
	if err := json.Unmarshal([]byte(raw[start:end+1]), &decoded); err != nil {
		return agentFollowupDecision{}, err
	}
	intent := normalizeAgentFollowupIntent(decoded.Intent)
	if intent == "" {
		return agentFollowupDecision{}, fmt.Errorf("followup intent is invalid: %s", decoded.Intent)
	}
	return agentFollowupDecision{
		Intent:     intent,
		Confidence: decoded.Confidence,
		Reason:     strings.TrimSpace(decoded.Reason),
	}, nil
}

func normalizeAgentFollowupIntent(value string) agentFollowupIntent {
	switch agentFollowupIntent(strings.TrimSpace(value)) {
	case agentFollowupIntentNewTask,
		agentFollowupIntentStop,
		agentFollowupIntentAppendConstraints,
		agentFollowupIntentRetry,
		agentFollowupIntentQuestion,
		agentFollowupIntentDeriveTask:
		return agentFollowupIntent(strings.TrimSpace(value))
	default:
		return ""
	}
}

func multiTurnPlanDecisionSummary(plan domain.AgentPlan, found bool) domain.AgentJSON {
	if !found || plan.ID < 1 {
		return domain.AgentJSON{"found": false}
	}
	return domain.AgentJSON{
		"found":      true,
		"id":         plan.ID,
		"status":     string(plan.Status),
		"goal":       safeSummary(plan.Goal, 300),
		"summary":    safeSummary(plan.Summary, 300),
		"updated_at": plan.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *AgentConversationService) recordMultiTurnDecisionAudit(ctx context.Context, userID int64, sessionID int64, turnID int64, input ReceiveWeChatWorkAppMessageInput, status string, message string, metadata domain.AgentJSON) {
	if s == nil || s.repository == nil {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: sessionID,
		TurnID:    turnID,
		UserID:    userID,
		EventType: "agent.followup_intent_decided",
		Status:    status,
		Message:   message,
		Metadata:  metadata,
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
	})
}

func isActiveMultiTurnPlan(status domain.AgentPlanStatus) bool {
	switch status {
	case domain.AgentPlanStatusAwaitingApproval, domain.AgentPlanStatusApproved, domain.AgentPlanStatusExecuting, domain.AgentPlanStatusFailed:
		return true
	default:
		return false
	}
}

func isStaleMultiTurnPlan(plan domain.AgentPlan, now time.Time) bool {
	if plan.Status == domain.AgentPlanStatusFailed {
		reference := plan.UpdatedAt
		if reference.IsZero() {
			reference = plan.CreatedAt
		}
		if reference.IsZero() {
			return false
		}
		return now.Sub(reference.UTC()) > 72*time.Hour
	}
	freshness := planResultFreshness(plan, now)
	return freshness.Stale
}

func updateMultiTurnMetadata(plan domain.AgentPlan, intent agentFollowupIntent, message string, now time.Time) domain.AgentJSON {
	metadata := cloneApprovalMetadata(plan.Metadata)
	raw, _ := metadata["multi_turn"].(map[string]any)
	if raw == nil {
		if typed, ok := metadata["multi_turn"].(domain.AgentJSON); ok {
			raw = map[string]any(typed)
		}
	}
	multiTurn := make(map[string]any, len(raw)+6)
	for key, value := range raw {
		multiTurn[key] = value
	}
	if originalGoal, _ := multiTurn["original_goal"].(string); strings.TrimSpace(originalGoal) == "" {
		multiTurn["original_goal"] = plan.Goal
	}
	multiTurn["latest_user_instruction"] = message
	multiTurn["latest_intent"] = string(intent)
	multiTurn["updated_at"] = now.UTC().Format(time.RFC3339)
	switch intent {
	case agentFollowupIntentAppendConstraints:
		multiTurn["appended_inputs"] = appendMultiTurnEntry(multiTurn["appended_inputs"], message, now)
	case agentFollowupIntentQuestion:
		multiTurn["followup_questions"] = appendMultiTurnEntry(multiTurn["followup_questions"], message, now)
	case agentFollowupIntentRetry:
		multiTurn["retry_requests"] = appendMultiTurnEntry(multiTurn["retry_requests"], message, now)
	case agentFollowupIntentStop:
		multiTurn["stopped"] = true
		multiTurn["stopped_reason"] = message
		multiTurn["stopped_at"] = now.UTC().Format(time.RFC3339)
	}
	metadata["multi_turn"] = multiTurn
	return metadata
}

type agentResultFreshness struct {
	Status      string
	Hint        string
	ReferenceAt time.Time
	StaleAfter  time.Duration
	Stale       bool
}

func planResultFreshness(plan domain.AgentPlan, now time.Time) agentResultFreshness {
	reference := plan.UpdatedAt
	if plan.CompletedAt != nil && !plan.CompletedAt.IsZero() {
		reference = *plan.CompletedAt
	}
	if reference.IsZero() {
		reference = plan.CreatedAt
	}
	staleAfter := 24 * time.Hour
	hint := "默认任务结果 24 小时内可直接复用。"
	if planUsesCapability(plan, "web.") {
		staleAfter = 6 * time.Hour
		hint = "联网结果 6 小时后建议刷新。"
	} else if planUsesCapability(plan, "feed.") || planUsesCapability(plan, "source.") {
		staleAfter = 12 * time.Hour
		hint = "订阅源结果 12 小时后建议刷新。"
	} else if planUsesCapability(plan, "conversation.") {
		staleAfter = 30 * 24 * time.Hour
		hint = "历史对话结果属于同用户会话记忆，30 天内可作为上下文引用。"
	}
	stale := !reference.IsZero() && now.Sub(reference.UTC()) > staleAfter
	status := "fresh"
	if stale {
		status = "stale"
	}
	return agentResultFreshness{Status: status, Hint: hint, ReferenceAt: reference.UTC(), StaleAfter: staleAfter, Stale: stale}
}

func planUsesCapability(plan domain.AgentPlan, prefix string) bool {
	for _, scope := range plan.AllowedScopes {
		if strings.HasPrefix(scope, prefix) {
			return true
		}
	}
	for _, step := range plan.Steps {
		if strings.HasPrefix(step.CapabilityKey, prefix) {
			return true
		}
	}
	return false
}

func updateResultReuseMetadata(plan domain.AgentPlan, question string, now time.Time, stale bool) domain.AgentJSON {
	metadata := updateMultiTurnMetadata(plan, agentFollowupIntentQuestion, question, now)
	raw, _ := metadata["multi_turn"].(map[string]any)
	if raw == nil {
		raw = map[string]any{}
	}
	reuse := buildPlanResultReuseMetadata(plan, now)
	if stale {
		reuse["freshness_status"] = "stale"
		reuse["reuse_allowed"] = false
	}
	reuse["question"] = question
	reuse["reused_at"] = now.UTC().Format(time.RFC3339)
	raw["result_reuse"] = reuse
	raw["memory_scope"] = "task_result"
	metadata["multi_turn"] = raw
	metadata["memory_governance"] = domain.AgentJSON{
		"short_term_context": "current_session",
		"long_term_memory":   "agent_transcript_and_recall_events",
		"task_result_memory": "agent_plan_steps_artifacts_and_observations",
		"external_evidence":  "artifact_source_refs_and_capability_evidence_refs",
		"redaction_policy":   "secret, token, webhook url and database dsn are excluded from reusable metadata",
		"updated_at":         now.UTC().Format(time.RFC3339),
	}
	return metadata
}

func buildPlanResultReuseMetadata(plan domain.AgentPlan, now time.Time) map[string]any {
	freshness := planResultFreshness(plan, now)
	refs := planEvidenceRefs(plan)
	reuseAllowed := freshness.Status == "fresh"
	output := map[string]any{
		"source_plan_id":    plan.ID,
		"source_session_id": plan.SessionID,
		"source_turn_id":    plan.TurnID,
		"source_goal":       plan.Goal,
		"source_status":     string(plan.Status),
		"freshness_status":  freshness.Status,
		"freshness_hint":    freshness.Hint,
		"reuse_allowed":     reuseAllowed,
		"evidence_refs":     refs,
		"memory_type":       "task_result",
	}
	if !freshness.ReferenceAt.IsZero() {
		output["result_updated_at"] = freshness.ReferenceAt.Format(time.RFC3339)
		output["stale_after"] = freshness.ReferenceAt.Add(freshness.StaleAfter).Format(time.RFC3339)
	}
	return output
}

func planEvidenceRefs(plan domain.AgentPlan) []string {
	refs := []string{"agent_plan:" + strconv.FormatInt(plan.ID, 10)}
	if plan.TurnID > 0 {
		refs = append(refs, "agent_turn:"+strconv.FormatInt(plan.TurnID, 10))
	}
	for _, step := range plan.Steps {
		if step.ID > 0 {
			refs = append(refs, "agent_plan_step:"+strconv.FormatInt(step.ID, 10))
		}
		if strings.TrimSpace(step.ObservationRef) != "" {
			refs = append(refs, step.ObservationRef)
		}
		refs = append(refs, compactNonEmptyStrings(step.ArtifactRefs)...)
	}
	return compactNonEmptyStrings(refs)
}

func (s *AgentConversationService) rememberDerivedParentPlan(turnID int64, plan domain.AgentPlan) {
	if s == nil || turnID < 1 || plan.ID < 1 {
		return
	}
	s.activeProcessMu.Lock()
	defer s.activeProcessMu.Unlock()
	if s.derivedParentByTurnID == nil {
		s.derivedParentByTurnID = map[int64]domain.AgentPlan{}
	}
	s.derivedParentByTurnID[turnID] = plan
}

func (s *AgentConversationService) takeDerivedParentPlan(turnID int64, userID int64, sessionID int64) (domain.AgentPlan, bool, bool, error) {
	if s == nil || turnID < 1 {
		return domain.AgentPlan{}, false, false, nil
	}
	s.activeProcessMu.Lock()
	plan, found := s.derivedParentByTurnID[turnID]
	if found {
		delete(s.derivedParentByTurnID, turnID)
	}
	s.activeProcessMu.Unlock()
	if !found || plan.ID < 1 {
		return domain.AgentPlan{}, false, false, nil
	}
	if plan.UserID != userID || plan.SessionID != sessionID || plan.Status != domain.AgentPlanStatusCompleted {
		return domain.AgentPlan{}, false, false, nil
	}
	return plan, true, isStaleMultiTurnPlan(plan, s.now().UTC()), nil
}

func (s *AgentConversationService) selectDerivedParentPlanForTurn(ctx context.Context, userID int64, sessionID int64, turnID int64) (domain.AgentPlan, bool, bool, error) {
	return s.takeDerivedParentPlan(turnID, userID, sessionID)
}

func updateDerivedPlanMetadata(plan domain.AgentPlan, parent domain.AgentPlan, message string, now time.Time, parentStale bool) domain.AgentJSON {
	metadata := cloneApprovalMetadata(plan.Metadata)
	reuse := buildPlanResultReuseMetadata(parent, now)
	if parentStale {
		reuse["freshness_status"] = "stale"
		reuse["reuse_allowed"] = false
	}
	metadata["parent_plan"] = domain.AgentJSON{
		"id":               parent.ID,
		"session_id":       parent.SessionID,
		"turn_id":          parent.TurnID,
		"goal":             parent.Goal,
		"status":           string(parent.Status),
		"derive_reason":    message,
		"derived_at":       now.UTC().Format(time.RFC3339),
		"freshness_status": reuse["freshness_status"],
		"freshness_hint":   reuse["freshness_hint"],
		"evidence_refs":    reuse["evidence_refs"],
	}
	metadata["result_reuse"] = reuse
	metadata["memory_governance"] = domain.AgentJSON{
		"short_term_context": "current_session",
		"long_term_memory":   "agent_transcript_and_recall_events",
		"task_result_memory": "parent_agent_plan",
		"external_evidence":  "parent_plan_artifact_refs",
		"redaction_policy":   "secret, token, webhook url and database dsn are excluded from reusable metadata",
		"updated_at":         now.UTC().Format(time.RFC3339),
	}
	return metadata
}

func appendMultiTurnEntry(raw any, message string, now time.Time) []any {
	entries, _ := raw.([]any)
	copied := append([]any(nil), entries...)
	copied = append(copied, map[string]any{
		"message":    message,
		"created_at": now.UTC().Format(time.RFC3339),
	})
	if len(copied) > 20 {
		copied = copied[len(copied)-20:]
	}
	return copied
}

func (s *AgentConversationService) recordMultiTurnAudit(ctx context.Context, userID int64, sessionID int64, turnID int64, plan domain.AgentPlan, input ReceiveWeChatWorkAppMessageInput, eventType string, status string, message string) {
	if s == nil || s.repository == nil {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: sessionID,
		TurnID:    turnID,
		UserID:    userID,
		EventType: eventType,
		Status:    status,
		Message:   message,
		Metadata: domain.AgentJSON{
			"plan_id":      plan.ID,
			"plan_status":  string(plan.Status),
			"progress_url": s.agentPlanURL(plan.ID),
			"metadata":     cloneApprovalMetadata(plan.Metadata),
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
	})
}

func (s *AgentConversationService) generateMultiTurnFollowupAnswer(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	plan domain.AgentPlan,
	question string,
) (string, error) {
	if s == nil || s.llmClient == nil {
		return "", domain.NewAppError(domain.ErrorKindUnavailable, "agent_followup_answer_llm_unavailable", "agent followup answer llm is unavailable", "service.agent.followup_answer", true, nil)
	}
	payload := s.multiTurnFollowupAnswerPayload(account, session, turn, input, plan, question)
	body, err := json.Marshal(payload)
	if err != nil {
		return "", domain.NewAppError(domain.ErrorKindInternal, "agent_followup_answer_payload_invalid", "agent followup answer payload is invalid", "service.agent.followup_answer", true, err)
	}
	messages := []llm.ChatMessage{
		{Role: "system", Content: agentFollowupAnswerSystemPrompt()},
		{Role: "user", Content: string(body)},
	}
	response, err := s.llmClient.Chat(ctx, llm.ChatRequest{
		Messages:    messages,
		Temperature: 0.2,
		MaxTokens:   512,
	})
	if err != nil {
		s.recordMultiTurnAudit(ctx, account.UserID, session.ID, turn.ID, plan, input, "agent.plan_followup_answer", "failed", err.Error())
		return "", domain.NewAppError(domain.ErrorKindUnavailable, "agent_followup_answer_failed", "agent followup answer generation failed", "service.agent.followup_answer", true, err)
	}
	reply, err := s.repairMultiTurnFollowupPlainText(ctx, messages, response)
	if err != nil {
		s.recordMultiTurnAudit(ctx, account.UserID, session.ID, turn.ID, plan, input, "agent.plan_followup_answer", "failed", err.Error())
		return "", err
	}
	s.recordMultiTurnAudit(ctx, account.UserID, session.ID, turn.ID, plan, input, "agent.plan_followup_answer", "succeeded", "model generated followup answer")
	return reply, nil
}

func (s *AgentConversationService) multiTurnFollowupAnswerPayload(
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
	plan domain.AgentPlan,
	question string,
) domain.AgentJSON {
	freshness := planResultFreshness(plan, s.now().UTC())
	staleAfter := ""
	if !freshness.ReferenceAt.IsZero() {
		staleAfter = formatAgentTime(freshness.ReferenceAt.Add(freshness.StaleAfter))
	}
	return domain.AgentJSON{
		"user_message": safeSummary(question, 600),
		"user_id":      account.UserID,
		"session_id":   session.ID,
		"turn_id":      turn.ID,
		"source_plan": domain.AgentJSON{
			"id":             plan.ID,
			"goal":           safeSummary(plan.Goal, 600),
			"summary":        safeSummary(plan.Summary, 1200),
			"impact_summary": safeSummary(plan.ImpactSummary, 1200),
			"status":         string(plan.Status),
			"error":          safeSummary(plan.ErrorMessage, 600),
			"completed_at":   formatOptionalAgentTime(plan.CompletedAt),
			"updated_at":     plan.UpdatedAt.UTC().Format(time.RFC3339),
			"progress_url":   s.agentPlanURLIfAvailable(plan.ID),
		},
		"freshness": domain.AgentJSON{
			"status":       freshness.Status,
			"hint":         freshness.Hint,
			"reference_at": formatAgentTime(freshness.ReferenceAt),
			"stale_after":  staleAfter,
		},
		"steps":               multiTurnFollowupStepPayload(plan),
		"evidence_refs":       planEvidenceRefs(plan),
		"provider_message_id": input.ProviderMessageID,
	}
}

func multiTurnFollowupStepPayload(plan domain.AgentPlan) []domain.AgentJSON {
	steps := make([]domain.AgentJSON, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		steps = append(steps, domain.AgentJSON{
			"id":              step.ID,
			"title":           safeSummary(step.Title, 200),
			"capability":      step.CapabilityKey,
			"status":          string(step.Status),
			"input_summary":   safeSummary(step.InputSummary, 500),
			"output_summary":  safeSummary(step.OutputSummary, 1000),
			"expected_output": safeSummary(step.ExpectedOutput, 500),
			"error":           safeSummary(step.ErrorMessage, 500),
			"observation_ref": step.ObservationRef,
			"artifact_refs":   append([]string(nil), step.ArtifactRefs...),
		})
	}
	return steps
}

func (s *AgentConversationService) repairMultiTurnFollowupPlainText(ctx context.Context, messages []llm.ChatMessage, response llm.ChatResponse) (string, error) {
	current := strings.TrimSpace(response.Content)
	if current == "" {
		return "", domain.NewAppError(domain.ErrorKindUnavailable, "agent_followup_answer_empty", "agent followup answer is empty", "service.agent.followup_answer", true, nil)
	}
	for attempt := 0; attempt < 2; attempt++ {
		violation := agent.PlainTextReplyViolation(current)
		if violation == "" {
			return current, nil
		}
		repairMessages := append([]llm.ChatMessage(nil), messages...)
		repairMessages = append(repairMessages, llm.ChatMessage{Role: "assistant", Content: current})
		repairMessages = append(repairMessages, llm.ChatMessage{Role: "user", Content: agent.PlainTextReplyRepairPrompt(current, violation, attempt+1)})
		repaired, err := s.llmClient.Chat(ctx, llm.ChatRequest{
			Messages:    repairMessages,
			Temperature: 0.2,
			MaxTokens:   512,
		})
		if err != nil {
			return "", domain.NewAppError(domain.ErrorKindUnavailable, "agent_followup_answer_repair_failed", "agent followup answer repair failed", "service.agent.followup_answer", true, err)
		}
		current = strings.TrimSpace(repaired.Content)
		if current == "" {
			return "", domain.NewAppError(domain.ErrorKindUnavailable, "agent_followup_answer_empty", "agent followup answer is empty", "service.agent.followup_answer", true, nil)
		}
	}
	return "", domain.NewAppError(domain.ErrorKindUnavailable, "agent_followup_answer_plain_text_violation", "agent followup answer does not satisfy plain text format", "service.agent.followup_answer", true, nil)
}

func formatOptionalAgentTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatAgentTime(*value)
}

func formatAgentTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func planStoppedByUser(plan domain.AgentPlan) bool {
	if strings.Contains(strings.ToLower(plan.ErrorMessage), "stopped by user") || strings.Contains(plan.ErrorMessage, "用户停止") {
		return true
	}
	if stop := metadataMap(plan.Metadata, "stop"); len(stop) > 0 {
		return true
	}
	raw, _ := plan.Metadata["multi_turn"].(map[string]any)
	if raw == nil {
		if typed, ok := plan.Metadata["multi_turn"].(domain.AgentJSON); ok {
			raw = map[string]any(typed)
		}
	}
	stopped, _ := raw["stopped"].(bool)
	return stopped
}
