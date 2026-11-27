package modules

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"html"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db/lang"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/reactions"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/chat_status"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/formatting"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/helpers"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/tracing"
)

var reactionsModule = moduleStruct{
	moduleName:   "Reactions",
	handlerGroup: 8,
}

var supportedReactionEmoji = []string{
	"❤", "👍", "👎", "🔥", "🥰", "👏", "😁", "🤔", "🤯", "😱", "🤬", "😢",
	"🎉", "🤩", "🤮", "💩", "🙏", "👌", "🕊", "🤡", "🥱", "🥴", "😍", "🐳",
	"❤‍🔥", "🌚", "🌭", "💯", "🤣", "⚡", "🍌", "🏆", "💔", "🤨", "😐", "🍓",
	"🍾", "💋", "🖕", "😈", "😴", "😭", "🤓", "👻", "👨‍💻", "👀", "🎃", "🙈",
	"😇", "😨", "🤝", "✍", "🤗", "🫡", "🎅", "🎄", "☃", "💅", "🤪", "🗿",
	"🆒", "💘", "🙉", "🦄", "😘", "💊", "🙊", "😎", "👾", "🤷‍♂", "🤷",
	"🤷‍♀", "😡",
}

func LoadReactions(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(helpers.NewBotCommand("addreaction", reactionsModule.addReaction))
	dispatcher.AddHandler(helpers.NewBotCommand("removereaction", reactionsModule.removeReaction))
	dispatcher.AddHandler(helpers.NewBotCommand("reactions", reactionsModule.listReactions))
	dispatcher.AddHandler(helpers.NewBotCommand("resetreactions", reactionsModule.resetReactions))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("reactions_help"), reactionsModule.reactionsHelpHandler))

	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, reactionsModule.checkReactions), reactionsModule.handlerGroup)

	SetModuleEnabled(reactionsModule.moduleName, true)

	DefaultHelpRegistry().AltHelpOptions["Reactions"] = []string{"reaction"}
	SetModuleHelpFunc(reactionsModule.moduleName, reactionsHelpKb)

	log.Info("[Modules] Reactions module loaded")
}

