//go:build testtools

package modules

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/eventmsg"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
)

// settingsMsgScreenFields are the only callback fields a button of the Messages page may carry: the action, the owner
// field of either menu, the page, the family or part, the event, the destination (d=p on the private copy's buttons,
// Phase 22) and the absolute switch target.
var settingsMsgScreenFields = map[string]bool{"a": true, "c": true, "u": true, "p": true, "t": true, "k": true, "d": true, "v": true}

// settingsMsgAllCustom is the stored messages of a chat whose every event of every family has its own text, so the hub and
// the family list draw every line Custom. The map key is the repository's own (event|dest); a Lookup of each event proves
// the set was built right, so a change of that key fails here and not silently.
func settingsMsgAllCustom(t *testing.T, silent bool) *eventmsg.Messages {
	t.Helper()
	msgs := &eventmsg.Messages{ChatID: -1001234567890, Rows: map[string]eventmsg.Message{}}
	for _, fam := range settingsMsgFamilies {
		for _, ev := range fam.Events {
			msgs.Rows[ev.Key+"|"+eventmsg.DestGroup] = eventmsg.Message{
				Event: ev.Key, Dest: eventmsg.DestGroup, Text: "custom", Silent: silent,
			}
		}
	}
	for _, fam := range settingsMsgFamilies {
		for _, ev := range fam.Events {
			if _, ok := msgs.Lookup(ev.Key, eventmsg.DestGroup); !ok {
				t.Fatalf("the hand-built messages have no row for %s", ev.Key)
			}
		}
	}
	return msgs
}

// settingsMsgFamilyPM says whether the events of a family have a private copy: every event of the manual, Automatic and
// Member requirements families has a spec (since plans 23-05 and 24-03), and no event of the Lockdown family has one
// (Phase 25, D-04). A family the table does not list fails, so a new family states its answer here.
var settingsMsgFamilyPM = map[string]bool{"m": true, "a": true, "r": true, "l": false}

// settingsMsgWantFamilyPM returns whether ev, an event of fam, has a private copy, and fails when its spec disagrees with
// settingsMsgFamilyPM: a missing spec in a family that has them, or any spec in the Lockdown family.
func settingsMsgWantFamilyPM(t *testing.T, fam settingsMsgFamily, ev settingsMsgEvent) bool {
	t.Helper()
	want, ok := settingsMsgFamilyPM[fam.Code]
	if !ok {
		t.Fatalf("no expected private copy answer for the %s family", fam.Code)
	}
	if _, spec := eventPMSpecFor(ev.Key); spec != want {
		t.Fatalf("the %s event of the %s family has a private copy spec %v, want %v", ev.Key, fam.Code, spec, want)
	}
	return want
}

// settingsMsgEveryPMOn is settingsMsgAllCustom with every event's private copy switched on (a pm row with a text and
// Silent false) and both options off, so the hub draws the private copy mark on every event line that has a private
// copy beside the block's Off words. Every event of the manual, Automatic and Member requirements families has a spec,
// and no lockdown event has one (settingsMsgWantFamilyPM), so a lockdown event gets no pm row.
func settingsMsgEveryPMOn(t *testing.T) *eventmsg.Messages {
	t.Helper()
	msgs := settingsMsgAllCustom(t, false)
	for _, fam := range settingsMsgFamilies {
		for _, ev := range fam.Events {
			if !settingsMsgWantFamilyPM(t, fam, ev) {
				continue
			}
			msgs.Rows[ev.Key+"|"+eventmsg.DestPM] = eventmsg.Message{Event: ev.Key, Dest: eventmsg.DestPM, Text: "custom"}
			if !settingsMsgPMOn(msgs, ev) {
				t.Fatalf("the hand-built messages do not switch the %s private copy on", ev.Key)
			}
		}
	}
	return msgs
}

// settingsMsgFullState is an event message with every part stored (a long text, the media kind given, one button) and
// Silent on: the event screen's worst case, eight rows (the private copy screen's worst case too, seven rows).
func settingsMsgFullState(mediaType int) settingsEditState {
	return settingsEditState{
		Kind: settingsEditEvent, Text: strings.Repeat("x", 3000), MediaType: mediaType, FileID: "f",
		Buttons: models.ButtonArray{{Name: "B", Url: "https://example.com"}}, Silent: true,
	}
}

// settingsMsgEmptyState is an event message with nothing stored: the default text, no media, no buttons, Silent off.
func settingsMsgEmptyState() settingsEditState {
	return settingsEditState{Kind: settingsEditEvent, TextIsDefault: true}
}

