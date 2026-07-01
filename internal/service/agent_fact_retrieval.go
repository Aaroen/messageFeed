package service

import (
	"context"
	"fmt"
	"messagefeed/internal/agent"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"messagefeed/internal/metrics"
	"messagefeed/internal/observability"
	"sort"
	"strings"
	"time"
)

type agentFactRetriever struct {
	repository     AgentConversationRepository
	embedding      llm.EmbeddingClient
	embedder       *agentFactEmbeddingService
	embeddingModel string
	now            func() time.Time
}

type agentRecallTraceStore interface {
	CreateAgentRecallTrace(ctx context.Context, trace domain.AgentRecallTrace) (domain.AgentRecallTrace, error)
}

func newAgentFactRetriever(repository AgentConversationRepository, embedding llm.EmbeddingClient, embeddingModel string, now func() time.Time) *agentFactRetriever {
	if repository == nil {
		return nil
	}
	if now == nil {
		now = time.Now
	}
	var embedder *agentFactEmbeddingService
	if store, ok := any(repository).(agentFactEmbeddingStore); ok {
		embedder = newAgentFactEmbeddingService(store, embedding, embeddingModel, now)
	}
	return &agentFactRetriever{
		repository:     repository,
		embedding:      embedding,
		embedder:       embedder,
		embeddingModel: strings.TrimSpace(embeddingModel),
		now:            now,
	}
}

