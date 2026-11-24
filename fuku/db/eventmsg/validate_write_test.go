//go:build testtools

package eventmsg

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	utilscache "github.com/uasneppy/fuku_robot/fuku/utils/cache"
)

func TestEventMsgValidateAndReplaceAll(t *testing.T) {
	ctx := context.Background()

	t.Run("ValidateMessage rejects every invalid shape and normalizes NoSilent", func(t *testing.T) {
		manyRows := func(n int) []models.Button {
			out := make([]models.Button, n)
			for i := range out {
				out[i] = models.Button{Name: "Btn", Url: "https://example.com"}
			}
			return out
		}
		sameRow := func(n int) []models.Button {
			out := make([]models.Button, n)
			for i := range out {
				out[i] = models.Button{Name: "Btn", Url: "https://example.com", SameLine: i > 0}
			}
			return out
		}

		rejectCases := []struct {
			name   string
			msg    Message
			reason string
		}{
			{
				name:   "unknown event",
				msg:    Message{Event: "nope", Dest: DestGroup, Text: "hi"},
				reason: "unknown_event",
			},
			{
				name:   "unknown dest",
				msg:    Message{Event: EventBan, Dest: "channel", Text: "hi"},
				reason: "unknown_dest",
			},
			{
				name:   "lockdown PM unsupported",
				msg:    Message{Event: EventLockdown, Dest: DestPM, Text: "hi"},
				reason: "pm_not_supported",
			},
			{
				name:   "locklift PM unsupported",
				msg:    Message{Event: EventLockLift, Dest: DestPM, Text: "hi"},
				reason: "pm_not_supported",
			},
			{
				name:   "unpaired media missing type",
				msg:    Message{Event: EventBan, Dest: DestGroup, FileID: "file-1"},
				reason: "invalid_media",
			},
			{
				name:   "unpaired media missing file id",
				msg:    Message{Event: EventBan, Dest: DestGroup, MediaType: db.PHOTO},
				reason: "invalid_media",
			},
			{
				name:   "disallowed media type TEXT",
				msg:    Message{Event: EventBan, Dest: DestGroup, FileID: "file-1", MediaType: db.TEXT},
				reason: "invalid_media",
			},
			{
				name:   "disallowed media type VIDEO_NOTE",
				msg:    Message{Event: EventBan, Dest: DestGroup, FileID: "file-1", MediaType: db.VIDEO_NOTE},
				reason: "invalid_media",
			},
			{
				name:   "mreq group media refused",
				msg:    Message{Event: EventReqWarn, Dest: DestGroup, FileID: "file-1", MediaType: db.PHOTO},
				reason: "media_not_allowed",
			},
			{
				name:   "mreqmute group media refused",
				msg:    Message{Event: EventReqMute, Dest: DestGroup, FileID: "file-1", MediaType: db.ANIMATION},
				reason: "media_not_allowed",
			},
			{
				name:   "lockdown group sticker refused",
				msg:    Message{Event: EventLockdown, Dest: DestGroup, FileID: "sticker-1", MediaType: db.STICKER},
				reason: "invalid_media",
			},
			{
				name:   "locklift group sticker refused",
				msg:    Message{Event: EventLockLift, Dest: DestGroup, FileID: "sticker-1", MediaType: db.STICKER},
				reason: "invalid_media",
			},
			{
				name:   "javascript button URL",
				msg:    Message{Event: EventBan, Dest: DestGroup, Buttons: []models.Button{{Name: "X", Url: "javascript:alert(1)"}}},
				reason: "invalid_button_url",
			},
			{
				name:   "tg button URL",
				msg:    Message{Event: EventBan, Dest: DestGroup, Buttons: []models.Button{{Name: "X", Url: "tg://user?id=1"}}},
				reason: "invalid_button_url",
			},
			{
				name:   "file button URL",
				msg:    Message{Event: EventBan, Dest: DestGroup, Buttons: []models.Button{{Name: "X", Url: "file:///etc/passwd"}}},
				reason: "invalid_button_url",
			},
			{
				name:   "empty host button URL",
				msg:    Message{Event: EventBan, Dest: DestGroup, Buttons: []models.Button{{Name: "X", Url: "https://"}}},
				reason: "invalid_button_url",
			},
			{
				name:   "empty button name",
				msg:    Message{Event: EventBan, Dest: DestGroup, Buttons: []models.Button{{Name: "   ", Url: "https://example.com"}}},
				reason: "invalid_button",
			},
			{
				name:   "too many buttons including reserved on ban group (99 + 2 > 100)",
				msg:    Message{Event: EventBan, Dest: DestGroup, Buttons: manyRows(99)},
				reason: "buttons_limit",
			},
			{
				name:   "too many buttons including rules token + reserved on ban group (98 + 1 + 2 > 100)",
				msg:    Message{Event: EventBan, Dest: DestGroup, Text: "See {rules}", Buttons: manyRows(98)},
				reason: "buttons_limit",
			},
			{
				name:   "too many buttons in one row (9 > 8)",
				msg:    Message{Event: EventKick, Dest: DestGroup, Buttons: sameRow(9)},
				reason: "buttons_limit",
			},
			{
				name:   "too many buttons in one row with rules:same (8 + 1 > 8)",
				msg:    Message{Event: EventKick, Dest: DestGroup, Text: "{rules:same}", Buttons: sameRow(8)},
				reason: "buttons_limit",
			},
			{
				name:   "text over 4096",
				msg:    Message{Event: EventBan, Dest: DestGroup, Text: strings.Repeat("a", MaxTextLength+1)},
				reason: "text_too_long",
			},
			{
				name:   "caption over 1024",
				msg:    Message{Event: EventBan, Dest: DestGroup, Text: strings.Repeat("a", MaxCaptionLength+1), FileID: "f1", MediaType: db.PHOTO},
				reason: "text_too_long",
			},
			{
				name:   "mreq group text over 3496",
				msg:    Message{Event: EventReqBan, Dest: DestGroup, Text: strings.Repeat("a", MaxTextLength-MreqTextReserve+1)},
				reason: "text_too_long",
			},
			{
				name:   "lockdown group text over 3696",
				msg:    Message{Event: EventLockdown, Dest: DestGroup, Text: strings.Repeat("a", MaxTextLength-LockdownTextReserve+1)},
				reason: "text_too_long",
			},
			{
				name:   "lockdown group caption over 624",
				msg:    Message{Event: EventLockdown, Dest: DestGroup, Text: strings.Repeat("a", MaxCaptionLength-LockdownTextReserve+1), FileID: "f1", MediaType: db.PHOTO},
				reason: "text_too_long",
			},
			{
				name:   "UTF-16 surrogate pair exceeds 4096 visible units",
				msg:    Message{Event: EventKick, Dest: DestGroup, Text: strings.Repeat("😀", 2049)},
				reason: "text_too_long",
			},
		}

		for _, tc := range rejectCases {
			t.Run(tc.name, func(t *testing.T) {
				if _, reason := ValidateMessage(tc.msg); reason != tc.reason {
					t.Errorf("ValidateMessage(%s) reason = %q, want %q", tc.name, reason, tc.reason)
				}
			})
		}

		// Boundary accepts.
		acceptCases := []struct {
			name string
			msg  Message
		}{
			{
				name: "ban group 98 buttons (+ 2 reserved = 100)",
				msg:  Message{Event: EventBan, Dest: DestGroup, Buttons: manyRows(98)},
			},
			{
				name: "kick group 8 buttons in one row",
				msg:  Message{Event: EventKick, Dest: DestGroup, Buttons: sameRow(8)},
			},
			{
				name: "mreq group text exact 3496",
				msg:  Message{Event: EventReqBan, Dest: DestGroup, Text: strings.Repeat("a", MaxTextLength-MreqTextReserve)},
			},
			{
				name: "lockdown group text exact 3696",
				msg:  Message{Event: EventLockdown, Dest: DestGroup, Text: strings.Repeat("a", MaxTextLength-LockdownTextReserve)},
			},
			{
				name: "mreq PM text full 4096",
				msg:  Message{Event: EventReqBan, Dest: DestPM, Text: strings.Repeat("a", MaxTextLength)},
			},
			{
				name: "all allowed media kinds accepted on ban group",
				msg:  Message{Event: EventBan, Dest: DestGroup, Text: strings.Repeat("a", MaxCaptionLength), FileID: "f1", MediaType: db.ANIMATION},
			},
		}
		for _, tc := range acceptCases {
			t.Run(tc.name, func(t *testing.T) {
				if _, reason := ValidateMessage(tc.msg); reason != "" {
					t.Errorf("ValidateMessage(%s) unexpectedly refused: %q", tc.name, reason)
				}
			})
		}

		// NoSilent normalization on mreqmute|group.
		got, reason := ValidateMessage(Message{Event: EventReqMute, Dest: DestGroup, Text: "Fix it", Silent: true})
		if reason != "" {
			t.Fatalf("mreqmute valid row refused: %q", reason)
		}
		if got.Silent {
			t.Errorf("ValidateMessage(mreqmute|group, Silent=true) kept Silent=true, want false")
		}
	})

	t.Run("ReplaceAllValidated replaces atomically, skips invalid rows, clears on empty, and drops cache", func(t *testing.T) {
		utilscache.SetupTestMemoryMarshaler(t)
		chatID := freshChat(t)

		// Seed existing rows and options, then prime cache.
		if err := SetText(ctx, chatID, EventKick, DestGroup, "old kick", 1); err != nil {
			t.Fatalf("seed kick: %v", err)
		}
		if err := SetPMDetails(ctx, chatID, true, 1); err != nil {
			t.Fatalf("seed pm_details: %v", err)
		}
		if _, err := Get(ctx, chatID); err != nil {
			t.Fatalf("prime cache: %v", err)
		}

		// Replace with mix of valid, default-empty (omitted), duplicate (last wins), NoSilent, and invalid rows.
		input := []Message{
			{Event: EventBan, Dest: DestGroup, Text: "first ban"},
			{Event: EventBan, Dest: DestGroup, Text: "final ban", Buttons: []models.Button{{Name: "Appeal", Url: "https://example.com/a"}}},
			{Event: EventBan, Dest: DestPM, Text: "pm ban", Silent: false},
			{Event: EventReqMute, Dest: DestGroup, Text: "muted", Silent: true},
			// Default empty row should be omitted (no content, silent == default).
			{Event: EventMute, Dest: DestGroup, Silent: false},
			// Non-default silent-only row should be kept (ban group silenced).
			{Event: EventUnban, Dest: DestGroup, Silent: true},
			// Invalid rows must be skipped without aborting valid rows.
			{Event: EventWarn, Dest: DestGroup, Buttons: []models.Button{{Name: "Bad", Url: "javascript:alert(1)"}}},
			{Event: EventLockdown, Dest: DestPM, Text: "no pm"},
		}
		opts := Options{PMDetails: false, PMLog: true, PlainTone: true}
		rep, err := ReplaceAllValidated(ctx, chatID, input, opts, 42)
		if err != nil {
			t.Fatalf("ReplaceAllValidated: %v", err)
		}
		if rep.Written != 4 {
			t.Errorf("Written = %d, want 4", rep.Written)
		}
		if len(rep.Skipped) != 2 {
			t.Fatalf("Skipped = %+v, want 2 entries", rep.Skipped)
		}
		if rep.Skipped[0].Reason != "invalid_button_url" || rep.Skipped[1].Reason != "pm_not_supported" {
			t.Errorf("Skipped reasons = %+v", rep.Skipped)
		}

		// Cached Get must reflect the replacement (proving cache.DeleteCache ran).
		msgs, err := Get(ctx, chatID)
		if err != nil {
			t.Fatalf("Get after replace: %v", err)
		}
		if _, ok := msgs.Lookup(EventKick, DestGroup); ok {
			t.Errorf("old kick row survived replacement")
		}
		if _, ok := msgs.Lookup(EventMute, DestGroup); ok {
			t.Errorf("empty default mute row should not have been written")
		}
		if ban, ok := msgs.Lookup(EventBan, DestGroup); !ok || ban.Text != "final ban" || len(ban.Buttons) != 1 {
			t.Errorf("ban group = %+v, %v, want final ban with 1 button", ban, ok)
		}
		if pm, ok := msgs.Lookup(EventBan, DestPM); !ok || pm.Text != "pm ban" || pm.Silent {
			t.Errorf("ban pm = %+v, %v, want pm ban Silent=false", pm, ok)
		}
		if rm, ok := msgs.Lookup(EventReqMute, DestGroup); !ok || rm.Text != "muted" || rm.Silent {
			t.Errorf("mreqmute group = %+v, %v, want muted Silent=false", rm, ok)
		}
		if ub, ok := msgs.Lookup(EventUnban, DestGroup); !ok || !ub.Silent {
			t.Errorf("unban group = %+v, %v, want Silent=true", ub, ok)
		}
		if msgs.Options != opts {
			t.Errorf("Options = %+v, want %+v", msgs.Options, opts)
		}

		// Empty replace clears everything (messages AND options row) and drops cache.
		repEmpty, err := ReplaceAllValidated(ctx, chatID, nil, Options{}, 42)
		if err != nil {
			t.Fatalf("ReplaceAllValidated empty: %v", err)
		}
		if repEmpty.Written != 0 || len(repEmpty.Skipped) != 0 {
			t.Errorf("empty report = %+v, want zero", repEmpty)
		}
		cache.ResetLocalForTest()
		after, err := Get(ctx, chatID)
		if err != nil {
			t.Fatalf("Get after empty replace: %v", err)
		}
		if len(after.Rows) != 0 {
			t.Errorf("rows after empty replace = %d, want 0", len(after.Rows))
		}
		if after.Options != (Options{}) {
			t.Errorf("options after empty replace = %+v, want all false", after.Options)
		}
		var optCount int64
		if err := db.DB.Model(&models.ChatEventMessageOptions{}).Where("chat_id = ?", chatID).Count(&optCount).Error; err != nil {
			t.Fatalf("count options: %v", err)
		}
		if optCount != 0 {
			t.Errorf("chat_event_message_options row count = %d, want 0", optCount)
		}
	})
}

