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

const escalationMigration = "20261122140000_add_escalation_ladder.sql"

// escalationStatements reads the escalation ladder migration and returns the statements the runner
// would execute, dropping empty and transaction-control statements as applyMigration does.
func escalationStatements(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", escalationMigration))
	if err != nil {
		t.Fatalf("resolve the migration path: %v", err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", escalationMigration, err)
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

// TestEscalationMigrationStatements proves, without a database, what the Go runner hands to
// PostgreSQL for the escalation ladder migration: two idempotent CREATE TABLE statements, the
// ladder's with its primary key, one check per step and the forget check inline, the counts' with
// its primary key and count check inline; no transaction control, no CONCURRENTLY, no NOT VALID, no
// top-level ADD CONSTRAINT and no foreign key; and a file name that sorts after the warn expiry
// migration, the latest before it.
func TestEscalationMigrationStatements(t *testing.T) {
	run := escalationStatements(t)
	if len(run) != 2 {
		t.Fatalf("%s gives %d statements, want 2:\n%s", escalationMigration, len(run), strings.Join(run, "\n---\n"))
	}
	tables := []struct {
		prefix      string
		constraints []string
	}{
		{"CREATE TABLE IF NOT EXISTS chat_escalation ", []string{
			"pk_chat_escalation", "chk_chat_escalation_step1", "chk_chat_escalation_step2", "chk_chat_escalation_step3",
			"chk_chat_escalation_step4", "chk_chat_escalation_step5", "chk_chat_escalation_forget",
		}},
		{"CREATE TABLE IF NOT EXISTS chat_offences ", []string{"pk_chat_offences", "chk_chat_offences_count"}},
	}
	for i, table := range tables {
		if !strings.HasPrefix(run[i], table.prefix) {
			t.Errorf("statement %d must start with %q, got:\n%s", i+1, table.prefix, run[i])
		}
		for _, name := range table.constraints {
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
	if escalationMigration <= warnExpiryMigration {
		t.Errorf("%s must sort after %s", escalationMigration, warnExpiryMigration)
	}
}

// TestEscalationMigrationDefaults proves, on PostgreSQL only, that both tables exist after the
// chain, that the statements run a second time without error, that a ladder row with only chat_id
// reads off, no steps and 30 days and an offence row with only its key reads 0 and 0, that the
// named checks refuse a step action outside the set, step seconds over 366 days, a forget period of
// 0 or 366 and a negative count with SQLSTATE 23514, and that a second row of a key fails with 23505.
func TestEscalationMigrationDefaults(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("FUKU_TEST_DATABASE")), "true") {
		t.Skip("set FUKU_TEST_DATABASE=true and DATABASE_URL to run the escalation migration test on PostgreSQL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required to run the escalation migration test on PostgreSQL")
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
	for _, table := range []string{"chat_escalation", "chat_offences"} {
		if !database.Migrator().HasTable(table) {
			t.Fatalf("table %s is missing: the migration chain must run before this test", table)
		}
	}

	tx := database.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	// Never committed: the rows below vanish, and so does the second run of the migration.
	defer func() { _ = tx.Rollback().Error }()

	for i, stmt := range escalationStatements(t) {
		if err := tx.Exec(stmt).Error; err != nil {
			t.Fatalf("second run: statement %d failed: %v", i+1, err)
		}
	}
	chat := captchaModeChatID(t)
	if err := tx.Exec(`INSERT INTO chat_escalation (chat_id) VALUES (?)`, chat).Error; err != nil {
		t.Fatalf("insert a ladder row with only chat_id: %v", err)
	}
	if err := tx.Exec(`INSERT INTO chat_offences (chat_id, user_id) VALUES (?, 7)`, chat).Error; err != nil {
		t.Fatalf("insert an offence row with only its key: %v", err)
	}
	var ladder struct {
		Enabled    bool
		Steps      string
		Secs       int64
		ForgetDays int
		UpdatedBy  int64
	}
	if err := tx.Raw(`SELECT enabled,
		step1_action || step2_action || step3_action || step4_action || step5_action AS steps,
		step1_secs + step2_secs + step3_secs + step4_secs + step5_secs AS secs,
		forget_days, updated_by FROM chat_escalation WHERE chat_id = ?`, chat).Scan(&ladder).Error; err != nil {
		t.Fatalf("read the ladder defaults: %v", err)
	}
	if ladder.Enabled || ladder.Steps != "" || ladder.Secs != 0 || ladder.ForgetDays != 30 || ladder.UpdatedBy != 0 {
		t.Errorf("ladder defaults = %+v, want off, no steps, 30 days", ladder)
	}
	var offence struct {
		Offences        int
		LastOffenceUnix int64
	}
	if err := tx.Raw(`SELECT offences, last_offence_unix FROM chat_offences WHERE chat_id = ? AND user_id = 7`, chat).Scan(&offence).Error; err != nil {
		t.Fatalf("read the offence defaults: %v", err)
	}
	if offence.Offences != 0 || offence.LastOffenceUnix != 0 {
		t.Errorf("offence defaults = %+v, want 0 and 0", offence)
	}

	refused := func(name, stmt, sqlstate, constraint string) {
		t.Helper()
		// A refused statement aborts the transaction in PostgreSQL, so it runs behind a savepoint.
		if err := tx.Exec("SAVEPOINT refused").Error; err != nil {
			t.Fatal(err)
		}
		err := tx.Exec(stmt, chat).Error
		if err == nil || !strings.Contains(err.Error(), "SQLSTATE "+sqlstate) || !strings.Contains(err.Error(), constraint) {
			t.Errorf("%s: err %v, want %s naming %s", name, err, sqlstate, constraint)
		}
		if err := tx.Exec("ROLLBACK TO SAVEPOINT refused").Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ name, stmt, constraint string }{
		{"step1_action warn", `UPDATE chat_escalation SET step1_action = 'warn' WHERE chat_id = ?`, "chk_chat_escalation_step1"},
		{"step2_secs 31622401", `UPDATE chat_escalation SET step2_secs = 31622401 WHERE chat_id = ?`, "chk_chat_escalation_step2"},
		{"step5_secs -1", `UPDATE chat_escalation SET step5_secs = -1 WHERE chat_id = ?`, "chk_chat_escalation_step5"},
		{"forget_days 0", `UPDATE chat_escalation SET forget_days = 0 WHERE chat_id = ?`, "chk_chat_escalation_forget"},
		{"forget_days 366", `UPDATE chat_escalation SET forget_days = 366 WHERE chat_id = ?`, "chk_chat_escalation_forget"},
		{"offences -1", `UPDATE chat_offences SET offences = -1 WHERE chat_id = ?`, "chk_chat_offences_count"},
	} {
		refused(tc.name, tc.stmt, "23514", tc.constraint)
	}
	refused("a second ladder row", `INSERT INTO chat_escalation (chat_id) VALUES (?)`, "23505", "pk_chat_escalation")
	refused("a second offence row", `INSERT INTO chat_offences (chat_id, user_id) VALUES (?, 7)`, "23505", "pk_chat_offences")

	// The bounds themselves are accepted.
	for _, stmt := range []string{
		`UPDATE chat_escalation SET step3_action = 'tban', step3_secs = 31622400, forget_days = 365 WHERE chat_id = ?`,
		`UPDATE chat_escalation SET step4_action = 'mute', forget_days = 1 WHERE chat_id = ?`,
	} {
		if err := tx.Exec(stmt, chat).Error; err != nil {
			t.Errorf("a value inside the checks was refused: %v\n%s", err, stmt)
		}
	}
}
