package modules

import (
	"strings"
	"testing"
)

func TestChatStripReasoning(t *testing.T) {
	leak := "Ukrainian reply, short, in character. Huntress says \"з себе посміявся\".\n\n" +
		"Something like: \"Запит відхилено. Це не моя заслуга.\" Maybe add a dry line. Keep it brief.\n\n" +
		"Avoid gendered forms for the user. Use «ви».\n\nFinal:\n\n" +
		"Зафіксовано. Смішний результат пов'язаний із формулюванням запиту. Наступного разу формулюйте точніше."
	cases := []struct{ name, in, want string }{
		{"leak with final marker", leak, "Зафіксовано. Смішний результат пов'язаний із формулюванням запиту. Наступного разу формулюйте точніше."},
		{"leak without marker is dropped", "Ukrainian reply, short, in character. Something like: \"Запит відхилено.\"", ""},
		{"clean reply untouched", "Зафіксовано. Запит відхилено.", "Зафіксовано. Запит відхилено."},
		{"final word in normal reply untouched", "Final: procedure complete.", "Final: procedure complete."},
		{"one ordinary cue is a normal reply", "Maybe add some salt, then taste it.", "Maybe add some salt, then taste it."},
		{"the user wants is a normal reply", "The user wants more? Then ask nicely.", "The user wants more? Then ask nicely."},
		{"two cues deep in a long answer are not a leak", "Зафіксовано. " + strings.Repeat("Процедура триває. ", 30) + "Maybe add a form. Keep it brief.", "Зафіксовано. " + strings.Repeat("Процедура триває. ", 30) + "Maybe add a form. Keep it brief."},
	}
	for _, tc := range cases {
		if got := chatFilterReply(tc.in); got != tc.want {
			t.Fatalf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestChatSanitizeTextKeepsMemberQuotes: a brief, a verdict or a poll text is never run through the
// reasoning guard, so a summary that quotes a member's "maybe add ..." and a quiz thread's "Answer:"
// lines arrive whole.
func TestChatSanitizeTextKeepsMemberQuotes(t *testing.T) {
	brief := "Alex said maybe add a rules channel and keep it short.\nAnswer: 42\nReply: no."
	if got := chatSanitizeText(brief); got != brief {
		t.Fatalf("sanitize changed a brief: %q", got)
	}
	if got := chatSanitizeText("see https://evil.example @bob"); got != "see [link removed] @\u200bbob" {
		t.Fatalf("links and handles not neutralised: %q", got)
	}
}
