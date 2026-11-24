//go:build testtools

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

const manualCommandTuningMigration = "20261122150000_add_manual_command_tuning.sql"

func manualCommandTuningStatements(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", manualCommandTuningMigration))
	if err != nil {
		t.Fatalf("resolve the migration path: %v", err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", manualCommandTuningMigration, err)
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

// TestManualCommandTuningMigrationStatements proves, without a database, that migration
// 20261122150000 contains idempotent DDL with inline named constraints and no foreign keys.
func TestManualCommandTuningMigrationStatements(t *testing.T) {
	run := manualCommandTuningStatements(t)
	if len(run) != 3 {
		t.Fatalf("%s gives %d statements, want 3:\n%s", manualCommandTuningMigration, len(run), strings.Join(run, "\n---\n"))
	}
	tables := []struct {
		prefix      string
		constraints []string
	}{
		{"CREATE TABLE IF NOT EXISTS chat_manual_defaults ", []string{
			"pk_chat_manual_defaults",
			"chk_chat_manual_defaults_ban",
			"chk_chat_manual_defaults_mute",
			"chk_chat_manual_defaults_appeal",
		}},
		{"CREATE TABLE IF NOT EXISTS chat_reason_shortcuts ", []string{
			"uk_chat_reason_shortcuts_word",
			"chk_chat_reason_shortcuts_word",
			"chk_chat_reason_shortcuts_reason",
		}},
		{"CREATE INDEX IF NOT EXISTS idx_chat_reason_shortcuts_chat ", nil},
	}
	for i, spec := range tables {
		if !strings.HasPrefix(run[i], spec.prefix) {
			t.Errorf("statement %d must start with %q, got:\n%s", i+1, spec.prefix, run[i])
		}
		for _, name := range spec.constraints {
			if !strings.Contains(run[i], "CONSTRAINT "+name+" ") {
				t.Errorf("statement %d must carry constraint %s inline", i+1, name)
			}
		}
		upper := strings.ToUpper(run[i])
		for _, banned := range []string{"CONCURRENTLY", "NOT VALID", "BEGIN", "COMMIT", "ROLLBACK", "ADD CONSTRAINT", "REFERENCES"} {
			if strings.Contains(upper, banned) {
				t.Errorf("statement %d contains %s", i+1, banned)
			}
		}
	}
	if manualCommandTuningMigration <= escalationMigration {
		t.Errorf("%s must sort after %s", manualCommandTuningMigration, escalationMigration)
	}
}

// TestManualCommandTuningMigrationDefaults verifies on PostgreSQL that 20261122150000 runs
// idempotently twice, sets defaults accurately, and enforces all CHECK and UNIQUE constraints.
func TestManualCommandTuningMigrationDefaults(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("FUKU_TEST_DATABASE")), "true") {
		t.Skip("set FUKU_TEST_DATABASE=true and DATABASE_URL to run the manual command tuning migration test on PostgreSQL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required to run the manual command tuning migration test on PostgreSQL")
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
	for _, table := range []string{"chat_manual_defaults", "chat_reason_shortcuts"} {
		if !database.Migrator().HasTable(table) {
			t.Fatalf("table %s is missing: the migration chain must run before this test", table)
		}
	}

	tx := database.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	defer func() { _ = tx.Rollback().Error }()

	for pass := 1; pass <= 2; pass++ {
		for i, stmt := range manualCommandTuningStatements(t) {
			if err := tx.Exec(stmt).Error; err != nil {
				t.Fatalf("pass %d: statement %d failed: %v", pass, i+1, err)
			}
		}
	}

	chat := captchaModeChatID(t)
	if err := tx.Exec(`INSERT INTO chat_manual_defaults (chat_id) VALUES (?)`, chat).Error; err != nil {
		t.Fatalf("insert default row: %v", err)
	}
	var defs struct {
		DefaultBanSecs  int
		DefaultMuteSecs int
		ReasonRequired  bool
		AppealURL       string
		UpdatedBy       int64
	}
	if err := tx.Raw(`SELECT default_ban_secs, default_mute_secs, reason_required, appeal_url, updated_by
		FROM chat_manual_defaults WHERE chat_id = ?`, chat).Scan(&defs).Error; err != nil {
		t.Fatalf("read defaults: %v", err)
	}
	if defs.DefaultBanSecs != 0 || defs.DefaultMuteSecs != 0 || defs.ReasonRequired || defs.AppealURL != "" || defs.UpdatedBy != 0 {
		t.Errorf("defaults = %+v, want all zero", defs)
	}

	if err := tx.Exec(`INSERT INTO chat_reason_shortcuts (chat_id, word, reason) VALUES (?, 'spam', 'No spam')`, chat).Error; err != nil {
		t.Fatalf("insert shortcut row: %v", err)
	}

	refused := func(name, stmt, sqlstate, constraint string, args ...any) {
		t.Helper()
		if err := tx.Exec("SAVEPOINT refused").Error; err != nil {
			t.Fatal(err)
		}
		err := tx.Exec(stmt, args...).Error
		if err == nil || !strings.Contains(err.Error(), "SQLSTATE "+sqlstate) || !strings.Contains(err.Error(), constraint) {
			t.Errorf("%s: err %v, want %s naming %s", name, err, sqlstate, constraint)
		}
		if err := tx.Exec("ROLLBACK TO SAVEPOINT refused").Error; err != nil {
			t.Fatal(err)
		}
	}

	refused("ban secs 59", `UPDATE chat_manual_defaults SET default_ban_secs = 59 WHERE chat_id = ?`, "23514", "chk_chat_manual_defaults_ban", chat)
	refused("ban secs 31622401", `UPDATE chat_manual_defaults SET default_ban_secs = 31622401 WHERE chat_id = ?`, "23514", "chk_chat_manual_defaults_ban", chat)
	refused("mute secs 59", `UPDATE chat_manual_defaults SET default_mute_secs = 59 WHERE chat_id = ?`, "23514", "chk_chat_manual_defaults_mute", chat)
	refused("mute secs 31622401", `UPDATE chat_manual_defaults SET default_mute_secs = 31622401 WHERE chat_id = ?`, "23514", "chk_chat_manual_defaults_mute", chat)
	refused("appeal_url > 2048", `UPDATE chat_manual_defaults SET appeal_url = ? WHERE chat_id = ?`, "23514", "chk_chat_manual_defaults_appeal", strings.Repeat("a", 2049), chat)
	refused("empty word", `INSERT INTO chat_reason_shortcuts (chat_id, word, reason) VALUES (?, '', 'ok')`, "23514", "chk_chat_reason_shortcuts_word", chat)
	refused("empty reason", `INSERT INTO chat_reason_shortcuts (chat_id, word, reason) VALUES (?, 'flood', '')`, "23514", "chk_chat_reason_shortcuts_reason", chat)
	refused("reason > 500", `INSERT INTO chat_reason_shortcuts (chat_id, word, reason) VALUES (?, 'flood', ?)`, "23514", "chk_chat_reason_shortcuts_reason", chat, strings.Repeat("r", 501))
	refused("duplicate chat_id", `INSERT INTO chat_manual_defaults (chat_id) VALUES (?)`, "23505", "pk_chat_manual_defaults", chat)
	refused("duplicate (chat_id, word)", `INSERT INTO chat_reason_shortcuts (chat_id, word, reason) VALUES (?, 'spam', 'Other')`, "23505", "uk_chat_reason_shortcuts_word", chat)

	// Boundary values are accepted.
	if err := tx.Exec(`UPDATE chat_manual_defaults SET default_ban_secs = 60, default_mute_secs = 31622400, appeal_url = ? WHERE chat_id = ?`, strings.Repeat("a", 2048), chat).Error; err != nil {
		t.Errorf("valid boundary defaults refused: %v", err)
	}
	if err := tx.Exec(`INSERT INTO chat_reason_shortcuts (chat_id, word, reason) VALUES (?, 'max', ?)`, chat, strings.Repeat("r", 500)).Error; err != nil {
		t.Errorf("valid boundary shortcut refused: %v", err)
	}
}
