//go:build testtools

package modules

import (
	"slices"
	"strings"
	"testing"
)

type settingsPhase33Code struct {
	code, page string
	valid      []string
}

func settingsPhase33Codes() []settingsPhase33Code {
	return []settingsPhase33Code{
		{settingsActPresenceHighlights, settingsPagePresence, nil},
		{settingsActPresenceArchiveUnlink, settingsPagePresence, nil},
		{settingsActPresenceRoom, settingsPagePresence, nil},
	}
}

// TestPhase33Codes verifies that Phase 33/34 keep len(settingsPages) == 31 and
// settingsMainOwnerRows == 17 unchanged, register ph, pu, and pc as actions on private-only
// page pr with zero collisions against core actions or other pages, and
// refuse extra unknown fields or group-menu presses (u instead of c) in step 1 with 0
// getChatMember calls.
func TestPhase33Codes(t *testing.T) {
	allPages := []string{
		"s", "v", "l", "c", "f", "q", "n", "j", "k", "rs",
		"b", "i", "w", "g", "r", "e", "x", "h", "m", "av",
		"cp", "ba", "d", "t", "u", "z", "o", "y", "pr", "p", "a",
	}
	if len(allPages) != 31 {
		t.Fatalf("expected 31 registered pages in test list, got %d", len(allPages))
	}
	if len(settingsPages) != len(allPages) {
		t.Errorf("registered settingsPages count = %d, want %d", len(settingsPages), len(allPages))
	}
	seenPages := map[string]bool{}
	for _, key := range allPages {
		if seenPages[key] {
			t.Errorf("duplicate page key %q in test list", key)
		}
		seenPages[key] = true
		if _, ok := settingsPageByKey(key); !ok {
			t.Errorf("page %q is not registered", key)
		}
	}
	if settingsMainOwnerRows != 17 {
		t.Fatalf("settingsMainOwnerRows = %d, want 17", settingsMainOwnerRows)
	}

	prPage, ok := settingsPageByKey(settingsPagePresence)
	if !ok {
		t.Fatalf("page %q is not registered", settingsPagePresence)
	}
	if !prPage.PrivateOnly {
		t.Errorf("page %q PrivateOnly = false, want true", settingsPagePresence)
	}
	if prPage.Order != 91 {
		t.Errorf("page %q Order = %d, want 91", settingsPagePresence, prPage.Order)
	}
	if prPage.Icon != "✨" {
		t.Errorf("page %q Icon = %q, want ✨", settingsPagePresence, prPage.Icon)
	}
	codes := settingsPhase33Codes()
	if len(prPage.Actions) != len(codes) {
		t.Errorf("page %q registers %d action codes, want %d", settingsPagePresence, len(prPage.Actions), len(codes))
	}

	for _, ctrl := range []string{
		settingsPresenceControlConsensus,
		settingsPresenceControlThreshold,
		settingsPresenceControlRetract,
		settingsPresenceControlAIReact,
		settingsPresenceControlCatSticker,
		settingsPresenceControlArchiveEnabled,
		settingsPresenceControlArchiveThreshold,
		settingsPresenceControlArchiveEmoji,
		settingsPresenceControlArchiveDelivery,
		settingsPresenceControlAmbient,
		settingsPresenceControlVocative,
		settingsPresenceControlPMBrief,
	} {
		if _, ok := prPage.Controls[ctrl]; !ok {
			t.Errorf("control %q missing from page %q Controls", ctrl, settingsPagePresence)
		}
	}

	for _, row := range codes {
		entry, ok := settingsActions[row.code]
		if !ok || entry.pageKey != row.page {
			t.Errorf("code %q maps to %+v (registered %v), want page %q", row.code, entry, ok, row.page)
		}
		if _, inPage := prPage.Actions[row.code]; !inPage {
			t.Errorf("code %q missing from page %q Actions", row.code, row.page)
		}
		if slices.Contains(settingsCoreActions, row.code) {
			t.Errorf("code %q collides with settingsCoreActions", row.code)
		}
		for otherKey, otherPage := range settingsPages {
			if otherKey == row.page {
				continue
			}
			if _, dup := otherPage.Actions[row.code]; dup {
				t.Errorf("code %q collides with page %q", row.code, otherKey)
			}
		}
	}

	env := newPhase28SettingsEnv(t)
	settingsPresenceCleanup(t, env.chat.Id)
	owner := env.newOwner(env.chat)
	privateMsg := settingsPrivateOnlyMessage(t, env, owner)
	env.pressPrivate(owner, privateMsg, env.privateFields(env.chat.Id, "a", settingsActBack))
	_, mainKB := env.lastEdit(owner.Id, privateMsg)
	if len(mainKB) != settingsMainOwnerRows {
		t.Errorf("owner Main rows = %d, want %d", len(mainKB), settingsMainOwnerRows)
	}

	groupMsg := env.openMenu(owner)
	lookups := func() int { return len(env.fake.callsFor("getChatMember")) }
	refused := func(name string, press func()) {
		t.Helper()
		before := lookups()
		press()
		if answer, alert := env.lastAnswer(); !alert || !strings.Contains(answer, staffMarker("common_callback_invalid_request")) {
			t.Errorf("%s answered %q (alert %v), want the invalid-request alert", name, answer, alert)
		}
		if got := lookups(); got != before {
			t.Errorf("%s made %d getChatMember calls, want 0 (refused in step 1)", name, got-before)
		}
	}

	for _, row := range codes {
		kv := append([]string{"a", row.code}, row.valid...)
		withExtra := append(slices.Clone(kv), "x", "1")
		refused(row.code+" with an extra key", func() {
			env.pressPrivate(owner, privateMsg, env.privateFields(env.chat.Id, withExtra...))
		})
		refused(row.code+" from a group menu", func() {
			env.press(owner, groupMsg, env.groupFields(owner, kv...))
		})
	}
}
