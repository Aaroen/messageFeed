package repository

import (
	"context"
	"encoding/json"
	"messagefeed/internal/domain"
	"time"

	"gorm.io/gorm"
)

type FeedRecommendationRepository struct {
	db *gorm.DB
}

func NewFeedRecommendationRepository(db *gorm.DB) *FeedRecommendationRepository {
	return &FeedRecommendationRepository{db: db}
}

func (repo *FeedRecommendationRepository) Load(ctx context.Context, key, generation string) (domain.FeedRecommendationSnapshot, error) {
	var row struct {
		Generation string
		Status     string
		Items      []byte
		UpdatedAt  time.Time
	}
	query := repo.db.WithContext(ctx).Table("feed_recommendation_snapshots").Where("cache_key = ?", key)
	if generation != "" {
		query = query.Where("generation = ?", generation)
	}
	if err := query.Order("updated_at DESC").Limit(1).Scan(&row).Error; err != nil {
		return domain.FeedRecommendationSnapshot{}, err
	}
	snapshot := domain.FeedRecommendationSnapshot{Generation: row.Generation, Status: row.Status, UpdatedAt: row.UpdatedAt}
	if row.Generation == "" {
		return snapshot, nil
	}
	if snapshot.Status == "running" && time.Since(snapshot.UpdatedAt) > time.Minute {
		snapshot.Status = "failed"
	}
	return snapshot, json.Unmarshal(row.Items, &snapshot.Items)
}

func (repo *FeedRecommendationRepository) Start(ctx context.Context, key, generation string) (bool, error) {
	var started bool
	err := repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("UPDATE feed_recommendation_snapshots SET status = 'failed' WHERE cache_key = ? AND status = 'running' AND updated_at < now() - interval '1 minute'", key).Error; err != nil {
			return err
		}
		result := tx.Exec(`INSERT INTO feed_recommendation_snapshots (cache_key, generation, status)
			VALUES (?, ?, 'running') ON CONFLICT DO NOTHING`, key, generation)
		started = result.RowsAffected == 1
		return result.Error
	})
	return started, err
}

func (repo *FeedRecommendationRepository) Complete(ctx context.Context, key, generation, status string, items []domain.Item) error {
	encoded, err := json.Marshal(items)
	if err != nil {
		return err
	}
	if err := repo.db.WithContext(ctx).Exec(`UPDATE feed_recommendation_snapshots
		SET status = ?, items = CAST(? AS jsonb), updated_at = now()
		WHERE cache_key = ? AND generation = ? AND status = 'running'`, status, string(encoded), key, generation).Error; err != nil {
		return err
	}
	return repo.db.WithContext(ctx).Exec(`DELETE FROM feed_recommendation_snapshots WHERE updated_at < now() - interval '1 day'`).Error
}
