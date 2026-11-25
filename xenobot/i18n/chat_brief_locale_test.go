//go:build testtools

package i18n

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// chatBriefKeyPrefixes are the key families of the brief commands (/unrollshort, /unrolllong and
// /judge). The settings_ keys of the Message log have their rows in settingsLocaleLimits.
var chatBriefKeyPrefixes = []string{"brief_", "judge_"}

// chatBriefLimit is the length ceiling of one key in code points as written, placeholders and
// markup included: English against max, every other language against other.
type chatBriefLimit struct {
	max   int
	other int
}

// chatBriefLocaleLimits has one row per brief_ and judge_ key. The test fails when it does not
// cover exactly those keys of every locale file, so a change that adds a key adds its row.
var chatBriefLocaleLimits = map[string]chatBriefLimit{
	"brief_off_hint":     {200, 280},
	"brief_unavailable":  {80, 120},
	"brief_usage_short":  {200, 280},
	"brief_too_old":      {220, 300},
	"brief_header_short": {60, 80},
	"brief_ai_notice":    {100, 140},
	"brief_malfunction":  {80, 120},
	"brief_allowance":    {90, 130},
	"brief_refused":      {70, 100},
}

// chatBriefAllowedTags is the markup a brief_ or judge_ text may carry: bold and italic lines.
var chatBriefAllowedTags = []string{"<b>", "</b>", "<i>", "</i>", "<code>", "</code>"}

func TestChatBriefLocaleKeys(t *testing.T) {
	english := loadStaffLocale(t, "en")
	for _, lang := range staffLocales {
		t.Run(lang, func(t *testing.T) {
			data := loadStaffLocale(t, lang)
			var failures []string
			for key := range data {
				for _, prefix := range chatBriefKeyPrefixes {
					if strings.HasPrefix(key, prefix) {
						if _, ok := chatBriefLocaleLimits[key]; !ok {
							failures = append(failures, fmt.Sprintf("%s: the limits table has no row for %s", lang, key))
						}
					}
				}
			}
			for key, limit := range chatBriefLocaleLimits {
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
				for _, tag := range chatBriefAllowedTags {
					stripped = strings.ReplaceAll(stripped, tag, "")
				}
				if strings.ContainsAny(stripped, "<>&`") || strings.Contains(stripped, "**") {
					failures = append(failures, fmt.Sprintf("%s: %s holds markup other than b, i and code tags", lang, key))
				}
				for _, tag := range []string{"b", "i", "code"} {
					if strings.Count(text, "<"+tag+">") != strings.Count(text, "</"+tag+">") {
						failures = append(failures, fmt.Sprintf("%s: %s has an unbalanced <%s>", lang, key, tag))
					}
				}
				if settingsVerbPattern.MatchString(text) {
					failures = append(failures, fmt.Sprintf("%s: %s holds a printf verb", lang, key))
				}
				englishText, _ := english[key].(string)
				if got, want := staffPlaceholders(text), staffPlaceholders(englishText); !slices.Equal(got, want) {
					failures = append(failures, fmt.Sprintf("%s: %s has placeholders %v, en.yml has %v", lang, key, got, want))
				}
			}
			slices.Sort(failures)
			for _, failure := range failures {
				t.Error(failure)
			}
		})
	}
}