// settingsMsgPMOffState is a private copy with nothing stored, as the page reads a missing pm row: the default text,
// no media, no buttons and the switch off (Silent true).
func settingsMsgPMOffState() settingsEditState {
	return settingsEditState{Kind: settingsEditEvent, TextIsDefault: true, Silent: true}
}

// settingsMsgFamilyWidths are the row widths of each family's list: its event cells two to a row (an odd last one
// alone), then Back and Close. Phase 24 gave the Automatic family its seventh event and added the Member requirements
// family's four; Phase 25 added the Lockdown family's five.
var settingsMsgFamilyWidths = map[string]string{"m": "[2 2 2 1 2]", "a": "[2 2 2 1 2]", "r": "[2 2 2]", "l": "[2 2 1 2]"}

// settingsMsgScreenWidths are the row widths of an event screen or a private copy screen of def in the state st: one
// row per part of the definition ([Set] while the part is not set, [Set | Remove] while it is), the preview buttons two
// to a row (Preview of each part that is set, then Full preview), the switch row when the screen has one (every private
// copy screen, and every event screen but a NoSilent event's), the private copy row when the event has one, then Back
// and Close. With the three parts of the manual and Automatic families these are the Phase 21 and 22 shapes ([1 1 1 1 1
// 1 2] empty and [2 2 2 2 2 1 1 2] full for an event screen); the Member requirements family has two parts (D-29).
func settingsMsgScreenWidths(def settingsEditPageDef, st settingsEditState, switchRow, pmRow bool) string {
	var widths []int
	previews := 1
	for _, part := range def.Parts {
		if settingsEditPartIsSet(st, part.Code) {
			widths = append(widths, 2)
			previews++
			continue
		}
		widths = append(widths, 1)
	}
	for ; previews > 1; previews -= 2 {
		widths = append(widths, 2)
	}
	if previews == 1 {
		widths = append(widths, 1)
	}
	if switchRow {
		widths = append(widths, 1)
	}
	if pmRow {
		widths = append(widths, 1)
	}
	return fmt.Sprint(append(widths, 2))
}

// settingsMsgNamedScreen is one rendered screen of the page with the row widths its keyboard must have ("" for a screen
// whose shape a test does not fix, such as the plain reply of a refusal, which always has its one Cancel).
type settingsMsgNamedScreen struct {
	screen settingsScreen
	widths string
}

