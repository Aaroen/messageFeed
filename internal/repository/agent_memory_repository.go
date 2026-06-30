package repository

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type agentFactArchiveIndexModel struct {
	ID              int64 `gorm:"primaryKey"`
	CanonicalRef    string
	FactType        string
	FactID          int64
	UserID          int64 `gorm:"not null"`
	SessionID       *int64
	TurnID          *int64
	MemoryKind      string
	Topics          []string `gorm:"column:topics_json;serializer:json;type:jsonb;not null"`
	Keywords        []string `gorm:"column:keywords_json;serializer:json;type:jsonb;not null"`
	Entities        []string `gorm:"column:entities_json;serializer:json;type:jsonb;not null"`
	SummaryForIndex string
	ContextualText  string
	Embedding       domain.AgentJSON `gorm:"column:embedding_json;serializer:json;type:jsonb;not null"`
	Importance      int
	Confidence      float64
	SourceRefs      []string `gorm:"column:source_refs_json;serializer:json;type:jsonb;not null"`
	RelationRefs    []string `gorm:"column:relation_refs_json;serializer:json;type:jsonb;not null"`
	IndexStatus     string
	RiskLevel       string
	AccessCount     int
	LastAccessedAt  *time.Time
	Metadata        domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type agentMemoryCandidateModel struct {
	ID             int64 `gorm:"primaryKey"`
	UserID         int64 `gorm:"not null"`
	SessionID      *int64
	TurnID         *int64
	MemoryKind     string
	CandidateText  string
	Summary        string
	EvidenceRefs   []string `gorm:"column:evidence_refs_json;serializer:json;type:jsonb;not null"`
	SourceRefs     []string `gorm:"column:source_refs_json;serializer:json;type:jsonb;not null"`
	Confidence     float64
	Importance     int
	RiskLevel      string
	Status         string
	ProposedBy     string
	ExpiresAt      *time.Time
	ReviewedAt     *time.Time
	ReviewerUserID *int64
	MemoryBlockID  *int64
	Metadata       domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type agentMemoryBlockModel struct {
	ID                int64 `gorm:"primaryKey"`
	UserID            int64 `gorm:"not null"`
	MemoryKind        string
	Title             string
	Content           string
	Summary           string
	EvidenceRefs      []string `gorm:"column:evidence_refs_json;serializer:json;type:jsonb;not null"`
	SourceCandidateID *int64
	Confidence        float64
	Importance        int
	Status            string
	Version           int
	LastUsedAt        *time.Time
	UseCount          int
	Metadata          domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type agentMemoryEventModel struct {
	ID            int64 `gorm:"primaryKey"`
	UserID        int64 `gorm:"not null"`
	SessionID     *int64
	TurnID        *int64
	CandidateID   *int64
	MemoryBlockID *int64
	EventType     string
	ActorType     string
	ActorUserID   *int64
	Reason        string
	Payload       domain.AgentJSON `gorm:"column:payload_json;serializer:json;type:jsonb;not null"`
	CreatedAt     time.Time
}

func (agentFactArchiveIndexModel) TableName() string { return "agent_fact_archive_index" }
func (agentMemoryCandidateModel) TableName() string  { return "agent_memory_candidates" }
func (agentMemoryBlockModel) TableName() string      { return "agent_memory_blocks" }
func (agentMemoryEventModel) TableName() string      { return "agent_memory_events" }

func (r *AgentRepository) UpsertAgentFactArchiveIndex(ctx context.Context, fact domain.AgentFactArchiveIndex) (domain.AgentFactArchiveIndex, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory.fact.upsert", "upsert", "agent_fact_archive_index")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentFactArchiveIndexModelFromDomain(normalizeAgentFactArchiveIndex(fact))
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "canonical_ref"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"fact_type",
				"fact_id",
				"user_id",
				"session_id",
				"turn_id",
				"memory_kind",
				"topics_json",
				"keywords_json",
				"entities_json",
				"summary_for_index",
				"contextual_text",
				"embedding_json",
				"importance",
				"confidence",
				"source_refs_json",
				"relation_refs_json",
				"index_status",
				"risk_level",
				"metadata_json",
				"updated_at",
			}),
		}).
		Create(&model).Error
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentFactArchiveIndex{}, opErr
	}
	var stored agentFactArchiveIndexModel
	if err := r.db.WithContext(ctx).Where("canonical_ref = ?", model.CanonicalRef).First(&stored).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentFactArchiveIndex{}, opErr
	}
	return agentFactArchiveIndexModelToDomain(stored), nil
}

