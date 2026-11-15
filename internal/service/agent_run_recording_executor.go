package service

import (
	"context"
	"errors"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
	"strings"
	"time"
)

type agentRunRecordingExecutor struct {
	base       agentP0CapabilityExecutor
	runManager *agent.RunManager
	now        func() time.Time
}

func (e agentRunRecordingExecutor) Execute(ctx context.Context, input agent.CapabilityExecuteInput) (agent.CapabilityExecuteResult, error) {
	startedAt := e.nowUTC()
	run, runErr := e.createExecutorRun(ctx, input.ControllerRunID, input.SessionID, input.TurnID, input.Capability, domain.AgentJSON{
		"message":        safeSummary(input.Message, 500),
		"capability_key": input.Capability.Key,
	})
	dispatchErr := e.recordSubagentDispatchTraceEvent(ctx, input.UserID, input.SessionID, input.TurnID, input.ControllerRunID, run, input.Capability.Key, runErr)
	if err := errors.Join(runErr, dispatchErr); err != nil {
		return agent.CapabilityExecuteResult{}, e.failExecutorRun(ctx, run, err)
	}
	if run.ID > 0 {
		_, err := e.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
			RunID:     run.ID,
			TraceKind: "executor_input",
			Content: domain.AgentJSON{
				"task_packet":      run.TaskPacket,
				"capability_scope": run.CapabilityScope,
			},
			RedactionStatus: "redacted",
			TokenEstimate:   estimateTokenCount(input.Message),
		})
		if err != nil {
			return agent.CapabilityExecuteResult{}, e.failExecutorRun(ctx, run, fmt.Errorf("记录工具输入上下文: %w", err))
		}
	}

	result, callErr := e.base.Execute(ctx, input)
	normalized, executionErr := agent.NormalizeToolResult(agent.MCPCallToolResult{Observation: result.Observation}, callErr)
	result.Observation = normalized.Observation
	var recordErr error
	result.Observation, recordErr = e.recordExecutorResult(ctx, run, input.Capability.Key, input.Message, result.Observation, contextBlocksContent(result.Blocks), executionErr)
	traceErr := e.recordToolTraceEvent(ctx, agentToolTraceEventInput{
		UserID:          input.UserID,
		SessionID:       input.SessionID,
		TurnID:          input.TurnID,
		ControllerRunID: input.ControllerRunID,
		Run:             run,
		CapabilityKey:   input.Capability.Key,
		ToolName:        input.Capability.Key,
		InputSummary:    input.Message,
		OutputSummary:   result.Observation.Summary,
		Metadata:        result.Observation.Metadata,
		StartedAt:       startedAt,
		Status:          result.Observation.Status,
		Err:             errors.Join(executionErr, recordErr),
	})
	if err := errors.Join(recordErr, traceErr); err != nil {
		result.Observation.Status = "failed"
		return result, e.failExecutorRun(ctx, run, errors.Join(executionErr, err))
	}
	return result, executionErr
}

func (e agentRunRecordingExecutor) CallTool(ctx context.Context, input agent.MCPCallToolInput) (agent.MCPCallToolResult, error) {
	startedAt := e.nowUTC()
	run, runErr := e.createExecutorRun(ctx, input.ControllerRunID, input.SessionID, input.TurnID, input.Capability, domain.AgentJSON{
		"message":        safeSummary(input.Message, 500),
		"capability_key": input.Capability.Key,
		"tool_call_id":   input.CallID,
		"tool_arguments": safeSummary(input.RawArguments, 1000),
	})
	dispatchErr := e.recordSubagentDispatchTraceEvent(ctx, input.UserID, input.SessionID, input.TurnID, input.ControllerRunID, run, input.Capability.Key, runErr)
	if err := errors.Join(runErr, dispatchErr); err != nil {
		return agent.MCPCallToolResult{}, e.failExecutorRun(ctx, run, err)
	}
	if run.ID > 0 {
		_, err := e.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
			RunID:     run.ID,
			TraceKind: "tool_input",
			Content: domain.AgentJSON{
				"task_packet":      run.TaskPacket,
				"capability_scope": run.CapabilityScope,
			},
			RedactionStatus: "redacted",
			TokenEstimate:   estimateTokenCount(input.RawArguments),
		})
		if err != nil {
			return agent.MCPCallToolResult{}, e.failExecutorRun(ctx, run, fmt.Errorf("记录工具输入上下文: %w", err))
		}
	}

	result, callErr := e.base.CallTool(ctx, input)
	result, executionErr := agent.NormalizeToolResult(result, callErr)
	var recordErr error
	result.Observation, recordErr = e.recordExecutorResult(ctx, run, input.Capability.Key, input.RawArguments, result.Observation, result.TextContent(), executionErr)
	traceErr := e.recordToolTraceEvent(ctx, agentToolTraceEventInput{
		UserID:          input.UserID,
		SessionID:       input.SessionID,
		TurnID:          input.TurnID,
		ControllerRunID: input.ControllerRunID,
		Run:             run,
		CapabilityKey:   input.Capability.Key,
		ToolName:        firstNonEmptyString(input.Name, input.Tool.Name, input.Capability.Key),
		InputSummary:    input.RawArguments,
		OutputSummary:   result.TextContent(),
		Metadata:        result.Observation.Metadata,
		StartedAt:       startedAt,
		RequestID:       input.RequestID,
		TraceID:         input.TraceID,
		Status:          result.Observation.Status,
		Err:             errors.Join(executionErr, recordErr),
	})
	if err := errors.Join(recordErr, traceErr); err != nil {
		result.Observation.Status = "failed"
		result.Result.IsError = true
		return result, e.failExecutorRun(ctx, run, errors.Join(executionErr, err))
	}
	return result, executionErr
}

