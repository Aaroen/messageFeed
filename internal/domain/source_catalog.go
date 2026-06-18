package domain

import "time"

type SourceCatalogHealthStatus string

const (
	SourceCatalogHealthHealthy     SourceCatalogHealthStatus = "healthy"
	SourceCatalogHealthDegraded    SourceCatalogHealthStatus = "degraded"
	SourceCatalogHealthUnreachable SourceCatalogHealthStatus = "unreachable"
	SourceCatalogHealthUnknown     SourceCatalogHealthStatus = "unknown"
)

func (s SourceCatalogHealthStatus) Valid() bool {
	switch s {
	case SourceCatalogHealthHealthy, SourceCatalogHealthDegraded, SourceCatalogHealthUnreachable, SourceCatalogHealthUnknown:
		return true
	default:
		return false
	}
}

type SourceCatalogEntry struct {
	ID             int64
	SourceKey      string
	Name           string
	SiteURL        string
	FeedURL        string
	NormalizedURL  string
	Type           SourceType
	Category       string
	Tags           []string
	Language       string
	Country        string
	Official       bool
	SourceOrigin   string
	HealthStatus   SourceCatalogHealthStatus
	LastCheckedAt  *time.Time
	LastCheckError string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Subscribed     bool
	SourceID       int64
	SourceStatus   SourceStatus
}

type SourceCatalogListOptions struct {
	UserID   int64
	Category string
	Query    string
	Limit    int
	Offset   int
}

type SourceCatalogListResult struct {
	Entries []SourceCatalogEntry
	Total   int64
	Limit   int
	Offset  int
}
