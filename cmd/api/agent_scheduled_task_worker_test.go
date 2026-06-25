package main

import (
	"context"
	"io"
	"log/slog"
	"messagefeed/internal/domain"
	"messagefeed/internal/service"
	"testing"
	"time"
)

func TestRunAgentScheduledTaskWorkerProcessesOneTick(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &fakeAPIAgentScheduledTaskWorkerRepository{
		cancel: cancel,
		tasks: []domain.AgentScheduledTask{
			{
				ID:          1,
				UserID:      1,
				SessionID:   2,
				TurnID:      3,
				Status:      domain.AgentScheduledTaskStatusQueued,
				Goal:        "汇总新闻",
				ScheduledAt: now.Add(-time.Minute),
				MaxAttempts: 1,
			},
		},
	}
	worker := service.NewAgentScheduledTaskWorkerService(store, service.WithAgentScheduleEvalNow(func() time.Time { return now }))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	runAgentScheduledTaskWorker(ctx, logger, "node-a", worker)

	if len(store.runs) != 1 || store.runs[0].Status != domain.AgentRunStatusSucceeded {
		t.Fatalf("runs = %#v", store.runs)
	}
	if store.tasks[0].Status != domain.AgentScheduledTaskStatusSucceeded {
		t.Fatalf("task = %#v", store.tasks[0])
	}
	if len(store.audits) != 1 || store.audits[0].Status != "skipped" {
		t.Fatalf("audits = %#v", store.audits)
	}
}

type fakeAPIAgentScheduledTaskWorkerRepository struct {
	cancel func()
	tasks  []domain.AgentScheduledTask
	runs   []domain.AgentRun
	traces []domain.AgentRunContextTrace
	audits []domain.AgentAuditLog
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) CreateAgentScheduledTask(_ context.Context, task domain.AgentScheduledTask) (domain.AgentScheduledTask, error) {
	task.ID = int64(len(r.tasks) + 1)
	r.tasks = append(r.tasks, task)
	return task, nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) GetAgentScheduledTask(_ context.Context, userID int64, taskID int64) (domain.AgentScheduledTask, error) {
	for _, task := range r.tasks {
		if task.UserID == userID && task.ID == taskID {
			return task, nil
		}
	}
	return domain.AgentScheduledTask{}, domain.ErrNotFound
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) ListAgentScheduledTasks(_ context.Context, _ domain.AgentScheduledTaskListOptions) ([]domain.AgentScheduledTask, error) {
	return append([]domain.AgentScheduledTask(nil), r.tasks...), nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) ClaimDueAgentScheduledTasks(_ context.Context, input domain.AgentScheduledTaskClaimInput) ([]domain.AgentScheduledTask, error) {
	var claimed []domain.AgentScheduledTask
	for index, task := range r.tasks {
		if task.Status != domain.AgentScheduledTaskStatusQueued || task.ScheduledAt.After(input.Now) {
			continue
		}
		task.Status = domain.AgentScheduledTaskStatusRunning
		task.LockedBy = input.WorkerID
		task.AttemptCount++
		r.tasks[index] = task
		claimed = append(claimed, task)
	}
	return claimed, nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) UpdateAgentScheduledTask(_ context.Context, task domain.AgentScheduledTask) (domain.AgentScheduledTask, error) {
	for index, existing := range r.tasks {
		if existing.ID == task.ID {
			r.tasks[index] = task
			if r.cancel != nil {
				r.cancel()
			}
			return task, nil
		}
	}
	r.tasks = append(r.tasks, task)
	return task, nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) GetAgentNotificationPreference(_ context.Context, userID int64) (domain.AgentNotificationPreference, error) {
	return domain.AgentNotificationPreference{
		UserID:                       userID,
		ProcessNotificationsEnabled:  true,
		FinalReportsEnabled:          true,
		FailureNotificationsEnabled:  true,
		RecoveryNotificationsEnabled: true,
	}, nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) UpsertAgentNotificationPreference(_ context.Context, preference domain.AgentNotificationPreference) (domain.AgentNotificationPreference, error) {
	return preference, nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) CreateAgentRun(_ context.Context, run domain.AgentRun) (domain.AgentRun, error) {
	run.ID = int64(len(r.runs) + 1)
	r.runs = append(r.runs, run)
	return run, nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) UpdateAgentRun(_ context.Context, run domain.AgentRun) (domain.AgentRun, error) {
	for index, existing := range r.runs {
		if existing.ID == run.ID {
			r.runs[index] = run
			return run, nil
		}
	}
	r.runs = append(r.runs, run)
	return run, nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) CreateAgentRunContextTrace(_ context.Context, trace domain.AgentRunContextTrace) (domain.AgentRunContextTrace, error) {
	trace.ID = int64(len(r.traces) + 1)
	r.traces = append(r.traces, trace)
	return trace, nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) CreateAuditLog(_ context.Context, log domain.AgentAuditLog) (domain.AgentAuditLog, error) {
	log.ID = int64(len(r.audits) + 1)
	r.audits = append(r.audits, log)
	return log, nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) CreateAgentEvalCase(_ context.Context, evalCase domain.AgentEvalCase) (domain.AgentEvalCase, error) {
	return evalCase, nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) CreateAgentEvalRun(_ context.Context, run domain.AgentEvalRun) (domain.AgentEvalRun, error) {
	return run, nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) CreateAgentEvalResult(_ context.Context, result domain.AgentEvalResult) (domain.AgentEvalResult, error) {
	return result, nil
}

func (r *fakeAPIAgentScheduledTaskWorkerRepository) GetAgentEvalRunDetail(_ context.Context, runID int64) (domain.AgentEvalRun, error) {
	return domain.AgentEvalRun{ID: runID}, nil
}
