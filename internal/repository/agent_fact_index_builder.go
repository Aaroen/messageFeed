package repository

import (
	"encoding/json"
	"fmt"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

type agentFactIndexBuilder struct {
	now func() time.Time
}

func newAgentFactIndexBuilder(now func() time.Time) agentFactIndexBuilder {
	if now == nil {
		now = time.Now
	}
	return agentFactIndexBuilder{now: now}
}

func (b agentFactIndexBuilder) BuildTranscript(entry domain.AgentTranscriptEntry) (domain.AgentFactArchiveIndex, bool) {
	if entry.ID == 0 || entry.UserID == 0 {
		return domain.AgentFactArchiveIndex{}, false
	}
	classification := classifyTranscriptMemory(entry.Content)
	return domain.AgentFactArchiveIndex{
		CanonicalRef:    fmt.Sprintf("transcript:%d", entry.ID),
		FactType:        domain.AgentFactTypeTranscript,
		FactID:          entry.ID,
		UserID:          entry.UserID,
		SessionID:       entry.SessionID,
		TurnID:          entry.TurnID,
		MemoryKind:      classification.Kind,
		Topics:          []string{string(entry.Role), string(classification.Kind)},
		Keywords:        transcriptIndexKeywords(entry.Content),
		Entities:        transcriptIndexKeywords(entry.Content),
		SummaryForIndex: safeTextPrefix(entry.Content, 320),
		ContextualText:  entry.Content,
		Importance:      transcriptImportanceForKind(classification.Kind),
		Confidence:      confidenceForClassification(classification),
		SourceRefs:      []string{fmt.Sprintf("transcript:%d", entry.ID), fmt.Sprintf("turn:%d", entry.TurnID), fmt.Sprintf("session:%d", entry.SessionID)},
		RelationRefs:    []string{fmt.Sprintf("turn:%d", entry.TurnID), fmt.Sprintf("session:%d", entry.SessionID)},
		IndexStatus:     domain.AgentFactIndexStatusReady,
		RiskLevel:       inferMemoryRisk(entry.Content, classification.Kind),
		Metadata: domain.AgentJSON{
			"role":                    string(entry.Role),
			"classification_strategy": "rule_fallback",
			"memory_kind_reason":      classification.Reason,
			"indexer_version":         "fact_indexer_v1",
		},
		CreatedAt: entry.CreatedAt,
		UpdatedAt: b.now().UTC(),
	}, true
}

func (b agentFactIndexBuilder) BuildObservation(scope agentFactRunScope, observation domain.AgentObservation) (domain.AgentFactArchiveIndex, bool) {
	if observation.ID == 0 || scope.UserID == 0 {
		return domain.AgentFactArchiveIndex{}, false
	}
	content := strings.TrimSpace(strings.Join([]string{observation.InputSummary, observation.OutputSummary, observation.Error}, "\n"))
	if content == "" {
		content = observation.Status
	}
	classification := classifyTranscriptMemory(content)
	return domain.AgentFactArchiveIndex{
		CanonicalRef:    fmt.Sprintf("observation:%d", observation.ID),
		FactType:        domain.AgentFactTypeObservation,
		FactID:          observation.ID,
		UserID:          scope.UserID,
		SessionID:       scope.SessionID,
		TurnID:          scope.TurnID,
		MemoryKind:      classification.Kind,
		Topics:          compactNonEmptyStrings([]string{"observation", observation.CapabilityKey, observation.Status}),
		Keywords:        transcriptIndexKeywords(content + " " + observation.CapabilityKey),
		Entities:        transcriptIndexKeywords(content),
		SummaryForIndex: safeTextPrefix(content, 320),
		ContextualText:  content,
		Importance:      factImportanceForKind(classification.Kind, 45),
		Confidence:      confidenceForClassification(classification),
		SourceRefs:      compactNonEmptyStrings(append([]string{fmt.Sprintf("observation:%d", observation.ID), fmt.Sprintf("run:%d", observation.RunID)}, observation.ArtifactRefs...)),
		RelationRefs:    compactNonEmptyStrings(append([]string{fmt.Sprintf("run:%d", observation.RunID), fmt.Sprintf("turn:%d", scope.TurnID)}, observation.ArtifactRefs...)),
		IndexStatus:     domain.AgentFactIndexStatusReady,
		RiskLevel:       inferMemoryRisk(content, classification.Kind),
		Metadata: domain.AgentJSON{
			"capability_key":  observation.CapabilityKey,
			"status":          observation.Status,
			"indexer_version": "fact_indexer_v1",
		},
		CreatedAt: observation.CreatedAt,
		UpdatedAt: b.now().UTC(),
	}, true
}

func (b agentFactIndexBuilder) BuildArtifact(scope agentFactRunScope, artifact domain.AgentArtifact) (domain.AgentFactArchiveIndex, bool) {
	if artifact.ID == 0 || scope.UserID == 0 {
		return domain.AgentFactArchiveIndex{}, false
	}
	content := strings.TrimSpace(strings.Join([]string{artifact.Summary, artifact.ContentRef}, "\n"))
	classification := classifyTranscriptMemory(content)
	return domain.AgentFactArchiveIndex{
		CanonicalRef:    fmt.Sprintf("artifact:%d", artifact.ID),
		FactType:        domain.AgentFactTypeArtifact,
		FactID:          artifact.ID,
		UserID:          scope.UserID,
		SessionID:       scope.SessionID,
		TurnID:          scope.TurnID,
		MemoryKind:      classification.Kind,
		Topics:          compactNonEmptyStrings([]string{"artifact", artifact.ArtifactType}),
		Keywords:        transcriptIndexKeywords(content),
		Entities:        transcriptIndexKeywords(content),
		SummaryForIndex: safeTextPrefix(content, 320),
		ContextualText:  content,
		Importance:      factImportanceForKind(classification.Kind, 50),
		Confidence:      confidenceForClassification(classification),
		SourceRefs:      compactNonEmptyStrings(append([]string{fmt.Sprintf("artifact:%d", artifact.ID), fmt.Sprintf("run:%d", artifact.RunID)}, artifact.SourceRefs...)),
		RelationRefs:    compactNonEmptyStrings(append([]string{fmt.Sprintf("run:%d", artifact.RunID), fmt.Sprintf("turn:%d", scope.TurnID)}, artifact.SourceRefs...)),
		IndexStatus:     domain.AgentFactIndexStatusReady,
		RiskLevel:       inferMemoryRisk(content, classification.Kind),
		Metadata: domain.AgentJSON{
			"artifact_type":   artifact.ArtifactType,
			"content_hash":    artifact.ContentHash,
			"indexer_version": "fact_indexer_v1",
		},
		CreatedAt: artifact.CreatedAt,
		UpdatedAt: b.now().UTC(),
	}, true
}

func (b agentFactIndexBuilder) BuildPlan(plan domain.AgentPlan) (domain.AgentFactArchiveIndex, bool) {
	if plan.ID == 0 || plan.UserID == 0 {
		return domain.AgentFactArchiveIndex{}, false
	}
	content := strings.TrimSpace(strings.Join([]string{plan.Goal, plan.Summary, plan.ImpactSummary, plan.ErrorMessage}, "\n"))
	classification := classifyTranscriptMemory(content)
	return domain.AgentFactArchiveIndex{
		CanonicalRef:    fmt.Sprintf("plan:%d", plan.ID),
		FactType:        domain.AgentFactTypePlan,
		FactID:          plan.ID,
		UserID:          plan.UserID,
		SessionID:       plan.SessionID,
		TurnID:          plan.TurnID,
		MemoryKind:      classification.Kind,
		Topics:          compactNonEmptyStrings([]string{"plan", string(plan.Status), plan.RiskLevel}),
		Keywords:        transcriptIndexKeywords(content),
		Entities:        transcriptIndexKeywords(content),
		SummaryForIndex: safeTextPrefix(content, 320),
		ContextualText:  content,
		Importance:      factImportanceForKind(classification.Kind, 60),
		Confidence:      confidenceForClassification(classification),
		SourceRefs:      []string{fmt.Sprintf("plan:%d", plan.ID), fmt.Sprintf("turn:%d", plan.TurnID)},
		RelationRefs:    []string{fmt.Sprintf("turn:%d", plan.TurnID), fmt.Sprintf("run:%d", plan.ControllerRunID)},
		IndexStatus:     domain.AgentFactIndexStatusReady,
		RiskLevel:       domain.AgentMemoryRiskLevel(plan.RiskLevel),
		Metadata: domain.AgentJSON{
			"status":              string(plan.Status),
			"confirmation_policy": plan.ConfirmationPolicy,
			"allowed_scopes":      plan.AllowedScopes,
			"indexer_version":     "fact_indexer_v1",
		},
		CreatedAt: plan.CreatedAt,
		UpdatedAt: b.now().UTC(),
	}, true
}

func (b agentFactIndexBuilder) BuildPlanStep(scope agentFactRunScope, step domain.AgentPlanStep) (domain.AgentFactArchiveIndex, bool) {
	if step.ID == 0 || scope.UserID == 0 {
		return domain.AgentFactArchiveIndex{}, false
	}
	content := strings.TrimSpace(strings.Join([]string{step.Title, step.InputSummary, step.OutputSummary, step.ExpectedOutput, step.ErrorMessage}, "\n"))
	classification := classifyTranscriptMemory(content)
	return domain.AgentFactArchiveIndex{
		CanonicalRef:    fmt.Sprintf("plan_step:%d", step.ID),
		FactType:        domain.AgentFactTypePlanStep,
		FactID:          step.ID,
		UserID:          scope.UserID,
		SessionID:       scope.SessionID,
		TurnID:          scope.TurnID,
		MemoryKind:      classification.Kind,
		Topics:          compactNonEmptyStrings([]string{"plan_step", step.CapabilityKey, string(step.Status)}),
		Keywords:        transcriptIndexKeywords(content + " " + step.CapabilityKey),
		Entities:        transcriptIndexKeywords(content),
		SummaryForIndex: safeTextPrefix(content, 320),
		ContextualText:  content,
		Importance:      factImportanceForKind(classification.Kind, 55),
		Confidence:      confidenceForClassification(classification),
		SourceRefs:      compactNonEmptyStrings(append([]string{fmt.Sprintf("plan_step:%d", step.ID), fmt.Sprintf("plan:%d", step.PlanID), step.ObservationRef}, step.ArtifactRefs...)),
		RelationRefs:    compactNonEmptyStrings(append([]string{fmt.Sprintf("plan:%d", step.PlanID), fmt.Sprintf("turn:%d", scope.TurnID), step.ObservationRef}, step.ArtifactRefs...)),
		IndexStatus:     domain.AgentFactIndexStatusReady,
		RiskLevel:       inferMemoryRisk(content, classification.Kind),
		Metadata: domain.AgentJSON{
			"status":          string(step.Status),
			"capability_key":  step.CapabilityKey,
			"step_order":      step.StepOrder,
			"indexer_version": "fact_indexer_v1",
		},
		CreatedAt: step.CreatedAt,
		UpdatedAt: b.now().UTC(),
	}, true
}

func (b agentFactIndexBuilder) BuildRunContextTrace(scope agentFactRunScope, trace domain.AgentRunContextTrace) (domain.AgentFactArchiveIndex, bool) {
	if trace.ID == 0 || scope.UserID == 0 {
		return domain.AgentFactArchiveIndex{}, false
	}
	contentBytes, _ := json.Marshal(trace.Content)
	content := strings.TrimSpace(string(contentBytes))
	if content == "" || content == "{}" {
		content = strings.TrimSpace(trace.TraceKind + " " + trace.PromptVersion + " " + trace.ModelKey)
	}
	classification := classifyTranscriptMemory(content)
	return domain.AgentFactArchiveIndex{
		CanonicalRef:    fmt.Sprintf("run_trace:%d", trace.ID),
		FactType:        domain.AgentFactTypeRunTrace,
		FactID:          trace.ID,
		UserID:          scope.UserID,
		SessionID:       scope.SessionID,
		TurnID:          scope.TurnID,
		MemoryKind:      classification.Kind,
		Topics:          compactNonEmptyStrings([]string{"run_trace", trace.TraceKind, trace.PromptVersion, trace.ModelKey}),
		Keywords:        transcriptIndexKeywords(content),
		Entities:        transcriptIndexKeywords(content),
		SummaryForIndex: safeTextPrefix(content, 320),
		ContextualText:  content,
		Importance:      factImportanceForKind(classification.Kind, 40),
		Confidence:      confidenceForClassification(classification),
		SourceRefs:      []string{fmt.Sprintf("run_trace:%d", trace.ID), fmt.Sprintf("run:%d", trace.RunID), fmt.Sprintf("turn:%d", scope.TurnID)},
		RelationRefs:    []string{fmt.Sprintf("run:%d", trace.RunID), fmt.Sprintf("turn:%d", scope.TurnID), fmt.Sprintf("session:%d", scope.SessionID)},
		IndexStatus:     domain.AgentFactIndexStatusReady,
		RiskLevel:       inferMemoryRisk(content, classification.Kind),
		Metadata: domain.AgentJSON{
			"trace_kind":       trace.TraceKind,
			"prompt_version":   trace.PromptVersion,
			"model_key":        trace.ModelKey,
			"content_hash":     trace.ContentHash,
			"redaction_status": trace.RedactionStatus,
			"indexer_version":  "fact_indexer_v1",
		},
		CreatedAt: trace.CreatedAt,
		UpdatedAt: b.now().UTC(),
	}, true
}
