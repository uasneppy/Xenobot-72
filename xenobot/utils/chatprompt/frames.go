package chatprompt

import (
	"fmt"
	"strings"
)

// FrameNote is the code-owned line that tells the model an attached image is only one still
// preview frame of a moving picture (noun is "a GIF", "a video" and so on, never member text).
// Without it a model describes what happens in a clip it cannot see. index is the image's 1-based
// place among the total attachments the provider receives.
func FrameNote(index, total int, noun string) string {
	subject := "The attached image"
	if total > 1 {
		subject = fmt.Sprintf("Attachment %d of %d", index, total)
	}
	return subject + " is only one still preview frame of " + noun + ". " +
		"You cannot see its motion or hear any sound, so never describe or guess what happens in it."
}

// UnseenPictureNote is the line for a message whose picture exists but could not be attached
// (Telegram sent no preview frame). The model may judge the words only.
const UnseenPictureNote = "The picture itself is not available to you. Judge only the words, and never describe or guess what the picture shows."

// mediaNotesSection renders the bot's own notes about attached images as one section. It is
// empty when there are none, so a prompt without frames is byte-identical to what it was.
func mediaNotesSection(notes []string) string {
	var kept []string
	for _, n := range notes {
		if n = strings.TrimSpace(n); n != "" {
			kept = append(kept, n)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return "[Note from the bot about the attached images]\n" + strings.Join(kept, "\n") + "\n\n"
}
