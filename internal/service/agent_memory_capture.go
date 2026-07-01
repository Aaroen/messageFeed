package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"strings"
	"time"
)

type agentMemoryTopicChunkStore interface {
	CreateAgentMemoryTopic(ctx context.Context, topic domain.AgentMemoryTopic) (domain.AgentMemoryTopic, error)
	CreateAgentMemoryChunk(ctx context.Context, chunk domain.AgentMemoryChunk) (domain.AgentMemoryChunk, error)
}

type agentMemoryCaptureClassification struct {
	Kind       domain.AgentMemoryKind
	Terms      []string
	Importance int
	Confidence float64
	RiskLevel  domain.AgentMemoryRiskLevel
	Reason     string
}

func (s *AgentConversationService) captureMemoryCandidateFromTranscript(ctx context.Context, entry domain.AgentTranscriptEntry) {
	if s == nil || s.repository == nil || entry.ID == 0 || entry.UserID == 0 || entry.Role != domain.AgentTranscriptRoleUser {
		return
	}
	classification := classifyAgentMemoryCandidate(entry.Content)
	if !agentMemoryKindShouldCapture(classification.Kind) {
		return
	}
	if s.memoryBlockAlreadyExists(ctx, entry.UserID, entry.Content) {
		return
	}
	now := s.now().UTC()
	status := domain.AgentMemoryCandidatePending
	if classification.RiskLevel == domain.AgentMemoryRiskHigh {
		status = domain.AgentMemoryCandidateRequiresConfirmation
	}
	candidate, err := s.repository.CreateAgentMemoryCandidate(ctx, domain.AgentMemoryCandidate{
		UserID:        entry.UserID,
		SessionID:     entry.SessionID,
		TurnID:        entry.TurnID,
		MemoryKind:    classification.Kind,
		CandidateText: strings.TrimSpace(entry.Content),
		Summary:       summarizeMemoryCandidate(entry.Content),
		EvidenceRefs:  []string{fmt.Sprintf("transcript:%d", entry.ID)},
		SourceRefs:    []string{fmt.Sprintf("transcript:%d", entry.ID), fmt.Sprintf("turn:%d", entry.TurnID), fmt.Sprintf("session:%d", entry.SessionID)},
		Confidence:    classification.Confidence,
		Importance:    classification.Importance,
		RiskLevel:     classification.RiskLevel,
		Status:        status,
		ProposedBy:    "system",
		Metadata: domain.AgentJSON{
			"classification_reason": classification.Reason,
			"classification_terms":  classification.Terms,
			"classifier":            "rule_fallback",
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		s.recordMemoryCaptureFailure(ctx, entry, "candidate_create_failed", err)
		return
	}
	if status == domain.AgentMemoryCandidateRequiresConfirmation {
		_, _ = s.repository.CreateAgentMemoryEvent(ctx, domain.AgentMemoryEvent{
			UserID:      entry.UserID,
			SessionID:   entry.SessionID,
			TurnID:      entry.TurnID,
			CandidateID: candidate.ID,
			EventType:   domain.AgentMemoryEventCandidateRequiresConfirmation,
			ActorType:   domain.AgentMemoryActorSystem,
			Reason:      "high risk memory candidate requires confirmation",
			Payload: domain.AgentJSON{
				"risk_level": string(classification.RiskLevel),
			},
			CreatedAt: now,
		})
		return
	}
	block, err := s.repository.ApplyAgentMemoryCandidate(ctx, entry.UserID, candidate.ID, now)
	if err != nil {
		s.recordMemoryCaptureFailure(ctx, entry, "candidate_apply_failed", err)
		return
	}
	s.createMemoryChunkFromCapture(ctx, entry, classification, block)
}

func (s *AgentConversationService) createMemoryChunkFromCapture(ctx context.Context, entry domain.AgentTranscriptEntry, classification agentMemoryCaptureClassification, block domain.AgentMemoryBlock) {
	store, ok := any(s.repository).(agentMemoryTopicChunkStore)
	if !ok || entry.UserID == 0 || strings.TrimSpace(entry.Content) == "" {
		return
	}
	now := s.now().UTC()
	topic, err := store.CreateAgentMemoryTopic(ctx, domain.AgentMemoryTopic{
		UserID:        entry.UserID,
		SessionID:     entry.SessionID,
		TopicKey:      fmt.Sprintf("memory-capture:%d", entry.TurnID),
		Title:         summarizeMemoryCandidate(entry.Content),
		Summary:       summarizeMemoryCandidate(entry.Content),
		Keywords:      append([]string(nil), classification.Terms...),
		Intent:        classification.Reason,
		Status:        domain.AgentMemoryTopicClosed,
		MessageCount:  1,
		TokenEstimate: estimateTokenCount(entry.Content),
		StartTurnID:   entry.TurnID,
		EndTurnID:     entry.TurnID,
		LastMessageAt: &entry.CreatedAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		s.recordMemoryCaptureFailure(ctx, entry, "topic_create_failed", err)
		return
	}
	metrics.AgentMemoryTopicsTotal.WithLabelValues(string(topic.Status), "high_value_signal").Inc()
	chunk, err := store.CreateAgentMemoryChunk(ctx, domain.AgentMemoryChunk{
		UserID:              entry.UserID,
		SessionID:           entry.SessionID,
		TopicID:             topic.ID,
		Title:               topic.Title,
		Summary:             topic.Summary,
		Content:             strings.TrimSpace(entry.Content),
		MemoryKind:          classification.Kind,
		Importance:          classification.Importance,
		SourceRefs:          []string{fmt.Sprintf("transcript:%d", entry.ID), fmt.Sprintf("turn:%d", entry.TurnID), fmt.Sprintf("memory_block:%d", block.ID)},
		RelationRefs:        []string{fmt.Sprintf("session:%d", entry.SessionID), fmt.Sprintf("turn:%d", entry.TurnID)},
		StartTurnID:         entry.TurnID,
		EndTurnID:           entry.TurnID,
		EmbeddingStatus:     domain.AgentFactEmbeddingStatusPending,
		ConsolidationReason: "high_value_signal",
		Metadata: domain.AgentJSON{
			"classification_reason": classification.Reason,
			"classification_terms":  classification.Terms,
			"memory_block_id":       block.ID,
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		s.recordMemoryCaptureFailure(ctx, entry, "chunk_create_failed", err)
		metrics.AgentMemoryChunksTotal.WithLabelValues(string(classification.Kind), "high_value_signal", "failed").Inc()
		return
	}
	metrics.AgentMemoryChunksTotal.WithLabelValues(string(chunk.MemoryKind), chunk.ConsolidationReason, "created").Inc()
	s.recordAgentTraceEvent(ctx, domain.AgentTraceEvent{
		UserID:        entry.UserID,
		SessionID:     entry.SessionID,
		TurnID:        entry.TurnID,
		EventKind:     domain.AgentTraceEventMemory,
		EventName:     "memory_chunk_created",
		Status:        domain.AgentTraceEventSucceeded,
		StartedAt:     now,
		FinishedAt:    &now,
		DurationMS:    0,
		SourceRefs:    chunk.SourceRefs,
		InputSummary:  summarizeMemoryCandidate(entry.Content),
		OutputSummary: fmt.Sprintf("memory_chunk:%d", chunk.ID),
		Metadata: domain.AgentJSON{
			"topic_id":              topic.ID,
			"chunk_id":              chunk.ID,
			"memory_kind":           string(chunk.MemoryKind),
			"consolidation_reason":  chunk.ConsolidationReason,
			"embedding_status":      string(chunk.EmbeddingStatus),
			"classification_reason": classification.Reason,
		},
		CreatedAt: now,
	})
}

func (s *AgentConversationService) memoryBlockAlreadyExists(ctx context.Context, userID int64, content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return true
	}
	blocks, err := s.repository.ListAgentMemoryBlocks(ctx, domain.AgentMemoryBlockQueryOptions{
		UserID: userID,
		Query:  content,
		Limit:  5,
	})
	if err != nil {
		return false
	}
	for _, block := range blocks {
		if strings.EqualFold(strings.TrimSpace(block.Content), content) {
			return true
		}
	}
	return false
}

func (s *AgentConversationService) recordMemoryCaptureFailure(ctx context.Context, entry domain.AgentTranscriptEntry, eventType string, err error) {
	if err == nil {
		return
	}
	_, _ = s.repository.CreateAuditLog(ctx, domain.AgentAuditLog{
		SessionID: entry.SessionID,
		TurnID:    entry.TurnID,
		UserID:    entry.UserID,
		EventType: "agent.memory_capture." + eventType,
		Status:    "failed",
		Message:   err.Error(),
		Metadata: domain.AgentJSON{
			"transcript_entry_id": entry.ID,
		},
		CreatedAt: time.Now().UTC(),
	})
}

func classifyAgentMemoryCandidate(content string) agentMemoryCaptureClassification {
	content = strings.TrimSpace(content)
	if content == "" {
		return agentMemoryCaptureClassification{Kind: domain.AgentMemoryKindUnknown, Reason: "empty_content"}
	}
	categories := []struct {
		kind       domain.AgentMemoryKind
		reason     string
		importance int
		terms      []string
	}{
		{
			kind:       domain.AgentMemoryKindDecision,
			reason:     "matched_decision_terms",
			importance: 82,
			terms:      []string{"决定", "确定", "最终", "结论", "就用", "采用", "选择", "批准"},
		},
		{
			kind:       domain.AgentMemoryKindPreference,
			reason:     "matched_preference_terms",
			importance: 78,
			terms:      []string{"偏好", "喜欢", "不喜欢", "以后", "记住", "默认", "习惯", "希望", "不要", "别再", "优先"},
		},
		{
			kind:       domain.AgentMemoryKindFact,
			reason:     "matched_fact_terms",
			importance: 65,
			terms:      []string{"我是", "我的", "叫", "用户名", "公司", "账号", "地区", "时区", "邮箱", "项目"},
		},
	}
	for _, category := range categories {
		matched := matchedMemoryCaptureTerms(content, category.terms)
		if len(matched) == 0 {
			continue
		}
		confidence := 0.72
		if len(matched) >= 2 {
			confidence = 0.84
		}
		return agentMemoryCaptureClassification{
			Kind:       category.kind,
			Terms:      matched,
			Importance: category.importance,
			Confidence: confidence,
			RiskLevel:  classifyMemoryCaptureRisk(content),
			Reason:     category.reason,
		}
	}
	return agentMemoryCaptureClassification{Kind: domain.AgentMemoryKindCasual, Importance: 20, Confidence: 0.4, RiskLevel: classifyMemoryCaptureRisk(content), Reason: "fallback_casual"}
}

func agentMemoryKindShouldCapture(kind domain.AgentMemoryKind) bool {
	switch kind {
	case domain.AgentMemoryKindPreference, domain.AgentMemoryKindFact, domain.AgentMemoryKindDecision:
		return true
	default:
		return false
	}
}

func matchedMemoryCaptureTerms(content string, terms []string) []string {
	matched := make([]string, 0, 3)
	lower := strings.ToLower(content)
	for _, term := range terms {
		if strings.Contains(lower, strings.ToLower(term)) {
			matched = append(matched, term)
			if len(matched) >= 3 {
				break
			}
		}
	}
	return matched
}

func classifyMemoryCaptureRisk(content string) domain.AgentMemoryRiskLevel {
	lower := strings.ToLower(strings.TrimSpace(content))
	for _, term := range []string{"密码", "口令", "密钥", "token", "secret", "api key", "apikey", "身份证", "银行卡"} {
		if strings.Contains(lower, term) {
			return domain.AgentMemoryRiskHigh
		}
	}
	return domain.AgentMemoryRiskLow
}

func summarizeMemoryCandidate(content string) string {
	content = strings.TrimSpace(content)
	if len(content) <= 160 {
		return content
	}
	return strings.TrimSpace(content[:160])
}
