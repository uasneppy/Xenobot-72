//go:build testtools

package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const advancedThresholdsMigration = "20261123120000_add_advanced_thresholds.sql"

func advancedThresholdsStatements(t *testing.T) []string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "migrations", advancedThresholdsMigration))
	if err != nil {
		t.Fatalf("resolve the migration path: %v", err)
	}
	content, err := os.ReadFile(path) // #nosec G304 - fixed repository path
	if err != nil {
		t.Fatalf("read %s: %v", advancedThresholdsMigration, err)
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

// TestAdvancedThresholdsMigrationStatements proves, without a database, that migration
// 20261123120000 contains idempotent DDL with inline named constraints and no foreign keys.
func TestAdvancedThresholdsMigrationStatements(t *testing.T) {
	run := advancedThresholdsStatements(t)
	if len(run) != 1 {
		t.Fatalf("%s gives %d statements, want 1:\n%s", advancedThresholdsMigration, len(run), strings.Join(run, "\n---\n"))
	}
	stmt := run[0]
	if !strings.HasPrefix(stmt, "CREATE TABLE IF NOT EXISTS chat_advanced_thresholds ") {
		t.Errorf("statement must start with CREATE TABLE IF NOT EXISTS chat_advanced_thresholds, got:\n%s", stmt)
	}
	constraints := []string{
		"pk_chat_advanced_thresholds",
		"chk_chat_adv_join_window",
		"chk_chat_adv_flood_window",
		"chk_chat_adv_ai_window",
		"chk_chat_adv_flood_new_pct",
		"chk_chat_adv_flood_min_accts",
		"chk_chat_adv_recent_hours",
		"chk_chat_adv_raid_ai_bar",
		"chk_chat_adv_mreq_ai_bar",
		"chk_chat_adv_mreq_cap",
		"chk_chat_adv_mreq_pause",
		"chk_chat_adv_lockdown_rem",
		"chk_chat_adv_captcha_timeout",
	}
	for _, name := range constraints {
		if !strings.Contains(stmt, "CONSTRAINT "+name+" ") {
			t.Errorf("statement must carry constraint %s inline", name)
		}
	}
	upper := strings.ToUpper(stmt)
	for _, banned := range []string{"CONCURRENTLY", "NOT VALID", "BEGIN", "COMMIT", "ROLLBACK", "ADD CONSTRAINT", "REFERENCES"} {
		if strings.Contains(upper, banned) {
			t.Errorf("statement contains %s", banned)
		}
	}
	if advancedThresholdsMigration <= manualCommandTuningMigration {
		t.Errorf("%s must sort after %s", advancedThresholdsMigration, manualCommandTuningMigration)
	}
}

// TestAdvancedThresholdsMigrationDefaults verifies on PostgreSQL that 20261123120000 runs
// idempotently twice, sets defaults to 0, and enforces all 12 CHECK constraints plus the PK.
func TestAdvancedThresholdsMigrationDefaults(t *testing.T) {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("FUKU_TEST_DATABASE")), "true") {
		t.Skip("set FUKU_TEST_DATABASE=true and DATABASE_URL to run the advanced thresholds migration test on PostgreSQL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is required to run the advanced thresholds migration test on PostgreSQL")
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
	if !database.Migrator().HasTable("chat_advanced_thresholds") {
		t.Fatalf("table chat_advanced_thresholds is missing: the migration chain must run before this test")
	}

	tx := database.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	defer func() { _ = tx.Rollback().Error }()

	for pass := 1; pass <= 2; pass++ {
		for i, stmt := range advancedThresholdsStatements(t) {
			if err := tx.Exec(stmt).Error; err != nil {
				t.Fatalf("pass %d: statement %d failed: %v", pass, i+1, err)
			}
		}
	}

	chat := captchaModeChatID(t)
	if err := tx.Exec(`INSERT INTO chat_advanced_thresholds (chat_id) VALUES (?)`, chat).Error; err != nil {
		t.Fatalf("insert default row: %v", err)
	}
	var row struct {
		RaidJoinWindowSecs   int
		RaidFloodWindowSecs  int
		RaidAIWindowSecs     int
		RaidFloodNewPercent  int
		RaidFloodMinAccounts int
		RaidRecentHours      int
		RaidAIImageBarPct    int
		MemberReqAIBarPct    int
		MemberReqCapPerMin   int
		MemberReqPauseMins   int
		LockdownReminderMins int
		CaptchaTimeoutMins   int
		UpdatedBy            int64
	}
	if err := tx.Raw(`SELECT raid_join_window_secs, raid_flood_window_secs, raid_ai_window_secs,
		raid_flood_new_percent, raid_flood_min_accounts, raid_recent_hours,
		raid_ai_image_bar_pct, member_req_ai_bar_pct, member_req_cap_per_min,
		member_req_pause_mins, lockdown_reminder_mins, captcha_timeout_mins, updated_by
		FROM chat_advanced_thresholds WHERE chat_id = ?`, chat).Scan(&row).Error; err != nil {
		t.Fatalf("read defaults: %v", err)
	}
	if row.RaidJoinWindowSecs != 0 || row.RaidFloodWindowSecs != 0 || row.RaidAIWindowSecs != 0 ||
		row.RaidFloodNewPercent != 0 || row.RaidFloodMinAccounts != 0 || row.RaidRecentHours != 0 ||
		row.RaidAIImageBarPct != 0 || row.MemberReqAIBarPct != 0 || row.MemberReqCapPerMin != 0 ||
		row.MemberReqPauseMins != 0 || row.LockdownReminderMins != 0 || row.CaptchaTimeoutMins != 0 ||
		row.UpdatedBy != 0 {
		t.Errorf("defaults = %+v, want all zero", row)
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

	refused("duplicate chat_id", `INSERT INTO chat_advanced_thresholds (chat_id) VALUES (?)`, "23505", "pk_chat_advanced_thresholds", chat)

	checks := []struct {
		col        string
		constraint string
		low        int
		min        int
		max        int
		high       int
	}{
		{"raid_join_window_secs", "chk_chat_adv_join_window", 14, 15, 600, 601},
		{"raid_flood_window_secs", "chk_chat_adv_flood_window", 9, 10, 300, 301},
		{"raid_ai_window_secs", "chk_chat_adv_ai_window", 29, 30, 900, 901},
		{"raid_flood_new_percent", "chk_chat_adv_flood_new_pct", 19, 20, 100, 101},
		{"raid_flood_min_accounts", "chk_chat_adv_flood_min_accts", 1, 2, 20, 21},
		{"raid_recent_hours", "chk_chat_adv_recent_hours", -1, 1, 168, 169},
		{"raid_ai_image_bar_pct", "chk_chat_adv_raid_ai_bar", 49, 50, 99, 100},
		{"member_req_ai_bar_pct", "chk_chat_adv_mreq_ai_bar", 49, 50, 99, 100},
		{"member_req_cap_per_min", "chk_chat_adv_mreq_cap", 2, 3, 100, 101},
		{"member_req_pause_mins", "chk_chat_adv_mreq_pause", -1, 1, 60, 61},
		{"lockdown_reminder_mins", "chk_chat_adv_lockdown_rem", 14, 15, 1440, 1441},
		{"captcha_timeout_mins", "chk_chat_adv_captcha_timeout", 10, 11, 60, 61},
	}
	for _, c := range checks {
		q := fmt.Sprintf(`UPDATE chat_advanced_thresholds SET %s = ? WHERE chat_id = ?`, c.col)
		refused(fmt.Sprintf("%s below min (%d)", c.col, c.low), q, "23514", c.constraint, c.low, chat)
		refused(fmt.Sprintf("%s above max (%d)", c.col, c.high), q, "23514", c.constraint, c.high, chat)
		if err := tx.Exec(q, c.min, chat).Error; err != nil {
			t.Errorf("%s min=%d refused: %v", c.col, c.min, err)
		}
		if err := tx.Exec(q, c.max, chat).Error; err != nil {
			t.Errorf("%s max=%d refused: %v", c.col, c.max, err)
		}
		if err := tx.Exec(q, 0, chat).Error; err != nil {
			t.Errorf("%s zero refused: %v", c.col, err)
		}
	}
}
