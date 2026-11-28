package chatprompt

import (
	"strings"
	"testing"
)

func TestParseMemoryJSON(t *testing.T) {
	long := strings.Repeat("x", MaxMemoryNoteRunes+1)
	var thirteen []string
	for i := range MaxMemoryNotes + 1 {
		thirteen = append(thirteen, `"n`+strings.Repeat("a", i)+`"`)
	}
	cases := []struct {
		name  string
		raw   string
		want  []string
		valid bool
	}{
		{"plain", `{"notes":["Likes cats","Plays chess"]}`, []string{"Likes cats", "Plays chess"}, true},
		{"fenced", "```json\n{\"notes\":[\"Likes cats\"]}\n```", []string{"Likes cats"}, true},
		{"empty list is an answer", `{"notes":[]}`, []string{}, true},
		{"clean, drop empty overlong and repeats", `{"notes":["  Likes\n cats ","","` + long + `","likes CATS","\u202ePlays chess"]}`, []string{"Likes cats", "Plays chess"}, true},
		{"exactly the longest note", `{"notes":["` + strings.Repeat("y", MaxMemoryNoteRunes) + `"]}`, []string{strings.Repeat("y", MaxMemoryNoteRunes)}, true},
		{"too many notes", `{"notes":[` + strings.Join(thirteen, ",") + `]}`, nil, false},
		{"not an object", `["Likes cats"]`, nil, false},
		{"extra key", `{"notes":["a"],"x":1}`, nil, false},
		{"missing notes", `{"note":["a"]}`, nil, false},
		{"null notes", `{"notes":null}`, nil, false},
		{"notes not an array", `{"notes":"Likes cats"}`, nil, false},
		{"non-string entry", `{"notes":["a",3]}`, nil, false},
		{"trailing text", `{"notes":["a"]} thanks`, nil, false},
		{"empty", ``, nil, false},
	}
	for _, tc := range cases {
		got, ok := ParseMemoryJSON(tc.raw)
		if ok != tc.valid || (ok && strings.Join(got, "|") != strings.Join(tc.want, "|")) {
			t.Errorf("%s: got %q ok=%v, want %q ok=%v", tc.name, got, ok, tc.want, tc.valid)
		}
	}
}

func TestMemoryPrompts(t *testing.T) {
	user := MemoryUser([]string{"Likes cats", "  ", "Plays\nchess"}, "I just got\na second kitten!", "Notice. Congratulations.")
	for _, want := range []string{"[Current notes about this member]", "- Likes cats", "- Plays chess", "I just got a second kitten!", "Notice. Congratulations."} {
		if !strings.Contains(user, want) {
			t.Errorf("MemoryUser lacks %q:\n%s", want, user)
		}
	}
	if strings.Contains(user, "\n\n\n") || strings.Contains(user, "(none)") {
		t.Errorf("unexpected shape:\n%s", user)
	}
	if !strings.Contains(MemoryUser(nil, "hi", "yo"), "(none)") {
		t.Error("an empty list is not marked")
	}
	if !strings.Contains(MemoryStatic(), `{"notes":`) || !strings.Contains(MemoryStatic(), "untrusted") {
		t.Error("the writer instruction lost its format line or its untrusted warning")
	}
}

func TestMemorySection(t *testing.T) {
	if got := MemorySection(nil); got != "" {
		t.Errorf("no notes gave %q", got)
	}
	if got := UserWithMemory([]string{"", "  "}, "USER"); got != "USER" {
		t.Errorf("blank notes changed the user text: %q", got)
	}
	got := UserWithMemory([]string{"Likes cats", "Has\na dog"}, "USER")
	if !strings.HasPrefix(got, "[What this speaker told you") || !strings.Contains(got, "- Likes cats\n- Has a dog\n\nUSER") {
		t.Errorf("section = %q", got)
	}
	var many []string
	for range 20 {
		many = append(many, "note")
	}
	if n := strings.Count(MemorySection(many), "- note"); n != MaxMemoryNotes {
		t.Errorf("a section holds %d notes, want %d", n, MaxMemoryNotes)
	}
}
