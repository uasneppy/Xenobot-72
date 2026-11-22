//go:build testtools

package modules

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/uasneppy/fuku_robot/fuku/db/deletion"
	"github.com/uasneppy/fuku_robot/fuku/db/warns"
)

// Goldens recorded on the unchanged code at b7393953 before Phase 27 (manual command defaults,
// reason shortcuts and appeal button); <chat> stands for the case's chat ID. They are never
// re-recorded: a later change that alters one of these requests for a group with nothing stored
// is a regression.
const (
	goldenP27TbanMissingDuration     = "{\"chat_id\":<chat>,\"reply_parameters\":{\"message_id\":101},\"text\":\"Вкажіть тривалість (напр., 1m, 1h, 1d, 1w).\"}"
	goldenP27TbanInvalidDuration     = "{\"chat_id\":<chat>,\"reply_parameters\":{\"message_id\":101},\"text\":\"Некоректна одиниця часу. Очікувалося m, h, d або w, отримано: 99q\"}"
	goldenP27TmuteMissingDuration    = "{\"chat_id\":<chat>,\"reply_parameters\":{\"message_id\":101},\"text\":\"Вкажіть тривалість (напр., 1m, 1h, 1d, 1w).\"}"
	goldenP27TmuteInvalidDuration    = "{\"chat_id\":<chat>,\"reply_parameters\":{\"message_id\":101},\"text\":\"Некоректна одиниця часу. Очікувалося m, h, d або w, отримано: 99q\"}"
	goldenP27BanLeadingDurationText  = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"Розбанити (лише адміни)\",\"callback_data\":\"unrestrict|v1|a=unban\\u0026u=42\"}]]},\"reply_parameters\":{\"message_id\":101},\"text\":\"Видалення з об'єкта виконано: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e забанено.\\n\\u003cb\\u003eПричина:\\u003c/b\\u003e 2h spam\"}"
	goldenP27MuteLeadingDurationText = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"Зняти мут (лише адміни)\",\"callback_data\":\"unrestrict|v1|a=unmute\\u0026u=42\"}]]},\"reply_parameters\":{\"message_id\":101},\"text\":\"Режим тиші запроваджено: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e отримує мут.\\n\\u003cb\\u003eПричина:\\u003c/b\\u003e 30m noise\"}"
	goldenP27BanWordReasonNoUntil    = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"Розбанити (лише адміни)\",\"callback_data\":\"unrestrict|v1|a=unban\\u0026u=42\"}]]},\"reply_parameters\":{\"message_id\":101},\"text\":\"Видалення з об'єкта виконано: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e забанено.\\n\\u003cb\\u003eПричина:\\u003c/b\\u003e spam\"}"
	goldenP27DbanNoReasonNoUntil     = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"Розбанити (лише адміни)\",\"callback_data\":\"unrestrict|v1|a=unban\\u0026u=42\"}]]},\"reply_parameters\":{\"message_id\":101},\"text\":\"Видалення з об'єкта виконано: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e забанено.\"}"
	goldenP27MuteNoReasonNoUntil     = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"Зняти мут (лише адміни)\",\"callback_data\":\"unrestrict|v1|a=unmute\\u0026u=42\"}]]},\"reply_parameters\":{\"message_id\":101},\"text\":\"Режим тиші запроваджено: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e отримує мут.\"}"
	goldenP27DmuteWordReasonNoUntil  = "{\"chat_id\":<chat>,\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"Зняти мут (лише адміни)\",\"callback_data\":\"unrestrict|v1|a=unmute\\u0026u=42\"}]]},\"reply_parameters\":{\"message_id\":101},\"text\":\"Режим тиші запроваджено: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e отримує мут.\\n\\u003cb\\u003eПричина:\\u003c/b\\u003e noise\"}"
	goldenP27KickNoReason            = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"\\u003ca href=\\\"tg://user?id=42\\\"\\u003eAnn \\u0026amp; \\u0026lt;Bo\\u0026gt;\\u003c/a\\u003e вигнано з об'єкта.\"}"
	goldenP27WarnNoReason            = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"❌ Зняти попередження\",\"callback_data\":\"rmWarn|v1|u=42\"}]]},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попередження про порушення: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eMember\\u003c/a\\u003e, будьте обережні.\\n\\u003cb\\u003eПопереджень:\\u003c/b\\u003e 1/3\"}"
	goldenP27WarnWordReason          = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"❌ Зняти попередження\",\"callback_data\":\"rmWarn|v1|u=42\"}]]},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попередження про порушення: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eMember\\u003c/a\\u003e, будьте обережні.\\n\\u003cb\\u003eПопереджень:\\u003c/b\\u003e 1/3\\n\\u003cb\\u003eПричина:\\u003c/b\\u003e spam\"}"
	goldenP27DwarnNoReason           = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"❌ Зняти попередження\",\"callback_data\":\"rmWarn|v1|u=42\"}]]},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попередження про порушення: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eMember\\u003c/a\\u003e, будьте обережні.\\n\\u003cb\\u003eПопереджень:\\u003c/b\\u003e 1/3\"}"
	goldenP27SwarnNoReason           = "{\"chat_id\":<chat>,\"link_preview_options\":{\"is_disabled\":true},\"parse_mode\":\"HTML\",\"reply_markup\":{\"inline_keyboard\":[[{\"text\":\"❌ Зняти попередження\",\"callback_data\":\"rmWarn|v1|u=42\"}]]},\"reply_parameters\":{\"message_id\":101,\"allow_sending_without_reply\":true},\"text\":\"Попередження про порушення: \\u003ca href=\\\"tg://user?id=42\\\"\\u003eMember\\u003c/a\\u003e, будьте обережні.\\n\\u003cb\\u003eПопереджень:\\u003c/b\\u003e 1/3\"}"
)

