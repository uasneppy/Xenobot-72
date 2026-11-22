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

// TestEventEngineCallers holds EVT-10 structurally (Phase 25, D-33): the lockdown alert, its reminders,
// the raid-scale removals and their reports, the lift tally, the cannot-lock warning, every Staff Group
// surface, the owner log dump and the captcha challenge are not events, so none of them may reach the
// renderer or the private copy sender. It parses every non-test file of the package and proves two
// things.
//
// Part 1, callers: every function outside the engine's own files (event_message.go, event_message_pm.go)
// whose body names an engine entry (eventEngineEntries, plus every postLockdown* adapter declared in
// event_message_lockdown.go) is in eventEngineCallers, keyed "file.go: Function" (a method carries its
// receiver type, as repliesGuardDeclKey builds it; a method whose receiver has no name is keyed by its bare
// name), each with its reason; a new caller and a stale entry
// both fail. The callers of the postLockdown* adapters are also held to eventLockdownAdapterCallers
// exactly, so a lockdown message is posted only from the sites the phase chose.
//
// Part 2, never reach: from each root of eventEngineNeverRoots (each must exist, so a rename cannot
// switch the check off), and not entering the few functions eventEngineRootStops names for it, the scan follows the same-package functions a body names (a plain function,
// called or used as a value) and the methods it calls or names on its own receiver, function literals in
// a body included, with the name resolution of command_replies_guard_test.go, and fails when an engine
// entry or a postLockdown* adapter is reachable. Its limit: without type information a call through a
// function value held in a variable or a field, an interface, or another type's method is not followed.
// A log channel post through fuku/utils/actionlog cannot reach the engine at all, because that package
// cannot import fuku/modules.
//
// A later phase that gives a new send site a custom message adds its caller here with a reason, never
// from an alert, reminder, raid-scale, staff or captcha challenge path.
func TestEventEngineCallers(t *testing.T) {
	files := eventEngineParseDir(t, ".")
	scan := eventEngineScan(files)

	eventEngineWantExact(t, "an engine caller", scan.callers, eventEngineCallers)
	eventEngineWantExact(t, "a postLockdown* adapter caller", scan.adapterCallers, eventLockdownAdapterCallers)

	for _, root := range eventEngineNeverRoots {
		if len(scan.decls[root]) == 0 {
			t.Errorf("the never-reach root %s no longer exists: update eventEngineNeverRoots to its new name", root)
			continue
		}
		for stop := range eventEngineRootStops[root] {
			if len(scan.decls[stop]) == 0 {
				t.Errorf("the stop %s of the root %s no longer exists: update eventEngineRootStops", stop, root)
			}
		}
		if path := scan.reaches(root, eventEngineRootStops[root]); path != "" {
			t.Errorf("%s reaches the event message engine (%s): an alert, reminder, raid-scale, staff, log or captcha challenge path "+
				"must never go through the renderer or the private copy sender (EVT-10)", root, path)
		}
	}
}

