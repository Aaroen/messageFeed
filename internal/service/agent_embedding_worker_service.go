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

type AgentEmbeddingWorkerRepository interface {
	ClaimPendingAgentFactIndexJobs(ctx context.Context, input domain.AgentFactIndexJobClaimInput) ([]domain.AgentFactIndexJob, error)
	UpdateAgentFactIndexJob(ctx context.Context, job domain.AgentFactIndexJob) (domain.AgentFactIndexJob, error)
	QueryAgentFactArchiveIndex(ctx context.Context, options domain.AgentFactArchiveQueryOptions) ([]domain.AgentFactArchiveIndex, error)
	UpsertAgentFactEmbedding(ctx context.Context, embedding domain.AgentFactEmbedding) (domain.AgentFactEmbedding, error)
	UpsertAgentFactArchiveIndex(ctx context.Context, fact domain.AgentFactArchiveIndex) (domain.AgentFactArchiveIndex, error)
	CreateAgentEmbeddingTrace(ctx context.Context, trace domain.AgentEmbeddingTrace) (domain.AgentEmbeddingTrace, error)
	CreateAgentTraceEvent(ctx context.Context, event domain.AgentTraceEvent) (domain.AgentTraceEvent, error)
}

type AgentEmbeddingWorkerService struct {
	repository     AgentEmbeddingWorkerRepository
	embedding      llm.EmbeddingClient
	embeddingModel string
	now            func() time.Time
	batchSize      int
}

type RunAgentEmbeddingWorkerOnceInput struct {
	WorkerID string
	Limit    int
}

type AgentEmbeddingWorkerResult struct {
	ClaimedCount   int
	SucceededCount int
	FailedCount    int
	SkippedCount   int
}

func NewAgentEmbeddingWorkerService(repository AgentEmbeddingWorkerRepository, embedding llm.EmbeddingClient, embeddingModel string, now func() time.Time) *AgentEmbeddingWorkerService {
	if now == nil {
		now = time.Now
	}
	return &AgentEmbeddingWorkerService{
		repository:     repository,
		embedding:      embedding,
		embeddingModel: strings.TrimSpace(embeddingModel),
		now:            now,
		batchSize:      16,
	}
}

func (s *AgentEmbeddingWorkerService) RunOnce(ctx context.Context, input RunAgentEmbeddingWorkerOnceInput) (AgentEmbeddingWorkerResult, error) {
	if s == nil || s.repository == nil {
		return AgentEmbeddingWorkerResult{}, nil
	}
	if s.embedding == nil {
		return AgentEmbeddingWorkerResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "embedding_worker_client_unavailable", "embedding client is unavailable", "service.agent_embedding_worker.run_once", true, nil)
	}
	workerID := strings.TrimSpace(input.WorkerID)
	if workerID == "" {
		workerID = "agent-embedding-worker"
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}
	jobs, err := s.repository.ClaimPendingAgentFactIndexJobs(ctx, domain.AgentFactIndexJobClaimInput{
		JobType:  domain.AgentFactIndexJobEmbed,
		WorkerID: workerID,
		Limit:    limit,
		Now:      s.now().UTC(),
	})
	if err != nil {
		return AgentEmbeddingWorkerResult{}, err
	}
	result := AgentEmbeddingWorkerResult{ClaimedCount: len(jobs)}
	for _, job := range jobs {
		if err := s.runJob(ctx, workerID, job); err != nil {
			result.FailedCount++
			continue
		}
		result.SucceededCount++
	}
	return result, nil
}

