package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const memberMemoriesMigration = "20261203120000_add_member_memories.sql"

// TestMemberMemoriesMigrationStatements proves, without a database, what the Go runner hands to
// PostgreSQL for the memories migration: one idempotent ADD COLUMN with a false default and one
// CREATE TABLE IF NOT EXISTS with its named primary key and notes CHECK inline, no transaction
// control, no CONCURRENTLY, no NOT VALID, no top-level ADD CONSTRAINT, no foreign key, and a file name
// that sorts after the personality migration.
func TestMemberMemoriesMigrationStatements(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", memberMemoriesMigration))
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", memberMemoriesMigration, err)
	}
	runner := &MigrationRunner{}
	var run []string
	for _, stmt := range runner.splitSQLStatements(cleanSupabaseSQL(string(content))) {
		if strings.TrimSpace(stmt) == "" || isTransactionControlStatement(stmt) {
			continue
		}
		run = append(run, stripSQLComments(stmt))
	}
	if len(run) != 2 {
		t.Fatalf("%s gives %d statements, want 2:\n%s", memberMemoriesMigration, len(run), strings.Join(run, "\n---\n"))
	}
	if !strings.HasPrefix(run[0], "ALTER TABLE chat_conversations") || !strings.Contains(run[0], "ADD COLUMN IF NOT EXISTS memories_enabled BOOLEAN NOT NULL DEFAULT FALSE") {
		t.Errorf("the first statement is not the idempotent switch column:\n%s", run[0])
	}
	if !strings.HasPrefix(run[1], "CREATE TABLE IF NOT EXISTS chat_member_memories") {
		t.Errorf("the second statement is not an idempotent CREATE TABLE:\n%s", run[1])
	}
	for _, want := range []string{"CONSTRAINT pk_chat_member_memories PRIMARY KEY (chat_id, user_id)", "CONSTRAINT chk_chat_member_memories_notes CHECK (char_length(notes) <= 2500)"} {
		if !strings.Contains(run[1], want) {
			t.Errorf("the table lacks %q:\n%s", want, run[1])
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
	if memberMemoriesMigration <= chatPresencePersonalityMigration {
		t.Errorf("%s must sort after the personality migration", memberMemoriesMigration)
	}
}
