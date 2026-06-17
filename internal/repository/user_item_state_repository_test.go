package repository

import (
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestUserItemStateModelToDomain(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	model := userItemStateModel{
		ID:          1,
		UserID:      2,
		ItemID:      3,
		IsRead:      true,
		ReadAt:      &now,
		IsFavorite:  true,
		FavoritedAt: &now,
		IsHidden:    true,
		HiddenAt:    &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	state := userItemStateModelToDomain(model)

	if state.ID != model.ID {
		t.Fatalf("ID = %d, want %d", state.ID, model.ID)
	}
	if !state.IsRead || !state.IsFavorite || !state.IsHidden {
		t.Fatalf("state flags = read:%v favorite:%v hidden:%v", state.IsRead, state.IsFavorite, state.IsHidden)
	}
	if state.ReadAt == nil || !state.ReadAt.Equal(now) {
		t.Fatalf("ReadAt = %#v, want %s", state.ReadAt, now)
	}
}

func TestApplyUserItemStateUpdateClearsTimestamp(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	model := userItemStateModel{
		IsRead: true,
		ReadAt: &now,
	}
	value := false

	applyUserItemStateUpdate(&model, domain.UserItemStateUpdate{IsRead: &value}, now)

	if model.IsRead {
		t.Fatal("IsRead = true, want false")
	}
	if model.ReadAt != nil {
		t.Fatalf("ReadAt = %#v, want nil", model.ReadAt)
	}
}

func TestUserItemStateUpsertAssignmentsWritesFalseValue(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	value := false

	assignments := userItemStateUpsertAssignments(domain.UserItemStateUpdate{IsFavorite: &value}, now)

	if assignments["is_favorite"] != false {
		t.Fatalf("is_favorite assignment = %#v, want false", assignments["is_favorite"])
	}
	if assignments["favorited_at"] != nil {
		t.Fatalf("favorited_at assignment = %#v, want nil", assignments["favorited_at"])
	}
	if assignments["updated_at"] != now {
		t.Fatalf("updated_at assignment = %#v, want %s", assignments["updated_at"], now)
	}
}