type agentToolTraceEventInput struct {
	UserID          int64
	SessionID       int64
	TurnID          int64
	ControllerRunID int64
	Run             domain.AgentRun
	CapabilityKey   string
	ToolName        string
	InputSummary    string
	OutputSummary   string
	Metadata        domain.AgentJSON
	StartedAt       time.Time
	RequestID       string
	TraceID         string
	Status          string
	Err             error
}

func (e agentRunRecordingExecutor) recordSubagentDispatchTraceEvent(ctx context.Context, userID int64, sessionID int64, turnID int64, parentRunID int64, run domain.AgentRun, capabilityKey string, err error) error {
	store, ok := any(e.base.repository).(agentTraceEventStore)
	if !ok {
		return nil
	}
	status := domain.AgentTraceEventSucceeded
	if err != nil {
		status = domain.AgentTraceEventFailed
	} else if run.ID == 0 {
		status = domain.AgentTraceEventSkipped
	}
	metrics.AgentSubagentDispatchesTotal.WithLabelValues(agentTraceLabelValue(capabilityKey), string(status)).Inc()
	now := e.nowUTC()
	event := domain.AgentTraceEvent{
		RequestID:     observability.RequestID(ctx),
		TraceID:       observability.TraceID(ctx),
		SpanID:        observability.SpanID(ctx),
		UserID:        userID,
		SessionID:     sessionID,
		TurnID:        turnID,
		RunID:         run.ID,
		ParentRunID:   parentRunID,
		EventKind:     domain.AgentTraceEventSubagentDispatch,
		EventName:     "executor_run_create",
		Status:        status,
		StartedAt:     now,
		FinishedAt:    &now,
		ModelKey:      run.ModelKey,
		CapabilityKey: capabilityKey,
		Metadata: domain.AgentJSON{
			"capability_scope": run.CapabilityScope,
		},
		CreatedAt: now,
	}
	if err != nil {
		event.ErrorCode = "executor_run_create_failed"
		event.ErrorMessage = err.Error()
	}
	metrics.AgentTraceEventsTotal.WithLabelValues(string(event.EventKind), string(event.Status)).Inc()
	if _, writeErr := store.CreateAgentTraceEvent(ctx, event); writeErr != nil {
		return fmt.Errorf("记录工具分派事件: %w", writeErr)
	}
	return nil
}

