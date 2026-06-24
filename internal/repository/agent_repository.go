package repository

import (
	"context"
	"messagefeed/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AgentRepository struct {
	db *gorm.DB
}

func NewAgentRepository(db *gorm.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

type externalAccountModel struct {
	ID                   int64 `gorm:"primaryKey"`
	UserID               int64 `gorm:"not null"`
	Provider             string
	CorpID               string
	AgentID              string
	ExternalUserID       string
	DisplayName          string
	BindingStatus        string
	ActiveAgentSessionID *int64
	VerifiedAt           *time.Time
	LastSeenAt           *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type agentInboundMessageModel struct {
	ID                int64 `gorm:"primaryKey"`
	UserID            int64 `gorm:"not null"`
	ExternalAccountID int64 `gorm:"not null"`
	Provider          string
	ProviderMessageID string
	CorpID            string
	AgentID           string
	ExternalUserID    string
	ChatID            string
	ChatType          string
	MsgType           string
	TextContent       string
	Payload           domain.AgentJSON `gorm:"column:payload_json;serializer:json;type:jsonb;not null"`
	RequestID         string
	TraceID           string
	Status            string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type agentSessionModel struct {
	ID                       int64 `gorm:"primaryKey"`
	UserID                   int64 `gorm:"not null"`
	ExternalAccountID        int64 `gorm:"not null"`
	Provider                 string
	ChannelSessionKey        string
	Status                   string
	Title                    string
	StartedAt                time.Time
	LastActiveAt             time.Time
	ContextInitializedAt     *time.Time
	ContextRebuildStartedAt  *time.Time
	ContextRebuildFinishedAt *time.Time
	ContextVersion           int64
	TranscriptCountIndexed   int64
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type agentTurnModel struct {
	ID               int64 `gorm:"primaryKey"`
	SessionID        int64 `gorm:"not null"`
	InboundMessageID int64 `gorm:"not null"`
	UserID           int64 `gorm:"not null"`
	Status           string
	InputText        string
	OutputText       string
	ModelProvider    string
	Model            string
	ErrorMessage     string
	StartedAt        time.Time
	FinishedAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type agentTranscriptEntryModel struct {
	ID        int64 `gorm:"primaryKey"`
	SessionID int64 `gorm:"not null"`
	TurnID    int64 `gorm:"not null"`
	UserID    int64 `gorm:"not null"`
	Role      string
	Content   string
	Metadata  domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt time.Time
}

type agentTranscriptArchiveIndexModel struct {
	ID                int64 `gorm:"primaryKey"`
	TranscriptEntryID int64 `gorm:"not null"`
	SessionID         int64 `gorm:"not null"`
	UserID            int64 `gorm:"not null"`
	ArchiveStatus     string
	MemoryKind        string
	Importance        int
	Keywords          []string         `gorm:"column:keywords_json;serializer:json;type:jsonb;not null"`
	LastAccessedAt    *time.Time       `gorm:"column:last_accessed_at"`
	AccessCount       int              `gorm:"not null"`
	Metadata          domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type agentRecallEventModel struct {
	ID           int64 `gorm:"primaryKey"`
	SessionID    int64
	TurnID       int64
	UserID       int64            `gorm:"not null"`
	Query        string           `gorm:"column:query_text"`
	QueryParams  domain.AgentJSON `gorm:"column:query_json;serializer:json;type:jsonb;not null"`
	RecalledRefs domain.AgentJSON `gorm:"column:recalled_refs_json;serializer:json;type:jsonb;not null"`
	Reason       string
	BudgetChars  int
	CreatedAt    time.Time
}

type agentAuditLogModel struct {
	ID        int64 `gorm:"primaryKey"`
	SessionID int64
	TurnID    int64
	UserID    int64 `gorm:"not null"`
	EventType string
	Status    string
	Message   string
	Metadata  domain.AgentJSON `gorm:"column:metadata_json;serializer:json;type:jsonb;not null"`
	RequestID string
	TraceID   string
	CreatedAt time.Time
}

func (externalAccountModel) TableName() string      { return "external_accounts" }
func (agentInboundMessageModel) TableName() string  { return "agent_inbound_messages" }
func (agentSessionModel) TableName() string         { return "agent_sessions" }
func (agentTurnModel) TableName() string            { return "agent_turns" }
func (agentTranscriptEntryModel) TableName() string { return "agent_transcript_entries" }
func (agentTranscriptArchiveIndexModel) TableName() string {
	return "agent_transcript_archive_index"
}
func (agentRecallEventModel) TableName() string { return "agent_recall_events" }
func (agentAuditLogModel) TableName() string    { return "agent_audit_logs" }

func (r *AgentRepository) EnsureExternalAccount(ctx context.Context, account domain.ExternalAccount) (domain.ExternalAccount, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.external_account.ensure", "upsert", "external_accounts")
	var opErr error
	defer func() { finish(opErr) }()

	account = normalizeExternalAccount(account)
	model := externalAccountModelFromDomain(account)
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "provider"},
				{Name: "corp_id"},
				{Name: "agent_id"},
				{Name: "external_user_id"},
			},
			DoUpdates: clause.Assignments(map[string]any{
				"user_id":      model.UserID,
				"display_name": model.DisplayName,
				"verified_at":  model.VerifiedAt,
				"last_seen_at": model.LastSeenAt,
				"updated_at":   time.Now().UTC(),
			}),
		}).
		Create(&model).Error
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.ExternalAccount{}, opErr
	}

	var persisted externalAccountModel
	if err := r.db.WithContext(ctx).
		Where("provider = ? AND corp_id = ? AND agent_id = ? AND external_user_id = ?", account.Provider, account.CorpID, account.AgentID, account.ExternalUserID).
		First(&persisted).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.ExternalAccount{}, opErr
	}
	return externalAccountModelToDomain(persisted), nil
}