func (r *AgentRepository) QueryAgentFactArchiveIndex(ctx context.Context, options domain.AgentFactArchiveQueryOptions) ([]domain.AgentFactArchiveIndex, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory.fact.query", "select", "agent_fact_archive_index")
	var opErr error
	defer func() { finish(opErr) }()

	limit := options.Limit
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}
	query := r.db.WithContext(ctx).
		Model(&agentFactArchiveIndexModel{}).
		Where("user_id = ?", options.UserID).
		Where("index_status = ?", string(domain.AgentFactIndexStatusReady))
	if options.SessionID > 0 {
		query = query.Where("(session_id = ? OR session_id IS NULL)", options.SessionID)
	}
	if options.TurnID > 0 {
		query = query.Where("turn_id <= ?", options.TurnID)
	}
	if len(options.CanonicalRefs) > 0 {
		query = query.Where("canonical_ref IN ?", compactNonEmptyStrings(options.CanonicalRefs))
	}
	if len(options.FactTypes) > 0 {
		query = query.Where("fact_type IN ?", factTypeStrings(options.FactTypes))
	}
	if len(options.MemoryKinds) > 0 {
		query = query.Where("memory_kind IN ?", memoryKindStrings(options.MemoryKinds))
	}
	if options.MinImportance > 0 {
		query = query.Where("importance >= ?", options.MinImportance)
	}
	if options.After != nil {
		query = query.Where("created_at >= ?", options.After.UTC())
	}
	if options.Before != nil {
		query = query.Where("created_at <= ?", options.Before.UTC())
	}
	searchTerms := compactNonEmptyStrings(append([]string{options.Query}, options.Keywords...))
	for _, term := range searchTerms {
		like := "%" + escapeLike(term) + "%"
		query = query.Where("(summary_for_index ILIKE ? ESCAPE '\\' OR contextual_text ILIKE ? ESCAPE '\\')", like, like)
	}
	if options.Offset > 0 {
		query = query.Offset(options.Offset)
	}
	var models []agentFactArchiveIndexModel
	if err := query.
		Order("importance DESC, confidence DESC, updated_at DESC, id DESC").
		Limit(limit).
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	facts := make([]domain.AgentFactArchiveIndex, 0, len(models))
	ids := make([]int64, 0, len(models))
	for _, model := range models {
		facts = append(facts, agentFactArchiveIndexModelToDomain(model))
		ids = append(ids, model.ID)
	}
	r.touchAgentFactArchiveIndexes(ctx, ids)
	return facts, nil
}

func (r *AgentRepository) CreateAgentMemoryCandidate(ctx context.Context, candidate domain.AgentMemoryCandidate) (domain.AgentMemoryCandidate, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory.candidate.create", "insert", "agent_memory_candidates")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentMemoryCandidateModelFromDomain(normalizeAgentMemoryCandidate(candidate))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentMemoryCandidate{}, opErr
	}
	created := agentMemoryCandidateModelToDomain(model)
	_, _ = r.CreateAgentMemoryEvent(ctx, domain.AgentMemoryEvent{
		UserID:      created.UserID,
		SessionID:   created.SessionID,
		TurnID:      created.TurnID,
		CandidateID: created.ID,
		EventType:   domain.AgentMemoryEventCandidateGenerated,
		ActorType:   domain.AgentMemoryActorSystem,
		Reason:      "candidate generated",
		Payload: domain.AgentJSON{
			"status":      string(created.Status),
			"memory_kind": string(created.MemoryKind),
			"risk_level":  string(created.RiskLevel),
		},
		CreatedAt: time.Now().UTC(),
	})
	return created, nil
}

