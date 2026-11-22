//go:build testtools

package modules

import (
	"context"
	"errors"
	"testing"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/escalation"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
)

func escalationApplyChat(t *testing.T) int64 {
	t.Helper()
	chatID := uniqueModuleChatID()
	escalationLadder(t, chatID)
	// escalationLadder turned it on with no step; start from a chat with no row.
	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEscalation{}).Error; err != nil {
		t.Fatal(err)
	}
	return chatID
}

func escalationStored(t *testing.T, chatID int64) *escalation.Ladder {
	t.Helper()
	ladder, err := escalation.GetFresh(context.Background(), chatID)
	if err != nil {
		t.Fatal(err)
	}
	return ladder
}

// TestEscalationApply holds the ladder's four write paths: each rule is checked against a fresh read,
// a refusal writes nothing, a failed write is a save failure carrying the ladder before it, and every
// applied result carries the ladder read back.
func TestEscalationApply(t *testing.T) {
	ctx := context.Background()

	t.Run("turning on a chat with no row stores the example ladder", func(t *testing.T) {
		chatID := escalationApplyChat(t)
		got := applyEscalationEnabled(ctx, chatID, true, 5)
		if got.Outcome != escalationExampleAdded || got.Ladder == nil || !got.Ladder.Enabled || len(got.Ladder.Steps) != 3 {
			t.Fatalf("applyEscalationEnabled(on) = %+v, want the example ladder", got)
		}
		if got.Ladder.Steps[0] != (escalation.Step{Action: "tmute", Secs: 3600}) || got.Ladder.Steps[2].Action != "ban" {
			t.Errorf("steps = %+v", got.Ladder.Steps)
		}
	})

	t.Run("turning on a ladder with steps stores only the switch", func(t *testing.T) {
		chatID := escalationApplyChat(t)
		if err := escalation.SetStep(ctx, chatID, 1, escalation.Step{Action: "kick"}, 1); err != nil {
			t.Fatal(err)
		}
		got := applyEscalationEnabled(ctx, chatID, true, 5)
		if got.Outcome != escalationApplied || !got.Ladder.Enabled || len(got.Ladder.Steps) != 1 || got.Ladder.Steps[0].Action != "kick" {
			t.Fatalf("applyEscalationEnabled(on) = %+v, want on with the one step kept", got)
		}
	})

	t.Run("turning off clears this chat's counts only", func(t *testing.T) {
		chatID, other := escalationApplyChat(t), escalationApplyChat(t)
		if r := applyEscalationEnabled(ctx, chatID, true, 5); r.Outcome != escalationExampleAdded {
			t.Fatal(r)
		}
		for _, id := range []int64{chatID, other} {
			if _, err := escalation.CountOffence(ctx, id, 7, punishNow(), 24*3600*1e9); err != nil {
				t.Fatal(err)
			}
		}
		t.Cleanup(func() { db.DB.Where("chat_id IN ?", []int64{chatID, other}).Delete(&models.ChatOffence{}) })
		got := applyEscalationEnabled(ctx, chatID, false, 5)
		if got.Outcome != escalationApplied || got.Ladder.Enabled {
			t.Fatalf("applyEscalationEnabled(off) = %+v", got)
		}
		if rows := escalationOffenceRows(t, chatID); len(rows) != 0 {
			t.Errorf("counts after off = %+v, want none", rows)
		}
		if rows := escalationOffenceRows(t, other); len(rows) != 1 {
			t.Errorf("another chat's counts = %+v, want untouched", rows)
		}
	})

	t.Run("steps replace, append and go stale", func(t *testing.T) {
		chatID := escalationApplyChat(t)
		if err := escalation.SetEnabledWithSteps(ctx, chatID, escalationExampleSteps[:2], 1); err != nil {
			t.Fatal(err)
		}
		if got := applyEscalationStep(ctx, chatID, 2, escalation.Step{Action: "ban"}, 5); got.Outcome != escalationApplied || got.Ladder.Steps[1].Action != "ban" {
			t.Fatalf("replace step 2 = %+v", got)
		}
		if got := applyEscalationStep(ctx, chatID, 3, escalation.Step{Action: "tmute", Secs: 21600}, 5); got.Outcome != escalationApplied || len(got.Ladder.Steps) != 3 {
			t.Fatalf("append step 3 = %+v", got)
		}
		if got := applyEscalationStep(ctx, chatID, 5, escalation.Step{Action: "ban"}, 5); got.Outcome != escalationStale {
			t.Fatalf("step 5 on a three-step ladder = %+v, want stale", got)
		}
		if len(escalationStored(t, chatID).Steps) != 3 {
			t.Errorf("a stale step wrote something")
		}
		for _, bad := range []struct {
			n int
			s escalation.Step
		}{{1, escalation.Step{Action: "warn"}}, {1, escalation.Step{Action: "tmute", Secs: 59}}, {1, escalation.Step{Action: "ban", Secs: 60}}, {0, escalation.Step{Action: "ban"}}, {6, escalation.Step{Action: "ban"}}} {
			if got := applyEscalationStep(ctx, chatID, bad.n, bad.s, 5); got.Outcome != escalationInvalid {
				t.Errorf("applyEscalationStep(%d, %+v) = %+v, want invalid", bad.n, bad.s, got)
			}
		}
	})

	t.Run("remove clears the last step only", func(t *testing.T) {
		chatID := escalationApplyChat(t)
		if r := applyEscalationEnabled(ctx, chatID, true, 5); r.Outcome != escalationExampleAdded {
			t.Fatal(r)
		}
		if got := applyEscalationRemoveStep(ctx, chatID, 2, 5); got.Outcome != escalationStale {
			t.Fatalf("remove step 2 of 3 = %+v, want stale", got)
		}
		if got := applyEscalationRemoveStep(ctx, chatID, 3, 5); got.Outcome != escalationApplied || len(got.Ladder.Steps) != 2 {
			t.Fatalf("remove step 3 = %+v", got)
		}
		if got := applyEscalationRemoveStep(ctx, chatID, 2, 5); got.Outcome != escalationApplied || len(got.Ladder.Steps) != 1 {
			t.Fatalf("remove step 2 = %+v", got)
		}
		if got := applyEscalationRemoveStep(ctx, chatID, 1, 5); got.Outcome != escalationLastStepRefused || len(got.Ladder.Steps) != 1 {
			t.Fatalf("remove the only step of a ladder that is on = %+v, want refused", got)
		}
		if r := applyEscalationEnabled(ctx, chatID, false, 5); r.Outcome != escalationApplied {
			t.Fatal(r)
		}
		if got := applyEscalationRemoveStep(ctx, chatID, 1, 5); got.Outcome != escalationApplied || len(got.Ladder.Steps) != 0 {
			t.Fatalf("remove the only step of a ladder that is off = %+v, want cleared", got)
		}
	})

	t.Run("forget takes the presets", func(t *testing.T) {
		chatID := escalationApplyChat(t)
		if got := applyEscalationForget(ctx, chatID, 7, 5); got.Outcome != escalationApplied || got.Ladder.ForgetDays != 7 {
			t.Fatalf("applyEscalationForget(7) = %+v", got)
		}
		if got := applyEscalationForget(ctx, chatID, 2, 5); got.Outcome != escalationInvalid {
			t.Fatalf("applyEscalationForget(2) = %+v, want invalid", got)
		}
		if escalationStored(t, chatID).ForgetDays != 7 {
			t.Error("an invalid value wrote something")
		}
	})

	t.Run("a failed write is a save failure with the ladder before it", func(t *testing.T) {
		chatID := escalationApplyChat(t)
		if err := escalation.SetEnabledWithSteps(ctx, chatID, escalationExampleSteps[:1], 1); err != nil {
			t.Fatal(err)
		}
		mreqSwap(t, &escalationWrites.SetStep, func(context.Context, int64, int, escalation.Step, int64) error {
			return errors.New("boom")
		})
		mreqSwap(t, &escalationWrites.SetForgetDays, func(context.Context, int64, int, int64) error { return errors.New("boom") })
		for name, got := range map[string]escalationApplyResult{
			"step":   applyEscalationStep(ctx, chatID, 2, escalation.Step{Action: "ban"}, 5),
			"remove": applyEscalationRemoveStep(ctx, chatID, 1, 5),
			"forget": applyEscalationForget(ctx, chatID, 7, 5),
		} {
			if name == "remove" {
				// The only step of a ladder that is on is refused before any write.
				if got.Outcome != escalationLastStepRefused {
					t.Errorf("remove = %+v", got)
				}
				continue
			}
			if got.Outcome != escalationSaveFailed || got.Err == nil || got.Ladder == nil || len(got.Ladder.Steps) != 1 {
				t.Errorf("%s = %+v, want a save failure carrying the ladder before", name, got)
			}
		}
	})
}
