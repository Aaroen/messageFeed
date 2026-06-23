package domain

import "time"

type ItemEventType string

const (
	ItemEventTypeItemCreated       ItemEventType = "item.created"
	ItemEventTypeSourceFetchFailed ItemEventType = "source.fetch_failed"
)

func (t ItemEventType) Valid() bool {
	switch t {
	case ItemEventTypeItemCreated, ItemEventTypeSourceFetchFailed:
		return true
	default:
		return false
	}
}

type ItemEventStatus string

const (
	ItemEventStatusPending    ItemEventStatus = "pending"
	ItemEventStatusProcessing ItemEventStatus = "processing"
	ItemEventStatusProcessed  ItemEventStatus = "processed"
	ItemEventStatusFailed     ItemEventStatus = "failed"
)

func (s ItemEventStatus) Valid() bool {
	switch s {
	case ItemEventStatusPending, ItemEventStatusProcessing, ItemEventStatusProcessed, ItemEventStatusFailed:
		return true
	default:
		return false
	}
}

type ItemEventPayload map[string]any

type ItemEvent struct {
	ID           int64
	UserID       int64
	SourceID     int64
	ItemID       int64
	EventType    ItemEventType
	Status       ItemEventStatus
	Payload      ItemEventPayload
	DedupeKey    string
	AvailableAt  time.Time
	ProcessedAt  *time.Time
	AttemptCount int
	LastError    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ItemEventClaimInput struct {
	Now   time.Time
	Limit int
}
