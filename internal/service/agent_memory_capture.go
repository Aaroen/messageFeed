package service

import (
	"context"
	"encoding/json"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
	"strings"
	"time"
)

type agentMemoryTopicChunkStore interface {
	ListAgentMemoryTopics(ctx context.Context, options domain.AgentMemoryTopicListOptions) ([]domain.AgentMemoryTopic, error)
	CreateAgentMemoryTopic(ctx context.Context, topic domain.AgentMemoryTopic) (domain.AgentMemoryTopic, error)
	UpdateAgentMemoryTopic(ctx context.Context, topic domain.AgentMemoryTopic) (domain.AgentMemoryTopic, error)
	CreateAgentMemoryChunk(ctx context.Context, chunk domain.AgentMemoryChunk) (domain.AgentMemoryChunk, error)
}

type agentMemoryCaptureClassification struct {
	ShouldCapture        bool
	Kind                 domain.AgentMemoryKind
	Keywords             []string
	Importance           int
	Confidence           float64
	RiskLevel            domain.AgentMemoryRiskLevel
	Summary              string
	TopicDecision        string
	TopicTitle           string
	TopicSummary         string
	TopicIntent          string
	ConsolidationReason  string
	ShouldCreateChunk    bool
	ChunkTitle           string
	ChunkSummary         string
	ChunkContent         string
	RequiresConfirmation bool
	Reason               string
	Classifier           string
}