func (r *AgentRepository) CreateInboundMessage(ctx context.Context, message domain.AgentInboundMessage) (domain.AgentInboundMessage, bool, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.inbound_message.create", "insert", "agent_inbound_messages")
	var opErr error
	defer func() { finish(opErr) }()

	message = normalizeInboundMessage(message)
	model := agentInboundMessageModelFromDomain(message)
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "provider"}, {Name: "provider_message_id"}},
			DoNothing: true,
		}).
		Create(&model)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentInboundMessage{}, false, opErr
	}
	if result.RowsAffected > 0 {
		return agentInboundMessageModelToDomain(model), true, nil
	}

	var existing agentInboundMessageModel
	if err := r.db.WithContext(ctx).
		Where("provider = ? AND provider_message_id = ?", message.Provider, message.ProviderMessageID).
		First(&existing).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentInboundMessage{}, false, opErr
	}
	return agentInboundMessageModelToDomain(existing), false, nil
}

func (r *AgentRepository) UpdateInboundMessageStatus(ctx context.Context, userID int64, id int64, status domain.AgentInboundMessageStatus, now time.Time) (domain.AgentInboundMessage, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.inbound_message.update_status", "update", "agent_inbound_messages")
	var opErr error
	defer func() { finish(opErr) }()

	result := r.db.WithContext(ctx).
		Model(&agentInboundMessageModel{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"status":     string(status),
			"updated_at": now.UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentInboundMessage{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentInboundMessage{}, opErr
	}

	var updated agentInboundMessageModel
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&updated).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentInboundMessage{}, opErr
	}
	return agentInboundMessageModelToDomain(updated), nil
}

func (r *AgentRepository) GetOrCreateSession(ctx context.Context, session domain.AgentSession) (domain.AgentSession, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.session.get_or_create", "upsert", "agent_sessions")
	var opErr error
	defer func() { finish(opErr) }()

	session = normalizeAgentSession(session)
	model := agentSessionModelFromDomain(session)
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "provider"}, {Name: "channel_session_key"}},
			DoUpdates: clause.Assignments(map[string]any{
				"external_account_id": model.ExternalAccountID,
				"user_id":             model.UserID,
				"status":              model.Status,
				"last_active_at":      model.LastActiveAt,
				"updated_at":          time.Now().UTC(),
			}),
		}).
		Create(&model).Error
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentSession{}, opErr
	}

	var persisted agentSessionModel
	if err := r.db.WithContext(ctx).
		Where("provider = ? AND channel_session_key = ?", session.Provider, session.ChannelSessionKey).
		First(&persisted).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentSession{}, opErr
	}
	return agentSessionModelToDomain(persisted), nil
}

func (r *AgentRepository) CreateAgentSession(ctx context.Context, session domain.AgentSession) (domain.AgentSession, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.session.create", "insert", "agent_sessions")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentSessionModelFromDomain(normalizeAgentSession(session))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentSession{}, opErr
	}
	return agentSessionModelToDomain(model), nil
}

func (r *AgentRepository) GetAgentSession(ctx context.Context, userID int64, sessionID int64) (domain.AgentSession, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.session.get", "select", "agent_sessions")
	var opErr error
	defer func() { finish(opErr) }()

	var model agentSessionModel
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", sessionID, userID).First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentSession{}, opErr
	}
	return agentSessionModelToDomain(model), nil
}

func (r *AgentRepository) ListAgentSessions(ctx context.Context, userID int64) ([]domain.AgentSession, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.session.list", "select", "agent_sessions")
	var opErr error
	defer func() { finish(opErr) }()

	var models []agentSessionModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("last_active_at DESC, id DESC").
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	sessions := make([]domain.AgentSession, 0, len(models))
	for _, model := range models {
		sessions = append(sessions, agentSessionModelToDomain(model))
	}
	return sessions, nil
}

func (r *AgentRepository) ListExternalAccounts(ctx context.Context, userID int64) ([]domain.ExternalAccount, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.external_account.list", "select", "external_accounts")
	var opErr error
	defer func() { finish(opErr) }()

	var models []externalAccountModel
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("updated_at DESC, id DESC").
		Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	accounts := make([]domain.ExternalAccount, 0, len(models))
	for _, model := range models {
		accounts = append(accounts, externalAccountModelToDomain(model))
	}
	return accounts, nil
}

func (r *AgentRepository) GetExternalAccount(ctx context.Context, userID int64, accountID int64) (domain.ExternalAccount, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.external_account.get", "select", "external_accounts")
	var opErr error
	defer func() { finish(opErr) }()

	var model externalAccountModel
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", accountID, userID).First(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.ExternalAccount{}, opErr
	}
	return externalAccountModelToDomain(model), nil
}

func (r *AgentRepository) SetExternalAccountActiveSession(ctx context.Context, userID int64, externalAccountID int64, sessionID int64) (domain.ExternalAccount, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.external_account.set_active_session", "update", "external_accounts")
	var opErr error
	defer func() { finish(opErr) }()

	if sessionID > 0 {
		var session agentSessionModel
		if err := r.db.WithContext(ctx).
			Where("id = ? AND user_id = ? AND external_account_id = ?", sessionID, userID, externalAccountID).
			First(&session).Error; err != nil {
			opErr = mapRepositoryError(err)
			return domain.ExternalAccount{}, opErr
		}
	}
	result := r.db.WithContext(ctx).
		Model(&externalAccountModel{}).
		Where("id = ? AND user_id = ?", externalAccountID, userID).
		Updates(map[string]any{
			"active_agent_session_id": sessionID,
			"updated_at":              time.Now().UTC(),
		})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.ExternalAccount{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.ExternalAccount{}, opErr
	}
	var updated externalAccountModel
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", externalAccountID, userID).First(&updated).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.ExternalAccount{}, opErr
	}
	return externalAccountModelToDomain(updated), nil
}

func (r *AgentRepository) TouchAgentSession(ctx context.Context, userID int64, sessionID int64, now time.Time) error {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.session.touch", "update", "agent_sessions")
	var opErr error
	defer func() { finish(opErr) }()

	result := r.db.WithContext(ctx).
		Model(&agentSessionModel{}).
		Where("id = ? AND user_id = ?", sessionID, userID).
		Updates(map[string]any{"last_active_at": now.UTC(), "updated_at": now.UTC()})
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return opErr
	}
	return nil
}

