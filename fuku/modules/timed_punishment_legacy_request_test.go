//go:build testtools

package modules

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"gorm.io/gorm"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/antiflood"
	"github.com/uasneppy/fuku_robot/fuku/db/blacklists"
	dbcache "github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/chats"
	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/memberreq"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
	"github.com/uasneppy/fuku_robot/fuku/utils/cache"
)

// Goldens recorded on the code as it was before Phase 26 (timed punishments, the escalation ladder and
// warn expiry): <chat> stands for the case's chat ID, <admin> for the administrator who runs the
// command, and in the requirement case <user> for the member and <hold> for the hold. They are never
// re-recorded: a later change that alters one of these requests for a group that sets none of the new
// settings is a regression.
const (
	goldenTimedFloodModeNoArgument = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Вкажіть дію проти флуду: \\u003ccode\\u003eban\\u003c/code\\u003e, \\u003ccode\\u003ekick\\u003c/code\\u003e або \\u003ccode\\u003emute\\u003c/code\\u003e.\"}"
	goldenTimedFloodModeMute       = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Дію за флуд змінено на: mute.\"}"
	goldenTimedFloodModeBan        = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Дію за флуд змінено на: ban.\"}"
	goldenTimedFloodModeKick       = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Дію за флуд змінено на: kick.\"}"
	goldenTimedFloodModeUnknown    = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Невідомий тип «xyz». Використайте один із: ban/kick/mute.\"}"
	goldenTimedFloodModeTmute      = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Невідомий тип «tmute». Використайте один із: ban/kick/mute.\"}"
	goldenTimedFloodModeTban       = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Невідомий тип «tban». Використайте один із: ban/kick/mute.\"}"
	goldenTimedFloodModeSaveFailed = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Збій: не вдалося зберегти налаштування. Спробуйте ще раз.\"}"
	goldenTimedFloodModeNotAdmin   = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Доступ заборонено. Для цього потрібно бути адміном.\"}"

	goldenTimedFloodOff         = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Контроль флуду в цьому чаті вимкнено.\"}"
	goldenTimedFloodMute        = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Контроль флуду в цьому чаті спрацьовує після 6 повідомлень. Хто надішле більше, отримає дію: muted.\"}"
	goldenTimedFloodBan         = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Контроль флуду в цьому чаті спрацьовує після 6 повідомлень. Хто надішле більше, отримає дію: banned.\"}"
	goldenTimedFloodKick        = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Контроль флуду в цьому чаті спрацьовує після 6 повідомлень. Хто надішле більше, отримає дію: kicked.\"}"
	goldenTimedFloodStoredTmute = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Контроль флуду в цьому чаті спрацьовує після 6 повідомлень. Хто надішле більше, отримає дію: .\"}"

	goldenTimedWarnModeTmute = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Невідомий тип «tmute». Використайте один із: ban/kick/mute.\"}"

	goldenTimedWarningsDefault     = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Поточні налаштування попереджень групи:\\n\\u003cb\\u003eЛіміт попереджень:\\u003c/b\\u003e \\u003ccode\\u003e3\\u003c/code\\u003e\\n\\u003cb\\u003eДія:\\u003c/b\\u003e \\u003ccode\\u003emute\\u003c/code\\u003e\"}"
	goldenTimedWarningsBan         = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Поточні налаштування попереджень групи:\\n\\u003cb\\u003eЛіміт попереджень:\\u003c/b\\u003e \\u003ccode\\u003e4\\u003c/code\\u003e\\n\\u003cb\\u003eДія:\\u003c/b\\u003e \\u003ccode\\u003eban\\u003c/code\\u003e\"}"
	goldenTimedWarningsStoredTmute = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Поточні налаштування попереджень групи:\\n\\u003cb\\u003eЛіміт попереджень:\\u003c/b\\u003e \\u003ccode\\u003e4\\u003c/code\\u003e\\n\\u003cb\\u003eДія:\\u003c/b\\u003e \\u003ccode\\u003etmute\\u003c/code\\u003e\"}"

	goldenTimedBlActionTmute        = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Оберіть один із варіантів: \\u003ccode\\u003emute\\u003c/code\\u003e, \\u003ccode\\u003ekick\\u003c/code\\u003e, \\u003ccode\\u003eban\\u003c/code\\u003e, \\u003ccode\\u003ewarn\\u003c/code\\u003e або \\u003ccode\\u003enone\\u003c/code\\u003e.\"}"
	goldenTimedBlActionTban         = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Оберіть один із варіантів: \\u003ccode\\u003emute\\u003c/code\\u003e, \\u003ccode\\u003ekick\\u003c/code\\u003e, \\u003ccode\\u003eban\\u003c/code\\u003e, \\u003ccode\\u003ewarn\\u003c/code\\u003e або \\u003ccode\\u003enone\\u003c/code\\u003e.\"}"
	goldenTimedBlActionCurrentTmute = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Поточний режим чорного списку цього чату: \\u003cb\\u003etmute\\u003c/b\\u003e.\"}"

	goldenTimedWarnsNone         = "{\"chat_id\":<chat>,\"reply_parameters\":{\"message_id\":101},\"text\":\"У користувача немає попереджень. Бездоганний послужний список.\"}"
	goldenTimedWarnsThreeReasons = "{\"chat_id\":<chat>,\"reply_parameters\":{\"message_id\":101},\"text\":\"Попереджень у користувача: 3/3. Причини:\\n - first\\n - \\u003cb\\u003ebold\\u003c/b\\u003e \\u0026 co\\n - third\"}"
	goldenTimedWarnsNoReasons    = "{\"chat_id\":<chat>,\"reply_parameters\":{\"message_id\":101},\"text\":\"Попереджень у користувача: 2/3. Причин не вказано.\"}"

	goldenTimedRmWarnRemoved       = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попередження знято. Хто зняв: \\u003ca href=\\\"tg://user?id=<admin>\\\"\\u003eTelegram\\u003c/a\\u003e.\"}"
	goldenTimedRmWarnNothing       = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"У користувача й так немає попереджень.\"}"
	goldenTimedRmWarnButtonRemoved = "{\"chat_id\":<chat>,\"message_id\":102,\"parse_mode\":\"HTML\",\"reply_markup\":{},\"text\":\"Попередження знято. Хто зняв: \\u003ca href=\\\"tg://user?id=<admin>\\\"\\u003eTelegram\\u003c/a\\u003e.\"}"
	goldenTimedRmWarnButtonNothing = "{\"chat_id\":<chat>,\"message_id\":102,\"parse_mode\":\"HTML\",\"reply_markup\":{},\"text\":\"У користувача й так немає попереджень.\"}"
	goldenTimedResetWarnsReset     = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попередження скинуто. Облік очищено.\"}"
	goldenTimedResetWarnsNothing   = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"У користувача й так немає попереджень.\"}"

	goldenTimedWarnLimitInertTmute    = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"\\n - reason\"}"
	goldenTimedFloodInertTmute        = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"reply_markup\":{},\"text\":\"Виявлено флуд. \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt; Lee\\u003c/a\\u003e — дію виконано: .\"}"
	goldenTimedReqWarnLimitInertTmute = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"text\":\"❗ \\u003ca href=\\\"tg://user?id=<user>\\\"\\u003e\u2068Ann \\u0026amp; \\u0026lt;Bo\\u0026gt;\u2069\\u003c/a\\u003e, ви порушуєте вимоги групи: немає прізвища. Попередження 1 з 1.\"}"
)

