package bootstrap

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// MigrationRunner executes the application's database migrations.
// The interface keeps the process launcher testable without invoking a real CLI.
type MigrationRunner interface {
	Run(context.Context, string, string) error
}

type commandMigrationRunner struct{}

func (commandMigrationRunner) Run(ctx context.Context, databaseURL string, migrationsPath string) error {
	command := exec.CommandContext(ctx, "migrate", "-path", migrationsPath, "-database", databaseURL, "up")
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(output))
	if len(message) > 2000 {
		message = message[:2000]
	}
	if message == "" {
		return fmt.Errorf("run database migrations: %w", err)
	}
	return fmt.Errorf("run database migrations: %s: %w", message, err)
}