func (r *AgentRepository) ListAgentMemoryCandidates(ctx context.Context, userID int64, status domain.AgentMemoryCandidateStatus, limit int) ([]domain.AgentMemoryCandidate, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory.candidate.list", "select", "agent_memory_candidates")
	var opErr error
	defer func() { finish(opErr) }()

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	query := r.db.WithContext(ctx).Model(&agentMemoryCandidateModel{}).Where("user_id = ?", userID)
	if status.Valid() {
		query = query.Where("status = ?", string(status))
	}
	var models []agentMemoryCandidateModel
	if err := query.Order("updated_at DESC, id DESC").Limit(limit).Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	output := make([]domain.AgentMemoryCandidate, 0, len(models))
	for _, model := range models {
		output = append(output, agentMemoryCandidateModelToDomain(model))
	}
	return output, nil
}

func (r *AgentRepository) UpdateAgentMemoryCandidateStatus(ctx context.Context, userID int64, candidateID int64, status domain.AgentMemoryCandidateStatus, reason string, now time.Time) (domain.AgentMemoryCandidate, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory.candidate.update_status", "update", "agent_memory_candidates")
	var opErr error
	defer func() { finish(opErr) }()

	if !status.Valid() {
		opErr = domain.ErrInvalidInput
		return domain.AgentMemoryCandidate{}, opErr
	}
	var model agentMemoryCandidateModel
	result := r.db.WithContext(ctx).
		Model(&model).
		Clauses(clause.Returning{}).
		Where("id = ? AND user_id = ?", candidateID, userID).
		Updates(map[string]any{
			"status":      string(status),
			"reviewed_at": now.UTC(),
			"updated_at":  now.UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentMemoryCandidate{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentMemoryCandidate{}, opErr
	}
	candidate := agentMemoryCandidateModelToDomain(model)
	eventType := domain.AgentMemoryEventCandidateRejected
	if status == domain.AgentMemoryCandidateRevoked {
		eventType = domain.AgentMemoryEventCandidateRevoked
	}
	_, _ = r.CreateAgentMemoryEvent(ctx, domain.AgentMemoryEvent{
		UserID:      userID,
		SessionID:   candidate.SessionID,
		TurnID:      candidate.TurnID,
		CandidateID: candidate.ID,
		EventType:   eventType,
		ActorType:   domain.AgentMemoryActorUser,
		ActorUserID: userID,
		Reason:      reason,
		Payload:     domain.AgentJSON{"status": string(status)},
		CreatedAt:   now.UTC(),
	})
	return candidate, nil
}

func (r *AgentRepository) ApplyAgentMemoryCandidate(ctx context.Context, userID int64, candidateID int64, now time.Time) (domain.AgentMemoryBlock, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory.candidate.apply", "insert", "agent_memory_blocks")
	var opErr error
	defer func() { finish(opErr) }()

	var block domain.AgentMemoryBlock
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidateModel agentMemoryCandidateModel
		if err := tx.Where("id = ? AND user_id = ?", candidateID, userID).First(&candidateModel).Error; err != nil {
			return mapRepositoryError(err)
		}
		candidate := agentMemoryCandidateModelToDomain(candidateModel)
		if candidate.Status == domain.AgentMemoryCandidateRejected || candidate.Status == domain.AgentMemoryCandidateRevoked || candidate.Status == domain.AgentMemoryCandidateExpired {
			return domain.ErrInvalidInput
		}
		blockModel := agentMemoryBlockModelFromDomain(normalizeAgentMemoryBlock(domain.AgentMemoryBlock{
			UserID:            userID,
			MemoryKind:        candidate.MemoryKind,
			Title:             candidate.Summary,
			Content:           candidate.CandidateText,
			Summary:           candidate.Summary,
			EvidenceRefs:      candidate.EvidenceRefs,
			SourceCandidateID: candidate.ID,
			Confidence:        candidate.Confidence,
			Importance:        candidate.Importance,
			Status:            domain.AgentMemoryBlockActive,
			Version:           1,
			Metadata: domain.AgentJSON{
				"source_refs": candidate.SourceRefs,
				"risk_level":  string(candidate.RiskLevel),
			},
			CreatedAt: now.UTC(),
			UpdatedAt: now.UTC(),
		}))
		if err := tx.Create(&blockModel).Error; err != nil {
			return mapRepositoryError(err)
		}
		if err := tx.Model(&agentMemoryCandidateModel{}).
			Where("id = ? AND user_id = ?", candidate.ID, userID).
			Updates(map[string]any{
				"status":          string(domain.AgentMemoryCandidateApplied),
				"memory_block_id": blockModel.ID,
				"reviewed_at":     now.UTC(),
				"updated_at":      now.UTC(),
			}).Error; err != nil {
			return mapRepositoryError(err)
		}
		events := []agentMemoryEventModel{
			agentMemoryEventModelFromDomain(normalizeAgentMemoryEvent(domain.AgentMemoryEvent{
				UserID:        userID,
				SessionID:     candidate.SessionID,
				TurnID:        candidate.TurnID,
				CandidateID:   candidate.ID,
				MemoryBlockID: blockModel.ID,
				EventType:     domain.AgentMemoryEventCandidateApplied,
				ActorType:     domain.AgentMemoryActorSystem,
				Reason:        "candidate applied to memory block",
				Payload:       domain.AgentJSON{"candidate_status": string(domain.AgentMemoryCandidateApplied)},
				CreatedAt:     now.UTC(),
			})),
			agentMemoryEventModelFromDomain(normalizeAgentMemoryEvent(domain.AgentMemoryEvent{
				UserID:        userID,
				SessionID:     candidate.SessionID,
				TurnID:        candidate.TurnID,
				CandidateID:   candidate.ID,
				MemoryBlockID: blockModel.ID,
				EventType:     domain.AgentMemoryEventMemoryCreated,
				ActorType:     domain.AgentMemoryActorSystem,
				Reason:        "memory block created",
				Payload:       domain.AgentJSON{"memory_kind": string(candidate.MemoryKind)},
				CreatedAt:     now.UTC(),
			})),
		}
		if err := tx.Create(&events).Error; err != nil {
			return mapRepositoryError(err)
		}
		block = agentMemoryBlockModelToDomain(blockModel)
		return nil
	})
	if err != nil {
		opErr = err
		return domain.AgentMemoryBlock{}, err
	}
	return block, nil
}