func (m moduleStruct) reactionsHelpHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	if query == nil {
		return ext.EndGroups
	}

	action := ""
	if decoded, ok := decodeCallbackData(query.Data, "reactions_help"); ok {
		action, _ = decoded.Field("action")
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if action == "" {
		return answerInvalidCallback(b, ctx, query)
	}

	var helpText string
	switch action {
	case "add":
		helpText, _ = tr.GetString("reactions_add_usage")
	case "remove":
		helpText, _ = tr.GetString("reactions_remove_usage")
	default:
		return answerInvalidCallback(b, ctx, query)
	}

	if query.Message == nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
		return ext.EndGroups
	}

	backText, _ := tr.GetString("common_back")
	_, _, err := query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: helpText, ParseMode: formatting.HTML,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{
						Text:         backText,
						CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Reactions"}),
					},
				},
			},
		}})
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) addReaction(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][addReaction] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}

	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_change_info_cmd_error", "chat_status_change_info_button_error")
		return ext.EndGroups
	}

	args := ctx.Args()
	if len(args) < 3 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_add_usage")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	keyword := strings.ToLower(strings.TrimSpace(args[1]))
	result := applyReactionAdd(tracing.UpdateContext(ctx), chat.Id, keyword, args[2], true)
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	switch result.Outcome {
	case reactionBadEmoji:
		// ReactionTypeEmoji accepts only Telegram's documented reaction set.
		text, _ := tr.GetString("reactions_invalid_emoji")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	case reactionCapReached:
		text, _ := tr.GetString("reactions_limit_reached", i18n.TranslationParams{"max": reactionsCapMax})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	case reactionAdded:
	default:
		log.Errorf("[Reactions] Failed to save reaction: %v", result.Err)
		text, _ := tr.GetString("reactions_add_error")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	text, _ := tr.GetString("reactions_add_success", i18n.TranslationParams{
		"keyword": html.EscapeString(keyword),
		"emoji":   html.EscapeString(result.Emoji),
	})
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) removeReaction(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][removeReaction] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}

	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_change_info_cmd_error", "chat_status_change_info_button_error")
		return ext.EndGroups
	}

	args := ctx.Args()
	if len(args) < 2 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_remove_usage")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	keyword := strings.ToLower(strings.TrimSpace(args[1]))
	result := applyReactionRemove(tracing.UpdateContext(ctx), chat.Id, keyword)
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	switch result.Outcome {
	case reactionRemoveAbsent:
		text, _ := tr.GetString("reactions_keyword_not_found", i18n.TranslationParams{
			"keyword": html.EscapeString(keyword),
		})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	case reactionRemoveFailed:
		log.Errorf("[Reactions] Failed to update reactions: %v", result.Err)
		text, _ := tr.GetString("reactions_remove_error")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	text, _ := tr.GetString("reactions_remove_success", i18n.TranslationParams{
		"keyword": html.EscapeString(keyword),
	})
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) listReactions(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][listReactions] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	reactionsMap := reactions.GetReactionsContext(tracing.UpdateContext(ctx), chat.Id)
	if len(reactionsMap) == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_none")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	var sb strings.Builder
	for keyword, emoji := range reactionsMap {
		fmt.Fprintf(&sb, "• %s → %s\n", html.EscapeString(keyword), html.EscapeString(emoji))
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("reactions_list_header", i18n.TranslationParams{
		"list": sb.String(),
	})
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) resetReactions(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][resetReactions] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}

	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_change_info_cmd_error", "chat_status_change_info_button_error")
		return ext.EndGroups
	}

	if err := reactions.ResetReactions(chat.Id); err != nil {
		log.Errorf("[Reactions] Failed to reset reactions: %v", err)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_remove_error")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("reactions_reset_success")
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

const staticReactTimeout = 5 * time.Second

var (
	// staticReactionRoll decides whether a static reaction rule fires for its ChancePct (1..100).
	// Package variable only so tests can make probability rolls deterministic.
	staticReactionRoll = func(pct int) bool {
		return pct >= 100 || (pct > 0 && rand.IntN(100) < pct) // #nosec G404 -- non-cryptographic reaction sampling
	}
	// catStickerRoll decides whether an eligible cat sticker triggers a micro-reaction (20%).
	// Package variable only so tests can make probability rolls deterministic.
	catStickerRoll = func() bool {
		return rand.IntN(100) < 20 // #nosec G404 -- non-cryptographic reaction sampling
	}
	// catStickerCooldown gates per-chat cat-sticker micro-triggers across replicas.
	catStickerCooldown = 3 * time.Minute
	// staticChatCooldown gates per-chat static reaction bursts across replicas.
	staticChatCooldown = 10 * time.Second
)

var catStickerEmojis = map[string]struct{}{
	"🐱":  {},
	"😸":  {},
	"😹":  {},
	"😻":  {},
	"😼":  {},
	"😽":  {},
	"🙀":  {},
	"😿":  {},
	"😾":  {},
	"🐈":  {},
	"🐈‍⬛": {},
}

func staticRuleCooldownKey(chatID int64, ruleID uint) string {
	return presenceChatTag(chatID) + "static:r:" + strconv.FormatUint(uint64(ruleID), 10)
}

func staticChatCooldownKey(chatID int64) string {
	return presenceChatTag(chatID) + "static:cd"
}

func catStickerCooldownKey(chatID int64) string {
	return presenceChatTag(chatID) + "cat:cd"
}

func isCatStickerEmoji(raw string) bool {
	if raw == "" {
		return false
	}
	_, ok := catStickerEmojis[normaliseReactionEmoji(raw)]
	return ok
}

func isStaticWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// matchStaticReactionKeyword checks whether keyword matches text, optionally enforcing Unicode
// word boundaries using pure Go rune decoding (safe for UK/RU/EN with zero RE2 \b backtracking).
func matchStaticReactionKeyword(text, keyword string, wordBoundary bool) bool {
	if text == "" || keyword == "" {
		return false
	}
	lowerText := strings.ToLower(text)
	lowerKeyword := strings.ToLower(keyword)
	if !wordBoundary {
		return strings.Contains(lowerText, lowerKeyword)
	}
	for offset := 0; offset <= len(lowerText)-len(lowerKeyword); {
		idx := strings.Index(lowerText[offset:], lowerKeyword)
		if idx < 0 {
			return false
		}
		start := offset + idx
		end := start + len(lowerKeyword)
		beforeOK := true
		if start > 0 {
			r, _ := utf8.DecodeLastRuneInString(lowerText[:start])
			beforeOK = !isStaticWordRune(r)
		}
		afterOK := true
		if end < len(lowerText) {
			r, _ := utf8.DecodeRuneInString(lowerText[end:])
			afterOK = !isStaticWordRune(r)
		}
		if beforeOK && afterOK {
			return true
		}
		_, step := utf8.DecodeRuneInString(lowerText[start:])
		if step <= 0 {
			step = 1
		}
		offset = start + step
	}
	return false
}

func setPresenceStaticReacted(ctx *ext.Context) {
	if ctx == nil {
		return
	}
	if ctx.Data == nil {
		ctx.Data = make(map[string]any)
	}
	ctx.Data[presenceStaticReactedKey] = true
}

func presenceRedisKeyExists(ctx context.Context, client *redis.Client, key string) (bool, bool) {
	if client == nil {
		return false, true
	}
	rCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	n, err := client.Exists(rCtx, key).Result()
	if err != nil {
		log.WithField("key", key).Debugf("[Reactions] redis EXISTS failed: %s", chatRedisErrClass(err))
		return false, false
	}
	return n > 0, true
}

func presenceRedisClaimCooldown(ctx context.Context, client *redis.Client, key string, ttl time.Duration) bool {
	if client == nil || ttl <= 0 {
		return true
	}
	wCtx, cancel := context.WithTimeout(ctx, chatRedisTimeout)
	defer cancel()
	ok, err := client.SetNX(wCtx, key, "1", ttl).Result()
	if err != nil {
		log.WithField("key", key).Debugf("[Reactions] redis SET NX failed: %s", chatRedisErrClass(err))
		return false
	}
	return ok
}

func tryStaticReactionRules(
	ctx context.Context,
	b *gotgbot.Bot,
	extCtx *ext.Context,
	chatID, msgID, userID int64,
	text string,
	fromCaption bool,
	rules []reactions.Rule,
) bool {
	var candidates []reactions.Rule
	for _, rule := range rules {
		if fromCaption && !rule.MatchCaption {
			continue
		}
		if matchStaticReactionKeyword(text, rule.Keyword, rule.WordBoundary) {
			candidates = append(candidates, rule)
		}
	}
	if len(candidates) == 0 {
		return false
	}
	slices.SortFunc(candidates, func(a, b reactions.Rule) int {
		if c := strings.Compare(a.Keyword, b.Keyword); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})

	client := presenceRedisClient()
	if client != nil && staticChatCooldown > 0 {
		exists, ok := presenceRedisKeyExists(ctx, client, staticChatCooldownKey(chatID))
		if !ok || exists {
			return false
		}
	}

	for _, rule := range candidates {
		if client != nil && rule.CooldownSecs > 0 {
			exists, ok := presenceRedisKeyExists(ctx, client, staticRuleCooldownKey(chatID, rule.ID))
			if !ok {
				return false
			}
			if exists {
				continue
			}
		}
		if !staticReactionRoll(rule.ChancePct) {
			continue
		}
		if presenceIsLockedDown(chatID) {
			return false
		}
		if rule.CooldownSecs > 0 && !presenceRedisClaimCooldown(ctx, client, staticRuleCooldownKey(chatID, rule.ID), time.Duration(rule.CooldownSecs)*time.Second) {
			return false
		}
		if staticChatCooldown > 0 && !presenceRedisClaimCooldown(ctx, client, staticChatCooldownKey(chatID), staticChatCooldown) {
			return false
		}
		if presenceSetReaction(ctx, b, chatID, msgID, userID, rule.Emoji, "static") {
			setPresenceStaticReacted(extCtx)
			return true
		}
		return false
	}
	return false
}

func tryCatStickerReaction(ctx context.Context, b *gotgbot.Bot, extCtx *ext.Context, chatID, msgID, userID int64, sticker *gotgbot.Sticker) bool {
	if sticker == nil || !isCatStickerEmoji(sticker.Emoji) {
		return false
	}
	pres, err := presenceSettingsGet(ctx, chatID)
	if err != nil || !pres.CatStickerEnabled {
		return false
	}
	if !catStickerRoll() {
		return false
	}
	client := presenceRedisClient()
	if client != nil && catStickerCooldown > 0 {
		exists, ok := presenceRedisKeyExists(ctx, client, catStickerCooldownKey(chatID))
		if !ok || exists {
			return false
		}
	}
	if presenceIsLockedDown(chatID) {
		return false
	}
	palette, ok := presenceAllowedPalette(ctx, b, chatID)
	if !ok {
		return false
	}
	var choices []string
	for _, candidate := range []string{"👀", "❤"} {
		if slices.Contains(palette, candidate) {
			choices = append(choices, candidate)
		}
	}
	if len(choices) == 0 {
		return false
	}
	chosen := choices[rand.IntN(len(choices))] // #nosec G404 -- non-cryptographic reaction sampling
	if catStickerCooldown > 0 && !presenceRedisClaimCooldown(ctx, client, catStickerCooldownKey(chatID), catStickerCooldown) {
		return false
	}
	if presenceSetReaction(ctx, b, chatID, msgID, userID, chosen, "cat") {
		setPresenceStaticReacted(extCtx)
		return true
	}
	return false
}

func (m moduleStruct) checkReactions(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][checkReactions] Recovered from panic: %v", r)
		}
	}()

	userID, ok := presenceMessageAllowed(b, ctx)
	if !ok {
		return ext.ContinueGroups
	}
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	fromCaption := false
	text := msg.Text
	if text == "" {
		text = msg.Caption
		fromCaption = text != ""
	}
	if text == "" && msg.Sticker == nil {
		return ext.ContinueGroups
	}

	runCtx, cancel := context.WithTimeout(tracing.UpdateContext(ctx), staticReactTimeout)
	defer cancel()

	if text != "" {
		rules, err := reactionWrites.Rules(runCtx, chat.Id)
		if err != nil {
			log.WithField("chat_id", chat.Id).Debugf("[Reactions] failed to load reaction rules: %v", err)
		} else if len(rules) > 0 {
			if tryStaticReactionRules(runCtx, b, ctx, chat.Id, msg.MessageId, userID, text, fromCaption, rules) {
				return ext.ContinueGroups
			}
		}
	}

	if msg.Sticker != nil {
		tryCatStickerReaction(runCtx, b, ctx, chat.Id, msg.MessageId, userID, msg.Sticker)
	}

	return ext.ContinueGroups
}

