package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

const DefaultPromptVersion = "agent-controller-executor-p0"

type RunStore interface {
	CreateAgentRun(ctx context.Context, run domain.AgentRun) (domain.AgentRun, error)
	UpdateAgentRun(ctx context.Context, run domain.AgentRun) (domain.AgentRun, error)
	CreateAgentRunContextTrace(ctx context.Context, trace domain.AgentRunContextTrace) (domain.AgentRunContextTrace, error)
	CreateAgentObservation(ctx context.Context, observation domain.AgentObservation) (domain.AgentObservation, error)
	CreateAgentArtifact(ctx context.Context, artifact domain.AgentArtifact) (domain.AgentArtifact, error)
}

type RunManager struct {
	store RunStore
	now   func() time.Time
}

type RunManagerOptions struct {
	Store RunStore
	Now   func() time.Time
}

func NewRunManager(options RunManagerOptions) *RunManager {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &RunManager{store: options.Store, now: now}
}

type CreateRunInput struct {
	ParentRunID     int64
	SessionID       int64
	TurnID          int64
	Role            domain.AgentRunRole
	TaskPacket      domain.AgentJSON
	CapabilityScope []string
	ModelKey        string
	ContextBudget   domain.AgentJSON
	TraceID         string
}

type SaveContextTraceInput struct {
	RunID           int64
	TraceKind       string
	PromptVersion   string
	ModelKey        string
	Content         domain.AgentJSON
	RedactionStatus string
	TokenEstimate   int
}

func (m *RunManager) CreateControllerRun(ctx context.Context, input CreateRunInput) (domain.AgentRun, error) {
	input.Role = domain.AgentRunRoleController
	return m.createRun(ctx, input)
}

func (m *RunManager) CreateExecutorRun(ctx context.Context, input CreateRunInput) (domain.AgentRun, error) {
	input.Role = domain.AgentRunRoleExecutor
	return m.createRun(ctx, input)
}

func (m *RunManager) CompleteRun(ctx context.Context, run domain.AgentRun, resultRef string) (domain.AgentRun, error) {
	if m == nil || m.store == nil || run.ID == 0 {
		return run, nil
	}
	now := m.now().UTC()
	run.Status = domain.AgentRunStatusSucceeded
	run.ResultRef = strings.TrimSpace(resultRef)
	run.CompletedAt = &now
	run.UpdatedAt = now
	return m.store.UpdateAgentRun(ctx, run)
}

func (m *RunManager) FailRun(ctx context.Context, run domain.AgentRun, err error) (domain.AgentRun, error) {
	if m == nil || m.store == nil || run.ID == 0 {
		return run, nil
	}
	now := m.now().UTC()
	run.Status = domain.AgentRunStatusFailed
	if err != nil {
		run.ErrorMessage = err.Error()
	}
	run.CompletedAt = &now
	run.UpdatedAt = now
	return m.store.UpdateAgentRun(ctx, run)
}

func (m *RunManager) SaveContextTrace(ctx context.Context, input SaveContextTraceInput) (domain.AgentRunContextTrace, error) {
	if m == nil || m.store == nil || input.RunID == 0 {
		return domain.AgentRunContextTrace{}, nil
	}
	input.TraceKind = strings.TrimSpace(input.TraceKind)
	if input.TraceKind == "" {
		input.TraceKind = "context"
	}
	if strings.TrimSpace(input.PromptVersion) == "" {
		input.PromptVersion = DefaultPromptVersion
	}
	if input.Content == nil {
		input.Content = domain.AgentJSON{}
	}
	if strings.TrimSpace(input.RedactionStatus) == "" {
		input.RedactionStatus = "redacted"
	}
	trace := domain.AgentRunContextTrace{
		RunID:           input.RunID,
		TraceKind:       input.TraceKind,
		PromptVersion:   input.PromptVersion,
		ModelKey:        strings.TrimSpace(input.ModelKey),
		Content:         input.Content,
		ContentHash:     contentHash(input.Content),
		RedactionStatus: input.RedactionStatus,
		TokenEstimate:   input.TokenEstimate,
		CreatedAt:       m.now().UTC(),
	}
	return m.store.CreateAgentRunContextTrace(ctx, trace)
}

func (m *RunManager) RecordObservation(ctx context.Context, observation domain.AgentObservation) (domain.AgentObservation, error) {
	if m == nil || m.store == nil || observation.RunID == 0 {
		return observation, nil
	}
	if observation.CreatedAt.IsZero() {
		observation.CreatedAt = m.now().UTC()
	}
	if observation.ArtifactRefs == nil {
		observation.ArtifactRefs = []string{}
	}
	return m.store.CreateAgentObservation(ctx, observation)
}

func (m *RunManager) RecordArtifact(ctx context.Context, artifact domain.AgentArtifact) (domain.AgentArtifact, error) {
	if m == nil || m.store == nil || artifact.RunID == 0 {
		return artifact, nil
	}
	if artifact.CreatedAt.IsZero() {
		artifact.CreatedAt = m.now().UTC()
	}
	if artifact.SourceRefs == nil {
		artifact.SourceRefs = []string{}
	}
	if artifact.ContentHash == "" {
		artifact.ContentHash = textHash(artifact.ContentRef + "\n" + artifact.Summary)
	}
	return m.store.CreateAgentArtifact(ctx, artifact)
}

func (m *RunManager) createRun(ctx context.Context, input CreateRunInput) (domain.AgentRun, error) {
	if m == nil || m.store == nil {
		return domain.AgentRun{}, nil
	}
	now := m.now().UTC()
	run := domain.AgentRun{
		ParentRunID:     input.ParentRunID,
		SessionID:       input.SessionID,
		TurnID:          input.TurnID,
		Role:            input.Role,
		Status:          domain.AgentRunStatusRunning,
		TaskPacket:      input.TaskPacket,
		CapabilityScope: append([]string(nil), input.CapabilityScope...),
		ModelKey:        strings.TrimSpace(input.ModelKey),
		ContextBudget:   input.ContextBudget,
		TraceID:         strings.TrimSpace(input.TraceID),
		StartedAt:       now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return m.store.CreateAgentRun(ctx, run)
}

func contentHash(content domain.AgentJSON) string {
	payload, err := json.Marshal(content)
	if err != nil {
		return ""
	}
	return textHash(string(payload))
}

func textHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
