package domain

import "time"

type SourceType string

const (
	SourceTypeRSS      SourceType = "rss"
	SourceTypeAtom     SourceType = "atom"
	SourceTypeJSONFeed SourceType = "json_feed"
)

func (t SourceType) Valid() bool {
	switch t {
	case SourceTypeRSS, SourceTypeAtom, SourceTypeJSONFeed:
		return true
	default:
		return false
	}
}

type SourceStatus string

const (
	SourceStatusActive   SourceStatus = "active"
	SourceStatusInactive SourceStatus = "inactive"
)

func (s SourceStatus) Valid() bool {
	switch s {
	case SourceStatusActive, SourceStatusInactive:
		return true
	default:
		return false
	}
}

type Source struct {
	ID                   int64
	UserID               int64
	Name                 string
	Type                 SourceType
	URL                  string
	NormalizedURL        string
	Status               SourceStatus
	FetchIntervalSeconds int
	Tags                 []string
	Weight               int
	LastFetchedAt        *time.Time
	LastFetchStatus      string
	LastFetchError       string
	LastFetchDurationMS  *int
	LastFetchItemCount   *int
	NextFetchAt          *time.Time
	ETag                 string
	LastModified         string
	FetchPriority        int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

const (
	SourceLastFetchStatusSuccess = "success"
	SourceLastFetchStatusFailed  = "failed"
)

type FeedFetchResult struct {
	Items []Item
}
