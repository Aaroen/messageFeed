package repository

import (
	"context"
	"encoding/json"
	"fmt"
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
	if options.MaxRiskLevel.Valid() {
		query = query.Where("risk_level IN ?", allowedAgentMemoryRiskLevels(options.MaxRiskLevel))
	}
	if options.After != nil {
		query = query.Where("created_at >= ?", options.After.UTC())
	}
	if options.Before != nil {
		query = query.Where("created_at <= ?", options.Before.UTC())
	}
	for _, ref := range compactNonEmptyStrings(options.RelationRefs) {
		encoded, _ := json.Marshal([]string{ref})
		query = query.Where("(relation_refs_json @> ?::jsonb OR source_refs_json @> ?::jsonb)", string(encoded), string(encoded))
	}
	searchTerms := compactNonEmptyStrings(append([]string{options.Query}, options.Keywords...))
	searchText := strings.TrimSpace(strings.Join(searchTerms, " "))
	orderExpr := any("importance DESC, confidence DESC, updated_at DESC, id DESC")
	if searchText != "" {
		like := "%" + escapeLike(searchText) + "%"
		query = query.Where("(full_text_vector @@ plainto_tsquery('simple', ?) OR summary_for_index ILIKE ? ESCAPE '\\' OR contextual_text ILIKE ? ESCAPE '\\')", searchText, like, like)
		orderExpr = clause.Expr{
			SQL:  "ts_rank(full_text_vector, plainto_tsquery('simple', ?)) DESC, importance DESC, confidence DESC, updated_at DESC, id DESC",
			Vars: []any{searchText},
		}
	}
	if options.Offset > 0 {
		query = query.Offset(options.Offset)
	}
	var models []agentFactArchiveIndexModel
	if err := query.
		Order(orderExpr).
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

func (r *AgentRepository) ResolveAgentFactSources(ctx context.Context, userID int64, facts []domain.AgentFactArchiveIndex) ([]domain.AgentFactSource, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_memory.fact.resolve", "select", "agent_fact_sources")
	var opErr error
	defer func() { finish(opErr) }()

	sources := make([]domain.AgentFactSource, 0, len(facts))
	for _, fact := range facts {
		if fact.UserID != userID || fact.FactID == 0 {
			continue
		}
		source, err := r.resolveAgentFactSource(ctx, userID, fact)
		if err != nil {
			if err == domain.ErrNotFound {
				continue
			}
			opErr = err
			return nil, err
		}
		source = applyFactChunkProjection(fact, source)
		sources = append(sources, source)
	}
	return sources, nil
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

type agentFactRunScope struct {
	UserID    int64
	SessionID int64
	TurnID    int64
}

func (r *AgentRepository) indexTranscriptFact(ctx context.Context, entry domain.AgentTranscriptEntry) {
	if r == nil || r.db == nil || entry.ID == 0 || entry.UserID == 0 {
		return
	}
	if fact, ok := newAgentFactIndexBuilder(time.Now).BuildTranscript(entry); ok {
		_, _ = r.UpsertAgentFactArchiveIndex(ctx, fact)
	}
}

func (r *AgentRepository) indexObservationFact(ctx context.Context, observation domain.AgentObservation) {
	scope, err := r.agentRunScope(ctx, observation.RunID)
	if err != nil || scope.UserID == 0 {
		return
	}
	if fact, ok := newAgentFactIndexBuilder(time.Now).BuildObservation(scope, observation); ok {
		_, _ = r.UpsertAgentFactArchiveIndex(ctx, fact)
	}
}

func (r *AgentRepository) indexArtifactFact(ctx context.Context, artifact domain.AgentArtifact) {
	scope, err := r.agentRunScope(ctx, artifact.RunID)
	if err != nil || scope.UserID == 0 {
		return
	}
	if fact, ok := newAgentFactIndexBuilder(time.Now).BuildArtifact(scope, artifact); ok {
		_, _ = r.UpsertAgentFactArchiveIndex(ctx, fact)
	}
}

func (r *AgentRepository) indexPlanFact(ctx context.Context, plan domain.AgentPlan) {
	if plan.ID == 0 || plan.UserID == 0 {
		return
	}
	if fact, ok := newAgentFactIndexBuilder(time.Now).BuildPlan(plan); ok {
		_, _ = r.UpsertAgentFactArchiveIndex(ctx, fact)
	}
}

func (r *AgentRepository) indexPlanStepFact(ctx context.Context, userID int64, step domain.AgentPlanStep) {
	scope, err := r.planStepScope(ctx, userID, step.PlanID)
	if err != nil || scope.UserID == 0 {
		return
	}
	if fact, ok := newAgentFactIndexBuilder(time.Now).BuildPlanStep(scope, step); ok {
		_, _ = r.UpsertAgentFactArchiveIndex(ctx, fact)
	}
}

func (r *AgentRepository) indexRunContextTraceFact(ctx context.Context, trace domain.AgentRunContextTrace) {
	scope, err := r.agentRunScope(ctx, trace.RunID)
	if err != nil || scope.UserID == 0 {
		return
	}
	if fact, ok := newAgentFactIndexBuilder(time.Now).BuildRunContextTrace(scope, trace); ok {
		_, _ = r.UpsertAgentFactArchiveIndex(ctx, fact)
	}
}

type AgentFactBackfillResult struct {
	ProcessedCount int
	FailedCount    int
}

func (r *AgentRepository) BackfillAgentFactArchiveIndex(ctx context.Context, limit int) (AgentFactBackfillResult, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_fact_archive.backfill", "upsert", "agent_fact_archive_index")
	var opErr error
	defer func() { finish(opErr) }()

	if limit <= 0 {
		limit = 500
	}
	result := AgentFactBackfillResult{}
	startedAt := time.Now().UTC()
	job, _ := r.CreateAgentFactIndexJob(ctx, domain.AgentFactIndexJob{
		JobType:   domain.AgentFactIndexJobBackfill,
		Status:    domain.AgentFactIndexJobRunning,
		Scope:     domain.AgentJSON{"limit": limit},
		StartedAt: &startedAt,
		CreatedAt: startedAt,
		UpdatedAt: startedAt,
	})
	result.ProcessedCount += r.backfillTranscriptFacts(ctx, limit, &result)
	result.ProcessedCount += r.backfillObservationFacts(ctx, limit, &result)
	result.ProcessedCount += r.backfillArtifactFacts(ctx, limit, &result)
	result.ProcessedCount += r.backfillPlanFacts(ctx, limit, &result)
	result.ProcessedCount += r.backfillPlanStepFacts(ctx, limit, &result)
	result.ProcessedCount += r.backfillRunContextTraceFacts(ctx, limit, &result)
	if job.ID > 0 {
		now := time.Now().UTC()
		job.ProcessedCount = result.ProcessedCount
		job.FailedCount = result.FailedCount
		job.FinishedAt = &now
		job.Status = domain.AgentFactIndexJobSucceeded
		if result.FailedCount > 0 {
			job.Status = domain.AgentFactIndexJobFailed
		}
		_, _ = r.UpdateAgentFactIndexJob(ctx, job)
	}
	return result, nil
}

func (r *AgentRepository) backfillTranscriptFacts(ctx context.Context, limit int, result *AgentFactBackfillResult) int {
	processed := 0
	var lastID int64
	for {
		var models []agentTranscriptEntryModel
		if err := r.db.WithContext(ctx).Where("id > ?", lastID).Order("id ASC").Limit(limit).Find(&models).Error; err != nil {
			result.FailedCount++
			return processed
		}
		if len(models) == 0 {
			return processed
		}
		for _, model := range models {
			r.indexTranscriptFact(ctx, agentTranscriptEntryModelToDomain(model))
			lastID = model.ID
			processed++
		}
	}
}

func (r *AgentRepository) backfillObservationFacts(ctx context.Context, limit int, result *AgentFactBackfillResult) int {
	processed := 0
	var lastID int64
	for {
		var models []agentObservationModel
		if err := r.db.WithContext(ctx).Where("id > ?", lastID).Order("id ASC").Limit(limit).Find(&models).Error; err != nil {
			result.FailedCount++
			return processed
		}
		if len(models) == 0 {
			return processed
		}
		for _, model := range models {
			r.indexObservationFact(ctx, agentObservationModelToDomain(model))
			lastID = model.ID
			processed++
		}
	}
}

func (r *AgentRepository) backfillArtifactFacts(ctx context.Context, limit int, result *AgentFactBackfillResult) int {
	processed := 0
	var lastID int64
	for {
		var models []agentArtifactModel
		if err := r.db.WithContext(ctx).Where("id > ?", lastID).Order("id ASC").Limit(limit).Find(&models).Error; err != nil {
			result.FailedCount++
			return processed
		}
		if len(models) == 0 {
			return processed
		}
		for _, model := range models {
			r.indexArtifactFact(ctx, agentArtifactModelToDomain(model))
			lastID = model.ID
			processed++
		}
	}
}

func (r *AgentRepository) backfillPlanFacts(ctx context.Context, limit int, result *AgentFactBackfillResult) int {
	processed := 0
	var lastID int64
	for {
		var models []agentPlanModel
		if err := r.db.WithContext(ctx).Where("id > ?", lastID).Order("id ASC").Limit(limit).Find(&models).Error; err != nil {
			result.FailedCount++
			return processed
		}
		if len(models) == 0 {
			return processed
		}
		for _, model := range models {
			r.indexPlanFact(ctx, agentPlanModelToDomain(model))
			lastID = model.ID
			processed++
		}
	}
}

func (r *AgentRepository) backfillPlanStepFacts(ctx context.Context, limit int, result *AgentFactBackfillResult) int {
	processed := 0
	var lastID int64
	for {
		var models []agentPlanStepModel
		if err := r.db.WithContext(ctx).Where("id > ?", lastID).Order("id ASC").Limit(limit).Find(&models).Error; err != nil {
			result.FailedCount++
			return processed
		}
		if len(models) == 0 {
			return processed
		}
		for _, model := range models {
			r.indexPlanStepFact(ctx, 0, agentPlanStepModelToDomain(model))
			lastID = model.ID
			processed++
		}
	}
}

func (r *AgentRepository) backfillRunContextTraceFacts(ctx context.Context, limit int, result *AgentFactBackfillResult) int {
	processed := 0
	var lastID int64
	for {
		var models []agentRunContextTraceModel
		if err := r.db.WithContext(ctx).Where("id > ?", lastID).Order("id ASC").Limit(limit).Find(&models).Error; err != nil {
			result.FailedCount++
			return processed
		}
		if len(models) == 0 {
			return processed
		}
		for _, model := range models {
			r.indexRunContextTraceFact(ctx, agentRunContextTraceModelToDomain(model))
			lastID = model.ID
			processed++
		}
	}
}

func (r *AgentRepository) agentRunScope(ctx context.Context, runID int64) (agentFactRunScope, error) {
	if runID == 0 {
		return agentFactRunScope{}, domain.ErrNotFound
	}
	var scope agentFactRunScope
	err := r.db.WithContext(ctx).
		Table("agent_runs").
		Select("COALESCE(agent_turns.user_id, agent_sessions.user_id, 0) AS user_id, COALESCE(agent_runs.session_id, agent_turns.session_id, agent_sessions.id, 0) AS session_id, COALESCE(agent_runs.turn_id, 0) AS turn_id").
		Joins("LEFT JOIN agent_turns ON agent_turns.id = agent_runs.turn_id").
		Joins("LEFT JOIN agent_sessions ON agent_sessions.id = agent_runs.session_id").
		Where("agent_runs.id = ?", runID).
		Scan(&scope).Error
	if err != nil {
		return agentFactRunScope{}, mapRepositoryError(err)
	}
	if scope.UserID == 0 {
		return agentFactRunScope{}, domain.ErrNotFound
	}
	return scope, nil
}

func (r *AgentRepository) planStepScope(ctx context.Context, userID int64, planID int64) (agentFactRunScope, error) {
	var scope agentFactRunScope
	query := r.db.WithContext(ctx).
		Table("agent_plans").
		Select("user_id, COALESCE(session_id, 0) AS session_id, COALESCE(turn_id, 0) AS turn_id").
		Where("id = ?", planID)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	err := query.Scan(&scope).Error
	if err != nil {
		return agentFactRunScope{}, mapRepositoryError(err)
	}
	if scope.UserID == 0 {
		return agentFactRunScope{}, domain.ErrNotFound
	}
	return scope, nil
}

func (r *AgentRepository) resolveAgentFactSource(ctx context.Context, userID int64, fact domain.AgentFactArchiveIndex) (domain.AgentFactSource, error) {
	switch fact.FactType {
	case domain.AgentFactTypeTranscript:
		var model agentTranscriptEntryModel
		if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", fact.FactID, userID).First(&model).Error; err != nil {
			return domain.AgentFactSource{}, mapRepositoryError(err)
		}
		entry := agentTranscriptEntryModelToDomain(model)
		return domain.AgentFactSource{
			CanonicalRef: fact.CanonicalRef,
			FactType:     fact.FactType,
			FactID:       fact.FactID,
			UserID:       entry.UserID,
			SessionID:    entry.SessionID,
			TurnID:       entry.TurnID,
			Title:        string(entry.Role),
			Content:      entry.Content,
			Summary:      safeTextPrefix(entry.Content, 320),
			SourceRefs:   []string{fact.CanonicalRef},
			Metadata:     cloneAgentJSON(entry.Metadata),
			CreatedAt:    entry.CreatedAt,
		}, nil
	case domain.AgentFactTypeObservation:
		var model agentObservationModel
		if err := r.userScopedObservations(ctx, userID).Where("agent_observations.id = ?", fact.FactID).First(&model).Error; err != nil {
			return domain.AgentFactSource{}, mapRepositoryError(err)
		}
		observation := agentObservationModelToDomain(model)
		return domain.AgentFactSource{
			CanonicalRef: fact.CanonicalRef,
			FactType:     fact.FactType,
			FactID:       fact.FactID,
			UserID:       userID,
			SessionID:    fact.SessionID,
			TurnID:       fact.TurnID,
			Title:        observation.CapabilityKey,
			Content:      strings.TrimSpace(strings.Join([]string{observation.InputSummary, observation.OutputSummary, observation.Error}, "\n")),
			Summary:      observation.OutputSummary,
			SourceRefs:   observation.ArtifactRefs,
			Metadata:     domain.AgentJSON{"status": observation.Status},
			CreatedAt:    observation.CreatedAt,
		}, nil
	case domain.AgentFactTypeArtifact:
		var model agentArtifactModel
		if err := r.userScopedArtifacts(ctx, userID).Where("agent_artifacts.id = ?", fact.FactID).First(&model).Error; err != nil {
			return domain.AgentFactSource{}, mapRepositoryError(err)
		}
		artifact := agentArtifactModelToDomain(model)
		return domain.AgentFactSource{
			CanonicalRef: fact.CanonicalRef,
			FactType:     fact.FactType,
			FactID:       fact.FactID,
			UserID:       userID,
			SessionID:    fact.SessionID,
			TurnID:       fact.TurnID,
			Title:        artifact.ArtifactType,
			Content:      strings.TrimSpace(strings.Join([]string{artifact.Summary, artifact.ContentRef}, "\n")),
			Summary:      artifact.Summary,
			SourceRefs:   artifact.SourceRefs,
			Metadata:     domain.AgentJSON{"content_hash": artifact.ContentHash},
			CreatedAt:    artifact.CreatedAt,
		}, nil
	case domain.AgentFactTypePlan:
		plan, err := r.GetAgentPlan(ctx, userID, fact.FactID)
		if err != nil {
			return domain.AgentFactSource{}, err
		}
		return domain.AgentFactSource{
			CanonicalRef: fact.CanonicalRef,
			FactType:     fact.FactType,
			FactID:       fact.FactID,
			UserID:       plan.UserID,
			SessionID:    plan.SessionID,
			TurnID:       plan.TurnID,
			Title:        plan.Goal,
			Content:      strings.TrimSpace(strings.Join([]string{plan.Goal, plan.Summary, plan.ImpactSummary, plan.ErrorMessage}, "\n")),
			Summary:      plan.Summary,
			SourceRefs:   []string{fact.CanonicalRef},
			Metadata:     cloneAgentJSON(plan.Metadata),
			CreatedAt:    plan.CreatedAt,
		}, nil
	case domain.AgentFactTypePlanStep:
		var model agentPlanStepModel
		if err := r.userScopedPlanSteps(ctx, userID).Where("agent_plan_steps.id = ?", fact.FactID).First(&model).Error; err != nil {
			return domain.AgentFactSource{}, mapRepositoryError(err)
		}
		step := agentPlanStepModelToDomain(model)
		return domain.AgentFactSource{
			CanonicalRef: fact.CanonicalRef,
			FactType:     fact.FactType,
			FactID:       fact.FactID,
			UserID:       userID,
			SessionID:    fact.SessionID,
			TurnID:       fact.TurnID,
			Title:        step.Title,
			Content:      strings.TrimSpace(strings.Join([]string{step.Title, step.InputSummary, step.OutputSummary, step.ExpectedOutput, step.ErrorMessage}, "\n")),
			Summary:      step.OutputSummary,
			SourceRefs:   compactNonEmptyStrings(append([]string{step.ObservationRef}, step.ArtifactRefs...)),
			Metadata:     domain.AgentJSON{"capability_key": step.CapabilityKey, "status": string(step.Status)},
			CreatedAt:    step.CreatedAt,
		}, nil
	case domain.AgentFactTypeRunTrace:
		var model agentRunContextTraceModel
		if err := r.userScopedRunContextTraces(ctx, userID).Where("agent_run_context_traces.id = ?", fact.FactID).First(&model).Error; err != nil {
			return domain.AgentFactSource{}, mapRepositoryError(err)
		}
		trace := agentRunContextTraceModelToDomain(model)
		contentBytes, _ := json.Marshal(trace.Content)
		content := strings.TrimSpace(string(contentBytes))
		return domain.AgentFactSource{
			CanonicalRef: fact.CanonicalRef,
			FactType:     fact.FactType,
			FactID:       fact.FactID,
			UserID:       userID,
			SessionID:    fact.SessionID,
			TurnID:       fact.TurnID,
			Title:        trace.TraceKind,
			Content:      content,
			Summary:      safeTextPrefix(content, 320),
			SourceRefs:   []string{fmt.Sprintf("run:%d", trace.RunID)},
			Metadata: domain.AgentJSON{
				"prompt_version":   trace.PromptVersion,
				"model_key":        trace.ModelKey,
				"redaction_status": trace.RedactionStatus,
				"content_hash":     trace.ContentHash,
			},
			CreatedAt: trace.CreatedAt,
		}, nil
	default:
		return domain.AgentFactSource{}, domain.ErrNotFound
	}
}

func (r *AgentRepository) userScopedObservations(ctx context.Context, userID int64) *gorm.DB {
	return r.db.WithContext(ctx).
		Model(&agentObservationModel{}).
		Joins("JOIN agent_runs ON agent_runs.id = agent_observations.run_id").
		Joins("LEFT JOIN agent_turns ON agent_turns.id = agent_runs.turn_id").
		Joins("LEFT JOIN agent_sessions ON agent_sessions.id = agent_runs.session_id").
		Where("(agent_turns.user_id = ? OR agent_sessions.user_id = ?)", userID, userID).
		Select("agent_observations.*")
}

func (r *AgentRepository) userScopedArtifacts(ctx context.Context, userID int64) *gorm.DB {
	return r.db.WithContext(ctx).
		Model(&agentArtifactModel{}).
		Joins("JOIN agent_runs ON agent_runs.id = agent_artifacts.run_id").
		Joins("LEFT JOIN agent_turns ON agent_turns.id = agent_runs.turn_id").
		Joins("LEFT JOIN agent_sessions ON agent_sessions.id = agent_runs.session_id").
		Where("(agent_turns.user_id = ? OR agent_sessions.user_id = ?)", userID, userID).
		Select("agent_artifacts.*")
}

func (r *AgentRepository) userScopedPlanSteps(ctx context.Context, userID int64) *gorm.DB {
	return r.db.WithContext(ctx).
		Model(&agentPlanStepModel{}).
		Joins("JOIN agent_plans ON agent_plans.id = agent_plan_steps.plan_id").
		Where("agent_plans.user_id = ?", userID).
		Select("agent_plan_steps.*")
}

func (r *AgentRepository) userScopedRunContextTraces(ctx context.Context, userID int64) *gorm.DB {
	return r.db.WithContext(ctx).
		Model(&agentRunContextTraceModel{}).
		Joins("JOIN agent_runs ON agent_runs.id = agent_run_context_traces.run_id").
		Joins("LEFT JOIN agent_turns ON agent_turns.id = agent_runs.turn_id").
		Joins("LEFT JOIN agent_sessions ON agent_sessions.id = agent_runs.session_id").
		Where("(agent_turns.user_id = ? OR agent_sessions.user_id = ?)", userID, userID).
		Select("agent_run_context_traces.*")
}

func applyFactChunkProjection(fact domain.AgentFactArchiveIndex, source domain.AgentFactSource) domain.AgentFactSource {
	if !strings.Contains(fact.CanonicalRef, "#chunk:") {
		return source
	}
	parentRef := strings.TrimSpace(stringFromAgentJSON(fact.Metadata, "parent_ref"))
	chunkText := strings.TrimSpace(stringFromAgentJSON(fact.Metadata, "chunk_text"))
	if chunkText == "" {
		chunkText = strings.TrimSpace(stringFromAgentJSON(fact.Metadata, "projection_text"))
	}
	if chunkText != "" {
		source.Content = chunkText
		source.Summary = safeTextPrefix(chunkText, 320)
	}
	if source.Metadata == nil {
		source.Metadata = domain.AgentJSON{}
	}
	source.Metadata["chunk_ref"] = fact.CanonicalRef
	if parentRef != "" {
		source.Metadata["parent_ref"] = parentRef
		source.SourceRefs = compactNonEmptyStrings(append([]string{parentRef, fact.CanonicalRef}, source.SourceRefs...))
	} else {
		source.SourceRefs = compactNonEmptyStrings(append([]string{fact.CanonicalRef}, source.SourceRefs...))
	}
	return source
}

func stringFromAgentJSON(values domain.AgentJSON, key string) string {
	if values == nil {
		return ""
	}
	switch value := values[key].(type) {
	case string:
		return value
	default:
		return ""
	}
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

func allowedAgentMemoryRiskLevels(maxRisk domain.AgentMemoryRiskLevel) []string {
	switch maxRisk {
	case domain.AgentMemoryRiskLow:
		return []string{string(domain.AgentMemoryRiskLow)}
	case domain.AgentMemoryRiskMedium:
		return []string{string(domain.AgentMemoryRiskLow), string(domain.AgentMemoryRiskMedium)}
	case domain.AgentMemoryRiskHigh:
		return []string{string(domain.AgentMemoryRiskLow), string(domain.AgentMemoryRiskMedium), string(domain.AgentMemoryRiskHigh)}
	default:
		return []string{string(domain.AgentMemoryRiskLow)}
	}
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

func confidenceForClassification(classification transcriptMemoryClassification) float64 {
	switch {
	case classification.Kind == domain.AgentMemoryKindUnknown:
		return 0.2
	case len(classification.Terms) >= 2:
		return 0.82
	case len(classification.Terms) == 1:
		return 0.72
	default:
		return 0.45
	}
}

func factImportanceForKind(kind domain.AgentMemoryKind, fallback int) int {
	importance := transcriptImportanceForKind(kind)
	if importance == 0 {
		importance = fallback
	}
	if importance < fallback && kind != domain.AgentMemoryKindCasual {
		importance = fallback
	}
	return importance
}

func inferMemoryRisk(content string, kind domain.AgentMemoryKind) domain.AgentMemoryRiskLevel {
	content = strings.ToLower(strings.TrimSpace(content))
	highRiskTerms := []string{"密码", "口令", "secret", "token", "api key", "apikey", "密钥", "身份证", "银行卡"}
	for _, term := range highRiskTerms {
		if strings.Contains(content, term) {
			return domain.AgentMemoryRiskHigh
		}
	}
	if kind == domain.AgentMemoryKindPreference || kind == domain.AgentMemoryKindFact {
		return domain.AgentMemoryRiskLow
	}
	return domain.AgentMemoryRiskLow
}
