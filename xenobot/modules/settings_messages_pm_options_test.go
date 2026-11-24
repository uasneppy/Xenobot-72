//go:build testtools

package modules

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/logchannels"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/media"
)

// The private copies block of the Messages hub (plans 22-06 and 22-07, D-24, D-25): the Details and log options'
// toggles ho, their write paths applyEventPMDetails and applyEventPMLog, what the send path reads after a press, and who
// may press them. Every case runs for both option rows from one table.

// settingsMsgOptionCase is one option of the hub's private copies block as the tests see it: its code (the t value of
// ho), its line key, how it reads from the stored row and from the send path's options, the write seam its apply
// function calls and the repository writer that sets it directly.
type settingsMsgOptionCase struct {
	name   string
	code   string
	line   string
	stored func(models.ChatEventMessageOptions) bool
	read   func(eventmsg.Options) bool
	seam   *func(ctx context.Context, chatID int64, on bool, by int64) error
	set    func(ctx context.Context, chatID int64, on bool, by int64) error
}

// settingsMsgOptionCases are the hub's two options, Details then the log line.
var settingsMsgOptionCases = []settingsMsgOptionCase{
	{
		name: "details", code: "d", line: "settings_msg_line_pm_details",
		stored: func(r models.ChatEventMessageOptions) bool { return r.PMDetails },
		read:   func(o eventmsg.Options) bool { return o.PMDetails },
		seam:   &eventMsgWrites.PMDetails, set: eventmsg.SetPMDetails,
	},
	{
		name: "log", code: "l", line: "settings_msg_line_pm_log",
		stored: func(r models.ChatEventMessageOptions) bool { return r.PMLog },
		read:   func(o eventmsg.Options) bool { return o.PMLog },
		seam:   &eventMsgWrites.PMLog, set: eventmsg.SetPMLog,
	},
}

// settingsMsgOptionLine is the hub's line of the option in the marker locale: on reads the check and On, off the cross
// and Off.
func settingsMsgOptionLine(opt settingsMsgOptionCase, on bool) string {
	if on {
		return staffMarker(opt.line) + " ✅ " + staffMarker("common_status_on")
	}
	return staffMarker(opt.line) + " ❌ " + staffMarker("common_status_off")
}

// settingsMsgOptionToggle returns the hub's toggle of the option and its decoded fields, failing the test when the hub
// has none.
func settingsMsgOptionToggle(t *testing.T, keyboard [][]gotgbot.InlineKeyboardButton, opt settingsMsgOptionCase) (gotgbot.InlineKeyboardButton, map[string]string) {
	t.Helper()
	button, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgPMOption, "t", opt.code)
	if !ok {
		t.Fatalf("the hub has no %s toggle: %+v", opt.name, keyboard)
	}
	return button, settingsButtonFields(t, button)
}

// settingsMsgStoredOptions reads the chat's private copy options row straight from the database, past every cache
// layer. ok is false when the chat has no row.
func settingsMsgStoredOptions(t *testing.T, chatID int64) (models.ChatEventMessageOptions, bool) {
	t.Helper()
	var rows []models.ChatEventMessageOptions
	if err := db.DB.Where("chat_id = ?", chatID).Find(&rows).Error; err != nil {
		t.Fatalf("read the private copy options of chat %d: %v", chatID, err)
	}
	if len(rows) == 0 {
		return models.ChatEventMessageOptions{}, false
	}
	return rows[0], true
}

// settingsMsgSendPathOptions is the chat's private copy options as the send path would read them right now: the cached
// read of event_messages:<chat> (eventMessagesLookup), with no cache reset.
func settingsMsgSendPathOptions(t *testing.T, chatID int64) eventmsg.Options {
	t.Helper()
	msgs, err := eventMessagesLookup(context.Background(), chatID)
	if err != nil {
		t.Fatalf("the send path's read of chat %d failed: %v", chatID, err)
	}
	return msgs.Options
}

