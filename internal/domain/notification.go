package domain

import "time"

type NotificationChannel string

const (
	NotificationChannelWeChatWork NotificationChannel = "wechat_work"
	NotificationChannelNtfy       NotificationChannel = "ntfy"
	NotificationChannelInApp      NotificationChannel = "in_app"
)

func (c NotificationChannel) Valid() bool {
	switch c {
	case NotificationChannelWeChatWork, NotificationChannelNtfy, NotificationChannelInApp:
		return true
	default:
		return false
	}
}

type NotificationJobStatus string

const (
	NotificationJobStatusQueued    NotificationJobStatus = "queued"
	NotificationJobStatusRunning   NotificationJobStatus = "running"
	NotificationJobStatusSucceeded NotificationJobStatus = "succeeded"
	NotificationJobStatusFailed    NotificationJobStatus = "failed"
	NotificationJobStatusSkipped   NotificationJobStatus = "skipped"
	NotificationJobStatusCanceled  NotificationJobStatus = "canceled"
)

func (s NotificationJobStatus) Valid() bool {
	switch s {
	case NotificationJobStatusQueued, NotificationJobStatusRunning, NotificationJobStatusSucceeded, NotificationJobStatusFailed, NotificationJobStatusSkipped, NotificationJobStatusCanceled:
		return true
	default:
		return false
	}
}

type NotificationDeliveryStatus string

const (
	NotificationDeliveryStatusSucceeded NotificationDeliveryStatus = "succeeded"
	NotificationDeliveryStatusFailed    NotificationDeliveryStatus = "failed"
)

func (s NotificationDeliveryStatus) Valid() bool {
	switch s {
	case NotificationDeliveryStatusSucceeded, NotificationDeliveryStatusFailed:
		return true
	default:
		return false
	}
}

type NotificationPayload map[string]any

type NotificationJob struct {
	ID               int64
	UserID           int64
	AlertCandidateID int64
	AlertRuleID      int64
	AIAnalysisJobID  int64
	SourceID         int64
	ItemID           int64
	Status           NotificationJobStatus
	Channel          NotificationChannel
	PolicyDecision   AlertPolicyDecision
	Payload          NotificationPayload
	RequestID        string
	TraceID          string
	DedupeKey        string
	ScheduledAt      time.Time
	StartedAt        *time.Time
	FinishedAt       *time.Time
	AttemptCount     int
	MaxAttempts      int
	LockedBy         string
	LockedAt         *time.Time
	LastError        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type NotificationDelivery struct {
	ID                int64
	NotificationJobID int64
	UserID            int64
	Channel           NotificationChannel
	Status            NotificationDeliveryStatus
	RequestID         string
	TraceID           string
	ProviderMessageID string
	ResponseStatus    *int
	ResponseBody      string
	ErrorMessage      string
	SentAt            *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type NotificationJobClaimInput struct {
	Now      time.Time
	WorkerID string
	Limit    int
}

type NotificationJobListOptions struct {
	UserID int64
	Limit  int
	Offset int
}

type NotificationJobListResult struct {
	Jobs   []NotificationJob
	Total  int64
	Limit  int
	Offset int
}

type NotificationDeliveryListOptions struct {
	UserID int64
	JobID  int64
	Limit  int
	Offset int
}

type NotificationDeliveryListResult struct {
	Deliveries []NotificationDelivery
	Total      int64
	Limit      int
	Offset     int
}
