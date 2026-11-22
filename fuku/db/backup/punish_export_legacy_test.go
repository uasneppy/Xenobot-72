//go:build testtools

package backup

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/antiflood"
	"github.com/uasneppy/fuku_robot/fuku/db/chats"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
)

// Goldens recorded on the code as it was before Phase 26 (timed punishments, the escalation ladder and
// warn expiry): the export JSON of the warns, antiflood and blacklists modules for rows that carry no
// duration, no expiry and no warn times. <chat> stands for the chat ID, <user> for the warned member and
// <time> for every stored time. They are never re-recorded: a later change that alters one of these
// exports for a group that sets none of the new settings is a regression.
const (
	goldenExportWarns      = "{\"warn_settings\":{\"_id\":<chat>,\"warn_limit\":4,\"warn_mode\":\"ban\",\"created_at\":\"<time>\",\"updated_at\":\"<time>\"},\"warns\":[{\"user_id\":<user>,\"chat_id\":<chat>,\"num_warns\":2,\"warns\":[\"spam \\u0026 \\u003cb\\u003elinks\\u003c/b\\u003e\",\"flooding\"],\"created_at\":\"<time>\",\"updated_at\":\"<time>\"}]}"
	goldenExportAntiflood  = "{\"settings\":{\"chat_id\":<chat>,\"limit\":7,\"action\":\"tmute\",\"delete_antiflood_message\":true,\"created_at\":\"<time>\",\"updated_at\":\"<time>\"}}"
	goldenExportBlacklists = "{\"blacklist_mode\":\"kick\",\"entries\":[{\"chat_id\":<chat>,\"word\":\"scam\",\"action\":\"kick\",\"reason\":\"Blacklisted word: '%s'\",\"created_at\":\"<time>\",\"updated_at\":\"<time>\"},{\"chat_id\":<chat>,\"word\":\"spam\",\"action\":\"kick\",\"reason\":\"Blacklisted word: '%s'\",\"created_at\":\"<time>\",\"updated_at\":\"<time>\"}]}"
)

// punishExportTime matches a time as encoding/json writes it (RFC 3339, fractional seconds and zone
// included).
var punishExportTime = regexp.MustCompile(`"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})"`)

// punishExportTokens is the module's export as JSON with the chat ID, the member's ID and every time
// replaced by <chat>, <user> and <time>, the longer ID first.
func punishExportTokens(t *testing.T, chatID, userID int64, module string) string {
	t.Helper()
	exported, err := ExportModuleData(chatID, module)
	if err != nil {
		t.Fatalf("ExportModuleData(%s) error = %v", module, err)
	}
	raw, err := json.Marshal(exported)
	if err != nil {
		t.Fatalf("encode the %s export: %v", module, err)
	}
	out := punishExportTime.ReplaceAllString(string(raw), `"<time>"`)
	out = strings.ReplaceAll(out, strconv.FormatInt(chatID, 10), "<chat>")
	return strings.ReplaceAll(out, strconv.FormatInt(userID, 10), "<user>")
}

// TestLegacyPunishmentExport records what the backup export of the warns, antiflood and blacklists
// modules wrote before Phase 26 added durations, the warn expiry and warn times. It is committed alone,
// before any production change, and is never re-recorded or weakened.
func TestLegacyPunishmentExport(t *testing.T) {
	skipIfNoDb(t)

	chatID := -(time.Now().UnixNano()%1_000_000_000_000 + 1_000_000_000_000)
	userID := time.Now().UnixNano()%1_000_000_000 + 5_000_000_000
	require := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	require(chats.EnsureChatInDb(chatID, "punish_export_golden"), "EnsureChatInDb")
	t.Cleanup(func() {
		cleanupBackupChat(t, chatID)
		db.DB.Where("user_id = ?", userID).Delete(&models.User{})
	})

	require(warns.SetWarnLimit(chatID, 4), "SetWarnLimit")
	require(warns.SetWarnMode(chatID, "ban"), "SetWarnMode")
	for _, reason := range []string{"spam & <b>links</b>", "flooding"} {
		_, _, err := warns.WarnUser(userID, chatID, reason)
		require(err, "WarnUser")
	}

	require(antiflood.SetFlood(chatID, 7), "SetFlood")
	require(antiflood.SetFloodMode(chatID, "tmute"), "SetFloodMode")
	require(antiflood.SetFloodMsgDel(chatID, true), "SetFloodMsgDel")

	// The two words are inserted with their action and the reason /addblacklist stores, so the rows are
	// never updated. They are written in word order, so the export lists them in the same order whether
	// the database reads them through the (chat, word) index or in the order they were written.
	for _, word := range []string{"scam", "spam"} {
		row := models.BlacklistSettings{ChatId: chatID, Word: word, Action: "kick", Reason: "Blacklisted word: '%s'"}
		require(db.DB.Create(&row).Error, "create the blacklist word "+word)
	}

	for _, tc := range []struct {
		module string
		golden string
	}{
		{module: BackupModuleWarns, golden: goldenExportWarns},
		{module: BackupModuleAntiflood, golden: goldenExportAntiflood},
		{module: BackupModuleBlacklists, golden: goldenExportBlacklists},
	} {
		if got := punishExportTokens(t, chatID, userID, tc.module); got != tc.golden {
			t.Errorf("%s export changed\n got %q\nwant %q", tc.module, got, tc.golden)
		}
	}
}