// TestEventEngineCallersGuardFails proves the guard can fail: in a synthetic file set a root that reaches
// postEvent through a helper, one that reaches it through a function literal and one through a method of
// its own receiver are reported, a root that only sends is not, and an unlisted caller is found.
func TestEventEngineCallersGuardFails(t *testing.T) {
	sources := map[string]string{
		"event_message.go": `package modules
func postEvent(site eventSite, r eventReply) error { return nil }
`,
		"event_message_lockdown.go": `package modules
func postLockdownSampleEvent() { _ = postEvent(eventSite{}, eventReply{}) }
`,
		"sample.go": `package modules
func viaHelper()  { helper() }
func helper()     { _ = postEvent(eventSite{}, eventReply{}) }
func viaLiteral() { go func() { _ = postEvent(eventSite{}, eventReply{}) }() }
func viaAdapter() { postLockdownSampleEvent() }
func sendOnly(b Bot) { b.SendMessage(1, "x", nil) }

type mod struct{}

func (m mod) viaMethod() { m.later() }
func (m mod) later()     { helper() }
`,
	}
	fset := token.NewFileSet()
	var files []eventEngineFile
	for name, src := range sources {
		parsed, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, eventEngineFile{name: name, file: parsed})
	}
	scan := eventEngineScan(files)

	var callers []string
	for key := range scan.callers {
		callers = append(callers, key)
	}
	sort.Strings(callers)
	want := "event_message_lockdown.go: postLockdownSampleEvent,sample.go: helper,sample.go: viaAdapter,sample.go: viaLiteral"
	if got := strings.Join(callers, ","); got != want {
		t.Fatalf("callers = %q, want %q", got, want)
	}
	if !scan.adapterCallers["sample.go: viaAdapter"] || len(scan.adapterCallers) != 1 {
		t.Fatalf("adapter callers = %v, want only sample.go: viaAdapter", scan.adapterCallers)
	}
	for _, root := range []string{"viaHelper", "viaLiteral", "viaAdapter", "mod.viaMethod"} {
		if scan.reaches(root, nil) == "" {
			t.Errorf("the root %s reaches postEvent but was not reported", root)
		}
	}
	if path := scan.reaches("sendOnly", nil); path != "" {
		t.Errorf("sendOnly reaches nothing of the engine, but was reported through %s", path)
	}
	if path := scan.reaches("viaHelper", map[string]string{"helper": "a stop"}); path != "" {
		t.Errorf("a stop did not end the walk: viaHelper was reported through %s", path)
	}
}

// eventEngineEntries are the engine's entries: the post, the group step, the moderation pipeline's reply, the
// composers and renderer, the private copy offer and sender, and the messages read.
var eventEngineEntries = map[string]bool{
	"postEvent": true, "postEventSent": true, "postEventGroup": true, "replyEvent": true, "composeEventMessage": true,
	"renderEventText": true, "composeEventPM": true, "offerEventPM": true, "offerReplyPM": true, "eventMessagesLookup": true,
}

// eventEngineOwnFiles are the engine's own files, whose functions are not callers.
var eventEngineOwnFiles = map[string]bool{"event_message.go": true, "event_message_pm.go": true}

// eventLockdownAdapterFile declares the postLockdown* adapters, the lockdown files' only way to the engine.
const eventLockdownAdapterFile = "event_message_lockdown.go"

// eventEngineCallers is every function outside the engine's own files that names an engine entry or a
// postLockdown* adapter, recorded from the tree, each with the reason it may.
var eventEngineCallers = map[string]string{
	"aispam_notice.go: postAISpamNotice":              "the AI spam notice, after the message was deleted, behind its caps (Phase 24)",
	"antiflood.go: floodNotice":                       "the flood notice, after antiflood's mute, kick or ban (Phase 23)",
	"bans.go: banReplyWithButton":                     "the /ban and /dban reply, after the ban (Phase 21)",
	"bans.go: kickReply":                              "the /kick and /dkick reply, after the kick (Phase 21)",
	"bans.go: moderationTban":                         "the /tban reply, after the timed ban (Phase 21)",
	"bans.go: moderationUnban":                        "the /unban reply, after the unban (Phase 21)",
	"blacklists.go: blacklistNotice":                  "the blacklist's notice, after its mute, kick or ban (Phase 23)",
	"event_message_auto.go: postCaptchaFailureNotice": "the failed captcha's notice, after the failure action, for captcha.go (Phase 24)",
	"forwardblock_notice.go: forwardBlockNotice":      "the forward blocklist's notice of the linked group, after its mute, kick or ban (Phase 23)",
	"memberreq_notice.go: memberReqPostEventNotice":   "the member requirement notice, after the punishment took effect (Phase 24)",
	"mute.go: moderationTmute":                        "the /tmute reply, after the timed mute (Phase 21)",
	"mute.go: moderationUnmute":                       "the /unmute reply, after the unmute (Phase 21)",
	"mute.go: muteReplyWithButton":                    "the /mute and /dmute reply, after the mute (Phase 21)",
	"settings_messages.go: settingsMsgPreview": "the Messages page's preview of a group message, sent to the presser's private chat only " +
		"(Phase 21)",
	"settings_messages.go: settingsMsgPMPreview": "the Messages page's preview of a private copy, sent to the presser's private chat only " +
		"(Phase 22)",
	"warns.go: warnThisUser": "the warning and the warn limit's message, after the warn is stored and any limit action " +
		"applied (Phase 23)",
	"event_message_lockdown.go: postLockdownStartEvent": "the adapter of the manual lockdown's reply, posted after the lockdown " +
		"started (Phase 25)",
	"lockdown.go: moduleStruct.lockdown": "calls postLockdownStartEvent after startLockdownRow answered lockdownStartStarted (Phase 25)",
	"event_message_lockdown.go: postLockdownAutoEvent": "the adapter of the automatic lockdown's public notice, posted after a " +
		"trigger's lockdown started, inside the post raidPostAsync counts (Phase 25)",
	"raid_trigger.go: raidPostNotice": "calls postLockdownAutoEvent, run by raidFire through raidPostAsync only for " +
		"lockdownStartStarted (Phase 25)",
}

