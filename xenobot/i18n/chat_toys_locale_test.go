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

// chatToysKeyPrefixes are the key families of the Phase 35 Interactive Group Toys (/referendum,
// tied-poll tie-breaker, and /inspect). The settings_pc_* keys live in settingsLocaleLimits.
var chatToysKeyPrefixes = []string{"referendum_", "inspect_"}

type chatToysLimit struct {
	max   int
	other int
}

// chatToysLocaleLimits has one row per referendum_ and inspect_ key. The test fails when it does
// not cover exactly those keys of every locale file, so a change that adds a key adds its row.
var chatToysLocaleLimits = map[string]chatToysLimit{
	"referendum_off_hint":            {140, 200},
	"referendum_unavailable":         {80, 120},
	"referendum_need_context":        {160, 220},
	"referendum_allowance":           {90, 130},
	"referendum_malfunction":         {90, 130},
	"referendum_refused":             {80, 120},
	"referendum_tiebreaker_header":   {50, 70},
	"referendum_tiebreaker_fallback": {140, 200},
	"inspect_usage":                  {140, 200},
	"inspect_off_hint":               {140, 200},
	"inspect_unavailable":            {80, 120},
	"inspect_allowance":              {90, 130},
	"inspect_malfunction":            {90, 130},
	"inspect_refused":                {80, 120},
	"inspect_header":                 {50, 70},
}

var chatToysAllowedTags = []string{"<b>", "</b>", "<i>", "</i>", "<code>", "</code>"}

// TestChatToysLocaleKeys validates every referendum_* and inspect_* key across all 8 locales for
// presence, non-empty string value, rune ceiling, balanced allowed HTML tags, zero printf verbs,
// and placeholder parity with en.yml.
func TestChatToysLocaleKeys(t *testing.T) {
	english := loadStaffLocale(t, "en")
	for _, lang := range staffLocales {
		t.Run(lang, func(t *testing.T) {
			data := loadStaffLocale(t, lang)
			var failures []string
			for key := range data {
				for _, prefix := range chatToysKeyPrefixes {
					if strings.HasPrefix(key, prefix) {
						if _, ok := chatToysLocaleLimits[key]; !ok {
							failures = append(failures, fmt.Sprintf("%s: the limits table has no row for %s", lang, key))
						}
					}
				}
			}
			for key, limit := range chatToysLocaleLimits {
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
				for _, tag := range chatToysAllowedTags {
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

var chatToysKeyLiteral = regexp.MustCompile(`"((?:referendum|inspect)_[a-z_]+)"`)

// TestChatToysKeyLiteralsExist checks every referendum_* / inspect_* literal found in
// ../modules/*chat*.go (tolerating zero matches before Wave 3 handlers land) plus every key in
// chatToysLocaleLimits across all 8 locales.
func TestChatToysKeyLiteralsExist(t *testing.T) {
	files, err := filepath.Glob("../modules/*chat*.go")
	if err != nil {
		t.Fatalf("glob Chat module files: %v", err)
	}
	keys := map[string]bool{}
	for key := range chatToysLocaleLimits {
		keys[key] = true
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Clean(file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, m := range chatToysKeyLiteral.FindAllStringSubmatch(string(raw), -1) {
			keys[m[1]] = true
		}
	}
	for _, lang := range staffLocales {
		data := loadStaffLocale(t, lang)
		for key := range keys {
			if text, ok := data[key].(string); !ok || strings.TrimSpace(text) == "" {
				t.Errorf("%s: Chat toys key %s is missing or empty", lang, key)
			}
		}
	}
}

// TestChatHelpIncludesToys ensures chat_help_msg names /referendum and /inspect in every locale
// while staying within Telegram's 4096 UTF-16 unit limit.
func TestChatHelpIncludesToys(t *testing.T) {
	for _, lang := range staffLocales {
		help, _ := loadStaffLocale(t, lang)["chat_help_msg"].(string)
		if total := lockdownHelpLength(help); total > lockdownHelpTelegramLimit {
			t.Errorf("%s: chat_help_msg is %d UTF-16 units, over Telegram's %d", lang, total, lockdownHelpTelegramLimit)
		}
		for _, want := range []string{"/referendum", "/inspect"} {
			if !strings.Contains(help, want) {
				t.Errorf("%s: chat_help_msg does not name %s", lang, want)
			}
		}
	}
}