// timedGoldenAdmin runs every command of the goldens: the fake's creator, an administrator of every chat.
var timedGoldenAdmin = botSiteAdmin

// timedGoldenMember is a plain member of the case's chat, refused by every admin gate.
var timedGoldenMember = gotgbot.User{Id: 42, FirstName: "Member"}

// timedGoldenCase is one case of TestLegacyTimedPunishmentRequests.
type timedGoldenCase struct {
	name string
	kind string // the deletion kind the site's 600 s rule is for (newNoticeGoldenSite)
	// seed prepares the chat's stored state before the command.
	seed func(t *testing.T, s *botSite)
	// run runs the command or the send site.
	run func(t *testing.T, s *botSite)
	// method is the request pinned ("sendMessage" or "editMessageText"); sends how many of them the case
	// makes, the last one being pinned. A case with no request pins none.
	method string
	sends  int
	golden string
	// punish is the one punishment call the case makes (empty for none); deletes the message the case
	// deletes (0 for none checked).
	punish  string
	deletes int64
	// state is what the case pins of the chat's stored state afterwards: each entry is one field of
	// timedGoldenRow ("antiflood=...", "warns_settings=...", "blacklist=[...]" or "warns=..."). A field the
	// case does not name is not checked, so a read's incidental row creation is not pinned.
	state []string
	// queued is the delay of the one queue row of message 9001 under kind; zero checks there is none.
	queued time.Duration
}

