package service

import (
	"context"
	"encoding/json"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const (
	agentTaskRouteQuickAnswer = "quick_answer"
	agentTaskRouteRAGAnswer   = "rag_answer"
	agentTaskRouteDeepTask    = "deep_task"
)

type agentTurnRoutingKey struct{}

type agentTurnRouting struct {
	projection domain.AgentJSON
}

func (s *AgentConversationService) turnPlanningProjection(ctx context.Context, account domain.ExternalAccount, session domain.AgentSession, turn domain.AgentTurn, input ReceiveWeChatWorkAppMessageInput) (domain.AgentJSON, error) {
	routing, _ := ctx.Value(agentTurnRoutingKey{}).(*agentTurnRouting)
	if routing != nil && routing.projection != nil {
		return routing.projection, nil
	}
	projection, err := s.mainAgentPlanningContextProjection(ctx, account, session, turn, input)
	if err == nil && routing != nil {
		routing.projection = projection
	}
	return projection, err
}

type agentTaskRouteClassification struct {
	TaskType              string
	Confidence            float64
	NeedsHistoryRecall    bool
	NeedsTools            bool
	RequiresSubAgent      bool
	EstimatedLatencyClass string
	HistoryQuery          string
	CandidateCapabilities []string
	Reason                string
	Provider              string
	Model                 string
	Raw                   string
	PlanSpec              json.RawMessage
}

type agentTaskRouteJSON struct {
	TaskType              string         `json:"task_type"`
	Confidence            float64        `json:"confidence"`
	NeedsHistoryRecall    bool           `json:"needs_history_recall"`
	NeedsTools            bool           `json:"needs_tools"`
	RequiresSubAgent      bool           `json:"requires_sub_agent"`
	EstimatedLatencyClass string         `json:"estimated_latency_class"`
	HistoryQuery          string         `json:"history_query"`
	CandidateCapabilities []string       `json:"candidate_capabilities"`
	Reason                string         `json:"reason"`
	PlanSpec              map[string]any `json:"plan_spec"`
}

var agentRouteSchema = llm.SchemaFor[agentTaskRouteJSON]("agent_task_route_v2", "task_type")

func (s *AgentConversationService) classifyAgentTaskRoute(ctx context.Context, account domain.ExternalAccount, session domain.AgentSession, turn domain.AgentTurn, controllerRun domain.AgentRun, input ReceiveWeChatWorkAppMessageInput, loadProjection func() (domain.AgentJSON, error)) (agentTaskRouteClassification, error) {
	var startedAt time.Time
	if s != nil {
		startedAt = s.now().UTC()
	}
	ctx, span := observability.StartSpan(ctx, "service.agent.task_route",
		attribute.Int64("auth.user_id", account.UserID),
		attribute.Int64("agent.session_id", session.ID),
		attribute.Int64("agent.turn_id", turn.ID),
	)
	defer observability.EndSpan(span, nil)
	if s == nil || s.llmClient == nil {
		c := fallbackAgentTaskRoute("route_classifier_unavailable")
		s.recordAgentTaskRouteTrace(ctx, input, account, session, turn, controllerRun, c, domain.AgentTraceEventDegraded, startedAt, "route_classifier_unavailable", "")
		return c, nil
	}
	contextProjection, err := loadProjection()
	if err != nil {
		return agentTaskRouteClassification{}, err
	}
	payload := domain.AgentJSON{
		"user_id":            account.UserID,
		"session_id":         session.ID,
		"turn_id":            turn.ID,
		"provider":           input.Provider,
		"message_type":       input.MsgType,
		"user_message":       strings.TrimSpace(input.TextContent),
		"current_time":       s.now().UTC().Format(time.RFC3339),
		"capabilities":       s.mainAgentCapabilityCatalog(),
		"context_projection": contextProjection,
	}
	routeCtx := WithAgentLLMCallContext(ctx, AgentLLMCallContext{
		UserID: account.UserID, SessionID: session.ID, TurnID: turn.ID, ParentRunID: controllerRun.ID,
		Stage: AgentLLMCallStageRoute, PromptVersion: "agent-task-router-v1",
	})
	request := llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: agentTaskRouterSystemPrompt()},
			{Role: "user", Content: agentTaskRouterUserPrompt(payload)},
		},
		Temperature:  0,
		OutputSchema: agentRouteSchema,
	}
	response, err := s.llmClient.Chat(routeCtx, request)
	if routeCtx.Err() == nil && llm.CanRetryStructuredOutput(err) {
		request.SchemaLocalOnly = true
		response, err = s.llmClient.Chat(routeCtx, request)
	}
	if err != nil {
		c := fallbackAgentTaskRoute("route_classifier_failed")
		s.recordAgentTaskRouteTrace(ctx, input, account, session, turn, controllerRun, c, domain.AgentTraceEventDegraded, startedAt, "route_classifier_failed", err.Error())
		return c, nil
	}
	classification, err := parseAgentTaskRouteJSON(response.Content)
	if err != nil {
		c := fallbackAgentTaskRoute("route_classifier_invalid")
		s.recordAgentTaskRouteTrace(ctx, input, account, session, turn, controllerRun, c, domain.AgentTraceEventDegraded, startedAt, "route_classifier_invalid", err.Error())
		return c, nil
	}
	classification.Provider = response.Provider
	classification.Model = response.Model
	classification.Raw = strings.TrimSpace(response.Content)
	classification = guardAgentTaskRoute(classification)
	classification.CandidateCapabilities = s.validAgentTaskRouteCapabilities(classification.CandidateCapabilities)
	s.recordAgentTaskRouteTrace(ctx, input, account, session, turn, controllerRun, classification, domain.AgentTraceEventSucceeded, startedAt, "", "")
	return classification, nil
}

