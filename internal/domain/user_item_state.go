package domain

import "time"

type UserItemState struct {
	ID          int64
	UserID      int64
	ItemID      int64
	IsRead      bool
	ReadAt      *time.Time
	IsFavorite  bool
	FavoritedAt *time.Time
	IsHidden    bool
	HiddenAt    *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type UserItemStateUpdate struct {
	UserID     int64
	ItemID     int64
	IsRead     *bool
	IsFavorite *bool
	IsHidden   *bool
	Now        time.Time
}
