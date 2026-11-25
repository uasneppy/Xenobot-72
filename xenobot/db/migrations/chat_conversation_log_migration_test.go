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

const chatConversationLogMigration = "20261125120000_add_chat_conversation_log.sql"

// chatConversationLogStatements reads the message log migration and returns the statements the runner
// would execute.
func chatConversationLogStatements(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", chatConversationLogMigration))
	if err != nil {
		t.Fatalf("resolve the migration path: %v", err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", chatConversationLogMigration, err)
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

// TestChatConversationLogMigrationStatements proves, without a database, that the migration is two
// idempotent column additions, the second with its named check inline, and nothing the appliers rewrite.
func TestChatConversationLogMigrationStatements(t *testing.T) {
	run := chatConversationLogStatements(t)
	if len(run) != 2 {
		t.Fatalf("%s gives %d statements, want 2:\n%s", chatConversationLogMigration, len(run), strings.Join(run, "\n---\n"))
	}
	if !strings.Contains(run[0], "ALTER TABLE chat_conversations ADD COLUMN IF NOT EXISTS log_enabled BOOLEAN NOT NULL DEFAULT FALSE") {
		t.Errorf("statement 1 must add log_enabled idempotently, got:\n%s", run[0])
	}
	if !strings.Contains(run[1], "ALTER TABLE chat_conversations ADD COLUMN IF NOT EXISTS judge_to SMALLINT NOT NULL DEFAULT 0 ") ||
		!strings.Contains(run[1], "CONSTRAINT chk_chat_conversations_judge_to CHECK (judge_to BETWEEN 0 AND 2)") {
		t.Errorf("statement 2 must add judge_to with its named check inline, got:\n%s", run[1])
	}
	for i, stmt := range run {
		upper := strings.ToUpper(stmt)
		for _, banned := range []string{"CONCURRENTLY", "NOT VALID", "BEGIN", "COMMIT", "ROLLBACK", "ADD CONSTRAINT"} {
			if strings.Contains(upper, banned) {
				t.Errorf("statement %d contains %s:\n%s", i+1, banned, stmt)
			}
		}
	}
	const previous = "20261124120000_rename_staff_role_lock_prefix.sql"
	if chatConversationLogMigration <= previous {
		t.Errorf("%s must sort after %s", chatConversationLogMigration, previous)
	}
}

// TestChatConversationLogMigrationDefaults proves, on PostgreSQL only, that the statements run a second
// time without error, that a row with only chat_id reads the log off and the verdict going to the group,
// and that judge_to 3 is refused with SQLSTATE 23514 naming the constraint.
func TestChatConversationLogMigrationDefaults(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("XENOBOT_TEST_DATABASE")), "true") {
		t.Skip("set XENOBOT_TEST_DATABASE=true and DATABASE_URL to run the message log migration test on PostgreSQL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required to run the message log migration test on PostgreSQL")
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
	if !database.Migrator().HasTable("chat_conversations") {
		t.Fatal("table chat_conversations is missing: the migration chain must run before this test")
	}

	tx := database.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	defer func() { _ = tx.Rollback().Error }()

	for i, stmt := range chatConversationLogStatements(t) {
		if err := tx.Exec(stmt).Error; err != nil {
			t.Fatalf("second run: statement %d failed: %v", i+1, err)
		}
	}
	chat := captchaModeChatID(t)
	if err := tx.Exec(`INSERT INTO chat_conversations (chat_id) VALUES (?)`, chat).Error; err != nil {
		t.Fatalf("insert with only chat_id: %v", err)
	}
	var row struct {
		LogEnabled bool
		JudgeTo    int
	}
	if err := tx.Raw(`SELECT log_enabled, judge_to FROM chat_conversations WHERE chat_id = ?`, chat).Scan(&row).Error; err != nil {
		t.Fatalf("read the defaults: %v", err)
	}
	if row.LogEnabled || row.JudgeTo != 0 {
		t.Errorf("defaults = %+v, want off and 0", row)
	}
	if err := tx.Exec("SAVEPOINT refused").Error; err != nil {
		t.Fatal(err)
	}
	err = tx.Exec(`UPDATE chat_conversations SET judge_to = 3 WHERE chat_id = ?`, chat).Error
	if err == nil || !strings.Contains(err.Error(), "SQLSTATE 23514") || !strings.Contains(err.Error(), "chk_chat_conversations_judge_to") {
		t.Errorf("judge_to 3: err %v, want 23514 naming chk_chat_conversations_judge_to", err)
	}
	if err := tx.Exec("ROLLBACK TO SAVEPOINT refused").Error; err != nil {
		t.Fatal(err)
	}
}
