//go:build testtools

package i18n

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

const eventMsgKeyPrefix = "eventmsg_"

// eventMsgLimit is the length ceiling of one eventmsg_ template in code points as written
// (placeholders and markup included): English against max, every other language against other.
type eventMsgLimit struct {
	max   int
	other int
}

// eventMsgLocaleLimits has one row per eventmsg_ key of the locale files. The test fails when it does
// not cover exactly the eventmsg_ keys of every locale file, so a plan that adds a key adds its row.
var eventMsgLocaleLimits = map[string]eventMsgLimit{
	"eventmsg_admin_anon":         {20, 30},
	"eventmsg_pm_ban":             {140, 200},
	"eventmsg_pm_tban":            {140, 200},
	"eventmsg_pm_kick":            {140, 200},
	"eventmsg_pm_mute":            {140, 200},
	"eventmsg_pm_tmute":           {140, 200},
	"eventmsg_pm_unban":           {140, 200},
	"eventmsg_pm_unmute":          {140, 200},
	"eventmsg_pm_reason":          {40, 60},
	"eventmsg_pm_until":           {40, 60},
	"eventmsg_pm_unmute_lockdown": {140, 200},
	"eventmsg_pm_warn":            {140, 200},
	"eventmsg_pm_warnlimit":       {140, 200},
	"eventmsg_pm_flood":           {140, 200},
	"eventmsg_pm_blacklist":       {140, 200},
	"eventmsg_pm_fwdblock":        {140, 200},
	"eventmsg_pm_captcha":         {140, 200},
	"eventmsg_pm_aispam":          {140, 200},
	"eventmsg_pm_mreqwarn":        {140, 200},
	"eventmsg_pm_mreqkick":        {140, 200},
	"eventmsg_pm_mreqban":         {140, 200},
	"eventmsg_pm_mreqmute":        {140, 200},
	"eventmsg_btn_details":        {20, 28},
	"eventmsg_btn_appeal":         {20, 28},
	"eventmsg_pm_details_none":    {60, 90},
	"eventmsg_pm_log":             {140, 200},
	"eventmsg_pm_why_blocked":     {70, 100},
	"eventmsg_pm_why_failed":      {70, 100},
	"eventmsg_action_warn":        {20, 30},
	"eventmsg_action_mute":        {20, 30},
	"eventmsg_action_kick":        {20, 30},
	"eventmsg_action_ban":         {20, 30},
	"eventmsg_reason_flood":       {60, 90},
	"eventmsg_action_delete":      {20, 30},
	"eventmsg_reason_captcha":     {60, 90},
	"eventmsg_reason_aispam":      {60, 90},
	"eventmsg_mreq_rules":         {60, 90},
	// Plain tone defaults (Phase 29, TUNE-08).
	"eventmsg_plain_ban":                   {120, 180},
	"eventmsg_plain_tban":                  {120, 180},
	"eventmsg_plain_kick":                  {120, 180},
	"eventmsg_plain_mute":                  {120, 180},
	"eventmsg_plain_tmute":                 {120, 180},
	"eventmsg_plain_unban":                 {120, 180},
	"eventmsg_plain_unmute":                {120, 180},
	"eventmsg_plain_warn":                  {120, 180},
	"eventmsg_plain_warnlimit_kick":        {140, 200},
	"eventmsg_plain_warnlimit_mute":        {140, 200},
	"eventmsg_plain_warnlimit_ban":         {140, 200},
	"eventmsg_plain_warnlimit_tmute":       {140, 200},
	"eventmsg_plain_warnlimit_tban":        {140, 200},
	"eventmsg_plain_flood":                 {140, 200},
	"eventmsg_plain_flood_tmute":           {140, 200},
	"eventmsg_plain_flood_tban":            {140, 200},
	"eventmsg_plain_blacklist":             {140, 200},
	"eventmsg_plain_blacklist_timed":       {140, 200},
	"eventmsg_plain_captcha":               {140, 200},
	"eventmsg_plain_captcha_with_messages": {160, 220},
	"eventmsg_plain_aispam":                {140, 200},
	"eventmsg_plain_mreq_warn":             {140, 200},
	"eventmsg_plain_mreq_warn_limit_kick":  {140, 200},
	"eventmsg_plain_mreq_warn_limit_ban":   {140, 200},
	"eventmsg_plain_mreq_warn_limit_mute":  {140, 200},
	"eventmsg_plain_mreq_kick":             {140, 200},
	"eventmsg_plain_mreq_ban":              {140, 200},
	"eventmsg_plain_mreq_mute":             {140, 200},
	"eventmsg_plain_lockdown":              {160, 220},
	"eventmsg_plain_lockdown_by":           {160, 220},
	"eventmsg_plain_lockjoin":              {180, 240},
	"eventmsg_plain_lockflood":             {180, 240},
	"eventmsg_plain_lockai":                {180, 240},
	"eventmsg_plain_locklift":              {140, 200},
	"eventmsg_plain_locklift_by":           {140, 200},
	"eventmsg_plain_pm_ban":                {140, 200},
	"eventmsg_plain_pm_tban":               {140, 200},
	"eventmsg_plain_pm_kick":               {140, 200},
	"eventmsg_plain_pm_mute":               {140, 200},
	"eventmsg_plain_pm_tmute":              {140, 200},
	"eventmsg_plain_pm_unban":              {140, 200},
	"eventmsg_plain_pm_unmute":             {140, 200},
	"eventmsg_plain_pm_warn":               {140, 200},
	"eventmsg_plain_pm_warnlimit":          {140, 200},
	"eventmsg_plain_pm_flood":              {140, 200},
	"eventmsg_plain_pm_blacklist":          {140, 200},
	"eventmsg_plain_pm_fwdblock":           {140, 200},
	"eventmsg_plain_pm_captcha":            {140, 200},
	"eventmsg_plain_pm_aispam":             {140, 200},
	"eventmsg_plain_pm_mreqwarn":           {140, 200},
	"eventmsg_plain_pm_mreqkick":           {140, 200},
	"eventmsg_plain_pm_mreqban":            {140, 200},
	"eventmsg_plain_pm_mreqmute":           {140, 200},
}

