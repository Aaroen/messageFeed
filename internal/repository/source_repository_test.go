package repository

import (
	"errors"
	"messagefeed/internal/domain"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func TestSourceModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	durationMS := 120
	itemCount := 3
	source := domain.Source{
		ID:                   11,
		UserID:               1,
		Name:                 "Go Blog",
		Type:                 domain.SourceTypeAtom,
		URL:                  "https://go.dev/blog/feed.atom",
		NormalizedURL:        "https://go.dev/blog/feed.atom",
		Status:               domain.SourceStatusActive,
		FetchIntervalSeconds: 3600,
		Tags:                 []string{"go", "official"},
		Weight:               5,
		LastFetchedAt:        &now,
		LastFetchStatus:      "success",
		LastFetchError:       "",
		LastFetchDurationMS:  &durationMS,
		LastFetchItemCount:   &itemCount,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	model := sourceModelFromDomain(source)
	converted := sourceModelToDomain(model)

	if converted.ID != source.ID {
		t.Fatalf("ID = %d, want %d", converted.ID, source.ID)
	}
	if converted.Type != source.Type {
		t.Fatalf("Type = %q, want %q", converted.Type, source.Type)
	}
	if converted.Status != source.Status {
		t.Fatalf("Status = %q, want %q", converted.Status, source.Status)
	}
	if converted.NormalizedURL != source.NormalizedURL {
		t.Fatalf("NormalizedURL = %q, want %q", converted.NormalizedURL, source.NormalizedURL)
	}
	if got, want := len(converted.Tags), len(source.Tags); got != want {
		t.Fatalf("Tags length = %d, want %d", got, want)
	}
	if converted.Tags[0] != "go" || converted.Tags[1] != "official" {
		t.Fatalf("Tags = %#v", converted.Tags)
	}
	if converted.LastFetchDurationMS == nil || *converted.LastFetchDurationMS != durationMS {
		t.Fatalf("LastFetchDurationMS = %#v, want %d", converted.LastFetchDurationMS, durationMS)
	}
	if converted.LastFetchItemCount == nil || *converted.LastFetchItemCount != itemCount {
		t.Fatalf("LastFetchItemCount = %#v, want %d", converted.LastFetchItemCount, itemCount)
	}
}

func TestMapRepositoryError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "record not found",
			err:  gorm.ErrRecordNotFound,
			want: domain.ErrNotFound,
		},
		{
			name: "unique violation",
			err:  &pgconn.PgError{Code: "23505"},
			want: domain.ErrConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapRepositoryError(tt.err)
			if !errors.Is(got, tt.want) {
				t.Fatalf("error = %v, want %v", got, tt.want)
			}
		})
	}
}
