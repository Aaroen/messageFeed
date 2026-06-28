package service

import (
	"context"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/notifier"
	"messagefeed/internal/observability"
	"strings"
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
	ctx, cancelProcess := context.WithCancel(ctx)
	ctx = withAgentLLMUserID(ctx, account.UserID)
	activeProcess := s.registerAgentProcess(turn.ID, cancelProcess)
	defer cancelProcess()
	defer s.unregisterAgentProcess(activeProcess)

	lock := s.sessionLock(session.ID)
	lock.Lock()
	defer lock.Unlock()

	ctx, span := observability.StartSpan(ctx, "service.agent.process_turn",
		attribute.Int64("agent.session_id", session.ID),
		attribute.Int64("agent.turn_id", turn.ID),
		attribute.Int64("auth.user_id", account.UserID),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s.turnRunner == nil {
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "agent_runner_unavailable", "agent turn runner is unavailable", "service.agent.process_turn", true, nil)
		result := s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, domain.AgentPlan{}, opErr)
		return result, nil
	}
	controllerRun, err := s.createControllerRun(ctx, account, inbound, session, turn, input)
	if err != nil {
		opErr = err
		result := s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, domain.AgentPlan{}, err)
		return result, nil
	}
	plan, approvalToken, err := s.createPlanForTurn(ctx, account, session, turn, controllerRun, input)
	if err != nil {
		opErr = err
		_, _ = s.runManager.FailRun(ctx, controllerRun, err)
		result := s.failTurnWithFeedback(ctx, account, inbound, session, turn, input, plan, err)
		return result, nil
	}
	s.bindAgentProcessPlan(turn.ID, plan.ID)
	controllerRun = s.alignControllerRunWithPlan(ctx, controllerRun, plan, input)
	if plan.Status == domain.AgentPlanStatusApproved {
		executingPlan, _ := s.repository.UpdateAgentPlanStatus(ctx, account.UserID, plan.ID, domain.AgentPlanStatusExecuting, s.now().UTC(), "")
		if executingPlan.ID > 0 {
			plan = executingPlan
		}
	}
	if plan.Status == domain.AgentPlanStatusRejected {
		reply := s.generateAgentWeChatFeedbackText(ctx, agentWeChatFeedbackRequest{
			Stage:       "rejected",
			UserMessage: input.TextContent,
			Plan:        plan,
			ErrorText:   planCapabilityPolicySummary(plan),
			ProgressURL: s.agentPlanURLIfAvailable(plan.ID),
		})
		_, _ = s.runManager.CompleteRun(ctx, controllerRun, "plan_rejected_by_capability_policy")
		result, err := s.finishTurnWithReply(ctx, account, inbound, session, turn, input, reply, nil, "rejected")
		result.Plan = plan
		return result, err
	}
	if !s.processInline {
		s.sendPlanStartedFeedback(ctx, account, session, turn, input, plan)
	}
	if plan.Status == domain.AgentPlanStatusAwaitingApproval {
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
		result, err := s.finishTurnWithReply(ctx, account, inbound, session, turn, input, reply, nil, "awaiting_approval")
		result.Plan = plan
		return result, err
	}
	runResult, err := s.turnRunner.Run(ctx, agent.TurnRunInput{
		UserID:          account.UserID,
		Session:         session,
		Turn:            turn,
		InboundMessage:  inbound,
		ControllerRunID: controllerRun.ID,
		AllowedToolKeys: append([]string{}, plan.AllowedScopes...),
		MessageType:     input.MsgType,
		MessageText:     input.TextContent,
		RequestID:       input.RequestID,
		TraceID:         input.TraceID,
	})
	if err != nil {
		if isAgentProcessStopError(ctx, err, activeProcess) {
			return s.finishStoppedAgentProcess(ctx, account, inbound, session, turn, plan), nil
		}
		s.recordControllerTrace(ctx, controllerRun, runResult, "controller_error")
		if markedPlan, markErr := s.markInterruptedPlanSteps(ctx, account.UserID, plan, runResult.Context.Observations, err); markErr == nil && markedPlan.ID > 0 {
			plan = markedPlan
		}
		failedPlan, _ := s.repository.UpdateAgentPlanStatus(ctx, account.UserID, plan.ID, domain.AgentPlanStatusFailed, s.now().UTC(), err.Error())
		if failedPlan.ID > 0 {
			plan = s.applyAgentPlanTerminalMetadata(ctx, account.UserID, failedPlan)
		}
		if !s.processInline {
			s.sendPlanProgressNotification(ctx, account, session, turn, input, plan, "failed", "failed")
		}
		_, _ = s.runManager.FailRun(ctx, controllerRun, err)
		opErr = err
		result := s.sendTurnFailureFeedback(ctx, account, inbound, session, turn, runResult.Turn, input, plan, err)
		result.Plan = plan
		return result, nil
	}
	if activeProcess.stoppedByUser() {
		return s.finishStoppedAgentProcess(ctx, account, inbound, session, turn, plan), nil
	}
	s.recordControllerTrace(ctx, controllerRun, runResult, "controller_output")
	updatedPlan, err := s.bindPlanStepsToObservations(ctx, account.UserID, plan, runResult.Context.Observations)
	if err != nil {
		failedPlan, _ := s.repository.UpdateAgentPlanStatus(ctx, account.UserID, plan.ID, domain.AgentPlanStatusFailed, s.now().UTC(), err.Error())
		if failedPlan.ID > 0 {
			plan = s.applyAgentPlanTerminalMetadata(ctx, account.UserID, failedPlan)
		}
		if !s.processInline {
			s.sendPlanProgressNotification(ctx, account, session, turn, input, plan, "failed", "failed")
		}
		_, _ = s.runManager.FailRun(ctx, controllerRun, err)
		opErr = err
		result := s.sendTurnFailureFeedback(ctx, account, inbound, session, turn, runResult.Turn, input, plan, err)
		result.Plan = plan
		return result, nil
	}
	if updatedPlan.ID > 0 {
		plan = updatedPlan
	}
	if !s.processInline && plan.Status == domain.AgentPlanStatusFailed {
		s.sendPlanProgressNotification(ctx, account, session, turn, input, plan, "step_failed", "step_failed")
	} else if !s.processInline && len(runResult.Context.Observations) > 0 {
		s.sendPlanProgressNotification(ctx, account, session, turn, input, plan, "subagent_stage_completed", "subagent_stage_completed")
	}
	_, _ = s.runManager.CompleteRun(ctx, controllerRun, "turn_output")
	reply := runResult.Reply
	if !s.processInline {
		reply = s.agentTurnCompletionReply(plan, reply)
	}
	reply = sanitizeAgentReportText(reply)
	observations := runResult.Context.Observations
	turn = runResult.Turn
	span.SetAttributes(
		attribute.String("llm.provider", runResult.ModelProvider),
		attribute.String("llm.model", runResult.Model),
		attribute.Int("agent.reply_bytes", len([]byte(reply))),
		attribute.Int("agent.observation_count", len(observations)),
	)

	sendResult := notifier.WeChatWorkSendResult{}
	sendCount := 0
	finalDelivery := agentWeChatFinalReportDeliveryResult{}
	if s.shouldSendWeChatWorkNotification(ctx, account.UserID, input, "final") {
		finalDelivery, err = s.sendWeChatWorkFinalReportDelivery(ctx, input.ExternalUserID, plan, reply, string(plan.Status))
		sendResult = finalDelivery.SendResult
		sendCount = finalDelivery.SendCount
		if err != nil {
			opErr = err
			metrics.AgentReplyBytes.WithLabelValues(input.Provider, "failed").Observe(float64(len([]byte(reply))))
			metrics.AgentReplyChunksTotal.WithLabelValues(input.Provider, "failed").Add(float64(sendCount))
			_, _ = s.repository.UpdateInboundMessageStatus(ctx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusFailed, s.now().UTC())
			_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
				SessionID: session.ID,
				TurnID:    turn.ID,
				UserID:    account.UserID,
				EventType: "wechat_work.reply_failed",
				Status:    "failed",
				Message:   err.Error(),
				Metadata: domain.AgentJSON{
					"provider_message_id": input.ProviderMessageID,
					"send_count":          sendCount,
					"message_type":        finalDelivery.DeliveryMode,
					"template_status":     finalDelivery.TemplateStatus,
					"text_status":         finalDelivery.TextStatus,
					"template_error":      finalDelivery.TemplateError,
					"text_error":          finalDelivery.TextError,
					"progress_url":        finalDelivery.ProgressURL,
				},
				RequestID: input.RequestID,
				TraceID:   input.TraceID,
				CreatedAt: s.now().UTC(),
			})
			return ReceiveWeChatWorkAppMessageResult{Turn: turn, Plan: plan}, err
		}
	}
	metrics.AgentReplyBytes.WithLabelValues(input.Provider, "succeeded").Observe(float64(len([]byte(reply))))
	metrics.AgentReplyChunksTotal.WithLabelValues(input.Provider, "succeeded").Add(float64(sendCount))

	finishedAt := s.now().UTC()
	inbound, _ = s.repository.UpdateInboundMessageStatus(ctx, account.UserID, inbound.ID, domain.AgentInboundMessageStatusSucceeded, finishedAt)
	replyEventType := "agent.turn_completed"
	replyEventMessage := "agent turn completed"
	if sendCount > 0 {
		replyEventType = "wechat_work.reply_sent"
		replyEventMessage = "wechat work reply sent"
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: replyEventType,
		Status:    "succeeded",
		Message:   replyEventMessage,
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"wechat_msgid":        sendResult.MessageID,
			"invalid_user":        sendResult.InvalidUser,
			"send_count":          sendCount,
			"observations":        agent.ObservationMetadata(observations),
			"message_type":        finalDelivery.DeliveryMode,
			"template_status":     finalDelivery.TemplateStatus,
			"text_status":         finalDelivery.TextStatus,
			"template_error":      finalDelivery.TemplateError,
			"text_error":          finalDelivery.TextError,
			"progress_url":        finalDelivery.ProgressURL,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: finishedAt,
	})

	return ReceiveWeChatWorkAppMessageResult{
		ExternalAccount: account,
		InboundMessage:  inbound,
		Session:         session,
		Turn:            turn,
		Plan:            plan,
		Reply:           reply,
		SendResult:      sendResult,
	}, nil
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