// timedGoldenTokens replaces every ID of a serialised request with its token, the longest decimal
// first, so an ID inside a longer one is never matched there.
func timedGoldenTokens(raw string, ids map[int64]string) string {
	keys := make([]string, 0, len(ids))
	tokens := make(map[string]string, len(ids))
	for id, token := range ids {
		s := strconv.FormatInt(id, 10)
		keys = append(keys, s)
		tokens[s] = token
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, k := range keys {
		raw = strings.ReplaceAll(raw, k, tokens[k])
	}
	return raw
}

// timedGoldenRow is the chat's stored punishment state as one compact string of fields joined by "; ",
// read straight from the tables and never through the cache: the antiflood row (limit/action/delete),
// the warns_settings row (limit/mode), the blacklist words (word:action) and the warns of member 42
// (count and reasons).
func timedGoldenRow(t *testing.T, chatID int64) string {
	t.Helper()
	parts := make([]string, 0, 4)

	var flood models.AntifloodSettings
	switch err := db.DB.Where("chat_id = ?", chatID).First(&flood).Error; {
	case errors.Is(err, gorm.ErrRecordNotFound):
		parts = append(parts, "antiflood=none")
	case err != nil:
		t.Fatalf("read antiflood_settings: %v", err)
	default:
		parts = append(parts, fmt.Sprintf("antiflood=%d/%s/%t", flood.Limit, flood.Action, flood.DeleteAntifloodMessage))
	}

	var settings models.WarnSettings
	switch err := db.DB.Where("chat_id = ?", chatID).First(&settings).Error; {
	case errors.Is(err, gorm.ErrRecordNotFound):
		parts = append(parts, "warns_settings=none")
	case err != nil:
		t.Fatalf("read warns_settings: %v", err)
	default:
		parts = append(parts, fmt.Sprintf("warns_settings=%d/%s", settings.WarnLimit, settings.WarnMode))
	}

	var words []models.BlacklistSettings
	if err := db.DB.Where("chat_id = ?", chatID).Find(&words).Error; err != nil {
		t.Fatalf("read blacklists: %v", err)
	}
	list := make([]string, 0, len(words))
	for _, w := range words {
		list = append(list, w.Word+":"+w.Action)
	}
	slices.Sort(list)
	parts = append(parts, "blacklist=["+strings.Join(list, ",")+"]")

	var warnsRow models.Warns
	switch err := db.DB.Where("chat_id = ? AND user_id = ?", chatID, timedGoldenMember.Id).First(&warnsRow).Error; {
	case errors.Is(err, gorm.ErrRecordNotFound):
		parts = append(parts, "warns=none")
	case err != nil:
		t.Fatalf("read warns_users: %v", err)
	default:
		parts = append(parts, fmt.Sprintf("warns=%d%q", warnsRow.NumWarns, []string(warnsRow.Reasons)))
	}
	return strings.Join(parts, "; ")
}

// timedGoldenCleanup deletes the chat's antiflood, warns_settings, blacklist and warns rows and their
// cache entries before the case and again when it ends.
func timedGoldenCleanup(t *testing.T, chatID int64) {
	t.Helper()
	clean := func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.AntifloodSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.WarnSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.BlacklistSettings{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.Warns{})
		dbcache.DeleteCache(dbcache.CacheKey("antiflood", chatID))
		dbcache.DeleteCache(dbcache.CacheKey("warn_settings", chatID))
		dbcache.DeleteCache(dbcache.CacheKey("blacklist", chatID))
		dbcache.DeleteCache(dbcache.CacheKey("warns", timedGoldenMember.Id, chatID))
		cache.InvalidateAdminCache(chatID)
		dbcache.ResetLocalForTest()
	}
	clean()
	t.Cleanup(clean)
}