func (r *agentFactRetriever) Recall(ctx context.Context, plan domain.AgentFactRecallPlan) (result domain.AgentFactRecallResult, err error) {
	if r == nil || r.repository == nil {
		return domain.AgentFactRecallResult{}, nil
	}
	plan = normalizeAgentFactRecallPlan(plan, r.embeddingModel)
	totalStartedAt := time.Now()
	trace := domain.AgentRecallTrace{
		RequestID:          observability.RequestID(ctx),
		TraceID:            observability.TraceID(ctx),
		UserID:             plan.UserID,
		SessionID:          plan.SessionID,
		TurnID:             plan.TurnID,
		Mode:               plan.Mode,
		QueryText:          plan.Query,
		NeedsHistoryRecall: true,
		HistoryQueryPlan: domain.AgentJSON{
			"mode":            string(plan.Mode),
			"query":           plan.Query,
			"limit":           plan.Limit,
			"fact_types":      agentFactTypesTraceValues(plan.FactTypes),
			"memory_kinds":    agentMemoryKindsTraceValues(plan.MemoryKinds),
			"needs_source":    plan.NeedsSourceFact,
			"max_risk_level":  string(plan.MaxRiskLevel),
			"embedding_model": plan.EmbeddingModel,
		},
		Status:    domain.AgentRecallTraceSucceeded,
		CreatedAt: r.now().UTC(),
	}
	defer func() {
		trace.TotalMS = time.Since(totalStartedAt).Milliseconds()
		trace.FinalHitCount = len(result.Hits)
		trace.FinalSources = recallHitSources(result.Hits)
		if err != nil {
			trace.Status = domain.AgentRecallTraceFailed
			trace.ErrorMessage = err.Error()
		} else if strings.TrimSpace(trace.FallbackReason) != "" {
			trace.Status = domain.AgentRecallTraceDegraded
		}
		r.recordRecallTrace(ctx, trace)
		r.recordRecallMetrics(trace)
	}()
	hitsByRef := map[string]domain.AgentFactRecallHit{}
	diagnostics := domain.AgentFactRecallDiagnostics{}
	if shouldRunFullTextRecall(plan) {
		stageStartedAt := time.Now()
		trace.FullTextAttempted = true
		facts, err := r.repository.QueryAgentFactArchiveIndex(ctx, domain.AgentFactArchiveQueryOptions{
			UserID:        plan.UserID,
			SessionID:     plan.SessionID,
			TurnID:        plan.TurnID,
			FactTypes:     plan.FactTypes,
			MemoryKinds:   plan.MemoryKinds,
			Query:         plan.Query,
			After:         plan.After,
			Before:        plan.Before,
			Limit:         recallCandidateLimit(plan.Limit),
			MaxRiskLevel:  plan.MaxRiskLevel,
			UseFullText:   true,
			MinImportance: 0,
		})
		if err != nil {
			return domain.AgentFactRecallResult{}, err
		}
		trace.FullTextMS = time.Since(stageStartedAt).Milliseconds()
		trace.FullTextCount = len(facts)
		_ = r.embedFactsForFutureRecall(ctx, facts)
		for index, fact := range facts {
			hit := recallHitFromFact(fact)
			hit.FullTextScore = rankScore(index, len(facts))
			hit.StructuredScore = structuredScore(plan, fact)
			hit.ImportanceScore = float64(fact.Importance) / 100
			hit.RecencyScore = recencyScore(fact.UpdatedAt, r.now)
			hit.HitSources = append(hit.HitSources, "fulltext")
			mergeRecallHit(hitsByRef, hit)
		}
	}
	if shouldRunVectorRecall(plan) {
		diagnostics.VectorAttempted = true
		trace.VectorAttempted = true
		if r.embedding == nil {
			diagnostics.QueryEmbeddingStatus = "unavailable"
			diagnostics.VectorError = "embedding client is not configured"
			trace.EmbeddingStatus = diagnostics.QueryEmbeddingStatus
			trace.EmbeddingError = diagnostics.VectorError
			trace.FallbackReason = "embedding_unavailable"
			if plan.Mode == domain.AgentFactRecallModeSemantic {
				return domain.AgentFactRecallResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_fact_embedding_unavailable", diagnostics.VectorError, "service.agent_fact_retrieval.recall", false, nil)
			}
		} else {
			embeddingStartedAt := time.Now()
			trace.EmbeddingAttempted = true
			response, err := r.embedding.Embed(ctx, llm.EmbeddingRequest{Input: []string{plan.Query}, Operation: "query_embedding"})
			trace.EmbeddingMS = time.Since(embeddingStartedAt).Milliseconds()
			if err != nil {
				diagnostics.QueryEmbeddingStatus = "failed"
				diagnostics.VectorError = err.Error()
				trace.EmbeddingStatus = diagnostics.QueryEmbeddingStatus
				trace.EmbeddingError = diagnostics.VectorError
				trace.FallbackReason = "query_embedding_failed"
				if plan.Mode == domain.AgentFactRecallModeSemantic {
					return domain.AgentFactRecallResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_fact_query_embedding_failed", "query embedding failed", "service.agent_fact_retrieval.recall", true, err)
				}
			} else if len(response.Embeddings) == 0 {
				diagnostics.QueryEmbeddingStatus = "empty"
				diagnostics.QueryEmbeddingModel = strings.TrimSpace(response.Model)
				diagnostics.VectorError = "embedding response contains no vectors"
				trace.EmbeddingStatus = diagnostics.QueryEmbeddingStatus
				trace.EmbeddingModel = diagnostics.QueryEmbeddingModel
				trace.EmbeddingError = diagnostics.VectorError
				trace.FallbackReason = "query_embedding_empty"
				if plan.Mode == domain.AgentFactRecallModeSemantic {
					return domain.AgentFactRecallResult{}, domain.NewAppError(domain.ErrorKindUnavailable, "agent_fact_query_embedding_empty", diagnostics.VectorError, "service.agent_fact_retrieval.recall", true, nil)
				}
			} else {
				queryVector := response.Embeddings[0]
				diagnostics.QueryEmbeddingStatus = "ready"
				diagnostics.QueryEmbeddingModel = strings.TrimSpace(response.Model)
				diagnostics.QueryEmbeddingDimension = len(queryVector)
				diagnostics.QueryEmbeddingCount = len(response.Embeddings)
				trace.EmbeddingStatus = diagnostics.QueryEmbeddingStatus
				trace.EmbeddingModel = diagnostics.QueryEmbeddingModel
				trace.EmbeddingDimension = diagnostics.QueryEmbeddingDimension
				vectorPlan := plan
				if strings.TrimSpace(vectorPlan.EmbeddingModel) == "" {
					vectorPlan.EmbeddingModel = response.Model
				}
				vectorStartedAt := time.Now()
				vectorHits, err := r.repository.SearchAgentFactEmbeddings(ctx, vectorPlan, queryVector)
				trace.VectorMS = time.Since(vectorStartedAt).Milliseconds()
				if err != nil {
					diagnostics.VectorError = err.Error()
					trace.FallbackReason = "vector_search_failed"
					return domain.AgentFactRecallResult{}, err
				}
				diagnostics.VectorCandidateCount = len(vectorHits)
				trace.VectorCandidateCount = len(vectorHits)
				for index, hit := range vectorHits {
					hit.StructuredScore = structuredScore(plan, hit.Fact)
					hit.ImportanceScore = float64(hit.Fact.Importance) / 100
					hit.RecencyScore = recencyScore(hit.Fact.UpdatedAt, r.now)
					if hit.VectorScore == 0 {
						hit.VectorScore = rankScore(index, len(vectorHits))
					}
					mergeRecallHit(hitsByRef, hit)
				}
			}
		}
	}
	relationRefs := relationExpansionRefs(hitsByRef)
	if len(relationRefs) > 0 {
		relationStartedAt := time.Now()
		trace.RelationAttempted = true
		facts, err := r.repository.QueryAgentFactArchiveIndex(ctx, domain.AgentFactArchiveQueryOptions{
			UserID:       plan.UserID,
			SessionID:    plan.SessionID,
			TurnID:       plan.TurnID,
			FactTypes:    plan.FactTypes,
			MemoryKinds:  plan.MemoryKinds,
			RelationRefs: relationRefs,
			After:        plan.After,
			Before:       plan.Before,
			Limit:        recallRelationLimit(plan.Limit),
			MaxRiskLevel: plan.MaxRiskLevel,
		})
		if err != nil {
			return domain.AgentFactRecallResult{}, err
		}
		trace.RelationMS = time.Since(relationStartedAt).Milliseconds()
		trace.RelationCount = len(facts)
		_ = r.embedFactsForFutureRecall(ctx, facts)
		for _, fact := range facts {
			hit := recallHitFromFact(fact)
			hit.RelationScore = relationScore(fact, relationRefs)
			hit.StructuredScore = structuredScore(plan, fact)
			hit.ImportanceScore = float64(fact.Importance) / 100
			hit.RecencyScore = recencyScore(fact.UpdatedAt, r.now)
			hit.HitSources = append(hit.HitSources, "relation")
			mergeRecallHit(hitsByRef, hit)
		}
	}
	hits := finalizeRecallHits(hitsByRef, plan)
	facts := make([]domain.AgentFactArchiveIndex, 0, len(hits))
	for _, hit := range hits {
		facts = append(facts, hit.Fact)
	}
	var sources []domain.AgentFactSource
	var projections []domain.AgentFactProjection
	if plan.NeedsSourceFact && len(facts) > 0 {
		resolved, err := r.repository.ResolveAgentFactSources(ctx, plan.UserID, facts)
		if err != nil {
			return domain.AgentFactRecallResult{}, err
		}
		sources = resolved
		hitByRef := map[string]domain.AgentFactRecallHit{}
		for _, hit := range hits {
			hitByRef[agent.NormalizeCanonicalRef(hit.CanonicalRef)] = hit
		}
		for _, source := range sources {
			ref := agent.NormalizeCanonicalRef(source.CanonicalRef)
			hit := hitByRef[ref]
			projections = append(projections, projectFactSource(source, hit))
		}
	}
	return domain.AgentFactRecallResult{
		Plan:        plan,
		Hits:        hits,
		Sources:     sources,
		Projections: projections,
		Diagnostics: diagnostics,
		GeneratedAt: r.now().UTC(),
	}, nil
}

