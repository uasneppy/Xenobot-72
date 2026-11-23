package modules

import (
	"html"

	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/formatting"
)

// Pure plain-tone default builders (Phase 29, TUNE-08).
//
// This file holds only pure string builders and NEVER references any event engine entry
// (TestEventEngineCallers, TestPMCopyCallers).

// eventPlainReasonLine returns the neutral reason line appended to plain-tone group defaults when
// a reason is present, or empty when reason is empty. It reuses eventmsg_pm_reason
// ("<b>Reason:</b> {reason}").
func eventPlainReasonLine(tr *i18n.Translator, reason string) string {
	if reason == "" {
		return ""
	}
	line, _ := tr.GetString("eventmsg_pm_reason", i18n.TranslationParams{"reason": html.EscapeString(reason)})
	if line == "" {
		return ""
	}
	return "\n" + line
}

// banPlainDefaultReply builds the neutral plain-tone group default reply for an untimed ban
// (TUNE-08): no emoji, no first-person phrasing, optional reason line.
func banPlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	user := formatting.MentionHtml(in.Target.Id, in.Target.FirstName)
	text, _ := tr.GetString("eventmsg_plain_ban", i18n.TranslationParams{"user": user})
	return text + eventPlainReasonLine(tr, in.Reason)
}
