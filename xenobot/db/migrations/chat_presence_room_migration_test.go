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

const chatPresenceRoomMigration = "20261130120000_add_presence_room_and_catchup.sql"

// chatPresenceRoomStatements reads the Phase 34 presence room & catch-up migration and returns
// the statements the runner would execute, dropping empty and transaction-control statements as
// applyMigration does.
func chatPresenceRoomStatements(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", chatPresenceRoomMigration))
	if err != nil {
		t.Fatalf("resolve the migration path: %v", err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", chatPresenceRoomMigration, err)
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

// TestChatPresenceRoomMigrationStatements proves, without a database, what the Go runner hands to
// PostgreSQL for the Phase 34 presence migration: three idempotent ALTER TABLE chat_presence
// statements, no transaction control, no CONCURRENTLY, no NOT VALID, no ADD CONSTRAINT, and no
// foreign key, and a file name that sorts strictly after
// 20261129130000_add_presence_stickers_and_archive.sql.
func TestChatPresenceRoomMigrationStatements(t *testing.T) {
	run := chatPresenceRoomStatements(t)
	if len(run) != 3 {
		t.Fatalf("%s gives %d statements, want 3:\n%s", chatPresenceRoomMigration, len(run), strings.Join(run, "\n---\n"))
	}
	for i := range run {
		if !strings.HasPrefix(run[i], "ALTER TABLE chat_presence") {
			t.Errorf("statement %d unexpected:\n%s", i+1, run[i])
		}
	}

	joined := strings.Join(run, "\n")
	for _, colClause := range []string{
		"ADD COLUMN IF NOT EXISTS ambient_context BOOLEAN NOT NULL DEFAULT FALSE",
		"ADD COLUMN IF NOT EXISTS vocative_enabled BOOLEAN NOT NULL DEFAULT FALSE",
		"ADD COLUMN IF NOT EXISTS pm_brief_react BOOLEAN NOT NULL DEFAULT FALSE",
	} {
		if !strings.Contains(joined, colClause) {
			t.Errorf("migration missing clause %q", colClause)
		}
	}

	for i, stmt := range run {
		upper := strings.ToUpper(stmt)
		for _, banned := range []string{"CONCURRENTLY", "NOT VALID", "BEGIN", "COMMIT", "ROLLBACK", "ADD CONSTRAINT", "REFERENCES"} {
			if strings.Contains(upper, banned) {
				t.Errorf("statement %d contains %s", i+1, banned)
			}
		}
	}

	const previous = "20261129130000_add_presence_stickers_and_archive.sql"
	if chatPresenceRoomMigration <= previous {
		t.Errorf("%s must sort after %s", chatPresenceRoomMigration, previous)
	}
}

// TestChatPresenceRoomMigrationDefaults proves, on PostgreSQL only, that the statements run twice
// without error inside a rolled-back transaction and that ambient_context, vocative_enabled, and
// pm_brief_react default to false on chat_presence.
func TestChatPresenceRoomMigrationDefaults(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("XENOBOT_TEST_DATABASE")), "true") {
		t.Skip("set XENOBOT_TEST_DATABASE=true and DATABASE_URL to run the chat presence room migration test on PostgreSQL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required to run the chat presence room migration test on PostgreSQL")
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

	stmts := chatPresenceRoomStatements(t)
	for pass := 1; pass <= 2; pass++ {
		for i, stmt := range stmts {
			if err := tx.Exec(stmt).Error; err != nil {
				t.Fatalf("pass %d: statement %d failed: %v", pass, i+1, err)
			}
		}
	}

	chat := captchaModeChatID(t)
	if err := tx.Exec(`INSERT INTO chat_presence (chat_id) VALUES (?)`, chat).Error; err != nil {
		t.Fatalf("insert with only chat_id: %v", err)
	}
	var row struct {
		AmbientContext  bool
		VocativeEnabled bool
		PMBriefReact    bool
	}
	if err := tx.Raw(`
		SELECT ambient_context, vocative_enabled, pm_brief_react
		FROM chat_presence WHERE chat_id = ?`, chat).Scan(&row).Error; err != nil {
		t.Fatalf("read defaults: %v", err)
	}
	if row.AmbientContext || row.VocativeEnabled || row.PMBriefReact {
		t.Errorf("unexpected defaults: %+v, want all false", row)
	}
}
