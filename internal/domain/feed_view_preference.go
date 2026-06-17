package domain

import "time"

type FeedViewMode string

const (
	FeedViewModeTimeline FeedViewMode = "timeline"
)

func (m FeedViewMode) Valid() bool {
	return m == FeedViewModeTimeline
}

type FeedViewPreference struct {
	ID        int64
	UserID    int64
	ViewMode  FeedViewMode
	CreatedAt time.Time
	UpdatedAt time.Time
}