func (r *AgentRepository) ListAgentMemoryBlocks(ctx context.Context, options domain.AgentMemoryBlockQueryOptions) ([]domain.AgentMemoryBlock, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory.block.list", "select", "agent_memory_blocks")
	var opErr error
	defer func() { finish(opErr) }()

	limit := options.Limit
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}
	query := r.db.WithContext(ctx).Model(&agentMemoryBlockModel{}).Where("user_id = ?", options.UserID)
	if len(options.Statuses) > 0 {
		query = query.Where("status IN ?", memoryBlockStatusStrings(options.Statuses))
	} else {
		query = query.Where("status = ?", string(domain.AgentMemoryBlockActive))
	}
	if len(options.MemoryKinds) > 0 {
		query = query.Where("memory_kind IN ?", memoryKindStrings(options.MemoryKinds))
	}
	if strings.TrimSpace(options.Query) != "" {
		like := "%" + escapeLike(options.Query) + "%"
		query = query.Where("(title ILIKE ? ESCAPE '\\' OR content ILIKE ? ESCAPE '\\' OR summary ILIKE ? ESCAPE '\\')", like, like, like)
	}
	if options.Offset > 0 {
		query = query.Offset(options.Offset)
	}
	var models []agentMemoryBlockModel
	if err := query.Order("importance DESC, updated_at DESC, id DESC").Limit(limit).Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	blocks := make([]domain.AgentMemoryBlock, 0, len(models))
	ids := make([]int64, 0, len(models))
	for _, model := range models {
		blocks = append(blocks, agentMemoryBlockModelToDomain(model))
		ids = append(ids, model.ID)
	}
	r.touchAgentMemoryBlocks(ctx, options.UserID, ids)
	return blocks, nil
}

