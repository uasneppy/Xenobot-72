//go:build testtools

package escalation

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uasneppy/fuku_robot/fuku/db"
	"github.com/uasneppy/fuku_robot/fuku/db/cache"
	"github.com/uasneppy/fuku_robot/fuku/db/models"
	utilsCache "github.com/uasneppy/fuku_robot/fuku/utils/cache"
)

var chatSeq atomic.Int64

// escalationChat returns a fresh chat ID whose ladder row, offence rows and cached ladder are
// removed when the test ends. It installs an in-memory cache, so a test can tell a cached read
// from a fresh one.
func escalationChat(t *testing.T) int64 {
	t.Helper()
	utilsCache.SetupTestMemoryMarshaler(t)
	chatID := -time.Now().UnixNano() - chatSeq.Add(1)
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEscalation{})
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatOffence{})
		cache.DeleteCache(CacheKey(chatID))
		cache.ResetLocalForTest()
	})
	return chatID
}

func ladderRows(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n int64
	if err := db.DB.Model(&models.ChatEscalation{}).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func storedLadder(t *testing.T, chatID int64) models.ChatEscalation {
	t.Helper()
	var row models.ChatEscalation
	if err := db.DB.Where("chat_id = ?", chatID).Take(&row).Error; err != nil {
		t.Fatalf("read the ladder row: %v", err)
	}
	return row
}

func offenceRows(t *testing.T, chatID int64) []models.ChatOffence {
	t.Helper()
	var rows []models.ChatOffence
	if err := db.DB.Where("chat_id = ?", chatID).Order("user_id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

func mustGet(t *testing.T, chatID int64) *Ladder {
	t.Helper()
	got, err := Get(context.Background(), chatID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return got
}

func TestEscalationSettings(t *testing.T) {
	ctx := context.Background()

	t.Run("a chat with no row reads off, no steps, 30 days and creates nothing", func(t *testing.T) {
		chatID := escalationChat(t)
		got := mustGet(t, chatID)
		if got.Enabled || len(got.Steps) != 0 || got.ForgetDays != DefaultForgetDays {
			t.Fatalf("Get = %+v, want off, no steps, %d days", got, DefaultForgetDays)
		}
		fresh, err := GetFresh(ctx, chatID)
		if err != nil || !reflect.DeepEqual(fresh, got) {
			t.Fatalf("GetFresh = %+v, %v", fresh, err)
		}
		if ladderRows(t, chatID) != 0 {
			t.Fatal("a read created a row")
		}
	})

	t.Run("two writers of two steps leave both", func(t *testing.T) {
		chatID := escalationChat(t)
		mustGet(t, chatID) // fill the cache
		if err := SetStep(ctx, chatID, 1, Step{Action: ActionTMute, Secs: 3600}, 7); err != nil {
			t.Fatal(err)
		}
		if err := SetStep(ctx, chatID, 2, Step{Action: ActionBan}, 8); err != nil {
			t.Fatal(err)
		}
		want := []Step{{Action: ActionTMute, Secs: 3600}, {Action: ActionBan}}
		got := mustGet(t, chatID)
		if got.Enabled || !reflect.DeepEqual(got.Steps, want) || got.ForgetDays != DefaultForgetDays {
			t.Fatalf("Get = %+v, want off, %+v, %d days", got, want, DefaultForgetDays)
		}
		if row := storedLadder(t, chatID); row.UpdatedBy != 8 {
			t.Errorf("updated_by = %d, want the last writer 8", row.UpdatedBy)
		}

		// Concurrent writers of different steps never clobber each other.
		chatID = escalationChat(t)
		if err := SetForgetDays(ctx, chatID, 7, 1); err != nil { // the row exists, so every write below is an update
			t.Fatal(err)
		}
		steps := []Step{{Action: ActionMute}, {Action: ActionTMute, Secs: 86400}, {Action: ActionKick}, {Action: ActionTBan, Secs: 604800}, {Action: ActionBan}}
		var wg sync.WaitGroup
		errs := make([]error, MaxSteps)
		for i := range steps {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				errs[n] = SetStep(ctx, chatID, n+1, steps[n], int64(n+10))
			}(i)
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				// SQLite answers "database is locked" to a concurrent writer where PostgreSQL waits;
				// write the step again alone so the merge is still checked.
				if db.DB.Name() == "postgres" {
					t.Fatalf("concurrent SetStep %d: %v", i+1, err)
				}
				if err := SetStep(ctx, chatID, i+1, steps[i], int64(i+10)); err != nil {
					t.Fatalf("SetStep %d: %v", i+1, err)
				}
			}
		}
		got = mustGet(t, chatID)
		if !reflect.DeepEqual(got.Steps, steps) || got.ForgetDays != 7 {
			t.Fatalf("after concurrent writes: %+v", got)
		}
	})

	t.Run("SetEnabledWithSteps turns on and clears the steps it does not name", func(t *testing.T) {
		chatID := escalationChat(t)
		for n := 1; n <= MaxSteps; n++ {
			if err := SetStep(ctx, chatID, n, Step{Action: ActionKick}, 1); err != nil {
				t.Fatal(err)
			}
		}
		if err := SetForgetDays(ctx, chatID, 90, 1); err != nil {
			t.Fatal(err)
		}
		mustGet(t, chatID) // fill the cache
		steps := []Step{{Action: ActionTMute, Secs: 3600}, {Action: ActionTMute, Secs: 86400}, {Action: ActionBan}}
		if err := SetEnabledWithSteps(ctx, chatID, steps, 5); err != nil {
			t.Fatal(err)
		}
		got := mustGet(t, chatID)
		if !got.Enabled || !reflect.DeepEqual(got.Steps, steps) || got.ForgetDays != 90 {
			t.Fatalf("Get = %+v, want on, %+v, 90 days", got, steps)
		}
		row := storedLadder(t, chatID)
		if row.Step4Action != "" || row.Step4Secs != 0 || row.Step5Action != "" || row.Step5Secs != 0 {
			t.Errorf("steps 4 and 5 = %q %d, %q %d, want cleared", row.Step4Action, row.Step4Secs, row.Step5Action, row.Step5Secs)
		}

		// A Go false is written, and the steps stay.
		if err := SetEnabled(ctx, chatID, false, 6); err != nil {
			t.Fatal(err)
		}
		if got := mustGet(t, chatID); got.Enabled || !reflect.DeepEqual(got.Steps, steps) {
			t.Fatalf("after off: %+v", got)
		}
	})

	t.Run("steps stop at the first empty step", func(t *testing.T) {
		chatID := escalationChat(t)
		if err := SetEnabledWithSteps(ctx, chatID, []Step{{Action: ActionMute}, {Action: ActionKick}, {Action: ActionBan}}, 1); err != nil {
			t.Fatal(err)
		}
		if err := SetStep(ctx, chatID, 2, Step{}, 1); err != nil { // an empty step clears it
			t.Fatal(err)
		}
		row := storedLadder(t, chatID)
		if row.Step2Action != "" || row.Step3Action != ActionBan {
			t.Fatalf("stored steps 2 and 3 = %q, %q", row.Step2Action, row.Step3Action)
		}
		if got := mustGet(t, chatID); !reflect.DeepEqual(got.Steps, []Step{{Action: ActionMute}}) {
			t.Fatalf("Steps = %+v, want step 1 alone", got.Steps)
		}
	})

	t.Run("an invalid value writes nothing", func(t *testing.T) {
		chatID := escalationChat(t)
		invalid := []Step{
			{Action: "warn"}, {Action: ActionTMute}, {Action: ActionTMute, Secs: 59}, {Action: ActionTBan, Secs: models.PunishMaxSecs + 1},
			{Action: ActionBan, Secs: 60}, {Action: ActionMute, Secs: 3600}, {Action: "", Secs: 60}, {Action: "Ban"},
		}
		for _, s := range invalid {
			if err := SetStep(ctx, chatID, 1, s, 1); !errors.Is(err, ErrInvalidValue) {
				t.Errorf("SetStep(1, %+v) = %v, want ErrInvalidValue", s, err)
			}
			if err := SetEnabledWithSteps(ctx, chatID, []Step{{Action: ActionMute}, s}, 1); !errors.Is(err, ErrInvalidValue) {
				t.Errorf("SetEnabledWithSteps(..., %+v) = %v, want ErrInvalidValue", s, err)
			}
		}
		for _, n := range []int{0, 6, -1} {
			if err := SetStep(ctx, chatID, n, Step{Action: ActionBan}, 1); !errors.Is(err, ErrInvalidValue) {
				t.Errorf("SetStep(%d) = %v, want ErrInvalidValue", n, err)
			}
		}
		six := make([]Step, MaxSteps+1)
		for i := range six {
			six[i] = Step{Action: ActionBan}
		}
		if err := SetEnabledWithSteps(ctx, chatID, six, 1); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("six steps = %v, want ErrInvalidValue", err)
		}
		for _, days := range []int{0, -1, 366} {
			if err := SetForgetDays(ctx, chatID, days, 1); !errors.Is(err, ErrInvalidValue) {
				t.Errorf("SetForgetDays(%d) = %v, want ErrInvalidValue", days, err)
			}
		}
		if ladderRows(t, chatID) != 0 {
			t.Fatal("an invalid value wrote a row")
		}
		for _, s := range []Step{{Action: ActionMute}, {Action: ActionTMute, Secs: 60}, {Action: ActionKick}, {Action: ActionTBan, Secs: models.PunishMaxSecs}, {Action: ActionBan}} {
			if !ValidStep(s) {
				t.Errorf("ValidStep(%+v) = false", s)
			}
		}
	})

	t.Run("every write drops the key and GetFresh bypasses the cache", func(t *testing.T) {
		chatID := escalationChat(t)
		writes := []struct {
			name  string
			write func() error
			check func(*Ladder) bool
		}{
			{"SetStep", func() error { return SetStep(ctx, chatID, 1, Step{Action: ActionKick}, 1) },
				func(l *Ladder) bool { return len(l.Steps) == 1 && l.Steps[0].Action == ActionKick }},
			{"SetEnabled", func() error { return SetEnabled(ctx, chatID, true, 1) }, func(l *Ladder) bool { return l.Enabled }},
			{"SetForgetDays", func() error { return SetForgetDays(ctx, chatID, 1, 1) }, func(l *Ladder) bool { return l.ForgetDays == 1 }},
			{"SetEnabledWithSteps", func() error {
				return SetEnabledWithSteps(ctx, chatID, []Step{{Action: ActionBan}, {Action: ActionBan}}, 1)
			},
				func(l *Ladder) bool { return len(l.Steps) == 2 }},
		}
		for _, w := range writes {
			mustGet(t, chatID) // fill the cache
			if err := w.write(); err != nil {
				t.Fatalf("%s: %v", w.name, err)
			}
			if got := mustGet(t, chatID); !w.check(got) {
				t.Errorf("after %s the cached read is %+v: the key was not dropped", w.name, got)
			}
		}

		// A change outside the repository is not seen through the cache until the key is dropped,
		// and GetFresh sees it at once.
		mustGet(t, chatID)
		if err := db.DB.Exec("UPDATE chat_escalation SET forget_days = 90 WHERE chat_id = ?", chatID).Error; err != nil {
			t.Fatal(err)
		}
		if got := mustGet(t, chatID); got.ForgetDays != 1 {
			t.Errorf("cached forget_days = %d, want the cached 1", got.ForgetDays)
		}
		fresh, err := GetFresh(ctx, chatID)
		if err != nil || fresh.ForgetDays != 90 {
			t.Errorf("GetFresh = %+v, %v, want 90 days", fresh, err)
		}
	})

	t.Run("the table's checks refuse values outside the bounds", func(t *testing.T) {
		chatID := escalationChat(t)
		for _, row := range []*models.ChatEscalation{
			{ChatId: chatID, Step1Action: "warn", ForgetDays: 30},
			{ChatId: chatID, Step2Secs: models.PunishMaxSecs + 1, ForgetDays: 30},
			{ChatId: chatID, ForgetDays: 0},
			{ChatId: chatID, ForgetDays: 366},
		} {
			if err := db.DB.Create(row).Error; err == nil {
				t.Errorf("a row outside the checks was stored: %+v", row)
				db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatEscalation{})
			}
		}
		if err := db.DB.Create(&models.ChatOffence{ChatId: chatID, UserId: 1, Offences: -1}).Error; err == nil {
			t.Error("a negative count was stored")
		}
	})
}

