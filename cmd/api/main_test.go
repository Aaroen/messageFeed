package main

import (
	"strings"
	"testing"
)

func TestRunRejectsInvalidApplicationRoleBeforeStarting(t *testing.T) {
	t.Setenv("APP_ROLE", "invalid-role")

	err := run()
	if err == nil || !strings.Contains(err.Error(), "unsupported APP_ROLE") {
		t.Fatalf("run() error = %v, want unsupported role error", err)
	}
}