func (s *AgentConversationService) captureMemoryCandidateFromTranscript(ctx context.Context, entry domain.AgentTranscriptEntry) {
	if s == nil || s.repository == nil || entry.ID == 0 || entry.UserID == 0 || entry.Role != domain.AgentTranscriptRoleUser {
		return
	}
	activeTopic, hasActiveTopic := s.activeMemoryTopic(ctx, entry)
	classification := s.classifyAgentMemoryCandidateWithModel(ctx, entry, activeTopic, hasActiveTopic)
	activeTopic, hasActiveTopic = s.trackMemoryTopicFromTranscript(ctx, entry, activeTopic, hasActiveTopic, classification)
	if !classification.ShouldCapture || !agentMemoryKindShouldCapture(classification.Kind) {
		return
	}
	if s.memoryBlockAlreadyExists(ctx, entry.UserID, entry.Content) {
		return
	}
	now := s.now().UTC()
	status := domain.AgentMemoryCandidatePending
	if classification.RiskLevel == domain.AgentMemoryRiskHigh || classification.RequiresConfirmation {
		status = domain.AgentMemoryCandidateRequiresConfirmation
	}
	summary := firstNonEmptyString(classification.Summary, summarizeMemoryCandidate(entry.Content))
	candidate, err := s.repository.CreateAgentMemoryCandidate(ctx, domain.AgentMemoryCandidate{
		UserID:        entry.UserID,
		SessionID:     entry.SessionID,
		TurnID:        entry.TurnID,
		MemoryKind:    classification.Kind,
		CandidateText: strings.TrimSpace(entry.Content),
		Summary:       summary,
		EvidenceRefs:  []string{fmt.Sprintf("transcript:%d", entry.ID)},
		SourceRefs:    []string{fmt.Sprintf("transcript:%d", entry.ID), fmt.Sprintf("turn:%d", entry.TurnID), fmt.Sprintf("session:%d", entry.SessionID)},
		Confidence:    classification.Confidence,
		Importance:    classification.Importance,
		RiskLevel:     classification.RiskLevel,
		Status:        status,
		ProposedBy:    "system",
		Metadata: domain.AgentJSON{
			"classification_reason":   classification.Reason,
			"classification_keywords": classification.Keywords,
			"classifier":              classification.Classifier,
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		s.recordMemoryCaptureFailure(ctx, entry, "candidate_create_failed", err)
		return
	}
	s.recordAgentTraceEvent(ctx, domain.AgentTraceEvent{
		UserID:        entry.UserID,
		SessionID:     entry.SessionID,
		TurnID:        entry.TurnID,
		EventKind:     domain.AgentTraceEventMemory,
		EventName:     "memory_candidate_created",
		Status:        domain.AgentTraceEventSucceeded,
		StartedAt:     now,
		FinishedAt:    &now,
		InputSummary:  summarizeMemoryCandidate(entry.Content),
		OutputSummary: fmt.Sprintf("memory_candidate:%d", candidate.ID),
		Metadata: domain.AgentJSON{
			"candidate_id": candidate.ID,
			"memory_kind":  string(classification.Kind),
			"risk_level":   string(classification.RiskLevel),
			"classifier":   classification.Classifier,
		},
		CreatedAt: now,
	})
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
	if classification.ShouldCreateChunk {
		s.createMemoryChunkFromCapture(ctx, entry, activeTopic, hasActiveTopic, classification, block)
	}
}

func (s *AgentConversationService) captureMemoryCandidateFromTranscriptAsync(ctx context.Context, entry domain.AgentTranscriptEntry) {
	if s == nil || entry.ID == 0 || entry.UserID == 0 || entry.Role != domain.AgentTranscriptRoleUser {
		return
	}
	baseCtx := context.WithoutCancel(ctx)
	go func() {
		captureCtx, cancel := context.WithTimeout(baseCtx, defaultAgentMemoryCaptureTimeout)
		defer cancel()
		s.captureMemoryCandidateFromTranscript(captureCtx, entry)
	}()
}

func (s *AgentConversationService) createMemoryChunkFromCapture(ctx context.Context, entry domain.AgentTranscriptEntry, activeTopic domain.AgentMemoryTopic, hasActiveTopic bool, classification agentMemoryCaptureClassification, block domain.AgentMemoryBlock) {
	store, ok := any(s.repository).(agentMemoryTopicChunkStore)
	if !ok || entry.UserID == 0 || strings.TrimSpace(entry.Content) == "" {
		return
	}
	ctx, span := observability.StartSpan(ctx, "service.agent.memory.chunk.build")
	defer observability.EndSpan(span, nil)

	now := s.now().UTC()
	topic := activeTopic
	if !hasActiveTopic || topic.ID == 0 {
		created, err := s.createTrackedMemoryTopic(ctx, entry, classification, firstNonEmptyString(classification.ConsolidationReason, "high_value"))
		if err != nil {
			s.recordMemoryCaptureFailure(ctx, entry, "topic_create_failed", err)
			return
		}
		topic = created
	}
	reason := normalizeMemoryConsolidationReason(classification.ConsolidationReason)
	if reason == "none" {
		reason = "high_value"
	}
	title := firstNonEmptyString(classification.ChunkTitle, classification.TopicTitle, topic.Title, summarizeMemoryCandidate(entry.Content))
	summary := firstNonEmptyString(classification.ChunkSummary, classification.Summary, classification.TopicSummary, topic.Summary, summarizeMemoryCandidate(entry.Content))
	content := firstNonEmptyString(classification.ChunkContent, classification.Summary, strings.TrimSpace(entry.Content))
	chunk, err := store.CreateAgentMemoryChunk(ctx, domain.AgentMemoryChunk{
		UserID:              entry.UserID,
		SessionID:           entry.SessionID,
		TopicID:             topic.ID,
		Title:               title,
		Summary:             summary,
		Content:             content,
		MemoryKind:          classification.Kind,
		Importance:          classification.Importance,
		SourceRefs:          []string{fmt.Sprintf("transcript:%d", entry.ID), fmt.Sprintf("turn:%d", entry.TurnID), fmt.Sprintf("memory_block:%d", block.ID)},
		RelationRefs:        []string{fmt.Sprintf("session:%d", entry.SessionID), fmt.Sprintf("turn:%d", entry.TurnID)},
		StartTurnID:         entry.TurnID,
		EndTurnID:           entry.TurnID,
		EmbeddingStatus:     domain.AgentFactEmbeddingStatusPending,
		ConsolidationReason: reason,
		Metadata: domain.AgentJSON{
			"classification_reason":   classification.Reason,
			"classification_source":   classification.Classifier,
			"classification_keywords": classification.Keywords,
			"memory_block_id":         block.ID,
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		s.recordMemoryCaptureFailure(ctx, entry, "chunk_create_failed", err)
		metrics.AgentMemoryChunksTotal.WithLabelValues(string(classification.Kind), reason, "failed").Inc()
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

type agentMemoryClassificationJSON struct {
	ShouldCapture        bool     `json:"should_capture"`
	MemoryKind           string   `json:"memory_kind"`
	Confidence           float64  `json:"confidence"`
	Importance           int      `json:"importance"`
	RiskLevel            string   `json:"risk_level"`
	Summary              string   `json:"summary"`
	Keywords             []string `json:"keywords"`
	TopicDecision        string   `json:"topic_decision"`
	TopicTitle           string   `json:"topic_title"`
	TopicSummary         string   `json:"topic_summary"`
	TopicIntent          string   `json:"topic_intent"`
	ConsolidationReason  string   `json:"consolidation_reason"`
	ShouldCreateChunk    bool     `json:"should_create_chunk"`
	ChunkTitle           string   `json:"chunk_title"`
	ChunkSummary         string   `json:"chunk_summary"`
	ChunkContent         string   `json:"chunk_content"`
	RequiresConfirmation bool     `json:"requires_confirmation"`
	Reason               string   `json:"reason"`
}

func (s *AgentConversationService) classifyAgentMemoryCandidateWithModel(ctx context.Context, entry domain.AgentTranscriptEntry, activeTopic domain.AgentMemoryTopic, hasActiveTopic bool) agentMemoryCaptureClassification {
	startedAt := s.now().UTC()
	if s.llmClient == nil {
		classification := fallbackAgentMemoryClassification("llm_client_missing")
		s.recordMemoryClassificationTrace(ctx, entry, classification, domain.AgentTraceEventDegraded, startedAt, "llm_client_missing", "")
		return classification
	}
	ctx, span := observability.StartSpan(ctx, "service.agent.memory.classify")
	defer observability.EndSpan(span, nil)
	ctx, consolidationSpan := observability.StartSpan(ctx, "service.agent.memory.consolidation.evaluate")
	defer observability.EndSpan(consolidationSpan, nil)

	payload := domain.AgentJSON{
		"user_id":       entry.UserID,
		"session_id":    entry.SessionID,
		"turn_id":       entry.TurnID,
		"transcript_id": entry.ID,
		"user_message":  strings.TrimSpace(entry.Content),
		"current_time":  s.now().UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if hasActiveTopic && activeTopic.ID > 0 {
		payload["active_topic"] = domain.AgentJSON{
			"id":             activeTopic.ID,
			"title":          activeTopic.Title,
			"summary":        activeTopic.Summary,
			"keywords":       activeTopic.Keywords,
			"intent":         activeTopic.Intent,
			"message_count":  activeTopic.MessageCount,
			"token_estimate": activeTopic.TokenEstimate,
			"start_turn_id":  activeTopic.StartTurnID,
			"end_turn_id":    activeTopic.EndTurnID,
		}
	}
	response, err := s.llmClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: agentMemoryClassificationSystemPrompt()},
			{Role: "user", Content: agentMemoryClassificationUserPrompt(payload)},
		},
		Temperature: 0,
		MaxTokens:   1800,
	})
	if err != nil {
		classification := fallbackAgentMemoryClassification("llm_call_failed")
		s.recordMemoryClassificationTrace(ctx, entry, classification, domain.AgentTraceEventDegraded, startedAt, "llm_call_failed", err.Error())
		return classification
	}
	classification, parseErr := parseAgentMemoryClassificationJSON(response.Content)
	if parseErr != nil {
		classification := fallbackAgentMemoryClassification("llm_response_invalid")
		s.recordMemoryClassificationTrace(ctx, entry, classification, domain.AgentTraceEventDegraded, startedAt, "llm_response_invalid", parseErr.Error())
		return classification
	}
	classification = guardAgentMemoryClassification(entry.Content, classification)
	s.recordMemoryClassificationTrace(ctx, entry, classification, domain.AgentTraceEventSucceeded, startedAt, "", "")
	return classification
}

func parseAgentMemoryClassificationJSON(raw string) (agentMemoryCaptureClassification, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return agentMemoryCaptureClassification{}, fmt.Errorf("memory classification response is empty")
	}
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return agentMemoryCaptureClassification{}, fmt.Errorf("memory classification response has no JSON object")
	}
	var decoded agentMemoryClassificationJSON
	if err := json.Unmarshal([]byte(raw[start:end+1]), &decoded); err != nil {
		return agentMemoryCaptureClassification{}, fmt.Errorf("memory classification JSON parse failed: %w", err)
	}
	classification := agentMemoryCaptureClassification{
		ShouldCapture:        decoded.ShouldCapture,
		Kind:                 domain.AgentMemoryKind(strings.TrimSpace(decoded.MemoryKind)),
		Keywords:             compactNonEmptyStrings(decoded.Keywords),
		Importance:           decoded.Importance,
		Confidence:           decoded.Confidence,
		RiskLevel:            domain.AgentMemoryRiskLevel(strings.TrimSpace(decoded.RiskLevel)),
		Summary:              safeSummary(decoded.Summary, 500),
		TopicDecision:        normalizeMemoryTopicDecision(decoded.TopicDecision),
		TopicTitle:           safeSummary(decoded.TopicTitle, 160),
		TopicSummary:         safeSummary(decoded.TopicSummary, 800),
		TopicIntent:          safeSummary(decoded.TopicIntent, 120),
		ConsolidationReason:  normalizeMemoryConsolidationReason(decoded.ConsolidationReason),
		ShouldCreateChunk:    decoded.ShouldCreateChunk,
		ChunkTitle:           safeSummary(decoded.ChunkTitle, 160),
		ChunkSummary:         safeSummary(decoded.ChunkSummary, 800),
		ChunkContent:         safeSummary(decoded.ChunkContent, 3000),
		RequiresConfirmation: decoded.RequiresConfirmation,
		Reason:               safeSummary(decoded.Reason, 500),
		Classifier:           agentMemoryClassifierName,
	}
	if !classification.Kind.Valid() {
		classification.Kind = domain.AgentMemoryKindUnknown
	}
	if !classification.RiskLevel.Valid() {
		classification.RiskLevel = domain.AgentMemoryRiskMedium
	}
	if classification.Confidence < 0 {
		classification.Confidence = 0
	}
	if classification.Confidence > 1 {
		classification.Confidence = 1
	}
	if classification.Importance < 0 {
		classification.Importance = 0
	}
	if classification.Importance > 100 {
		classification.Importance = 100
	}
	if classification.TopicDecision == "" {
		classification.TopicDecision = "ignore"
	}
	if classification.ConsolidationReason == "" {
		classification.ConsolidationReason = "none"
	}
	return classification, nil
}

func guardAgentMemoryClassification(content string, classification agentMemoryCaptureClassification) agentMemoryCaptureClassification {
	if containsMemoryRiskGuardTerm(content) {
		classification.RiskLevel = domain.AgentMemoryRiskHigh
		classification.RequiresConfirmation = true
	}
	if classification.Kind == domain.AgentMemoryKindUnknown || classification.Kind == domain.AgentMemoryKindCasual {
		classification.ShouldCapture = false
	}
	if classification.ShouldCapture && classification.Summary == "" {
		classification.Summary = summarizeMemoryCandidate(content)
	}
	if classification.ShouldCreateChunk && classification.ChunkContent == "" {
		classification.ChunkContent = firstNonEmptyString(classification.Summary, summarizeMemoryCandidate(content))
	}
	return classification
}

func fallbackAgentMemoryClassification(reason string) agentMemoryCaptureClassification {
	return agentMemoryCaptureClassification{
		ShouldCapture:       false,
		Kind:                domain.AgentMemoryKindUnknown,
		Importance:          0,
		Confidence:          0,
		RiskLevel:           domain.AgentMemoryRiskMedium,
		TopicDecision:       "ignore",
		ConsolidationReason: "none",
		Reason:              reason,
		Classifier:          "conservative_fallback",
	}
}

func normalizeMemoryTopicDecision(value string) string {
	switch strings.TrimSpace(value) {
	case "new_topic", "same_topic", "close_and_new", "close_only", "ignore":
		return strings.TrimSpace(value)
	default:
		return "ignore"
	}
}

func normalizeMemoryConsolidationReason(value string) string {
	switch strings.TrimSpace(value) {
	case "high_value", "topic_switch", "topic_size_exceeded", "context_overflow", "idle", "none":
		return strings.TrimSpace(value)
	default:
		return "none"
	}
}

func (s *AgentConversationService) recordMemoryClassificationTrace(ctx context.Context, entry domain.AgentTranscriptEntry, classification agentMemoryCaptureClassification, status domain.AgentTraceEventStatus, startedAt time.Time, errorCode string, errorMessage string) {
	finishedAt, durationMS := agentTraceFinish(startedAt, s.now)
	s.recordAgentTraceEvent(ctx, domain.AgentTraceEvent{
		UserID:        entry.UserID,
		SessionID:     entry.SessionID,
		TurnID:        entry.TurnID,
		EventKind:     domain.AgentTraceEventMemory,
		EventName:     "memory_classification",
		Status:        status,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
		DurationMS:    durationMS,
		ModelKey:      llmModelKey(s.llmClient),
		InputSummary:  summarizeMemoryCandidate(entry.Content),
		OutputSummary: fmt.Sprintf("capture:%t kind:%s topic:%s chunk:%t", classification.ShouldCapture, classification.Kind, classification.TopicDecision, classification.ShouldCreateChunk),
		ErrorCode:     errorCode,
		ErrorMessage:  errorMessage,
		Metadata: domain.AgentJSON{
			"classifier":            classification.Classifier,
			"should_capture":        classification.ShouldCapture,
			"memory_kind":           string(classification.Kind),
			"risk_level":            string(classification.RiskLevel),
			"confidence":            classification.Confidence,
			"importance":            classification.Importance,
			"topic_decision":        classification.TopicDecision,
			"consolidation_reason":  classification.ConsolidationReason,
			"should_create_chunk":   classification.ShouldCreateChunk,
			"requires_confirmation": classification.RequiresConfirmation,
			"reason":                classification.Reason,
		},
		CreatedAt: startedAt,
	})
}

func agentMemoryKindShouldCapture(kind domain.AgentMemoryKind) bool {
	switch kind {
	case domain.AgentMemoryKindPreference, domain.AgentMemoryKindFact, domain.AgentMemoryKindDecision:
		return true
	default:
		return false
	}
}

func (s *AgentConversationService) activeMemoryTopic(ctx context.Context, entry domain.AgentTranscriptEntry) (domain.AgentMemoryTopic, bool) {
	store, ok := any(s.repository).(agentMemoryTopicChunkStore)
	if !ok || entry.UserID == 0 || strings.TrimSpace(entry.Content) == "" {
		return domain.AgentMemoryTopic{}, false
	}
	topics, err := store.ListAgentMemoryTopics(ctx, domain.AgentMemoryTopicListOptions{
		UserID:    entry.UserID,
		SessionID: entry.SessionID,
		Statuses:  []domain.AgentMemoryTopicStatus{domain.AgentMemoryTopicActive},
		Limit:     1,
	})
	if err != nil {
		s.recordMemoryCaptureFailure(ctx, entry, "topic_list_failed", err)
		return domain.AgentMemoryTopic{}, false
	}
	if len(topics) == 0 {
		return domain.AgentMemoryTopic{}, false
	}
	return topics[0], true
}

func (s *AgentConversationService) trackMemoryTopicFromTranscript(ctx context.Context, entry domain.AgentTranscriptEntry, active domain.AgentMemoryTopic, hasActive bool, classification agentMemoryCaptureClassification) (domain.AgentMemoryTopic, bool) {
	store, ok := any(s.repository).(agentMemoryTopicChunkStore)
	if !ok || entry.UserID == 0 || strings.TrimSpace(entry.Content) == "" {
		return active, hasActive
	}
	ctx, span := observability.StartSpan(ctx, "service.agent.memory.topic.classify")
	defer observability.EndSpan(span, nil)

	now := s.now().UTC()
	decision := normalizeMemoryTopicDecision(classification.TopicDecision)
	reason := normalizeMemoryConsolidationReason(classification.ConsolidationReason)
	if reason == "none" && decision == "close_and_new" {
		reason = "topic_switch"
	}
	if reason == "none" && classification.ShouldCreateChunk {
		reason = "high_value"
	}

	switch decision {
	case "new_topic":
		if !hasActive {
			topic, err := s.createTrackedMemoryTopic(ctx, entry, classification, "new_topic")
			if err == nil {
				return topic, true
			}
		}
	case "same_topic":
		if hasActive {
			topic, err := s.updateTrackedMemoryTopic(ctx, store, active, entry, classification, decision)
			if err == nil {
				return topic, true
			}
		} else {
			topic, err := s.createTrackedMemoryTopic(ctx, entry, classification, "new_topic")
			if err == nil {
				return topic, true
			}
		}
	case "close_and_new":
		if hasActive {
			closed, closeErr := s.closeTrackedMemoryTopic(ctx, store, active, entry, reason, now)
			if closeErr != nil {
				s.recordMemoryCaptureFailure(ctx, entry, "topic_close_failed", closeErr)
			} else if classification.ShouldCreateChunk {
				s.createMemoryChunkFromTopic(ctx, store, closed, reason, classification)
			}
		}
		topic, err := s.createTrackedMemoryTopic(ctx, entry, classification, reason)
		if err == nil {
			return topic, true
		}
	case "close_only":
		if !hasActive {
			return active, false
		}
		closed, closeErr := s.closeTrackedMemoryTopic(ctx, store, active, entry, reason, now)
		if closeErr != nil {
			s.recordMemoryCaptureFailure(ctx, entry, "topic_close_failed", closeErr)
			return active, hasActive
		}
		if classification.ShouldCreateChunk {
			s.createMemoryChunkFromTopic(ctx, store, closed, reason, classification)
		}
		return closed, true
	default:
		s.recordAgentTraceEvent(ctx, domain.AgentTraceEvent{
			UserID:     entry.UserID,
			SessionID:  entry.SessionID,
			TurnID:     entry.TurnID,
			EventKind:  domain.AgentTraceEventMemory,
			EventName:  "memory_topic_classified",
			Status:     domain.AgentTraceEventSkipped,
			StartedAt:  now,
			FinishedAt: &now,
			Metadata: domain.AgentJSON{
				"decision":   decision,
				"classifier": classification.Classifier,
				"reason":     classification.Reason,
			},
			CreatedAt: now,
		})
	}
	return active, hasActive
}

func (s *AgentConversationService) updateTrackedMemoryTopic(ctx context.Context, store agentMemoryTopicChunkStore, active domain.AgentMemoryTopic, entry domain.AgentTranscriptEntry, classification agentMemoryCaptureClassification, decision string) (domain.AgentMemoryTopic, error) {
	now := s.now().UTC()
	updated := active
	updated.MessageCount++
	updated.TokenEstimate += estimateTokenCount(entry.Content)
	updated.EndTurnID = entry.TurnID
	updated.LastMessageAt = &entry.CreatedAt
	updated.UpdatedAt = now
	if len(classification.Keywords) > 0 {
		updated.Keywords = compactNonEmptyStrings(classification.Keywords)
	}
	updated.Summary = firstNonEmptyString(classification.TopicSummary, classification.Summary, updated.Summary)
	updated.Title = firstNonEmptyString(classification.TopicTitle, updated.Title, summarizeMemoryCandidate(updated.Summary))
	updated.Intent = firstNonEmptyString(classification.TopicIntent, updated.Intent)
	updatedTopic, err := store.UpdateAgentMemoryTopic(ctx, updated)
	if err != nil {
		s.recordMemoryCaptureFailure(ctx, entry, "topic_update_failed", err)
		return domain.AgentMemoryTopic{}, err
	}
	s.recordAgentTraceEvent(ctx, domain.AgentTraceEvent{
		UserID:        entry.UserID,
		SessionID:     entry.SessionID,
		TurnID:        entry.TurnID,
		EventKind:     domain.AgentTraceEventMemory,
		EventName:     "memory_topic_classified",
		Status:        domain.AgentTraceEventSucceeded,
		StartedAt:     now,
		FinishedAt:    &now,
		OutputSummary: fmt.Sprintf("same_topic:%d", updatedTopic.ID),
		Metadata: domain.AgentJSON{
			"decision":       decision,
			"message_count":  updatedTopic.MessageCount,
			"token_estimate": updatedTopic.TokenEstimate,
			"intent":         updatedTopic.Intent,
			"classifier":     classification.Classifier,
		},
		CreatedAt: now,
	})
	return updatedTopic, nil
}

func (s *AgentConversationService) createTrackedMemoryTopic(ctx context.Context, entry domain.AgentTranscriptEntry, classification agentMemoryCaptureClassification, reason string) (domain.AgentMemoryTopic, error) {
	store, ok := any(s.repository).(agentMemoryTopicChunkStore)
	if !ok {
		return domain.AgentMemoryTopic{}, domain.ErrInvalidInput
	}
	now := s.now().UTC()
	title := firstNonEmptyString(classification.TopicTitle, classification.Summary, summarizeMemoryCandidate(entry.Content))
	summary := firstNonEmptyString(classification.TopicSummary, classification.Summary, summarizeMemoryCandidate(entry.Content))
	topic, err := store.CreateAgentMemoryTopic(ctx, domain.AgentMemoryTopic{
		UserID:        entry.UserID,
		SessionID:     entry.SessionID,
		TopicKey:      fmt.Sprintf("topic:%d:%d", entry.SessionID, entry.TurnID),
		Title:         title,
		Summary:       summary,
		Keywords:      compactNonEmptyStrings(classification.Keywords),
		Intent:        classification.TopicIntent,
		Status:        domain.AgentMemoryTopicActive,
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
		return domain.AgentMemoryTopic{}, err
	}
	metrics.AgentMemoryTopicsTotal.WithLabelValues(string(topic.Status), reason).Inc()
	s.recordAgentTraceEvent(ctx, domain.AgentTraceEvent{
		UserID:        entry.UserID,
		SessionID:     entry.SessionID,
		TurnID:        entry.TurnID,
		EventKind:     domain.AgentTraceEventMemory,
		EventName:     "memory_topic_created",
		Status:        domain.AgentTraceEventSucceeded,
		StartedAt:     now,
		FinishedAt:    &now,
		OutputSummary: fmt.Sprintf("memory_topic:%d", topic.ID),
		Metadata: domain.AgentJSON{
			"decision":      "new_topic",
			"reason":        reason,
			"topic_id":      topic.ID,
			"memory_kind":   string(classification.Kind),
			"topic_intent":  topic.Intent,
			"topic_keyword": topic.Keywords,
			"classifier":    classification.Classifier,
		},
		CreatedAt: now,
	})
	return topic, nil
}

func (s *AgentConversationService) closeTrackedMemoryTopic(ctx context.Context, store agentMemoryTopicChunkStore, topic domain.AgentMemoryTopic, entry domain.AgentTranscriptEntry, reason string, now time.Time) (domain.AgentMemoryTopic, error) {
	topic.Status = domain.AgentMemoryTopicClosed
	if entry.TurnID > 0 {
		topic.EndTurnID = entry.TurnID
	}
	topic.UpdatedAt = now
	closed, err := store.UpdateAgentMemoryTopic(ctx, topic)
	if err != nil {
		return domain.AgentMemoryTopic{}, err
	}
	metrics.AgentMemoryTopicsTotal.WithLabelValues(string(closed.Status), reason).Inc()
	s.recordAgentTraceEvent(ctx, domain.AgentTraceEvent{
		UserID:        topic.UserID,
		SessionID:     topic.SessionID,
		TurnID:        topic.EndTurnID,
		EventKind:     domain.AgentTraceEventMemory,
		EventName:     "memory_topic_closed",
		Status:        domain.AgentTraceEventSucceeded,
		StartedAt:     now,
		FinishedAt:    &now,
		OutputSummary: fmt.Sprintf("memory_topic:%d", topic.ID),
		Metadata: domain.AgentJSON{
			"topic_id":             topic.ID,
			"consolidation_reason": reason,
			"message_count":        topic.MessageCount,
			"token_estimate":       topic.TokenEstimate,
		},
		CreatedAt: now,
	})
	return closed, nil
}

func (s *AgentConversationService) createMemoryChunkFromTopic(ctx context.Context, store agentMemoryTopicChunkStore, topic domain.AgentMemoryTopic, reason string, classification agentMemoryCaptureClassification) {
	if topic.ID == 0 || strings.TrimSpace(topic.Summary) == "" {
		return
	}
	ctx, span := observability.StartSpan(ctx, "service.agent.memory.chunk.build")
	defer observability.EndSpan(span, nil)

	now := s.now().UTC()
	kind := classification.Kind
	if !agentMemoryKindShouldCapture(kind) {
		kind = domain.AgentMemoryKindUnknown
	}
	importance := classification.Importance
	if importance < 40 {
		importance = 45
	}
	chunk, err := store.CreateAgentMemoryChunk(ctx, domain.AgentMemoryChunk{
		UserID:              topic.UserID,
		SessionID:           topic.SessionID,
		TopicID:             topic.ID,
		Title:               firstNonEmptyString(classification.ChunkTitle, topic.Title),
		Summary:             firstNonEmptyString(classification.ChunkSummary, topic.Summary),
		Content:             firstNonEmptyString(classification.ChunkContent, classification.ChunkSummary, topic.Summary),
		MemoryKind:          kind,
		Importance:          importance,
		SourceRefs:          []string{fmt.Sprintf("topic:%d", topic.ID), fmt.Sprintf("turn:%d", topic.StartTurnID), fmt.Sprintf("turn:%d", topic.EndTurnID)},
		RelationRefs:        []string{fmt.Sprintf("session:%d", topic.SessionID)},
		StartTurnID:         topic.StartTurnID,
		EndTurnID:           topic.EndTurnID,
		EmbeddingStatus:     domain.AgentFactEmbeddingStatusPending,
		ConsolidationReason: reason,
		Metadata: domain.AgentJSON{
			"topic_key":      topic.TopicKey,
			"topic_keywords": topic.Keywords,
			"topic_intent":   topic.Intent,
			"classifier":     classification.Classifier,
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		metrics.AgentMemoryChunksTotal.WithLabelValues(string(kind), reason, "failed").Inc()
		return
	}
	metrics.AgentMemoryChunksTotal.WithLabelValues(string(chunk.MemoryKind), chunk.ConsolidationReason, "created").Inc()
	s.recordAgentTraceEvent(ctx, domain.AgentTraceEvent{
		UserID:        topic.UserID,
		SessionID:     topic.SessionID,
		TurnID:        topic.EndTurnID,
		EventKind:     domain.AgentTraceEventMemory,
		EventName:     "memory_topic_chunk_created",
		Status:        domain.AgentTraceEventSucceeded,
		StartedAt:     now,
		FinishedAt:    &now,
		SourceRefs:    chunk.SourceRefs,
		OutputSummary: fmt.Sprintf("memory_chunk:%d", chunk.ID),
		Metadata: domain.AgentJSON{
			"topic_id":             topic.ID,
			"chunk_id":             chunk.ID,
			"consolidation_reason": reason,
			"message_count":        topic.MessageCount,
			"token_estimate":       topic.TokenEstimate,
		},
		CreatedAt: now,
	})
}

func containsMemoryRiskGuardTerm(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	for _, term := range agentMemoryRiskGuardTerms {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func summarizeMemoryCandidate(content string) string {
	content = strings.TrimSpace(content)
	if len(content) <= 160 {
		return content
	}
	return strings.TrimSpace(content[:160])
}