func (r *AgentRepository) CreateTurn(ctx context.Context, turn domain.AgentTurn) (domain.AgentTurn, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.turn.create", "insert", "agent_turns")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentTurnModelFromDomain(normalizeAgentTurn(turn))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentTurn{}, opErr
	}
	return agentTurnModelToDomain(model), nil
}

func (r *AgentRepository) UpdateTurn(ctx context.Context, turn domain.AgentTurn) (domain.AgentTurn, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.turn.update", "update", "agent_turns")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentTurnModelFromDomain(normalizeAgentTurn(turn))
	result := r.db.WithContext(ctx).
		Model(&agentTurnModel{}).
		Where("id = ? AND user_id = ?", turn.ID, turn.UserID).
		Select("Status", "OutputText", "ModelProvider", "Model", "ErrorMessage", "FinishedAt").
		Updates(&model)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.AgentTurn{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.ErrNotFound
		return domain.AgentTurn{}, opErr
	}

	var updated agentTurnModel
	if err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", turn.ID, turn.UserID).First(&updated).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentTurn{}, opErr
	}
	return agentTurnModelToDomain(updated), nil
}

func (r *AgentRepository) AppendTranscriptEntry(ctx context.Context, entry domain.AgentTranscriptEntry) (domain.AgentTranscriptEntry, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.transcript.append", "insert", "agent_transcript_entries")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentTranscriptEntryModelFromDomain(normalizeTranscriptEntry(entry))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentTranscriptEntry{}, opErr
	}
	persisted := agentTranscriptEntryModelToDomain(model)
	_ = r.ensureTranscriptArchiveIndex(ctx, persisted)
	return persisted, nil
}

func (r *AgentRepository) ListRecentTranscriptEntries(ctx context.Context, options domain.AgentTranscriptListOptions) ([]domain.AgentTranscriptEntry, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.transcript.list_recent", "select", "agent_transcript_entries")
	var opErr error
	defer func() { finish(opErr) }()

	options = normalizeTranscriptListOptions(options)
	query := r.db.WithContext(ctx).Model(&agentTranscriptEntryModel{}).
		Where("session_id = ? AND user_id = ?", options.SessionID, options.UserID)
	if options.BeforeTurnID > 0 {
		query = query.Where("turn_id < ?", options.BeforeTurnID)
	}
	if options.BeforeEntryID > 0 {
		query = query.Where("id < ?", options.BeforeEntryID)
	}
	if len(options.Roles) > 0 {
		query = query.Where("role IN ?", transcriptRoleStrings(options.Roles))
	}
	var models []agentTranscriptEntryModel
	if err := query.Order("created_at DESC, id DESC").Limit(options.Limit).Find(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	return transcriptModelsToChronologicalDomain(models), nil
}

func (r *AgentRepository) QueryTranscriptEntries(ctx context.Context, options domain.AgentTranscriptQueryOptions) ([]domain.AgentTranscriptEntry, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.transcript.query", "select", "agent_transcript_entries")
	var opErr error
	defer func() { finish(opErr) }()

	options = normalizeTranscriptQueryOptions(options)
	query := r.db.WithContext(ctx).Table("agent_transcript_entries").
		Where("agent_transcript_entries.session_id = ? AND agent_transcript_entries.user_id = ?", options.SessionID, options.UserID)
	if len(options.Roles) > 0 {
		query = query.Where("agent_transcript_entries.role IN ?", transcriptRoleStrings(options.Roles))
	}
	if options.Keyword != "" {
		query = query.Where("agent_transcript_entries.content ILIKE ? ESCAPE '\\'", "%"+escapeLike(options.Keyword)+"%")
	}
	if options.BeforeEntryID > 0 {
		query = query.Where("agent_transcript_entries.id < ?", options.BeforeEntryID)
	}
	if options.AfterEntryID > 0 {
		query = query.Where("agent_transcript_entries.id > ?", options.AfterEntryID)
	}
	if options.BeforeTurnID > 0 {
		query = query.Where("agent_transcript_entries.turn_id < ?", options.BeforeTurnID)
	}
	if options.After != nil {
		query = query.Where("agent_transcript_entries.created_at >= ?", options.After.UTC())
	}
	if options.Before != nil {
		query = query.Where("agent_transcript_entries.created_at <= ?", options.Before.UTC())
	}
	if options.ArchiveStatus.Valid() || options.MemoryKind.Valid() {
		query = query.Joins("JOIN agent_transcript_archive_index ON agent_transcript_archive_index.transcript_entry_id = agent_transcript_entries.id")
		if options.ArchiveStatus.Valid() {
			query = query.Where("agent_transcript_archive_index.archive_status = ?", string(options.ArchiveStatus))
		}
		if options.MemoryKind.Valid() {
			query = query.Where("agent_transcript_archive_index.memory_kind = ?", string(options.MemoryKind))
		}
	}

	var models []agentTranscriptEntryModel
	orderClause := "agent_transcript_entries.created_at DESC, agent_transcript_entries.id DESC"
	if options.Order == "asc" {
		orderClause = "agent_transcript_entries.created_at ASC, agent_transcript_entries.id ASC"
	}
	query = query.Select("agent_transcript_entries.*").Order(orderClause)
	if options.Offset > 0 {
		query = query.Offset(options.Offset)
	}
	if err := query.Limit(options.Limit).Scan(&models).Error; err != nil {
		opErr = mapRepositoryError(err)
		return nil, opErr
	}
	entries := transcriptModelsToDomain(models)
	if options.Order != "asc" {
		entries = transcriptModelsToChronologicalDomain(models)
	}
	r.touchTranscriptArchiveIndexes(ctx, entries)
	return entries, nil
}

func (r *AgentRepository) CreateRecallEvent(ctx context.Context, event domain.AgentRecallEvent) (domain.AgentRecallEvent, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.recall.create", "insert", "agent_recall_events")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentRecallEventModelFromDomain(normalizeRecallEvent(event))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentRecallEvent{}, opErr
	}
	return agentRecallEventModelToDomain(model), nil
}