// timedGoldenFailFloodWrite makes every insert of the chat's antiflood row fail until the test ends,
// through a real GORM callback on the create chain the commands' upsert takes for a chat with no row.
func timedGoldenFailFloodWrite(t *testing.T, chatID int64) {
	t.Helper()
	name := fmt.Sprintf("test:fail_flood_write_%d", chatID)
	err := db.DB.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		row, ok := tx.Statement.Dest.(*models.AntifloodSettings)
		if ok && tx.Statement.Table == "antiflood_settings" && row.ChatId == chatID {
			_ = tx.AddError(errors.New("database is down"))
		}
	})
	if err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() {
		if err := db.DB.Callback().Create().Remove(name); err != nil {
			t.Errorf("remove callback: %v", err)
		}
	})
}

// timedGoldenCommand runs a command handler with the text, from the user, in the site's chat (as a reply
// to member 42's message when reply is set), and fails unless it ends the update.
func timedGoldenCommand(t *testing.T, s *botSite, call func(*gotgbot.Bot, *ext.Context) error, from gotgbot.User, text string, reply bool) {
	t.Helper()
	ctx := newModuleMessageContext(s.bot, s.chat, from, text)
	ctx.EffectiveMessage.Date = s.now.Unix()
	if reply {
		target := timedGoldenMember
		ctx.EffectiveMessage.ReplyToMessage = &gotgbot.Message{MessageId: 303, Date: 1, Chat: s.chat, From: &target, Text: "message being moderated"}
	}
	if err := call(s.bot, ctx); err != ext.EndGroups {
		t.Fatalf("%s error = %v, want EndGroups", text, err)
	}
}

// timedGoldenButton presses the Remove warn button of member 42 as the administrator, on message 102 of
// the site's chat.
func timedGoldenButton(t *testing.T, s *botSite) {
	t.Helper()
	data := encodeCallbackData("rmWarn", map[string]string{"u": strconv.FormatInt(timedGoldenMember.Id, 10)})
	if err := warnsModule.rmWarnButton(s.bot, newModuleCallbackContext(s.bot, s.chat, timedGoldenAdmin, data)); err != ext.EndGroups {
		t.Fatalf("rmWarnButton error = %v, want EndGroups", err)
	}
}

// timedGoldenWarnsOf stores the reasons as member 42's warns of the site's chat, one warn each.
func timedGoldenWarnsOf(reasons ...string) func(*testing.T, *botSite) {
	return func(t *testing.T, s *botSite) {
		t.Helper()
		for _, reason := range reasons {
			if _, _, err := warns.WarnUser(timedGoldenMember.Id, s.chat.Id, reason); err != nil {
				t.Fatalf("WarnUser(%q) error = %v", reason, err)
			}
		}
	}
}

// timedGoldenChatRow makes sure the chats table knows the site's chat, as it does for every group the
// bot has seen (RemoveWarn locks that row).
func timedGoldenChatRow(t *testing.T, s *botSite) {
	t.Helper()
	if err := chats.EnsureChatInDb(s.chat.Id, s.chat.Title); err != nil {
		t.Fatalf("EnsureChatInDb error = %v", err)
	}
}

// timedGoldenFloodRow stores an antiflood row with the limit and the action.
func timedGoldenFloodRow(limit int, action string) func(*testing.T, *botSite) {
	return func(t *testing.T, s *botSite) {
		t.Helper()
		if err := antiflood.SetFlood(s.chat.Id, limit); err != nil {
			t.Fatalf("SetFlood error = %v", err)
		}
		if err := antiflood.SetFloodMode(s.chat.Id, action); err != nil {
			t.Fatalf("SetFloodMode(%q) error = %v", action, err)
		}
	}
}

