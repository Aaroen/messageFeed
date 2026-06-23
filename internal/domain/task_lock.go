package domain

import "time"

type TaskLock struct {
	Name        string
	Owner       string
	LockedUntil time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
