package domain

import "time"

type ItemSortOrder string

const (
	ItemSortOrderDesc ItemSortOrder = "desc"
	ItemSortOrderAsc  ItemSortOrder = "asc"
)

type Item struct {
	ID             int64
	SourceID       int64
	SourceName     string
	Title          string
	URL            string
	NormalizedURL  string
	RawGUID        string
	ContentHash    string
	Summary        string
	ContentSnippet string
	Author         string
	PublishedAt    *time.Time
	FetchedAt      time.Time
	IsRead         bool
	ReadAt         *time.Time
	IsFavorite     bool
	FavoritedAt    *time.Time
	IsHidden       bool
	HiddenAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ItemUpsertResult struct {
	CreatedCount int
	UpdatedCount int
	TotalCount   int
	CreatedItems []Item
	UpdatedItems []Item
}

type ItemListOptions struct {
	UserID        int64
	SourceID      int64
	IsRead        *bool
	IsFavorite    *bool
	IsHidden      *bool
	IncludeHidden bool
	Limit         int
	Offset        int
	SortOrder     ItemSortOrder
}

type ItemListResult struct {
	Items  []Item
	Total  int64
	Limit  int
	Offset int
}
