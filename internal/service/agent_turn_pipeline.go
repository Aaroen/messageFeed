package service

import (
	"context"
	"errors"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/observability"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// processTurn 执行单个 turn 的主闭环：建计划、跑工具、回填结果并完成回复。
func (s *AgentConversationService) processTurn(
	ctx context.Context,
	account domain.ExternalAccount,
	inbound domain.AgentInboundMessage,
	session domain.AgentSession,
	turn domain.AgentTurn,
	input ReceiveWeChatWorkAppMessageInput,
) (ReceiveWeChatWorkAppMessageResult, error) {
	ctx, activeProcess, stopWatch := s.watchAgentProcess(ctx, turn)
	defer stopWatch()
	ctx = withAgentLLMUserID(ctx, account.UserID)

	lock := s.sessionLock(session.ID)
	lock.Lock()
	defer lock.Unlock()

	processStartedAt := s.now().UTC()
	ctx, span := observability.StartSpan(ctx, "service.agent.process_turn",
		attribute.Int64("agent.session_id", session.ID),
		attribute.Int64("agent.turn_id", turn.ID),
		attribute.Int64("auth.user_id", account.UserID),
	)
	var opErr error
	processStatus := domain.AgentTraceEventFailed
	processPlanID := int64(0)
	processRunID := int64(0)
	defer func() { observability.EndSpan(span, opErr) }()
	defer func() {
		finishedAt, durationMS := agentTraceFinish(processStartedAt, s.now)
		event := domain.AgentTraceEvent{
			RequestID:    input.RequestID,
			TraceID:      input.TraceID,
			UserID:       account.UserID,
			SessionID:    session.ID,
			TurnID:       turn.ID,
			PlanID:       processPlanID,
			RunID:        processRunID,
			EventKind:    domain.AgentTraceEventInbound,
			EventName:    "process_turn",
			Status:       processStatus,
			StartedAt:    processStartedAt,
			FinishedAt:   finishedAt,
			DurationMS:   durationMS,
			InputSummary: safeSummary(input.TextContent, 500),
			Metadata: domain.AgentJSON{
				"provider":            input.Provider,
				"msg_type":            input.MsgType,
				"provider_message_id": input.ProviderMessageID,
			},
		}
		if opErr != nil {
			event.ErrorCode = "agent_process_turn_failed"
			event.ErrorMessage = opErr.Error()
		}
		s.recordAgentTraceEvent(ctx, event)
	}()

	if s.turnRunner == nil {
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "agent_runner_unavailable", "agent turn runner is unavailable", "service.agent.process_turn", true, nil)
		return s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, domain.AgentPlan{}, opErr)
	}
	controllerRun, err := s.createControllerRun(ctx, account, inbound, session, turn, input)
	if err != nil {
		opErr = err
		return s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, domain.AgentPlan{}, err)
	}
	processRunID = controllerRun.ID
	plan, approvalToken, err := s.createPlanForTurn(ctx, account, session, turn, controllerRun, input)
	if err != nil {
		opErr = err
		_, _ = s.runManager.FailRun(ctx, controllerRun, err)
		return s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, plan, err)
	}
	processPlanID = plan.ID
	s.bindAgentProcessPlan(turn.ID, plan.ID)
	controllerRun = s.alignControllerRunWithPlan(ctx, controllerRun, plan, input)
	if plan.Status == domain.AgentPlanStatusApproved {
		s.recordAgentApprovalTraceEvent(ctx, input, account, session, turn, controllerRun, plan, "approved", domain.AgentTraceEventSucceeded)
		executingPlan, _ := s.repository.UpdateAgentPlanStatus(ctx, account.UserID, plan.ID, domain.AgentPlanStatusExecuting, s.now().UTC(), "")
		if executingPlan.ID > 0 {
			plan = executingPlan
		}
	}
	if plan.Status == domain.AgentPlanStatusRejected {
		s.recordAgentApprovalTraceEvent(ctx, input, account, session, turn, controllerRun, plan, "rejected", domain.AgentTraceEventSucceeded)
		reply := s.generateAgentWeChatFeedbackText(ctx, agentWeChatFeedbackRequest{
			Stage:       "rejected",
			UserMessage: input.TextContent,
			Plan:        plan,
			ErrorText:   planCapabilityPolicySummary(plan),
			ProgressURL: s.agentPlanURLIfAvailable(plan.ID),
		})
		_, _ = s.runManager.CompleteRun(ctx, controllerRun, "plan_rejected_by_capability_policy")
		processStatus = domain.AgentTraceEventSucceeded
		result, err := s.finishTurnWithReply(ctx, account, inbound, session, turn, input, plan, reply, nil, "rejected")
		result.Plan = plan
		return result, err
	}
	if !s.processInline {
		s.sendPlanStartedFeedback(ctx, account, session, turn, input, plan)
	}
	if plan.Status == domain.AgentPlanStatusAwaitingApproval {
		s.recordAgentApprovalTraceEvent(ctx, input, account, session, turn, controllerRun, plan, "awaiting_approval", domain.AgentTraceEventStarted)
		if !s.processInline {
			s.sendPlanProgressNotification(ctx, account, session, turn, input, plan, "approval_waiting", "approval_waiting")
		}
		reply := s.approvalRequiredReply(ctx, input, plan, approvalToken)
		_, _ = s.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
			RunID:     controllerRun.ID,
			TraceKind: "plan_awaiting_approval",
			ModelKey:  controllerRun.ModelKey,
			Content: domain.AgentJSON{
				"plan_id":             plan.ID,
				"status":              string(plan.Status),
				"policy_decision":     plan.PolicyDecision,
				"confirmation_policy": plan.ConfirmationPolicy,
				"allowed_scopes":      plan.AllowedScopes,
			},
			RedactionStatus: "redacted",
		})
		_, _ = s.runManager.CompleteRun(ctx, controllerRun, "plan_approval")
		processStatus = domain.AgentTraceEventSucceeded
		result, err := s.finishTurnWithReply(ctx, account, inbound, session, turn, input, plan, reply, nil, "awaiting_approval")
		result.Plan = plan
		return result, err
	}
	stopPeriodicProgress := s.startPlanPeriodicProgressNotifications(ctx, account, session, turn, input, plan)
	defer stopPeriodicProgress()
	historyQueryPlan := historyQueryPlanForTurn(plan)
	if !planAllowsConversationHistory(plan) {
		historyQueryPlan = agent.PlanHistoryQueryPlan{}
	}
	ctx = WithAgentLLMCallContext(ctx, AgentLLMCallContext{UserID: account.UserID, SessionID: session.ID, TurnID: turn.ID, ParentRunID: controllerRun.ID, Stage: AgentLLMCallStageGeneration})
	runResult, err := s.turnRunner.Run(ctx, agent.TurnRunInput{
		UserID:           account.UserID,
		Session:          session,
		Turn:             turn,
		InboundMessage:   inbound,
		ControllerRunID:  controllerRun.ID,
		AllowedToolKeys:  append([]string{}, plan.AllowedScopes...),
		MessageType:      input.MsgType,
		MessageText:      input.TextContent,
		RequestID:        input.RequestID,
		TraceID:          input.TraceID,
		HistoryQueryPlan: historyQueryPlan,
		ActiveGoal:       plan.Goal,
		ActivePlan:       activePlanContextBlock(plan),
		Complexity:       agent.PlanningComplexity(metadataString(metadataMap(plan.Metadata, "main_agent_plan"), "complexity")),
	})
	if err != nil {
		if isAgentProcessStopError(ctx, err, activeProcess) {
			return s.finishStoppedAgentProcess(ctx, account, inbound, session, turn, plan)
		}
		s.recordControllerTrace(ctx, controllerRun, runResult, "controller_error")
		markedPlan, markErr := s.bindPlanStepsToObservations(ctx, account.UserID, plan, runResult.Context.Observations)
		if markErr != nil {
			err = errors.Join(err, markErr)
		} else {
			plan = markedPlan
		}
		opErr = err
		stopPeriodicProgress()
		return s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, plan, err)
	}
	if activeProcess.stoppedByUser() {
		return s.finishStoppedAgentProcess(ctx, account, inbound, session, turn, plan)
	}
	s.recordControllerTrace(ctx, controllerRun, runResult, "controller_output")
	updatedPlan, err := s.bindPlanStepsToObservations(ctx, account.UserID, plan, runResult.Context.Observations)
	if err != nil {
		opErr = err
		stopPeriodicProgress()
		return s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, plan, err)
	}
	if updatedPlan.ID > 0 {
		plan = updatedPlan
	}
	if !s.processInline && plan.Status == domain.AgentPlanStatusFailed {
		s.sendPlanProgressNotification(ctx, account, session, turn, input, plan, "step_failed", "step_failed")
	} else if !s.processInline && len(runResult.Context.Observations) > 0 {
		s.sendPlanProgressNotification(ctx, account, session, turn, input, plan, "subagent_stage_completed", "subagent_stage_completed")
	}
	reply := sanitizeAgentReportText(runResult.Reply)
	observations := runResult.Context.Observations
	turn = runResult.Turn
	span.SetAttributes(
		attribute.String("llm.provider", runResult.ModelProvider),
		attribute.String("llm.model", runResult.Model),
		attribute.Int("agent.reply_bytes", len([]byte(reply))),
		attribute.Int("agent.observation_count", len(observations)),
	)

	stopPeriodicProgress()
	result, err := s.finishTurnWithReply(ctx, account, inbound, session, turn, input, plan, reply, observations, "succeeded")
	opErr = err
	if err == nil && result.Turn.Status == domain.AgentTurnStatusSucceeded {
		processStatus = domain.AgentTraceEventSucceeded
	}
	return result, err
}