func parseAgentTaskRouteJSON(raw string) (agentTaskRouteClassification, error) {
	content, err := agentRouteSchema.Validate(raw)
	if err != nil {
		return agentTaskRouteClassification{}, err
	}
	var decoded agentTaskRouteJSON
	if err := json.Unmarshal([]byte(content), &decoded); err != nil {
		return agentTaskRouteClassification{}, fmt.Errorf("task route JSON parse failed: %w", err)
	}
	var planSpec json.RawMessage
	if decoded.PlanSpec != nil {
		planSpec, err = json.Marshal(decoded.PlanSpec)
		if err != nil {
			return agentTaskRouteClassification{}, err
		}
	}
	return agentTaskRouteClassification{
		TaskType:              strings.TrimSpace(decoded.TaskType),
		Confidence:            decoded.Confidence,
		NeedsHistoryRecall:    decoded.NeedsHistoryRecall,
		NeedsTools:            decoded.NeedsTools,
		RequiresSubAgent:      decoded.RequiresSubAgent,
		EstimatedLatencyClass: strings.TrimSpace(decoded.EstimatedLatencyClass),
		HistoryQuery:          safeSummary(decoded.HistoryQuery, 300),
		CandidateCapabilities: compactStrings(decoded.CandidateCapabilities),
		Reason:                safeSummary(decoded.Reason, 500),
		PlanSpec:              planSpec,
	}, nil
}

func guardAgentTaskRoute(c agentTaskRouteClassification) agentTaskRouteClassification {
	switch c.TaskType {
	case agentTaskRouteQuickAnswer, agentTaskRouteRAGAnswer, agentTaskRouteDeepTask:
	default:
		c.TaskType = agentTaskRouteDeepTask
	}
	if c.Confidence < 0 {
		c.Confidence = 0
	}
	if c.Confidence > 1 {
		c.Confidence = 1
	}
	switch c.EstimatedLatencyClass {
	case "fast", "normal", "slow":
	default:
		c.EstimatedLatencyClass = "normal"
	}
	c.RequiresSubAgent = c.RequiresSubAgent && len(compactStrings(c.CandidateCapabilities)) >= 3
	if c.TaskType == agentTaskRouteQuickAnswer && (c.NeedsTools || c.RequiresSubAgent || c.NeedsHistoryRecall) {
		c.TaskType = agentTaskRouteDeepTask
	}
	if c.TaskType == agentTaskRouteRAGAnswer {
		c.RequiresSubAgent = false
		c.NeedsHistoryRecall = true
		c.NeedsTools = false
		if strings.TrimSpace(c.HistoryQuery) == "" {
			c.HistoryQuery = "当前用户问题相关的历史上下文"
		}
		if len(c.CandidateCapabilities) == 0 {
			c.CandidateCapabilities = []string{"conversation.query_history", "memory.fact_recall"}
		}
	}
	if strings.TrimSpace(c.Reason) == "" {
		c.Reason = "主 Agent 已完成任务分级。"
	}
	return c
}