// timedGoldenWarnSettings stores the warn limit and mode.
func timedGoldenWarnSettings(limit int, mode string) func(*testing.T, *botSite) {
	return func(t *testing.T, s *botSite) {
		t.Helper()
		if err := warns.SetWarnLimit(s.chat.Id, limit); err != nil {
			t.Fatalf("SetWarnLimit error = %v", err)
		}
		if err := warns.SetWarnMode(s.chat.Id, mode); err != nil {
			t.Fatalf("SetWarnMode(%q) error = %v", mode, err)
		}
	}
}

// timedGoldenWord lists the word spam with the action.
func timedGoldenWord(action string) func(*testing.T, *botSite) {
	return func(t *testing.T, s *botSite) {
		t.Helper()
		if err := blacklists.AddBlacklist(s.chat.Id, "spam"); err != nil {
			t.Fatalf("AddBlacklist error = %v", err)
		}
		if err := blacklists.SetBlacklistAction(s.chat.Id, action); err != nil {
			t.Fatalf("SetBlacklistAction(%q) error = %v", action, err)
		}
	}
}

// timedGoldenRun builds the case's environment (newNoticeGoldenSite with the case's kind, a clean chat),
// seeds it, runs the case and checks its request, punishment, deletion, stored state and queue row.
func timedGoldenRun(t *testing.T, tc timedGoldenCase) {
	t.Helper()
	s := newNoticeGoldenSite(t, tc.kind)
	timedGoldenCleanup(t, s.chat.Id)
	resetAntifloodState(t)
	t.Cleanup(func() { resetAntifloodState(t) })
	if tc.seed != nil {
		tc.seed(t, s)
	}

	tc.run(t, s)

	for _, method := range noticeGoldenPunishments {
		want := 0
		if method == tc.punish {
			want = 1
		}
		if got := len(s.client.callsFor(method)); got != want {
			t.Fatalf("%s calls = %d, want %d", method, got, want)
		}
	}
	if tc.deletes != 0 {
		deleted := false
		for _, call := range s.client.callsFor("deleteMessage") {
			if fmt.Sprint(call.Params["message_id"]) == strconv.FormatInt(tc.deletes, 10) {
				deleted = true
			}
		}
		if !deleted {
			t.Fatalf("deleteMessage of message %d not sent", tc.deletes)
		}
	}

	method := tc.method
	if method == "" {
		method = "sendMessage"
	}
	calls := s.client.callsFor(method)
	if len(calls) != tc.sends {
		t.Fatalf("%s calls = %d, want %d", method, len(calls), tc.sends)
	}
	if tc.sends > 0 {
		got := timedGoldenTokens(legacyRequestJSON(t, calls[len(calls)-1]), map[int64]string{
			s.chat.Id: "<chat>", timedGoldenAdmin.Id: "<admin>",
		})
		if got != tc.golden {
			t.Errorf("%s request changed\n got %q\nwant %q", method, got, tc.golden)
		}
	}
	if len(tc.state) > 0 {
		row := timedGoldenRow(t, s.chat.Id)
		fields := strings.Split(row, "; ")
		for _, want := range tc.state {
			key, _, _ := strings.Cut(want, "=")
			found := false
			for _, field := range fields {
				if strings.HasPrefix(field, key+"=") {
					found = true
					if field != want {
						t.Errorf("stored %s = %q, want %q (state %q)", key, field, want, row)
					}
				}
			}
			if !found {
				t.Errorf("stored state %q has no %s field", row, key)
			}
		}
	}
	if tc.queued != 0 {
		s.wantRow(s.chat.Id, 9001, tc.kind, tc.queued)
	} else {
		s.wantNoRows(s.chat.Id)
	}
}