// markInterruptedPlanSteps 在主流程异常中断时收敛 Web 计划步骤状态。
// 已产生 observation 的步骤按实际结果标记；未执行到的步骤标记为 skipped，
// 避免用户在详情页看到已经终止的计划仍停留在 pending。
func (s *AgentConversationService) markInterruptedPlanSteps(ctx context.Context, userID int64, plan domain.AgentPlan, observations []agent.CapabilityObservation, cause error) (domain.AgentPlan, error) {
	if s == nil || s.repository == nil || plan.ID == 0 || len(plan.Steps) == 0 {
		return plan, nil
	}
	now := s.now().UTC()
	errorText := ""
	if cause != nil {
		errorText = truncateError(cause.Error(), 500)
	}
	observationsByCapability := map[string][]agent.CapabilityObservation{}
	for _, observation := range observations {
		key := strings.TrimSpace(observation.Capability)
		if key == "" {
			continue
		}
		observationsByCapability[key] = append(observationsByCapability[key], observation)
	}
	for index, step := range plan.Steps {
		if agentPlanStepTerminal(step.Status) {
			continue
		}
		candidates := observationsByCapability[step.CapabilityKey]
		if len(candidates) > 0 {
			observation := candidates[0]
			observationsByCapability[step.CapabilityKey] = candidates[1:]
			step.Status = domain.AgentPlanStepStatusCompleted
			if strings.EqualFold(observation.Status, "failed") {
				step.Status = domain.AgentPlanStepStatusFailed
				step.ErrorMessage = firstNonEmptyString(observation.Summary, errorText)
			}
			if step.StartedAt == nil {
				startedAt := now
				step.StartedAt = &startedAt
			}
			completedAt := now
			step.CompletedAt = &completedAt
			step.ExecutorRunID = observation.RunID
			step.ObservationRef = observation.ObservationRef
			step.ArtifactRefs = append([]string(nil), observation.ArtifactRefs...)
			step.OutputSummary = firstNonEmptyString(observation.Summary, step.OutputSummary)
		} else {
			if step.Status == domain.AgentPlanStepStatusExecuting {
				step.Status = domain.AgentPlanStepStatusFailed
			} else {
				step.Status = domain.AgentPlanStepStatusSkipped
			}
			completedAt := now
			step.CompletedAt = &completedAt
			step.ErrorMessage = errorText
			step.OutputSummary = firstNonEmptyString(step.OutputSummary, "主流程中断，步骤未执行完成。")
		}
		updatedStep, err := s.repository.UpdateAgentPlanStepStatus(ctx, userID, step)
		if err != nil {
			return domain.AgentPlan{}, err
		}
		plan.Steps[index] = updatedStep
	}
	return plan, nil
}

