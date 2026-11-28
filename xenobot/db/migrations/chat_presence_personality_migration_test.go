package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const chatPresencePersonalityMigration = "20261202120000_add_presence_personality.sql"

// TestChatPresencePersonalityMigrationStatements proves, without a database, what the Go runner hands
// to PostgreSQL for the personality migration: five idempotent ALTER TABLE chat_presence statements
// with their named CHECKs inline, every column NOT NULL with a zero-meaning default, no transaction
// control, no CONCURRENTLY, no NOT VALID, no top-level ADD CONSTRAINT, no foreign key, and a file name
// that sorts after the previous presence migration.
func TestChatPresencePersonalityMigrationStatements(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", chatPresencePersonalityMigration))
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", chatPresencePersonalityMigration, err)
	}
	runner := &MigrationRunner{}
	var run []string
	for _, stmt := range runner.splitSQLStatements(cleanSupabaseSQL(string(content))) {
		if strings.TrimSpace(stmt) == "" || isTransactionControlStatement(stmt) {
			continue
		}
		run = append(run, stripSQLComments(stmt))
	}
	if len(run) != 5 {
		t.Fatalf("%s gives %d statements, want 5:\n%s", chatPresencePersonalityMigration, len(run), strings.Join(run, "\n---\n"))
	}
	wantConstraints := []string{
		"chk_chat_presence_react_text_freq", "chk_chat_presence_react_sticker_freq", "chk_chat_presence_react_photo_freq",
		"chk_chat_presence_react_topics", "chk_chat_presence_bot_names",
	}
	for i, stmt := range run {
		if !strings.HasPrefix(stmt, "ALTER TABLE chat_presence") || !strings.Contains(stmt, "ADD COLUMN IF NOT EXISTS") {
			t.Errorf("statement %d is not an idempotent ADD COLUMN:\n%s", i+1, stmt)
		}
		if !strings.Contains(stmt, "NOT NULL DEFAULT") {
			t.Errorf("statement %d must be NOT NULL with a default:\n%s", i+1, stmt)
		}
		if !strings.Contains(stmt, "CONSTRAINT "+wantConstraints[i]+" ") {
			t.Errorf("statement %d must carry constraint %s inline:\n%s", i+1, wantConstraints[i], stmt)
		}
		upper := strings.ToUpper(stmt)
		for _, banned := range []string{"CONCURRENTLY", "NOT VALID", "BEGIN", "COMMIT", "ROLLBACK", "ADD CONSTRAINT", "REFERENCES"} {
			if strings.Contains(upper, banned) {
				t.Errorf("statement %d contains %s", i+1, banned)
			}
		}
	}
	if chatPresencePersonalityMigration <= "20261201120000_add_presence_toys.sql" {
		t.Errorf("%s must sort after the toys migration", chatPresencePersonalityMigration)
	}
}