// bindPlanStepsToObservations 将工具观测回填到计划步骤，并汇总计划终态。
func (s *AgentConversationService) bindPlanStepsToObservations(ctx context.Context, userID int64, plan domain.AgentPlan, observations []agent.CapabilityObservation) (domain.AgentPlan, error) {
	if s == nil || s.repository == nil || plan.ID == 0 {
		return plan, nil
	}
	now := s.now().UTC()
	observationsByCapability := map[string][]agent.CapabilityObservation{}
	for _, observation := range observations {
		key := strings.TrimSpace(observation.Capability)
		if key == "" {
			continue
		}
		observationsByCapability[key] = append(observationsByCapability[key], observation)
	}
	hasFailure := false
	for _, step := range plan.Steps {
		candidates := observationsByCapability[step.CapabilityKey]
		if len(candidates) == 0 {
			continue
		}
		observation := candidates[0]
		observationsByCapability[step.CapabilityKey] = candidates[1:]
		step.Status = domain.AgentPlanStepStatusCompleted
		if strings.EqualFold(observation.Status, "failed") {
			step.Status = domain.AgentPlanStepStatusFailed
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
		if _, err := s.repository.UpdateAgentPlanStepStatus(ctx, userID, step); err != nil {
			return domain.AgentPlan{}, err
		}
	}
	status := domain.AgentPlanStatusCompleted
	errorMessage := ""
	if hasFailure {
		status = domain.AgentPlanStatusFailed
		errorMessage = "one or more plan steps failed"
	}
	plans, err := s.repository.ListAgentPlans(ctx, userID, plan.SessionID, 0, 20)
	if err == nil {
		for _, latest := range plans {
			if latest.ID == plan.ID && planStoppedByUser(latest) {
				return latest, nil
			}
		}
	}
	updated, err := s.repository.UpdateAgentPlanStatus(ctx, userID, plan.ID, status, now, errorMessage)
	if err != nil {
		return domain.AgentPlan{}, err
	}
	updated.Metadata = cloneApprovalMetadata(updated.Metadata)
	updated.Metadata["result_quality"] = buildAgentResultQualityMetadata(updated, now)
	updated.Metadata["cost_summary"] = buildAgentCostSummaryMetadata(updated, s.relatedScheduledTasksForPlan(ctx, userID, updated.ID), 0, now)
	updated.Metadata["deployment_acceptance"] = buildAgentDeploymentAcceptanceMetadata(updated, now)
	updated.Metadata["handoff"] = buildAgentHandoffMetadata(updated, s.agentNotificationPreference(ctx, userID), now)
	updated.Metadata["runtime_observability"] = buildAgentRuntimeObservabilityMetadata(updated, metadataMap(updated.Metadata, "admission_policy"), now)
	return s.repository.UpdateAgentPlanMetadata(ctx, userID, updated.ID, updated.Metadata, now)
}

func (s *AgentConversationService) applyAgentPlanTerminalMetadata(ctx context.Context, userID int64, plan domain.AgentPlan) domain.AgentPlan {
	if s == nil || s.repository == nil || plan.ID == 0 {
		return plan
	}
	now := s.now().UTC()
	plan.Metadata = cloneApprovalMetadata(plan.Metadata)
	plan.Metadata["result_quality"] = buildAgentResultQualityMetadata(plan, now)
	plan.Metadata["cost_summary"] = buildAgentCostSummaryMetadata(plan, s.relatedScheduledTasksForPlan(ctx, userID, plan.ID), 0, now)
	plan.Metadata["deployment_acceptance"] = buildAgentDeploymentAcceptanceMetadata(plan, now)
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
	// 主 Agent 先由模型生成 PlanSpec，避免 service 层继续通过关键词硬编码推断用户意图。
	mainPlan, err := s.buildMainAgentPlanSpec(ctx, account, session, turn, controllerRun, input)
	if err != nil {
		failedPlan, createErr := s.createPlanningFailedPlan(ctx, account, session, turn, controllerRun, input, err)
		if createErr != nil {
			return domain.AgentPlan{}, "", createErr
		}
		return failedPlan, "", err
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