func agentPlanStepTerminal(status domain.AgentPlanStepStatus) bool {
	switch status {
	case domain.AgentPlanStepStatusCompleted, domain.AgentPlanStepStatusFailed, domain.AgentPlanStepStatusSkipped:
		return true
	default:
		return false
	}
}

// popBestPlanStepObservation 为同一 capability 的多次调用选择最能代表最终执行结果的观测。
// 模型可能先给出错误工具参数，随后根据工具反馈重试成功；计划步骤应绑定成功观测，而不是第一次失败观测。
func popBestPlanStepObservation(observationsByCapability map[string][]agent.CapabilityObservation, capabilityKey string) (agent.CapabilityObservation, bool) {
	candidates := observationsByCapability[capabilityKey]
	if len(candidates) == 0 {
		return agent.CapabilityObservation{}, false
	}
	bestIndex := 0
	bestRank := agentPlanObservationRank(candidates[0])
	for index := 1; index < len(candidates); index++ {
		if rank := agentPlanObservationRank(candidates[index]); rank > bestRank {
			bestIndex = index
			bestRank = rank
		}
	}
	observation := candidates[bestIndex]
	candidates = append(candidates[:bestIndex], candidates[bestIndex+1:]...)
	if len(candidates) == 0 {
		delete(observationsByCapability, capabilityKey)
	} else {
		observationsByCapability[capabilityKey] = candidates
	}
	return observation, true
}

func agentPlanObservationRank(observation agent.CapabilityObservation) int {
	switch strings.ToLower(strings.TrimSpace(observation.Status)) {
	case "succeeded":
		return 4
	case "empty":
		return 3
	case "skipped":
		return 2
	case "failed":
		return 1
	default:
		return 2
	}
}

func agentPlanStepStatusForObservation(observation agent.CapabilityObservation) (domain.AgentPlanStepStatus, error) {
	outcome, err := agent.ToolOutcomeForStatus(observation.Status)
	if err != nil {
		return "", err
	}
	return outcome.PlanStepStatus, nil
}

// convergeUnobservedPlanStep 把终态计划中未被实际执行路径使用的步骤收敛为 skipped。
// Web 详情仍能看到原始计划步骤，但不会出现 completed plan 保留 pending/executing 的不一致状态。
func convergeUnobservedPlanStep(step domain.AgentPlanStep, now time.Time) domain.AgentPlanStep {
	if agentPlanStepTerminal(step.Status) {
		return step
	}
	step.Status = domain.AgentPlanStepStatusSkipped
	if strings.TrimSpace(step.OutputSummary) == "" {
		step.OutputSummary = "模型实际执行路径未使用该计划步骤，终态收敛时已跳过。"
	}
	completedAt := now
	step.CompletedAt = &completedAt
	return step
}

