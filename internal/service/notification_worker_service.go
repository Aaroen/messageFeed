package service

import (
	"context"
	"fmt"
	"messagefeed/internal/domain"
	"messagefeed/internal/metrics"
	"messagefeed/internal/notifier"
	"messagefeed/internal/observability"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

const notificationWorkerLastErrorMaxLength = 2000

type NotificationWorkerStore interface {
	ClaimDueJobs(ctx context.Context, input domain.NotificationJobClaimInput) ([]domain.NotificationJob, error)
	UpdateJob(ctx context.Context, job domain.NotificationJob) (domain.NotificationJob, error)
	CreateDelivery(ctx context.Context, delivery domain.NotificationDelivery) (domain.NotificationDelivery, error)
}

type ownedNotificationWorkerStore interface {
	UpdateJobIfOwned(ctx context.Context, job domain.NotificationJob, workerID string) (domain.NotificationJob, error)
}

type NotificationSender interface {
	SendText(ctx context.Context, message notifier.WeChatWorkTextMessage) (notifier.WeChatWorkSendResult, error)
}

type NotificationWorkerService struct {
	store  NotificationWorkerStore
	sender NotificationSender
	now    func() time.Time
}

type NotificationWorkerServiceOption func(*NotificationWorkerService)

func WithNotificationWorkerNow(now func() time.Time) NotificationWorkerServiceOption {
	return func(service *NotificationWorkerService) {
		if now != nil {
			service.now = now
		}
	}
}

func NewNotificationWorkerService(store NotificationWorkerStore, sender NotificationSender, options ...NotificationWorkerServiceOption) *NotificationWorkerService {
	service := &NotificationWorkerService{
		store:  store,
		sender: sender,
		now:    time.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type RunNotificationWorkerOnceInput struct {
	Now           time.Time
	WorkerID      string
	Limit         int
	LeaseDuration time.Duration
}

type RunNotificationWorkerOnceResult struct {
	ClaimedCount   int
	SucceededCount int
	FailedCount    int
	RetryCount     int
}

func (s *NotificationWorkerService) RunOnce(ctx context.Context, input RunNotificationWorkerOnceInput) (RunNotificationWorkerOnceResult, error) {
	ctx, span := observability.StartSpan(ctx, "service.notification_worker.run_once",
		attribute.String("worker.id", strings.TrimSpace(input.WorkerID)),
		attribute.Int("claim.limit", input.Limit),
	)
	var opErr error
	defer func() { observability.EndSpan(span, opErr) }()

	if s == nil || s.store == nil || s.sender == nil {
		opErr = fmt.Errorf("notification worker service is not configured")
		return RunNotificationWorkerOnceResult{}, opErr
	}
	now := input.Now
	if now.IsZero() {
		now = s.now().UTC()
	} else {
		now = now.UTC()
	}
	workerID := strings.TrimSpace(input.WorkerID)
	if workerID == "" {
		workerID = "notification-worker"
	}
	jobs, err := s.store.ClaimDueJobs(ctx, domain.NotificationJobClaimInput{
		Now:           now,
		WorkerID:      workerID,
		Limit:         input.Limit,
		LeaseDuration: input.LeaseDuration,
	})
	if err != nil {
		opErr = err
		return RunNotificationWorkerOnceResult{}, err
	}
	result := RunNotificationWorkerOnceResult{ClaimedCount: len(jobs)}
	for _, job := range jobs {
		status, err := s.processJob(ctx, job, now)
		if err != nil {
			opErr = err
			return result, err
		}
		switch status {
		case domain.NotificationJobStatusSucceeded:
			result.SucceededCount++
		case domain.NotificationJobStatusQueued:
			result.RetryCount++
		default:
			result.FailedCount++
		}
	}
	span.SetAttributes(
		attribute.Int("notification.claimed", result.ClaimedCount),
		attribute.Int("notification.succeeded", result.SucceededCount),
		attribute.Int("notification.failed", result.FailedCount),
		attribute.Int("notification.retry", result.RetryCount),
	)
	return result, nil
}

func (s *NotificationWorkerService) processJob(ctx context.Context, job domain.NotificationJob, now time.Time) (domain.NotificationJobStatus, error) {
	content := notificationPayloadString(job.Payload, "content")
	toUser := notificationPayloadString(job.Payload, "to_user")
	if content == "" || toUser == "" || job.Channel != domain.NotificationChannelWeChatWork {
		workerID := job.LockedBy
		job.Status = domain.NotificationJobStatusFailed
		job.LastError = "unsupported or incomplete notification job"
		finishedAt := now.UTC()
		job.FinishedAt = &finishedAt
		job.LockedBy = ""
		job.LockedAt = nil
		job.LeaseUntil = nil
		_, err := s.updateJob(ctx, job, workerID)
		if err == nil {
			metrics.TaskQueueDeadLettersTotal.WithLabelValues("notification").Inc()
		}
		return job.Status, err
	}
	sendResult, err := s.sender.SendText(ctx, notifier.WeChatWorkTextMessage{
		ToUser:  toUser,
		Content: content,
	})
	deliveryStatus := domain.NotificationDeliveryStatusSucceeded
	errorMessage := ""
	if err != nil {
		deliveryStatus = domain.NotificationDeliveryStatusFailed
		errorMessage = truncateError(err.Error(), notificationWorkerLastErrorMaxLength)
	}
	sentAt := now.UTC()
	if _, deliveryErr := s.store.CreateDelivery(ctx, domain.NotificationDelivery{
		NotificationJobID: job.ID,
		UserID:            job.UserID,
		Channel:           job.Channel,
		Status:            deliveryStatus,
		RequestID:         job.RequestID,
		TraceID:           job.TraceID,
		ProviderMessageID: sendResult.MessageID,
		ResponseBody:      sendResult.ResponseBody,
		ErrorMessage:      errorMessage,
		SentAt:            &sentAt,
	}); deliveryErr != nil {
		return job.Status, deliveryErr
	}
	if err == nil {
		job.Status = domain.NotificationJobStatusSucceeded
		job.LastError = ""
		finishedAt := now.UTC()
		job.FinishedAt = &finishedAt
		workerID := job.LockedBy
		job.LockedBy = ""
		job.LockedAt = nil
		job.LeaseUntil = nil
		_, updateErr := s.updateJob(ctx, job, workerID)
		return job.Status, updateErr
	}
	workerID := job.LockedBy
	job.LastError = errorMessage
	if job.AttemptCount < job.MaxAttempts {
		job.Status = domain.NotificationJobStatusQueued
		job.ScheduledAt = now.Add(notificationRetryDelay(job.AttemptCount)).UTC()
		job.StartedAt = nil
		job.FinishedAt = nil
	} else {
		job.Status = domain.NotificationJobStatusFailed
		finishedAt := now.UTC()
		job.FinishedAt = &finishedAt
	}
	job.LockedBy = ""
	job.LockedAt = nil
	job.LeaseUntil = nil
	_, updateErr := s.updateJob(ctx, job, workerID)
	if updateErr == nil {
		if job.Status == domain.NotificationJobStatusQueued {
			metrics.TaskQueueRetriesTotal.WithLabelValues("notification").Inc()
		} else {
			metrics.TaskQueueDeadLettersTotal.WithLabelValues("notification").Inc()
		}
	}
	return job.Status, updateErr
}

func (s *NotificationWorkerService) updateJob(ctx context.Context, job domain.NotificationJob, workerID string) (domain.NotificationJob, error) {
	if owned, ok := s.store.(ownedNotificationWorkerStore); ok && strings.TrimSpace(workerID) != "" {
		return owned.UpdateJobIfOwned(ctx, job, workerID)
	}
	return s.store.UpdateJob(ctx, job)
}

func notificationRetryDelay(attemptCount int) time.Duration {
	if attemptCount < 1 {
		attemptCount = 1
	}
	if attemptCount > 5 {
		attemptCount = 5
	}
	return time.Duration(attemptCount*attemptCount) * time.Minute
}

func notificationPayloadString(payload domain.NotificationPayload, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
