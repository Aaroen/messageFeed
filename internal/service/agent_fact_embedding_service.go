package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/llm"
	"strings"
	"time"
)

type agentFactEmbeddingStore interface {
	UpsertAgentFactEmbedding(ctx context.Context, embedding domain.AgentFactEmbedding) (domain.AgentFactEmbedding, error)
	UpsertAgentFactArchiveIndex(ctx context.Context, fact domain.AgentFactArchiveIndex) (domain.AgentFactArchiveIndex, error)
}

type agentFactEmbeddingService struct {
	store     agentFactEmbeddingStore
	provider  llm.EmbeddingClient
	model     string
	now       func() time.Time
	batchSize int
}

func newAgentFactEmbeddingService(store agentFactEmbeddingStore, provider llm.EmbeddingClient, model string, now func() time.Time) *agentFactEmbeddingService {
	if store == nil || provider == nil {
		return nil
	}
	if now == nil {
		now = time.Now
	}
	return &agentFactEmbeddingService{
		store:     store,
		provider:  provider,
		model:     strings.TrimSpace(model),
		now:       now,
		batchSize: 16,
	}
}

func (s *agentFactEmbeddingService) EmbedFacts(ctx context.Context, facts []domain.AgentFactArchiveIndex) error {
	if s == nil || s.store == nil || s.provider == nil || len(facts) == 0 {
		return nil
	}
	batch := make([]domain.AgentFactArchiveIndex, 0, s.batchSize)
	for _, fact := range facts {
		if fact.UserID == 0 || strings.TrimSpace(fact.CanonicalRef) == "" || strings.TrimSpace(fact.ContextualText) == "" {
			continue
		}
		if embeddingStatusReadyForCurrentHash(fact) {
			continue
		}
		batch = append(batch, fact)
		if len(batch) >= s.batchSize {
			if err := s.embedBatch(ctx, batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		return s.embedBatch(ctx, batch)
	}
	return nil
}

func (s *agentFactEmbeddingService) embedBatch(ctx context.Context, facts []domain.AgentFactArchiveIndex) error {
	inputs := make([]string, 0, len(facts))
	hashes := make([]string, 0, len(facts))
	for _, fact := range facts {
		text := normalizeEmbeddingInput(fact.ContextualText)
		inputs = append(inputs, text)
		hashes = append(hashes, embeddingContentHash(text))
	}
	response, err := s.provider.Embed(ctx, llm.EmbeddingRequest{Input: inputs})
	if err != nil {
		return err
	}
	if len(response.Embeddings) != len(facts) {
		return domain.ErrInvalidInput
	}
	model := strings.TrimSpace(response.Model)
	if model == "" {
		model = s.model
	}
	now := s.now().UTC()
	for index, vector := range response.Embeddings {
		fact := facts[index]
		hash := hashes[index]
		if len(vector) == 0 {
			continue
		}
		if _, err := s.store.UpsertAgentFactEmbedding(ctx, domain.AgentFactEmbedding{
			CanonicalRef:       fact.CanonicalRef,
			UserID:             fact.UserID,
			EmbeddingModel:     model,
			EmbeddingDimension: len(vector),
			ContentHash:        hash,
			Vector:             vector,
			EmbeddingStatus:    domain.AgentFactEmbeddingStatusReady,
			Metadata: domain.AgentJSON{
				"source":          "agent_fact_archive_index",
				"indexer_version": fact.Metadata["indexer_version"],
			},
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			return err
		}
		fact.Embedding = domain.AgentJSON{
			"provider":     "openai_compatible",
			"model":        model,
			"dimension":    len(vector),
			"content_hash": hash,
			"status":       string(domain.AgentFactEmbeddingStatusReady),
			"embedded_at":  now.Format(time.RFC3339),
		}
		fact.UpdatedAt = now
		if _, err := s.store.UpsertAgentFactArchiveIndex(ctx, fact); err != nil {
			return err
		}
	}
	return nil
}

func embeddingStatusReadyForCurrentHash(fact domain.AgentFactArchiveIndex) bool {
	if fact.Embedding == nil {
		return false
	}
	status, _ := fact.Embedding["status"].(string)
	hash, _ := fact.Embedding["content_hash"].(string)
	return status == string(domain.AgentFactEmbeddingStatusReady) && hash == embeddingContentHash(normalizeEmbeddingInput(fact.ContextualText))
}

func normalizeEmbeddingInput(text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	return strings.Join(fields, " ")
}

func embeddingContentHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return "sha256:" + fmt.Sprintf("%x", sum[:])
}