// bindPlanStepsToObservations 将工具观测回填到计划步骤，并汇总计划终态。
func (s *AgentConversationService) bindPlanStepsToObservations(ctx context.Context, userID int64, plan domain.AgentPlan, observations []agent.CapabilityObservation) (domain.AgentPlan, error) {
	if s == nil || s.repository == nil || plan.ID == 0 {
		return plan, nil
	}
	plan.Steps = append([]domain.AgentPlanStep(nil), plan.Steps...)
	now := s.now().UTC()
	observationsByCapability := map[string][]agent.CapabilityObservation{}
	for _, observation := range observations {
		if _, err := agent.ToolOutcomeForStatus(observation.Status); err != nil {
			return domain.AgentPlan{}, err
		}
		key := strings.TrimSpace(observation.Capability)
		if key == "" {
			continue
		}
		observationsByCapability[key] = append(observationsByCapability[key], observation)
	}
	hasFailure := false
	for index := range plan.Steps {
		step := plan.Steps[index]
		observation, ok := popBestPlanStepObservation(observationsByCapability, step.CapabilityKey)
		if !ok {
			continue
		}
		status, err := agentPlanStepStatusForObservation(observation)
		if err != nil {
			return domain.AgentPlan{}, err
		}
		if status == domain.AgentPlanStepStatusPending {
			return domain.AgentPlan{}, domain.NewAppError(domain.ErrorKindConflict, "agent_tool_confirmation_required", "agent tool requires confirmation before the task can complete", "service.agent.bind_steps", false, nil)
		}
		step.Status = status
		if step.Status == domain.AgentPlanStepStatusFailed {
			step.ErrorMessage = observation.Summary
			hasFailure = true
		}
		if step.StartedAt == nil {
			startedAt := now
			step.StartedAt = &startedAt
		}
		completedAt := now
		step.CompletedAt = &completedAt
		step.ExecutorRunID = observation.RunID
		step.ObservationRef = observation.ObservationRef
		step.ArtifactRefs = append([]string(nil), observation.ArtifactRefs...)
		step.OutputSummary = observation.Summary
		plan.Steps[index] = step
	}
	for index := range plan.Steps {
		step := convergeUnobservedPlanStep(plan.Steps[index], now)
		if step.Status == plan.Steps[index].Status {
			continue
		}
		plan.Steps[index] = step
	}
	status := domain.AgentPlanStatusCompleted
	errorMessage := ""
	if hasFailure {
		status = domain.AgentPlanStatusFailed
		errorMessage = "one or more plan steps failed"
	}
	plan.Status = status
	plan.ErrorMessage = errorMessage
	plan.Metadata = cloneApprovalMetadata(plan.Metadata)
	plan.Metadata["execution_completeness"] = buildAgentExecutionCompletenessMetadata(plan, now)
	plan.Metadata["cost_summary"] = buildAgentCostSummaryMetadata(plan, s.relatedScheduledTasksForPlan(ctx, userID, plan.ID), 0, now)
	plan.Metadata["runtime_observability"] = buildAgentRuntimeObservabilityMetadata(plan, metadataMap(plan.Metadata, "admission_policy"), now)
	return plan, nil
}

func (s *AgentConversationService) applyAgentPlanTerminalMetadata(ctx context.Context, userID int64, plan domain.AgentPlan) domain.AgentPlan {
	if s == nil || s.repository == nil || plan.ID == 0 {
		return plan
	}
	now := s.now().UTC()
	plan.Metadata = cloneApprovalMetadata(plan.Metadata)
	plan.Metadata["execution_completeness"] = buildAgentExecutionCompletenessMetadata(plan, now)
	plan.Metadata["cost_summary"] = buildAgentCostSummaryMetadata(plan, s.relatedScheduledTasksForPlan(ctx, userID, plan.ID), 0, now)
	plan.Metadata["handoff"] = buildAgentHandoffMetadata(plan, s.agentNotificationPreference(ctx, userID), now)
	plan.Metadata["runtime_observability"] = buildAgentRuntimeObservabilityMetadata(plan, metadataMap(plan.Metadata, "admission_policy"), now)
	updated, err := s.repository.UpdateAgentPlanMetadata(ctx, userID, plan.ID, plan.Metadata, now)
	if err != nil {
		return plan
	}
	return updated
}