func timedGoldenCases() []timedGoldenCase {
	manual, auto := deletion.KindBotManual, deletion.KindBotAuto
	cmd := func(call func(*gotgbot.Bot, *ext.Context) error, text string) func(*testing.T, *botSite) {
		return func(t *testing.T, s *botSite) { timedGoldenCommand(t, s, call, timedGoldenAdmin, text, false) }
	}
	cmdReply := func(call func(*gotgbot.Bot, *ext.Context) error, text string) func(*testing.T, *botSite) {
		return func(t *testing.T, s *botSite) { timedGoldenCommand(t, s, call, timedGoldenAdmin, text, true) }
	}
	setFloodMode, flood := antifloodModule.setFloodMode, antifloodModule.flood
	return []timedGoldenCase{
		{name: "floodmode_no_argument", kind: manual, run: cmd(setFloodMode, "/setfloodmode"),
			sends: 1, golden: goldenTimedFloodModeNoArgument, state: []string{"antiflood=none"}},
		{name: "floodmode_mute", kind: manual, run: cmd(setFloodMode, "/setfloodmode mute"),
			sends: 1, golden: goldenTimedFloodModeMute, state: []string{"antiflood=none"}},
		{name: "floodmode_ban_upper", kind: manual, run: cmd(setFloodMode, "/setfloodmode BAN"),
			sends: 1, golden: goldenTimedFloodModeBan, state: []string{"antiflood=5/ban/false"}},
		{name: "floodmode_kick", kind: manual, run: cmd(setFloodMode, "/setfloodmode kick"),
			sends: 1, golden: goldenTimedFloodModeKick, state: []string{"antiflood=5/kick/false"}},
		{name: "floodmode_unknown", kind: manual, run: cmd(setFloodMode, "/setfloodmode xyz"),
			sends: 1, golden: goldenTimedFloodModeUnknown, state: []string{"antiflood=none"}},
		{name: "floodmode_tmute_alone", kind: manual, run: cmd(setFloodMode, "/setfloodmode tmute"),
			sends: 1, golden: goldenTimedFloodModeTmute, state: []string{"antiflood=none"}},
		{name: "floodmode_tban_alone", kind: manual, run: cmd(setFloodMode, "/setfloodmode tban"),
			sends: 1, golden: goldenTimedFloodModeTban, state: []string{"antiflood=none"}},
		{name: "floodmode_save_failed", kind: manual, run: func(t *testing.T, s *botSite) {
			timedGoldenFailFloodWrite(t, s.chat.Id)
			timedGoldenCommand(t, s, setFloodMode, timedGoldenAdmin, "/setfloodmode kick", false)
		}, sends: 1, golden: goldenTimedFloodModeSaveFailed, state: []string{"antiflood=none"}},
		{name: "floodmode_not_admin", kind: manual, run: func(t *testing.T, s *botSite) {
			timedGoldenCommand(t, s, setFloodMode, timedGoldenMember, "/setfloodmode ban", false)
		}, sends: 1, golden: goldenTimedFloodModeNotAdmin, state: []string{"antiflood=none"}},

		{name: "flood_off", kind: manual, run: cmd(flood, "/flood"),
			sends: 1, golden: goldenTimedFloodOff},
		{name: "flood_mute", kind: manual, seed: timedGoldenFloodRow(6, "mute"), run: cmd(flood, "/flood"),
			sends: 1, golden: goldenTimedFloodMute},
		{name: "flood_ban", kind: manual, seed: timedGoldenFloodRow(6, "ban"), run: cmd(flood, "/flood"),
			sends: 1, golden: goldenTimedFloodBan},
		{name: "flood_kick", kind: manual, seed: timedGoldenFloodRow(6, "kick"), run: cmd(flood, "/flood"),
			sends: 1, golden: goldenTimedFloodKick},
		{name: "flood_stored_tmute", kind: manual, seed: timedGoldenFloodRow(6, "tmute"), run: cmd(flood, "/flood"),
			sends: 1, golden: goldenTimedFloodStoredTmute},

		{name: "warnmode_tmute_alone", kind: manual, run: cmd(warnsModule.setWarnMode, "/setwarnmode tmute"),
			sends: 1, golden: goldenTimedWarnModeTmute, state: []string{"warns_settings=none"}},

		{name: "warnings_default", kind: manual, run: cmd(warnsModule.warnings, "/warnings"),
			sends: 1, golden: goldenTimedWarningsDefault},
		{name: "warnings_ban", kind: manual, seed: timedGoldenWarnSettings(4, "ban"), run: cmd(warnsModule.warnings, "/warnings"),
			sends: 1, golden: goldenTimedWarningsBan},
		{name: "warnings_stored_tmute", kind: manual, seed: timedGoldenWarnSettings(4, "tmute"), run: cmd(warnsModule.warnings, "/warnings"),
			sends: 1, golden: goldenTimedWarningsStoredTmute},

		{name: "blaction_tmute_alone", kind: manual, seed: timedGoldenWord("warn"), run: cmd(blacklistsModule.setBlacklistAction, "/blacklistaction tmute"),
			sends: 1, golden: goldenTimedBlActionTmute, state: []string{"blacklist=[spam:warn]"}},
		{name: "blaction_tban_alone", kind: manual, seed: timedGoldenWord("warn"), run: cmd(blacklistsModule.setBlacklistAction, "/blacklistaction tban"),
			sends: 1, golden: goldenTimedBlActionTban, state: []string{"blacklist=[spam:warn]"}},
		{name: "blaction_current_tmute", kind: manual, seed: timedGoldenWord("tmute"), run: cmd(blacklistsModule.setBlacklistAction, "/blacklistaction"),
			sends: 1, golden: goldenTimedBlActionCurrentTmute, state: []string{"blacklist=[spam:tmute]"}},

		{name: "warns_none", kind: manual, seed: timedGoldenChatRow, run: cmdReply(warnsModule.warns, "/warns"),
			sends: 1, golden: goldenTimedWarnsNone},
		{name: "warns_three_reasons", kind: manual, seed: timedGoldenWarnsOf("first", "<b>bold</b> & co", "third"),
			run: cmdReply(warnsModule.warns, "/warns"), sends: 1, golden: goldenTimedWarnsThreeReasons},
		{name: "warns_no_reasons", kind: manual, seed: func(t *testing.T, s *botSite) {
			t.Helper()
			timedGoldenChatRow(t, s)
			row := models.Warns{UserId: timedGoldenMember.Id, ChatId: s.chat.Id, NumWarns: 2, Reasons: models.StringArray{}}
			if err := db.DB.Create(&row).Error; err != nil {
				t.Fatalf("seed warns_users: %v", err)
			}
		}, run: cmdReply(warnsModule.warns, "/warns"), sends: 1, golden: goldenTimedWarnsNoReasons},

		{name: "rmwarn_removed", kind: manual, seed: timedGoldenWarnsOf("first", "second"),
			run: cmdReply(warnsModule.removeWarn, "/rmwarn"), sends: 1, golden: goldenTimedRmWarnRemoved, state: []string{"warns=1[\"first\"]"}},
		{name: "rmwarn_nothing", kind: manual, seed: timedGoldenChatRow,
			run: cmdReply(warnsModule.removeWarn, "/rmwarn"), sends: 1, golden: goldenTimedRmWarnNothing, state: []string{"warns=none"}},
		{name: "rmwarn_button_removed", kind: manual, seed: timedGoldenWarnsOf("button"), run: timedGoldenButton,
			method: "editMessageText", sends: 1, golden: goldenTimedRmWarnButtonRemoved, state: []string{"warns=0[]"}},
		{name: "rmwarn_button_nothing", kind: manual, seed: timedGoldenChatRow, run: timedGoldenButton,
			method: "editMessageText", sends: 1, golden: goldenTimedRmWarnButtonNothing, state: []string{"warns=none"}},
		{name: "resetwarns_reset", kind: manual, seed: timedGoldenWarnsOf("first", "second"),
			run: cmdReply(warnsModule.resetWarns, "/resetwarns"), sends: 1, golden: goldenTimedResetWarnsReset, state: []string{"warns=none"}},
		{name: "resetwarns_nothing", kind: manual, seed: timedGoldenChatRow,
			run: cmdReply(warnsModule.resetWarns, "/resetwarns"), sends: 1, golden: goldenTimedResetWarnsNothing, state: []string{"warns=none"}},

		{name: "warnlimit_inert_tmute", kind: manual, seed: timedGoldenWarnSettings(1, "tmute"),
			run:   func(t *testing.T, s *botSite) { noticeGoldenWarn(t, s, warnsModule.warnUser, "/warn reason") },
			sends: 1, golden: goldenTimedWarnLimitInertTmute, state: []string{"warns=1[\"reason\"]"}, queued: 600 * time.Second},
		{name: "flood_inert_tmute", kind: auto,
			run:     func(t *testing.T, s *botSite) { noticeGoldenFlood(t, s, "tmute", false) },
			deletes: 102, sends: 1, golden: goldenTimedFloodInertTmute, queued: 600 * time.Second},
		{name: "blacklist_inert_tban", kind: auto,
			run:     func(t *testing.T, s *botSite) { noticeGoldenBlacklist(t, s, "tban", false, ext.ContinueGroups) },
			deletes: 101, sends: 0},
		{name: "blacklist_inert_tmute", kind: auto,
			run:     func(t *testing.T, s *botSite) { noticeGoldenBlacklist(t, s, "tmute", false, ext.ContinueGroups) },
			deletes: 101, sends: 0},
	}
}

