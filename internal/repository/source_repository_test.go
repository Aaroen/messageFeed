package repository

import (
	"errors"
	"messagefeed/internal/domain"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestSourceModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	durationMS := 120
	itemCount := 3
	nextFetchAt := now.Add(time.Hour)
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
		NextFetchAt:          &nextFetchAt,
		ETag:                 `"etag-value"`,
		LastModified:         "Tue, 23 Jun 2026 09:00:00 GMT",
		FetchPriority:        2,
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
	if converted.NextFetchAt == nil || !converted.NextFetchAt.Equal(nextFetchAt) {
		t.Fatalf("NextFetchAt = %#v, want %s", converted.NextFetchAt, nextFetchAt)
	}
	if converted.ETag != source.ETag {
		t.Fatalf("ETag = %q, want %q", converted.ETag, source.ETag)
	}
	if converted.LastModified != source.LastModified {
		t.Fatalf("LastModified = %q, want %q", converted.LastModified, source.LastModified)
	}
	if converted.FetchPriority != source.FetchPriority {
		t.Fatalf("FetchPriority = %d, want %d", converted.FetchPriority, source.FetchPriority)
	}
}

func TestSourceModelETagColumnName(t *testing.T) {
	parsed, err := schema.Parse(&sourceModel{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse source model schema: %v", err)
	}
	field := parsed.LookUpField("ETag")
	if field == nil {
		t.Fatal("ETag field not found")
	}
	if field.DBName != "etag" {
		t.Fatalf("ETag DBName = %q, want etag", field.DBName)
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

func TestNormalizeSourceDueFetchOptions(t *testing.T) {
	now := time.Date(2026, 6, 23, 16, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	options := normalizeSourceDueFetchOptions(domain.SourceDueFetchOptions{
		Now:   now,
		Limit: maxDueSourceFetchLimit + 1,
	})

	if options.Limit != maxDueSourceFetchLimit {
		t.Fatalf("Limit = %d, want %d", options.Limit, maxDueSourceFetchLimit)
	}
	if options.Now.Location() != time.UTC {
		t.Fatalf("Now location = %s, want UTC", options.Now.Location())
	}

	defaulted := normalizeSourceDueFetchOptions(domain.SourceDueFetchOptions{})
	if defaulted.Limit != defaultDueSourceFetchLimit {
		t.Fatalf("default Limit = %d, want %d", defaulted.Limit, defaultDueSourceFetchLimit)
	}
	if defaulted.Now.IsZero() {
		t.Fatal("default Now is zero")
	}
}
