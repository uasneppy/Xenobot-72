package modules

import "testing"

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
	}
	for _, tc := range cases {
		if got := chatFilterReply(tc.in); got != tc.want {
			t.Fatalf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}
