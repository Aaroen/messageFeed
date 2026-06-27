package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/notifier"
	"messagefeed/internal/observability"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// ReceiveWeChatWorkAppMessage 接收企业微信消息，完成入口校验、幂等入库和执行调度。
func (s *AgentConversationService) ReceiveWeChatWorkAppMessage(ctx context.Context, input ReceiveWeChatWorkAppMessageInput) (ReceiveWeChatWorkAppMessageResult, error) {
	startedAt := time.Now()
	if s == nil || s.repository == nil {
		metrics.AgentTurnsTotal.WithLabelValues(domain.AgentProviderWeChatWorkApp, "failed").Inc()
		metrics.AgentTurnDuration.WithLabelValues(domain.AgentProviderWeChatWorkApp, "failed").Observe(time.Since(startedAt).Seconds())
		return ReceiveWeChatWorkAppMessageResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_conversation_unavailable", "agent conversation service is unavailable", "service.agent.receive_wechat_work", true, nil)
	}
	input = normalizeReceiveWeChatWorkInput(input)
	status := "succeeded"
	ctx, span := observability.StartSpan(ctx, "service.agent.receive_wechat_work",
		attribute.String("agent.provider", input.Provider),
		attribute.String("message.type", input.MsgType),
		attribute.String("message.chat_type", input.ChatType),
		attribute.Int("message.text_chars", len([]rune(input.TextContent))),
	)
	var opErr error
	defer func() {
		span.SetAttributes(attribute.String("agent.turn.status", status))
		metrics.AgentTurnsTotal.WithLabelValues(input.Provider, status).Inc()
		metrics.AgentTurnDuration.WithLabelValues(input.Provider, status).Observe(time.Since(startedAt).Seconds())
		observability.EndSpan(span, opErr)
	}()

	if err := validateReceiveWeChatWorkInput(input); err != nil {
		status = "failed"
		opErr = err
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "failed").Inc()
		return ReceiveWeChatWorkAppMessageResult{}, err
	}

	now := s.now().UTC()
	if s.resolver == nil {
		status = "failed"
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "agent_identity_resolver_unavailable", "external account resolver is unavailable", "service.agent.receive_wechat_work", true, nil)
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "failed").Inc()
		return ReceiveWeChatWorkAppMessageResult{}, opErr
	}
	account, err := s.resolver.ResolveExternalAccount(ctx, input.Provider, input.CorpID, input.AgentID, input.ExternalUserID)
	if err != nil {
		if domain.ClassifyError(err) == domain.ErrorKindNotFound {
			status = "binding_required"
			reply := "请先登录 messageFeed，在设置页完成企业微信绑定后再发送消息。"
			sendResult := notifier.WeChatWorkSendResult{}
			sendCount := 0
			if s.shouldSendWeChatWorkReply(input) {
				sendResult, sendCount, _ = s.sendWeChatWorkReply(ctx, input.ExternalUserID, reply)
				metrics.AgentReplyChunksTotal.WithLabelValues(input.Provider, "binding_required").Add(float64(sendCount))
			}
			metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "binding_required").Inc()
			return ReceiveWeChatWorkAppMessageResult{
				Reply:           reply,
				SendResult:      sendResult,
				BindingRequired: true,
			}, nil
		}
		status = "failed"
		opErr = err
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "failed").Inc()
		return ReceiveWeChatWorkAppMessageResult{}, err
	}
	if account.BindingStatus == domain.ExternalAccountBindingStatusDisabled {
		status = "failed"
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "agent_external_account_disabled", "external account binding is disabled", "service.agent.receive_wechat_work", false, nil)
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "failed").Inc()
		return ReceiveWeChatWorkAppMessageResult{}, opErr
	}
	admission := s.agentTaskAdmissionDecision(ctx, account.UserID, "wechat_work", 0)
	if !admission.Allowed {
		status = "throttled"
		reply := "当前 Agent 任务达到用户级运行限制。\n原因：" + admission.Reason + "\n下一步：" + admission.NextAction
		sendResult := notifier.WeChatWorkSendResult{}
		if s.shouldSendWeChatWorkReply(input) {
			sendResult, _, _ = s.sendWeChatWorkReply(ctx, input.ExternalUserID, reply)
		}
		_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
			UserID:    account.UserID,
			EventType: "agent.task_admission_throttled",
			Status:    admission.Status,
			Message:   admission.Reason,
			Metadata:  admission.Metadata,
			RequestID: input.RequestID,
			TraceID:   input.TraceID,
			CreatedAt: now,
		})
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "throttled").Inc()
		return ReceiveWeChatWorkAppMessageResult{ExternalAccount: account, Reply: reply, SendResult: sendResult}, nil
	}

	inbound, created, err := s.repository.CreateInboundMessage(ctx, domain.AgentInboundMessage{
		UserID:            account.UserID,
		ExternalAccountID: account.ID,
		Provider:          input.Provider,
		ProviderMessageID: input.ProviderMessageID,
		CorpID:            input.CorpID,
		AgentID:           input.AgentID,
		ExternalUserID:    input.ExternalUserID,
		ChatID:            input.ChatID,
		ChatType:          input.ChatType,
		MsgType:           input.MsgType,
		TextContent:       input.TextContent,
		Payload: domain.AgentJSON{
			"event_type": input.EventType,
			"event_key":  input.EventKey,
			"raw_xml":    input.RawXML,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		Status:    domain.AgentInboundMessageStatusReceived,
	})
	if err != nil {
		status = "failed"
		opErr = err
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "failed").Inc()
		return ReceiveWeChatWorkAppMessageResult{}, err
	}
	if !created {
		status = "duplicate"
		metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "duplicate").Inc()
		return ReceiveWeChatWorkAppMessageResult{
			ExternalAccount: account,
			InboundMessage:  inbound,
			Duplicate:       true,
		}, nil
	}
	metrics.AgentInboundMessagesTotal.WithLabelValues(input.Provider, input.MsgType, "received").Inc()

	session, err := s.resolveConversationSession(ctx, account, input, now)
	if err != nil {
		status = "failed"
		opErr = err
		return ReceiveWeChatWorkAppMessageResult{}, err
	}
	span.SetAttributes(attribute.Int64("agent.session_id", session.ID))

	turn, err := s.repository.CreateTurn(ctx, domain.AgentTurn{
		SessionID:        session.ID,
		InboundMessageID: inbound.ID,
		UserID:           account.UserID,
		Status:           domain.AgentTurnStatusRunning,
		InputText:        input.TextContent,
		StartedAt:        now,
	})
	if err != nil {
		status = "failed"
		opErr = err
		return ReceiveWeChatWorkAppMessageResult{}, err
	}
	span.SetAttributes(attribute.Int64("agent.turn_id", turn.ID))

	_, _ = s.repository.AppendTranscriptEntry(ctx, domain.AgentTranscriptEntry{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		Role:      domain.AgentTranscriptRoleUser,
		Content:   input.TextContent,
		Metadata:  domain.AgentJSON{"provider_message_id": input.ProviderMessageID},
		CreatedAt: now,
	})

	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "wechat_work.inbound_queued",
		Status:    "queued",
		Message:   "wechat work inbound message queued for turn processing",
		Metadata: domain.AgentJSON{
			"provider_message_id": input.ProviderMessageID,
			"msg_type":            input.MsgType,
			"admission_policy":    admission.Metadata,
		},
		RequestID: input.RequestID,
		TraceID:   input.TraceID,
		CreatedAt: now,
	})

	if handled, handledResult, err := s.handleMultiTurnMessage(ctx, account, inbound, session, turn, input); err != nil {
		status = "failed"
		opErr = err
		return ReceiveWeChatWorkAppMessageResult{}, err
	} else if handled {
		return handledResult, nil
	}

	if handled, handledResult, err := s.handleWeChatButtonCallback(ctx, account, inbound, session, turn, input); err != nil {
		status = "failed"
		opErr = err
		return ReceiveWeChatWorkAppMessageResult{}, err
	} else if handled {
		return handledResult, nil
	}

	result := ReceiveWeChatWorkAppMessageResult{
		ExternalAccount: account,
		InboundMessage:  inbound,
		Session:         session,
		Turn:            turn,
		ProcessingAsync: !s.processInline,
	}
	if !s.processInline {
		reply, sendResult, _ := s.sendWeChatWorkTaskAcceptedFeedback(ctx, account, session, turn, input)
		result.Reply = reply
		result.SendResult = sendResult
	}
	if s.processInline {
		processed, err := s.processTurn(context.WithoutCancel(ctx), account, inbound, session, turn, input)
		if err != nil {
			status = "failed"
			opErr = err
			return processed, err
		}
		return processed, nil
	}

	processCtx := context.WithoutCancel(ctx)
	go func() {
		ctx, cancel := context.WithTimeout(processCtx, s.processTimeout)
		defer cancel()
		_, _ = s.processTurn(ctx, account, inbound, session, turn, input)
	}()

	return result, nil
}