func (e agentRunRecordingExecutor) recordToolTraceEvent(ctx context.Context, input agentToolTraceEventInput) error {
	status := input.Status
	if input.Err != nil {
		status = "failed"
	}
	outcome, err := agent.ToolOutcomeForStatus(status)
	if err != nil {
		return err
	}
	store, ok := any(e.base.repository).(agentTraceEventStore)
	if !ok {
		return nil
	}
	finishedAt := e.nowUTC()
	durationMS := finishedAt.Sub(input.StartedAt).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}
	traceStatus := outcome.TraceStatus
	capability := agentTraceLabelValue(input.CapabilityKey)
	tool := agentTraceLabelValue(input.ToolName)
	metrics.AgentToolExecutionsTotal.WithLabelValues(capability, tool, string(traceStatus)).Inc()
	metrics.AgentToolExecutionDuration.WithLabelValues(capability, tool, string(traceStatus)).Observe(float64(durationMS) / 1000)
	metrics.AgentTraceEventsTotal.WithLabelValues(string(domain.AgentTraceEventToolExecution), string(traceStatus)).Inc()
	if durationMS > 0 {
		metrics.AgentTraceEventDuration.WithLabelValues(string(domain.AgentTraceEventToolExecution), string(traceStatus)).Observe(float64(durationMS) / 1000)
	}
	event := domain.AgentTraceEvent{
		RequestID:     firstNonEmptyString(input.RequestID, observability.RequestID(ctx)),
		TraceID:       firstNonEmptyString(input.TraceID, observability.TraceID(ctx)),
		SpanID:        observability.SpanID(ctx),
		UserID:        input.UserID,
		SessionID:     input.SessionID,
		TurnID:        input.TurnID,
		RunID:         input.Run.ID,
		ParentRunID:   input.ControllerRunID,
		EventKind:     domain.AgentTraceEventToolExecution,
		EventName:     "capability_execute",
		Status:        traceStatus,
		StartedAt:     input.StartedAt,
		FinishedAt:    &finishedAt,
		DurationMS:    durationMS,
		ModelKey:      input.Run.ModelKey,
		CapabilityKey: input.CapabilityKey,
		ToolName:      input.ToolName,
		InputSummary:  safeSummary(input.InputSummary, 1000),
		OutputSummary: safeSummary(input.OutputSummary, 1000),
		Metadata:      cloneAgentTraceMetadata(input.Metadata),
		CreatedAt:     finishedAt,
	}
	if event.Metadata == nil {
		event.Metadata = domain.AgentJSON{}
	}
	event.Metadata["observation_status"] = outcome.Status
	event.Metadata["evidence_usable"] = outcome.EvidenceUsable
	if input.Err != nil {
		event.ErrorCode = "capability_execution_failed"
		event.ErrorMessage = input.Err.Error()
	} else if outcome.IsError {
		event.ErrorCode = "capability_result_" + outcome.Status
		event.ErrorMessage = safeSummary(input.OutputSummary, 1000)
	}
	if _, err := store.CreateAgentTraceEvent(ctx, event); err != nil {
		return fmt.Errorf("记录工具执行事件: %w", err)
	}
	return nil
}

func (e agentRunRecordingExecutor) nowUTC() time.Time {
	now := e.now
	if now == nil {
		now = time.Now
	}
	return now().UTC()
}

func agentTraceLabelValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func (e agentRunRecordingExecutor) createExecutorRun(ctx context.Context, parentRunID int64, sessionID int64, turnID int64, capability agent.Capability, task domain.AgentJSON) (domain.AgentRun, error) {
	if e.runManager == nil || parentRunID == 0 {
		return domain.AgentRun{}, nil
	}
	return e.runManager.CreateExecutorRun(ctx, agent.CreateRunInput{
		ParentRunID:     parentRunID,
		SessionID:       sessionID,
		TurnID:          turnID,
		TaskPacket:      task,
		CapabilityScope: []string{capability.Key},
		ModelKey:        "service-bound-capability",
		ContextBudget: domain.AgentJSON{
			"mode":             "p0_read_only",
			"max_tool_calls":   1,
			"max_output_chars": 4000,
		},
	})
}

