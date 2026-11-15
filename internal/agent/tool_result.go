package agent

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"strings"
)

type ToolOutcome struct {
	Status         string
	RunStatus      domain.AgentRunStatus
	TraceStatus    domain.AgentTraceEventStatus
	PlanStepStatus domain.AgentPlanStepStatus
	IsError        bool
	EvidenceUsable bool
}

func ToolOutcomeForStatus(status string) (ToolOutcome, error) {
	outcome := ToolOutcome{Status: strings.TrimSpace(status)}
	switch outcome.Status {
	case "succeeded":
		outcome.RunStatus = domain.AgentRunStatusSucceeded
		outcome.TraceStatus = domain.AgentTraceEventSucceeded
		outcome.PlanStepStatus = domain.AgentPlanStepStatusCompleted
		outcome.EvidenceUsable = true
	case "empty":
		outcome.RunStatus = domain.AgentRunStatusSucceeded
		outcome.TraceStatus = domain.AgentTraceEventSucceeded
		outcome.PlanStepStatus = domain.AgentPlanStepStatusSkipped
	case "skipped":
		outcome.RunStatus = domain.AgentRunStatusCanceled
		outcome.TraceStatus = domain.AgentTraceEventSkipped
		outcome.PlanStepStatus = domain.AgentPlanStepStatusSkipped
	case "failed", "rejected", "blocked":
		if outcome.Status == "blocked" {
			outcome.Status = "rejected"
		}
		outcome.RunStatus = domain.AgentRunStatusFailed
		outcome.TraceStatus = domain.AgentTraceEventFailed
		outcome.PlanStepStatus = domain.AgentPlanStepStatusFailed
		outcome.IsError = true
	case "requires_confirmation":
		outcome.RunStatus = domain.AgentRunStatusInputRequired
		outcome.TraceStatus = domain.AgentTraceEventSkipped
		outcome.PlanStepStatus = domain.AgentPlanStepStatusPending
	default:
		return ToolOutcome{}, fmt.Errorf("未知工具结果状态 %q", status)
	}
	return outcome, nil
}

func NormalizeToolResult(result MCPCallToolResult, callErr error) (MCPCallToolResult, error) {
	if callErr == nil {
		status := strings.TrimSpace(result.Observation.Status)
		if result.Result.IsError && (status == "empty" || status == "skipped") {
			status = "rejected"
		}
		outcome, err := ToolOutcomeForStatus(status)
		if err != nil {
			callErr = err
		} else if result.Result.IsError && !outcome.IsError {
			callErr = fmt.Errorf("工具结果状态 %q 与 IsError=true 冲突", outcome.Status)
		} else {
			result.Observation.Status = outcome.Status
			result.Result.IsError = outcome.IsError
			return result, nil
		}
	}
	result.Observation.Status = "failed"
	result.Result.IsError = true
	if strings.TrimSpace(result.Observation.Summary) == "" {
		result.Observation.Summary = callErr.Error()
	}
	return result, callErr
}

func (m *RunManager) FinishToolRun(ctx context.Context, run domain.AgentRun, status string, resultRef string, errorMessage string) (domain.AgentRun, error) {
	outcome, err := ToolOutcomeForStatus(status)
	if err != nil {
		return domain.AgentRun{}, err
	}
	if m == nil || m.store == nil || run.ID == 0 {
		return run, nil
	}
	now := m.now().UTC()
	run.Status = outcome.RunStatus
	run.ResultRef = strings.TrimSpace(resultRef)
	run.ErrorMessage = strings.TrimSpace(errorMessage)
	run.CompletedAt = &now
	run.UpdatedAt = now
	return m.store.UpdateAgentRun(ctx, run)
}