func TestOffenceCountRules(t *testing.T) {
	ctx := context.Background()
	const forget = 30 * 24 * time.Hour
	start := time.Unix(1_800_000_000, 0)

	t.Run("counts, the same-offence window and the forget period", func(t *testing.T) {
		chatID := escalationChat(t)
		const userID = int64(41)
		steps := []struct {
			at       time.Time
			want     int
			wantLast time.Time
		}{
			{start, 1, start},
			{start.Add(10 * time.Second), 1, start}, // the same offence: the time does not move
			{start.Add(61 * time.Second), 2, start.Add(61 * time.Second)},
			{start.Add(100 * time.Second), 2, start.Add(61 * time.Second)},
			{start.Add(200 * time.Second), 3, start.Add(200 * time.Second)},
			{start.Add(200*time.Second + 31*24*time.Hour), 1, start.Add(200*time.Second + 31*24*time.Hour)},
		}
		for i, s := range steps {
			got, err := CountOffence(ctx, chatID, userID, s.at, forget)
			if err != nil {
				t.Fatalf("offence %d: %v", i+1, err)
			}
			if got != s.want {
				t.Errorf("offence %d at +%v = %d, want %d", i+1, s.at.Sub(start), got, s.want)
			}
			rows := offenceRows(t, chatID)
			if len(rows) != 1 || rows[0].Offences != s.want || rows[0].LastOffenceUnix != s.wantLast.Unix() {
				t.Fatalf("offence %d: rows %+v, want one row of %d at %d", i+1, rows, s.want, s.wantLast.Unix())
			}
		}
	})

	t.Run("another member and another chat count apart, and ClearOffences deletes the chat's rows only", func(t *testing.T) {
		chatID := escalationChat(t)
		other := escalationChat(t)
		for _, c := range []struct {
			chat, user int64
			at         time.Duration
			want       int
		}{
			{chatID, 1, 0, 1},
			{chatID, 2, 0, 1},
			{other, 1, 0, 1},
			{chatID, 1, 2 * time.Minute, 2},
			{chatID, 2, 2 * time.Minute, 2},
			{chatID, 1, 4 * time.Minute, 3},
			{other, 1, 4 * time.Minute, 2},
		} {
			got, err := CountOffence(ctx, c.chat, c.user, start.Add(c.at), forget)
			if err != nil || got != c.want {
				t.Fatalf("chat %d user %d at +%v = %d, %v, want %d", c.chat, c.user, c.at, got, err, c.want)
			}
		}
		if err := ClearOffences(ctx, chatID); err != nil {
			t.Fatal(err)
		}
		if rows := offenceRows(t, chatID); len(rows) != 0 {
			t.Fatalf("rows left in the cleared chat: %+v", rows)
		}
		if rows := offenceRows(t, other); len(rows) != 1 || rows[0].Offences != 2 {
			t.Fatalf("the other chat's rows = %+v, want one row of 2", rows)
		}
		if got, err := CountOffence(ctx, chatID, 1, start.Add(5*time.Minute), forget); err != nil || got != 1 {
			t.Fatalf("after clearing = %d, %v, want a fresh 1", got, err)
		}
	})

	t.Run("an invalid call writes nothing", func(t *testing.T) {
		chatID := escalationChat(t)
		for _, c := range []struct {
			chat, user int64
			forget     time.Duration
		}{
			{chatID, 0, forget}, {chatID, -5, forget}, {0, 7, forget}, {chatID, 7, 0}, {chatID, 7, -time.Hour},
		} {
			if _, err := CountOffence(ctx, c.chat, c.user, start, c.forget); !errors.Is(err, ErrInvalidValue) {
				t.Errorf("CountOffence(%d, %d, %v) = %v, want ErrInvalidValue", c.chat, c.user, c.forget, err)
			}
		}
		var n int64
		if err := db.DB.Model(&models.ChatOffence{}).Where("chat_id IN ?", []int64{chatID, 0}).Count(&n).Error; err != nil || n != 0 {
			t.Fatalf("rows = %d, %v, want none", n, err)
		}
	})
}

