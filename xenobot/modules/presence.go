package modules

import (
	"context"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/redis/go-redis/v9"

	"github.com/uasneppy/Xenobot-72/xenobot/db/lockdown"
	"github.com/uasneppy/Xenobot-72/xenobot/db/presence"
	"github.com/uasneppy/Xenobot-72/xenobot/db/staff"
)

var presenceModule = moduleStruct{moduleName: "Presence"}

const presenceKeyPrefix = "xenobot:presence:"

var presenceRedisClient = func() *redis.Client {
	return chatRedisClient()
}

var (
	presenceStaffLookup    = staff.GetStaffGroup
	presenceLockdownLookup = lockdown.GetActiveFresh
)

func presenceBotReactKey(chatID, msgID int64) string           { return "" }
func presenceUserCooldownKey(chatID, userID int64) string      { return "" }
func presenceConsensusUsersKey(chatID, msgID int64, emoji string) string {
	return ""
}
func presenceConsensusCooldownKey(chatID int64) string         { return "" }
func presenceConsensusDoneKey(chatID, msgID int64) string      { return "" }

func presenceChatTypeAllowed(chat *gotgbot.Chat) bool { return false }
func presenceIsStaffGroup(chatID int64) bool         { return false }
func presenceIsLockedDown(chatID int64) bool         { return false }

func presenceMessageAllowed(b *gotgbot.Bot, ctx *ext.Context) (int64, bool) {
	return 0, false
}

func presenceIsAnonGroupAdmin(mru *gotgbot.MessageReactionUpdated) bool { return false }

var presenceWrites = struct {
	ConsensusEnabled   func(ctx context.Context, chatID int64, enabled bool, by int64) error
	ConsensusThreshold func(ctx context.Context, chatID int64, threshold int, by int64) error
	RetractEnabled     func(ctx context.Context, chatID int64, enabled bool, by int64) error
}{
	ConsensusEnabled:   presence.SetConsensusEnabled,
	ConsensusThreshold: presence.SetConsensusThreshold,
	RetractEnabled:     presence.SetRetractEnabled,
}

var presenceReadFresh = presence.GetFresh

type presenceApplyOutcome int

const (
	presenceApplied presenceApplyOutcome = iota
	presenceInvalidValue
	presenceSaveFailed
)

type presenceApplyResult struct {
	Outcome  presenceApplyOutcome
	Settings *presence.Settings
	Err      error
}

func applyPresenceConsensusEnabled(ctx context.Context, chatID int64, enabled bool, by int64) presenceApplyResult {
	return presenceApplyResult{Outcome: presenceSaveFailed}
}

func applyPresenceConsensusThreshold(ctx context.Context, chatID int64, threshold int, by int64) presenceApplyResult {
	return presenceApplyResult{Outcome: presenceSaveFailed}
}

func applyPresenceRetractEnabled(ctx context.Context, chatID int64, enabled bool, by int64) presenceApplyResult {
	return presenceApplyResult{Outcome: presenceSaveFailed}
}
