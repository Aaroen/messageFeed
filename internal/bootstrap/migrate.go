package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	migrationPolicyBaselineVersion uint64 = 37
	migrationAdvisoryLockKey       int64  = 0x4d45535341474546
)

var (
	migrationFilenamePattern     = regexp.MustCompile(`^([0-9]+)_(.+)\.up\.sql$`)
	destructiveMigrationPatterns = []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{name: "DROP", pattern: regexp.MustCompile(`\bDROP\s+(TABLE|COLUMN|TYPE|VIEW|MATERIALIZED\s+VIEW|FUNCTION|TRIGGER|CONSTRAINT|INDEX)\b`)},
		{name: "RENAME", pattern: regexp.MustCompile(`\bALTER\s+TABLE\b[\s\S]*?\bRENAME\s+(TO|COLUMN)\b`)},
		{name: "ALTER COLUMN TYPE", pattern: regexp.MustCompile(`\bALTER\s+TABLE\b[\s\S]*?\bALTER\s+COLUMN\b[\s\S]*?\bTYPE\b`)},
		{name: "SET NOT NULL", pattern: regexp.MustCompile(`\bALTER\s+TABLE\b[\s\S]*?\bALTER\s+COLUMN\b[\s\S]*?\bSET\s+NOT\s+NULL\b`)},
		{name: "TRUNCATE", pattern: regexp.MustCompile(`\bTRUNCATE\b`)},
		{name: "DELETE", pattern: regexp.MustCompile(`\bDELETE\s+FROM\b`)},
	}
)

// MigrationRunner executes the application's database migrations.
// The interface keeps the process launcher testable without invoking a real CLI.
type MigrationRunner interface {
	Run(context.Context, string, string) error
}

type commandMigrationRunner struct {
	lockTimeout time.Duration
	phase       string
}

func (runner commandMigrationRunner) Run(ctx context.Context, databaseURL string, migrationsPath string) error {
	lockConnection, err := acquireMigrationAdvisoryLock(ctx, databaseURL, runner.lockTimeout)
	if err != nil {
		return err
	}
	defer closeMigrationLockConnection(lockConnection)

	currentVersion, err := currentMigrationVersion(ctx, databaseURL, migrationsPath)
	if err != nil {
		return err
	}
	if err := validatePendingMigrations(migrationsPath, currentVersion, runner.phase); err != nil {
		return err
	}

	lockTimeoutSeconds := int(runner.lockTimeout / time.Second)
	command := exec.CommandContext(
		ctx,
		"migrate",
		"-path", migrationsPath,
		"-database", databaseURL,
		"-lock-timeout", strconv.Itoa(lockTimeoutSeconds),
		"up",
	)
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	message := limitedCommandOutput(output)
	if message == "" {
		return fmt.Errorf("run database migrations: %w", err)
	}
	return fmt.Errorf("run database migrations: %s: %w", message, err)
}

func acquireMigrationAdvisoryLock(ctx context.Context, databaseURL string, timeout time.Duration) (*pgx.Conn, error) {
	lockCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	connection, err := pgx.Connect(lockCtx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect for database migration lock: %w", err)
	}
	if _, err := connection.Exec(lockCtx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockKey); err != nil {
		_ = connection.Close(context.Background())
		if lockCtx.Err() != nil {
			return nil, fmt.Errorf("acquire database migration lock within %s: %w", timeout, lockCtx.Err())
		}
		return nil, fmt.Errorf("acquire database migration lock: %w", err)
	}
	return connection, nil
}

func closeMigrationLockConnection(connection *pgx.Conn) {
	if connection == nil {
		return
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = connection.Close(closeCtx)
}

func currentMigrationVersion(ctx context.Context, databaseURL string, migrationsPath string) (uint64, error) {
	command := exec.CommandContext(ctx, "migrate", "-path", migrationsPath, "-database", databaseURL, "version")
	output, err := command.CombinedOutput()
	message := limitedCommandOutput(output)
	if strings.Contains(strings.ToLower(message), "no migration") {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read database migration version: %s: %w", message, err)
	}
	return parseMigrationVersion(message)
}

func parseMigrationVersion(output string) (uint64, error) {
	fields := strings.Fields(strings.TrimSpace(output))
	if len(fields) == 0 {
		return 0, fmt.Errorf("read database migration version: empty output")
	}
	version, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("read database migration version %q: %w", fields[0], err)
	}
	if strings.Contains(strings.ToLower(output), "dirty") {
		return 0, fmt.Errorf("database migration version %d is dirty; manual recovery is required", version)
	}
	return version, nil
}

