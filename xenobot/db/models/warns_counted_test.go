package models

import (
	"reflect"
	"testing"
	"time"
)

// TestWarnsCounted holds the one counting rule of warn expiry: every warn with expiry off, the warns
// newer than the expiry with it on, old rows dated at their last change, a previous build's cached
// value never expiring, and the times always one per warn and non-decreasing.
func TestWarnsCounted(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	day := int64(24 * 60 * 60)
	ago := func(days int64) int64 { return now.Unix() - days*day }

	t.Run("expiry off counts every warn and reason", func(t *testing.T) {
		w := &Warns{NumWarns: 3, Reasons: StringArray{"a", "b", "c"}, WarnTimes: Int64Array{ago(40), ago(30), ago(20)}, UpdatedAt: now}
		count, reasons, times := w.Counted(0, now)
		if count != 3 || !reflect.DeepEqual(reasons, []string{"a", "b", "c"}) || len(times) != 3 {
			t.Fatalf("Counted(0) = %d %v %v, want 3 [a b c] and three times", count, reasons, times)
		}
	})

	t.Run("expiry on counts the newer suffix", func(t *testing.T) {
		w := &Warns{NumWarns: 3, Reasons: StringArray{"old1", "old2", "new"}, WarnTimes: Int64Array{ago(9), ago(8), ago(1)}, UpdatedAt: now}
		count, reasons, times := w.Counted(7, now)
		if count != 1 || !reflect.DeepEqual(reasons, []string{"new"}) || !reflect.DeepEqual(times, []int64{ago(1)}) {
			t.Fatalf("Counted(7) = %d %v %v, want 1 [new] [%d]", count, reasons, times, ago(1))
		}
	})

	t.Run("an old row is dated at its last change", func(t *testing.T) {
		w := &Warns{NumWarns: 3, Reasons: StringArray{"a", "b", "c"}, UpdatedAt: time.Unix(ago(10), 0)}
		if got := w.NormalisedTimes(); !reflect.DeepEqual(got, []int64{ago(10), ago(10), ago(10)}) {
			t.Fatalf("NormalisedTimes() = %v, want three times at the row's update time", got)
		}
		if count, reasons, _ := w.Counted(7, now); count != 0 || len(reasons) != 0 {
			t.Errorf("Counted(7) = %d %v, want 0 and no reason (dated 10 days ago)", count, reasons)
		}
		if count, _, _ := w.Counted(30, now); count != 3 {
			t.Errorf("Counted(30) = %d, want 3", count)
		}
	})

	t.Run("missing times are padded with the earlier of the update time and the first time", func(t *testing.T) {
		w := &Warns{NumWarns: 3, Reasons: StringArray{"a", "b", "c"}, WarnTimes: Int64Array{ago(5)}, UpdatedAt: time.Unix(ago(2), 0)}
		if got := w.NormalisedTimes(); !reflect.DeepEqual(got, []int64{ago(5), ago(5), ago(5)}) {
			t.Fatalf("NormalisedTimes() = %v, want the first stored time as the padding", got)
		}
	})

	t.Run("a previous build's cached value counts every warn", func(t *testing.T) {
		w := &Warns{NumWarns: 2, Reasons: StringArray{"a", "b"}}
		if count, reasons, _ := w.Counted(1, now); count != 2 || !reflect.DeepEqual(reasons, []string{"a", "b"}) {
			t.Fatalf("Counted(1) of a value with no update time = %d %v, want 2 [a b]", count, reasons)
		}
	})

	t.Run("extra times are cut from the end", func(t *testing.T) {
		w := &Warns{NumWarns: 2, Reasons: StringArray{"a", "b"}, WarnTimes: Int64Array{ago(3), ago(2), ago(1)}, UpdatedAt: now}
		if got := w.NormalisedTimes(); !reflect.DeepEqual(got, []int64{ago(3), ago(2)}) {
			t.Fatalf("NormalisedTimes() = %v, want the first two times", got)
		}
	})

	t.Run("decreasing times never go back", func(t *testing.T) {
		w := &Warns{NumWarns: 3, WarnTimes: Int64Array{ago(1), ago(9), ago(2)}, UpdatedAt: now}
		if got := w.NormalisedTimes(); !reflect.DeepEqual(got, []int64{ago(1), ago(1), ago(1)}) {
			t.Fatalf("NormalisedTimes() = %v, want each time at least the one before it", got)
		}
	})

	t.Run("warns without reasons", func(t *testing.T) {
		w := &Warns{NumWarns: 2, UpdatedAt: now}
		count, reasons, times := w.Counted(7, now)
		if count != 2 || len(reasons) != 0 || len(times) != 2 {
			t.Fatalf("Counted(7) = %d %v %v, want 2, no reason, two times", count, reasons, times)
		}
		if count, reasons, _ := w.Counted(0, now); count != 2 || len(reasons) != 0 {
			t.Errorf("Counted(0) = %d %v, want 2 and no reason", count, reasons)
		}
	})

	t.Run("fewer reasons than counted warns", func(t *testing.T) {
		w := &Warns{NumWarns: 3, Reasons: StringArray{"only"}, WarnTimes: Int64Array{ago(9), ago(2), ago(1)}, UpdatedAt: now}
		count, reasons, _ := w.Counted(7, now)
		if count != 2 || !reflect.DeepEqual(reasons, []string{"only"}) {
			t.Fatalf("Counted(7) = %d %v, want 2 [only]", count, reasons)
		}
	})

	t.Run("no warn", func(t *testing.T) {
		var nilWarns *Warns
		if count, _, times := nilWarns.Counted(7, now); count != 0 || len(times) != 0 {
			t.Errorf("nil Counted = %d %v, want nothing", count, times)
		}
		if got := (&Warns{}).NormalisedTimes(); len(got) != 0 {
			t.Errorf("NormalisedTimes of no warn = %v, want none", got)
		}
	})
}
