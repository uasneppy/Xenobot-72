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

const staticReactionOptionsMigration = "20261129120000_add_static_reaction_options.sql"

// staticReactionOptionsStatements reads the static reaction options migration and returns the
// statements the runner would execute, dropping empty and transaction-control statements as
// applyMigration does.
func staticReactionOptionsStatements(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", staticReactionOptionsMigration))
	if err != nil {
		t.Fatalf("resolve the migration path: %v", err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", staticReactionOptionsMigration, err)
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

// TestStaticReactionOptionsMigrationStatements proves, without a database, what the Go runner
// hands to PostgreSQL for the static reaction options migration: four idempotent ALTER TABLE
// reactions statements with named CHECK constraints inline, no transaction control, no
// CONCURRENTLY, no NOT VALID, no top-level ADD CONSTRAINT and no foreign key, and a file name
// that sorts strictly after 20261128120000_add_chat_presence_ai_react_mode.sql.
func TestStaticReactionOptionsMigrationStatements(t *testing.T) {
	run := staticReactionOptionsStatements(t)
	if len(run) != 4 {
		t.Fatalf("%s gives %d statements, want 4:\n%s", staticReactionOptionsMigration, len(run), strings.Join(run, "\n---\n"))
	}
	for i, stmt := range run {
		if !strings.HasPrefix(stmt, "ALTER TABLE reactions") {
			t.Errorf("statement %d: unexpected statement:\n%s", i+1, stmt)
		}
		upper := strings.ToUpper(stmt)
		for _, banned := range []string{"CONCURRENTLY", "NOT VALID", "BEGIN", "COMMIT", "ROLLBACK", "ADD CONSTRAINT", "REFERENCES"} {
			if strings.Contains(upper, banned) {
				t.Errorf("statement %d contains %s", i+1, banned)
			}
		}
	}
	joined := strings.Join(run, "\n")
	for _, constraintName := range []string{"chk_reactions_chance_pct", "chk_reactions_cooldown_secs"} {
		if !strings.Contains(joined, "CONSTRAINT "+constraintName+" ") {
			t.Errorf("migration must carry constraint %s inline", constraintName)
		}
	}
	if staticReactionOptionsMigration <= chatPresenceAIReactMigration {
		t.Errorf("%s must sort after %s", staticReactionOptionsMigration, chatPresenceAIReactMigration)
	}
}

// TestStaticReactionOptionsMigrationDefaults proves, on PostgreSQL only, that the statements run
// idempotently, that a row with only (chat_id, keyword, emoji) defaults word_boundary=false,
// match_caption=false, chance_pct=100, and cooldown_secs=0, and that the named CHECK constraints
// refuse out-of-range values with SQLSTATE 23514.
func TestStaticReactionOptionsMigrationDefaults(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("XENOBOT_TEST_DATABASE")), "true") {
		t.Skip("set XENOBOT_TEST_DATABASE=true and DATABASE_URL to run the static reaction options migration test on PostgreSQL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required to run the static reaction options migration test on PostgreSQL")
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
	if !database.Migrator().HasTable("reactions") {
		t.Fatal("table reactions is missing: the migration chain must run before this test")
	}

	tx := database.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	defer func() { _ = tx.Rollback().Error }()

	stmts := staticReactionOptionsStatements(t)
	for pass := 0; pass < 2; pass++ {
		for i, stmt := range stmts {
			if err := tx.Exec(stmt).Error; err != nil {
				t.Fatalf("pass %d: statement %d failed: %v", pass+1, i+1, err)
			}
		}
	}
	chat := captchaModeChatID(t)
	now := time.Now().UTC()
	if err := tx.Exec(
		`INSERT INTO chats (chat_id, chat_name, language, is_inactive, created_at, updated_at)
		 VALUES (?, ?, ?, FALSE, ?, ?)`,
		chat, "static reaction options test", "en", now, now,
	).Error; err != nil {
		t.Fatalf("insert chat %d: %v", chat, err)
	}
	if err := tx.Exec(`INSERT INTO reactions (chat_id, keyword, emoji) VALUES (?, 'hello', '👋')`, chat).Error; err != nil {
		t.Fatalf("insert minimal reaction row: %v", err)
	}
	var row struct {
		WordBoundary bool
		MatchCaption bool
		ChancePct    int16
		CooldownSecs int
	}
	if err := tx.Raw(`SELECT word_boundary, match_caption, chance_pct, cooldown_secs FROM reactions WHERE chat_id = ? AND keyword = 'hello'`, chat).Scan(&row).Error; err != nil {
		t.Fatalf("read default reaction options: %v", err)
	}
	if row.WordBoundary || row.MatchCaption || row.ChancePct != 100 || row.CooldownSecs != 0 {
		t.Errorf("defaults = %+v, want word_boundary=false match_caption=false chance_pct=100 cooldown_secs=0", row)
	}
	for _, tc := range []struct{ name, stmt, constraint string }{
		{"chance_pct 0", `UPDATE reactions SET chance_pct = 0 WHERE chat_id = ? AND keyword = 'hello'`, "chk_reactions_chance_pct"},
		{"chance_pct 101", `UPDATE reactions SET chance_pct = 101 WHERE chat_id = ? AND keyword = 'hello'`, "chk_reactions_chance_pct"},
		{"cooldown_secs -1", `UPDATE reactions SET cooldown_secs = -1 WHERE chat_id = ? AND keyword = 'hello'`, "chk_reactions_cooldown_secs"},
		{"cooldown_secs 86401", `UPDATE reactions SET cooldown_secs = 86401 WHERE chat_id = ? AND keyword = 'hello'`, "chk_reactions_cooldown_secs"},
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
