package repository

import (
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestFeedViewPreferenceModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	preference := domain.FeedViewPreference{
		ID:        1,
		UserID:    2,
		ViewMode:  domain.FeedViewModeTimeline,
		CreatedAt: now,
		UpdatedAt: now,
	}

	model := feedViewPreferenceModelFromDomain(preference)
	converted := feedViewPreferenceModelToDomain(model)

	if converted.UserID != preference.UserID {
		t.Fatalf("UserID = %d, want %d", converted.UserID, preference.UserID)
	}
	if converted.ViewMode != preference.ViewMode {
		t.Fatalf("ViewMode = %q, want %q", converted.ViewMode, preference.ViewMode)
	}
	if !converted.UpdatedAt.Equal(now) {
		t.Fatalf("UpdatedAt = %s, want %s", converted.UpdatedAt, now)
	}
}
