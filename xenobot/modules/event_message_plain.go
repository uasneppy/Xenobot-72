package modules

import (
	"fmt"
	"html"
	"maps"
	"strconv"
	"strings"

	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/formatting"
)

// Pure plain-tone default builders (Phase 29, TUNE-08).
//
// This file holds only pure string builders and NEVER references any event engine entry
// (TestEventEngineCallers, TestPMCopyCallers).

// eventPlainRender translates key with params, splicing the already-escaped user/admin HTML
// mention after interpolation so a display name containing "{time}" or "{count}" can never be
// re-interpolated.
func eventPlainRender(tr *i18n.Translator, key string, params i18n.TranslationParams, mentionKey, mention string) string {
	if mentionKey != "" {
		if params == nil {
			params = i18n.TranslationParams{}
		} else {
			params = maps.Clone(params)
		}
		params[mentionKey] = lockdownNameToken
	}
	text, _ := tr.GetString(key, params)
	if mentionKey != "" {
		return lockdownSplice(text, lockdownNameToken, mention)
	}
	return text
}

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
	return eventPlainRender(tr, "eventmsg_plain_ban", nil, "user", user) + eventPlainReasonLine(tr, in.Reason)
}

// tbanPlainDefaultReply builds the neutral plain-tone group default reply for a timed ban.
func tbanPlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	user := formatting.MentionHtml(in.Target.Id, in.Target.FirstName)
	return eventPlainRender(tr, "eventmsg_plain_tban", i18n.TranslationParams{
		"time": html.EscapeString(in.TimeVal),
	}, "user", user) + eventPlainReasonLine(tr, in.Reason)
}

// kickPlainDefaultReply builds the neutral plain-tone group default reply for a kick.
func kickPlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	user := formatting.MentionHtml(in.Target.Id, in.Target.FirstName)
	return eventPlainRender(tr, "eventmsg_plain_kick", nil, "user", user) + eventPlainReasonLine(tr, in.Reason)
}

// mutePlainDefaultReply builds the neutral plain-tone group default reply for an untimed mute.
func mutePlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	user := formatting.MentionHtml(in.Target.Id, in.Target.FirstName)
	return eventPlainRender(tr, "eventmsg_plain_mute", nil, "user", user) + eventPlainReasonLine(tr, in.Reason)
}

// tmutePlainDefaultReply builds the neutral plain-tone group default reply for a timed mute.
func tmutePlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	user := formatting.MentionHtml(in.Target.Id, in.Target.FirstName)
	return eventPlainRender(tr, "eventmsg_plain_tmute", i18n.TranslationParams{
		"time": html.EscapeString(in.TimeVal),
	}, "user", user) + eventPlainReasonLine(tr, in.Reason)
}

// unbanPlainDefaultReply builds the neutral plain-tone group default reply for an unban.
func unbanPlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	user := formatting.MentionHtml(in.Target.Id, in.Target.FirstName)
	return eventPlainRender(tr, "eventmsg_plain_unban", nil, "user", user)
}

// unmutePlainDefaultReply builds the neutral plain-tone group default reply for an unmute
// (without the optional lockdown note, which the unmute send site appends when active).
func unmutePlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	user := formatting.MentionHtml(in.Target.Id, in.Target.FirstName)
	return eventPlainRender(tr, "eventmsg_plain_unmute", nil, "user", user)
}

// warnPlainDefaultReply builds the neutral plain-tone group default reply for a warning.
func warnPlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	user := formatting.MentionHtml(in.Target.Id, in.Target.FirstName)
	return eventPlainRender(tr, "eventmsg_plain_warn", i18n.TranslationParams{
		"count": in.Count,
		"limit": in.Limit,
	}, "user", user) + eventPlainReasonLine(tr, in.Reason)
}

var warnLimitPlainKeys = map[string]string{
	"kick": "eventmsg_plain_warnlimit_kick",
	"mute": "eventmsg_plain_warnlimit_mute",
	"ban":  "eventmsg_plain_warnlimit_ban",
}

var warnLimitPlainTimedKeys = map[string]string{
	"tmute": "eventmsg_plain_warnlimit_tmute",
	"tban":  "eventmsg_plain_warnlimit_tban",
}