func (s *AgentConversationService) alignControllerRunWithPlan(ctx context.Context, run domain.AgentRun, plan domain.AgentPlan, input ReceiveWeChatWorkAppMessageInput) domain.AgentRun {
	if s == nil || s.repository == nil || run.ID == 0 || plan.ID == 0 {
		return run
	}
	scopes := append([]string(nil), plan.AllowedScopes...)
	if len(scopes) == 0 {
		scopes = capabilityKeysFromPlanSteps(plan.Steps)
	}
	run.CapabilityScope = scopes
	if run.TaskPacket == nil {
		run.TaskPacket = domain.AgentJSON{}
	}
	run.TaskPacket["plan_id"] = plan.ID
	run.TaskPacket["plan_status"] = string(plan.Status)
	run.TaskPacket["plan_allowed_scopes"] = append([]string(nil), scopes...)
	run.TaskPacket["plan_summary"] = safeSummary(plan.Summary, 500)
	run.UpdatedAt = s.now().UTC()
	if updated, err := s.repository.UpdateAgentRun(ctx, run); err == nil {
		run = updated
	}
	_, _ = s.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
		RunID:     run.ID,
		TraceKind: "controller_scope_aligned",
		ModelKey:  run.ModelKey,
		Content: domain.AgentJSON{
			"plan_id":             plan.ID,
			"status":              string(plan.Status),
			"capability_scope":    scopes,
			"confirmation_policy": plan.ConfirmationPolicy,
			"request_id":          input.RequestID,
		},
		RedactionStatus: "redacted",
		TokenEstimate:   estimateTokenCount(plan.Summary),
	})
	return run
}