// settingsMsgScreens renders every screen of the Messages page for view through the functions the code draws and sends
// them with: the hub (every event Custom, every event Silent, nothing stored), the family list of every family, every
// event screen and every private copy screen with nothing stored and with every part stored and Silent on (a photo, and a
// sticker, which adds the no-caption line), each under every notice given, the three waiting prompts of each (replaced, at the largest limit the prompt
// can name), the refusal reply of every editor refusal reason and, in a group view, the group note with and without a bot
// username. The keys are descriptive and unique. The page exists only in a private menu, so a group view's screens other
// than the note are what no press can reach; they are drawn all the same, because the privacy sweep searches them too.
func settingsMsgScreens(t *testing.T, view settingsView, notices []string) map[string]settingsMsgNamedScreen {
	t.Helper()
	tr := view.Tr
	screens := map[string]settingsMsgNamedScreen{}
	add := func(name string, screen settingsScreen, widths string) {
		screens[name] = settingsMsgNamedScreen{screen: screen, widths: widths}
	}
	stored := map[string]*eventmsg.Messages{
		"all custom": settingsMsgAllCustom(t, false), "all silent": settingsMsgAllCustom(t, true),
		"nothing": {ChatID: -1001234567890, Rows: map[string]eventmsg.Message{}},
	}
	// Every option is on with every event custom, so the hub's blocks are drawn with their longest state words.
	stored["all custom"].Options = eventmsg.Options{PMDetails: true, PMLog: true, PlainTone: true}
	// Every event custom with its private copy on as well: every event line carries the private copy mark (Phase 22).
	stored["every private copy on"] = settingsMsgEveryPMOn(t)
	for i, notice := range notices {
		for name, msgs := range stored {
			add(fmt.Sprintf("hub %s notice %d", name, i), renderSettingsMsgHub(view, msgs, notice), "[2 2 1 1 1 2]")
			for _, fam := range settingsMsgFamilies {
				if len(fam.Events) == 0 {
					continue
				}
				widths, ok := settingsMsgFamilyWidths[fam.Code]
				if !ok {
					t.Fatalf("no expected list widths for the %s family", fam.Code)
				}
				add(fmt.Sprintf("family %s %s notice %d", fam.Code, name, i), renderSettingsMsgFamily(view, fam, msgs, notice), widths)
			}
		}
	}
	for _, fam := range settingsMsgFamilies {
		for _, ev := range fam.Events {
			def, ok := settingsEditPageByKey(settingsMsgWaitPage(ev))
			if !ok {
				t.Fatalf("no editor definition for the %s event", ev.Key)
			}
			pmDef, ok := settingsEditPageByKey(settingsMsgPMWaitPage(ev))
			if !ok {
				t.Fatalf("no private copy definition for the %s event", ev.Key)
			}
			// Every event screen of the manual, Automatic and Member requirements families has its private copy row (Phase 22,
			// D-22), and no lockdown event screen has one (Phase 25, D-04); a lockdown event's private copy screens are
			// reached by no press (settingsMsgDefByFields refuses d=p without a spec), so they are not drawn.
			hasPM := settingsMsgWantFamilyPM(t, fam, ev)
			emptyEvent := settingsMsgScreenWidths(def, settingsMsgEmptyState(), !ev.NoSilent, hasPM)
			fullEvent := settingsMsgScreenWidths(def, settingsMsgFullState(db.PHOTO), !ev.NoSilent, hasPM)
			emptyPM := settingsMsgScreenWidths(pmDef, settingsMsgPMOffState(), true, false)
			fullPM := settingsMsgScreenWidths(pmDef, settingsMsgFullState(db.PHOTO), true, false)
			for i, notice := range notices {
				opts := settingsEditScreenOpts{Notice: notice, BotUsername: "XenobotTestBot"}
				add(fmt.Sprintf("event %s empty notice %d", ev.Code, i), renderSettingsMsgEvent(view, fam, ev, def, settingsMsgEmptyState(), opts), emptyEvent)
				if hasPM {
					add(fmt.Sprintf("pm %s empty notice %d", ev.Code, i), renderSettingsMsgPM(view, fam, ev, pmDef, settingsMsgPMOffState(), opts), emptyPM)
				}
				for kind, mediaType := range map[string]int{"photo": db.PHOTO, "sticker": db.STICKER} {
					add(fmt.Sprintf("event %s full %s notice %d", ev.Code, kind, i),
						renderSettingsMsgEvent(view, fam, ev, def, settingsMsgFullState(mediaType), opts), fullEvent)
					if hasPM {
						add(fmt.Sprintf("pm %s full %s notice %d", ev.Code, kind, i),
							renderSettingsMsgPM(view, fam, ev, pmDef, settingsMsgFullState(mediaType), opts), fullPM)
					}
				}
				// Phase 22: the event screen whose private copy line reads On, and the private copy screen with every part
				// stored and its switch on (Silent false), the state a copy that is sent is drawn in. A lockdown event's
				// screen is drawn with PMOn too and must still have no private copy row.
				pmOn := settingsMsgFullState(db.PHOTO)
				pmOn.PMOn = true
				add(fmt.Sprintf("event %s full pm on notice %d", ev.Code, i), renderSettingsMsgEvent(view, fam, ev, def, pmOn, opts), fullEvent)
				if hasPM {
					pmSent := settingsMsgFullState(db.PHOTO)
					pmSent.Silent = false
					add(fmt.Sprintf("pm %s full switch on notice %d", ev.Code, i), renderSettingsMsgPM(view, fam, ev, pmDef, pmSent, opts), fullPM)
				}
			}
			defs := map[string]settingsEditPageDef{"": def}
			if hasPM {
				defs["pm "] = pmDef
			}
			for name, d := range defs {
				for _, part := range d.Parts {
					add(fmt.Sprintf("%sprompt %s %s", name, ev.Code, part.Code),
						renderSettingsEditPrompt(view, d, part.Code, time.Now(), settingsEditTextMax, true), "[1]")
					for key, reason := range settingsEditRefusalReasons(tr) {
						add(fmt.Sprintf("%srefusal %s %s %s", name, ev.Code, part.Code, key), renderSettingsInputRefusal(view, d, part.Code, reason), "[1]")
					}
				}
			}
		}
	}
	// The note is what a menu opened in the group draws in place of the page (its Open in private row is a group menu's).
	if !view.Private {
		add("group note", renderSettingsPrivateOnlyNoteWith(view, "settings_page_msgs", "settings_msg_group_note", "XenobotTestBot"), "[1 2]")
		add("group note without a bot username", renderSettingsPrivateOnlyNoteWith(view, "settings_page_msgs", "settings_msg_group_note", ""), "[2]")
	}
	return screens
}

