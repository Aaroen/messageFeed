package service

import (
	"context"
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
	e.recordSubagentDispatchTraceEvent(ctx, input.UserID, input.SessionID, input.TurnID, input.ControllerRunID, run, input.Capability.Key, runErr)
	if run.ID > 0 {
		_, _ = e.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
			RunID:     run.ID,
			TraceKind: "executor_input",
			Content: domain.AgentJSON{
				"task_packet":      run.TaskPacket,
				"capability_scope": run.CapabilityScope,
			},
			RedactionStatus: "redacted",
			TokenEstimate:   estimateTokenCount(input.Message),
		})
	}

	result, err := e.base.Execute(ctx, input)
	e.recordToolTraceEvent(ctx, agentToolTraceEventInput{
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
		Err:             err,
	})
	if err != nil {
		result.Observation = e.recordExecutorFailure(ctx, run, input.Capability.Key, input.Message, err)
		return result, err
	}
	result.Observation = e.recordExecutorSuccess(ctx, run, input.Capability.Key, input.Message, result.Observation, contextBlocksContent(result.Blocks), len(result.Blocks))
	return result, nil
}

func (e agentRunRecordingExecutor) CallTool(ctx context.Context, input agent.MCPCallToolInput) (agent.MCPCallToolResult, error) {
	startedAt := e.nowUTC()
	run, runErr := e.createExecutorRun(ctx, input.ControllerRunID, input.SessionID, input.TurnID, input.Capability, domain.AgentJSON{
		"message":        safeSummary(input.Message, 500),
		"capability_key": input.Capability.Key,
		"tool_call_id":   input.CallID,
		"tool_arguments": safeSummary(input.RawArguments, 1000),
	})
	e.recordSubagentDispatchTraceEvent(ctx, input.UserID, input.SessionID, input.TurnID, input.ControllerRunID, run, input.Capability.Key, runErr)
	if run.ID > 0 {
		_, _ = e.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
			RunID:     run.ID,
			TraceKind: "tool_input",
			Content: domain.AgentJSON{
				"task_packet":      run.TaskPacket,
				"capability_scope": run.CapabilityScope,
			},
			RedactionStatus: "redacted",
			TokenEstimate:   estimateTokenCount(input.RawArguments),
		})
	}

	result, err := e.base.CallTool(ctx, input)
	e.recordToolTraceEvent(ctx, agentToolTraceEventInput{
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
		Err:             err,
	})
	if err != nil {
		result.Observation = e.recordExecutorFailure(ctx, run, input.Capability.Key, input.RawArguments, err)
		return result, err
	}
	result.Observation = e.recordExecutorSuccess(ctx, run, input.Capability.Key, input.RawArguments, result.Observation, result.TextContent(), 1)
	return result, nil
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
	Err             error
}

