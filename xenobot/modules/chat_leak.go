package modules

import (
	"regexp"
	"strings"
)

// Reasoning leak guard. A provider sometimes writes its planning into the visible answer instead of
// the thinking channel ("Ukrainian reply, short, in character. ... Something like: ... Final: ...").
// Every provider text passes chatSanitizeText, which calls chatStripReasoning first: the text after
// the last "Final:" style marker is the answer; a draft with no marker is dropped (blank, never sent).

var (
	// chatFinalMarker matches a line that starts the final answer of a leaked draft, with optional
	// markdown emphasis, bullets and a label such as "Final:", "Final answer:" or "Фінал:".
	chatFinalMarker = regexp.MustCompile(`(?im)^[\s>*_#\-]*(?:final(?:\s+(?:answer|reply|response|output|version|text))?|output|answer|reply|response|фінал(?:ьна\s+відповідь)?|финал(?:ьный\s+ответ)?)[\s*_]*:[\s*_]*`)
	// chatDraftCue matches planning phrases that never belong in an in-character reply.
	chatDraftCue = regexp.MustCompile(`(?i)(?:\bsomething like\s*:|\bmaybe add\b|\bkeep it (?:brief|short)\b|\bavoid gendered forms\b|\b(?:ukrainian|english|russian|polish|german|french|spanish|italian|turkish) (?:reply|response|answer)\b[^.\n]{0,40}\bin character\b|\bthe user (?:is asking|wants|said|asked)\b|\bi (?:should|need to|must) (?:reply|respond|answer|keep|avoid|use)\b)`)
)

// chatStripReasoning returns the answer inside a leaked planning draft, "" for a draft with no
// marked answer, and text unchanged when it shows no sign of a leak.
func chatStripReasoning(text string) string {
	if locs := chatFinalMarker.FindAllStringIndex(text, -1); len(locs) > 0 && chatDraftCue.MatchString(text[:locs[len(locs)-1][0]]) {
		answer := strings.TrimSpace(text[locs[len(locs)-1][1]:])
		return strings.TrimSpace(strings.Trim(answer, "\"“”"))
	}
	if chatDraftCue.MatchString(text) {
		return ""
	}
	return text
}
