package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"messagefeed/internal/domain"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

type agentFactEmbeddingModel struct {
	ID                 int64 `gorm:"primaryKey"`
	CanonicalRef       string
	UserID             int64 `gorm:"not null"`
	EmbeddingModel     string
	EmbeddingDimension int
	ContentHash        string
	EmbeddingStatus    string
	ErrorMessage       string
	Metadata           domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type agentFactIndexJobModel struct {
	ID             int64 `gorm:"primaryKey"`
	JobType        string
	Scope          domain.AgentJSON `gorm:"column:scope_json;serializer:json;type:jsonb;not null"`
	Status         string
	Cursor         domain.AgentJSON `gorm:"column:cursor_json;serializer:json;type:jsonb;not null"`
	TotalCount     int
	ProcessedCount int
	FailedCount    int
	ErrorMessage   string
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type agentFactArchiveIndexVectorHitModel struct {
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
	VectorScore     float64 `gorm:"column:vector_score"`
}

func (agentFactEmbeddingModel) TableName() string { return "agent_fact_embeddings" }
func (agentFactIndexJobModel) TableName() string  { return "agent_fact_index_jobs" }

func (r *AgentRepository) UpsertAgentFactEmbedding(ctx context.Context, embedding domain.AgentFactEmbedding) (domain.AgentFactEmbedding, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_fact_embedding.upsert", "upsert", "agent_fact_embeddings")
	var opErr error
	defer func() { finish(opErr) }()

	embedding = normalizeAgentFactEmbedding(embedding)
	if embedding.CanonicalRef == "" || embedding.UserID == 0 || embedding.EmbeddingModel == "" || len(embedding.Vector) == 0 {
		opErr = domain.ErrInvalidInput
		return domain.AgentFactEmbedding{}, opErr
	}
	vectorLiteral := pgVectorLiteral(embedding.Vector)
	metadata, err := json.Marshal(embedding.Metadata)
	if err != nil {
		opErr = domain.ErrInvalidInput
		return domain.AgentFactEmbedding{}, opErr
	}
	now := time.Now().UTC()
	if embedding.CreatedAt.IsZero() {
		embedding.CreatedAt = now
	}
	if embedding.UpdatedAt.IsZero() {
		embedding.UpdatedAt = now
	}
	err = r.db.WithContext(ctx).Exec(`
		INSERT INTO agent_fact_embeddings (
			canonical_ref,
			user_id,
			embedding_model,
			embedding_dimension,
			content_hash,
			embedding,
			embedding_status,
			error_message,
			metadata_json,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?::vector, ?, ?, ?::jsonb, ?, ?)
		ON CONFLICT (canonical_ref, embedding_model, content_hash)
		DO UPDATE SET
			user_id = EXCLUDED.user_id,
			embedding_dimension = EXCLUDED.embedding_dimension,
			embedding = EXCLUDED.embedding,
			embedding_status = EXCLUDED.embedding_status,
			error_message = EXCLUDED.error_message,
			metadata_json = EXCLUDED.metadata_json,
			updated_at = EXCLUDED.updated_at
	`, embedding.CanonicalRef, embedding.UserID, embedding.EmbeddingModel, embedding.EmbeddingDimension, embedding.ContentHash, vectorLiteral, string(embedding.EmbeddingStatus), embedding.ErrorMessage, string(metadata), embedding.CreatedAt, embedding.UpdatedAt).Error
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentFactEmbedding{}, opErr
	}
	var stored agentFactEmbeddingModel
	if err := r.db.WithContext(ctx).
		Where("canonical_ref = ? AND embedding_model = ? AND content_hash = ?", embedding.CanonicalRef, embedding.EmbeddingModel, embedding.ContentHash).
		First(&stored).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentFactEmbedding{}, opErr
	}
	output := agentFactEmbeddingModelToDomain(stored)
	output.Vector = append([]float32(nil), embedding.Vector...)
	return output, nil
}