func (r *AgentRepository) CreateAgentMemoryEvent(ctx context.Context, event domain.AgentMemoryEvent) (domain.AgentMemoryEvent, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory.event.create", "insert", "agent_memory_events")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentMemoryEventModelFromDomain(normalizeAgentMemoryEvent(event))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentMemoryEvent{}, opErr
	}
	return agentMemoryEventModelToDomain(model), nil
}

func (r *AgentRepository) touchAgentFactArchiveIndexes(ctx context.Context, ids []int64) {
	if r == nil || r.db == nil || len(ids) == 0 {
		return
	}
	now := time.Now().UTC()
	_ = r.db.WithContext(ctx).
		Model(&agentFactArchiveIndexModel{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"last_accessed_at": now,
			"access_count":     gorm.Expr("access_count + ?", 1),
			"updated_at":       now,
		}).Error
}

func (r *AgentRepository) touchAgentMemoryBlocks(ctx context.Context, userID int64, ids []int64) {
	if r == nil || r.db == nil || userID == 0 || len(ids) == 0 {
		return
	}
	now := time.Now().UTC()
	_ = r.db.WithContext(ctx).
		Model(&agentMemoryBlockModel{}).
		Where("user_id = ? AND id IN ?", userID, ids).
		Updates(map[string]any{
			"last_used_at": now,
			"use_count":    gorm.Expr("use_count + ?", 1),
			"updated_at":   now,
		}).Error
}

func normalizeAgentFactArchiveIndex(fact domain.AgentFactArchiveIndex) domain.AgentFactArchiveIndex {
	fact.CanonicalRef = strings.TrimSpace(fact.CanonicalRef)
	if !fact.FactType.Valid() {
		fact.FactType = domain.AgentFactTypeTranscript
	}
	if !fact.MemoryKind.Valid() {
		fact.MemoryKind = domain.AgentMemoryKindUnknown
	}
	if !fact.IndexStatus.Valid() {
		fact.IndexStatus = domain.AgentFactIndexStatusReady
	}
	if !fact.RiskLevel.Valid() {
		fact.RiskLevel = domain.AgentMemoryRiskLow
	}
	fact.Topics = compactNonEmptyStrings(fact.Topics)
	fact.Keywords = compactNonEmptyStrings(fact.Keywords)
	fact.Entities = compactNonEmptyStrings(fact.Entities)
	fact.SummaryForIndex = strings.TrimSpace(fact.SummaryForIndex)
	fact.ContextualText = strings.TrimSpace(fact.ContextualText)
	fact.SourceRefs = compactNonEmptyStrings(fact.SourceRefs)
	fact.RelationRefs = compactNonEmptyStrings(fact.RelationRefs)
	if fact.Embedding == nil {
		fact.Embedding = domain.AgentJSON{}
	}
	if fact.Metadata == nil {
		fact.Metadata = domain.AgentJSON{}
	}
	if fact.Importance < 0 {
		fact.Importance = 0
	}
	if fact.Importance > 100 {
		fact.Importance = 100
	}
	fact.Confidence = clampConfidence(fact.Confidence)
	return fact
}

func normalizeAgentMemoryCandidate(candidate domain.AgentMemoryCandidate) domain.AgentMemoryCandidate {
	if !candidate.MemoryKind.Valid() {
		candidate.MemoryKind = domain.AgentMemoryKindUnknown
	}
	candidate.CandidateText = strings.TrimSpace(candidate.CandidateText)
	candidate.Summary = strings.TrimSpace(candidate.Summary)
	if candidate.Summary == "" {
		candidate.Summary = safeTextPrefix(candidate.CandidateText, 160)
	}
	candidate.EvidenceRefs = compactNonEmptyStrings(candidate.EvidenceRefs)
	candidate.SourceRefs = compactNonEmptyStrings(candidate.SourceRefs)
	candidate.Confidence = clampConfidence(candidate.Confidence)
	if candidate.Importance < 0 {
		candidate.Importance = 0
	}
	if candidate.Importance > 100 {
		candidate.Importance = 100
	}
	if !candidate.RiskLevel.Valid() {
		candidate.RiskLevel = domain.AgentMemoryRiskLow
	}
	if !candidate.Status.Valid() {
		if candidate.RiskLevel == domain.AgentMemoryRiskHigh {
			candidate.Status = domain.AgentMemoryCandidateRequiresConfirmation
		} else {
			candidate.Status = domain.AgentMemoryCandidatePending
		}
	}
	candidate.ProposedBy = strings.TrimSpace(candidate.ProposedBy)
	if candidate.ProposedBy == "" {
		candidate.ProposedBy = "system"
	}
	if candidate.Metadata == nil {
		candidate.Metadata = domain.AgentJSON{}
	}
	return candidate
}