func (r *AgentRepository) GetAgentSessionStats(ctx context.Context, userID int64, sessionID int64) (domain.AgentSessionStats, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.session.stats", "select", "agent_transcript_entries")
	var opErr error
	defer func() { finish(opErr) }()

	stats := domain.AgentSessionStats{SessionID: sessionID}
	type transcriptStats struct {
		Count   int64
		FirstAt *time.Time
		LastAt  *time.Time
	}
	var transcript transcriptStats
	if err := r.db.WithContext(ctx).
		Table("agent_transcript_entries").
		Select("COUNT(*) AS count, MIN(created_at) AS first_at, MAX(created_at) AS last_at").
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		Scan(&transcript).Error; err != nil {
		opErr = mapRepositoryError(err)
		return stats, opErr
	}
	stats.TranscriptCount = transcript.Count
	stats.FirstTranscriptAt = transcript.FirstAt
	stats.LastTranscriptAt = transcript.LastAt
	if err := r.db.WithContext(ctx).
		Table("agent_transcript_archive_index").
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		Count(&stats.ArchiveIndexCount).Error; err != nil {
		opErr = mapRepositoryError(err)
		return stats, opErr
	}
	if err := r.db.WithContext(ctx).
		Table("agent_recall_events").
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		Count(&stats.RecallCount).Error; err != nil {
		opErr = mapRepositoryError(err)
		return stats, opErr
	}
	return stats, nil
}

func (r *AgentRepository) ClearAgentSessionContext(ctx context.Context, userID int64, sessionID int64, now time.Time) (domain.AgentSessionStats, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.session.clear_context", "delete", "agent_transcript_archive_index")
	var opErr error
	defer func() { finish(opErr) }()

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND session_id = ?", userID, sessionID).Delete(&agentTranscriptArchiveIndexModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND session_id = ?", userID, sessionID).Delete(&agentRecallEventModel{}).Error; err != nil {
			return err
		}
		result := tx.Model(&agentSessionModel{}).
			Where("id = ? AND user_id = ?", sessionID, userID).
			Updates(map[string]any{
				"context_initialized_at":      nil,
				"context_rebuild_started_at":  nil,
				"context_rebuild_finished_at": nil,
				"context_version":             gorm.Expr("context_version + ?", 1),
				"transcript_count_indexed":    0,
				"updated_at":                  now.UTC(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentSessionStats{}, opErr
	}
	return r.GetAgentSessionStats(ctx, userID, sessionID)
}

func (r *AgentRepository) RebuildAgentSessionContext(ctx context.Context, userID int64, sessionID int64, now time.Time) (domain.AgentSessionStats, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.session.rebuild_context", "upsert", "agent_transcript_archive_index")
	var opErr error
	defer func() { finish(opErr) }()

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		startedAt := now.UTC()
		result := tx.Model(&agentSessionModel{}).
			Where("id = ? AND user_id = ?", sessionID, userID).
			Updates(map[string]any{
				"context_rebuild_started_at":  startedAt,
				"context_rebuild_finished_at": nil,
				"updated_at":                  startedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		if err := tx.Where("user_id = ? AND session_id = ?", userID, sessionID).Delete(&agentTranscriptArchiveIndexModel{}).Error; err != nil {
			return err
		}
		var entries []agentTranscriptEntryModel
		if err := tx.Where("user_id = ? AND session_id = ?", userID, sessionID).
			Order("created_at ASC, id ASC").
			Find(&entries).Error; err != nil {
			return err
		}
		for _, model := range entries {
			entry := agentTranscriptEntryModelToDomain(model)
			classification := classifyTranscriptMemory(entry.Content)
			index := agentTranscriptArchiveIndexModelFromDomain(domain.AgentTranscriptArchiveIndex{
				TranscriptEntryID: entry.ID,
				SessionID:         entry.SessionID,
				UserID:            entry.UserID,
				ArchiveStatus:     domain.AgentTranscriptArchiveStatusHot,
				MemoryKind:        classification.Kind,
				Importance:        transcriptImportanceForKind(classification.Kind),
				Keywords:          transcriptIndexKeywords(entry.Content),
				Metadata:          transcriptClassificationMetadata(classification, true),
				CreatedAt:         startedAt,
				UpdatedAt:         startedAt,
			})
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "transcript_entry_id"}},
				DoUpdates: clause.Assignments(map[string]any{
					"archive_status": string(index.ArchiveStatus),
					"memory_kind":    string(index.MemoryKind),
					"importance":     index.Importance,
					"keywords_json":  index.Keywords,
					"metadata_json":  index.Metadata,
					"updated_at":     startedAt,
				}),
			}).Create(&index).Error; err != nil {
				return err
			}
		}
		finishedAt := now.UTC()
		return tx.Model(&agentSessionModel{}).
			Where("id = ? AND user_id = ?", sessionID, userID).
			Updates(map[string]any{
				"context_initialized_at":      finishedAt,
				"context_rebuild_finished_at": finishedAt,
				"context_version":             gorm.Expr("context_version + ?", 1),
				"transcript_count_indexed":    len(entries),
				"updated_at":                  finishedAt,
			}).Error
	})
	if err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentSessionStats{}, opErr
	}
	return r.GetAgentSessionStats(ctx, userID, sessionID)
}

func (r *AgentRepository) DeleteAgentSession(ctx context.Context, userID int64, sessionID int64) error {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.session.delete", "delete", "agent_sessions")
	var opErr error
	defer func() { finish(opErr) }()

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inboundIDs []int64
		if err := tx.Model(&agentTurnModel{}).
			Where("user_id = ? AND session_id = ?", userID, sessionID).
			Pluck("inbound_message_id", &inboundIDs).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND session_id = ?", userID, sessionID).Delete(&agentRecallEventModel{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND user_id = ?", sessionID, userID).Delete(&agentSessionModel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return domain.ErrNotFound
		}
		if len(inboundIDs) > 0 {
			if err := tx.Where("user_id = ? AND id IN ?", userID, inboundIDs).Delete(&agentInboundMessageModel{}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		opErr = mapRepositoryError(err)
		return opErr
	}
	return nil
}

func (r *AgentRepository) CreateAuditLog(ctx context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.agent.audit.create", "insert", "agent_audit_logs")
	var opErr error
	defer func() { finish(opErr) }()

	model := agentAuditLogModelFromDomain(normalizeAuditLog(log))
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		opErr = mapRepositoryError(err)
		return domain.AgentAuditLog{}, opErr
	}
	return agentAuditLogModelToDomain(model), nil
}