// phase27AssertNoUntilDate asserts that an untimed banChatMember or restrictChatMember call
// carries no until_date (either omitted or zero).
func phase27AssertNoUntilDate(t *testing.T, call moduleBotCall) {
	t.Helper()
	raw, ok := call.Params["until_date"]
	if !ok || raw == nil {
		return
	}
	switch v := raw.(type) {
	case int:
		if v == 0 {
			return
		}
	case int64:
		if v == 0 {
			return
		}
	case float64:
		if v == 0 {
			return
		}
	case string:
		if v == "" || v == "0" {
			return
		}
	}
	t.Fatalf("%s carries unexpected until_date = %#v (%T), want absent or zero", call.Method, raw, raw)
}

// phase27GoldenCase describes one manual moderation characterization case for Phase 27.
type phase27GoldenCase struct {
	name           string
	text           string
	reply          bool
	call           func(b *gotgbot.Bot, ctx *ext.Context) error
	method         string // expected punishment method, or "" for warn cases
	forbidMethod   string // method that must not be called (refusal cases)
	checkNoUntil   bool   // assert until_date is absent or zero on the punishment call
	deletes        int64  // message ID expected to be deleted via deleteMessage; 0 if none
	golden         string // exact sendMessage JSON with <chat> placeholder; empty for silent
	silent         bool   // expect zero sendMessage calls and zero deletion queue rows
	refusal        bool   // expect zero deletion queue rows despite sending a refusal reply
	checkWarns     bool   // verify warns.GetWarns after handler returns
	wantWarnCount  int
	wantWarnReason []string
}

