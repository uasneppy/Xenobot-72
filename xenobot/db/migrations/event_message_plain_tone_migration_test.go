package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const eventMessagePlainToneMigration = "20261123130000_add_event_message_plain_tone.sql"

// eventMessagePlainToneStatements reads the plain_tone migration and returns the statements the
// runner would execute, dropping empty and transaction-control statements as applyMigration does.
func eventMessagePlainToneStatements(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", eventMessagePlainToneMigration))
	if err != nil {
		t.Fatalf("resolve the migration path: %v", err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", eventMessagePlainToneMigration, err)
	}
	runner := &MigrationRunner{}
	var run []string
	for _, stmt := range runner.splitSQLStatements(cleanSupabaseSQL(string(content))) {
		if strings.TrimSpace(stmt) == "" || isTransactionControlStatement(stmt) {
			continue
		}
		run = append(run, stripSQLComments(stmt))
	}
	return run
}

// TestEventMessagePlainToneMigrationStatements proves, without a database, what the Go runner
// hands to PostgreSQL for the plain_tone migration: one idempotent ALTER TABLE adding
// plain_tone BOOLEAN NOT NULL DEFAULT FALSE to chat_event_message_options, no transaction
// control, no CONCURRENTLY, no NOT VALID, no ADD CONSTRAINT, no foreign key and no CHECK, and
// a file name that sorts after the previous migration.
func TestEventMessagePlainToneMigrationStatements(t *testing.T) {
	run := eventMessagePlainToneStatements(t)
	if len(run) != 1 {
		t.Fatalf("%s gives %d statements, want 1:\n%s", eventMessagePlainToneMigration, len(run), strings.Join(run, "\n---\n"))
	}
	stmt := run[0]
	if !strings.HasPrefix(stmt, "ALTER TABLE chat_event_message_options") {
		t.Errorf("the statement must start with ALTER TABLE chat_event_message_options, got:\n%s", stmt)
	}
	if !strings.Contains(stmt, "ADD COLUMN IF NOT EXISTS plain_tone BOOLEAN NOT NULL DEFAULT FALSE") {
		t.Errorf("the statement must add plain_tone BOOLEAN NOT NULL DEFAULT FALSE idempotently:\n%s", stmt)
	}
	upper := strings.ToUpper(stmt)
	if isTransactionControlStatement(stmt) {
		t.Errorf("the statement is transaction control: %s", stmt)
	}
	for _, banned := range []string{"CONCURRENTLY", "NOT VALID", "BEGIN", "COMMIT", "ROLLBACK", "ADD CONSTRAINT", "REFERENCES", "CHECK"} {
		if strings.Contains(upper, banned) {
			t.Errorf("the statement contains %s:\n%s", banned, stmt)
		}
	}
	if eventMessagePlainToneMigration <= advancedThresholdsMigration {
		t.Errorf("%s must sort after %s", eventMessagePlainToneMigration, advancedThresholdsMigration)
	}
}

// TestEventMessagePlainToneMigrationDefaults proves, on PostgreSQL only, that after the migration
// chain chat_event_message_options carries plain_tone, that running the migration statement a
// second time is idempotent without error, and that a row inserted with only chat_id reads
// plain_tone = FALSE.
func TestEventMessagePlainToneMigrationDefaults(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("XENOBOT_TEST_DATABASE")), "true") {
		t.Skip("set XENOBOT_TEST_DATABASE=true and DATABASE_URL to run the plain tone migration test on PostgreSQL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required to run the plain tone migration test on PostgreSQL")
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

	if !database.Migrator().HasTable("chat_event_message_options") {
		t.Fatalf("table chat_event_message_options is missing: the migration chain must run before this test")
	}

	run := eventMessagePlainToneStatements(t)
	tx := database.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	defer func() {
		if err := tx.Rollback().Error; err != nil {
			t.Errorf("roll back the plain tone transaction: %v", err)
		}
	}()

	for i, stmt := range run {
		if err := tx.Exec(stmt).Error; err != nil {
			t.Fatalf("second run: statement %d/%d of %s failed: %v", i+1, len(run), eventMessagePlainToneMigration, err)
		}
	}

	chat := captchaModeChatID(t)
	if err := tx.Exec(`INSERT INTO chat_event_message_options (chat_id) VALUES (?)`, chat).Error; err != nil {
		t.Fatalf("insert a row with only its key: %v", err)
	}
	var row struct {
		PMDetails bool `gorm:"column:pm_details"`
		PMLog     bool `gorm:"column:pm_log"`
		PlainTone bool `gorm:"column:plain_tone"`
	}
	if err := tx.Raw(`SELECT pm_details, pm_log, plain_tone FROM chat_event_message_options WHERE chat_id = ?`, chat).Scan(&row).Error; err != nil {
		t.Fatalf("read the defaults: %v", err)
	}
	if row.PMDetails || row.PMLog || row.PlainTone {
		t.Errorf("defaults = %+v, want all three options false", row)
	}
}