func init() {
	RegisterLegacyModule("Reactions", 250, LoadReactions)
}

// reactionsHelpKb is the Reactions help keyboard in the reader's language.
func reactionsHelpKb(tr *i18n.Translator) [][]gotgbot.InlineKeyboardButton {
	return [][]gotgbot.InlineKeyboardButton{{
		{
			Text:         trS(tr, "reactions_button_add"),
			CallbackData: encodeCallbackData("reactions_help", map[string]string{"action": "add"}),
		},
		{
			Text:         trS(tr, "reactions_button_remove"),
			CallbackData: encodeCallbackData("reactions_help", map[string]string{"action": "remove"}),
		},
	}}
}

// reactionWrites is the write and read seam of the reaction apply functions, a variable only so a test can fail it.
var reactionWrites = struct {
	Add            func(ctx context.Context, chatID int64, keyword, emoji string, overwrite bool) (bool, error)
	Remove         func(chatID int64, keyword string) error
	RemoveByID     func(ctx context.Context, chatID, id int64) (*models.Reactions, error)
	Lookup         func(ctx context.Context, chatID int64) map[string]string
	Rules          func(ctx context.Context, chatID int64) ([]reactions.Rule, error)
	Entry          func(ctx context.Context, chatID, id int64) (*models.Reactions, error)
	SetOptionsByID func(ctx context.Context, chatID, id int64, wordBoundary, matchCaption bool, chancePct, cooldownSecs int) (*models.Reactions, error)
	Exists         func(ctx context.Context, chatID int64, keyword string) (bool, error)
	Count          func(ctx context.Context, chatID int64) (int64, error)
}{
	Add:            reactions.AddReactionContext,
	Remove:         reactions.RemoveReaction,
	RemoveByID:     reactions.RemoveReactionByID,
	Lookup:         reactions.GetReactionsContext,
	Rules:          reactions.GetReactionRulesContext,
	Entry:          reactions.GetReactionEntryFresh,
	SetOptionsByID: reactions.SetReactionOptionsByID,
	Exists:         reactions.ReactionExistsFresh,
	Count:          reactions.CountReactionsFresh,
}

