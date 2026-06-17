package service

import (
	"context"
	"errors"
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestItemServiceUpdatesState(t *testing.T) {
	repository := &fakeUserItemStateRepository{}
	service := NewItemService(
		repository,
		WithItemServiceNow(func() time.Time {
			return time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
		}),
	)

	state, err := service.MarkRead(context.Background(), UpdateItemStateInput{
		UserID: 1,
		ItemID: 2,
		Value:  true,
	})
	if err != nil {
		t.Fatalf("MarkRead returned error: %v", err)
	}

	if !state.IsRead {
		t.Fatal("IsRead = false, want true")
	}
	if repository.update.IsRead == nil || !*repository.update.IsRead {
		t.Fatalf("repository IsRead = %#v, want true", repository.update.IsRead)
	}
	if repository.update.Now.IsZero() {
		t.Fatal("repository Now is zero")
	}
}

func TestItemServiceRejectsInvalidItemID(t *testing.T) {
	service := NewItemService(&fakeUserItemStateRepository{})

	_, err := service.SetFavorite(context.Background(), UpdateItemStateInput{
		UserID: 1,
		ItemID: 0,
		Value:  true,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

type fakeUserItemStateRepository struct {
	update domain.UserItemStateUpdate
}

func (r *fakeUserItemStateRepository) UpdateState(_ context.Context, update domain.UserItemStateUpdate) (domain.UserItemState, error) {
	r.update = update
	state := domain.UserItemState{
		UserID: update.UserID,
		ItemID: update.ItemID,
	}
	if update.IsRead != nil {
		state.IsRead = *update.IsRead
	}
	if update.IsFavorite != nil {
		state.IsFavorite = *update.IsFavorite
	}
	if update.IsHidden != nil {
		state.IsHidden = *update.IsHidden
	}
	return state, nil
}