// TestOffenceCountConcurrent proves that concurrent offences of one member at one instant, on any
// number of replicas, are one offence: PostgreSQL's row lock makes each upsert wait for the one
// before it. SQLite answers "database is locked" to a concurrent writer where PostgreSQL waits,
// so the concurrent case is registered on PostgreSQL only (make test-postgres-integrity), while
// TestOffenceCountRules proves the rules on both databases.
func TestOffenceCountConcurrent(t *testing.T) {
	ctx := context.Background()
	const workers = 16
	if db.DB.Name() != "postgres" {
		return
	}
	t.Run("sixteen offences at one instant count once", func(t *testing.T) {
		chatID := escalationChat(t)
		const userID = int64(77)
		now := time.Unix(1_800_000_000, 0)
		results := make([]int, workers)
		errs := make([]error, workers)
		var ready, wg sync.WaitGroup
		gate := make(chan struct{})
		for i := 0; i < workers; i++ {
			ready.Add(1)
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				ready.Done()
				<-gate
				results[i], errs[i] = CountOffence(ctx, chatID, userID, now, 30*24*time.Hour)
			}(i)
		}
		ready.Wait()
		close(gate)
		wg.Wait()
		for i := range results {
			if errs[i] != nil {
				t.Fatalf("worker %d: %v", i, errs[i])
			}
			if results[i] != 1 {
				t.Errorf("worker %d counted %d, want 1", i, results[i])
			}
		}
		rows := offenceRows(t, chatID)
		if len(rows) != 1 || rows[0].Offences != 1 || rows[0].LastOffenceUnix != now.Unix() {
			t.Fatalf("rows = %+v, want one row of 1", rows)
		}
	})
}