// reactionAddOutcome is what applyReactionAdd did; its zero value is added.
type reactionAddOutcome int

const (
	// reactionAdded means the reaction was stored.
	reactionAdded reactionAddOutcome = iota
	// reactionBadEmoji means the emoji is not one of Telegram's reactions; nothing was written.
	reactionBadEmoji
	// reactionExists means the keyword already has a reaction and the caller asked not to overwrite it.
	reactionExists
	// reactionCapReached means the keyword is new and the chat already has reactionsCapMax reactions.
	reactionCapReached
	// reactionSaveFailed means a read or the write failed; nothing is known to be stored.
	reactionSaveFailed
)

// reactionAddResult is the typed result of applyReactionAdd: the outcome, the stored emoji (U+FE0F stripped)
// and the error of a failed save.
type reactionAddResult struct {
	Outcome reactionAddOutcome
	Emoji   string
	Err     error
}

// normaliseReactionEmoji is the emoji as the reaction is stored: trimmed, with every U+FE0F stripped.
func normaliseReactionEmoji(raw string) string {
	return strings.ReplaceAll(strings.TrimSpace(raw), "\ufe0f", "")
}

// reactionsCapMax is the most reactions one chat can have. It is a soft cap read before the insert, so two
// concurrent adds can pass it by one. Replacing the emoji of an existing keyword is allowed at the cap.
const reactionsCapMax = 100

