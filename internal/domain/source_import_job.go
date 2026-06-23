package domain

import "time"

type SourceImportType string

const (
	SourceImportTypeCatalog SourceImportType = "catalog"
	SourceImportTypeURLs    SourceImportType = "urls"
	SourceImportTypeOPML    SourceImportType = "opml"
)

const (
	DefaultSourceImportJobListLimit = 20
	MaxSourceImportJobListLimit     = 100
)

func (t SourceImportType) Valid() bool {
	switch t {
	case SourceImportTypeCatalog, SourceImportTypeURLs, SourceImportTypeOPML:
		return true
	default:
		return false
	}
}

type SourceImportStatus string

const (
	SourceImportStatusCompleted SourceImportStatus = "completed"
	SourceImportStatusPartial   SourceImportStatus = "partial"
	SourceImportStatusFailed    SourceImportStatus = "failed"
)

func (s SourceImportStatus) Valid() bool {
	switch s {
	case SourceImportStatusCompleted, SourceImportStatusPartial, SourceImportStatusFailed:
		return true
	default:
		return false
	}
}

type SourceImportJobError struct {
	Reference string `json:"reference"`
	Message   string `json:"message"`
}

type SourceImportJob struct {
	ID             int64
	UserID         int64
	ImportType     SourceImportType
	Status         SourceImportStatus
	RequestedCount int
	SuccessCount   int
	FailureCount   int
	ErrorDetails   []SourceImportJobError
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type SourceImportJobListOptions struct {
	UserID int64
	Limit  int
	Offset int
}

type SourceImportJobListResult struct {
	Jobs   []SourceImportJob
	Total  int64
	Limit  int
	Offset int
}
