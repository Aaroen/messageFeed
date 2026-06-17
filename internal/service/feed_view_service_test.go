package service

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestFeedViewServiceReturnsDefaultPreference(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	service := NewFeedViewService(
		&fakeFeedViewPreferenceRepository{getErr: domain.ErrNotFound},
		WithFeedViewServiceNow(func() time.Time {
			return now
		}),
	)

	preference, err := service.GetPreference(context.Background(), GetFeedViewPreferenceInput{UserID: 1})
	if err != nil {
		t.Fatalf("GetPreference returned error: %v", err)
	}

	if preference.ViewMode != domain.FeedViewModeTimeline {
		t.Fatalf("ViewMode = %q, want %q", preference.ViewMode, domain.FeedViewModeTimeline)
	}
	if preference.ID != 0 {
		t.Fatalf("ID = %d, want default preference without persisted ID", preference.ID)
	}
	if !preference.UpdatedAt.Equal(now) {
		t.Fatalf("UpdatedAt = %s, want %s", preference.UpdatedAt, now)
	}
}

func TestFeedViewServiceRejectsInvalidMode(t *testing.T) {
	service := NewFeedViewService(&fakeFeedViewPreferenceRepository{})

	_, err := service.SavePreference(context.Background(), SaveFeedViewPreferenceInput{
		UserID:   1,
		ViewMode: domain.FeedViewMode("recommendations"),
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

type fakeFeedViewPreferenceRepository struct {
	getErr     error
	preference domain.FeedViewPreference
}

func (r *fakeFeedViewPreferenceRepository) GetByUser(_ context.Context, userID int64) (domain.FeedViewPreference, error) {
	if r.getErr != nil {
		return domain.FeedViewPreference{}, r.getErr
	}
	if r.preference.UserID == 0 {
		r.preference = domain.FeedViewPreference{ID: 1, UserID: userID, ViewMode: domain.FeedViewModeTimeline}
	}
	return r.preference, nil
}

func (r *fakeFeedViewPreferenceRepository) Upsert(_ context.Context, preference domain.FeedViewPreference) (domain.FeedViewPreference, error) {
	r.preference = preference
	r.preference.ID = 1
	return r.preference, nil
}