func (r *agentFactRetriever) embedFactsForFutureRecall(ctx context.Context, facts []domain.AgentFactArchiveIndex) error {
	if r == nil || r.embedder == nil || len(facts) == 0 {
		return nil
	}
	if len(facts) > 8 {
		facts = facts[:8]
	}
	return r.embedder.EmbedFacts(ctx, facts)
}

func (r *agentFactRetriever) recordRecallTrace(ctx context.Context, trace domain.AgentRecallTrace) {
	store, ok := any(r.repository).(agentRecallTraceStore)
	if !ok {
		return
	}
	_, _ = store.CreateAgentRecallTrace(ctx, trace)
	if trace.UserID == 0 {
		return
	}
	eventStore, ok := any(r.repository).(agentTraceEventStore)
	if !ok {
		return
	}
	status := domain.AgentTraceEventSucceeded
	if trace.Status == domain.AgentRecallTraceDegraded {
		status = domain.AgentTraceEventDegraded
	} else if trace.Status == domain.AgentRecallTraceFailed {
		status = domain.AgentTraceEventFailed
	}
	now := r.now().UTC()
	event := domain.AgentTraceEvent{
		RequestID:     trace.RequestID,
		TraceID:       trace.TraceID,
		UserID:        trace.UserID,
		SessionID:     trace.SessionID,
		TurnID:        trace.TurnID,
		EventKind:     domain.AgentTraceEventRecall,
		EventName:     "agent_fact_recall",
		Status:        status,
		StartedAt:     now.Add(-time.Duration(trace.TotalMS) * time.Millisecond),
		FinishedAt:    &now,
		DurationMS:    trace.TotalMS,
		InputSummary:  safeSummary(trace.QueryText, 500),
		OutputSummary: fmt.Sprintf("%d hits", trace.FinalHitCount),
		Metadata: domain.AgentJSON{
			"mode":                   string(trace.Mode),
			"fallback_reason":        trace.FallbackReason,
			"fulltext_count":         trace.FullTextCount,
			"vector_candidate_count": trace.VectorCandidateCount,
			"relation_count":         trace.RelationCount,
			"embedding_status":       trace.EmbeddingStatus,
		},
		CreatedAt: now,
	}
	if trace.ErrorMessage != "" {
		event.ErrorCode = "agent_fact_recall_failed"
		event.ErrorMessage = trace.ErrorMessage
	}
	_, _ = eventStore.CreateAgentTraceEvent(ctx, event)
}

