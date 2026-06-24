package repository

import (
	"os"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestUserProfileModelTimezoneColumn(t *testing.T) {
	parsed, err := schema.Parse(&userProfileModel{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse userProfileModel schema: %v", err)
	}

	field := parsed.LookUpField("TimeZone")
	if field == nil {
		t.Fatal("TimeZone field not found")
	}
	if field.DBName != "timezone" {
		t.Fatalf("TimeZone DBName = %q, want timezone", field.DBName)
	}
}

func TestStringListJSONEmptyValue(t *testing.T) {
	value, err := stringListJSON(nil).Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	if value != "[]" {
		t.Fatalf("Value() = %#v, want []", value)
	}
}

func TestUserIDSequenceMigrationSynchronizesUsersSequence(t *testing.T) {
	body, err := os.ReadFile("../../migrations/000017_sync_user_id_sequence.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(body))
	for _, want := range []string{
		"setval",
		"pg_get_serial_sequence('users', 'id')",
		"max(id) from users",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration does not contain %q: %s", want, sql)
		}
	}
}