func (r *AgentRepository) SearchAgentFactEmbeddings(ctx context.Context, plan domain.AgentFactRecallPlan, queryVector []float32) ([]domain.AgentFactRecallHit, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_fact_embedding.search", "select", "agent_fact_embeddings")
	var opErr error
	defer func() { finish(opErr) }()

	if plan.UserID == 0 || len(queryVector) == 0 {
		opErr = domain.ErrInvalidInput
		return nil, opErr
	}
	limit := plan.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 80 {
		limit = 80
	}
	model := strings.TrimSpace(plan.EmbeddingModel)
	vectorLiteral := pgVectorLiteral(queryVector)
	query := r.db.WithContext(ctx).
		Table("agent_fact_archive_index AS i").
		Select("i.*, 1 - (e.embedding <=> ?::vector) AS vector_score", vectorLiteral).
		Joins("JOIN agent_fact_embeddings e ON e.canonical_ref = i.canonical_ref AND e.user_id = i.user_id").
		Where("i.user_id = ? AND e.user_id = ?", plan.UserID, plan.UserID).
		Where("i.index_status = ? AND e.embedding_status = ?", string(domain.AgentFactIndexStatusReady), string(domain.AgentFactEmbeddingStatusReady))
	if model != "" {
		query = query.Where("e.embedding_model = ?", model)
	}
	if plan.SessionID > 0 {
		query = query.Where("(i.session_id = ? OR i.session_id IS NULL)", plan.SessionID)
	}
	if plan.TurnID > 0 {
		query = query.Where("(i.turn_id IS NULL OR i.turn_id <= ?)", plan.TurnID)
	}
	if len(plan.FactTypes) > 0 {
		query = query.Where("i.fact_type IN ?", factTypeStrings(plan.FactTypes))
	}
	if len(plan.MemoryKinds) > 0 {
		query = query.Where("i.memory_kind IN ?", memoryKindStrings(plan.MemoryKinds))
	}
	if plan.MaxRiskLevel.Valid() {
		query = query.Where("i.risk_level IN ?", allowedAgentMemoryRiskLevels(plan.MaxRiskLevel))
	}
	if plan.After != nil {
		query = query.Where("i.created_at >= ?", plan.After.UTC())
	}
	if plan.Before != nil {
		query = query.Where("i.created_at <= ?", plan.Before.UTC())
	}
	var rows []agentFactArchiveIndexVectorHitModel
	if err := query.Order(clause.Expr{SQL: "e.embedding <=> ?::vector", Vars: []any{vectorLiteral}}).
		Limit(limit).
		Find(&rows).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	hits := make([]domain.AgentFactRecallHit, 0, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		fact := agentFactArchiveIndexModelToDomain(row.agentFactArchiveIndexModel())
		hits = append(hits, domain.AgentFactRecallHit{
			Fact:         fact,
			CanonicalRef: fact.CanonicalRef,
			VectorScore:  clampUnitScore(row.VectorScore),
			HitSources:   []string{"vector"},
		})
		ids = append(ids, fact.ID)
	}
	r.touchAgentFactArchiveIndexes(ctx, ids)
	return hits, nil
}

