//go:build testtools

package i18n

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	"brief_usage_long":      {200, 280},
	"brief_header_long":     {60, 80},
	"brief_empty":           {80, 120},
	"brief_header_short_pm": {60, 80},
	"brief_header_long_pm":  {60, 80},
	"brief_note_pm":         {80, 120},
	"brief_pm_blocked":      {140, 200},
	"judge_usage":           {200, 280},
	"judge_header":       {60, 80},
	"judge_header_other": {100, 140},
	"judge_ai_notice":    {180, 240},
	// judge_unscored_note and judge_unscored_more are plain text spliced into the verdict's closing
	// block; the names are escaped by the code.
	"judge_unscored_note":      {120, 160},
	"judge_unscored_more":      {24, 32},
	"judge_note_pm":            {80, 120},
	"judge_note_staff":         {80, 120},
	"judge_pm_blocked":         {140, 200},
	"judge_no_staff":           {120, 170},
	"judge_staff_check_failed": {100, 140},
	"judge_staff_send_failed":  {90, 130},
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

// chatKeyLiteral finds a locale key literal of the Chat module's code: a brief_, judge_ or chat_
// reply key, or a key of the Conversations page.
var chatKeyLiteral = regexp.MustCompile(`"((?:brief|judge|chat_reply|chat_off|chat_private|chat_ask|chat_help|settings_chat|settings_btn_chat|settings_state_chat)_[a-z_]+)"`)

// TestChatKeyLiteralsExist holds every key literal the Chat module's Go code passes to a translator
// to the locale files: make check-translations cannot see a key handed to a helper such as
// chatReplyText or chatSayGated, so a typo there would ship an empty message.
func TestChatKeyLiteralsExist(t *testing.T) {
	files, err := filepath.Glob("../modules/*chat*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no Chat module files found: %v", err)
	}
	keys := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, m := range chatKeyLiteral.FindAllStringSubmatch(string(raw), -1) {
			keys[m[1]] = true
		}
	}
	for _, key := range []string{"brief_usage_short", "brief_usage_long", "judge_usage", "settings_chat_log_notice"} {
		if !keys[key] {
			t.Fatalf("the scan found no %s, so it reads the wrong files", key)
		}
	}
	for _, lang := range staffLocales {
		data := loadStaffLocale(t, lang)
		for key := range keys {
			if text, ok := data[key].(string); !ok || strings.TrimSpace(text) == "" {
				t.Errorf("%s: the Chat module uses %s, which is missing or empty", lang, key)
			}
		}
	}
}

// TestChatHelpFitsOneMessage holds chat_help_msg in every language to one Telegram message and to the
// commands and facts it must name: /ask, /xeno, /unrollshort, /unrolllong, /judge with its three
// destinations, /settings, and the 1000 messages the log keeps.
func TestChatHelpFitsOneMessage(t *testing.T) {
	for _, lang := range staffLocales {
		help, _ := loadStaffLocale(t, lang)["chat_help_msg"].(string)
		if total := lockdownHelpLength(help); total > lockdownHelpTelegramLimit {
			t.Errorf("%s: chat_help_msg is %d UTF-16 units, over Telegram's %d", lang, total, lockdownHelpTelegramLimit)
		}
		for _, want := range []string{"/ask", "/xeno", "/unrollshort", "/unrolllong", "/judge", "/settings", "1000", "<code>group</code>", "<code>pm</code>", "<code>staff</code>"} {
			if !strings.Contains(help, want) {
				t.Errorf("%s: chat_help_msg does not name %s", lang, want)
			}
		}
	}
}
