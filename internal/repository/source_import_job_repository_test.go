package repository

import (
	"messagefeed/internal/domain"
	"testing"
	"time"
)

func TestSourceImportJobModelRoundTrip(t *testing.T) {
	now := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	job := domain.SourceImportJob{
		ID:             7,
		UserID:         2,
		ImportType:     domain.SourceImportTypeOPML,
		Status:         domain.SourceImportStatusPartial,
		RequestedCount: 3,
		SuccessCount:   2,
		FailureCount:   1,
		ErrorDetails: []domain.SourceImportJobError{
			{Reference: "https://bad.example/feed.xml", Message: "unsupported feed"},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	model := sourceImportJobModelFromDomain(job)
	converted := sourceImportJobModelToDomain(model)

	if converted.ID != job.ID {
		t.Fatalf("ID = %d, want %d", converted.ID, job.ID)
	}
	if converted.UserID != job.UserID {
		t.Fatalf("UserID = %d, want %d", converted.UserID, job.UserID)
	}
	if converted.ImportType != job.ImportType {
		t.Fatalf("ImportType = %q, want %q", converted.ImportType, job.ImportType)
	}
	if converted.Status != job.Status {
		t.Fatalf("Status = %q, want %q", converted.Status, job.Status)
	}
	if converted.RequestedCount != job.RequestedCount || converted.SuccessCount != job.SuccessCount || converted.FailureCount != job.FailureCount {
		t.Fatalf("counts = requested:%d success:%d failure:%d", converted.RequestedCount, converted.SuccessCount, converted.FailureCount)
	}
	if got, want := len(converted.ErrorDetails), len(job.ErrorDetails); got != want {
		t.Fatalf("ErrorDetails length = %d, want %d", got, want)
	}
	if converted.ErrorDetails[0].Reference != job.ErrorDetails[0].Reference {
		t.Fatalf("ErrorDetails[0].Reference = %q, want %q", converted.ErrorDetails[0].Reference, job.ErrorDetails[0].Reference)
	}
	if !converted.CreatedAt.Equal(now) || !converted.UpdatedAt.Equal(now) {
		t.Fatalf("timestamps = %s/%s, want %s", converted.CreatedAt, converted.UpdatedAt, now)
	}
}

func TestSourceImportJobModelCopiesErrorDetails(t *testing.T) {
	job := domain.SourceImportJob{
		ErrorDetails: []domain.SourceImportJobError{
			{Reference: "one", Message: "failed"},
		},
	}

	model := sourceImportJobModelFromDomain(job)
	model.ErrorDetails[0].Reference = "changed"

	if job.ErrorDetails[0].Reference != "one" {
		t.Fatalf("source ErrorDetails was mutated: %#v", job.ErrorDetails)
	}

	converted := sourceImportJobModelToDomain(model)
	converted.ErrorDetails[0].Reference = "converted"

	if model.ErrorDetails[0].Reference != "changed" {
		t.Fatalf("model ErrorDetails was mutated: %#v", model.ErrorDetails)
	}
}

func TestSourceImportJobModelUsesEmptyErrorDetailsArray(t *testing.T) {
	model := sourceImportJobModelFromDomain(domain.SourceImportJob{})

	if model.ErrorDetails == nil {
		t.Fatal("ErrorDetails is nil, want empty slice for JSONB []")
	}
	if got, want := len(model.ErrorDetails), 0; got != want {
		t.Fatalf("ErrorDetails length = %d, want %d", got, want)
	}
}

func TestNormalizeSourceImportJobListOptions(t *testing.T) {
	tests := []struct {
		name    string
		options domain.SourceImportJobListOptions
		want    domain.SourceImportJobListOptions
	}{
		{
			name:    "default limit and offset",
			options: domain.SourceImportJobListOptions{UserID: 1},
			want: domain.SourceImportJobListOptions{
				UserID: 1,
				Limit:  domain.DefaultSourceImportJobListLimit,
				Offset: 0,
			},
		},
		{
			name: "cap maximum limit",
			options: domain.SourceImportJobListOptions{
				UserID: 1,
				Limit:  domain.MaxSourceImportJobListLimit + 1,
				Offset: 5,
			},
			want: domain.SourceImportJobListOptions{
				UserID: 1,
				Limit:  domain.MaxSourceImportJobListLimit,
				Offset: 5,
			},
		},
		{
			name: "clamp negative offset",
			options: domain.SourceImportJobListOptions{
				UserID: 1,
				Limit:  10,
				Offset: -3,
			},
			want: domain.SourceImportJobListOptions{
				UserID: 1,
				Limit:  10,
				Offset: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSourceImportJobListOptions(tt.options)
			if got != tt.want {
				t.Fatalf("options = %#v, want %#v", got, tt.want)
			}
		})
	}
}
