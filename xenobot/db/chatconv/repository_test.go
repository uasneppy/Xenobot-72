//go:build testtools

package chatconv

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/cache"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
)

func newChat(t *testing.T) int64 {
	t.Helper()
	chatID := -time.Now().UnixNano()
	t.Cleanup(func() {
		db.DB.Where("chat_id = ?", chatID).Delete(&models.ChatConversation{})
		cache.DeleteCache(CacheKey(chatID))
	})
	return chatID
}

func rowCount(t *testing.T, chatID int64) int64 {
	t.Helper()
	var n int64
	if err := db.DB.Model(&models.ChatConversation{}).Where("chat_id = ?", chatID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestChatConversationSettings(t *testing.T) {
	ctx := context.Background()

	t.Run("a chat with no row reads the defaults and creates nothing", func(t *testing.T) {
		chatID := newChat(t)
		got, err := Get(ctx, chatID)
		if err != nil || got != Defaults() {
			t.Fatalf("Get = %+v, %v", got, err)
		}
		if rowCount(t, chatID) != 0 {
			t.Fatal("a read created a row")
		}
	})

	t.Run("each write changes its own column and drops the key", func(t *testing.T) {
		chatID := newChat(t)
		if _, err := Get(ctx, chatID); err != nil { // fill the cache
			t.Fatal(err)
		}
		if err := SetProvider(ctx, chatID, ProviderGoogle, 7); err != nil {
			t.Fatal(err)
		}
		if got, _ := Get(ctx, chatID); got.Provider != ProviderGoogle || got.Enabled {
			t.Fatalf("after SetProvider(google): %+v", got)
		}
		if err := SetProvider(ctx, chatID, ProviderFeatherless, 7); err != nil {
			t.Fatal(err)
		}
		if got, _ := Get(ctx, chatID); got.Provider != ProviderFeatherless || got.Enabled {
			t.Fatalf("after SetProvider(featherless): %+v", got)
		}
		if err := SetProvider(ctx, chatID, ProviderGoogle, 7); err != nil {
			t.Fatal(err)
		}
		if err := SetEnabled(ctx, chatID, true, 7); err != nil {
			t.Fatal(err)
		}
		if err := SetAudience(ctx, chatID, AudienceAdminsApproved, 8); err != nil {
			t.Fatal(err)
		}
		got, err := Get(ctx, chatID)
		want := Settings{Enabled: true, Provider: ProviderGoogle, Audience: AudienceAdminsApproved}
		if err != nil || got != want {
			t.Fatalf("Get = %+v, %v, want %+v", got, err, want)
		}
		if fresh, _ := GetFresh(ctx, chatID); fresh != want {
			t.Fatalf("GetFresh = %+v", fresh)
		}

		// A Go false is written.
		if err := SetEnabled(ctx, chatID, false, 9); err != nil {
			t.Fatal(err)
		}
		if got, _ := Get(ctx, chatID); got.Enabled || got.Provider != ProviderGoogle || got.Audience != AudienceAdminsApproved {
			t.Fatalf("after off: %+v", got)
		}
		var row models.ChatConversation
		db.DB.Where("chat_id = ?", chatID).Take(&row)
		if row.UpdatedBy != 9 {
			t.Errorf("updated_by = %d", row.UpdatedBy)
		}
	})

	t.Run("the message log is its own column", func(t *testing.T) {
		chatID := newChat(t)
		if got, _ := Get(ctx, chatID); got.LogEnabled {
			t.Fatal("a chat with no row reads the log on")
		}
		if err := SetProvider(ctx, chatID, ProviderGoogle, 7); err != nil {
			t.Fatal(err)
		}
		if err := SetAudience(ctx, chatID, AudienceAdminsApproved, 7); err != nil {
			t.Fatal(err)
		}
		if err := SetLogEnabled(ctx, chatID, true, 8); err != nil {
			t.Fatal(err)
		}
		want := Settings{Provider: ProviderGoogle, Audience: AudienceAdminsApproved, LogEnabled: true}
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after SetLogEnabled(true): %+v, %v", got, err)
		}
		// A Go false is written and the other columns stay.
		if err := SetLogEnabled(ctx, chatID, false, 9); err != nil {
			t.Fatal(err)
		}
		want.LogEnabled = false
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after SetLogEnabled(false): %+v, %v", got, err)
		}
		// Writing the other controls leaves the log alone.
		if err := SetLogEnabled(ctx, chatID, true, 9); err != nil {
			t.Fatal(err)
		}
		if err := SetEnabled(ctx, chatID, true, 9); err != nil {
			t.Fatal(err)
		}
		if got, _ := GetFresh(ctx, chatID); !got.LogEnabled || !got.Enabled {
			t.Fatalf("SetEnabled clobbered the log: %+v", got)
		}
		var row models.ChatConversation
		db.DB.Where("chat_id = ?", chatID).Take(&row)
		if row.JudgeTo != 0 {
			t.Errorf("judge_to = %d", row.JudgeTo)
		}
	})

	t.Run("the judge destination is its own column", func(t *testing.T) {
		chatID := newChat(t)
		if got, err := Get(ctx, chatID); err != nil || got.JudgeTo != JudgeToGroup {
			t.Fatalf("a chat with no row reads judge_to %+v, %v", got, err)
		}
		if err := SetProvider(ctx, chatID, ProviderGoogle, 7); err != nil {
			t.Fatal(err)
		}
		if err := SetLogEnabled(ctx, chatID, true, 7); err != nil {
			t.Fatal(err)
		}
		for _, to := range []int{JudgeToStaff, JudgeToPrivate, JudgeToGroup} {
			if err := SetJudgeTo(ctx, chatID, to, 8); err != nil {
				t.Fatal(err)
			}
			want := Settings{Provider: ProviderGoogle, Audience: AudienceEveryone, LogEnabled: true, JudgeTo: to}
			if got, err := Get(ctx, chatID); err != nil || got != want {
				t.Fatalf("after SetJudgeTo(%d): %+v, %v, want %+v", to, got, err, want)
			}
			if fresh, _ := GetFresh(ctx, chatID); fresh != want {
				t.Fatalf("GetFresh = %+v", fresh)
			}
		}
		// Writing the other controls leaves the destination alone.
		if err := SetJudgeTo(ctx, chatID, JudgeToStaff, 8); err != nil {
			t.Fatal(err)
		}
		if err := SetLogEnabled(ctx, chatID, false, 9); err != nil {
			t.Fatal(err)
		}
		if got, _ := Get(ctx, chatID); got.JudgeTo != JudgeToStaff || got.LogEnabled {
			t.Fatalf("SetLogEnabled clobbered the destination: %+v", got)
		}
	})

	t.Run("the memories switch is its own column", func(t *testing.T) {
		chatID := newChat(t)
		if got, _ := Get(ctx, chatID); got.MemoriesEnabled {
			t.Fatal("a chat with no row reads the memories on")
		}
		if err := SetProvider(ctx, chatID, ProviderGoogle, 7); err != nil {
			t.Fatal(err)
		}
		if err := SetLogEnabled(ctx, chatID, true, 7); err != nil {
			t.Fatal(err)
		}
		if err := SetMemoriesEnabled(ctx, chatID, true, 8); err != nil {
			t.Fatal(err)
		}
		want := Settings{Provider: ProviderGoogle, Audience: AudienceEveryone, LogEnabled: true, MemoriesEnabled: true}
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after SetMemoriesEnabled(true): %+v, %v", got, err)
		}
		// A Go false is written, and the other switches stay.
		if err := SetMemoriesEnabled(ctx, chatID, false, 9); err != nil {
			t.Fatal(err)
		}
		want.MemoriesEnabled = false
		if got, err := Get(ctx, chatID); err != nil || got != want {
			t.Fatalf("after SetMemoriesEnabled(false): %+v, %v", got, err)
		}
		// Writing the other controls leaves it alone.
		if err := SetMemoriesEnabled(ctx, chatID, true, 9); err != nil {
			t.Fatal(err)
		}
		if err := SetLogEnabled(ctx, chatID, false, 9); err != nil {
			t.Fatal(err)
		}
		if err := SetEnabled(ctx, chatID, true, 9); err != nil {
			t.Fatal(err)
		}
		if got, _ := GetFresh(ctx, chatID); !got.MemoriesEnabled || !got.Enabled || got.LogEnabled {
			t.Fatalf("another control clobbered the memories switch: %+v", got)
		}
	})

	t.Run("an invalid value writes nothing", func(t *testing.T) {
		chatID := newChat(t)
		if err := SetJudgeTo(ctx, chatID, 3, 1); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("judge: %v", err)
		}
		if err := SetJudgeTo(ctx, chatID, -1, 1); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("judge -1: %v", err)
		}
		if err := SetProvider(ctx, chatID, "openai", 1); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("provider: %v", err)
		}
		if err := SetAudience(ctx, chatID, 2, 1); !errors.Is(err, ErrInvalidValue) {
			t.Errorf("audience: %v", err)
		}
		if rowCount(t, chatID) != 0 {
			t.Fatal("an invalid value wrote a row")
		}
	})

	t.Run("the table's checks refuse values outside the stored ones", func(t *testing.T) {
		chatID := newChat(t)
		bad := &models.ChatConversation{ChatID: chatID, Provider: "openai"}
		if err := db.DB.Create(bad).Error; err == nil {
			t.Error("a provider outside the check was stored")
		}
		bad = &models.ChatConversation{ChatID: chatID, Provider: ProviderAnthropic, Audience: 2}
		if err := db.DB.Create(bad).Error; err == nil {
			t.Error("an audience outside the check was stored")
		}
		bad = &models.ChatConversation{ChatID: chatID, Provider: ProviderAnthropic, JudgeTo: 3}
		if err := db.DB.Create(bad).Error; err == nil {
			t.Error("a judge destination outside the check was stored")
		}
	})

}