func normalizeExternalAccount(account domain.ExternalAccount) domain.ExternalAccount {
	account.Provider = strings.TrimSpace(account.Provider)
	account.CorpID = strings.TrimSpace(account.CorpID)
	account.AgentID = strings.TrimSpace(account.AgentID)
	account.ExternalUserID = strings.TrimSpace(account.ExternalUserID)
	account.DisplayName = strings.TrimSpace(account.DisplayName)
	if !account.BindingStatus.Valid() {
		account.BindingStatus = domain.ExternalAccountBindingStatusActive
	}
	return account
}

func normalizeInboundMessage(message domain.AgentInboundMessage) domain.AgentInboundMessage {
	message.Provider = strings.TrimSpace(message.Provider)
	message.ProviderMessageID = strings.TrimSpace(message.ProviderMessageID)
	message.CorpID = strings.TrimSpace(message.CorpID)
	message.AgentID = strings.TrimSpace(message.AgentID)
	message.ExternalUserID = strings.TrimSpace(message.ExternalUserID)
	message.ChatID = strings.TrimSpace(message.ChatID)
	message.ChatType = strings.TrimSpace(message.ChatType)
	message.MsgType = strings.TrimSpace(message.MsgType)
	message.TextContent = strings.TrimSpace(message.TextContent)
	message.RequestID = strings.TrimSpace(message.RequestID)
	message.TraceID = strings.TrimSpace(message.TraceID)
	if message.Payload == nil {
		message.Payload = domain.AgentJSON{}
	}
	if message.ChatType == "" {
		message.ChatType = "direct"
	}
	if !message.Status.Valid() {
		message.Status = domain.AgentInboundMessageStatusReceived
	}
	return message
}

func normalizeAgentSession(session domain.AgentSession) domain.AgentSession {
	session.Provider = strings.TrimSpace(session.Provider)
	session.ChannelSessionKey = strings.TrimSpace(session.ChannelSessionKey)
	session.Title = strings.TrimSpace(session.Title)
	if !session.Status.Valid() {
		session.Status = domain.AgentSessionStatusActive
	}
	return session
}

func normalizeAgentTurn(turn domain.AgentTurn) domain.AgentTurn {
	turn.InputText = strings.TrimSpace(turn.InputText)
	turn.OutputText = strings.TrimSpace(turn.OutputText)
	turn.ModelProvider = strings.TrimSpace(turn.ModelProvider)
	turn.Model = strings.TrimSpace(turn.Model)
	turn.ErrorMessage = strings.TrimSpace(turn.ErrorMessage)
	if !turn.Status.Valid() {
		turn.Status = domain.AgentTurnStatusRunning
	}
	return turn
}

func normalizeTranscriptEntry(entry domain.AgentTranscriptEntry) domain.AgentTranscriptEntry {
	entry.Content = strings.TrimSpace(entry.Content)
	if entry.Metadata == nil {
		entry.Metadata = domain.AgentJSON{}
	}
	if !entry.Role.Valid() {
		entry.Role = domain.AgentTranscriptRoleSystem
	}
	return entry
}

func normalizeTranscriptListOptions(options domain.AgentTranscriptListOptions) domain.AgentTranscriptListOptions {
	if options.Limit <= 0 {
		options.Limit = 12
	}
	if options.Limit > 50 {
		options.Limit = 50
	}
	return options
}

func normalizeTranscriptQueryOptions(options domain.AgentTranscriptQueryOptions) domain.AgentTranscriptQueryOptions {
	options.Mode = strings.ToLower(strings.TrimSpace(options.Mode))
	options.Keyword = strings.TrimSpace(options.Keyword)
	options.Order = strings.ToLower(strings.TrimSpace(options.Order))
	if options.Order != "asc" {
		options.Order = "desc"
	}
	if options.Offset < 0 {
		options.Offset = 0
	}
	if options.Limit <= 0 {
		options.Limit = 8
	}
	if options.Limit > 50 {
		options.Limit = 50
	}
	return options
}

func normalizeTranscriptArchiveIndex(index domain.AgentTranscriptArchiveIndex) domain.AgentTranscriptArchiveIndex {
	if !index.ArchiveStatus.Valid() {
		index.ArchiveStatus = domain.AgentTranscriptArchiveStatusHot
	}
	if !index.MemoryKind.Valid() {
		index.MemoryKind = domain.AgentMemoryKindUnknown
	}
	if index.Importance < 0 {
		index.Importance = 0
	}
	if index.Importance > 100 {
		index.Importance = 100
	}
	if index.Metadata == nil {
		index.Metadata = domain.AgentJSON{}
	}
	return index
}

func normalizeRecallEvent(event domain.AgentRecallEvent) domain.AgentRecallEvent {
	event.Query = strings.TrimSpace(event.Query)
	event.Reason = strings.TrimSpace(event.Reason)
	if event.QueryParams == nil {
		event.QueryParams = domain.AgentJSON{}
	}
	if event.RecalledRefs == nil {
		event.RecalledRefs = domain.AgentJSON{}
	}
	if event.BudgetChars < 0 {
		event.BudgetChars = 0
	}
	return event
}

func normalizeAuditLog(log domain.AgentAuditLog) domain.AgentAuditLog {
	log.EventType = strings.TrimSpace(log.EventType)
	log.Status = strings.TrimSpace(log.Status)
	log.Message = strings.TrimSpace(log.Message)
	log.RequestID = strings.TrimSpace(log.RequestID)
	log.TraceID = strings.TrimSpace(log.TraceID)
	if log.Metadata == nil {
		log.Metadata = domain.AgentJSON{}
	}
	return log
}

