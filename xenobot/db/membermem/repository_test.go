//go:build testtools

package membermem

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	long := strings.Repeat("x", MaxNoteRunes+1)
	ok := strings.Repeat("y", MaxNoteRunes)
	var many []string
	for i := range MaxNotes + 5 {
		many = append(many, strings.Repeat("n", i+1))
	}
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil", nil, []string{}},
		{"clean and collapse", []string{"  likes\tcats \n and dogs  ", "\u200bzero\u202ewidth"}, []string{"likes cats and dogs", "zerowidth"}},
		{"drop empty and overlong", []string{"", "   ", long, ok}, []string{ok}},
		{"drop case repeats, keep the first", []string{"Has a cat", "has a CAT", "Plays chess"}, []string{"Has a cat", "Plays chess"}},
		{"cap at MaxNotes keeps the first", many, many[:MaxNotes]},
	}
	for _, tc := range cases {
		got := Normalize(tc.in)
		if strings.Join(got, "\n") != strings.Join(tc.want, "\n") || len(got) != len(tc.want) {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestMemberMemoryStorage: notes are keyed by (chat, member), replaced as a whole, never cross a chat or
// a member, and removed by Delete and DeleteChat.
func TestMemberMemoryStorage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	chatA, chatB := time.Now().UnixNano()%1_000_000_000*-1-1, time.Now().UnixNano()%1_000_000_000*-1-2
	alice, bob := int64(7001), int64(7002)

	if got, err := Get(ctx, chatA, alice); err != nil || len(got) != 0 {
		t.Fatalf("a member with no row read %q, %v", got, err)
	}
	stored, err := Replace(ctx, chatA, alice, []string{"Likes cats", "Works as a nurse"})
	if err != nil || len(stored) != 2 {
		t.Fatalf("Replace stored %q, %v", stored, err)
	}
	if _, err := Replace(ctx, chatA, bob, []string{"Plays chess"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Replace(ctx, chatB, alice, []string{"Collects stamps"}); err != nil {
		t.Fatal(err)
	}
	read := func(chat, user int64) string {
		t.Helper()
		got, err := Get(ctx, chat, user)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(got, "|")
	}
	if read(chatA, alice) != "Likes cats|Works as a nurse" || read(chatA, bob) != "Plays chess" || read(chatB, alice) != "Collects stamps" {
		t.Fatalf("notes crossed a chat or a member: %q %q %q", read(chatA, alice), read(chatA, bob), read(chatB, alice))
	}

	// A replace is the whole list, not an append.
	if _, err := Replace(ctx, chatA, alice, []string{"Likes dogs"}); err != nil {
		t.Fatal(err)
	}
	if got := read(chatA, alice); got != "Likes dogs" {
		t.Fatalf("after a replace: %q", got)
	}
	// A list that normalises to nothing deletes the row.
	if _, err := Replace(ctx, chatA, alice, []string{"  ", strings.Repeat("z", MaxNoteRunes+1)}); err != nil {
		t.Fatal(err)
	}
	if got := read(chatA, alice); got != "" {
		t.Fatalf("an empty replace left %q", got)
	}

	if removed, err := Delete(ctx, chatA, bob); err != nil || !removed {
		t.Fatalf("Delete of a stored row = %v, %v", removed, err)
	}
	if removed, err := Delete(ctx, chatA, bob); err != nil || removed {
		t.Fatalf("Delete of a missing row = %v, %v", removed, err)
	}
	if err := DeleteChat(ctx, chatB); err != nil {
		t.Fatal(err)
	}
	if got := read(chatB, alice); got != "" {
		t.Fatalf("DeleteChat left %q", got)
	}
	_ = DeleteChat(ctx, chatA)
}