// eventMsgNotDefaultText are the eventmsg_pm_ keys that are not an event's default private text, so they do
// not name the group: the reason line, the until line of a timed action, the lockdown note, the neutral Details
// reply and the log line with its two reasons (the log channel's header names the group).
var eventMsgNotDefaultText = map[string]bool{
	"eventmsg_pm_reason": true, "eventmsg_pm_until": true, "eventmsg_pm_unmute_lockdown": true, "eventmsg_pm_details_none": true,
	"eventmsg_pm_log": true, "eventmsg_pm_why_blocked": true, "eventmsg_pm_why_failed": true,
}

// eventMsgAllowedTags is the markup an eventmsg_ text may carry: <b> only, around {group} in the
// default private texts and around the label of the reason line.
var eventMsgAllowedTags = []string{"<b>", "</b>"}

func TestEventMsgLocaleKeys(t *testing.T) {
	english := loadStaffLocale(t, "en")

	for _, lang := range staffLocales {
		t.Run(lang, func(t *testing.T) {
			data := loadStaffLocale(t, lang)

			var failures []string
			for key := range data {
				if !strings.HasPrefix(key, eventMsgKeyPrefix) {
					continue
				}
				if _, ok := eventMsgLocaleLimits[key]; !ok {
					failures = append(failures, fmt.Sprintf("%s: the limits table has no row for %s", lang, key))
				}
			}

			for key, limit := range eventMsgLocaleLimits {
				text, isString := data[key].(string)
				if !isString || strings.TrimSpace(text) == "" {
					failures = append(failures, fmt.Sprintf("%s: missing, empty or non-string %s", lang, key))
					continue
				}
				ceiling := limit.other
				if lang == "en" {
					ceiling = limit.max
				}
				if runes := utf8.RuneCountInString(text); runes > ceiling {
					failures = append(failures, fmt.Sprintf("%s: %s has %d characters, ceiling %d", lang, key, runes, ceiling))
				}
				stripped := text
				for _, tag := range eventMsgAllowedTags {
					stripped = strings.ReplaceAll(stripped, tag, "")
				}
				if strings.ContainsAny(stripped, "<>&") {
					failures = append(failures, fmt.Sprintf("%s: %s holds markup other than <b> or an ampersand", lang, key))
				}
				if strings.Count(text, "<b>") != strings.Count(text, "</b>") {
					failures = append(failures, fmt.Sprintf("%s: %s has an unclosed <b>", lang, key))
				}
				if settingsVerbPattern.MatchString(text) {
					failures = append(failures, fmt.Sprintf("%s: %s holds a printf verb", lang, key))
				}
				englishText, _ := english[key].(string)
				if got, want := staffPlaceholders(text), staffPlaceholders(englishText); !slices.Equal(got, want) {
					failures = append(failures, fmt.Sprintf("%s: %s has placeholders %v, en.yml has %v", lang, key, got, want))
				}
				// The default private texts name the group; the neutral Details reply must name nothing (D-20).
				isPMDefault := (strings.HasPrefix(key, "eventmsg_pm_") && !eventMsgNotDefaultText[key]) || strings.HasPrefix(key, "eventmsg_plain_pm_")
				if isPMDefault && !strings.Contains(text, "<b>{group}</b>") {
					failures = append(failures, fmt.Sprintf("%s: %s does not name the group as <b>{group}</b>", lang, key))
				}
				if key == "eventmsg_pm_details_none" && (strings.Contains(text, "{") || strings.Contains(text, "<b>")) {
					failures = append(failures, fmt.Sprintf("%s: %s names something, want a neutral reply", lang, key))
				}
			}

			slices.Sort(failures)
			for _, failure := range failures {
				t.Error(failure)
			}
		})
	}
}