func phase27GoldenRun(t *testing.T, tc phase27GoldenCase) {
	t.Helper()
	s := newModerationGoldenSite(t)

	if err := tc.call(s.bot, s.commandCtx(tc.text, tc.reply, false)); err != ext.EndGroups {
		t.Fatalf("%s error = %v, want EndGroups", tc.text, err)
	}

	if tc.forbidMethod != "" {
		if got := len(s.client.callsFor(tc.forbidMethod)); got != 0 {
			t.Fatalf("%s calls = %d, want 0 on refusal", tc.forbidMethod, got)
		}
	}

	if tc.method != "" {
		punish := s.client.callsFor(tc.method)
		if len(punish) != 1 {
			t.Fatalf("%s calls = %d, want 1", tc.method, len(punish))
		}
		if got := fmt.Sprint(punish[0].Params["user_id"]); got != strconv.FormatInt(moderationGoldenTarget.Id, 10) {
			t.Fatalf("%s user_id = %s, want %d", tc.method, got, moderationGoldenTarget.Id)
		}
		if tc.checkNoUntil {
			phase27AssertNoUntilDate(t, punish[0])
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

	if tc.checkWarns {
		count, reasons := warns.GetWarns(moderationGoldenTarget.Id, s.chat.Id)
		if count != tc.wantWarnCount {
			t.Fatalf("warns.GetWarns count = %d, want %d", count, tc.wantWarnCount)
		}
		if !slices.Equal(reasons, tc.wantWarnReason) {
			t.Fatalf("warns.GetWarns reasons = %#v, want %#v", reasons, tc.wantWarnReason)
		}
	}

	sends := s.client.callsFor("sendMessage")
	if tc.silent {
		if len(sends) != 0 {
			t.Fatalf("sendMessage calls = %d, want 0 from a silent command", len(sends))
		}
		s.wantNoRows(s.chat.Id)
		return
	}

	if len(sends) != 1 {
		t.Fatalf("sendMessage calls = %d, want 1", len(sends))
	}
	got := strings.ReplaceAll(legacyRequestJSON(t, sends[0]), strconv.FormatInt(s.chat.Id, 10), "<chat>")
	if got != tc.golden {
		t.Fatalf("sendMessage request changed\n got %q\nwant %q", got, tc.golden)
	}

	if tc.refusal {
		s.wantNoRows(s.chat.Id)
	} else {
		s.wantRow(s.chat.Id, 9001, deletion.KindBotManual, 600*time.Second)
	}
}

func phase27GoldenCases() []phase27GoldenCase {
	return []phase27GoldenCase{
		{
			name:         "tban_missing_duration",
			text:         "/tban",
			reply:        true,
			call:         bansModule.tBan,
			forbidMethod: "banChatMember",
			golden:       goldenP27TbanMissingDuration,
			refusal:      true,
		},
		{
			name:         "tban_invalid_duration",
			text:         "/tban 99q flood",
			reply:        true,
			call:         bansModule.tBan,
			forbidMethod: "banChatMember",
			golden:       goldenP27TbanInvalidDuration,
			refusal:      true,
		},
		{
			name:         "tmute_missing_duration",
			text:         "/tmute",
			reply:        true,
			call:         mutesModule.tMute,
			forbidMethod: "restrictChatMember",
			golden:       goldenP27TmuteMissingDuration,
			refusal:      true,
		},
		{
			name:         "tmute_invalid_duration",
			text:         "/tmute 99q noise",
			reply:        true,
			call:         mutesModule.tMute,
			forbidMethod: "restrictChatMember",
			golden:       goldenP27TmuteInvalidDuration,
			refusal:      true,
		},
		{
			name:         "ban_leading_duration_like_text",
			text:         "/ban 2h spam",
			reply:        true,
			call:         bansModule.ban,
			method:       "banChatMember",
			checkNoUntil: true,
			golden:       goldenP27BanLeadingDurationText,
		},
		{
			name:         "mute_leading_duration_like_text",
			text:         "/mute 30m noise",
			reply:        true,
			call:         mutesModule.mute,
			method:       "restrictChatMember",
			checkNoUntil: true,
			golden:       goldenP27MuteLeadingDurationText,
		},
		{
			name:         "ban_word_reason_no_until",
			text:         "/ban spam",
			reply:        true,
			call:         bansModule.ban,
			method:       "banChatMember",
			checkNoUntil: true,
			golden:       goldenP27BanWordReasonNoUntil,
		},
		{
			name:         "dban_no_reason_no_until",
			text:         "/dban",
			reply:        true,
			call:         bansModule.dBan,
			method:       "banChatMember",
			checkNoUntil: true,
			deletes:      303,
			golden:       goldenP27DbanNoReasonNoUntil,
		},
		{
			name:         "sban_no_reason_no_until",
			text:         "/sban 42",
			call:         bansModule.sBan,
			method:       "banChatMember",
			checkNoUntil: true,
			deletes:      101,
			silent:       true,
		},
		{
			name:         "mute_no_reason_no_until",
			text:         "/mute",
			reply:        true,
			call:         mutesModule.mute,
			method:       "restrictChatMember",
			checkNoUntil: true,
			golden:       goldenP27MuteNoReasonNoUntil,
		},
		{
			name:         "dmute_word_reason_no_until",
			text:         "/dmute noise",
			reply:        true,
			call:         mutesModule.dMute,
			method:       "restrictChatMember",
			checkNoUntil: true,
			deletes:      303,
			golden:       goldenP27DmuteWordReasonNoUntil,
		},
		{
			name:         "smute_no_reason_no_until",
			text:         "/smute 42",
			call:         mutesModule.sMute,
			method:       "restrictChatMember",
			checkNoUntil: true,
			deletes:      101,
			silent:       true,
		},
		{
			name:   "kick_no_reason",
			text:   "/kick",
			reply:  true,
			call:   bansModule.kick,
			method: "unbanChatMember",
			golden: goldenP27KickNoReason,
		},
		{
			name:           "warn_no_reason",
			text:           "/warn",
			reply:          true,
			call:           warnsModule.warnUser,
			golden:         goldenP27WarnNoReason,
			checkWarns:     true,
			wantWarnCount:  1,
			wantWarnReason: []string{"Причину не вказано"},
		},
		{
			name:           "warn_word_reason",
			text:           "/warn spam",
			reply:          true,
			call:           warnsModule.warnUser,
			golden:         goldenP27WarnWordReason,
			checkWarns:     true,
			wantWarnCount:  1,
			wantWarnReason: []string{"spam"},
		},
		{
			name:           "dwarn_no_reason",
			text:           "/dwarn",
			reply:          true,
			call:           warnsModule.dWarnUser,
			deletes:        303,
			golden:         goldenP27DwarnNoReason,
			checkWarns:     true,
			wantWarnCount:  1,
			wantWarnReason: []string{"Причину не вказано"},
		},
		{
			name:           "swarn_no_reason",
			text:           "/swarn",
			reply:          true,
			call:           warnsModule.sWarnUser,
			deletes:        101,
			golden:         goldenP27SwarnNoReason,
			checkWarns:     true,
			wantWarnCount:  1,
			wantWarnReason: []string{"Причину не вказано"},
		},
	}
}

func TestLegacyPhase27ModerationRequests(t *testing.T) {
	withModerationGoldenLocales(t)
	for _, tc := range phase27GoldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			phase27GoldenRun(t, tc)
		})
	}
}
