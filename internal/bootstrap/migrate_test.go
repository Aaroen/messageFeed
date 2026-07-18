package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMigrationVersion(t *testing.T) {
	version, err := parseMigrationVersion("37\n")
	if err != nil || version != 37 {
		t.Fatalf("parseMigrationVersion() = %d, %v", version, err)
	}
	if _, err := parseMigrationVersion("37 (dirty)"); err == nil {
		t.Fatal("parseMigrationVersion(dirty) error = nil")
	}
	if _, err := parseMigrationVersion(""); err == nil {
		t.Fatal("parseMigrationVersion(empty) error = nil")
	}
}

func TestValidatePendingMigrationsEnforcesExpandContract(t *testing.T) {
	tests := []struct {
		name           string
		filename       string
		sql            string
		currentVersion uint64
		phase          string
		wantError      string
	}{
		{
			name:           "expand accepts additive schema",
			filename:       "000038_expand_add_probe.up.sql",
			sql:            "CREATE TABLE migration_probe (id BIGINT PRIMARY KEY);",
			currentVersion: 37,
			phase:          "expand",
		},
		{
			name:           "expand rejects destructive schema",
			filename:       "000038_expand_drop_probe.up.sql",
			sql:            "ALTER TABLE migration_probe DROP COLUMN obsolete_value;",
			currentVersion: 37,
			phase:          "expand",
			wantError:      "destructive operation DROP",
		},
		{
			name:           "comments and string literals are ignored",
			filename:       "000038_expand_add_comment.up.sql",
			sql:            "-- DROP TABLE ignored\nCOMMENT ON TABLE migration_probe IS 'DELETE FROM ignored';",
			currentVersion: 37,
			phase:          "expand",
		},
		{
			name:           "procedural destructive schema is rejected",
			filename:       "000038_expand_procedural_drop.up.sql",
			sql:            "DO $$ BEGIN ALTER TABLE migration_probe DROP CONSTRAINT old_constraint; END $$;",
			currentVersion: 37,
			phase:          "expand",
			wantError:      "destructive operation DROP",
		},
		{
			name:           "contract accepts explicit destructive schema",
			filename:       "000038_contract_drop_probe.up.sql",
			sql:            "DROP TABLE migration_probe;",
			currentVersion: 37,
			phase:          "contract",
		},
		{
			name:           "phase mismatch is rejected",
			filename:       "000038_contract_drop_probe.up.sql",
			sql:            "DROP TABLE migration_probe;",
			currentVersion: 37,
			phase:          "expand",
			wantError:      "MIGRATION_PHASE=expand",
		},
		{
			name:           "missing phase marker is rejected",
			filename:       "000038_add_probe.up.sql",
			sql:            "CREATE TABLE migration_probe (id BIGINT PRIMARY KEY);",
			currentVersion: 37,
			phase:          "expand",
			wantError:      "must include _expand_ or _contract_",
		},
		{
			name:           "applied contract migration is ignored",
			filename:       "000038_contract_drop_probe.up.sql",
			sql:            "DROP TABLE migration_probe;",
			currentVersion: 38,
			phase:          "expand",
		},
		{
			name:           "legacy migration is below policy baseline",
			filename:       "000001_init_schema.up.sql",
			sql:            "DROP TABLE legacy_probe;",
			currentVersion: 0,
			phase:          "expand",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, test.filename), []byte(test.sql), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			err := validatePendingMigrations(directory, test.currentVersion, test.phase)
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("validatePendingMigrations() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("validatePendingMigrations() error = %v, want %q", err, test.wantError)
			}
		})
	}
}
