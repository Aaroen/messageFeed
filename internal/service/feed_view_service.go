package service

import (
	"context"
	"errors"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/observability"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

type FeedViewPreferenceRepository interface {
	GetByUser(ctx context.Context, userID int64) (domain.FeedViewPreference, error)
	Upsert(ctx context.Context, preference domain.FeedViewPreference) (domain.FeedViewPreference, error)
}

type FeedViewService struct {
	repository FeedViewPreferenceRepository
	now        func() time.Time
}

type FeedViewServiceOption func(*FeedViewService)

func WithFeedViewServiceNow(now func() time.Time) FeedViewServiceOption {
	return func(service *FeedViewService) {
		if now != nil {
			service.now = now
		}
	}
}

func NewFeedViewService(repository FeedViewPreferenceRepository, options ...FeedViewServiceOption) *FeedViewService {
	service := &FeedViewService{
		repository: repository,
		now:        time.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type GetFeedViewPreferenceInput struct {
	UserID int64
}

type SaveFeedViewPreferenceInput struct {
	UserID   int64
	ViewMode domain.FeedViewMode
}

func (s *FeedViewService) GetPreference(ctx context.Context, input GetFeedViewPreferenceInput) (domain.FeedViewPreference, error) {
	ctx, span := observability.StartSpan(ctx, "service.feed_view.get_preference",
		attribute.Int64("user.id", input.UserID),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.repository == nil {
		opErr = fmt.Errorf("feed view service is not configured")
		return domain.FeedViewPreference{}, opErr
	}
	if input.UserID < 1 {
		opErr = fmt.Errorf("%w: user id must be positive", domain.ErrInvalidInput)
		return domain.FeedViewPreference{}, opErr
	}

	preference, err := s.repository.GetByUser(ctx, input.UserID)
	if err == nil {
		span.SetAttributes(attribute.String("feed.view_mode", string(preference.ViewMode)))
		return preference, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		opErr = err
		return domain.FeedViewPreference{}, opErr
	}

	now := s.now().UTC()
	preference = domain.FeedViewPreference{
		UserID:    input.UserID,
		ViewMode:  domain.FeedViewModeTimeline,
		CreatedAt: now,
		UpdatedAt: now,
	}
	span.SetAttributes(attribute.String("feed.view_mode", string(preference.ViewMode)))
	return preference, nil
}

func (s *FeedViewService) SavePreference(ctx context.Context, input SaveFeedViewPreferenceInput) (domain.FeedViewPreference, error) {
	ctx, span := observability.StartSpan(ctx, "service.feed_view.save_preference",
		attribute.Int64("user.id", input.UserID),
		attribute.String("feed.view_mode", string(input.ViewMode)),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.repository == nil {
		opErr = fmt.Errorf("feed view service is not configured")
		return domain.FeedViewPreference{}, opErr
	}
	if input.UserID < 1 {
		opErr = fmt.Errorf("%w: user id must be positive", domain.ErrInvalidInput)
		return domain.FeedViewPreference{}, opErr
	}
	if !input.ViewMode.Valid() {
		opErr = fmt.Errorf("%w: invalid feed view mode", domain.ErrInvalidInput)
		return domain.FeedViewPreference{}, opErr
	}

	now := s.now().UTC()
	preference, err := s.repository.Upsert(ctx, domain.FeedViewPreference{
		UserID:    input.UserID,
		ViewMode:  input.ViewMode,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		opErr = err
		return domain.FeedViewPreference{}, opErr
	}
	return preference, nil
}