func (e agentRunRecordingExecutor) recordExecutorResult(ctx context.Context, run domain.AgentRun, capabilityKey string, input string, observation agent.CapabilityObservation, output string, executionErr error) (agent.CapabilityObservation, error) {
	if observation.Capability == "" {
		observation.Capability = capabilityKey
	}
	outcome, err := agent.ToolOutcomeForStatus(observation.Status)
	if err != nil {
		return observation, err
	}
	observation.Status = outcome.Status
	if e.runManager == nil || run.ID == 0 {
		return observation, nil
	}
	observation.RunID = run.ID
	evidenceRefs := append(capabilityEvidenceRefs(capabilityKey, run), observation.ArtifactRefs...)
	artifactCount := 0
	if outcome.EvidenceUsable && strings.TrimSpace(output) != "" {
		artifact, err := e.runManager.RecordArtifact(ctx, domain.AgentArtifact{
			RunID:        run.ID,
			ArtifactType: "capability_output",
			ContentRef:   fmt.Sprintf("agent_run:%d:capability_output", run.ID),
			Content:      output,
			Summary:      safeSummary(output, 1000),
			SourceRefs:   evidenceRefs,
		})
		if err != nil {
			return observation, fmt.Errorf("记录工具结果正文: %w", err)
		}
		if artifact.ID > 0 {
			evidenceRefs = append(evidenceRefs, fmt.Sprintf("agent_artifact:%d", artifact.ID), fmt.Sprintf("agent_artifacts/%d", artifact.ID))
			artifactCount = 1
		}
	}
	errorMessage := ""
	if executionErr != nil {
		errorMessage = executionErr.Error()
	} else if outcome.IsError {
		errorMessage = observation.Summary
	}
	recordedObservation, err := e.runManager.RecordObservation(ctx, domain.AgentObservation{
		RunID:         run.ID,
		CapabilityKey: capabilityKey,
		InputSummary:  safeSummary(input, 500),
		OutputSummary: safeSummary(observation.Summary, 500),
		Status:        outcome.Status,
		Error:         safeSummary(errorMessage, 500),
		ArtifactRefs:  evidenceRefs,
	})
	if err != nil {
		return observation, fmt.Errorf("记录工具观察结果: %w", err)
	}
	if recordedObservation.ID > 0 {
		observation.ObservationRef = fmt.Sprintf("agent_observations/%d", recordedObservation.ID)
		evidenceRefs = append(evidenceRefs, fmt.Sprintf("agent_observation:%d", recordedObservation.ID), observation.ObservationRef)
	}
	observation.ArtifactRefs = append([]string(nil), evidenceRefs...)
	traceKind := "executor_output"
	if executionErr != nil {
		traceKind = "executor_error"
	}
	traceContent := domain.AgentJSON{
		"observation": domain.AgentJSON{
			"capability": observation.Capability,
			"decision":   observation.Decision,
			"status":     outcome.Status,
			"summary":    observation.Summary,
			"metadata":   cloneAgentTraceMetadata(observation.Metadata),
		},
		"artifact_count": artifactCount,
		"error":          safeSummary(errorMessage, 1000),
	}
	if !outcome.EvidenceUsable && output != "" {
		traceContent["diagnostic_output"] = output
	}
	_, err = e.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
		RunID:           run.ID,
		TraceKind:       traceKind,
		Content:         traceContent,
		RedactionStatus: "redacted",
		TokenEstimate:   estimateTokenCount(output),
	})
	if err != nil {
		return observation, fmt.Errorf("记录工具输出上下文: %w", err)
	}
	if _, err := e.runManager.FinishToolRun(ctx, run, outcome.Status, observation.ObservationRef, errorMessage); err != nil {
		return observation, fmt.Errorf("提交工具执行状态: %w", err)
	}
	return observation, nil
}

func cloneAgentTraceMetadata(input domain.AgentJSON) domain.AgentJSON {
	if input == nil {
		return nil
	}
	output := make(domain.AgentJSON, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func (e agentRunRecordingExecutor) failExecutorRun(ctx context.Context, run domain.AgentRun, executionErr error) error {
	if e.runManager == nil || run.ID == 0 {
		return executionErr
	}
	_, err := e.runManager.FailRun(ctx, run, executionErr)
	if err != nil {
		return errors.Join(executionErr, fmt.Errorf("提交工具失败状态: %w", err))
	}
	return executionErr
}

func capabilityEvidenceRefs(capabilityKey string, run domain.AgentRun) []string {
	refs := []string{}
	if strings.TrimSpace(capabilityKey) != "" {
		refs = append(refs, "capability:"+strings.TrimSpace(capabilityKey))
	}
	if run.ID > 0 {
		refs = append(refs, fmt.Sprintf("agent_run:%d", run.ID))
	}
	if run.TurnID > 0 {
		refs = append(refs, fmt.Sprintf("agent_turn:%d", run.TurnID))
	}
	return refs
}

func contextBlocksContent(blocks []agent.ContextBlock) string {
	var builder strings.Builder
	for _, block := range blocks {
		if strings.TrimSpace(block.Content) == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(block.Name)
		builder.WriteString("\n")
		builder.WriteString(block.Content)
	}
	return builder.String()
}

func safeSummary(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func estimateTokenCount(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	return (len([]rune(value)) + 3) / 4
}