func externalAccountModelFromDomain(account domain.ExternalAccount) externalAccountModel {
	return externalAccountModel{
		ID:                   account.ID,
		UserID:               account.UserID,
		Provider:             account.Provider,
		CorpID:               account.CorpID,
		AgentID:              account.AgentID,
		ExternalUserID:       account.ExternalUserID,
		DisplayName:          account.DisplayName,
		BindingStatus:        string(account.BindingStatus),
		ActiveAgentSessionID: int64Pointer(account.ActiveAgentSessionID),
		VerifiedAt:           account.VerifiedAt,
		LastSeenAt:           account.LastSeenAt,
		CreatedAt:            account.CreatedAt,
		UpdatedAt:            account.UpdatedAt,
	}
}

func externalAccountModelToDomain(model externalAccountModel) domain.ExternalAccount {
	return domain.ExternalAccount{
		ID:                   model.ID,
		UserID:               model.UserID,
		Provider:             model.Provider,
		CorpID:               model.CorpID,
		AgentID:              model.AgentID,
		ExternalUserID:       model.ExternalUserID,
		DisplayName:          model.DisplayName,
		BindingStatus:        domain.ExternalAccountBindingStatus(model.BindingStatus),
		ActiveAgentSessionID: int64Value(model.ActiveAgentSessionID),
		VerifiedAt:           model.VerifiedAt,
		LastSeenAt:           model.LastSeenAt,
		CreatedAt:            model.CreatedAt,
		UpdatedAt:            model.UpdatedAt,
	}
}

func agentInboundMessageModelFromDomain(message domain.AgentInboundMessage) agentInboundMessageModel {
	return agentInboundMessageModel{
		ID:                message.ID,
		UserID:            message.UserID,
		ExternalAccountID: message.ExternalAccountID,
		Provider:          message.Provider,
		ProviderMessageID: message.ProviderMessageID,
		CorpID:            message.CorpID,
		AgentID:           message.AgentID,
		ExternalUserID:    message.ExternalUserID,
		ChatID:            message.ChatID,
		ChatType:          message.ChatType,
		MsgType:           message.MsgType,
		TextContent:       message.TextContent,
		Payload:           cloneAgentJSON(message.Payload),
		RequestID:         message.RequestID,
		TraceID:           message.TraceID,
		Status:            string(message.Status),
		CreatedAt:         message.CreatedAt,
		UpdatedAt:         message.UpdatedAt,
	}
}

func agentInboundMessageModelToDomain(model agentInboundMessageModel) domain.AgentInboundMessage {
	return domain.AgentInboundMessage{
		ID:                model.ID,
		UserID:            model.UserID,
		ExternalAccountID: model.ExternalAccountID,
		Provider:          model.Provider,
		ProviderMessageID: model.ProviderMessageID,
		CorpID:            model.CorpID,
		AgentID:           model.AgentID,
		ExternalUserID:    model.ExternalUserID,
		ChatID:            model.ChatID,
		ChatType:          model.ChatType,
		MsgType:           model.MsgType,
		TextContent:       model.TextContent,
		Payload:           cloneAgentJSON(model.Payload),
		RequestID:         model.RequestID,
		TraceID:           model.TraceID,
		Status:            domain.AgentInboundMessageStatus(model.Status),
		CreatedAt:         model.CreatedAt,
		UpdatedAt:         model.UpdatedAt,
	}
}

func agentSessionModelFromDomain(session domain.AgentSession) agentSessionModel {
	return agentSessionModel{
		ID:                       session.ID,
		UserID:                   session.UserID,
		ExternalAccountID:        session.ExternalAccountID,
		Provider:                 session.Provider,
		ChannelSessionKey:        session.ChannelSessionKey,
		Status:                   string(session.Status),
		Title:                    session.Title,
		StartedAt:                session.StartedAt,
		LastActiveAt:             session.LastActiveAt,
		ContextInitializedAt:     session.ContextInitializedAt,
		ContextRebuildStartedAt:  session.ContextRebuildStartedAt,
		ContextRebuildFinishedAt: session.ContextRebuildFinishedAt,
		ContextVersion:           session.ContextVersion,
		TranscriptCountIndexed:   session.TranscriptCountIndexed,
		CreatedAt:                session.CreatedAt,
		UpdatedAt:                session.UpdatedAt,
	}
}

func agentSessionModelToDomain(model agentSessionModel) domain.AgentSession {
	return domain.AgentSession{
		ID:                       model.ID,
		UserID:                   model.UserID,
		ExternalAccountID:        model.ExternalAccountID,
		Provider:                 model.Provider,
		ChannelSessionKey:        model.ChannelSessionKey,
		Status:                   domain.AgentSessionStatus(model.Status),
		Title:                    model.Title,
		StartedAt:                model.StartedAt,
		LastActiveAt:             model.LastActiveAt,
		ContextInitializedAt:     model.ContextInitializedAt,
		ContextRebuildStartedAt:  model.ContextRebuildStartedAt,
		ContextRebuildFinishedAt: model.ContextRebuildFinishedAt,
		ContextVersion:           model.ContextVersion,
		TranscriptCountIndexed:   model.TranscriptCountIndexed,
		CreatedAt:                model.CreatedAt,
		UpdatedAt:                model.UpdatedAt,
	}
}

func agentTurnModelFromDomain(turn domain.AgentTurn) agentTurnModel {
	return agentTurnModel{
		ID:               turn.ID,
		SessionID:        turn.SessionID,
		InboundMessageID: turn.InboundMessageID,
		UserID:           turn.UserID,
		Status:           string(turn.Status),
		InputText:        turn.InputText,
		OutputText:       turn.OutputText,
		ModelProvider:    turn.ModelProvider,
		Model:            turn.Model,
		ErrorMessage:     turn.ErrorMessage,
		StartedAt:        turn.StartedAt,
		FinishedAt:       turn.FinishedAt,
		CreatedAt:        turn.CreatedAt,
		UpdatedAt:        turn.UpdatedAt,
	}
}

func agentTurnModelToDomain(model agentTurnModel) domain.AgentTurn {
	return domain.AgentTurn{
		ID:               model.ID,
		SessionID:        model.SessionID,
		InboundMessageID: model.InboundMessageID,
		UserID:           model.UserID,
		Status:           domain.AgentTurnStatus(model.Status),
		InputText:        model.InputText,
		OutputText:       model.OutputText,
		ModelProvider:    model.ModelProvider,
		Model:            model.Model,
		ErrorMessage:     model.ErrorMessage,
		StartedAt:        model.StartedAt,
		FinishedAt:       model.FinishedAt,
		CreatedAt:        model.CreatedAt,
		UpdatedAt:        model.UpdatedAt,
	}
}