func validatePendingMigrations(migrationsPath string, currentVersion uint64, phase string) error {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}
	threshold := currentVersion
	if threshold < migrationPolicyBaselineVersion {
		threshold = migrationPolicyBaselineVersion
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		matches := migrationFilenamePattern.FindStringSubmatch(entry.Name())
		if len(matches) != 3 {
			return fmt.Errorf("migration policy: invalid up migration filename %q", entry.Name())
		}
		version, err := strconv.ParseUint(matches[1], 10, 64)
		if err != nil {
			return fmt.Errorf("migration policy: parse version from %q: %w", entry.Name(), err)
		}
		if version <= threshold {
			continue
		}

		description := "_" + strings.ToLower(matches[2]) + "_"
		filePhase := ""
		switch {
		case strings.Contains(description, "_expand_"):
			filePhase = "expand"
		case strings.Contains(description, "_contract_"):
			filePhase = "contract"
		default:
			return fmt.Errorf("migration policy: %s must include _expand_ or _contract_ in its filename", entry.Name())
		}
		if filePhase != phase {
			return fmt.Errorf("migration policy: %s is a %s migration but MIGRATION_PHASE=%s", entry.Name(), filePhase, phase)
		}
		if filePhase == "contract" {
			continue
		}

		contents, err := os.ReadFile(filepath.Join(migrationsPath, entry.Name()))
		if err != nil {
			return fmt.Errorf("migration policy: read %s: %w", entry.Name(), err)
		}
		normalized := normalizeSQLForMigrationPolicy(string(contents))
		for _, destructive := range destructiveMigrationPatterns {
			if destructive.pattern.MatchString(normalized) {
				return fmt.Errorf("migration policy: expand migration %s contains destructive operation %s", entry.Name(), destructive.name)
			}
		}
	}
	return nil
}

func normalizeSQLForMigrationPolicy(sql string) string {
	var normalized strings.Builder
	for index := 0; index < len(sql); {
		switch {
		case strings.HasPrefix(sql[index:], "--"):
			if end := strings.IndexByte(sql[index:], '\n'); end >= 0 {
				index += end + 1
				normalized.WriteByte(' ')
				continue
			}
			index = len(sql)
		case strings.HasPrefix(sql[index:], "/*"):
			if end := strings.Index(sql[index+2:], "*/"); end >= 0 {
				index += end + 4
				normalized.WriteByte(' ')
				continue
			}
			index = len(sql)
		case sql[index] == '\'' || sql[index] == '"':
			quote := sql[index]
			index++
			for index < len(sql) {
				if sql[index] != quote {
					index++
					continue
				}
				if index+1 < len(sql) && sql[index+1] == quote {
					index += 2
					continue
				}
				index++
				break
			}
			normalized.WriteByte(' ')
		case sql[index] == '$':
			delimiterEnd := strings.IndexByte(sql[index+1:], '$')
			if delimiterEnd < 0 {
				normalized.WriteByte(sql[index])
				index++
				continue
			}
			delimiterEnd += index + 1
			delimiter := sql[index : delimiterEnd+1]
			if !validDollarQuoteDelimiter(delimiter) {
				normalized.WriteByte(sql[index])
				index++
				continue
			}
			bodyStart := delimiterEnd + 1
			closing := strings.Index(sql[bodyStart:], delimiter)
			if closing < 0 {
				normalized.WriteByte(sql[index])
				index++
				continue
			}
			bodyEnd := bodyStart + closing
			normalized.WriteString(normalizeSQLForMigrationPolicy(sql[bodyStart:bodyEnd]))
			index = bodyEnd + len(delimiter)
		default:
			normalized.WriteByte(sql[index])
			index++
		}
	}
	return strings.ToUpper(normalized.String())
}

func validDollarQuoteDelimiter(delimiter string) bool {
	if len(delimiter) < 2 || delimiter[0] != '$' || delimiter[len(delimiter)-1] != '$' {
		return false
	}
	for _, character := range delimiter[1 : len(delimiter)-1] {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '_' {
			continue
		}
		return false
	}
	return true
}

func limitedCommandOutput(output []byte) string {
	message := strings.TrimSpace(string(output))
	if len(message) > 2000 {
		message = message[:2000]
	}
	return message
}
