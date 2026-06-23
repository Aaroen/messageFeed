package repository

import (
	"testing"
	"time"
)

func TestTaskLockModelToDomain(t *testing.T) {
	now := time.Date(2026, 6, 23, 18, 0, 0, 0, time.UTC)
	model := taskLockModel{
		Name:        "source-sync",
		Owner:       "worker-a",
		LockedUntil: now.Add(time.Minute),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	lock := taskLockModelToDomain(model)

	if lock.Name != model.Name {
		t.Fatalf("Name = %q, want %q", lock.Name, model.Name)
	}
	if lock.Owner != model.Owner {
		t.Fatalf("Owner = %q, want %q", lock.Owner, model.Owner)
	}
	if !lock.LockedUntil.Equal(model.LockedUntil) {
		t.Fatalf("LockedUntil = %s, want %s", lock.LockedUntil, model.LockedUntil)
	}
}

func TestNewTaskLockOwner(t *testing.T) {
	owner := newTaskLockOwner()
	if owner == "" {
		t.Fatal("owner is empty")
	}
}
