//go:build testtools

package modules

import (
	"testing"

	"github.com/uasneppy/fuku_robot/fuku/i18n"
)

// The private copies of the Automatic family (Phase 23 plan 23-05): the warn and the warn limit, whose
// specs are manual but whose rule-raised replies count against the per-group cap (eventPMOffer.ByRule),
// and the flood, blacklist and forward blocklist punishments, whose specs are automatic (D-28).

// newEventPMAutoSite is a notice golden site (a fresh supergroup whose client answers the member 42's
// getChatMember and getChat, the real locales, a 600 s rule of kind) with miniredis, a fast private copy
// pacer and a client that answers the member's private chat. The chat's event messages and the private
// copy keys are removed when the test ends, after the jobs.
func newEventPMAutoSite(t *testing.T, kind string) *botSite {
	t.Helper()
	withMiniredisServer(t)
	withFastPMCopyPacer(t)
	s := newNoticeGoldenSite(t, kind)
	s.bot.BotClient = eventPMTestClient{BotClient: s.bot.BotClient, script: &eventPMScript{}}
	eventPMCleanup(t, s.chat.Id)
	t.Cleanup(pmCopyJobs.wait)
	return s
}

// eventPMAutoDefault is an event's default private text as the group's language (uk) words it with params,
// followed by the reason line when reason is set.
func eventPMAutoDefault(t *testing.T, key string, params i18n.TranslationParams, reason string) string {
	t.Helper()
	tr := i18n.MustNewTranslator("uk")
	text, err := tr.GetString(key, params)
	if err != nil || text == "" {
		t.Fatalf("translate %s: %q, %v", key, text, err)
	}
	if reason != "" {
		line, _ := tr.GetString("eventmsg_pm_reason", i18n.TranslationParams{"reason": reason})
		text += "\n" + line
	}
	return text
}
