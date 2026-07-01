package repository

import (
	"context"
	"crypto/sha256"
	"fmt"
	"messagefeed/internal/domain"
	"strings"
	"time"
)

type agentMemoryTopicModel struct {
	ID            int64 `gorm:"primaryKey"`
	UserID        int64 `gorm:"not null"`
	SessionID     *int64
	TopicKey      string
	Title         string
	Summary       string
	Keywords      []string `gorm:"column:keywords_json;serializer:json;type:jsonb;not null"`
	Intent        string
	Status        string
	MessageCount  int
	TokenEstimate int
	StartTurnID   *int64
	EndTurnID     *int64
	LastMessageAt *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type agentMemoryChunkModel struct {
	ID                  int64 `gorm:"primaryKey"`
	UserID              int64 `gorm:"not null"`
	SessionID           *int64
	TopicID             *int64
	Title               string
	Summary             string
	Content             string
	MemoryKind          string
	Importance          int
	SourceRefs          []string `gorm:"column:source_refs_json;serializer:json;type:jsonb;not null"`
	RelationRefs        []string `gorm:"column:relation_refs_json;serializer:json;type:jsonb;not null"`
	StartTurnID         *int64
	EndTurnID           *int64
	ContentHash         string
	EmbeddingStatus     string
	ConsolidationReason string
	Metadata            domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (agentMemoryTopicModel) TableName() string { return "agent_memory_topics" }
func (agentMemoryChunkModel) TableName() string { return "agent_memory_chunks" }

func (r *AgentRepository) CreateAgentMemoryTopic(ctx context.Context, topic domain.AgentMemoryTopic) (domain.AgentMemoryTopic, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory_topic.create", "insert", "agent_memory_topics")
	var opErr error
	defer func() { finish(opErr) }()

	topic = normalizeAgentMemoryTopic(topic)
	if topic.UserID == 0 {
		opErr = domain.ErrInvalidInput
		return domain.AgentMemoryTopic{}, opErr
	}
	model := agentMemoryTopicModelFromDomain(topic)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentMemoryTopic{}, opErr
	}
	return agentMemoryTopicModelToDomain(model), nil
}

func (r *AgentRepository) ListAgentMemoryTopics(ctx context.Context, options domain.AgentMemoryTopicListOptions) ([]domain.AgentMemoryTopic, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory_topic.list", "select", "agent_memory_topics")
	var opErr error
	defer func() { finish(opErr) }()

	query := r.db.WithContext(ctx).Model(&agentMemoryTopicModel{})
	if options.UserID > 0 {
		query = query.Where("user_id = ?", options.UserID)
	}
	if options.SessionID > 0 {
		query = query.Where("session_id = ?", options.SessionID)
	}
	if len(options.Statuses) > 0 {
		statuses := make([]string, 0, len(options.Statuses))
		for _, status := range options.Statuses {
			if status.Valid() {
				statuses = append(statuses, string(status))
			}
		}
		if len(statuses) > 0 {
			query = query.Where("status IN ?", statuses)
		}
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	var models []agentMemoryTopicModel
	if err := query.Order("updated_at DESC, id DESC").Limit(limit).Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	topics := make([]domain.AgentMemoryTopic, 0, len(models))
	for _, model := range models {
		topics = append(topics, agentMemoryTopicModelToDomain(model))
	}
	return topics, nil
}

func (r *AgentRepository) UpdateAgentMemoryTopic(ctx context.Context, topic domain.AgentMemoryTopic) (domain.AgentMemoryTopic, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory_topic.update", "update", "agent_memory_topics")
	var opErr error
	defer func() { finish(opErr) }()

	topic = normalizeAgentMemoryTopic(topic)
	model := agentMemoryTopicModelFromDomain(topic)
	result := r.db.WithContext(ctx).
		Model(&agentMemoryTopicModel{}).
		Where("id = ? AND user_id = ?", topic.ID, topic.UserID).
		Updates(map[string]any{
			"topic_key":       model.TopicKey,
			"title":           model.Title,
			"summary":         model.Summary,
			"keywords_json":   model.Keywords,
			"intent":          model.Intent,
			"status":          model.Status,
			"message_count":   model.MessageCount,
			"token_estimate":  model.TokenEstimate,
			"end_turn_id":     model.EndTurnID,
			"last_message_at": model.LastMessageAt,
			"updated_at":      time.Now().UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentMemoryTopic{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentMemoryTopic{}, opErr
	}
	var stored agentMemoryTopicModel
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", topic.ID, topic.UserID).First(&stored).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentMemoryTopic{}, opErr
	}
	return agentMemoryTopicModelToDomain(stored), nil
}

func (r *AgentRepository) CreateAgentMemoryChunk(ctx context.Context, chunk domain.AgentMemoryChunk) (domain.AgentMemoryChunk, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory_chunk.create", "insert", "agent_memory_chunks")
	var opErr error
	defer func() { finish(opErr) }()

	chunk = normalizeAgentMemoryChunk(chunk)
	if chunk.UserID == 0 || strings.TrimSpace(chunk.Content) == "" {
		opErr = domain.ErrInvalidInput
		return domain.AgentMemoryChunk{}, opErr
	}
	model := agentMemoryChunkModelFromDomain(chunk)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentMemoryChunk{}, opErr
	}
	persisted := agentMemoryChunkModelToDomain(model)
	r.indexMemoryChunkFact(ctx, persisted)
	return persisted, nil
}

func (r *AgentRepository) indexMemoryChunkFact(ctx context.Context, chunk domain.AgentMemoryChunk) {
	if chunk.ID == 0 || chunk.UserID == 0 {
		return
	}
	ref := fmt.Sprintf("memory_chunk:%d", chunk.ID)
	now := chunk.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	fact, err := r.UpsertAgentFactArchiveIndex(ctx, domain.AgentFactArchiveIndex{
		CanonicalRef:    ref,
		FactType:        domain.AgentFactTypeMemoryChunk,
		FactID:          chunk.ID,
		UserID:          chunk.UserID,
		SessionID:       chunk.SessionID,
		TurnID:          chunk.EndTurnID,
		MemoryKind:      chunk.MemoryKind,
		Topics:          []string{fmt.Sprintf("topic:%d", chunk.TopicID), chunk.Title},
		Keywords:        []string{},
		SummaryForIndex: memoryChunkIndexSummary(chunk),
		ContextualText:  chunk.Content,
		Embedding: domain.AgentJSON{
			"status": string(chunk.EmbeddingStatus),
		},
		Importance:   chunk.Importance,
		Confidence:   0.8,
		SourceRefs:   append([]string(nil), chunk.SourceRefs...),
		RelationRefs: append([]string(nil), chunk.RelationRefs...),
		IndexStatus:  domain.AgentFactIndexStatusReady,
		RiskLevel:    domain.AgentMemoryRiskLow,
		Metadata: domain.AgentJSON{
			"source":               "agent_memory_chunks",
			"consolidation_reason": chunk.ConsolidationReason,
			"content_hash":         chunk.ContentHash,
		},
		CreatedAt: chunk.CreatedAt,
		UpdatedAt: now,
	})
	if err != nil || fact.CanonicalRef == "" || chunk.EmbeddingStatus != domain.AgentFactEmbeddingStatusPending {
		return
	}
	_, _ = r.CreateAgentFactIndexJob(ctx, domain.AgentFactIndexJob{
		JobType: domain.AgentFactIndexJobEmbed,
		Scope: domain.AgentJSON{
			"user_id":        chunk.UserID,
			"canonical_refs": []string{fact.CanonicalRef},
			"reason":         chunk.ConsolidationReason,
		},
		Status:    domain.AgentFactIndexJobPending,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func normalizeAgentMemoryTopic(topic domain.AgentMemoryTopic) domain.AgentMemoryTopic {
	topic.TopicKey = strings.TrimSpace(topic.TopicKey)
	topic.Title = strings.TrimSpace(topic.Title)
	topic.Summary = strings.TrimSpace(topic.Summary)
	topic.Intent = strings.TrimSpace(topic.Intent)
	if !topic.Status.Valid() {
		topic.Status = domain.AgentMemoryTopicActive
	}
	if topic.MessageCount < 0 {
		topic.MessageCount = 0
	}
	if topic.TokenEstimate < 0 {
		topic.TokenEstimate = 0
	}
	if topic.CreatedAt.IsZero() {
		topic.CreatedAt = time.Now().UTC()
	}
	if topic.UpdatedAt.IsZero() {
		topic.UpdatedAt = topic.CreatedAt
	}
	return topic
}

func normalizeAgentMemoryChunk(chunk domain.AgentMemoryChunk) domain.AgentMemoryChunk {
	chunk.Title = strings.TrimSpace(chunk.Title)
	chunk.Summary = strings.TrimSpace(chunk.Summary)
	chunk.Content = strings.TrimSpace(chunk.Content)
	chunk.ConsolidationReason = strings.TrimSpace(chunk.ConsolidationReason)
	if chunk.ConsolidationReason == "" {
		chunk.ConsolidationReason = "unknown"
	}
	if !chunk.MemoryKind.Valid() {
		chunk.MemoryKind = domain.AgentMemoryKindUnknown
	}
	if chunk.Importance < 0 {
		chunk.Importance = 0
	}
	if chunk.Importance > 100 {
		chunk.Importance = 100
	}
	if !chunk.EmbeddingStatus.Valid() {
		chunk.EmbeddingStatus = domain.AgentFactEmbeddingStatusPending
	}
	if chunk.ContentHash == "" && chunk.Content != "" {
		chunk.ContentHash = memoryChunkContentHash(chunk.Content)
	}
	if chunk.Metadata == nil {
		chunk.Metadata = domain.AgentJSON{}
	}
	if chunk.CreatedAt.IsZero() {
		chunk.CreatedAt = time.Now().UTC()
	}
	if chunk.UpdatedAt.IsZero() {
		chunk.UpdatedAt = chunk.CreatedAt
	}
	return chunk
}

func agentMemoryTopicModelFromDomain(topic domain.AgentMemoryTopic) agentMemoryTopicModel {
	return agentMemoryTopicModel{
		ID:            topic.ID,
		UserID:        topic.UserID,
		SessionID:     int64Pointer(topic.SessionID),
		TopicKey:      topic.TopicKey,
		Title:         topic.Title,
		Summary:       topic.Summary,
		Keywords:      cloneStringSlice(topic.Keywords),
		Intent:        topic.Intent,
		Status:        string(topic.Status),
		MessageCount:  topic.MessageCount,
		TokenEstimate: topic.TokenEstimate,
		StartTurnID:   int64Pointer(topic.StartTurnID),
		EndTurnID:     int64Pointer(topic.EndTurnID),
		LastMessageAt: topic.LastMessageAt,
		CreatedAt:     topic.CreatedAt,
		UpdatedAt:     topic.UpdatedAt,
	}
}

func agentMemoryTopicModelToDomain(model agentMemoryTopicModel) domain.AgentMemoryTopic {
	return domain.AgentMemoryTopic{
		ID:            model.ID,
		UserID:        model.UserID,
		SessionID:     int64Value(model.SessionID),
		TopicKey:      model.TopicKey,
		Title:         model.Title,
		Summary:       model.Summary,
		Keywords:      cloneStringSlice(model.Keywords),
		Intent:        model.Intent,
		Status:        domain.AgentMemoryTopicStatus(model.Status),
		MessageCount:  model.MessageCount,
		TokenEstimate: model.TokenEstimate,
		StartTurnID:   int64Value(model.StartTurnID),
		EndTurnID:     int64Value(model.EndTurnID),
		LastMessageAt: model.LastMessageAt,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}
}

func agentMemoryChunkModelFromDomain(chunk domain.AgentMemoryChunk) agentMemoryChunkModel {
	return agentMemoryChunkModel{
		ID:                  chunk.ID,
		UserID:              chunk.UserID,
		SessionID:           int64Pointer(chunk.SessionID),
		TopicID:             int64Pointer(chunk.TopicID),
		Title:               chunk.Title,
		Summary:             chunk.Summary,
		Content:             chunk.Content,
		MemoryKind:          string(chunk.MemoryKind),
		Importance:          chunk.Importance,
		SourceRefs:          cloneStringSlice(chunk.SourceRefs),
		RelationRefs:        cloneStringSlice(chunk.RelationRefs),
		StartTurnID:         int64Pointer(chunk.StartTurnID),
		EndTurnID:           int64Pointer(chunk.EndTurnID),
		ContentHash:         chunk.ContentHash,
		EmbeddingStatus:     string(chunk.EmbeddingStatus),
		ConsolidationReason: chunk.ConsolidationReason,
		Metadata:            cloneAgentJSON(chunk.Metadata),
		CreatedAt:           chunk.CreatedAt,
		UpdatedAt:           chunk.UpdatedAt,
	}
}

func agentMemoryChunkModelToDomain(model agentMemoryChunkModel) domain.AgentMemoryChunk {
	return domain.AgentMemoryChunk{
		ID:                  model.ID,
		UserID:              model.UserID,
		SessionID:           int64Value(model.SessionID),
		TopicID:             int64Value(model.TopicID),
		Title:               model.Title,
		Summary:             model.Summary,
		Content:             model.Content,
		MemoryKind:          domain.AgentMemoryKind(model.MemoryKind),
		Importance:          model.Importance,
		SourceRefs:          cloneStringSlice(model.SourceRefs),
		RelationRefs:        cloneStringSlice(model.RelationRefs),
		StartTurnID:         int64Value(model.StartTurnID),
		EndTurnID:           int64Value(model.EndTurnID),
		ContentHash:         model.ContentHash,
		EmbeddingStatus:     domain.AgentFactEmbeddingStatus(model.EmbeddingStatus),
		ConsolidationReason: model.ConsolidationReason,
		Metadata:            cloneAgentJSON(model.Metadata),
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
	}
}

func memoryChunkContentHash(content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return "sha256:" + fmt.Sprintf("%x", sum[:])
}

func memoryChunkIndexSummary(chunk domain.AgentMemoryChunk) string {
	if strings.TrimSpace(chunk.Summary) != "" {
		return strings.TrimSpace(chunk.Summary)
	}
	return strings.TrimSpace(chunk.Title)
}
