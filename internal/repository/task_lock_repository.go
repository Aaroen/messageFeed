package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"messagefeed/internal/domain"
	"strings"
	"time"

	"gorm.io/gorm"
)

type TaskLockRepository struct {
	db    *gorm.DB
	owner func() string
	now   func() time.Time
}

type TaskLockRepositoryOption func(*TaskLockRepository)

func WithTaskLockOwner(owner func() string) TaskLockRepositoryOption {
	return func(repository *TaskLockRepository) {
		if owner != nil {
			repository.owner = owner
		}
	}
}

func WithTaskLockNow(now func() time.Time) TaskLockRepositoryOption {
	return func(repository *TaskLockRepository) {
		if now != nil {
			repository.now = now
		}
	}
}

func NewTaskLockRepository(db *gorm.DB, options ...TaskLockRepositoryOption) *TaskLockRepository {
	repository := &TaskLockRepository{
		db:    db,
		owner: newTaskLockOwner,
		now:   time.Now,
	}
	for _, option := range options {
		option(repository)
	}
	return repository
}

type taskLockModel struct {
	Name        string `gorm:"primaryKey"`
	Owner       string
	LockedUntil time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (taskLockModel) TableName() string {
	return "task_locks"
}

func (r *TaskLockRepository) WithLock(ctx context.Context, name string, ttl time.Duration, run func(context.Context) error) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("task lock repository is not configured")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: task lock name must not be empty", domain.ErrInvalidInput)
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	owner := r.owner()
	now := r.now().UTC()
	lock, err := r.acquire(ctx, name, owner, now, ttl)
	if err != nil {
		return err
	}
	if lock.Owner != owner {
		return domain.NewAppError(domain.ErrorKindUnavailable, "TASK_LOCK_BUSY", "task lock is busy", "repository.task_lock.acquire", true, nil)
	}
	defer func() {
		_ = r.release(context.Background(), name, owner)
	}()
	return run(ctx)
}

func (r *TaskLockRepository) acquire(ctx context.Context, name string, owner string, now time.Time, ttl time.Duration) (domain.TaskLock, error) {
	ctx, finish := traceRepositoryOperation(ctx, "repository.task_lock.acquire", "upsert", "task_locks")
	var opErr error
	defer func() { finish(opErr) }()

	model := taskLockModel{}
	result := r.db.WithContext(ctx).Raw(`
		INSERT INTO task_locks (name, owner, locked_until, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (name) DO UPDATE
		SET owner = EXCLUDED.owner,
			locked_until = EXCLUDED.locked_until,
			updated_at = EXCLUDED.updated_at
		WHERE task_locks.locked_until <= ? OR task_locks.owner = ?
		RETURNING name, owner, locked_until, created_at, updated_at`,
		name, owner, now.Add(ttl), now, now, now, owner,
	).Scan(&model)
	if result.Error != nil {
		opErr = mapRepositoryError(result.Error)
		return domain.TaskLock{}, opErr
	}
	if result.RowsAffected == 0 {
		opErr = domain.NewAppError(domain.ErrorKindUnavailable, "TASK_LOCK_BUSY", "task lock is busy", "repository.task_lock.acquire", true, nil)
		return domain.TaskLock{}, opErr
	}
	return taskLockModelToDomain(model), nil
}

func (r *TaskLockRepository) release(ctx context.Context, name string, owner string) error {
	ctx, finish := traceRepositoryOperation(ctx, "repository.task_lock.release", "update", "task_locks")
	var opErr error
	defer func() { finish(opErr) }()

	now := r.now().UTC()
	err := r.db.WithContext(ctx).
		Model(&taskLockModel{}).
		Where("name = ? AND owner = ?", name, owner).
		Updates(map[string]interface{}{
			"locked_until": now,
			"updated_at":   now,
		}).Error
	if err != nil {
		opErr = mapRepositoryError(err)
		return opErr
	}
	return nil
}

func taskLockModelToDomain(model taskLockModel) domain.TaskLock {
	return domain.TaskLock{
		Name:        model.Name,
		Owner:       model.Owner,
		LockedUntil: model.LockedUntil,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}
}

func newTaskLockOwner() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("owner-%d", time.Now().UTC().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