// warnLimitPlainDefaultReply builds the neutral plain-tone group default reply when a member
// reaches the warning limit, preserving escaped stored reasons as bullet lines.
func warnLimitPlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	user := formatting.MentionHtml(in.Target.Id, in.Target.FirstName)
	var text string
	if key, ok := warnLimitPlainTimedKeys[in.Action]; ok && in.Secs > 0 {
		text = eventPlainRender(tr, key, i18n.TranslationParams{
			"count": in.Count,
			"limit": in.Limit,
			"time":  punishDurationLabel(tr, in.Secs),
		}, "user", user)
	} else if key, ok := warnLimitPlainKeys[in.Action]; ok {
		text = eventPlainRender(tr, key, i18n.TranslationParams{
			"count": in.Count,
			"limit": in.Limit,
		}, "user", user)
	}
	var sb strings.Builder
	for _, reason := range in.Reasons {
		fmt.Fprintf(&sb, "\n - %s", html.EscapeString(reason))
	}
	return text + sb.String()
}

var floodPlainTimedKeys = map[string]string{
	"tmute": "eventmsg_plain_flood_tmute",
	"tban":  "eventmsg_plain_flood_tban",
}

// floodPlainDefaultReply builds the neutral plain-tone group default reply for an antiflood action.
func floodPlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	name := formatting.GetFullName(in.Target.FirstName, in.Target.LastName)
	user := formatting.MentionHtml(in.Target.Id, name)
	if key, ok := floodPlainTimedKeys[in.Action]; ok && in.Secs > 0 {
		return eventPlainRender(tr, key, i18n.TranslationParams{
			"time": punishDurationLabel(tr, in.Secs),
		}, "user", user)
	}
	return eventPlainRender(tr, "eventmsg_plain_flood", i18n.TranslationParams{
		"action": eventActionWord(tr, in.Action),
	}, "user", user)
}

// blacklistPlainDefaultReply builds the neutral plain-tone group default reply shared by the
// blacklist and forward blocklist events. For fwdblock, in.Reason is staff_fwdblock_reason
// (never a blocked source or Staff Group).
func blacklistPlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	name := formatting.GetFullName(in.Target.FirstName, in.Target.LastName)
	user := formatting.MentionHtml(in.Target.Id, name)
	if _, ok := blacklistNoticeTimedKeys[in.Action]; ok && in.Secs > 0 {
		return eventPlainRender(tr, "eventmsg_plain_blacklist_timed", i18n.TranslationParams{
			"action": eventActionWord(tr, in.Action),
			"time":   punishDurationLabel(tr, in.Secs),
		}, "user", user) + eventPlainReasonLine(tr, in.Reason)
	}
	if _, ok := blacklistNoticeKeys[in.Action]; !ok {
		return ""
	}
	return eventPlainRender(tr, "eventmsg_plain_blacklist", i18n.TranslationParams{
		"action": eventActionWord(tr, in.Action),
	}, "user", user) + eventPlainReasonLine(tr, in.Reason)
}

// captchaFailurePlainDefaultReply builds the neutral plain-tone group default reply for a failed
// captcha verification.
func captchaFailurePlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	name := in.Target.FirstName
	if name == "" {
		name = "User"
	}
	user := formatting.MentionHtml(in.Target.Id, name)
	action := "kick"
	switch in.Action {
	case "ban", "mute":
		action = in.Action
	}
	actionWord := eventActionWord(tr, action)
	if in.Count > 0 {
		return eventPlainRender(tr, "eventmsg_plain_captcha_with_messages", i18n.TranslationParams{
			"action": actionWord,
			"count":  in.Count,
		}, "user", user)
	}
	return eventPlainRender(tr, "eventmsg_plain_captcha", i18n.TranslationParams{
		"action": actionWord,
	}, "user", user)
}

// aispamPlainDefaultReply builds the neutral plain-tone group default reply for an AI spam removal.
// It names only the target user — never the provider, model, probability, or category.
func aispamPlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	name := formatting.GetFullName(in.Target.FirstName, in.Target.LastName)
	user := formatting.MentionHtml(in.Target.Id, name)
	return eventPlainRender(tr, "eventmsg_plain_aispam", nil, "user", user)
}

var memberReqPlainKeys = map[string]string{
	"mreq_notice_warn":            "eventmsg_plain_mreq_warn",
	"mreq_notice_warn_limit_kick": "eventmsg_plain_mreq_warn_limit_kick",
	"mreq_notice_warn_limit_ban":  "eventmsg_plain_mreq_warn_limit_ban",
	"mreq_notice_warn_limit_mute": "eventmsg_plain_mreq_warn_limit_mute",
	"mreq_notice_kick":            "eventmsg_plain_mreq_kick",
	"mreq_notice_ban":             "eventmsg_plain_mreq_ban",
	"mreq_notice_mute":            "eventmsg_plain_mreq_mute",
}