func normalizeAgentMemoryBlock(block domain.AgentMemoryBlock) domain.AgentMemoryBlock {
	if !block.MemoryKind.Valid() {
		block.MemoryKind = domain.AgentMemoryKindPreference
	}
	block.Title = strings.TrimSpace(block.Title)
	block.Content = strings.TrimSpace(block.Content)
	block.Summary = strings.TrimSpace(block.Summary)
	if block.Summary == "" {
		block.Summary = safeTextPrefix(block.Content, 160)
	}
	if block.Title == "" {
		block.Title = block.Summary
	}
	block.EvidenceRefs = compactNonEmptyStrings(block.EvidenceRefs)
	block.Confidence = clampConfidence(block.Confidence)
	if block.Importance < 0 {
		block.Importance = 0
	}
	if block.Importance > 100 {
		block.Importance = 100
	}
	if !block.Status.Valid() {
		block.Status = domain.AgentMemoryBlockActive
	}
	if block.Version <= 0 {
		block.Version = 1
	}
	if block.Metadata == nil {
		block.Metadata = domain.AgentJSON{}
	}
	return block
}

func normalizeAgentMemoryEvent(event domain.AgentMemoryEvent) domain.AgentMemoryEvent {
	if !event.EventType.Valid() {
		event.EventType = domain.AgentMemoryEventCandidateGenerated
	}
	if !event.ActorType.Valid() {
		event.ActorType = domain.AgentMemoryActorSystem
	}
	event.Reason = strings.TrimSpace(event.Reason)
	if event.Payload == nil {
		event.Payload = domain.AgentJSON{}
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	return event
}

func agentFactArchiveIndexModelFromDomain(fact domain.AgentFactArchiveIndex) agentFactArchiveIndexModel {
	return agentFactArchiveIndexModel{
		ID:              fact.ID,
		CanonicalRef:    fact.CanonicalRef,
		FactType:        string(fact.FactType),
		FactID:          fact.FactID,
		UserID:          fact.UserID,
		SessionID:       int64Pointer(fact.SessionID),
		TurnID:          int64Pointer(fact.TurnID),
		MemoryKind:      string(fact.MemoryKind),
		Topics:          cloneStringSlice(fact.Topics),
		Keywords:        cloneStringSlice(fact.Keywords),
		Entities:        cloneStringSlice(fact.Entities),
		SummaryForIndex: fact.SummaryForIndex,
		ContextualText:  fact.ContextualText,
		Embedding:       cloneAgentJSON(fact.Embedding),
		Importance:      fact.Importance,
		Confidence:      fact.Confidence,
		SourceRefs:      cloneStringSlice(fact.SourceRefs),
		RelationRefs:    cloneStringSlice(fact.RelationRefs),
		IndexStatus:     string(fact.IndexStatus),
		RiskLevel:       string(fact.RiskLevel),
		AccessCount:     fact.AccessCount,
		LastAccessedAt:  fact.LastAccessedAt,
		Metadata:        cloneAgentJSON(fact.Metadata),
		CreatedAt:       fact.CreatedAt,
		UpdatedAt:       fact.UpdatedAt,
	}
}

func agentFactArchiveIndexModelToDomain(model agentFactArchiveIndexModel) domain.AgentFactArchiveIndex {
	return domain.AgentFactArchiveIndex{
		ID:              model.ID,
		CanonicalRef:    model.CanonicalRef,
		FactType:        domain.AgentFactType(model.FactType),
		FactID:          model.FactID,
		UserID:          model.UserID,
		SessionID:       int64Value(model.SessionID),
		TurnID:          int64Value(model.TurnID),
		MemoryKind:      domain.AgentMemoryKind(model.MemoryKind),
		Topics:          cloneStringSlice(model.Topics),
		Keywords:        cloneStringSlice(model.Keywords),
		Entities:        cloneStringSlice(model.Entities),
		SummaryForIndex: model.SummaryForIndex,
		ContextualText:  model.ContextualText,
		Embedding:       cloneAgentJSON(model.Embedding),
		Importance:      model.Importance,
		Confidence:      model.Confidence,
		SourceRefs:      cloneStringSlice(model.SourceRefs),
		RelationRefs:    cloneStringSlice(model.RelationRefs),
		IndexStatus:     domain.AgentFactIndexStatus(model.IndexStatus),
		RiskLevel:       domain.AgentMemoryRiskLevel(model.RiskLevel),
		AccessCount:     model.AccessCount,
		LastAccessedAt:  model.LastAccessedAt,
		Metadata:        cloneAgentJSON(model.Metadata),
		CreatedAt:       model.CreatedAt,
		UpdatedAt:       model.UpdatedAt,
	}
}

func agentMemoryCandidateModelFromDomain(candidate domain.AgentMemoryCandidate) agentMemoryCandidateModel {
	return agentMemoryCandidateModel{
		ID:             candidate.ID,
		UserID:         candidate.UserID,
		SessionID:      int64Pointer(candidate.SessionID),
		TurnID:         int64Pointer(candidate.TurnID),
		MemoryKind:     string(candidate.MemoryKind),
		CandidateText:  candidate.CandidateText,
		Summary:        candidate.Summary,
		EvidenceRefs:   cloneStringSlice(candidate.EvidenceRefs),
		SourceRefs:     cloneStringSlice(candidate.SourceRefs),
		Confidence:     candidate.Confidence,
		Importance:     candidate.Importance,
		RiskLevel:      string(candidate.RiskLevel),
		Status:         string(candidate.Status),
		ProposedBy:     candidate.ProposedBy,
		ExpiresAt:      candidate.ExpiresAt,
		ReviewedAt:     candidate.ReviewedAt,
		ReviewerUserID: int64Pointer(candidate.ReviewerUserID),
		MemoryBlockID:  int64Pointer(candidate.MemoryBlockID),
		Metadata:       cloneAgentJSON(candidate.Metadata),
		CreatedAt:      candidate.CreatedAt,
		UpdatedAt:      candidate.UpdatedAt,
	}
}

func agentMemoryCandidateModelToDomain(model agentMemoryCandidateModel) domain.AgentMemoryCandidate {
	return domain.AgentMemoryCandidate{
		ID:             model.ID,
		UserID:         model.UserID,
		SessionID:      int64Value(model.SessionID),
		TurnID:         int64Value(model.TurnID),
		MemoryKind:     domain.AgentMemoryKind(model.MemoryKind),
		CandidateText:  model.CandidateText,
		Summary:        model.Summary,
		EvidenceRefs:   cloneStringSlice(model.EvidenceRefs),
		SourceRefs:     cloneStringSlice(model.SourceRefs),
		Confidence:     model.Confidence,
		Importance:     model.Importance,
		RiskLevel:      domain.AgentMemoryRiskLevel(model.RiskLevel),
		Status:         domain.AgentMemoryCandidateStatus(model.Status),
		ProposedBy:     model.ProposedBy,
		ExpiresAt:      model.ExpiresAt,
		ReviewedAt:     model.ReviewedAt,
		ReviewerUserID: int64Value(model.ReviewerUserID),
		MemoryBlockID:  int64Value(model.MemoryBlockID),
		Metadata:       cloneAgentJSON(model.Metadata),
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}

func agentMemoryBlockModelFromDomain(block domain.AgentMemoryBlock) agentMemoryBlockModel {
	return agentMemoryBlockModel{
		ID:                block.ID,
		UserID:            block.UserID,
		MemoryKind:        string(block.MemoryKind),
		Title:             block.Title,
		Content:           block.Content,
		Summary:           block.Summary,
		EvidenceRefs:      cloneStringSlice(block.EvidenceRefs),
		SourceCandidateID: int64Pointer(block.SourceCandidateID),
		Confidence:        block.Confidence,
		Importance:        block.Importance,
		Status:            string(block.Status),
		Version:           block.Version,
		LastUsedAt:        block.LastUsedAt,
		UseCount:          block.UseCount,
		Metadata:          cloneAgentJSON(block.Metadata),
		CreatedAt:         block.CreatedAt,
		UpdatedAt:         block.UpdatedAt,
	}
}

func agentMemoryBlockModelToDomain(model agentMemoryBlockModel) domain.AgentMemoryBlock {
	return domain.AgentMemoryBlock{
		ID:                model.ID,
		UserID:            model.UserID,
		MemoryKind:        domain.AgentMemoryKind(model.MemoryKind),
		Title:             model.Title,
		Content:           model.Content,
		Summary:           model.Summary,
		EvidenceRefs:      cloneStringSlice(model.EvidenceRefs),
		SourceCandidateID: int64Value(model.SourceCandidateID),
		Confidence:        model.Confidence,
		Importance:        model.Importance,
		Status:            domain.AgentMemoryBlockStatus(model.Status),
		Version:           model.Version,
		LastUsedAt:        model.LastUsedAt,
		UseCount:          model.UseCount,
		Metadata:          cloneAgentJSON(model.Metadata),
		CreatedAt:         model.CreatedAt,
		UpdatedAt:         model.UpdatedAt,
	}
}

func agentMemoryEventModelFromDomain(event domain.AgentMemoryEvent) agentMemoryEventModel {
	return agentMemoryEventModel{
		ID:            event.ID,
		UserID:        event.UserID,
		SessionID:     int64Pointer(event.SessionID),
		TurnID:        int64Pointer(event.TurnID),
		CandidateID:   int64Pointer(event.CandidateID),
		MemoryBlockID: int64Pointer(event.MemoryBlockID),
		EventType:     string(event.EventType),
		ActorType:     string(event.ActorType),
		ActorUserID:   int64Pointer(event.ActorUserID),
		Reason:        event.Reason,
		Payload:       cloneAgentJSON(event.Payload),
		CreatedAt:     event.CreatedAt,
	}
}

func agentMemoryEventModelToDomain(model agentMemoryEventModel) domain.AgentMemoryEvent {
	return domain.AgentMemoryEvent{
		ID:            model.ID,
		UserID:        model.UserID,
		SessionID:     int64Value(model.SessionID),
		TurnID:        int64Value(model.TurnID),
		CandidateID:   int64Value(model.CandidateID),
		MemoryBlockID: int64Value(model.MemoryBlockID),
		EventType:     domain.AgentMemoryEventType(model.EventType),
		ActorType:     domain.AgentMemoryActorType(model.ActorType),
		ActorUserID:   int64Value(model.ActorUserID),
		Reason:        model.Reason,
		Payload:       cloneAgentJSON(model.Payload),
		CreatedAt:     model.CreatedAt,
	}
}

func factTypeStrings(values []domain.AgentFactType) []string {
	output := make([]string, 0, len(values))
	for _, value := range values {
		if value.Valid() {
			output = append(output, string(value))
		}
	}
	return output
}

func memoryKindStrings(values []domain.AgentMemoryKind) []string {
	output := make([]string, 0, len(values))
	for _, value := range values {
		if value.Valid() {
			output = append(output, string(value))
		}
	}
	return output
}

func memoryBlockStatusStrings(values []domain.AgentMemoryBlockStatus) []string {
	output := make([]string, 0, len(values))
	for _, value := range values {
		if value.Valid() {
			output = append(output, string(value))
		}
	}
	return output
}

func compactNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	output := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		output = append(output, value)
	}
	return output
}

func clampConfidence(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func safeTextPrefix(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit])
}