// applyReactionAdd stores one reaction of a chat: the one write path of /addreaction and of the Reactions page.
// The emoji must be one of Telegram's reactions (supportedReactionEmoji, after U+FE0F is stripped). With
// overwrite (the command) an existing keyword gets the new emoji; without it (the page) an existing keyword is
// refused and never overwritten. It judges no person's rights and sends nothing.
func applyReactionAdd(ctx context.Context, chatID int64, keyword, rawEmoji string, overwrite bool) reactionAddResult {
	emoji := normaliseReactionEmoji(rawEmoji)
	if !slices.Contains(supportedReactionEmoji, emoji) {
		return reactionAddResult{Outcome: reactionBadEmoji}
	}
	exists, err := reactionWrites.Exists(ctx, chatID, keyword)
	if err != nil {
		return reactionAddResult{Outcome: reactionSaveFailed, Emoji: emoji, Err: err}
	}
	if exists && !overwrite {
		return reactionAddResult{Outcome: reactionExists, Emoji: emoji}
	}
	if !exists {
		n, err := reactionWrites.Count(ctx, chatID)
		if err != nil {
			return reactionAddResult{Outcome: reactionSaveFailed, Emoji: emoji, Err: err}
		}
		if n >= reactionsCapMax {
			return reactionAddResult{Outcome: reactionCapReached, Emoji: emoji}
		}
	}
	added, err := reactionWrites.Add(ctx, chatID, keyword, emoji, overwrite)
	if err != nil {
		return reactionAddResult{Outcome: reactionSaveFailed, Emoji: emoji, Err: err}
	}
	if !added {
		return reactionAddResult{Outcome: reactionExists, Emoji: emoji}
	}
	return reactionAddResult{Outcome: reactionAdded, Emoji: emoji}
}

// reactionRemoveOutcome is what applyReactionRemove and applyReactionRemoveByID did; its zero value is removed.
type reactionRemoveOutcome int