func (r *agentFactRetriever) recordRecallMetrics(trace domain.AgentRecallTrace) {
	mode := string(trace.Mode)
	status := string(trace.Status)
	fallback := strings.TrimSpace(trace.FallbackReason)
	if fallback == "" {
		fallback = "none"
	}
	metrics.AgentRecallRequestsTotal.WithLabelValues(mode, status, fallback).Inc()
	if trace.TotalMS > 0 {
		metrics.AgentRecallDuration.WithLabelValues(mode, "total", status).Observe(float64(trace.TotalMS) / 1000)
	}
	if trace.FullTextAttempted {
		metrics.AgentRecallDuration.WithLabelValues(mode, "fulltext", status).Observe(float64(trace.FullTextMS) / 1000)
		metrics.AgentRecallHits.WithLabelValues(mode, "fulltext").Observe(float64(trace.FullTextCount))
	}
	if trace.EmbeddingAttempted {
		metrics.AgentRecallDuration.WithLabelValues(mode, "query_embedding", status).Observe(float64(trace.EmbeddingMS) / 1000)
	}
	if trace.VectorAttempted {
		metrics.AgentRecallDuration.WithLabelValues(mode, "vector", status).Observe(float64(trace.VectorMS) / 1000)
		metrics.AgentRecallHits.WithLabelValues(mode, "vector").Observe(float64(trace.VectorCandidateCount))
	}
	if trace.RelationAttempted {
		metrics.AgentRecallDuration.WithLabelValues(mode, "relation", status).Observe(float64(trace.RelationMS) / 1000)
		metrics.AgentRecallHits.WithLabelValues(mode, "relation").Observe(float64(trace.RelationCount))
	}
	metrics.AgentRecallHits.WithLabelValues(mode, "final").Observe(float64(trace.FinalHitCount))
}

func agentFactTypesTraceValues(types []domain.AgentFactType) []string {
	values := make([]string, 0, len(types))
	for _, value := range types {
		if strings.TrimSpace(string(value)) != "" {
			values = append(values, string(value))
		}
	}
	return values
}

func agentMemoryKindsTraceValues(kinds []domain.AgentMemoryKind) []string {
	values := make([]string, 0, len(kinds))
	for _, value := range kinds {
		if strings.TrimSpace(string(value)) != "" {
			values = append(values, string(value))
		}
	}
	return values
}

func normalizeAgentFactRecallPlan(plan domain.AgentFactRecallPlan, defaultEmbeddingModel string) domain.AgentFactRecallPlan {
	if !plan.Mode.Valid() {
		plan.Mode = domain.AgentFactRecallModeHybrid
	}
	plan.Query = strings.TrimSpace(plan.Query)
	if plan.Limit <= 0 {
		plan.Limit = 8
	}
	if plan.Limit > 30 {
		plan.Limit = 30
	}
	if !plan.MaxRiskLevel.Valid() {
		plan.MaxRiskLevel = domain.AgentMemoryRiskMedium
	}
	if strings.TrimSpace(plan.EmbeddingModel) == "" {
		plan.EmbeddingModel = strings.TrimSpace(defaultEmbeddingModel)
	}
	if plan.UserID == 0 {
		plan.NeedsSourceFact = false
	}
	return plan
}