func agentTranscriptEntryModelFromDomain(entry domain.AgentTranscriptEntry) agentTranscriptEntryModel {
	return agentTranscriptEntryModel{
		ID:        entry.ID,
		SessionID: entry.SessionID,
		TurnID:    entry.TurnID,
		UserID:    entry.UserID,
		Role:      string(entry.Role),
		Content:   entry.Content,
		Metadata:  cloneAgentJSON(entry.Metadata),
		CreatedAt: entry.CreatedAt,
	}
}

func agentTranscriptEntryModelToDomain(model agentTranscriptEntryModel) domain.AgentTranscriptEntry {
	return domain.AgentTranscriptEntry{
		ID:        model.ID,
		SessionID: model.SessionID,
		TurnID:    model.TurnID,
		UserID:    model.UserID,
		Role:      domain.AgentTranscriptRole(model.Role),
		Content:   model.Content,
		Metadata:  cloneAgentJSON(model.Metadata),
		CreatedAt: model.CreatedAt,
	}
}

func agentTranscriptArchiveIndexModelFromDomain(index domain.AgentTranscriptArchiveIndex) agentTranscriptArchiveIndexModel {
	index = normalizeTranscriptArchiveIndex(index)
	return agentTranscriptArchiveIndexModel{
		ID:                index.ID,
		TranscriptEntryID: index.TranscriptEntryID,
		SessionID:         index.SessionID,
		UserID:            index.UserID,
		ArchiveStatus:     string(index.ArchiveStatus),
		MemoryKind:        string(index.MemoryKind),
		Importance:        index.Importance,
		Keywords:          append([]string(nil), index.Keywords...),
		LastAccessedAt:    index.LastAccessedAt,
		AccessCount:       index.AccessCount,
		Metadata:          cloneAgentJSON(index.Metadata),
		CreatedAt:         index.CreatedAt,
		UpdatedAt:         index.UpdatedAt,
	}
}

func agentTranscriptArchiveIndexModelToDomain(model agentTranscriptArchiveIndexModel) domain.AgentTranscriptArchiveIndex {
	return domain.AgentTranscriptArchiveIndex{
		ID:                model.ID,
		TranscriptEntryID: model.TranscriptEntryID,
		SessionID:         model.SessionID,
		UserID:            model.UserID,
		ArchiveStatus:     domain.AgentTranscriptArchiveStatus(model.ArchiveStatus),
		MemoryKind:        domain.AgentMemoryKind(model.MemoryKind),
		Importance:        model.Importance,
		Keywords:          append([]string(nil), model.Keywords...),
		LastAccessedAt:    model.LastAccessedAt,
		AccessCount:       model.AccessCount,
		Metadata:          cloneAgentJSON(model.Metadata),
		CreatedAt:         model.CreatedAt,
		UpdatedAt:         model.UpdatedAt,
	}
}

func agentRecallEventModelFromDomain(event domain.AgentRecallEvent) agentRecallEventModel {
	event = normalizeRecallEvent(event)
	return agentRecallEventModel{
		ID:           event.ID,
		SessionID:    event.SessionID,
		TurnID:       event.TurnID,
		UserID:       event.UserID,
		Query:        event.Query,
		QueryParams:  cloneAgentJSON(event.QueryParams),
		RecalledRefs: cloneAgentJSON(event.RecalledRefs),
		Reason:       event.Reason,
		BudgetChars:  event.BudgetChars,
		CreatedAt:    event.CreatedAt,
	}
}

func agentRecallEventModelToDomain(model agentRecallEventModel) domain.AgentRecallEvent {
	return domain.AgentRecallEvent{
		ID:           model.ID,
		SessionID:    model.SessionID,
		TurnID:       model.TurnID,
		UserID:       model.UserID,
		Query:        model.Query,
		QueryParams:  cloneAgentJSON(model.QueryParams),
		RecalledRefs: cloneAgentJSON(model.RecalledRefs),
		Reason:       model.Reason,
		BudgetChars:  model.BudgetChars,
		CreatedAt:    model.CreatedAt,
	}
}

func (r *AgentRepository) ensureTranscriptArchiveIndex(ctx context.Context, entry domain.AgentTranscriptEntry) error {
	if r == nil || r.db == nil || entry.ID == 0 || entry.SessionID == 0 || entry.UserID == 0 {
		return nil
	}
	classification := classifyTranscriptMemory(entry.Content)
	index := agentTranscriptArchiveIndexModelFromDomain(domain.AgentTranscriptArchiveIndex{
		TranscriptEntryID: entry.ID,
		SessionID:         entry.SessionID,
		UserID:            entry.UserID,
		ArchiveStatus:     domain.AgentTranscriptArchiveStatusHot,
		MemoryKind:        classification.Kind,
		Importance:        transcriptImportanceForKind(classification.Kind),
		Keywords:          transcriptIndexKeywords(entry.Content),
		Metadata:          transcriptClassificationMetadata(classification, false),
	})
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "transcript_entry_id"}},
			DoNothing: true,
		}).
		Create(&index).Error
}

