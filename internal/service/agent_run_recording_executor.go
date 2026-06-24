package service

import (
	"context"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

type agentRunRecordingExecutor struct {
	base       agentP0CapabilityExecutor
	runManager *agent.RunManager
	now        func() time.Time
}

func (e agentRunRecordingExecutor) Execute(ctx context.Context, input agent.CapabilityExecuteInput) (agent.CapabilityExecuteResult, error) {
	run, _ := e.createExecutorRun(ctx, input.ControllerRunID, input.SessionID, input.TurnID, input.Capability, domain.AgentJSON{
		"message":        safeSummary(input.Message, 500),
		"capability_key": input.Capability.Key,
	})
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
	if err != nil {
		e.recordExecutorFailure(ctx, run, input.Capability.Key, input.Message, err)
		return result, err
	}
	e.recordExecutorSuccess(ctx, run, input.Capability.Key, input.Message, result.Observation, contextBlocksContent(result.Blocks), len(result.Blocks))
	return result, nil
}

func (e agentRunRecordingExecutor) ExecuteTool(ctx context.Context, input agent.ToolExecuteInput) (agent.ToolExecuteResult, error) {
	run, _ := e.createExecutorRun(ctx, input.ControllerRunID, input.SessionID, input.TurnID, input.Capability, domain.AgentJSON{
		"message":        safeSummary(input.Message, 500),
		"capability_key": input.Capability.Key,
		"tool_call_id":   input.ToolCallID,
		"tool_arguments": safeSummary(input.RawArguments, 1000),
	})
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

	result, err := e.base.ExecuteTool(ctx, input)
	if err != nil {
		e.recordExecutorFailure(ctx, run, input.Capability.Key, input.RawArguments, err)
		return result, err
	}
	e.recordExecutorSuccess(ctx, run, input.Capability.Key, input.RawArguments, result.Observation, result.Content, 1)
	return result, nil
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

func (e agentRunRecordingExecutor) recordExecutorSuccess(ctx context.Context, run domain.AgentRun, capabilityKey string, input string, observation agent.CapabilityObservation, output string, artifactCount int) {
	if e.runManager == nil || run.ID == 0 {
		return
	}
	status := observation.Status
	if strings.TrimSpace(status) == "" {
		status = "succeeded"
	}
	artifactRefs := []string{}
	if strings.TrimSpace(output) != "" {
		artifact, _ := e.runManager.RecordArtifact(ctx, domain.AgentArtifact{
			RunID:        run.ID,
			ArtifactType: "capability_output",
			ContentRef:   fmt.Sprintf("agent_run:%d:capability_output", run.ID),
			Summary:      safeSummary(output, 1000),
			SourceRefs:   []string{capabilityKey},
		})
		if artifact.ID > 0 {
			artifactRefs = append(artifactRefs, fmt.Sprintf("agent_artifacts/%d", artifact.ID))
		}
	}
	_, _ = e.runManager.RecordObservation(ctx, domain.AgentObservation{
		RunID:         run.ID,
		CapabilityKey: capabilityKey,
		InputSummary:  safeSummary(input, 500),
		OutputSummary: safeSummary(observation.Summary, 500),
		Status:        status,
		ArtifactRefs:  artifactRefs,
	})
	_, _ = e.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
		RunID:     run.ID,
		TraceKind: "executor_output",
		Content: domain.AgentJSON{
			"observation": domain.AgentJSON{
				"capability": observation.Capability,
				"decision":   observation.Decision,
				"status":     status,
				"summary":    observation.Summary,
			},
			"artifact_count": artifactCount,
		},
		RedactionStatus: "redacted",
		TokenEstimate:   estimateTokenCount(output),
	})
	_, _ = e.runManager.CompleteRun(ctx, run, "observation")
}

func (e agentRunRecordingExecutor) recordExecutorFailure(ctx context.Context, run domain.AgentRun, capabilityKey string, input string, err error) {
	if e.runManager == nil || run.ID == 0 {
		return
	}
	_, _ = e.runManager.RecordObservation(ctx, domain.AgentObservation{
		RunID:         run.ID,
		CapabilityKey: capabilityKey,
		InputSummary:  safeSummary(input, 500),
		OutputSummary: "capability execution failed",
		Status:        "failed",
		Error:         safeSummary(err.Error(), 500),
	})
	_, _ = e.runManager.SaveContextTrace(ctx, agent.SaveContextTraceInput{
		RunID:     run.ID,
		TraceKind: "executor_error",
		Content: domain.AgentJSON{
			"error": safeSummary(err.Error(), 1000),
		},
		RedactionStatus: "redacted",
	})
	_, _ = e.runManager.FailRun(ctx, run, err)
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