func shouldRunFullTextRecall(plan domain.AgentFactRecallPlan) bool {
	switch plan.Mode {
	case domain.AgentFactRecallModeSemantic:
		return false
	default:
		return plan.UserID > 0
	}
}

func shouldRunVectorRecall(plan domain.AgentFactRecallPlan) bool {
	if plan.UserID == 0 || strings.TrimSpace(plan.Query) == "" {
		return false
	}
	switch plan.Mode {
	case domain.AgentFactRecallModeSemantic, domain.AgentFactRecallModeHybrid:
		return true
	default:
		return false
	}
}

func recallHitFromFact(fact domain.AgentFactArchiveIndex) domain.AgentFactRecallHit {
	return domain.AgentFactRecallHit{
		Fact:         fact,
		CanonicalRef: agent.NormalizeCanonicalRef(fact.CanonicalRef),
	}
}

func mergeRecallHit(hitsByRef map[string]domain.AgentFactRecallHit, hit domain.AgentFactRecallHit) {
	ref := agent.NormalizeCanonicalRef(hit.CanonicalRef)
	if ref == "" {
		ref = agent.NormalizeCanonicalRef(hit.Fact.CanonicalRef)
	}
	if ref == "" {
		return
	}
	hit.CanonicalRef = ref
	existing, ok := hitsByRef[ref]
	if !ok {
		hitsByRef[ref] = hit
		return
	}
	existing.StructuredScore = maxFloat(existing.StructuredScore, hit.StructuredScore)
	existing.FullTextScore = maxFloat(existing.FullTextScore, hit.FullTextScore)
	existing.VectorScore = maxFloat(existing.VectorScore, hit.VectorScore)
	existing.ImportanceScore = maxFloat(existing.ImportanceScore, hit.ImportanceScore)
	existing.RecencyScore = maxFloat(existing.RecencyScore, hit.RecencyScore)
	existing.RelationScore = maxFloat(existing.RelationScore, hit.RelationScore)
	existing.HitSources = uniqueStrings(append(existing.HitSources, hit.HitSources...))
	hitsByRef[ref] = existing
}

func finalizeRecallHits(hitsByRef map[string]domain.AgentFactRecallHit, plan domain.AgentFactRecallPlan) []domain.AgentFactRecallHit {
	hits := make([]domain.AgentFactRecallHit, 0, len(hitsByRef))
	for _, hit := range hitsByRef {
		hit.FinalScore = 0.25*hit.StructuredScore +
			0.25*hit.FullTextScore +
			0.30*hit.VectorScore +
			0.10*hit.ImportanceScore +
			0.05*hit.RecencyScore +
			0.05*hit.RelationScore
		if hit.FinalScore == 0 {
			hit.FinalScore = hit.ImportanceScore
		}
		hit.Reason = strings.Join(hit.HitSources, "+")
		hits = append(hits, hit)
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].FinalScore == hits[j].FinalScore {
			return hits[i].Fact.UpdatedAt.After(hits[j].Fact.UpdatedAt)
		}
		return hits[i].FinalScore > hits[j].FinalScore
	})
	if len(hits) > plan.Limit {
		hits = hits[:plan.Limit]
	}
	return hits
}

func relationExpansionRefs(hitsByRef map[string]domain.AgentFactRecallHit) []string {
	refs := make([]string, 0, len(hitsByRef)*2)
	for _, hit := range hitsByRef {
		for _, ref := range hit.Fact.RelationRefs {
			ref = strings.TrimSpace(ref)
			if ref != "" {
				refs = append(refs, ref)
			}
		}
	}
	if len(refs) > 12 {
		refs = refs[:12]
	}
	return uniqueStrings(refs)
}

func projectFactSource(source domain.AgentFactSource, hit domain.AgentFactRecallHit) domain.AgentFactProjection {
	content := strings.TrimSpace(source.Content)
	if content == "" {
		content = strings.TrimSpace(source.Summary)
	}
	if content == "" {
		content = strings.TrimSpace(source.Title)
	}
	projected := content
	if len([]rune(projected)) > 1800 {
		projected = firstRunes(projected, 1800) + "\n[projection: content shortened; source_fact remains available by canonical_ref]"
	}
	return domain.AgentFactProjection{
		CanonicalRef:  agent.NormalizeCanonicalRef(source.CanonicalRef),
		IndexHit:      hit,
		SourceFact:    source,
		Text:          projected,
		TokenEstimate: estimateProjectionTokens(projected),
		TrustLevel:    "source_fact",
		RiskLevel:     hit.Fact.RiskLevel,
	}
}