func (e agentRunRecordingExecutor) recordSubagentDispatchTraceEvent(ctx context.Context, userID int64, sessionID int64, turnID int64, parentRunID int64, run domain.AgentRun, capabilityKey string, err error) {
	store, ok := any(e.base.repository).(agentTraceEventStore)
	if !ok {
		return
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
	_, _ = store.CreateAgentTraceEvent(ctx, event)
}

func (e agentRunRecordingExecutor) recordToolTraceEvent(ctx context.Context, input agentToolTraceEventInput) {
	store, ok := any(e.base.repository).(agentTraceEventStore)
	if !ok {
		return
	}
	finishedAt := e.nowUTC()
	durationMS := finishedAt.Sub(input.StartedAt).Milliseconds()
	if durationMS < 0 {
		durationMS = 0
	}
	status := agentTraceStatusFromError(input.Err)
	capability := agentTraceLabelValue(input.CapabilityKey)
	tool := agentTraceLabelValue(input.ToolName)
	metrics.AgentToolExecutionsTotal.WithLabelValues(capability, tool, string(status)).Inc()
	metrics.AgentToolExecutionDuration.WithLabelValues(capability, tool, string(status)).Observe(float64(durationMS) / 1000)
	metrics.AgentTraceEventsTotal.WithLabelValues(string(domain.AgentTraceEventToolExecution), string(status)).Inc()
	if durationMS > 0 {
		metrics.AgentTraceEventDuration.WithLabelValues(string(domain.AgentTraceEventToolExecution), string(status)).Observe(float64(durationMS) / 1000)
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
		Status:        status,
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
	if input.Err != nil {
		event.ErrorCode = "capability_execution_failed"
		event.ErrorMessage = input.Err.Error()
	}
	_, _ = store.CreateAgentTraceEvent(ctx, event)
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

func (e agentRunRecordingExecutor) recordExecutorSuccess(ctx context.Context, run domain.AgentRun, capabilityKey string, input string, observation agent.CapabilityObservation, output string, artifactCount int) agent.CapabilityObservation {
	if observation.Capability == "" {
		observation.Capability = capabilityKey
	}
	if e.runManager == nil || run.ID == 0 {
		return observation
	}
	status := observation.Status
	if strings.TrimSpace(status) == "" {
		status = "succeeded"
	}
	observation.Status = status
	observation.RunID = run.ID
	evidenceRefs := capabilityEvidenceRefs(capabilityKey, run)
	if strings.TrimSpace(output) != "" {
		artifact, _ := e.runManager.RecordArtifact(ctx, domain.AgentArtifact{
			RunID:        run.ID,
			ArtifactType: "capability_output",
			ContentRef:   fmt.Sprintf("agent_run:%d:capability_output", run.ID),
			Summary:      safeSummary(output, 1000),
			SourceRefs:   evidenceRefs,
		})
		if artifact.ID > 0 {
			evidenceRefs = append(evidenceRefs, fmt.Sprintf("agent_artifact:%d", artifact.ID), fmt.Sprintf("agent_artifacts/%d", artifact.ID))
		}
	}
	recordedObservation, _ := e.runManager.RecordObservation(ctx, domain.AgentObservation{
		RunID:         run.ID,
		CapabilityKey: capabilityKey,
		InputSummary:  safeSummary(input, 500),
		OutputSummary: safeSummary(observation.Summary, 500),
		Status:        status,
		ArtifactRefs:  evidenceRefs,
	})
	if recordedObservation.ID > 0 {
		observation.ObservationRef = fmt.Sprintf("agent_observations/%d", recordedObservation.ID)
		evidenceRefs = append(evidenceRefs, fmt.Sprintf("agent_observation:%d", recordedObservation.ID), observation.ObservationRef)
	}
	observation.ArtifactRefs = append([]string(nil), evidenceRefs...)
	_, _ = e.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
		RunID:     run.ID,
		TraceKind: "executor_output",
		Content: domain.AgentJSON{
			"observation": domain.AgentJSON{
				"capability": observation.Capability,
				"decision":   observation.Decision,
				"status":     status,
				"summary":    observation.Summary,
				"metadata":   cloneAgentTraceMetadata(observation.Metadata),
			},
			"artifact_count": artifactCount,
		},
		RedactionStatus: "redacted",
		TokenEstimate:   estimateTokenCount(output),
	})
	_, _ = e.runManager.CompleteRun(ctx, run, "observation")
	return observation
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

func (e agentRunRecordingExecutor) recordExecutorFailure(ctx context.Context, run domain.AgentRun, capabilityKey string, input string, err error) agent.CapabilityObservation {
	observation := agent.CapabilityObservation{
		Capability: capabilityKey,
		Decision:   string(agent.PolicyDecisionAllow),
		Status:     "failed",
		Summary:    "capability execution failed",
		RunID:      run.ID,
	}
	if e.runManager == nil || run.ID == 0 {
		return observation
	}
	recordedObservation, _ := e.runManager.RecordObservation(ctx, domain.AgentObservation{
		RunID:         run.ID,
		CapabilityKey: capabilityKey,
		InputSummary:  safeSummary(input, 500),
		OutputSummary: "capability execution failed",
		Status:        "failed",
		Error:         safeSummary(err.Error(), 500),
		ArtifactRefs:  capabilityEvidenceRefs(capabilityKey, run),
	})
	if recordedObservation.ID > 0 {
		observation.ObservationRef = fmt.Sprintf("agent_observations/%d", recordedObservation.ID)
		observation.ArtifactRefs = append(capabilityEvidenceRefs(capabilityKey, run), fmt.Sprintf("agent_observation:%d", recordedObservation.ID), observation.ObservationRef)
	}
	_, _ = e.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
		RunID:     run.ID,
		TraceKind: "executor_error",
		Content: domain.AgentJSON{
			"error": safeSummary(err.Error(), 1000),
		},
		RedactionStatus: "redacted",
	})
	_, _ = e.runManager.FailRun(ctx, run, err)
	return observation
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