func TestSettingsMessagesPMOptions(t *testing.T) {
	for _, opt := range settingsMsgOptionCases {
		t.Run(opt.name, func(t *testing.T) {
			env := newSettingsMsgEnv(t)
			listEditAdmin(env.settingsEnv, env.chat, env.admin, true)

			// The send path has read the option off and cached that before any press.
			if opt.read(settingsMsgSendPathOptions(t, env.chat.Id)) {
				t.Fatalf("the send path reads the %s option on before any press", opt.name)
			}
			m, text, keyboard := settingsMsgOpenPrivate(env, env.admin)
			for _, key := range []string{"settings_msg_pm_title", "settings_msg_hint_pm_options", "settings_msg_hint_pm_log"} {
				if !containsMarker(text, key) {
					t.Errorf("the hub %q has no %s", text, key)
				}
			}
			for _, each := range settingsMsgOptionCases {
				if !strings.Contains(text, settingsMsgOptionLine(each, false)) {
					t.Errorf("the hub %q does not read the %s option Off", text, each.name)
				}
			}
			settingsMsgWantWidths(t, "the hub", keyboard, 2, 2, 1, 1, 1, 2)
			toggle, fields := settingsMsgOptionToggle(t, keyboard, opt)
			if fields["v"] != "1" || fields["c"] != strconv.FormatInt(env.chat.Id, 10) {
				t.Errorf("the %s toggle carries %v, want v=1 for chat %d", opt.name, fields, env.chat.Id)
			}
			if len(toggle.CallbackData) > 64 {
				t.Errorf("the %s toggle's data %q is %d bytes, over 64", opt.name, toggle.CallbackData, len(toggle.CallbackData))
			}

			// A press writes this option on and no other, and redraws the hub reading On with v=0; the send path's cached
			// read sees it at once.
			answers := env.answerCount()
			text, keyboard = m.pressButton(toggle)
			row, ok := settingsMsgStoredOptions(t, env.chat.Id)
			if !ok || !opt.stored(row) || row.UpdatedBy != env.admin.Id {
				t.Fatalf("after the press the options row is %+v (row %v), want %s on by %d", row, ok, opt.name, env.admin.Id)
			}
			for _, each := range settingsMsgOptionCases {
				if each.code != opt.code && each.stored(row) {
					t.Errorf("the %s toggle also switched the %s option on: %+v", opt.name, each.name, row)
				}
			}
			if n := settingsMsgRows(t, env.chat.Id); n != 0 {
				t.Errorf("the %s toggle wrote %d event message rows, want none", opt.name, n)
			}
			if !strings.Contains(text, settingsMsgOptionLine(opt, true)) || containsMarker(text, "common_settings_save_failed") {
				t.Errorf("the redraw %q is not the hub reading %s On with no notice", text, opt.name)
			}
			settingsMsgWantWidths(t, "the redrawn hub", keyboard, 2, 2, 1, 1, 1, 2)
			if _, fields := settingsMsgOptionToggle(t, keyboard, opt); fields["v"] != "0" {
				t.Errorf("the redrawn toggle carries %v, want v=0", fields)
			}
			if got := env.answerCount() - answers; got != 1 {
				t.Errorf("the press answered %d times, want once", got)
			}
			if answer, alert := env.lastAnswer(); answer != "" || alert {
				t.Errorf("the press answered %q (alert %v), want an empty answer", answer, alert)
			}
			if !opt.read(settingsMsgSendPathOptions(t, env.chat.Id)) {
				t.Errorf("after the hub switched the %s option on, the send path still reads it off", opt.name)
			}

			// A second press writes it off again.
			toggle, _ = settingsMsgOptionToggle(t, keyboard, opt)
			text, _ = m.pressButton(toggle)
			if row, _ := settingsMsgStoredOptions(t, env.chat.Id); opt.stored(row) {
				t.Errorf("the second press left the %s option on", opt.name)
			}
			if !strings.Contains(text, settingsMsgOptionLine(opt, false)) {
				t.Errorf("the redraw after the second press %q does not read Off", text)
			}
			if opt.read(settingsMsgSendPathOptions(t, env.chat.Id)) {
				t.Errorf("after the hub switched the %s option off, the send path still reads it on", opt.name)
			}

			// The toggle is 41 bytes at a 14-character chat ID.
			data := encodeCallbackData(settingsCallbackNamespace, map[string]string{
				"a": settingsActMsgPMOption, "c": "-1001234567890", "t": opt.code, "v": "1",
			})
			if len(data) != 41 {
				t.Errorf("the toggle's data %q is %d bytes, want 41", data, len(data))
			}

			t.Run("a failed write redraws the stored state with the save-failed notice", func(t *testing.T) {
				mreqSwap(t, opt.seam, func(context.Context, int64, bool, int64) error {
					return errors.New("the private copy options are not writable")
				})
				text, keyboard := m.press("a", settingsActMsgPMOption, "t", opt.code, "v", "1")
				if !containsMarker(text, "common_settings_save_failed") || !strings.Contains(text, settingsMsgOptionLine(opt, false)) {
					t.Errorf("the failed press drew %q, want the save-failed notice over the option reading Off", text)
				}
				if _, fields := settingsMsgOptionToggle(t, keyboard, opt); fields["v"] != "1" {
					t.Errorf("the toggle after the failed press carries %v, want v=1", fields)
				}
				if row, _ := settingsMsgStoredOptions(t, env.chat.Id); opt.stored(row) {
					t.Errorf("the failed press turned the %s option on", opt.name)
				}
			})
		})
	}

	t.Run("plain tone switch and authority", func(t *testing.T) {
		env := newSettingsMsgEnv(t)
		listEditAdmin(env.settingsEnv, env.chat, env.admin, true)

		if settingsMsgSendPathOptions(t, env.chat.Id).PlainTone {
			t.Fatal("PlainTone reads on before any press")
		}
		m, text, keyboard := settingsMsgOpenPrivate(env, env.admin)
		for _, key := range []string{"settings_msg_line_plain_tone", "settings_msg_hint_plain_tone"} {
			if !containsMarker(text, key) {
				t.Errorf("the hub %q lacks %s", text, key)
			}
		}
		toggle, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgPlainTone)
		if !ok {
			t.Fatalf("the hub has no Plain tone toggle: %+v", keyboard)
		}
		fields := settingsButtonFields(t, toggle)
		if fields["v"] != "1" || fields["c"] != strconv.FormatInt(env.chat.Id, 10) {
			t.Errorf("Plain tone toggle fields = %v, want v=1 for chat %d", fields, env.chat.Id)
		}
		if len(toggle.CallbackData) > 64 {
			t.Errorf("Plain tone toggle callback data is %d bytes, over 64", len(toggle.CallbackData))
		}

		// Seed PMDetails=true first to prove ht never clobbers sibling option columns.
		if err := eventmsg.SetPMDetails(context.Background(), env.chat.Id, true, 1); err != nil {
			t.Fatalf("SetPMDetails(true): %v", err)
		}

		_, keyboard = m.pressButton(toggle)
		row, exists := settingsMsgStoredOptions(t, env.chat.Id)
		if !exists || !row.PlainTone || !row.PMDetails || row.PMLog || row.UpdatedBy != env.admin.Id {
			t.Fatalf("options row after ht v=1 = %+v (exists=%v), want PlainTone=true, PMDetails=true, PMLog=false", row, exists)
		}
		settingsMsgWantWidths(t, "the redrawn hub after ht", keyboard, 2, 2, 1, 1, 1, 2)
		toggleOff, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgPlainTone)
		if !ok || settingsButtonFields(t, toggleOff)["v"] != "0" {
			t.Fatalf("redrawn Plain tone toggle = %+v, want v=0", toggleOff)
		}
		if !settingsMsgSendPathOptions(t, env.chat.Id).PlainTone {
			t.Error("send path still reads PlainTone=false after ht v=1")
		}

		// Main line reports settings_state_msg_plain when no custom event messages exist.
		mainText, _ := m.press("a", settingsActBack)
		if line, ok := settingsMainStateLine(mainText, "settings_page_msgs"); !ok || !containsMarker(line, "settings_state_msg_plain") {
			t.Errorf("Main Messages line with PlainTone=true = %q (ok=%v), want settings_state_msg_plain", line, ok)
		}

		// Admin without can_change_info cannot toggle Plain tone.
		listEditAdmin(env.settingsEnv, env.chat, env.admin, false)
		env.pressPrivate(env.admin, env.updateID(), env.privateFields(env.chat.Id, "a", settingsActMsgPlainTone, "v", "0"))
		if answer, alert := env.lastAnswer(); !alert || !containsMarker(answer, "settings_no_change_info") {
			t.Errorf("ht without can_change_info answered %q (alert %v), want settings_no_change_info", answer, alert)
		}
		if row, _ := settingsMsgStoredOptions(t, env.chat.Id); !row.PlainTone {
			t.Error("refused ht without can_change_info flipped PlainTone off")
		}

		// Group menu press and forged v=2 are refused in step 1 with 0 Telegram lookups.
		listEditAdmin(env.settingsEnv, env.chat, env.admin, true)
		groupMsgID := env.openMenu(env.admin)
		lookups := env.memberLookupsOf(env.chat.Id, env.admin.Id)
		env.press(env.admin, groupMsgID, env.groupFields(env.admin, "a", settingsActMsgPlainTone, "v", "0"))
		if got := settingsEditLastAnswer(env.settingsEnv); !containsMarker(got, "common_callback_invalid_request") {
			t.Errorf("group ht answered %q, want invalid request", got)
		}
		env.pressPrivate(env.admin, env.updateID(), env.privateFields(env.chat.Id, "a", settingsActMsgPlainTone, "v", "2"))
		if got := settingsEditLastAnswer(env.settingsEnv); !containsMarker(got, "common_callback_invalid_request") {
			t.Errorf("ht v=2 answered %q, want invalid request", got)
		}
		if got := env.memberLookupsOf(env.chat.Id, env.admin.Id) - lookups; got != 0 {
			t.Errorf("refused ht presses made %d member lookups, want 0", got)
		}

		// Failed write redraws stored state with save-failed notice.
		mreqSwap(t, &eventMsgWrites.PlainTone, func(context.Context, int64, bool, int64) error {
			return errors.New("plain tone write failed")
		})
		failText, _ := m.press("a", settingsActMsgPlainTone, "v", "0")
		if !containsMarker(failText, "common_settings_save_failed") {
			t.Errorf("failed ht drew %q, want common_settings_save_failed", failText)
		}
	})

	t.Run("plain tone tracer ban group PM preview and custom override", func(t *testing.T) {
		phase28InProcessMiniredis(t)
		withFastPMCopyPacer(t)
		t.Cleanup(pmCopyJobs.wait)

		realEnv := &settingsMsgEnv{settingsEnv: newSettingsEnvWith(t, settingsEnvOptions{RealLocale: true})}
		settingsMsgCleanup(t, realEnv.chat.Id)
		listEditAdmin(realEnv.settingsEnv, realEnv.chat, realEnv.admin, true)
		settingsEditSetLanguages(t, realEnv.admin.Id, "en", realEnv.chat.Id, "en")
		tr := i18n.MustNewTranslator("en")

		// Enable ban PM copy so the tracer verifies both group and PM outputs end-to-end.
		if err := eventmsg.SetSilent(context.Background(), realEnv.chat.Id, eventmsg.EventBan, eventmsg.DestPM, false, 1); err != nil {
			t.Fatalf("enable ban PM copy: %v", err)
		}

		// Toggle Plain tone ON through the Messages hub button (a=ht&v=1).
		m, _, keyboard := settingsMsgOpenPrivate(realEnv, realEnv.admin)
		toggleOn, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgPlainTone)
		if !ok {
			t.Fatalf("hub lacks Plain tone button: %+v", keyboard)
		}
		_, keyboard = m.pressButton(toggleOn)
		if !settingsMsgSendPathOptions(t, realEnv.chat.Id).PlainTone {
			t.Fatal("PlainTone is not enabled after pressing ht v=1")
		}

		target := gotgbot.User{Id: 7001, FirstName: "Alice"}
		reason := "spam"
		banIn := eventDefaultInput{Target: target, Reason: reason}
		wantPlainGroup := banPlainDefaultReply(tr, banIn)
		if !strings.Contains(wantPlainGroup, "was banned.") || strings.Contains(wantPlainGroup, "🔨") || strings.Contains(wantPlainGroup, "🚫") {
			t.Fatalf("unexpected plain group ban text: %q", wantPlainGroup)
		}
		spec, _ := eventPMSpecFor(eventmsg.EventBan)
		wantPlainPM := eventPMDefaultText(tr, spec, &realEnv.chat, eventValues{Reason: reason}, "", true)
		if !strings.Contains(wantPlainPM, "You were banned from <b>") || strings.Contains(wantPlainPM, "🚫") {
			t.Fatalf("unexpected plain PM ban text: %q", wantPlainPM)
		}

		// 1. postEventSent with Plain tone ON emits plain group text AND plain PM text.
		sendBan := func() (string, string) {
			t.Helper()
			var gotGroup string
			pmMark := len(realEnv.fake.sentTo(target.Id))
			defText, defOpts := banDefaultReply(tr, banIn)
			site := eventSite{
				Bot:   realEnv.bot,
				Chat:  &realEnv.chat,
				Tr:    tr,
				Actor: eventActor{User: &realEnv.admin},
				Kind:  "bot:punish_manual",
				SendDefault: func(text string, _ *gotgbot.SendMessageOpts) (*gotgbot.Message, error) {
					gotGroup = text
					return &gotgbot.Message{MessageId: 5001, Chat: realEnv.chat}, nil
				},
				SendCustom: func(msg media.GreetingMessage) (*gotgbot.Message, error) {
					gotGroup = msg.Text
					return &gotgbot.Message{MessageId: 5002, Chat: realEnv.chat}, nil
				},
				Schedule: func(*gotgbot.Message) {},
			}
			_, err := postEventSent(site, eventReply{
				Event:        eventmsg.EventBan,
				Target:       target,
				Values:       eventValues{Reason: reason},
				Default:      defText,
				PlainDefault: func() string { return banPlainDefaultReply(tr, banIn) },
				Opts:         defOpts,
			})
			if err != nil {
				t.Fatalf("postEventSent(EventBan): %v", err)
			}
			waitEventPM(t)
			pmSent := realEnv.fake.sentTo(target.Id)[pmMark:]
			if len(pmSent) != 1 {
				t.Fatalf("PM deliveries = %d, want 1", len(pmSent))
			}
			return gotGroup, fmt.Sprint(pmSent[0].Params["text"])
		}

		gotGroup, gotPM := sendBan()
		if gotGroup != wantPlainGroup {
			t.Errorf("Plain tone group ban = %q, want %q", gotGroup, wantPlainGroup)
		}
		if gotPM != wantPlainPM {
			t.Errorf("Plain tone PM ban = %q, want %q", gotPM, wantPlainPM)
		}

		// 2. Full previews (group and PM) reflect Plain tone when no custom text is stored.
		sampleReason, _ := tr.GetString("settings_msg_sample_reason")
		wantGroupPreview := banPlainDefaultReply(tr, eventDefaultInput{Target: realEnv.admin, Reason: sampleReason})
		previewGroup := gotgbot.Chat{Id: realEnv.chat.Id, Type: "supergroup", Title: "Test Chat"}
		wantPMPreview := eventPMDefaultText(tr, spec, &previewGroup, eventValues{Reason: sampleReason}, "", true)

		mark := settingsEditMarkSends(realEnv.settingsEnv, realEnv.admin.Id)
		realEnv.pressPrivate(realEnv.admin, realEnv.updateID(), realEnv.privateFields(realEnv.chat.Id,
			"a", settingsActMsgFull, settingsMsgEventField, "bn"))
		sent := settingsEditNewSends(realEnv.settingsEnv, realEnv.admin.Id, mark, false)
		if len(sent) != 2 || fmt.Sprint(rulesCallParams(t, sent[1])["text"]) != wantGroupPreview {
			t.Errorf("group Full preview with PlainTone=true = %+v, want text %q", sent, wantGroupPreview)
		}

		mark = settingsEditMarkSends(realEnv.settingsEnv, realEnv.admin.Id)
		realEnv.pressPrivate(realEnv.admin, realEnv.updateID(), realEnv.privateFields(realEnv.chat.Id,
			"a", settingsActMsgFull, settingsMsgEventField, "bn", settingsMsgDestField, settingsMsgDestPMCode))
		sent = settingsEditNewSends(realEnv.settingsEnv, realEnv.admin.Id, mark, false)
		if len(sent) != 2 || fmt.Sprint(rulesCallParams(t, sent[1])["text"]) != wantPMPreview {
			t.Errorf("PM Full preview with PlainTone=true = %+v, want text %q", sent, wantPMPreview)
		}

		// 3. Custom ban text overrides Plain tone in both group and PM sends and previews.
		if err := eventmsg.SetText(context.Background(), realEnv.chat.Id, eventmsg.EventBan, eventmsg.DestGroup, "Custom group ban {first}", 1); err != nil {
			t.Fatalf("SetText group ban: %v", err)
		}
		if err := eventmsg.SetText(context.Background(), realEnv.chat.Id, eventmsg.EventBan, eventmsg.DestPM, "Custom PM ban {first}", 1); err != nil {
			t.Fatalf("SetText PM ban: %v", err)
		}
		gotGroup, gotPM = sendBan()
		if gotGroup != "Custom group ban "+target.FirstName {
			t.Errorf("custom group ban with PlainTone=true = %q, want custom override", gotGroup)
		}
		if gotPM != "Custom PM ban "+target.FirstName {
			t.Errorf("custom PM ban with PlainTone=true = %q, want custom override", gotPM)
		}

		// 4. Removing custom text and toggling Plain tone OFF via ht v=0 restores TestLegacyPhase29Baseline byte-for-byte.
		if err := eventmsg.SetText(context.Background(), realEnv.chat.Id, eventmsg.EventBan, eventmsg.DestGroup, "", 1); err != nil {
			t.Fatalf("clear group ban text: %v", err)
		}
		if err := eventmsg.SetText(context.Background(), realEnv.chat.Id, eventmsg.EventBan, eventmsg.DestPM, "", 1); err != nil {
			t.Fatalf("clear PM ban text: %v", err)
		}
		toggleOff, ok := settingsMsgButtonWith(t, keyboard, "a", settingsActMsgPlainTone)
		if !ok || settingsButtonFields(t, toggleOff)["v"] != "0" {
			t.Fatalf("hub lacks ht v=0 button: %+v", keyboard)
		}
		m.pressButton(toggleOff)
		if settingsMsgSendPathOptions(t, realEnv.chat.Id).PlainTone {
			t.Fatal("PlainTone is still on after pressing ht v=0")
		}
		gotGroup, gotPM = sendBan()
		if gotGroup != goldenP29GroupDefaults[eventmsg.EventBan] {
			t.Errorf("tone-off restored group ban = %q, want baseline %q", gotGroup, goldenP29GroupDefaults[eventmsg.EventBan])
		}
		wantBaselinePM := eventPMDefaultText(tr, spec, &realEnv.chat, eventValues{Reason: reason}, "", false)
		if gotPM != wantBaselinePM {
			t.Errorf("tone-off restored PM ban = %q, want %q", gotPM, wantBaselinePM)
		}
	})
}