// eventLockdownAdapterCallers is exactly every function that names a postLockdown* adapter: the sites the
// phase chose for the lockdown messages. Plans 25-04 and 25-05 add the automatic notice and the lifts.
var eventLockdownAdapterCallers = map[string]string{
	"lockdown.go: moduleStruct.lockdown": "the reply of a /lockdown that started a lockdown (EVT-06)",
	"raid_trigger.go: raidPostNotice":    "the public notice of a lockdown a join surge, a flood or an AI burst started (EVT-07)",
}

// eventEngineNeverRoots are the roots that must never reach the engine (D-33): the lockdown worker's cycle
// (the alerts, reminders, their refresh and close, the joiner bans, declines and unbans, the lift tally, the
// "Ban N recent joiners" runs, their reports and #LOCKDOWN_BAN), the cannot-lock warning, the inline lift
// tally, the alert's buttons other than Lift Confirm, every Staff Group surface (action cards and their runs,
// undo, staff log posts, the panel), the captcha challenge and the AI spam owner log dump.
var eventEngineNeverRoots = []string{
	"runLockdownCycle",
	"raidWarnCannotLock",
	"lockdownPostInlineTally",
	"moduleStruct.lockdownLiftAsk",
	"moduleStruct.lockdownBanAsk",
	"moduleStruct.lockdownBanConfirm",
	"moduleStruct.lockdownRevokeAsk",
	"moduleStruct.lockdownRevokeConfirm",
	"moduleStruct.lockdownCardCancel",
	"moduleStruct.handleStaffAction",
	"moduleStruct.staffActionConfirm",
	"moduleStruct.staffActionCancel",
	"moduleStruct.staffUndoAsk",
	"moduleStruct.staffUndoConfirm",
	"startStaffRun",
	"runStaffActionInGroup",
	"sendStaffLogPost",
	"buildStaffPanel",
	"SendCaptcha",
	"aispamMirrorDeletion",
}

// eventEngineRootStops are, per never-reach root, the functions the walk does not enter, each with the reason
// what lies behind it is a different, sanctioned event and not the root's own surface. The captcha challenge
// schedules the attempt's timeout, and a timeout that fails the member posts the failed captcha's notice
// (the captcha event, Phase 24); the challenge message itself never goes through the engine.
var eventEngineRootStops = map[string]map[string]string{
	"SendCaptcha": {
		"handleCaptchaTimeout": "the failure of the attempt the challenge scheduled posts the captcha event (postCaptchaFailureNotice), " +
			"not the challenge",
	},
}

// eventEngineFile is one parsed file of the package.
type eventEngineFile struct {
	name string
	file *ast.File
}

// eventEngineParseDir parses the non-test files of dir.
func eventEngineParseDir(t *testing.T, dir string) []eventEngineFile {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var files []eventEngineFile
	for _, path := range paths {
		base := filepath.Base(path)
		if strings.HasSuffix(base, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		parsed, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, eventEngineFile{name: base, file: parsed})
	}
	if len(files) == 0 {
		t.Fatal("no source file was parsed")
	}
	return files
}

// eventEngineScanResult is the declaration index, the targets (the engine entries and the adapters), the
// callers of any target and the callers of an adapter.
type eventEngineScanResult struct {
	decls          repliesGuardDecls
	targets        map[string]bool
	adapters       map[string]bool
	callers        map[string]bool
	adapterCallers map[string]bool
}