func (s *AgentConversationService) validAgentTaskRouteCapabilities(values []string) []string {
	values = compactStrings(values)
	if s == nil || s.capabilityRegistry == nil || len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		capability, ok := s.capabilityRegistry.Get(value)
		if !ok || capability.Mode == agent.CapabilityModeHidden {
			continue
		}
		result = append(result, value)
	}
	return result
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func fallbackAgentTaskRoute(reason string) agentTaskRouteClassification {
	return agentTaskRouteClassification{
		TaskType:              agentTaskRouteDeepTask,
		Confidence:            0,
		NeedsHistoryRecall:    true,
		NeedsTools:            true,
		RequiresSubAgent:      false,
		EstimatedLatencyClass: "slow",
		Reason:                reason,
	}
}

func (s *AgentConversationService) recordAgentTaskRouteTrace(ctx context.Context, input ReceiveWeChatWorkAppMessageInput, account domain.ExternalAccount, session domain.AgentSession, turn domain.AgentTurn, controllerRun domain.AgentRun, route agentTaskRouteClassification, status domain.AgentTraceEventStatus, startedAt time.Time, errorCode string, errorMessage string) {
	if s == nil {
		return
	}
	finishedAt, durationMS := agentTraceFinish(startedAt, s.now)
	metrics.AgentTaskRoutesTotal.WithLabelValues(route.TaskType, string(status), route.EstimatedLatencyClass).Inc()
	metrics.AgentTaskRouteDuration.WithLabelValues(route.TaskType, string(status), route.EstimatedLatencyClass).Observe(float64(durationMS) / 1000)
	event := domain.AgentTraceEvent{
		RequestID:     input.RequestID,
		TraceID:       input.TraceID,
		UserID:        account.UserID,
		SessionID:     session.ID,
		TurnID:        turn.ID,
		RunID:         controllerRun.ID,
		EventKind:     domain.AgentTraceEventPlanner,
		EventName:     "main_agent_task_route",
		Status:        status,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		DurationMS:    durationMS,
		ModelKey:      firstNonEmptyString(llmModelKeyFromParts(route.Provider, route.Model), controllerRun.ModelKey),
		InputSummary:  safeSummary(input.TextContent, 500),
		OutputSummary: fmt.Sprintf("%s confidence %.2f", route.TaskType, route.Confidence),
		ErrorCode:     errorCode,
		ErrorMessage:  errorMessage,
		Metadata: domain.AgentJSON{
			"task_type":               route.TaskType,
			"confidence":              route.Confidence,
			"needs_history_recall":    route.NeedsHistoryRecall,
			"needs_tools":             route.NeedsTools,
			"requires_sub_agent":      route.RequiresSubAgent,
			"estimated_latency_class": route.EstimatedLatencyClass,
			"history_query":           route.HistoryQuery,
			"candidate_capabilities":  append([]string(nil), route.CandidateCapabilities...),
			"reason":                  route.Reason,
		},
		CreatedAt: startedAt,
	}
	s.recordAgentTraceEvent(ctx, event)
}

func llmModelKeyFromParts(provider string, model string) string {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if provider == "" && model == "" {
		return ""
	}
	if provider == "" {
		return model
	}
	if model == "" {
		return provider
	}
	return provider + "/" + model
}

func agentTaskRouteShouldUseLightPlan(route agentTaskRouteClassification) bool {
	if route.Confidence < 0.65 {
		return false
	}
	if route.TaskType == agentTaskRouteQuickAnswer || route.TaskType == agentTaskRouteRAGAnswer {
		return true
	}
	if route.TaskType != agentTaskRouteDeepTask || route.RequiresSubAgent || len(route.CandidateCapabilities) == 0 || len(route.CandidateCapabilities) > 2 {
		return false
	}
	for _, capability := range route.CandidateCapabilities {
		if capability != "conversation.query_history" && capability != "memory.fact_recall" {
			return false
		}
	}
	return true
}

