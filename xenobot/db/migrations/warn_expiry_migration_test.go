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

const warnExpiryMigration = "20261122130000_add_warn_expiry.sql"

// warnExpiryStatements reads the warn expiry migration and returns the statements the runner would
// execute, dropping empty and transaction-control statements exactly as applyMigration does.
func warnExpiryStatements(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", warnExpiryMigration))
	if err != nil {
		t.Fatalf("resolve the migration path: %v", err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", warnExpiryMigration, err)
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

// TestWarnExpiryMigrationStatements proves, without a database, what the Go runner hands to
// PostgreSQL for the warn expiry migration: two idempotent column additions (the expiry, INTEGER NOT
// NULL DEFAULT 0 with its named CHECK 0..365 inline, and the warn times, JSONB NOT NULL DEFAULT
// '[]'), no transaction control, no CONCURRENTLY, no NOT VALID and no top-level constraint addition,
// and a file name that sorts after the timed punishment migration, the latest before it.
func TestWarnExpiryMigrationStatements(t *testing.T) {
	run := warnExpiryStatements(t)
	for i, stmt := range run {
		run[i] = stripSQLComments(stmt)
	}
	if len(run) != 2 {
		t.Fatalf("%s gives %d statements, want 2:\n%s", warnExpiryMigration, len(run), strings.Join(run, "\n---\n"))
	}
	if want := "ALTER TABLE warns_settings ADD COLUMN IF NOT EXISTS warn_expiry_days INTEGER NOT NULL DEFAULT 0 CONSTRAINT chk_warns_settings_expiry CHECK (warn_expiry_days BETWEEN 0 AND 365)"; !strings.Contains(run[0], want) {
		t.Errorf("statement 1 must be %q, got:\n%s", want, run[0])
	}
	if want := "ALTER TABLE warns_users ADD COLUMN IF NOT EXISTS warn_times JSONB NOT NULL DEFAULT '[]'"; !strings.Contains(run[1], want) {
		t.Errorf("statement 2 must be %q, got:\n%s", want, run[1])
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
	if warnExpiryMigration <= timedPunishmentMigration {
		t.Errorf("%s must sort after %s", warnExpiryMigration, timedPunishmentMigration)
	}
}

// TestWarnExpiryMigrationDefaults proves, on PostgreSQL only, that after the migration chain the
// two columns exist, that the migration's statements run a second time without error, that rows
// inserted with only their required columns read 0 and '[]', that the named check exists on the
// expiry column (found by its column, conkey) and that 366 is refused with SQLSTATE 23514 naming it
// while 365 is accepted.
func TestWarnExpiryMigrationDefaults(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("XENOBOT_TEST_DATABASE")), "true") {
		t.Skip("set XENOBOT_TEST_DATABASE=true and DATABASE_URL to run the warn expiry migration test on PostgreSQL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required to run the warn expiry migration test on PostgreSQL")
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
	for _, table := range []string{"warns_settings", "warns_users"} {
		if !database.Migrator().HasTable(table) {
			t.Fatalf("table %s is missing: the migration chain must run before this test", table)
		}
	}

	run := warnExpiryStatements(t)
	tx := database.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	// Never committed: the rows below vanish, and so does the second run of the migration.
	defer func() {
		if err := tx.Rollback().Error; err != nil {
			t.Errorf("roll back the warn expiry transaction: %v", err)
		}
	}()
	for i, stmt := range run {
		if err := tx.Exec(stmt).Error; err != nil {
			t.Fatalf("second run: statement %d/%d of %s failed: %v", i+1, len(run), warnExpiryMigration, err)
		}
	}

	chatID := captchaModeChatID(t)
	userID := time.Now().UnixNano()%1_000_000_000 + 8_000_000_000
	now := time.Now().UTC()
	inserts := []struct {
		what, sql string
		args      []any
	}{
		{"chat", `INSERT INTO chats (chat_id, chat_name, language, is_inactive, created_at, updated_at) VALUES (?, 'warn expiry test', 'en', FALSE, ?, ?)`, []any{chatID, now, now}},
		{"user", `INSERT INTO users (user_id, created_at, updated_at) VALUES (?, ?, ?)`, []any{userID, now, now}},
		{"warns_settings", `INSERT INTO warns_settings (chat_id, created_at, updated_at) VALUES (?, ?, ?)`, []any{chatID, now, now}},
		{"warns_users", `INSERT INTO warns_users (user_id, chat_id, created_at, updated_at) VALUES (?, ?, ?, ?)`, []any{userID, chatID, now, now}},
	}
	for _, in := range inserts {
		if err := tx.Exec(in.sql, in.args...).Error; err != nil {
			t.Fatalf("insert a %s row with only its required columns: %v", in.what, err)
		}
	}

	var expiry []int64
	if err := tx.Raw(`SELECT warn_expiry_days FROM warns_settings WHERE chat_id = ?`, chatID).Scan(&expiry).Error; err != nil {
		t.Fatalf("read warn_expiry_days: %v", err)
	}
	if len(expiry) != 1 || expiry[0] != 0 {
		t.Errorf("warn_expiry_days defaults to %v, want one row reading 0", expiry)
	}
	var times []string
	if err := tx.Raw(`SELECT warn_times::text FROM warns_users WHERE chat_id = ?`, chatID).Scan(&times).Error; err != nil {
		t.Fatalf("read warn_times: %v", err)
	}
	if len(times) != 1 || times[0] != "[]" {
		t.Errorf("warn_times defaults to %v, want one row reading []", times)
	}

	var names []string
	if err := tx.Raw(`SELECT c.conname FROM pg_constraint c
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY (c.conkey)
		WHERE c.conrelid = 'warns_settings'::regclass AND c.contype = 'c' AND a.attname = 'warn_expiry_days'`).Scan(&names).Error; err != nil {
		t.Fatalf("list the checks of warn_expiry_days: %v", err)
	}
	if len(names) != 1 || names[0] != "chk_warns_settings_expiry" {
		t.Errorf("checks of warn_expiry_days = %v, want exactly chk_warns_settings_expiry", names)
	}

	update := func(value int64) error {
		// A refused statement aborts the transaction in PostgreSQL, so it runs behind a savepoint.
		if err := tx.Exec("SAVEPOINT bad_expiry").Error; err != nil {
			t.Fatalf("savepoint: %v", err)
		}
		err := tx.Exec(`UPDATE warns_settings SET warn_expiry_days = ? WHERE chat_id = ?`, value, chatID).Error
		if rollbackErr := tx.Exec("ROLLBACK TO SAVEPOINT bad_expiry").Error; rollbackErr != nil {
			t.Fatalf("roll back to savepoint: %v", rollbackErr)
		}
		return err
	}
	if err := update(365); err != nil {
		t.Errorf("warn_expiry_days = 365 must be accepted, got: %v", err)
	}
	if err := update(366); err == nil {
		t.Error("warn_expiry_days = 366 was accepted by the database")
	} else if !strings.Contains(err.Error(), "SQLSTATE 23514") || !strings.Contains(err.Error(), "chk_warns_settings_expiry") {
		t.Errorf("warn_expiry_days = 366 failed with %v, want SQLSTATE 23514 naming chk_warns_settings_expiry", err)
	}
}
