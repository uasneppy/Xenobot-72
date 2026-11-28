package modules

import (
	"regexp"
	"strings"
)

// Reasoning leak guard. A provider sometimes writes its planning into the visible answer instead of
// the thinking channel ("Ukrainian reply, short, in character. ... Something like: ... Final: ...").
// A conversation reply passes chatFilterReply, which calls chatStripReasoning first: the text after
// the last "Final:" style marker is the answer; a draft with no marker is dropped (blank, never sent).

var (
	// chatFinalMarker matches a line that starts the final answer of a leaked draft, with optional
	// markdown emphasis, bullets and a label such as "Final:", "Final answer:" or "Фінал:".
	chatFinalMarker = regexp.MustCompile(`(?im)^[\s>*_#\-]*(?:final(?:\s+(?:answer|reply|response|output|version|text))?|output|answer|reply|response|фінал(?:ьна\s+відповідь)?|финал(?:ьный\s+ответ)?)[\s*_]*:[\s*_]*`)
	// chatDraftCue matches planning phrases that never belong in an in-character reply.
	chatDraftCue = regexp.MustCompile(`(?i)(?:\bsomething like\s*:|\bmaybe add\b|\bkeep it (?:brief|short)\b|\bavoid gendered forms\b|\b(?:ukrainian|english|russian|polish|german|french|spanish|italian|turkish) (?:reply|response|answer)\b[^.\n]{0,40}\bin character\b|\bthe user (?:is asking|wants|said|asked)\b|\bi (?:should|need to|must) (?:reply|respond|answer|keep|avoid|use)\b)`)
)

// chatDraftCueWindow is how far into the text the first planning cue may start. A leak opens with
// its planning, so a phrase like "maybe add" deep inside a long, ordinary answer is not one.
const chatDraftCueWindow = 240

// chatStripReasoning returns the answer inside a leaked planning draft, "" for a draft with no
// marked answer, and text unchanged when it shows no sign of a leak. A leak needs at least two
// distinct planning cues, the first near the start of the text: one ordinary phrase such as
// "maybe add some salt" is a normal reply. It is applied to conversation replies only; a brief, a
// verdict or a poll quotes members and is never run through it.
func chatStripReasoning(text string) string {
	locs := chatFinalMarker.FindAllStringIndex(text, -1)
	draftEnd := len(text)
	if len(locs) > 0 {
		draftEnd = locs[len(locs)-1][0]
	}
	cues := chatDraftCue.FindAllStringIndex(text[:draftEnd], -1)
	if len(cues) < 2 || cues[0][0] > chatDraftCueWindow {
		return text
	}
	if len(locs) == 0 {
		return ""
	}
	answer := strings.TrimSpace(text[locs[len(locs)-1][1]:])
	return strings.TrimSpace(strings.Trim(answer, "\"“”"))
}