// memberReqPlainNoticeDefault builds the neutral plain-tone group default reply for a member
// requirement notice, reusing memberReqRulesText, memberReqNoticeRemedies, and memberReqMention
// so broken rules, remedies, and HTML escaping match the themed notice.
func memberReqPlainNoticeDefault(action int) func(tr *i18n.Translator, in eventDefaultInput) string {
	return func(tr *i18n.Translator, in eventDefaultInput) string {
		warn := memberReqNoticeWarnOf(action, in)
		themedKey := memberReqNoticeKey(action, warn)
		plainKey, ok := memberReqPlainKeys[themedKey]
		if !ok {
			plainKey = "eventmsg_plain_mreq_warn"
		}
		params := i18n.TranslationParams{
			"user":  memberReqNamePlaceholder,
			"rules": memberReqRulesText(tr, in.Rules),
		}
		if warn != nil {
			params["count"] = strconv.Itoa(warn.Count)
			params["limit"] = strconv.Itoa(warn.Limit)
		}
		text := memberReqText(tr, plainKey, params)
		for _, remedy := range memberReqNoticeRemedies(themedKey, in.Rules) {
			text += " " + memberReqText(tr, remedy)
		}
		fallback := memberReqText(tr, "mreq_name_unknown", i18n.TranslationParams{"id": strconv.FormatInt(in.Target.Id, 10)})
		return lockdownSplice(text, memberReqNamePlaceholder, memberReqMention(in.Target, fallback))
	}
}

// lockdownStartPlainDefaultReply builds the neutral plain-tone group default reply for a manual
// /lockdown start, preserving the reason line and all technical start note lines.
func lockdownStartPlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	notes := lockdownStartNoteLines(tr, in.Lockdown)
	var intro string
	if in.Target.Id != 0 || strings.TrimSpace(in.Target.FirstName) != "" {
		intro = eventPlainRender(tr, "eventmsg_plain_lockdown_by", nil, "admin", lockdownMention(in.Target.Id, in.Target.FirstName))
	} else {
		intro = lockdownText(tr, "eventmsg_plain_lockdown")
	}
	parts := []string{intro, notes[0]}
	if line := strings.TrimPrefix(eventPlainReasonLine(tr, in.Reason), "\n"); line != "" {
		parts = append(parts, line)
	}
	parts = append(parts, notes[1:]...)
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

var lockdownAutoPlainKeys = map[string]string{
	models.LockdownTriggerJoinSurge: "eventmsg_plain_lockjoin",
	models.LockdownTriggerFlood:     "eventmsg_plain_lockflood",
	models.LockdownTriggerAIBurst:   "eventmsg_plain_lockai",
}

// lockdownAutoPlainDefaultReply builds the neutral plain-tone group default reply for an automatic
// lockdown trigger (join surge, flood, or AI burst), preserving all technical start note lines.
func lockdownAutoPlainDefaultReply(kind string) func(tr *i18n.Translator, in eventDefaultInput) string {
	return func(tr *i18n.Translator, in eventDefaultInput) string {
		key, ok := lockdownAutoPlainKeys[kind]
		if !ok {
			return ""
		}
		params := i18n.TranslationParams{
			"count":  strconv.Itoa(in.Count),
			"window": strconv.Itoa(in.Window),
		}
		if kind == models.LockdownTriggerFlood {
			params["percent"] = strconv.Itoa(in.Percent)
		}
		lines := append([]string{lockdownText(tr, key, params)}, lockdownStartNoteLines(tr, in.Lockdown)...)
		return lockdownJoinLines(lines)
	}
}

// lockdownLiftPlainDefaultReply builds the neutral plain-tone group default reply for lifting a
// lockdown, preserving all technical lift note lines.
func lockdownLiftPlainDefaultReply(tr *i18n.Translator, in eventDefaultInput) string {
	name := lockdownCapRunes(staffFullName(&in.Target), lockdownNameMaxRunes)
	var intro string
	if in.Target.Id != 0 || name != "" {
		intro = eventPlainRender(tr, "eventmsg_plain_locklift_by", nil, "admin", lockdownMention(in.Target.Id, name))
	} else {
		intro = lockdownText(tr, "eventmsg_plain_locklift")
	}
	lines := append([]string{intro}, lockdownLiftNoteLines(tr, in.Lockdown)...)
	return lockdownJoinLines(lines)
}