func agentTaskRouteFallbackPlanSpec(route agentTaskRouteClassification, goal string) (agent.PlanSpec, bool) {
	if route.TaskType == agentTaskRouteRAGAnswer || route.TaskType == agentTaskRouteQuickAnswer {
		return agentTaskRoutePlanSpec(route, goal), true
	}
	capabilities := route.CandidateCapabilities
	if len(capabilities) == 0 {
		capabilities = fallbackAgentTaskRouteCapabilities(route)
	}
	if route.TaskType != agentTaskRouteDeepTask || len(capabilities) == 0 {
		return agent.PlanSpec{}, false
	}
	spec := agentTaskRoutePlanSpec(route, goal)
	spec.Complexity = agent.PlanningComplexityComplex
	spec.RequiresSubAgent = false
	spec.DirectAnswerAllowed = false
	spec.RequiredCapabilities = append([]string(nil), capabilities...)
	spec.Subtasks = []agent.PlanSubtask{
		{
			Title:           "执行用户任务",
			Prompt:          "根据用户目标选择可用能力执行任务，优先获取必要证据，再生成用户可读结果。",
			ContextSummary:  route.Reason,
			CapabilityKeys:  append([]string(nil), capabilities...),
			ExpectedOutput:  "可验证证据、执行结论和面向用户的最终答复。",
			FailureStrategy: "能力调用失败时说明已完成部分、失败原因和可继续处理的范围。",
			MaxRetries:      1,
		},
	}
	spec.ExpectedEvidenceScope = []string{"tool_result", "current_context"}
	spec.Metadata["fallback_reason"] = "main_agent_planner_failed"
	if len(route.CandidateCapabilities) == 0 {
		spec.Metadata["fallback_capabilities"] = capabilities
	}
	return spec, true
}

func fallbackAgentTaskRouteCapabilities(route agentTaskRouteClassification) []string {
	capabilities := make([]string, 0, 3)
	if route.NeedsHistoryRecall {
		capabilities = append(capabilities, "conversation.query_history", "memory.fact_recall")
	}
	if route.NeedsTools || route.RequiresSubAgent {
		capabilities = append(capabilities, "web.search")
	}
	return compactStrings(capabilities)
}

func agentTaskRoutePlanSpec(route agentTaskRouteClassification, goal string) agent.PlanSpec {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		goal = "用户任务"
	}
	spec := agent.PlanSpec{
		Goal:                   goal,
		Intent:                 route.Reason,
		TaskType:               route.TaskType,
		Complexity:             agent.PlanningComplexitySimple,
		DirectAnswerAllowed:    true,
		MaxIterations:          1,
		FinalAnswerConstraints: []string{"只输出用户可读结果。"},
		Metadata: domain.AgentJSON{
			"source":                  "task_router",
			"estimated_latency_class": route.EstimatedLatencyClass,
			"route_confidence":        route.Confidence,
		},
	}
	if route.TaskType == agentTaskRouteRAGAnswer {
		query := firstNonEmptyString(route.HistoryQuery, goal)
		spec.Complexity = agent.PlanningComplexityStandard
		spec.RequiresSubAgent = false
		spec.DirectAnswerAllowed = false
		spec.RequiredCapabilities = []string{"conversation.query_history", "memory.fact_recall"}
		spec.Subtasks = []agent.PlanSubtask{
			{
				Title:           "召回历史上下文",
				Prompt:          "围绕用户问题召回相关历史上下文和长期事实索引，提炼可引用依据，并生成简洁答复。",
				ContextSummary:  route.Reason,
				CapabilityKeys:  []string{"conversation.query_history", "memory.fact_recall"},
				ExpectedOutput:  "相关历史依据和面向用户的回答。",
				FailureStrategy: "召回失败时说明依据不足，并给出可回答范围。",
				MaxRetries:      1,
			},
		}
		spec.NeedsHistoryRecall = true
		spec.HistoryQueryPlan = agent.PlanHistoryQueryPlan{
			Mode:     string(domain.AgentFactRecallModeHybrid),
			Query:    query,
			Reason:   route.Reason,
			Limit:    8,
			TimeHint: "",
		}
		spec.ExpectedEvidenceScope = []string{"history", "long_term_fact_index"}
	}
	if route.TaskType == agentTaskRouteDeepTask {
		spec.Complexity = agent.PlanningComplexityStandard
		spec.DirectAnswerAllowed = false
		spec.RequiredCapabilities = append([]string(nil), route.CandidateCapabilities...)
		spec.NeedsHistoryRecall = route.NeedsHistoryRecall
		if route.NeedsHistoryRecall {
			spec.HistoryQueryPlan = agent.PlanHistoryQueryPlan{Mode: "search", Query: firstNonEmptyString(route.HistoryQuery, goal), Reason: route.Reason, Limit: 8}
		}
	}
	return spec
}