// eventEngineScan indexes the files' functions, collects the postLockdown* adapters of the adapter file and
// records every function outside the engine's own files that names a target.
func eventEngineScan(files []eventEngineFile) eventEngineScanResult {
	scan := eventEngineScanResult{
		decls:          repliesGuardDecls{},
		targets:        map[string]bool{},
		adapters:       map[string]bool{},
		callers:        map[string]bool{},
		adapterCallers: map[string]bool{},
	}
	for name := range eventEngineEntries {
		scan.targets[name] = true
	}
	for _, f := range files {
		for _, decl := range f.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			key := repliesGuardDeclKey(fn)
			scan.decls[key] = append(scan.decls[key], fn)
			if f.name == eventLockdownAdapterFile && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "postLockdown") {
				scan.adapters[fn.Name.Name] = true
				scan.targets[fn.Name.Name] = true
			}
		}
	}
	for _, f := range files {
		if eventEngineOwnFiles[f.name] {
			continue
		}
		for _, decl := range f.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			key := f.name + ": " + repliesGuardDeclKey(fn)
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if ident, ok := node.(*ast.Ident); ok {
					if scan.targets[ident.Name] {
						scan.callers[key] = true
					}
					if scan.adapters[ident.Name] {
						scan.adapterCallers[key] = true
					}
				}
				return true
			})
		}
		// A package-level value that names a target is a caller too.
		for _, decl := range f.file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok {
				ast.Inspect(gen, func(node ast.Node) bool {
					if ident, ok := node.(*ast.Ident); ok && scan.targets[ident.Name] {
						scan.callers[f.name+": (package level)"] = true
					}
					return true
				})
			}
		}
	}
	return scan
}

// reaches reports the path from root to the first target it can reach ("" for none), following the
// same-package functions a body names and the methods it names on its own receiver, function literals
// included, and never entering a function of stops.
func (scan eventEngineScanResult) reaches(root string, stops map[string]string) string {
	seen := map[string]bool{}
	for stop := range stops {
		seen[stop] = true
	}
	var walk func(key string, path []string) string
	walk = func(key string, path []string) string {
		if seen[key] {
			return ""
		}
		seen[key] = true
		path = append(path, key)
		for _, fn := range scan.decls[key] {
			scope := repliesGuardScopeOf(fn)
			var found string
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if found != "" {
					return false
				}
				switch n := node.(type) {
				case *ast.Ident:
					if scan.targets[n.Name] {
						found = strings.Join(append(path, n.Name), " -> ")
						return false
					}
					if len(scan.decls[n.Name]) > 0 {
						found = walk(n.Name, path)
					}
				case *ast.SelectorExpr:
					if x, ok := n.X.(*ast.Ident); ok && scope.recvName != "" && x.Name == scope.recvName {
						if method := scope.recvType + "." + n.Sel.Name; len(scan.decls[method]) > 0 {
							found = walk(method, path)
						}
					}
				}
				return found == ""
			})
			if found != "" {
				return found
			}
		}
		return ""
	}
	return walk(root, nil)
}

// eventEngineWantExact fails for every key of got missing from want (or listed without a reason) and every
// key of want missing from got.
func eventEngineWantExact(t *testing.T, what string, got map[string]bool, want map[string]string) {
	t.Helper()
	var unexpected, stale []string
	for key := range got {
		if reason, ok := want[key]; !ok || strings.TrimSpace(reason) == "" {
			unexpected = append(unexpected, key)
		}
	}
	for key := range want {
		if !got[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(unexpected)
	sort.Strings(stale)
	if len(unexpected) > 0 {
		t.Errorf("%s is not in its allow-list (or has no reason): %s\nadd it with a reason, never from an alert, reminder, "+
			"raid-scale, staff or captcha challenge path", what, strings.Join(unexpected, ", "))
	}
	if len(stale) > 0 {
		t.Errorf("allow-list entries for %s no longer found (remove them): %s", what, strings.Join(stale, ", "))
	}
}