// settingsMsgEditorCodes are the editor's action codes, the only buttons of a private copy screen, prompt or refusal that
// carry the destination d.
var settingsMsgEditorCodes = map[string]bool{
	settingsActMsgSet: true, settingsActMsgRemove: true, settingsActMsgCancel: true, settingsActMsgPreview: true, settingsActMsgFull: true,
}

// settingsMsgCheckDest holds the field d of one button of the screen name (Phase 22, D-26): its only value is p, it is on
// the event screen's private copy row (he) and on the editor buttons of a private copy's screens (whose keys start "pm "),
// and every such editor button carries it, so no Phase 21 button gains a byte and no private copy button edits the group
// message.
func settingsMsgCheckDest(t *testing.T, name, label string, fields map[string]string) {
	t.Helper()
	d, hasD := fields[settingsMsgDestField]
	pmScreen := strings.HasPrefix(name, "pm ")
	editor := settingsMsgEditorCodes[fields["a"]]
	switch {
	case hasD && d != settingsMsgDestPMCode:
		t.Errorf("%s: button %q carries d=%q, want only p", name, label, d)
	case hasD && pmScreen && !editor:
		t.Errorf("%s: button %q (%s) carries d, which only the editor buttons of a private copy screen carry", name, label, fields["a"])
	case hasD && !pmScreen && (!strings.HasPrefix(name, "event ") || fields["a"] != settingsActMsgEvent):
		t.Errorf("%s: button %q (%s) carries d outside the private copy row and the private copy screens", name, label, fields["a"])
	case !hasD && pmScreen && editor:
		t.Errorf("%s: the editor button %q (%s) of a private copy screen has no d, so it would edit the group message", name, label, fields["a"])
	}
}

func TestSettingsMessagesCallbackBytes(t *testing.T) {
	withRealLocales(t)
	tr := i18n.MustNewTranslator("en")
	views := settingsBlacklistWorstViews(tr)
	private := views["private"]

	// sizes records the largest button of each action code over every private screen; Back to Main and Close have codes too.
	sizes := make(map[string]int)
	pmRows := make(map[string]int)
	screensOf := settingsMsgScreens(t, private, []string{""})
	for name, named := range screensOf {
		screen := named.screen
		if screen.Omitted != 0 {
			t.Errorf("%s: %d buttons were omitted for their size, want none", name, screen.Omitted)
		}
		for _, button := range settingsAllButtons(screen.Keyboard.InlineKeyboard) {
			if button.CallbackData == "" {
				continue
			}
			if size := len(button.CallbackData); size > 64 {
				t.Errorf("%s: button %q carries %d bytes, over the 64-byte cap", name, button.Text, size)
			}
			fields := settingsButtonFields(t, button)
			for key := range fields {
				if !settingsMsgScreenFields[key] {
					t.Errorf("%s: button %q carries the field %q, outside a c u p t k d v", name, button.Text, key)
				}
			}
			settingsMsgCheckDest(t, name, button.Text, fields)
			if fields[settingsMsgDestField] != "" && strings.HasPrefix(name, "event ") {
				pmRows[name]++
			}
			sizes[fields["a"]] = max(sizes[fields["a"]], len(button.CallbackData))
		}
	}
	// Every event screen of the manual, Automatic and Member requirements families has exactly one button carrying d:
	// its private copy row; a lockdown event screen has none (Phase 25, D-04).
	for name := range screensOf {
		if !strings.HasPrefix(name, "event ") {
			continue
		}
		fam, ev, ok := settingsMsgEventByCode(strings.Fields(name)[1])
		if !ok {
			t.Fatalf("%s names no event of the page", name)
		}
		want := 0
		if settingsMsgWantFamilyPM(t, fam, ev) {
			want = 1
		}
		if pmRows[name] != want {
			t.Errorf("%s has %d buttons carrying d, want %d", name, pmRows[name], want)
		}
	}
	for code, want := range map[string]int{
		settingsActMsgFamily: 37, settingsActMsgEvent: 42,
		settingsActMsgSet: 46, settingsActMsgRemove: 46, settingsActMsgPreview: 46, settingsActMsgCancel: 46, settingsActMsgSilent: 42,
		settingsActMsgFull: 42, settingsActMsgPM: 42, settingsActMsgPMOption: 41, settingsActMsgPlainTone: 37, settingsActOpen: 37, settingsActBack: 33, settingsActClose: 33,
	} {
		if got := sizes[code]; got != want {
			t.Errorf("the largest %s button is %d bytes, the plans fixed %d", code, got, want)
		}
	}

	// Main's cell of the page in both menus, and the group note's buttons.
	cell := func(v settingsView) int {
		screen := renderSettingsMain(v, []settingsMainEntry{{Key: settingsPageMessages, Icon: "📨", Title: "Messages", State: "7", Known: true}}, "")
		for _, button := range settingsAllButtons(screen.Keyboard.InlineKeyboard) {
			if f := settingsButtonFields(t, button); f["a"] == settingsActOpen && f["p"] == settingsPageMessages {
				return len(button.CallbackData)
			}
		}
		return 0
	}
	if got := cell(private); got != 37 {
		t.Errorf("the Main cell is %d bytes in a private menu, want 37", got)
	}
	if got := cell(views["group"]); got != 33 {
		t.Errorf("the Main cell is %d bytes in a group menu, want 33", got)
	}
	note := renderSettingsPrivateOnlyNoteWith(views["group"], "settings_page_msgs", "settings_msg_group_note", "XenobotTestBot")
	if note.Omitted != 0 {
		t.Errorf("the group note omitted %d buttons", note.Omitted)
	}
	for _, button := range settingsAllButtons(note.Keyboard.InlineKeyboard) {
		if button.CallbackData != "" && len(button.CallbackData) != 29 {
			t.Errorf("group note button %q is %d bytes, want 29", button.Text, len(button.CallbackData))
		}
	}
}