func (row agentFactArchiveIndexVectorHitModel) agentFactArchiveIndexModel() agentFactArchiveIndexModel {
	return agentFactArchiveIndexModel{
		ID:              row.ID,
		CanonicalRef:    row.CanonicalRef,
		FactType:        row.FactType,
		FactID:          row.FactID,
		UserID:          row.UserID,
		SessionID:       row.SessionID,
		TurnID:          row.TurnID,
		MemoryKind:      row.MemoryKind,
		Topics:          row.Topics,
		Keywords:        row.Keywords,
		Entities:        row.Entities,
		SummaryForIndex: row.SummaryForIndex,
		ContextualText:  row.ContextualText,
		Embedding:       row.Embedding,
		Importance:      row.Importance,
		Confidence:      row.Confidence,
		SourceRefs:      row.SourceRefs,
		RelationRefs:    row.RelationRefs,
		IndexStatus:     row.IndexStatus,
		RiskLevel:       row.RiskLevel,
		AccessCount:     row.AccessCount,
		LastAccessedAt:  row.LastAccessedAt,
		Metadata:        row.Metadata,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func (r *AgentRepository) CreateAgentFactIndexJob(ctx context.Context, job domain.AgentFactIndexJob) (domain.AgentFactIndexJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_fact_index_job.create", "insert", "agent_fact_index_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentFactIndexJobModelFromDomain(normalizeAgentFactIndexJob(job))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentFactIndexJob{}, opErr
	}
	return agentFactIndexJobModelToDomain(model), nil
}

func (r *AgentRepository) UpdateAgentFactIndexJob(ctx context.Context, job domain.AgentFactIndexJob) (domain.AgentFactIndexJob, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_fact_index_job.update", "update", "agent_fact_index_jobs")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentFactIndexJobModelFromDomain(normalizeAgentFactIndexJob(job))
	result := r.db.WithContext(ctx).
		Model(&agentFactIndexJobModel{}).
		Where("id = ?", job.ID).
		Updates(map[string]any{
			"status":          model.Status,
			"cursor_json":     model.Cursor,
			"total_count":     model.TotalCount,
			"processed_count": model.ProcessedCount,
			"failed_count":    model.FailedCount,
			"error_message":   model.ErrorMessage,
			"started_at":      model.StartedAt,
			"finished_at":     model.FinishedAt,
			"updated_at":      time.Now().UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentFactIndexJob{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentFactIndexJob{}, opErr
	}
	var stored agentFactIndexJobModel
	if err := r.db.WithContext(ctx).Where("id = ?", job.ID).First(&stored).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentFactIndexJob{}, opErr
	}
	return agentFactIndexJobModelToDomain(stored), nil
}

func (r *AgentRepository) GetAgentFactIndexStats(ctx context.Context, userID int64) (domain.AgentFactIndexStats, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent_fact_index.stats", "select", "agent_fact_archive_index")
	var opErr error
	defer func() { finish(opErr) }()

	if userID == 0 {
		opErr = domain.ErrInvalidInput
		return domain.AgentFactIndexStats{}, opErr
	}
	stats := domain.AgentFactIndexStats{
		UserID:       userID,
		ByFactType:   map[string]int64{},
		ByMemoryKind: map[string]int64{},
	}
	base := r.db.WithContext(ctx).Model(&agentFactArchiveIndexModel{}).Where("user_id = ?", userID)
	if err := base.Count(&stats.FactIndexCount).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentFactIndexStats{}, opErr
	}
	_ = base.Where("index_status = ?", string(domain.AgentFactIndexStatusReady)).Count(&stats.ReadyCount).Error
	_ = base.Where("index_status = ?", string(domain.AgentFactIndexStatusPending)).Count(&stats.PendingCount).Error
	_ = base.Where("index_status = ?", string(domain.AgentFactIndexStatusFailed)).Count(&stats.FailedCount).Error
	_ = base.Where("index_status = ?", string(domain.AgentFactIndexStatusArchived)).Count(&stats.ArchivedCount).Error

	var lastIndexed sql.NullTime
	if err := r.db.WithContext(ctx).
		Table("agent_fact_archive_index").
		Select("MAX(updated_at)").
		Where("user_id = ?", userID).
		Row().
		Scan(&lastIndexed); err == nil && lastIndexed.Valid {
		value := lastIndexed.Time
		stats.LastIndexedAt = &value
	}
	var groups []struct {
		Key   string `gorm:"column:key"`
		Count int64  `gorm:"column:count"`
	}
	if err := r.db.WithContext(ctx).
		Table("agent_fact_archive_index").
		Select("fact_type AS key, COUNT(*) AS count").
		Where("user_id = ?", userID).
		Group("fact_type").
		Scan(&groups).Error; err == nil {
		for _, group := range groups {
			stats.ByFactType[group.Key] = group.Count
		}
	}
	groups = nil
	if err := r.db.WithContext(ctx).
		Table("agent_fact_archive_index").
		Select("memory_kind AS key, COUNT(*) AS count").
		Where("user_id = ?", userID).
		Group("memory_kind").
		Scan(&groups).Error; err == nil {
		for _, group := range groups {
			stats.ByMemoryKind[group.Key] = group.Count
		}
	}

	embeddingBase := r.db.WithContext(ctx).Model(&agentFactEmbeddingModel{}).Where("user_id = ?", userID)
	_ = embeddingBase.Count(&stats.EmbeddingCount).Error
	_ = embeddingBase.Where("embedding_status = ?", string(domain.AgentFactEmbeddingStatusReady)).Count(&stats.ReadyEmbeddingCount).Error
	_ = embeddingBase.Where("embedding_status = ?", string(domain.AgentFactEmbeddingStatusFailed)).Count(&stats.FailedEmbeddingCount).Error
	var lastEmbedded sql.NullTime
	if err := r.db.WithContext(ctx).
		Table("agent_fact_embeddings").
		Select("MAX(updated_at)").
		Where("user_id = ?", userID).
		Row().
		Scan(&lastEmbedded); err == nil && lastEmbedded.Valid {
		value := lastEmbedded.Time
		stats.LastEmbeddedAt = &value
	}
	return stats, nil
}

func normalizeAgentFactEmbedding(embedding domain.AgentFactEmbedding) domain.AgentFactEmbedding {
	embedding.CanonicalRef = strings.TrimSpace(embedding.CanonicalRef)
	embedding.EmbeddingModel = strings.TrimSpace(embedding.EmbeddingModel)
	embedding.ContentHash = strings.TrimSpace(embedding.ContentHash)
	if embedding.EmbeddingDimension <= 0 {
		embedding.EmbeddingDimension = len(embedding.Vector)
	}
	if !embedding.EmbeddingStatus.Valid() {
		embedding.EmbeddingStatus = domain.AgentFactEmbeddingStatusReady
	}
	if embedding.Metadata == nil {
		embedding.Metadata = domain.AgentJSON{}
	}
	return embedding
}

func agentFactEmbeddingModelToDomain(model agentFactEmbeddingModel) domain.AgentFactEmbedding {
	return domain.AgentFactEmbedding{
		ID:                 model.ID,
		CanonicalRef:       model.CanonicalRef,
		UserID:             model.UserID,
		EmbeddingModel:     model.EmbeddingModel,
		EmbeddingDimension: model.EmbeddingDimension,
		ContentHash:        model.ContentHash,
		EmbeddingStatus:    domain.AgentFactEmbeddingStatus(model.EmbeddingStatus),
		ErrorMessage:       model.ErrorMessage,
		Metadata:           cloneAgentJSON(model.Metadata),
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
	}
}

func normalizeAgentFactIndexJob(job domain.AgentFactIndexJob) domain.AgentFactIndexJob {
	if !job.JobType.Valid() {
		job.JobType = domain.AgentFactIndexJobBackfill
	}
	if !job.Status.Valid() {
		job.Status = domain.AgentFactIndexJobPending
	}
	if job.Scope == nil {
		job.Scope = domain.AgentJSON{}
	}
	if job.Cursor == nil {
		job.Cursor = domain.AgentJSON{}
	}
	job.ErrorMessage = strings.TrimSpace(job.ErrorMessage)
	return job
}

func agentFactIndexJobModelFromDomain(job domain.AgentFactIndexJob) agentFactIndexJobModel {
	return agentFactIndexJobModel{
		ID:             job.ID,
		JobType:        string(job.JobType),
		Scope:          cloneAgentJSON(job.Scope),
		Status:         string(job.Status),
		Cursor:         cloneAgentJSON(job.Cursor),
		TotalCount:     job.TotalCount,
		ProcessedCount: job.ProcessedCount,
		FailedCount:    job.FailedCount,
		ErrorMessage:   job.ErrorMessage,
		StartedAt:      job.StartedAt,
		FinishedAt:     job.FinishedAt,
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
	}
}

func agentFactIndexJobModelToDomain(model agentFactIndexJobModel) domain.AgentFactIndexJob {
	return domain.AgentFactIndexJob{
		ID:             model.ID,
		JobType:        domain.AgentFactIndexJobType(model.JobType),
		Scope:          cloneAgentJSON(model.Scope),
		Status:         domain.AgentFactIndexJobStatus(model.Status),
		Cursor:         cloneAgentJSON(model.Cursor),
		TotalCount:     model.TotalCount,
		ProcessedCount: model.ProcessedCount,
		FailedCount:    model.FailedCount,
		ErrorMessage:   model.ErrorMessage,
		StartedAt:      model.StartedAt,
		FinishedAt:     model.FinishedAt,
		CreatedAt:      model.CreatedAt,
		UpdatedAt:      model.UpdatedAt,
	}
}

func pgVectorLiteral(vector []float32) string {
	parts := make([]string, 0, len(vector))
	for _, value := range vector {
		parts = append(parts, strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func clampUnitScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}
