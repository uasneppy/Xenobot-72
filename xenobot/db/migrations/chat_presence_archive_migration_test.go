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

const chatPresenceArchiveMigration = "20261129130000_add_presence_stickers_and_archive.sql"

// chatPresenceArchiveStatements reads the Phase 33 presence stickers & archive migration and
// returns the statements the runner would execute, dropping empty and transaction-control
// statements as applyMigration does.
func chatPresenceArchiveStatements(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", chatPresenceArchiveMigration))
	if err != nil {
		t.Fatalf("resolve the migration path: %v", err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", chatPresenceArchiveMigration, err)
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

// TestChatPresenceArchiveMigrationStatements proves, without a database, what the Go runner hands
// to PostgreSQL for the Phase 33 presence migration: seven idempotent ALTER TABLE chat_presence
// statements with inline named CHECK constraints and one CREATE TABLE IF NOT EXISTS
// chat_presence_archived statement with composite primary key, no transaction control, no
// CONCURRENTLY, no NOT VALID, no top-level ADD CONSTRAINT and no foreign key, and a file name that
// sorts strictly after 20261129120000_add_static_reaction_options.sql.
func TestChatPresenceArchiveMigrationStatements(t *testing.T) {
	run := chatPresenceArchiveStatements(t)
	if len(run) != 8 {
		t.Fatalf("%s gives %d statements, want 8:\n%s", chatPresenceArchiveMigration, len(run), strings.Join(run, "\n---\n"))
	}
	for i := 0; i < 7; i++ {
		if !strings.HasPrefix(run[i], "ALTER TABLE chat_presence") {
			t.Errorf("statement %d unexpected:\n%s", i+1, run[i])
		}
	}
	if !strings.HasPrefix(run[7], "CREATE TABLE IF NOT EXISTS chat_presence_archived ") {
		t.Errorf("statement 8 unexpected:\n%s", run[7])
	}

	joined := strings.Join(run, "\n")
	for _, constraintName := range []string{
		"chk_chat_presence_archive_threshold",
		"chk_chat_presence_archive_emoji_mode",
		"chk_chat_presence_archive_delivery",
		"pk_chat_presence_archived",
	} {
		if !strings.Contains(joined, "CONSTRAINT "+constraintName+" ") {
			t.Errorf("migration must carry constraint %s inline", constraintName)
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

	const previous = "20261129120000_add_static_reaction_options.sql"
	if chatPresenceArchiveMigration <= previous {
		t.Errorf("%s must sort after %s", chatPresenceArchiveMigration, previous)
	}
}

// TestChatPresenceArchiveMigrationDefaults proves, on PostgreSQL only, that the statements run a
// second time without error, that defaults on chat_presence match the Phase 33 specification,
// that inline CHECK constraints refuse invalid values (SQLSTATE 23514), and that
// pk_chat_presence_archived rejects duplicate (chat_id, message_id) inserts (SQLSTATE 23505).
func TestChatPresenceArchiveMigrationDefaults(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("XENOBOT_TEST_DATABASE")), "true") {
		t.Skip("set XENOBOT_TEST_DATABASE=true and DATABASE_URL to run the chat presence archive migration test on PostgreSQL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required to run the chat presence archive migration test on PostgreSQL")
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
	if !database.Migrator().HasTable("chat_presence_archived") {
		t.Fatal("table chat_presence_archived is missing: the migration chain must run before this test")
	}

	tx := database.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	defer func() { _ = tx.Rollback().Error }()

	for i, stmt := range chatPresenceArchiveStatements(t) {
		if err := tx.Exec(stmt).Error; err != nil {
			t.Fatalf("second run: statement %d failed: %v", i+1, err)
		}
	}

	chat := captchaModeChatID(t)
	if err := tx.Exec(`INSERT INTO chat_presence (chat_id) VALUES (?)`, chat).Error; err != nil {
		t.Fatalf("insert with only chat_id: %v", err)
	}
	var row struct {
		CatStickerEnabled bool
		ArchiveEnabled    bool
		ArchiveChatID     int64
		ArchiveThreadID   int64
		ArchiveThreshold  int16
		ArchiveEmojiMode  string
		ArchiveDelivery   string
	}
	if err := tx.Raw(`
		SELECT cat_sticker_enabled, archive_enabled, archive_chat_id, archive_thread_id,
		       archive_threshold, archive_emoji_mode, archive_delivery
		FROM chat_presence WHERE chat_id = ?`, chat).Scan(&row).Error; err != nil {
		t.Fatalf("read defaults: %v", err)
	}
	if row.CatStickerEnabled || row.ArchiveEnabled || row.ArchiveChatID != 0 || row.ArchiveThreadID != 0 ||
		row.ArchiveThreshold != 5 || row.ArchiveEmojiMode != "positive" || row.ArchiveDelivery != "copy" {
		t.Errorf("unexpected defaults: %+v", row)
	}

	for _, tc := range []struct{ name, stmt, constraint string }{
		{"threshold 1", `UPDATE chat_presence SET archive_threshold = 1 WHERE chat_id = ?`, "chk_chat_presence_archive_threshold"},
		{"threshold 51", `UPDATE chat_presence SET archive_threshold = 51 WHERE chat_id = ?`, "chk_chat_presence_archive_threshold"},
		{"emoji mode negative", `UPDATE chat_presence SET archive_emoji_mode = 'negative' WHERE chat_id = ?`, "chk_chat_presence_archive_emoji_mode"},
		{"emoji mode empty", `UPDATE chat_presence SET archive_emoji_mode = '' WHERE chat_id = ?`, "chk_chat_presence_archive_emoji_mode"},
		{"delivery link", `UPDATE chat_presence SET archive_delivery = 'link' WHERE chat_id = ?`, "chk_chat_presence_archive_delivery"},
		{"delivery empty", `UPDATE chat_presence SET archive_delivery = '' WHERE chat_id = ?`, "chk_chat_presence_archive_delivery"},
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

	if err := tx.Exec(`INSERT INTO chat_presence_archived (chat_id, message_id) VALUES (?, 42)`, chat).Error; err != nil {
		t.Fatalf("first insert into chat_presence_archived: %v", err)
	}
	if err := tx.Exec("SAVEPOINT dup_archive").Error; err != nil {
		t.Fatal(err)
	}
	err = tx.Exec(`INSERT INTO chat_presence_archived (chat_id, message_id) VALUES (?, 42)`, chat).Error
	if err == nil || !strings.Contains(err.Error(), "SQLSTATE 23505") || !strings.Contains(err.Error(), "pk_chat_presence_archived") {
		t.Errorf("duplicate archive insert: err %v, want 23505 naming pk_chat_presence_archived", err)
	}
	if err := tx.Exec("ROLLBACK TO SAVEPOINT dup_archive").Error; err != nil {
		t.Fatal(err)
	}
}