func TestSettingsMessagesPageFitsEveryLocale(t *testing.T) {
	withRealLocales(t)
	title := strings.Repeat("Ж", 64)
	for _, code := range memberReqScreenLanguages {
		t.Run(code, func(t *testing.T) {
			tr := i18n.MustNewTranslator(code)
			views := settingsBlacklistWorstViews(tr)
			// A notice adds only its own length, so the two longest of the notices the page's screens can carry stand for
			// every combination: a failed save, the saved notice of each part with the caption and album notes, the
			// removed notices and the editor's other page notices.
			candidates := []string{
				settingsNoticeWarn(settingsText(tr, "common_settings_save_failed")),
				settingsNoticeWarn(settingsText(tr, "settings_edit_nothing_to_remove")),
				settingsNoticeWarn(settingsText(tr, "settings_edit_group_note")),
				settingsNoticeWarn(settingsText(tr, "settings_edit_cancelled")),
				settingsNoticeWarn(settingsText(tr, "settings_edit_removed_text_default")),
				settingsNoticeWarn(settingsText(tr, "settings_edit_removed_media")),
				settingsNoticeWarn(settingsText(tr, "settings_edit_removed_buttons")),
			}
			for _, part := range []string{settingsEditPartText, settingsEditPartMedia, settingsEditPartButtons} {
				candidates = append(candidates, settingsInputSavedNotice(tr, part, true, true))
			}
			notices := append([]string{""}, settingsBlacklistLongest(candidates, 2)...)

			for viewName, view := range views {
				view.GroupTitle, view.OpenerName = title, title
				screens := settingsMsgScreens(t, view, notices)
				for name, named := range screens {
					label := fmt.Sprintf("%s %s", viewName, name)
					screen := named.screen
					if units := panelUTF16Len(screen.Text); units == 0 || units > 3800 {
						t.Errorf("%s is %d UTF-16 units, want 1..3800", label, units)
					}
					for _, placeholder := range []string{"{n}", "{max}", "{time}", "{part}", "{state}", "{page}", "{icon}", "%!"} {
						if strings.Contains(screen.Text, placeholder) {
							t.Errorf("%s keeps the unfilled %s", label, placeholder)
						}
					}
					if screen.Omitted != 0 {
						t.Errorf("%s omitted %d buttons", label, screen.Omitted)
					}
					if got := fmt.Sprint(settingsListRowWidths(screen.Keyboard.InlineKeyboard)); got != named.widths {
						t.Errorf("%s has row widths %s, want %s", label, got, named.widths)
					}
					// A half-width cell fits 20 code points and a full-width button 40; the Open in private URL row is
					// a URL button of its own.
					for _, row := range screen.Keyboard.InlineKeyboard {
						limit := 20
						if len(row) == 1 {
							limit = 40
						}
						for _, button := range row {
							if strings.TrimSpace(button.Text) == "" {
								t.Errorf("%s has a button with an empty label", label)
							}
							if n := len([]rune(button.Text)); n > limit && button.Url == "" {
								t.Errorf("%s: button %q has %d code points, want at most %d", label, button.Text, n, limit)
							}
						}
					}
				}
			}
		})
	}
}
