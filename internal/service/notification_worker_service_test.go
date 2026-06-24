package service

import (
	"context"
	"messagefeed/internal/domain"
	"messagefeed/internal/notifier"
	"testing"
	"time"
)

func TestNotificationWorkerSendsDueWeChatWorkJob(t *testing.T) {
	now := time.Date(2026, 6, 24, 21, 0, 0, 0, time.UTC)
	store := &fakeNotificationWorkerStore{
		jobs: []domain.NotificationJob{
			{
				ID:          1,
				UserID:      2,
				Status:      domain.NotificationJobStatusQueued,
				Channel:     domain.NotificationChannelWeChatWork,
				Payload:     domain.NotificationPayload{"to_user": "zhangsan", "content": "检查部署状态"},
				ScheduledAt: now.Add(-time.Minute),
				MaxAttempts: 3,
			},
		},
	}
	sender := &fakeNotificationSender{result: notifier.WeChatWorkSendResult{MessageID: "wx-1", ResponseBody: `{"errcode":0}`}}
	worker := NewNotificationWorkerService(store, sender, WithNotificationWorkerNow(func() time.Time { return now }))

	result, err := worker.RunOnce(context.Background(), RunNotificationWorkerOnceInput{WorkerID: "worker-a", Limit: 10})
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.ClaimedCount != 1 || result.SucceededCount != 1 {
		t.Fatalf("result = %#v", result)
	}
	if len(sender.messages) != 1 || sender.messages[0].ToUser != "zhangsan" || sender.messages[0].Content != "检查部署状态" {
		t.Fatalf("sent messages = %#v", sender.messages)
	}
	if len(store.deliveries) != 1 || store.deliveries[0].Status != domain.NotificationDeliveryStatusSucceeded {
		t.Fatalf("deliveries = %#v", store.deliveries)
	}
	if store.jobs[0].Status != domain.NotificationJobStatusSucceeded {
		t.Fatalf("job status = %q", store.jobs[0].Status)
	}
}

type fakeNotificationWorkerStore struct {
	jobs       []domain.NotificationJob
	deliveries []domain.NotificationDelivery
}

func (s *fakeNotificationWorkerStore) ClaimDueJobs(_ context.Context, input domain.NotificationJobClaimInput) ([]domain.NotificationJob, error) {
	claimed := make([]domain.NotificationJob, 0)
	for i := range s.jobs {
		job := &s.jobs[i]
		if job.Status != domain.NotificationJobStatusQueued || job.ScheduledAt.After(input.Now) {
			continue
		}
		job.Status = domain.NotificationJobStatusRunning
		job.AttemptCount++
		job.LockedBy = input.WorkerID
		lockedAt := input.Now
		job.LockedAt = &lockedAt
		claimed = append(claimed, *job)
		if input.Limit > 0 && len(claimed) >= input.Limit {
			break
		}
	}
	return claimed, nil
}

func (s *fakeNotificationWorkerStore) UpdateJob(_ context.Context, job domain.NotificationJob) (domain.NotificationJob, error) {
	for i := range s.jobs {
		if s.jobs[i].ID == job.ID {
			s.jobs[i] = job
			return job, nil
		}
	}
	s.jobs = append(s.jobs, job)
	return job, nil
}

func (s *fakeNotificationWorkerStore) CreateDelivery(_ context.Context, delivery domain.NotificationDelivery) (domain.NotificationDelivery, error) {
	delivery.ID = int64(len(s.deliveries) + 1)
	s.deliveries = append(s.deliveries, delivery)
	return delivery, nil
}

type fakeNotificationSender struct {
	result   notifier.WeChatWorkSendResult
	messages []notifier.WeChatWorkTextMessage
}

func (s *fakeNotificationSender) SendText(_ context.Context, message notifier.WeChatWorkTextMessage) (notifier.WeChatWorkSendResult, error) {
	s.messages = append(s.messages, message)
	return s.result, nil
}