func (s *AgentEmbeddingWorkerService) runJob(ctx context.Context, workerID string, job domain.AgentFactIndexJob) error {
	startedAt := s.now().UTC()
	ctx, span := observability.StartSpan(ctx, "service.agent.embedding_job.run_once")
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	refs := embeddingJobCanonicalRefs(job.Scope)
	if len(refs) == 0 {
		opErr = fmt.Errorf("embedding job has no canonical refs")
		return s.finishJob(ctx, workerID, job, domain.AgentFactIndexJobFailed, 0, 1, opErr, startedAt)
	}
	userID := embeddingJobUserID(job.Scope)
	if userID == 0 {
		opErr = fmt.Errorf("embedding job has no user_id")
		return s.finishJob(ctx, workerID, job, domain.AgentFactIndexJobFailed, 0, 1, opErr, startedAt)
	}
	facts, err := s.repository.QueryAgentFactArchiveIndex(ctx, domain.AgentFactArchiveQueryOptions{
		UserID:        userID,
		CanonicalRefs: refs,
		Limit:         len(refs),
	})
	if err != nil {
		opErr = err
		return s.finishJob(ctx, workerID, job, domain.AgentFactIndexJobFailed, 0, 1, err, startedAt)
	}
	if len(facts) == 0 {
		return s.finishJob(ctx, workerID, job, domain.AgentFactIndexJobSucceeded, 0, 0, nil, startedAt)
	}
	embedder := newAgentFactEmbeddingService(s.repository, s.embedding, s.embeddingModel, s.now)
	if embedder == nil {
		opErr = fmt.Errorf("embedding service unavailable")
		return s.finishJob(ctx, workerID, job, domain.AgentFactIndexJobFailed, 0, len(facts), opErr, startedAt)
	}
	err = embedder.EmbedFacts(ctx, facts)
	if err != nil {
		opErr = err
		return s.finishJob(ctx, workerID, job, domain.AgentFactIndexJobFailed, 0, len(facts), err, startedAt)
	}
	return s.finishJob(ctx, workerID, job, domain.AgentFactIndexJobSucceeded, len(facts), 0, nil, startedAt)
}

func (s *AgentEmbeddingWorkerService) finishJob(ctx context.Context, workerID string, job domain.AgentFactIndexJob, status domain.AgentFactIndexJobStatus, processed int, failed int, err error, startedAt time.Time) error {
	now := s.now().UTC()
	job.Status = status
	job.ProcessedCount += processed
	job.FailedCount += failed
	job.FinishedAt = &now
	job.UpdatedAt = now
	if job.Cursor == nil {
		job.Cursor = domain.AgentJSON{}
	}
	job.Cursor["worker_id"] = workerID
	if err != nil {
		job.ErrorMessage = safeSummary(err.Error(), 1000)
	}
	_, updateErr := s.repository.UpdateAgentFactIndexJob(ctx, job)
	duration := now.Sub(startedAt)
	statusLabel := string(status)
	reason := embeddingJobReason(job.Scope)
	metrics.AgentEmbeddingJobsTotal.WithLabelValues(statusLabel, reason).Inc()
	metrics.AgentEmbeddingJobDuration.WithLabelValues(statusLabel).Observe(duration.Seconds())
	eventStatus := domain.AgentTraceEventSucceeded
	if status == domain.AgentFactIndexJobFailed {
		eventStatus = domain.AgentTraceEventFailed
	}
	event := domain.AgentTraceEvent{
		RequestID:  observability.RequestID(ctx),
		TraceID:    observability.TraceID(ctx),
		EventKind:  domain.AgentTraceEventWorker,
		EventName:  "embed_fact_index",
		Status:     eventStatus,
		StartedAt:  startedAt,
		FinishedAt: &now,
		DurationMS: duration.Milliseconds(),
		JobID:      fmt.Sprintf("agent_fact_index_job:%d", job.ID),
		CreatedAt:  now,
		Metadata: domain.AgentJSON{
			"worker_id":       workerID,
			"processed_count": processed,
			"failed_count":    failed,
			"job_status":      string(status),
			"reason":          reason,
		},
	}
	if err != nil {
		event.ErrorCode = "embedding_job_failed"
		event.ErrorMessage = err.Error()
	}
	_, _ = s.repository.CreateAgentTraceEvent(ctx, event)
	if updateErr != nil {
		return updateErr
	}
	return err
}

func embeddingJobCanonicalRefs(scope domain.AgentJSON) []string {
	values, ok := scope["canonical_refs"].([]any)
	if !ok {
		if typed, ok := scope["canonical_refs"].([]string); ok {
			return compactNonEmptyStrings(typed)
		}
		return nil
	}
	refs := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			refs = append(refs, text)
		}
	}
	return compactNonEmptyStrings(refs)
}

func embeddingJobReason(scope domain.AgentJSON) string {
	reason, _ := scope["reason"].(string)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "unknown"
	}
	return reason
}

func embeddingJobUserID(scope domain.AgentJSON) int64 {
	switch value := scope["user_id"].(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		if value > 0 {
			return int64(value)
		}
	case json.Number:
		id, _ := value.Int64()
		return id
	}
	return 0
}