// TestLegacyTimedPunishmentRequests records what /setfloodmode, /flood, /setwarnmode, /warnings,
// /blacklistaction, /warns, /rmwarn, the Remove warn button and /resetwarns sent and stored, and what the
// warn limit, the flood, the blacklist and a member requirement warn did at a timed setting with no
// duration, before Phase 26 touched any of them. It is committed alone, before any production change,
// and is never re-recorded or weakened.
func TestLegacyTimedPunishmentRequests(t *testing.T) {
	withModerationGoldenLocales(t)
	for _, tc := range timedGoldenCases() {
		t.Run(tc.name, func(t *testing.T) { timedGoldenRun(t, tc) })
	}

	t.Run("mreq_warnlimit_inert_tmute", func(t *testing.T) {
		env := newScreenGoldenMreqEnv(t)
		timedGoldenCleanup(t, env.chat.Id)
		env.mreqRules(map[memberreq.Rule]int{memberreq.RuleLastName: models.RequirementActionWarn})
		if err := warns.SetWarnLimit(env.chat.Id, 1); err != nil {
			t.Fatalf("SetWarnLimit() error = %v", err)
		}
		if err := warns.SetWarnMode(env.chat.Id, "tmute"); err != nil {
			t.Fatalf("SetWarnMode() error = %v", err)
		}
		member := screenGoldenReqMember(env)

		got := env.mreqEnforce(env.mreqCase(member))
		if got.Outcome != memberReqWarned || got.Hold == nil {
			t.Fatalf("enforcement = %+v, want a warn with a hold", got)
		}
		screenGoldenWantAction(t, env.calls, "", member.Id)
		if n, _ := warns.GetWarns(member.Id, env.chat.Id); n != 1 {
			t.Fatalf("stored warns = %d, want 1: an inert limit resets nothing", n)
		}
		notices := env.mreqNotices()
		if len(notices) != 1 {
			t.Fatalf("notices = %d, want the one notice", len(notices))
		}
		raw := legacyRequestJSON(t, moduleBotCall{Method: "sendMessage", Params: notices[0].Params})
		if got := screenGoldenTokens(raw, env.chat.Id, member.Id, got.Hold.ID); got != goldenTimedReqWarnLimitInertTmute {
			t.Fatalf("sendMessage request changed\n got %q\nwant %q", got, goldenTimedReqWarnLimitInertTmute)
		}
		env.wantRowFor(notices[0].MessageID, deletion.KindBotAuto, env.now().Add(600*time.Second))
	})
}