func TestReplaceAllValidatedTx(t *testing.T) {
	ctx := context.Background()

	t.Run("nil tx returns error", func(t *testing.T) {
		_, err := ReplaceAllValidatedTx(nil, -10012345, nil, Options{}, 1)
		if err == nil {
			t.Fatal("ReplaceAllValidatedTx(nil, ...) = nil, want error")
		}
		if !errors.Is(err, errNoDatabase) {
			t.Errorf("ReplaceAllValidatedTx(nil, ...) err = %v, want errors.Is(errNoDatabase)", err)
		}
	})

	t.Run("participates in outer transaction and rolls back completely on outer error", func(t *testing.T) {
		utilscache.SetupTestMemoryMarshaler(t)
		chatID := freshChat(t)

		if err := SetText(ctx, chatID, EventKick, DestGroup, "original kick", 1); err != nil {
			t.Fatalf("seed kick: %v", err)
		}
		if err := SetPMDetails(ctx, chatID, true, 1); err != nil {
			t.Fatalf("seed pm_details: %v", err)
		}
		if _, err := Get(ctx, chatID); err != nil {
			t.Fatalf("prime cache: %v", err)
		}

		candidate := []Message{
			{Event: EventBan, Dest: DestGroup, Text: "new ban"},
			{Event: EventWarn, Dest: DestGroup, Buttons: []models.Button{{Name: "Bad", Url: "javascript:alert(1)"}}},
		}
		newOpts := Options{PMLog: true, PlainTone: true}
		rollbackErr := errors.New("force outer rollback")

		var rollbackReport WriteReport
		err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			rollbackReport, txErr = ReplaceAllValidatedTx(tx, chatID, candidate, newOpts, 77)
			if txErr != nil {
				return txErr
			}
			return rollbackErr
		})
		if !errors.Is(err, rollbackErr) {
			t.Fatalf("outer rollback tx err = %v, want %v", err, rollbackErr)
		}
		if rollbackReport.Written != 1 {
			t.Errorf("rollbackReport.Written = %d, want 1", rollbackReport.Written)
		}
		if len(rollbackReport.Skipped) != 1 || rollbackReport.Skipped[0].Reason != "invalid_button_url" {
			t.Errorf("rollbackReport.Skipped = %+v, want 1 invalid_button_url", rollbackReport.Skipped)
		}

		// Fresh DB read must still contain the original data, proving complete rollback.
		afterRollback, err := GetFresh(ctx, chatID)
		if err != nil {
			t.Fatalf("GetFresh after rollback: %v", err)
		}
		if kick, ok := afterRollback.Lookup(EventKick, DestGroup); !ok || kick.Text != "original kick" {
			t.Errorf("after rollback kick = %+v, %v, want original kick", kick, ok)
		}
		if _, ok := afterRollback.Lookup(EventBan, DestGroup); ok {
			t.Errorf("rolled-back ban row leaked into DB")
		}
		if afterRollback.Options != (Options{PMDetails: true}) {
			t.Errorf("after rollback options = %+v, want {PMDetails:true}", afterRollback.Options)
		}

		// Now commit the outer transaction and verify ReplaceAllValidatedTx leaves cache invalidation to the caller.
		var commitReport WriteReport
		err = db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			commitReport, txErr = ReplaceAllValidatedTx(tx, chatID, candidate, newOpts, 77)
			return txErr
		})
		if err != nil {
			t.Fatalf("outer commit tx err = %v", err)
		}
		if commitReport.Written != 1 || len(commitReport.Skipped) != 1 {
			t.Errorf("commitReport = %+v, want Written=1 Skipped=1", commitReport)
		}

		// Before caller deletes cache, cached Get still returns pre-commit cached snapshot.
		cachedBeforeDelete, err := Get(ctx, chatID)
		if err != nil {
			t.Fatalf("Get before cache invalidation: %v", err)
		}
		if _, ok := cachedBeforeDelete.Lookup(EventKick, DestGroup); !ok {
			t.Errorf("expected cached pre-commit kick row before caller invalidates cache")
		}

		// Fresh read sees committed rows.
		afterCommit, err := GetFresh(ctx, chatID)
		if err != nil {
			t.Fatalf("GetFresh after commit: %v", err)
		}
		if _, ok := afterCommit.Lookup(EventKick, DestGroup); ok {
			t.Errorf("old kick row survived committed ReplaceAllValidatedTx")
		}
		if ban, ok := afterCommit.Lookup(EventBan, DestGroup); !ok || ban.Text != "new ban" {
			t.Errorf("after commit ban = %+v, %v, want new ban", ban, ok)
		}
		if afterCommit.Options != newOpts {
			t.Errorf("after commit options = %+v, want %+v", afterCommit.Options, newOpts)
		}

		// After caller invalidates cache, cached Get reflects committed state.
		cache.DeleteCache(CacheKey(chatID))
		cachedAfterDelete, err := Get(ctx, chatID)
		if err != nil {
			t.Fatalf("Get after cache invalidation: %v", err)
		}
		if ban, ok := cachedAfterDelete.Lookup(EventBan, DestGroup); !ok || ban.Text != "new ban" {
			t.Errorf("cached after invalidation ban = %+v, %v, want new ban", ban, ok)
		}
	})
}
