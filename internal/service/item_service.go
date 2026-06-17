package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/observability"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

type UserItemStateRepository interface {
	UpdateState(ctx context.Context, update domain.UserItemStateUpdate) (domain.UserItemState, error)
}

type ItemService struct {
	repository UserItemStateRepository
	now        func() time.Time
}

type ItemServiceOption func(*ItemService)

func WithItemServiceNow(now func() time.Time) ItemServiceOption {
	return func(service *ItemService) {
		if now != nil {
			service.now = now
		}
	}
}

func NewItemService(repository UserItemStateRepository, options ...ItemServiceOption) *ItemService {
	service := &ItemService{
		repository: repository,
		now:        time.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type UpdateItemStateInput struct {
	UserID int64
	ItemID int64
	Value  bool
}

func (s *ItemService) MarkRead(ctx context.Context, input UpdateItemStateInput) (domain.UserItemState, error) {
	ctx, span := observability.StartSpan(ctx, "service.item.mark_read",
		attribute.Int64("user.id", input.UserID),
		attribute.Int64("item.id", input.ItemID),
		attribute.Bool("state.value", input.Value),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if err := validateItemStateInput(input); err != nil {
		opErr = err
		return domain.UserItemState{}, opErr
	}
	value := input.Value
	state, err := s.updateState(ctx, domain.UserItemStateUpdate{
		UserID: input.UserID,
		ItemID: input.ItemID,
		IsRead: &value,
	})
	if err != nil {
		opErr = err
		return domain.UserItemState{}, opErr
	}
	return state, nil
}

func (s *ItemService) SetFavorite(ctx context.Context, input UpdateItemStateInput) (domain.UserItemState, error) {
	ctx, span := observability.StartSpan(ctx, "service.item.set_favorite",
		attribute.Int64("user.id", input.UserID),
		attribute.Int64("item.id", input.ItemID),
		attribute.Bool("state.value", input.Value),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if err := validateItemStateInput(input); err != nil {
		opErr = err
		return domain.UserItemState{}, opErr
	}
	value := input.Value
	state, err := s.updateState(ctx, domain.UserItemStateUpdate{
		UserID:     input.UserID,
		ItemID:     input.ItemID,
		IsFavorite: &value,
	})
	if err != nil {
		opErr = err
		return domain.UserItemState{}, opErr
	}
	return state, nil
}

func (s *ItemService) SetHidden(ctx context.Context, input UpdateItemStateInput) (domain.UserItemState, error) {
	ctx, span := observability.StartSpan(ctx, "service.item.set_hidden",
		attribute.Int64("user.id", input.UserID),
		attribute.Int64("item.id", input.ItemID),
		attribute.Bool("state.value", input.Value),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if err := validateItemStateInput(input); err != nil {
		opErr = err
		return domain.UserItemState{}, opErr
	}
	value := input.Value
	state, err := s.updateState(ctx, domain.UserItemStateUpdate{
		UserID:   input.UserID,
		ItemID:   input.ItemID,
		IsHidden: &value,
	})
	if err != nil {
		opErr = err
		return domain.UserItemState{}, opErr
	}
	return state, nil
}

func (s *ItemService) updateState(ctx context.Context, update domain.UserItemStateUpdate) (domain.UserItemState, error) {
	if s == nil || s.repository == nil {
		return domain.UserItemState{}, fmt.Errorf("item service is not configured")
	}
	update.Now = s.now()
	return s.repository.UpdateState(ctx, update)
}

func validateItemStateInput(input UpdateItemStateInput) error {
	if input.UserID < 1 {
		return fmt.Errorf("%w: user id must be positive", domain.ErrInvalidInput)
	}
	if input.ItemID < 1 {
		return fmt.Errorf("%w: item id must be positive", domain.ErrInvalidInput)
	}
	return nil
}