func (r *AgentRepository) touchTranscriptArchiveIndexes(ctx context.Context, entries []domain.AgentTranscriptEntry) {
	if r == nil || r.db == nil || len(entries) == 0 {
		return
	}
	ids := make([]int64, 0, len(entries))
	for _, entry := range entries {
		if entry.ID > 0 {
			ids = append(ids, entry.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	now := time.Now().UTC()
	_ = r.db.WithContext(ctx).
		Model(&agentTranscriptArchiveIndexModel{}).
		Where("transcript_entry_id IN ?", ids).
		Updates(map[string]any{
			"last_accessed_at": now,
			"access_count":     gorm.Expr("access_count + ?", 1),
		}).Error
}

func transcriptModelsToChronologicalDomain(models []agentTranscriptEntryModel) []domain.AgentTranscriptEntry {
	entries := make([]domain.AgentTranscriptEntry, 0, len(models))
	for i := len(models) - 1; i >= 0; i-- {
		entries = append(entries, agentTranscriptEntryModelToDomain(models[i]))
	}
	return entries
}

func transcriptModelsToDomain(models []agentTranscriptEntryModel) []domain.AgentTranscriptEntry {
	entries := make([]domain.AgentTranscriptEntry, 0, len(models))
	for _, model := range models {
		entries = append(entries, agentTranscriptEntryModelToDomain(model))
	}
	return entries
}

func transcriptRoleStrings(roles []domain.AgentTranscriptRole) []string {
	values := make([]string, 0, len(roles))
	for _, role := range roles {
		if role.Valid() {
			values = append(values, string(role))
		}
	}
	return values
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

type transcriptMemoryClassification struct {
	Kind   domain.AgentMemoryKind
	Terms  []string
	Reason string
}

func classifyTranscriptMemoryKind(content string) domain.AgentMemoryKind {
	return classifyTranscriptMemory(content).Kind
}

func classifyTranscriptMemory(content string) transcriptMemoryClassification {
	content = strings.TrimSpace(content)
	if content == "" {
		return transcriptMemoryClassification{Kind: domain.AgentMemoryKindUnknown, Reason: "empty_content"}
	}
	categories := []struct {
		kind   domain.AgentMemoryKind
		reason string
		terms  []string
	}{
		{
			kind:   domain.AgentMemoryKindDecision,
			reason: "matched_decision_terms",
			terms:  []string{"决定", "确定", "确认采用", "就用", "定为", "选择", "最终", "结论", "同意", "批准"},
		},
		{
			kind:   domain.AgentMemoryKindTask,
			reason: "matched_task_terms",
			terms:  []string{"任务", "计划", "待办", "提醒", "今天", "明天", "下周", "帮我", "执行", "安排", "创建", "更新", "检查", "跟进"},
		},
		{
			kind:   domain.AgentMemoryKindPreference,
			reason: "matched_preference_terms",
			terms:  []string{"偏好", "喜欢", "不喜欢", "关注", "优先", "以后", "记住", "习惯", "希望", "默认", "风格", "不要", "别再"},
		},
		{
			kind:   domain.AgentMemoryKindFact,
			reason: "matched_fact_terms",
			terms:  []string{"我是", "我的", "叫", "用户名", "公司", "账号", "绑定", "来源", "事实", "信息", "生日", "地区", "时区", "邮箱", "模型", "能力", "项目"},
		},
	}
	for _, category := range categories {
		matched := matchedTerms(content, category.terms)
		if len(matched) > 0 {
			return transcriptMemoryClassification{Kind: category.kind, Terms: matched, Reason: category.reason}
		}
	}
	return transcriptMemoryClassification{Kind: domain.AgentMemoryKindCasual, Reason: "fallback_casual"}
}

func matchedTerms(content string, terms []string) []string {
	matched := make([]string, 0, 3)
	for _, term := range terms {
		if strings.Contains(content, term) {
			matched = append(matched, term)
			if len(matched) >= 3 {
				break
			}
		}
	}
	return matched
}

func transcriptImportance(content string) int {
	return transcriptImportanceForKind(classifyTranscriptMemoryKind(content))
}

func transcriptImportanceForKind(kind domain.AgentMemoryKind) int {
	switch kind {
	case domain.AgentMemoryKindDecision:
		return 80
	case domain.AgentMemoryKindPreference:
		return 75
	case domain.AgentMemoryKindTask:
		return 70
	case domain.AgentMemoryKindFact:
		return 55
	case domain.AgentMemoryKindCasual:
		return 20
	default:
		return 0
	}
}

func transcriptClassificationMetadata(classification transcriptMemoryClassification, rebuild bool) domain.AgentJSON {
	metadata := domain.AgentJSON{
		"classification_strategy": "rule_v2",
		"classification_version":  2,
		"classifier":              "rule",
		"memory_kind_reason":      classification.Reason,
		"llm_classifier_status":   "not_requested",
		"background_reclassify":   true,
	}
	if len(classification.Terms) > 0 {
		metadata["classification_terms"] = classification.Terms
	}
	if rebuild {
		metadata["rebuild"] = true
	}
	return metadata
}

func transcriptIndexKeywords(content string) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	fields := strings.FieldsFunc(content, func(r rune) bool {
		switch {
		case r >= '0' && r <= '9':
			return false
		case r >= 'a' && r <= 'z':
			return false
		case r >= 'A' && r <= 'Z':
			return false
		case r >= '\u4e00' && r <= '\u9fff':
			return false
		default:
			return true
		}
	})
	keywords := make([]string, 0, 5)
	seen := map[string]struct{}{}
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len([]rune(field)) < 2 {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		keywords = append(keywords, field)
		if len(keywords) >= 5 {
			break
		}
	}
	return keywords
}

func agentAuditLogModelFromDomain(log domain.AgentAuditLog) agentAuditLogModel {
	return agentAuditLogModel{
		ID:        log.ID,
		SessionID: log.SessionID,
		TurnID:    log.TurnID,
		UserID:    log.UserID,
		EventType: log.EventType,
		Status:    log.Status,
		Message:   log.Message,
		Metadata:  cloneAgentJSON(log.Metadata),
		RequestID: log.RequestID,
		TraceID:   log.TraceID,
		CreatedAt: log.CreatedAt,
	}
}

func agentAuditLogModelToDomain(model agentAuditLogModel) domain.AgentAuditLog {
	return domain.AgentAuditLog{
		ID:        model.ID,
		SessionID: model.SessionID,
		TurnID:    model.TurnID,
		UserID:    model.UserID,
		EventType: model.EventType,
		Status:    model.Status,
		Message:   model.Message,
		Metadata:  cloneAgentJSON(model.Metadata),
		RequestID: model.RequestID,
		TraceID:   model.TraceID,
		CreatedAt: model.CreatedAt,
	}
}

func cloneAgentJSON(input domain.AgentJSON) domain.AgentJSON {
	if input == nil {
		return domain.AgentJSON{}
	}
	output := make(domain.AgentJSON, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func int64Pointer(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func int64Value(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