// ReceiveWebAgentTask 接收 Web 任务，复用同一套 Agent turn 闭环。
func (s *AgentConversationService) ReceiveWebAgentTask(ctx context.Context, auth CurrentAuth, input ReceiveWebAgentTaskInput) (ReceiveWebAgentTaskResult, error) {
	if s == nil || s.repository == nil {
		return ReceiveWebAgentTaskResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_conversation_unavailable", "agent conversation service is unavailable", "service.agent.receive_web_task", false, nil)
	}
	if !auth.Authenticated || auth.User.ID < 1 {
		return ReceiveWebAgentTaskResult{}, fmt.Errorf("%w: authenticated user is required", domain.ErrInvalidInput)
	}
	input.Message = strings.TrimSpace(input.Message)
	if input.Message == "" {
		return ReceiveWebAgentTaskResult{}, fmt.Errorf("%w: message is required", domain.ErrInvalidInput)
	}
	channel := normalizeWebAgentChannel(input.Channel)
	now := s.now().UTC()
	admission := s.agentTaskAdmissionDecision(ctx, auth.User.ID, "web", 0)
	if !admission.Allowed {
		_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
			UserID:    auth.User.ID,
			EventType: "agent.task_admission_throttled",
			Status:    admission.Status,
			Message:   admission.Reason,
			Metadata:  admission.Metadata,
			RequestID: strings.TrimSpace(input.RequestID),
			TraceID:   strings.TrimSpace(input.TraceID),
			CreatedAt: now,
		})
		return ReceiveWebAgentTaskResult{}, domain.NewAppError(domain.ErrorKindConflict, "agent_task_throttled", admission.Reason, "service.agent.receive_web_task", false, nil)
	}
	requestID := strings.TrimSpace(input.RequestID)
	if requestID == "" {
		requestID = fmt.Sprintf("web-%d-%d", auth.User.ID, now.UnixNano())
	}
	traceID := strings.TrimSpace(input.TraceID)
	externalUserID := fmt.Sprintf("user:%d", auth.User.ID)

	account, err := s.repository.EnsureExternalAccount(ctx, domain.ExternalAccount{
		UserID:         auth.User.ID,
		Provider:       domain.AgentProviderWeb,
		CorpID:         domain.AgentProviderWeb,
		AgentID:        channel,
		ExternalUserID: externalUserID,
		DisplayName:    strings.TrimSpace(auth.User.DisplayName),
		BindingStatus:  domain.ExternalAccountBindingStatusActive,
		VerifiedAt:     &now,
		LastSeenAt:     &now,
	})
	if err != nil {
		return ReceiveWebAgentTaskResult{}, err
	}

	providerMessageID := fmt.Sprintf("web:%d:%s", auth.User.ID, requestID)
	inbound, created, err := s.repository.CreateInboundMessage(ctx, domain.AgentInboundMessage{
		UserID:            account.UserID,
		ExternalAccountID: account.ID,
		Provider:          domain.AgentProviderWeb,
		ProviderMessageID: providerMessageID,
		CorpID:            domain.AgentProviderWeb,
		AgentID:           channel,
		ExternalUserID:    externalUserID,
		ChatID:            webAgentSessionKey(auth.User.ID, channel),
		ChatType:          channel,
		MsgType:           "text",
		TextContent:       input.Message,
		Payload: domain.AgentJSON{
			"channel":    channel,
			"session_id": input.SessionID,
		},
		RequestID: requestID,
		TraceID:   traceID,
		Status:    domain.AgentInboundMessageStatusReceived,
	})
	if err != nil {
		return ReceiveWebAgentTaskResult{}, err
	}
	if !created {
		return ReceiveWebAgentTaskResult{
			Duplicate: true,
		}, nil
	}

	session, err := s.resolveWebConversationSession(ctx, account, input.SessionID, channel, now)
	if err != nil {
		return ReceiveWebAgentTaskResult{}, err
	}
	turn, err := s.repository.CreateTurn(ctx, domain.AgentTurn{
		SessionID:        session.ID,
		InboundMessageID: inbound.ID,
		UserID:           account.UserID,
		Status:           domain.AgentTurnStatusRunning,
		InputText:        input.Message,
		StartedAt:        now,
	})
	if err != nil {
		return ReceiveWebAgentTaskResult{}, err
	}
	_, _ = s.repository.AppendTranscriptEntry(ctx, domain.AgentTranscriptEntry{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		Role:      domain.AgentTranscriptRoleUser,
		Content:   input.Message,
		Metadata: domain.AgentJSON{
			"provider_message_id": providerMessageID,
			"channel":             channel,
			"admission_policy":    admission.Metadata,
		},
		CreatedAt: now,
	})
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: session.ID,
		TurnID:    turn.ID,
		UserID:    account.UserID,
		EventType: "web.agent_task_created",
		Status:    "queued",
		Message:   "web agent task created for turn processing",
		Metadata: domain.AgentJSON{
			"provider_message_id": providerMessageID,
			"channel":             channel,
		},
		RequestID: requestID,
		TraceID:   traceID,
		CreatedAt: now,
	})

	if handled, handledResult, err := s.handleMultiTurnMessage(ctx, account, inbound, session, turn, ReceiveWeChatWorkAppMessageInput{
		Provider:          domain.AgentProviderWeb,
		ProviderMessageID: providerMessageID,
		CorpID:            domain.AgentProviderWeb,
		AgentID:           channel,
		ExternalUserID:    externalUserID,
		ChatID:            webAgentSessionKey(auth.User.ID, channel),
		ChatType:          channel,
		MsgType:           "text",
		TextContent:       input.Message,
		RequestID:         requestID,
		TraceID:           traceID,
	}); err != nil {
		return ReceiveWebAgentTaskResult{}, err
	} else if handled {
		progressURL := ""
		if handledResult.Plan.ID > 0 {
			progressURL = s.agentPlanURL(handledResult.Plan.ID)
		}
		return ReceiveWebAgentTaskResult{
			Session:     agentSessionResponse(handledResult.Session, domain.AgentSessionStats{}, false),
			Turn:        agentTurnResponse(handledResult.Turn),
			Plan:        agentPlanResponse(handledResult.Plan, true),
			Reply:       handledResult.Reply,
			ProgressURL: progressURL,
		}, nil
	}

	processed, err := s.processTurn(context.WithoutCancel(ctx), account, inbound, session, turn, ReceiveWeChatWorkAppMessageInput{
		Provider:          domain.AgentProviderWeb,
		ProviderMessageID: providerMessageID,
		CorpID:            domain.AgentProviderWeb,
		AgentID:           channel,
		ExternalUserID:    externalUserID,
		ChatID:            webAgentSessionKey(auth.User.ID, channel),
		ChatType:          channel,
		MsgType:           "text",
		TextContent:       input.Message,
		RequestID:         requestID,
		TraceID:           traceID,
	})
	if err != nil {
		return ReceiveWebAgentTaskResult{}, err
	}
	plan := processed.Plan
	progressURL := ""
	if plan.ID > 0 {
		progressURL = s.agentPlanURL(plan.ID)
	}
	if err := s.sendWebAgentTaskFinalReport(ctx, account, inbound, session, processed.Turn, plan, processed.Reply, ReceiveWeChatWorkAppMessageInput{
		Provider:          domain.AgentProviderWeb,
		ProviderMessageID: providerMessageID,
		CorpID:            domain.AgentProviderWeb,
		AgentID:           channel,
		ExternalUserID:    externalUserID,
		ChatID:            webAgentSessionKey(auth.User.ID, channel),
		ChatType:          channel,
		MsgType:           "text",
		TextContent:       input.Message,
		RequestID:         requestID,
		TraceID:           traceID,
	}); err != nil {
		return ReceiveWebAgentTaskResult{}, err
	}
	return ReceiveWebAgentTaskResult{
		Session:     agentSessionResponse(processed.Session, domain.AgentSessionStats{}, false),
		Turn:        agentTurnResponse(processed.Turn),
		Plan:        agentPlanResponse(plan, true),
		Reply:       processed.Reply,
		ProgressURL: progressURL,
	}, nil
}