func capabilityKeysFromPlanSteps(steps []domain.AgentPlanStep) []string {
	keys := make([]string, 0, len(steps))
	seen := map[string]struct{}{}
	for _, step := range steps {
		key := strings.TrimSpace(step.CapabilityKey)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	return keys
}

func (s *AgentConversationService) relatedScheduledTasksForPlan(ctx context.Context, userID int64, planID int64) []domain.AgentScheduledTask {
	if s == nil || s.repository == nil || userID < 1 || planID < 1 {
		return nil
	}
	tasks, err := s.repository.ListAgentScheduledTasks(ctx, domain.AgentScheduledTaskListOptions{UserID: userID, Limit: 100})
	if err != nil {
		return nil
	}
	filtered := make([]domain.AgentScheduledTask, 0)
	for _, task := range tasks {
		if task.PlanID == planID {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

// createPlanForTurn 创建计划并附加准入、权限、预算和 capability 审计信息。
func (s *AgentConversationService) createPlanForTurn(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	controllerRun domain.AgentRun,
	input ReceiveWeChatWorkAppMessageInput,
) (domain.AgentPlan, string, error) {
	if s.planner == nil || s.repository == nil {
		return domain.AgentPlan{}, "", nil
	}
	parentPlan, hasParent, parentStale, err := s.selectDerivedParentPlanForTurn(ctx, account.UserID, session.ID, turn.ID)
	if err != nil {
		return domain.AgentPlan{}, "", err
	}
	planInput := agent.PlanInput{
		UserID:          account.UserID,
		SessionID:       session.ID,
		TurnID:          turn.ID,
		ControllerRunID: controllerRun.ID,
		Goal:            input.TextContent,
	}
	loadProjection := sync.OnceValues(func() (domain.AgentJSON, error) {
		return s.turnPlanningProjection(ctx, account, session, turn, input)
	})
	route, err := s.classifyAgentTaskRoute(ctx, account, session, turn, controllerRun, input, loadProjection)
	if err != nil {
		return domain.AgentPlan{}, "", err
	}
	mainPlan := mainAgentPlanSpecResult{}
	if agentTaskRouteShouldUseLightPlan(route) {
		spec := agentTaskRoutePlanSpec(route, input.TextContent)
		mainPlan = mainAgentPlanSpecResult{
			Spec:      spec,
			Provider:  route.Provider,
			Model:     route.Model,
			Raw:       route.Raw,
			Attempts:  1,
			Validated: true,
		}
	} else if len(route.PlanSpec) > 0 && string(route.PlanSpec) != "null" {
		spec, specErr := parseMainAgentPlanSpecJSON(string(route.PlanSpec))
		if specErr == nil {
			specErr = s.validateMainAgentPlanSpec(spec)
		}
		if specErr != nil {
			failedPlan, createErr := s.createPlanningFailedPlan(ctx, account, session, turn, controllerRun, input, specErr)
			return failedPlan, "", errors.Join(specErr, createErr)
		}
		mainPlan = mainAgentPlanSpecResult{Spec: spec, Provider: route.Provider, Model: route.Model, Raw: string(route.PlanSpec), Attempts: 1, Validated: true}
	} else {
		// 主 Agent 先由模型生成 PlanSpec，避免 service 层继续通过关键词硬编码推断用户意图。
		mainPlan, err = s.buildMainAgentPlanSpec(ctx, account, session, turn, controllerRun, input, loadProjection)
		if err != nil {
			if fallbackSpec, ok := agentTaskRouteFallbackPlanSpec(route, input.TextContent); ok && mainAgentPlannerErrorAllowsRouteFallback(err) {
				mainPlan = mainAgentPlanSpecResult{
					Spec:      fallbackSpec,
					Provider:  route.Provider,
					Model:     route.Model,
					Raw:       route.Raw,
					Attempts:  0,
					Validated: true,
				}
			} else {
				failedPlan, createErr := s.createPlanningFailedPlan(ctx, account, session, turn, controllerRun, input, err)
				if createErr != nil {
					return domain.AgentPlan{}, "", createErr
				}
				return failedPlan, "", err
			}
		}
	}
	// planner 只把模型计划转换为持久化计划和步骤，权限、预算、确认策略仍走后续治理链路。
	output := s.planner.BuildFromSpec(ctx, planInput, mainPlan.Spec)
	output.Plan.Metadata = cloneApprovalMetadata(output.Plan.Metadata)
	// 记录规划模型调用摘要，供 Web 详情页展示主 Agent 的规划来源。
	output.Plan.Metadata["main_agent_planning_call"] = domain.AgentJSON{
		"provider":   mainPlan.Provider,
		"model":      mainPlan.Model,
		"attempts":   mainPlan.Attempts,
		"validated":  mainPlan.Validated,
		"raw_length": len(mainPlan.Raw),
	}
	output.Plan.Metadata["task_route"] = domain.AgentJSON{
		"task_type":               route.TaskType,
		"confidence":              route.Confidence,
		"needs_history_recall":    route.NeedsHistoryRecall,
		"needs_tools":             route.NeedsTools,
		"requires_sub_agent":      route.RequiresSubAgent,
		"estimated_latency_class": route.EstimatedLatencyClass,
		"history_query":           route.HistoryQuery,
		"reason":                  route.Reason,
		"light_plan":              agentTaskRouteShouldUseLightPlan(route),
	}
	plan, err := s.repository.CreateAgentPlan(ctx, output.Plan, output.Steps)
	if err != nil {
		return domain.AgentPlan{}, "", err
	}
	if hasParent {
		plan.Metadata = updateDerivedPlanMetadata(plan, parentPlan, input.TextContent, s.now().UTC(), parentStale)
		if updated, updateErr := s.repository.UpdateAgentPlanMetadata(ctx, account.UserID, plan.ID, plan.Metadata, s.now().UTC()); updateErr == nil {
			plan = updated
		} else {
			return domain.AgentPlan{}, "", updateErr
		}
		s.recordMultiTurnAudit(ctx, account.UserID, session.ID, turn.ID, plan, input, "agent.plan_derived", "created", input.TextContent)
	}
	admission := s.agentTaskAdmissionDecision(ctx, account.UserID, input.Provider, 0)
	plan.Metadata = cloneApprovalMetadata(plan.Metadata)
	plan.Metadata["admission_policy"] = admission.Metadata
	if updated, updateErr := s.repository.UpdateAgentPlanMetadata(ctx, account.UserID, plan.ID, plan.Metadata, s.now().UTC()); updateErr == nil {
		plan = updated
	} else {
		return domain.AgentPlan{}, "", updateErr
	}
	plan, err = s.applyCapabilityPolicyToPlan(ctx, account.UserID, session.ID, turn.ID, plan, input)
	if err != nil {
		return domain.AgentPlan{}, "", err
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "agent.plan_governance_recorded",
		Status:    planBudgetStatus(plan),
		Message:   "agent plan permission and budget governance recorded",
		Metadata: domain.AgentJSON{
			"plan_id":     plan.ID,
			"permission":  metadataMap(plan.Metadata, "permission_governance"),
			"budget":      metadataMap(plan.Metadata, "budget_governance"),
			"quality":     metadataMap(plan.Metadata, "planner_quality"),
			"admission":   metadataMap(plan.Metadata, "admission_policy"),
			"capability":  metadataMap(plan.Metadata, "capability_policy"),
			"next_action": agentProgressNextAction(string(plan.Status), true, plan, nil),
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: s.now().UTC(),
	})
	for _, step := range plan.Steps {
		_, _ = s.repository.CreateAgentCapabilityAuditLog(ctx, domain.AgentCapabilityAuditLog{
			UserID:        account.UserID,
			SessionID:     session.ID,
			TurnID:        turn.ID,
			RunID:         controllerRun.ID,
			PlanID:        plan.ID,
			PlanStepID:    step.ID,
			CapabilityKey: step.CapabilityKey,
			Decision:      plan.PolicyDecision,
			Reason:        plan.PolicyReason,
			InputSummary:  step.InputSummary,
			Status:        "planned",
			Metadata: domain.AgentJSON{
				"capability_scope": step.CapabilityScope,
				"policy":           metadataMap(plan.Metadata, "capability_policy"),
				"request_id":       input.RequestID,
				"trace_id":         input.TraceID,
			},
			CreatedAt: s.now().UTC(),
		})
	}
	if plan.Status != domain.AgentPlanStatusAwaitingApproval {
		return plan, "", nil
	}
	token, err := newURLToken()
	if err != nil {
		return domain.AgentPlan{}, "", err
	}
	now := s.now().UTC()
	planID := plan.ID
	externalAccountID := account.ID
	approvalChannel := strings.TrimSpace(input.Provider)
	if approvalChannel == "" {
		approvalChannel = domain.AgentProviderWeChatWorkApp
	}
	_, err = s.repository.CreateAgentApproval(ctx, domain.AgentApproval{
		PlanID:            &planID,
		UserID:            account.UserID,
		ExternalAccountID: &externalAccountID,
		ApprovalTokenHash: hashSecret(token),
		Channel:           approvalChannel,
		Status:            domain.AgentApprovalStatusPending,
		ExpiresAt:         now.Add(30 * time.Minute),
		RequestID:         input.RequestID,
		TraceID:           input.TraceID,
		Metadata: domain.AgentJSON{
			"plan_summary":        plan.Summary,
			"impact_summary":      plan.ImpactSummary,
			"risk_level":          plan.RiskLevel,
			"confirmation_policy": plan.ConfirmationPolicy,
			"allowed_scopes":      plan.AllowedScopes,
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return domain.AgentPlan{}, "", err
	}
	return plan, token, nil
}

func mainAgentPlannerErrorAllowsRouteFallback(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	for _, blocked := range []string{
		"invalid api key",
		"api key",
		"unauthorized",
		"forbidden",
		"permission denied",
		"401",
		"403",
	} {
		if strings.Contains(lower, blocked) {
			return false
		}
	}
	return true
}

func historyQueryPlanForTurn(plan domain.AgentPlan) agent.PlanHistoryQueryPlan {
	mainPlan := metadataMap(plan.Metadata, "main_agent_plan")
	historyPlan := metadataMap(domain.AgentJSON(mainPlan), "history_query_plan")
	return agent.PlanHistoryQueryPlan{
		Mode:     metadataString(historyPlan, "mode"),
		Query:    metadataString(historyPlan, "query"),
		TimeHint: metadataString(historyPlan, "time_hint"),
		Reason:   metadataString(historyPlan, "reason"),
		Limit:    metadataNumber(historyPlan, "limit"),
	}
}

func activePlanContextBlock(plan domain.AgentPlan) *agent.ContextBlock {
	if plan.ID == 0 {
		return nil
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "plan_id: %d\n", plan.ID)
	fmt.Fprintf(&builder, "status: %s\n", plan.Status)
	if strings.TrimSpace(plan.Goal) != "" {
		builder.WriteString("goal: ")
		builder.WriteString(strings.TrimSpace(plan.Goal))
		builder.WriteString("\n")
	}
	if strings.TrimSpace(plan.Summary) != "" {
		builder.WriteString("summary: ")
		builder.WriteString(strings.TrimSpace(plan.Summary))
		builder.WriteString("\n")
	}
	if len(plan.AllowedScopes) > 0 {
		builder.WriteString("allowed_scopes: ")
		builder.WriteString(strings.Join(plan.AllowedScopes, ", "))
		builder.WriteString("\n")
	}
	mainPlan := metadataMap(plan.Metadata, "main_agent_plan")
	if len(mainPlan) > 0 {
		if needsRecent, ok := mainPlan["needs_recent_context"]; ok {
			fmt.Fprintf(&builder, "needs_recent_context: %v\n", needsRecent)
		}
		if needsHistory, ok := mainPlan["needs_history_recall"]; ok {
			fmt.Fprintf(&builder, "needs_history_recall: %v\n", needsHistory)
		}
	}
	if len(plan.Steps) > 0 {
		builder.WriteString("steps:\n")
		for _, step := range plan.Steps {
			fmt.Fprintf(&builder, "- #%d %s [%s] capability=%s", step.StepOrder, strings.TrimSpace(step.Title), step.Status, step.CapabilityKey)
			if strings.TrimSpace(step.OutputSummary) != "" {
				builder.WriteString(" output=")
				builder.WriteString(strings.TrimSpace(step.OutputSummary))
			}
			if strings.TrimSpace(step.ObservationRef) != "" {
				builder.WriteString(" observation_ref=")
				builder.WriteString(agent.NormalizeCanonicalRef(step.ObservationRef))
			}
			if len(step.ArtifactRefs) > 0 {
				builder.WriteString(" artifact_refs=")
				builder.WriteString(strings.Join(agent.NormalizeCanonicalRefs(step.ArtifactRefs), ","))
			}
			builder.WriteString("\n")
		}
	}
	evidenceRefs := []string{fmt.Sprintf("plan:%d", plan.ID)}
	for _, step := range plan.Steps {
		if step.ID > 0 {
			evidenceRefs = append(evidenceRefs, fmt.Sprintf("plan_step:%d", step.ID))
		}
	}
	return &agent.ContextBlock{
		Name:                   "当前活动计划",
		CapabilityKey:          "agent.plan",
		Content:                strings.TrimSpace(builder.String()),
		ItemCount:              len(plan.Steps),
		GeneratedAt:            plan.UpdatedAt,
		TrustLevel:             "planner",
		Source:                 "active_plan",
		EvidenceRefs:           agent.NormalizeCanonicalRefs(evidenceRefs),
		CanonicalRef:           fmt.Sprintf("plan:%d", plan.ID),
		RetentionReason:        "active_plan",
		FinalAnswerConstraints: compactNonEmptyStrings(agentJSONStringSlice(domain.AgentJSON(mainPlan), "final_answer_constraints")),
	}
}

func planAllowsConversationHistory(plan domain.AgentPlan) bool {
	for _, key := range plan.AllowedScopes {
		if strings.TrimSpace(key) == "conversation.query_history" {
			return true
		}
	}
	return false
}

// createPlanningFailedPlan 在主 Agent 规划阶段失败时保留可审计计划。
// 规划失败发生在普通计划创建之前；如果不补建失败态 plan，Web 端只能看到 turn 失败，
// 无法进入任务详情查看阶段、错误和后续审计信息。
func (s *AgentConversationService) createPlanningFailedPlan(
	ctx context.Context,
	account domain.ExternalAccount,
	session domain.AgentSession,
	turn domain.AgentTurn,
	controllerRun domain.AgentRun,
	input ReceiveWeChatWorkAppMessageInput,
	cause error,
) (domain.AgentPlan, error) {
	if s == nil || s.repository == nil || cause == nil {
		return domain.AgentPlan{}, nil
	}
	now := s.now().UTC()
	failedAt := now
	startedAt := now
	completedAt := now
	errorText := truncateError(cause.Error(), 500)
	plan := domain.AgentPlan{
		UserID:             account.UserID,
		SessionID:          session.ID,
		TurnID:             turn.ID,
		ControllerRunID:    controllerRun.ID,
		Status:             domain.AgentPlanStatusFailed,
		Goal:               input.TextContent,
		Summary:            "主 Agent 规划阶段未完成，错误原因已记录在任务详情中。",
		ImpactSummary:      "未进入工具执行阶段。",
		RiskLevel:          "low",
		ConfirmationPolicy: "auto",
		AllowedScopes:      []string{},
		PolicyDecision:     "allow",
		PolicyReason:       "规划阶段未涉及外部能力调用。",
		FailedAt:           &failedAt,
		ErrorMessage:       errorText,
		Metadata: domain.AgentJSON{
			"failure_stage":       "main_agent_planning",
			"failure_reason":      errorText,
			"provider_message_id": input.ProviderMessageID,
			"request_id":          input.RequestID,
			"trace_id":            input.TraceID,
			"created_from":        "planning_failure",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	step := domain.AgentPlanStep{
		StepOrder:       1,
		Status:          domain.AgentPlanStepStatusFailed,
		CapabilityKey:   "main_agent.plan",
		CapabilityScope: []string{},
		Title:           "主 Agent 理解与规划",
		InputSummary:    "根据用户消息生成结构化执行计划。",
		OutputSummary:   "规划阶段失败，未生成可执行 PlanSpec。",
		ExpectedOutput:  "结构化 PlanSpec。",
		FailureStrategy: "停止本轮任务，并把失败阶段和错误原因反馈给用户。",
		ErrorMessage:    errorText,
		MaxRetries:      mainAgentPlanSpecMaxAttempts,
		RetryCount:      mainAgentPlanSpecMaxAttempts,
		RetryReason:     errorText,
		RetryMetadata: domain.AgentJSON{
			"failure_stage": "main_agent_planning",
		},
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	created, err := s.repository.CreateAgentPlan(ctx, plan, []domain.AgentPlanStep{step})
	if err != nil {
		return domain.AgentPlan{}, err
	}
	created = s.applyAgentPlanTerminalMetadata(ctx, account.UserID, created)
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "agent.plan_planning_failed",
		Status:    "failed",
		Message:   errorText,
		Metadata: domain.AgentJSON{
			"plan_id":             created.ID,
			"failure_stage":       "main_agent_planning",
			"provider_message_id": input.ProviderMessageID,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: now,
	})
	return created, nil
}

func (s *AgentConversationService) applyCapabilityPolicyToPlan(ctx context.Context, userID int64, sessionID int64, turnID int64, plan domain.AgentPlan, input ReceiveWeChatWorkAppMessageInput) (domain.AgentPlan, error) {
	if s == nil || s.repository == nil || plan.ID == 0 {
		return plan, nil
	}
	now := s.now().UTC()
	metadata := buildAgentCapabilityPolicyMetadata(plan, s.agentNotificationPreference(ctx, userID), now)
	plan.Metadata = cloneApprovalMetadata(plan.Metadata)
	plan.Metadata["capability_policy"] = metadata
	updated, err := s.repository.UpdateAgentPlanMetadata(ctx, userID, plan.ID, plan.Metadata, now)
	if err != nil {
		return domain.AgentPlan{}, err
	}
	plan = updated
	status := metadataString(metadataMap(plan.Metadata, "capability_policy"), "status")
	switch status {
	case "reject":
		plan, err = s.repository.UpdateAgentPlanStatus(ctx, userID, plan.ID, domain.AgentPlanStatusRejected, now, "capability policy rejected one or more plan steps")
	case "confirm", "degrade":
		if plan.Status != domain.AgentPlanStatusAwaitingApproval {
			plan, err = s.repository.UpdateAgentPlanStatus(ctx, userID, plan.ID, domain.AgentPlanStatusAwaitingApproval, now, "")
		}
	}
	if err != nil {
		return domain.AgentPlan{}, err
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: sessionID,
		TurnID:    turnID,
		UserID:    userID,
		EventType: "agent.capability_policy_applied",
		Status:    status,
		Message:   "agent capability policy applied to plan",
		Metadata:  metadata,
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: now,
	})
	return plan, nil
}