func structuredScore(plan domain.AgentFactRecallPlan, fact domain.AgentFactArchiveIndex) float64 {
	score := 0.35
	if plan.SessionID > 0 && fact.SessionID == plan.SessionID {
		score += 0.25
	}
	if plan.TurnID > 0 && fact.TurnID > 0 && fact.TurnID <= plan.TurnID {
		score += 0.10
	}
	if len(plan.FactTypes) > 0 && factTypeIncluded(plan.FactTypes, fact.FactType) {
		score += 0.15
	}
	if len(plan.MemoryKinds) > 0 && memoryKindIncluded(plan.MemoryKinds, fact.MemoryKind) {
		score += 0.15
	}
	return clampServiceUnitScore(score)
}

func relationScore(fact domain.AgentFactArchiveIndex, refs []string) float64 {
	refSet := map[string]struct{}{}
	for _, ref := range refs {
		refSet[strings.TrimSpace(ref)] = struct{}{}
	}
	matches := 0
	for _, ref := range append(fact.RelationRefs, fact.SourceRefs...) {
		if _, ok := refSet[strings.TrimSpace(ref)]; ok {
			matches++
		}
	}
	if matches == 0 {
		return 0
	}
	if matches > 5 {
		matches = 5
	}
	return float64(matches) / 5
}

func recencyScore(value time.Time, now func() time.Time) float64 {
	if value.IsZero() {
		return 0
	}
	age := now().UTC().Sub(value.UTC())
	switch {
	case age <= 24*time.Hour:
		return 1
	case age <= 7*24*time.Hour:
		return 0.75
	case age <= 30*24*time.Hour:
		return 0.5
	case age <= 180*24*time.Hour:
		return 0.25
	default:
		return 0.1
	}
}

func rankScore(index int, total int) float64 {
	if total <= 1 {
		return 1
	}
	return 1 - float64(index)/float64(total)
}

func recallCandidateLimit(limit int) int {
	if limit <= 0 {
		return 30
	}
	if limit < 20 {
		return limit * 4
	}
	return limit * 2
}

func recallRelationLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	return limit * 2
}

func estimateProjectionTokens(text string) int {
	runes := len([]rune(text))
	if runes == 0 {
		return 0
	}
	return (runes / 3) + 1
}

func factTypeIncluded(types []domain.AgentFactType, value domain.AgentFactType) bool {
	for _, item := range types {
		if item == value {
			return true
		}
	}
	return false
}

func memoryKindIncluded(kinds []domain.AgentMemoryKind, value domain.AgentMemoryKind) bool {
	for _, item := range kinds {
		if item == value {
			return true
		}
	}
	return false
}

func firstRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func maxFloat(a float64, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func clampServiceUnitScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func formatFactProjectionForContext(projection domain.AgentFactProjection) string {
	var builder strings.Builder
	builder.WriteString("index_hit: ")
	builder.WriteString(projection.CanonicalRef)
	if projection.IndexHit.Reason != "" {
		builder.WriteString(" reason=")
		builder.WriteString(projection.IndexHit.Reason)
	}
	builder.WriteString(fmt.Sprintf(" score=%.3f", projection.IndexHit.FinalScore))
	builder.WriteString("\nsource_fact: ")
	builder.WriteString(string(projection.SourceFact.FactType))
	builder.WriteString(":")
	builder.WriteString(fmt.Sprint(projection.SourceFact.FactID))
	builder.WriteString("\nprojection:\n")
	builder.WriteString(projection.Text)
	return builder.String()
}

func recallHitSources(hits []domain.AgentFactRecallHit) domain.AgentJSON {
	output := domain.AgentJSON{}
	for _, hit := range hits {
		ref := agent.NormalizeCanonicalRef(hit.CanonicalRef)
		if ref == "" {
			continue
		}
		output[ref] = domain.AgentJSON{
			"sources": hit.HitSources,
			"score":   hit.FinalScore,
		}
	}
	return output
}