// settingsMsgOptionEffect counts what a blocked copy's job did for the option after the /ban that punished member: the
// Details edits of the group message, or the #PM_COPY posts in the log channel.
func settingsMsgOptionEffect(e *messagesE2E, opt settingsMsgOptionCase, channelID int64) int {
	if opt.code == "d" {
		return len(callsToChat(e.fake.staffBotClient, "editMessageReplyMarkup", e.chat.Id))
	}
	n := 0
	for _, call := range callsToChat(e.fake.staffBotClient, "sendMessage", channelID) {
		if strings.Contains(fmt.Sprint(call.Params["text"]), pmCopyLogTag) {
			n++
		}
	}
	return n
}

func TestSettingsDriftMessagesPMOptions(t *testing.T) {
	for _, opt := range settingsMsgOptionCases {
		t.Run(opt.name, func(t *testing.T) {
			t.Run("the hub draws what the repository stored", func(t *testing.T) {
				env := newSettingsMsgEnv(t)
				listEditAdmin(env.settingsEnv, env.chat, env.admin, true)
				if err := opt.set(context.Background(), env.chat.Id, true, 1); err != nil {
					t.Fatalf("setting the %s option error = %v", opt.name, err)
				}
				_, text, keyboard := settingsMsgOpenPrivate(env, env.admin)
				if !strings.Contains(text, settingsMsgOptionLine(opt, true)) {
					t.Errorf("the hub %q does not read the %s option the repository stored", text, opt.name)
				}
				if _, fields := settingsMsgOptionToggle(t, keyboard, opt); fields["v"] != "0" {
					t.Errorf("the toggle carries %v, want v=0", fields)
				}
			})

			t.Run("what the hub sets is what a blocked copy's job reads at the next /ban", func(t *testing.T) {
				withFastPMCopyPacer(t)
				e := newMessagesE2E(t, false)
				t.Cleanup(pmCopyJobs.wait)
				channelID := uniqueModuleChatID()
				if err := logchannels.Set(e.chat.Id, e.chat.Title, channelID); err != nil {
					t.Fatalf("logchannels.Set error = %v", err)
				}
				t.Cleanup(func() { db.DB.Where("chat_id = ?", e.chat.Id).Delete(&models.LogChannel{}) })
				blocked := &gotgbot.TelegramError{Method: "sendMessage", Code: 403, Description: "Forbidden: bot was blocked by the user"}
				first, second := e.member("Ann"), e.member("Bob")
				e.client.refuse(first.Id, blocked)
				e.client.refuse(second.Id, blocked)
				if err := eventmsg.SetSilent(context.Background(), e.chat.Id, eventmsg.EventBan, eventmsg.DestPM, false, 1); err != nil {
					t.Fatalf("switch the ban's private copy on: %v", err)
				}

				// With the option off, a blocked copy does nothing for it; the send path has cached the off.
				e.command(e.settingsEnv, e.admin, "/ban spam", first)
				waitEventPM(t)
				if n := settingsMsgOptionEffect(e, opt, channelID); n != 0 {
					t.Fatalf("with the %s option off the blocked copy's job did it %d times, want never", opt.name, n)
				}

				// The owner switches the option on with the hub's drawn toggle.
				m := e.openMain(e.owner)
				m.tap("a", settingsActOpen, "p", settingsPageMessages)
				m.tap("a", settingsActMsgPMOption, "t", opt.code, "v", "1")
				if row, ok := settingsMsgStoredOptions(t, e.chat.Id); !ok || !opt.stored(row) {
					t.Fatalf("the hub left the options row %+v (row %v), want %s on", row, ok, opt.name)
				}

				// The next /ban's blocked copy reads it and acts once.
				e.command(e.settingsEnv, e.admin, "/ban spam", second)
				waitEventPM(t)
				if n := settingsMsgOptionEffect(e, opt, channelID); n != 1 {
					t.Fatalf("after the hub switched the %s option on the blocked copy's job did it %d times, want once", opt.name, n)
				}
				if opt.code == "d" {
					edits := callsToChat(e.fake.staffBotClient, "editMessageReplyMarkup", e.chat.Id)
					if markup := fmt.Sprint(edits[0].Params["reply_markup"]); !strings.Contains(markup, "https://t.me/XenobotTestBot?start="+pmCopyDetailsPrefix) {
						t.Errorf("the edit's keyboard %s has no Details button", markup)
					}
				}
				if got := len(e.fake.sentTo(second.Id)); got != 0 {
					t.Errorf("the blocked member got %d private messages, want none", got)
				}
			})
		})
	}
}

