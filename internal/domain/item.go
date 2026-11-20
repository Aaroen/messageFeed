package domain

import "time"

type ItemSortOrder string

const (
	ItemSortOrderDesc ItemSortOrder = "desc"
	ItemSortOrderAsc  ItemSortOrder = "asc"
)

type Item struct {
	ID                  int64
	SourceID            int64
	CatalogID           int64
	SubscriptionID      int64
	SourceKind          string
	SourceName          string
	Title               string
	URL                 string
	NormalizedURL       string
	RawGUID             string
	ContentHash         string
	Summary             string
	ContentSnippet      string
	Author              string
	PublishedAt         *time.Time
	SourceUpdatedAt     *time.Time
	PublicationVerified bool
	FetchedAt           time.Time
	IsRead              bool
	ReadAt              *time.Time
	IsFavorite          bool
	FavoritedAt         *time.Time
	IsHidden            bool
	HiddenAt            *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
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
	Cursor        *ItemCursor
	CatalogOnly   bool
}

type FeedRecommendationSnapshot struct {
	Generation string
	Status     string
	Items      []Item
	UpdatedAt  time.Time
}

type ItemCursor struct {
	Snapshot time.Time
	Time     time.Time
	ID       int64
}

func (item Item) FeedOrderTime() time.Time {
	if item.PublishedAt != nil {
		return *item.PublishedAt
	}
	if item.SourceUpdatedAt != nil {
		return *item.SourceUpdatedAt
	}
	if !item.CreatedAt.IsZero() {
		return item.CreatedAt
	}
	return item.FetchedAt
}

type ItemListResult struct {
	Items  []Item
	Total  int64
	Limit  int
	Offset int
}
