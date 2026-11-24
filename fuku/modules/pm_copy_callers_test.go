//go:build testtools

package modules

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestPMCopyCallers holds PMSG-05 structurally (D-04, D-33): a lockdown's joiner bans, "Ban N recent
// joiners" and every Staff Group action must never send a private copy, so no raid-scale or staff path
// may reach the private copy engine. It parses every non-test file of the package and compares the
// files that use offerEventPM, registerEventPM, postEvent and postEventSent (any reference but the
// declaration itself) with the allow-lists below, and it fails when a file whose base name starts with staff,
// lockdown or raid names offerEventPM, registerEventPM, offerReplyPM, replyEvent, postEvent, postEventSent or
// any identifier starting with pmCopy.
// Each allow-list entry carries the reason the file may use the engine, and an entry no longer found
// fails, so the lists stay exact. A later phase that gives its events a private copy adds its caller
// here with a reason, never a staff, lockdown or raid file.
func TestPMCopyCallers(t *testing.T) {
	allowed := map[string]map[string]string{
		"offerEventPM": {
			"event_message.go": "the moderation pipeline's reply step, after the punishment",
		},
		"registerEventPM": {
			"event_message_pm.go": "the manual events' specs, registered with the engine",
		},
		"postEvent": {
			"event_message.go": "replyEvent builds the moderation pipeline's site and posts its reply after the punishment",
			"warns.go":         "warnThisUser posts its warning after the warn is stored and any limit action applied",
			"antiflood.go":     "checkFlood posts its notice (floodNotice) after the mute, kick or ban",
			"blacklists.go":    "the blacklist watcher posts its notice (blacklistNotice) after the mute, kick or ban",
			"forwardblock_notice.go": "the forward blocklist's notice of the linked group, after its mute, kick or ban; " +
				"the watcher reaches the engine only through this non-staff file and warns.go (D-30)",
			"event_message_auto.go": "postCaptchaFailureNotice posts the captcha failure's notice after the failure action, " +
				"for captcha.go, which stays free of the engine",
			"aispam_notice.go": "postAISpamNotice posts the AI spam notice after the message was deleted, behind its per-member " +
				"and per-group cap (off by default)",
			"event_message_lockdown.go": "the lockdown and lift messages, posted after the lockdown or the lift took effect, for the " +
				"lockdown files, which stay free of the engine; the lockdown events have no private copy spec, so no copy is ever offered",
		},
		"postEventSent": {
			"event_message.go": "postEvent wraps it; a site that needs what the group step posted calls it after the punishment",
			"memberreq_notice.go": "memberReqPostEventNotice posts the requirement notice of the episode after the punishment took effect " +
				"and stores the posted message's ID on the hold (the mute notice's \"I fixed it\")",
		},
	}
	forbiddenNames := map[string]bool{
		"offerEventPM": true, "registerEventPM": true, "offerReplyPM": true, "replyEvent": true, "postEvent": true,
		"postEventSent": true,
	}
	forbiddenPrefixes := []string{"staff", "lockdown", "raid"}

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	fset := token.NewFileSet()
	users := map[string]map[string]bool{"offerEventPM": {}, "registerEventPM": {}, "postEvent": {}, "postEventSent": {}}
	scanned := 0
	for _, path := range paths {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		scanned++
		declared := map[*ast.Ident]bool{}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				declared[fn.Name] = true
			}
		}
		guarded := false
		for _, prefix := range forbiddenPrefixes {
			guarded = guarded || strings.HasPrefix(base, prefix)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			if guarded && (forbiddenNames[ident.Name] || strings.HasPrefix(ident.Name, "pmCopy")) {
				t.Errorf("%s names %s: a staff, lockdown or raid path must never reach the private copy engine (PMSG-05)",
					base, ident.Name)
			}
			if set, tracked := users[ident.Name]; tracked && !declared[ident] {
				set[base] = true
			}
			return true
		})
	}
	if scanned == 0 {
		t.Fatal("no source file was scanned")
	}

	for name, files := range users {
		var got []string
		for file := range files {
			got = append(got, file)
		}
		sort.Strings(got)
		for _, file := range got {
			if reason, ok := allowed[name][file]; !ok || strings.TrimSpace(reason) == "" {
				t.Errorf("%s uses %s but is not in its allow-list (or has no reason): add it with a reason, never from a staff, lockdown or raid path", file, name)
			}
		}
		for file := range allowed[name] {
			if !files[file] {
				t.Errorf("the allow-list of %s names %s, which no longer uses it: remove the stale entry", name, file)
			}
		}
	}
}
