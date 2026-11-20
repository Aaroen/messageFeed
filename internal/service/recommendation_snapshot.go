package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"messagefeed/internal/domain"
	"time"
)

func (s *RecommendationService) listRecommendationSnapshot(ctx context.Context, input ListRecommendationsInput, entries []domain.SourceCatalogEntry, limit int, order domain.ItemSortOrder, now time.Time) (ListItemsResult, error) {
	key := fmt.Sprintf("publication-v3:%d:%d:%s", input.UserID, input.SourceID, order)
	snapshot, err := s.snapshots.Load(ctx, key, input.Generation)
	if err != nil {
		return ListItemsResult{}, err
	}
	if input.Generation != "" {
		if snapshot.Generation == "" {
			return ListItemsResult{}, fmt.Errorf("%w: recommendation page expired", domain.ErrConflict)
		}
		return recommendationSnapshotPage(snapshot, limit, input.Offset), nil
	}
	if snapshot.Status == "running" {
		return recommendationSnapshotPage(snapshot, limit, input.Offset), nil
	}
	if !input.Refresh && snapshot.Generation != "" && now.Sub(snapshot.UpdatedAt) < recommendationCacheTTL {
		return recommendationSnapshotPage(snapshot, limit, input.Offset), nil
	}

	var identifier [16]byte
	if _, err := rand.Read(identifier[:]); err != nil {
		return ListItemsResult{}, err
	}
	generation := hex.EncodeToString(identifier[:])
	started, err := s.snapshots.Start(ctx, key, generation)
	if err != nil {
		return ListItemsResult{}, err
	}
	if !started {
		snapshot, err := s.snapshots.Load(ctx, key, "")
		return recommendationSnapshotPage(snapshot, limit, input.Offset), err
	}
	maxItems := maxRecommendationItemsPerSource
	if input.SourceID > 0 {
		maxItems = 0
	}
	go s.completeRecommendationSnapshot(key, generation, input.UserID, entries, order, maxItems)
	if snapshot.Generation == "" {
		history, ok, err := s.listRecommendationHistoryFallback(ctx, input, limit, order)
		if err != nil {
			return ListItemsResult{}, err
		}
		if ok {
			history.Generation = generation
			history.RefreshStatus = "running"
			return history, nil
		}
	}
	snapshot.Generation = generation
	snapshot.Status = "running"
	return recommendationSnapshotPage(snapshot, limit, input.Offset), nil
}

func recommendationSnapshotPage(snapshot domain.FeedRecommendationSnapshot, limit, offset int) ListItemsResult {
	return ListItemsResult{
		Items:         paginateRecommendationItems(snapshot.Items, limit, offset),
		Total:         int64(len(snapshot.Items)),
		Limit:         limit,
		Offset:        offset,
		Generation:    snapshot.Generation,
		RefreshStatus: snapshot.Status,
	}
}

func (s *RecommendationService) completeRecommendationSnapshot(key, generation string, userID int64, entries []domain.SourceCatalogEntry, order domain.ItemSortOrder, maxItems int) {
	fetchCtx, cancel := context.WithTimeout(context.Background(), recommendationRefreshTimeout)
	defer cancel()
	groups := s.fetchRecommendationItems(fetchCtx, userID, entries, maxItems)
	status := "ready"
	if len(groups) == 0 {
		status = "failed"
	}
	storeCtx, cancelStore := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelStore()
	if err := s.snapshots.Complete(storeCtx, key, generation, status, sortedRecommendationItems(groups, order)); err != nil {
		slog.Error("保存推荐抓取结果失败", "error", err)
	}
}