func TestSettingsAuthorityMessagesPMOptions(t *testing.T) {
	// refused presses kv (plus the private owner field) as user on a fresh private message and fails the test unless it
	// is answered the alert key with nothing edited and no options row written.
	refused := func(t *testing.T, env *settingsMsgEnv, user gotgbot.User, key string, kv ...string) {
		t.Helper()
		before, existed := settingsMsgStoredOptions(t, env.chat.Id)
		msgID := env.updateID()
		env.pressPrivate(user, msgID, env.privateFields(env.chat.Id, kv...))
		if answer, alert := env.lastAnswer(); !alert || !containsMarker(answer, key) {
			t.Errorf("%v answered %q (alert %v), want the alert %s", kv, answer, alert, key)
		}
		if got := len(env.edits(user.Id, msgID)); got != 0 {
			t.Errorf("the refused %v made %d edits, want none", kv, got)
		}
		after, exists := settingsMsgStoredOptions(t, env.chat.Id)
		if exists != existed || after.PMDetails != before.PMDetails || after.PMLog != before.PMLog || !after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Errorf("the refused %v wrote the options row: %+v", kv, after)
		}
	}

	for _, opt := range settingsMsgOptionCases {
		toggle := []string{"a", settingsActMsgPMOption, "t", opt.code, "v", "1"}
		t.Run(opt.name, func(t *testing.T) {
			t.Run("an administrator without can_change_info views but changes nothing", func(t *testing.T) {
				env := newSettingsMsgEnv(t)
				listEditAdmin(env.settingsEnv, env.chat, env.admin, false)
				_, text, keyboard := settingsMsgOpenPrivate(env, env.admin)
				if !strings.Contains(text, settingsMsgOptionLine(opt, false)) {
					t.Fatalf("the hub did not open for an administrator without can_change_info: %q", text)
				}
				settingsMsgOptionToggle(t, keyboard, opt)
				refused(t, env, env.admin, "settings_no_change_info", toggle...)
			})

			t.Run("an administrator the owner gate refuses changes nothing", func(t *testing.T) {
				resetSettingsOwnerMemo(t)
				env := newSettingsMsgEnv(t)
				env.newOwner(env.chat)
				listEditAdmin(env.settingsEnv, env.chat, env.admin, true)
				env.setAdminsAllowed(env.chat.Id, false)
				refused(t, env, env.admin, "settings_gate_owner_only", toggle...)
			})

			t.Run("forged fields and a group menu's press are refused in step 1", func(t *testing.T) {
				env := newSettingsMsgEnv(t)
				listEditAdmin(env.settingsEnv, env.chat, env.admin, true)
				for _, kv := range [][]string{
					{"a", settingsActMsgPMOption, "t", "x", "v", "1"},
					{"a", settingsActMsgPMOption, "t", opt.code, "v", "2"},
					{"a", settingsActMsgPMOption, "t", opt.code},
				} {
					lookups := env.memberLookupsOf(env.chat.Id, env.admin.Id)
					msgID := env.updateID()
					env.pressPrivate(env.admin, msgID, env.privateFields(env.chat.Id, kv...))
					if got := settingsEditLastAnswer(env.settingsEnv); !containsMarker(got, "common_callback_invalid_request") {
						t.Errorf("the private %v answered %q, want the invalid request", kv, got)
					}
					if got := env.memberLookupsOf(env.chat.Id, env.admin.Id); got != lookups {
						t.Errorf("the private %v made %d member lookups, want none", kv, got-lookups)
					}
				}
				msgID := env.openMenu(env.admin)
				lookups, edits := env.memberLookupsOf(env.chat.Id, env.admin.Id), len(env.edits(env.chat.Id, msgID))
				env.press(env.admin, msgID, env.groupFields(env.admin, toggle...))
				if got := settingsEditLastAnswer(env.settingsEnv); !containsMarker(got, "common_callback_invalid_request") {
					t.Errorf("a group menu's ho answered %q, want the invalid request", got)
				}
				if got := env.memberLookupsOf(env.chat.Id, env.admin.Id); got != lookups {
					t.Errorf("a group menu's ho made %d member lookups, want none", got-lookups)
				}
				if got := len(env.edits(env.chat.Id, msgID)); got != edits {
					t.Error("a group menu's ho edited the menu")
				}
				if _, ok := settingsMsgStoredOptions(t, env.chat.Id); ok {
					t.Error("the refused presses wrote an options row")
				}
			})
		})
	}
}
