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

const chatPresenceAIReactMigration = "20261128120000_add_chat_presence_ai_react_mode.sql"

// chatPresenceAIReactStatements reads the ai_react_mode migration and returns the statements the
// runner would execute, dropping empty and transaction-control statements as applyMigration does.
func chatPresenceAIReactStatements(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", chatPresenceAIReactMigration))
	if err != nil {
		t.Fatalf("resolve the migration path: %v", err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", chatPresenceAIReactMigration, err)
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

// TestChatPresenceAIReactMigrationStatements proves, without a database, what the Go runner hands
// to PostgreSQL for the ai_react_mode migration: one idempotent ALTER TABLE chat_presence with its
// named CHECK inline, no transaction control, no CONCURRENTLY, no NOT VALID, no top-level
// ADD CONSTRAINT and no foreign key, and a file name that sorts after 20261127120000_add_chat_presence.sql.
func TestChatPresenceAIReactMigrationStatements(t *testing.T) {
	run := chatPresenceAIReactStatements(t)
	if len(run) != 1 {
		t.Fatalf("%s gives %d statements, want 1:\n%s", chatPresenceAIReactMigration, len(run), strings.Join(run, "\n---\n"))
	}
	if !strings.HasPrefix(run[0], "ALTER TABLE chat_presence") {
		t.Errorf("unexpected statement:\n%s", run[0])
	}
	const constraintName = "chk_chat_presence_ai_react_mode"
	if !strings.Contains(run[0], "CONSTRAINT "+constraintName+" ") {
		t.Errorf("the statement must carry constraint %s inline", constraintName)
	}
	upper := strings.ToUpper(run[0])
	for _, banned := range []string{"CONCURRENTLY", "NOT VALID", "BEGIN", "COMMIT", "ROLLBACK", "ADD CONSTRAINT", "REFERENCES"} {
		if strings.Contains(upper, banned) {
			t.Errorf("the statement contains %s", banned)
		}
	}
	if chatPresenceAIReactMigration <= chatPresenceMigration {
		t.Errorf("%s must sort after %s", chatPresenceAIReactMigration, chatPresenceMigration)
	}
}

// TestChatPresenceAIReactMigrationDefaults proves, on PostgreSQL only, that the statement runs a
// second time without error, that a row with only chat_id defaults ai_react_mode to 'off', and
// that the named check refuses 'high' and '' (SQLSTATE 23514).
func TestChatPresenceAIReactMigrationDefaults(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("XENOBOT_TEST_DATABASE")), "true") {
		t.Skip("set XENOBOT_TEST_DATABASE=true and DATABASE_URL to run the chat presence ai_react_mode migration test on PostgreSQL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required to run the chat presence ai_react_mode migration test on PostgreSQL")
	}
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if !database.Migrator().HasTable("chat_presence") {
		t.Fatal("table chat_presence is missing: the migration chain must run before this test")
	}

	tx := database.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	defer func() { _ = tx.Rollback().Error }()

	for i, stmt := range chatPresenceAIReactStatements(t) {
		if err := tx.Exec(stmt).Error; err != nil {
			t.Fatalf("second run: statement %d failed: %v", i+1, err)
		}
	}
	chat := captchaModeChatID(t)
	if err := tx.Exec(`INSERT INTO chat_presence (chat_id) VALUES (?)`, chat).Error; err != nil {
		t.Fatalf("insert with only chat_id: %v", err)
	}
	var aiReactMode string
	if err := tx.Raw(`SELECT ai_react_mode FROM chat_presence WHERE chat_id = ?`, chat).Scan(&aiReactMode).Error; err != nil {
		t.Fatalf("read the default ai_react_mode: %v", err)
	}
	if aiReactMode != "off" {
		t.Errorf("default ai_react_mode = %q, want %q", aiReactMode, "off")
	}
	for _, tc := range []struct{ name, stmt, constraint string }{
		{"invalid mode high", `UPDATE chat_presence SET ai_react_mode = 'high' WHERE chat_id = ?`, "chk_chat_presence_ai_react_mode"},
		{"invalid mode empty", `UPDATE chat_presence SET ai_react_mode = '' WHERE chat_id = ?`, "chk_chat_presence_ai_react_mode"},
	} {
		if err := tx.Exec("SAVEPOINT refused").Error; err != nil {
			t.Fatal(err)
		}
		err := tx.Exec(tc.stmt, chat).Error
		if err == nil || !strings.Contains(err.Error(), "SQLSTATE 23514") || !strings.Contains(err.Error(), tc.constraint) {
			t.Errorf("%s: err %v, want 23514 naming %s", tc.name, err, tc.constraint)
		}
		if err := tx.Exec("ROLLBACK TO SAVEPOINT refused").Error; err != nil {
			t.Fatal(err)
		}
	}
}
