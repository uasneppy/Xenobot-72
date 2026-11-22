package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const timedPunishmentMigration = "20261122120000_add_timed_punishment_durations.sql"

// timedPunishmentColumns are the three duration columns the migration adds, in file order.
var timedPunishmentColumns = []struct{ table, column, constraint string }{
	{"warns_settings", "warn_mode_secs", "chk_warns_settings_mode_secs"},
	{"antiflood_settings", "action_secs", "chk_antiflood_action_secs"},
	{"blacklists", "action_secs", "chk_blacklists_action_secs"},
}

// timedPunishmentStatements reads the timed punishment migration and returns the statements the
// runner would execute, dropping empty and transaction-control statements exactly as
// applyMigration does.
func timedPunishmentStatements(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", timedPunishmentMigration))
	if err != nil {
		t.Fatalf("resolve the migration path: %v", err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", timedPunishmentMigration, err)
	}
	runner := &MigrationRunner{}
	var run []string
	for _, stmt := range runner.splitSQLStatements(cleanSupabaseSQL(string(content))) {
		if strings.TrimSpace(stmt) == "" || isTransactionControlStatement(stmt) {
			continue
		}
		run = append(run, stmt)
	}
	return run
}

// TestTimedPunishmentMigrationStatements proves, without a database, what the Go runner hands to
// PostgreSQL for the timed punishment migration: three idempotent column additions, each INTEGER
// NOT NULL DEFAULT 0 with its named CHECK 0..31622400 inline, no transaction control, no
// CONCURRENTLY, no NOT VALID and no top-level constraint addition, and a file name that sorts after
// the event message options migration, the latest before it.
func TestTimedPunishmentMigrationStatements(t *testing.T) {
	run := timedPunishmentStatements(t)
	for i, stmt := range run {
		run[i] = stripSQLComments(stmt)
	}
	if len(run) != len(timedPunishmentColumns) {
		t.Fatalf("%s gives %d statements, want %d (one per column):\n%s",
			timedPunishmentMigration, len(run), len(timedPunishmentColumns), strings.Join(run, "\n---\n"))
	}
	for i, want := range timedPunishmentColumns {
		stmt := run[i]
		if !strings.Contains(stmt, "ALTER TABLE "+want.table+" ADD COLUMN IF NOT EXISTS "+want.column+" ") {
			t.Errorf("statement %d must add %s to %s idempotently, got:\n%s", i+1, want.column, want.table, stmt)
		}
		if !strings.Contains(stmt, want.column+" INTEGER NOT NULL DEFAULT 0 ") {
			t.Errorf("statement %d: %s must be INTEGER NOT NULL DEFAULT 0, got:\n%s", i+1, want.column, stmt)
		}
		if !strings.Contains(stmt, "CONSTRAINT "+want.constraint+" CHECK ("+want.column+" BETWEEN 0 AND 31622400)") {
			t.Errorf("statement %d must carry constraint %s inline, got:\n%s", i+1, want.constraint, stmt)
		}
	}
	for i, stmt := range run {
		upper := strings.ToUpper(stmt)
		if isTransactionControlStatement(stmt) {
			t.Errorf("statement %d is transaction control: %s", i+1, stmt)
		}
		for _, banned := range []string{"CONCURRENTLY", "NOT VALID", "BEGIN", "COMMIT", "ROLLBACK", "ADD CONSTRAINT"} {
			if strings.Contains(upper, banned) {
				t.Errorf("statement %d contains %s:\n%s", i+1, banned, stmt)
			}
		}
	}

	if timedPunishmentMigration <= "20261120120000_add_event_message_options.sql" {
		t.Errorf("%s must sort after 20261120120000_add_event_message_options.sql", timedPunishmentMigration)
	}
}

// TestTimedPunishmentMigrationDefaults proves, on PostgreSQL only, that after the migration chain
// the three duration columns exist, that the migration's statements run a second time without
// error, that a row inserted with only its required columns reads 0 (no duration, inert), that each
// named constraint exists, and that 31622401 is refused with SQLSTATE 23514 naming the constraint
// while 31622400 is accepted.
func TestTimedPunishmentMigrationDefaults(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("FUKU_TEST_DATABASE")), "true") {
		t.Skip("set FUKU_TEST_DATABASE=true and DATABASE_URL to run the timed punishment migration test on PostgreSQL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required to run the timed punishment migration test on PostgreSQL")
	}

	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL handle: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close PostgreSQL: %v", err)
		}
	})

	for _, c := range timedPunishmentColumns {
		if !database.Migrator().HasTable(c.table) {
			t.Fatalf("table %s is missing: the migration chain must run before this test", c.table)
		}
	}

	run := timedPunishmentStatements(t)
	tx := database.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	// Never committed: the rows below vanish, and so does the second run of the migration.
	defer func() {
		if err := tx.Rollback().Error; err != nil {
			t.Errorf("roll back the timed punishment transaction: %v", err)
		}
	}()
	for i, stmt := range run {
		if err := tx.Exec(stmt).Error; err != nil {
			t.Fatalf("second run: statement %d/%d of %s failed: %v", i+1, len(run), timedPunishmentMigration, err)
		}
	}

	chatID := captchaModeChatID(t)
	now := time.Now().UTC()
	inserts := []struct{ what, sql string }{
		{"chat", `INSERT INTO chats (chat_id, chat_name, language, is_inactive, created_at, updated_at) VALUES (?, 'timed punishment test', 'en', FALSE, ?, ?)`},
		{"warns_settings", `INSERT INTO warns_settings (chat_id, created_at, updated_at) VALUES (?, ?, ?)`},
		{"antiflood_settings", `INSERT INTO antiflood_settings (chat_id, created_at, updated_at) VALUES (?, ?, ?)`},
		{"blacklists", `INSERT INTO blacklists (chat_id, word, created_at, updated_at) VALUES (?, 'probe', ?, ?)`},
	}
	for _, in := range inserts {
		if err := tx.Exec(in.sql, chatID, now, now).Error; err != nil {
			t.Fatalf("insert a %s row with only its required columns: %v", in.what, err)
		}
	}

	for _, c := range timedPunishmentColumns {
		var stored []int64
		if err := tx.Raw("SELECT "+c.column+" FROM "+c.table+" WHERE chat_id = ?", chatID).Scan(&stored).Error; err != nil {
			t.Fatalf("read %s.%s: %v", c.table, c.column, err)
		}
		if len(stored) != 1 || stored[0] != 0 {
			t.Errorf("%s.%s defaults to %v, want one row reading 0", c.table, c.column, stored)
		}

		var names []string
		if err := tx.Raw(`SELECT conname FROM pg_constraint WHERE conrelid = ?::regclass`, c.table).Scan(&names).Error; err != nil {
			t.Fatalf("list the constraints of %s: %v", c.table, err)
		}
		found := false
		for _, name := range names {
			found = found || name == c.constraint
		}
		if !found {
			t.Errorf("constraint %s is missing from %s (found %v)", c.constraint, c.table, names)
		}

		update := func(value int64) error {
			// A refused statement aborts the transaction in PostgreSQL, so it runs behind a savepoint.
			if err := tx.Exec("SAVEPOINT bad_secs").Error; err != nil {
				t.Fatalf("savepoint: %v", err)
			}
			err := tx.Exec("UPDATE "+c.table+" SET "+c.column+" = ? WHERE chat_id = ?", value, chatID).Error
			if rollbackErr := tx.Exec("ROLLBACK TO SAVEPOINT bad_secs").Error; rollbackErr != nil {
				t.Fatalf("roll back to savepoint: %v", rollbackErr)
			}
			return err
		}
		if err := update(31622400); err != nil {
			t.Errorf("%s.%s = 31622400 must be accepted, got: %v", c.table, c.column, err)
		}
		err := update(31622401)
		if err == nil {
			t.Errorf("%s.%s = 31622401 was accepted by the database", c.table, c.column)
		} else if !strings.Contains(err.Error(), "SQLSTATE 23514") || !strings.Contains(err.Error(), c.constraint) {
			t.Errorf("%s.%s = 31622401 failed with %v, want SQLSTATE 23514 naming %s", c.table, c.column, err, c.constraint)
		}
	}
}
