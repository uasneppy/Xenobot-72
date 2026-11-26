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

const chatConversationProviderMigration = "20261126120000_allow_featherless_chat_provider.sql"

func chatConversationProviderStatements(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", chatConversationProviderMigration))
	if err != nil {
		t.Fatalf("resolve the migration path: %v", err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", chatConversationProviderMigration, err)
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

// TestChatConversationProviderMigrationStatements proves, without a database, what the Go runner
// hands to PostgreSQL: drop the named check, drop any legacy provider checks, then add the widened
// check with 'featherless' as NOT VALID.
func TestChatConversationProviderMigrationStatements(t *testing.T) {
	run := chatConversationProviderStatements(t)
	if len(run) != 3 {
		t.Fatalf("%s gives %d statements, want 3:\n%s",
			chatConversationProviderMigration, len(run), strings.Join(run, "\n---\n"))
	}
	drop, legacy, add := run[0], run[1], run[2]
	if !strings.Contains(drop, "DROP CONSTRAINT IF EXISTS chk_chat_conversations_provider") {
		t.Errorf("statement 1 must drop chk_chat_conversations_provider, got:\n%s", drop)
	}
	if !strings.Contains(legacy, "pg_constraint") || !strings.Contains(legacy, "provider") || !strings.Contains(legacy, "DROP CONSTRAINT") {
		t.Errorf("statement 2 must drop any other provider check, got:\n%s", legacy)
	}
	if strings.Contains(legacy, "duplicate_object") {
		t.Errorf("statement 2 must not be wrapped by duplicate_object, got:\n%s", legacy)
	}
	if !strings.Contains(add, "'featherless'") || !strings.Contains(add, "NOT VALID") {
		t.Errorf("statement 3 must add the check with featherless as NOT VALID, got:\n%s", add)
	}
	if !strings.Contains(add, "duplicate_object") {
		t.Errorf("statement 3 should arrive wrapped by the runner's duplicate_object rewrite, got:\n%s", add)
	}
	const previous = "20261125120000_add_chat_conversation_log.sql"
	if chatConversationProviderMigration <= previous {
		t.Errorf("%s must sort after %s", chatConversationProviderMigration, previous)
	}
}

// TestChatConversationProviderMigrationConstraint proves, on PostgreSQL only, that the migration
// runs idempotently and chat_conversations.provider accepts anthropic, google and featherless and
// refuses anything else with SQLSTATE 23514 naming chk_chat_conversations_provider.
func TestChatConversationProviderMigrationConstraint(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("XENOBOT_TEST_DATABASE")), "true") {
		t.Skip("set XENOBOT_TEST_DATABASE=true and DATABASE_URL to run the chat provider migration test on PostgreSQL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required to run the chat provider migration test on PostgreSQL")
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

	run := chatConversationProviderStatements(t)
	for pass := 1; pass <= 2; pass++ {
		for i, stmt := range run {
			if err := tx.Exec(stmt).Error; err != nil {
				t.Fatalf("pass %d: statement %d failed: %v", pass, i+1, err)
			}
		}
	}

	var names []string
	if err := tx.Raw(
		`SELECT conname FROM pg_constraint
		 WHERE conrelid = 'chat_conversations'::regclass AND contype = 'c'
		   AND pg_get_constraintdef(oid) LIKE '%provider%'`,
	).Scan(&names).Error; err != nil {
		t.Fatalf("list provider checks: %v", err)
	}
	if len(names) != 1 || names[0] != "chk_chat_conversations_provider" {
		t.Errorf("checks mentioning provider = %v, want exactly [chk_chat_conversations_provider]", names)
	}

	for _, provider := range []string{"anthropic", "google", "featherless"} {
		chat := captchaModeChatID(t)
		if err := tx.Exec(`INSERT INTO chat_conversations (chat_id, provider) VALUES (?, ?)`, chat, provider).Error; err != nil {
			t.Errorf("provider %q must be accepted, got: %v", provider, err)
		}
	}

	if err := tx.Exec("SAVEPOINT refused_provider").Error; err != nil {
		t.Fatal(err)
	}
	err = tx.Exec(`INSERT INTO chat_conversations (chat_id, provider) VALUES (?, 'openai')`, captchaModeChatID(t)).Error
	if err == nil || !strings.Contains(err.Error(), "SQLSTATE 23514") || !strings.Contains(err.Error(), "chk_chat_conversations_provider") {
		t.Errorf("provider openai: err %v, want 23514 naming chk_chat_conversations_provider", err)
	}
	if err := tx.Exec("ROLLBACK TO SAVEPOINT refused_provider").Error; err != nil {
		t.Fatal(err)
	}
}