const (
	// reactionRemoved means the chat's row was deleted.
	reactionRemoved reactionRemoveOutcome = iota
	// reactionRemoveAbsent means the chat has no such reaction; nothing was written.
	reactionRemoveAbsent
	// reactionRemoveFailed means the delete failed.
	reactionRemoveFailed
)

// reactionRemoveResult is the typed result of a reaction removal: the outcome, the removed row's keyword and
// emoji (by ID only) and the error of a failed delete.
type reactionRemoveResult struct {
	Outcome reactionRemoveOutcome
	Keyword string
	Emoji   string
	Err     error
}

// applyReactionRemove removes a chat's reaction by keyword, the write /removereaction makes: an absent keyword
// (read through the cached map, as the command always has) writes nothing.
func applyReactionRemove(ctx context.Context, chatID int64, keyword string) reactionRemoveResult {
	if _, exists := reactionWrites.Lookup(ctx, chatID)[keyword]; !exists {
		return reactionRemoveResult{Outcome: reactionRemoveAbsent, Keyword: keyword}
	}
	if err := reactionWrites.Remove(chatID, keyword); err != nil {
		return reactionRemoveResult{Outcome: reactionRemoveFailed, Keyword: keyword, Err: err}
	}
	return reactionRemoveResult{Outcome: reactionRemoved, Keyword: keyword}
}

// applyReactionRemoveByID removes one reaction by its row ID, only when it belongs to the chat (the page's
// delete): one conditional DELETE and the cache drop when a row went.
func applyReactionRemoveByID(ctx context.Context, chatID, id int64) reactionRemoveResult {
	row, err := reactionWrites.RemoveByID(ctx, chatID, id)
	if err != nil {
		log.Errorf("[Reactions] Failed to delete a reaction of chat %d: %v", chatID, err)
		return reactionRemoveResult{Outcome: reactionRemoveFailed, Err: err}
	}
	if row == nil {
		return reactionRemoveResult{Outcome: reactionRemoveAbsent}
	}
	return reactionRemoveResult{Outcome: reactionRemoved, Keyword: row.Keyword, Emoji: row.Emoji}
}

// reactionOptionsOutcome is what applyReactionSetOptions did; its zero value is updated.
type reactionOptionsOutcome int

const (
	// reactionOptionsUpdated means the rule's options were stored.
	reactionOptionsUpdated reactionOptionsOutcome = iota
	// reactionOptionsInvalid means chancePct or cooldownSecs was out of bounds; no DB call ran.
	reactionOptionsInvalid
	// reactionOptionsAbsent means no row of the chat has the ID (missing or foreign chat).
	reactionOptionsAbsent
	// reactionOptionsFailed means the DB write failed.
	reactionOptionsFailed
)

// reactionOptionsResult is the typed result of applyReactionSetOptions.
type reactionOptionsResult struct {
	Outcome reactionOptionsOutcome
	Row     *models.Reactions
	Err     error
}

// applyReactionSetOptions validates option bounds before any DB call and updates one rule's
// per-rule options strictly scoped to chatID and id.
func applyReactionSetOptions(ctx context.Context, chatID, id int64, wordBoundary, matchCaption bool, chancePct, cooldownSecs int) reactionOptionsResult {
	if !reactions.ValidChancePct(chancePct) || !reactions.ValidCooldownSecs(cooldownSecs) {
		return reactionOptionsResult{Outcome: reactionOptionsInvalid}
	}
	row, err := reactionWrites.SetOptionsByID(ctx, chatID, id, wordBoundary, matchCaption, chancePct, cooldownSecs)
	if err != nil {
		if errors.Is(err, reactions.ErrInvalidOption) {
			return reactionOptionsResult{Outcome: reactionOptionsInvalid, Err: err}
		}
		log.Errorf("[Reactions] Failed to update reaction options of chat %d id %d: %v", chatID, id, err)
		return reactionOptionsResult{Outcome: reactionOptionsFailed, Err: err}
	}
	if row == nil {
		return reactionOptionsResult{Outcome: reactionOptionsAbsent}
	}
	return reactionOptionsResult{Outcome: reactionOptionsUpdated, Row: row}
}
