package modules

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
	"github.com/uasneppy/Xenobot-72/xenobot/db/adminindex"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/xenobot/db/staff"
	"github.com/uasneppy/Xenobot-72/xenobot/i18n"
	"github.com/uasneppy/Xenobot-72/xenobot/utils/cache"
)

// The Copy Settings page (page cp, Phase 29, TUNE-09): private-only one-time copy of all 6
// event-message and moderation-tuning areas from the open source group to another group the
// presser also manages. Hidden in Staff Groups, with a static zero-I/O Main state. Both source
// and target groups are verified live at ask (ck) and again at Yes (cy) before consuming the
// pair-bound one-time Redis confirm token via an atomic Lua GET+DEL script. Every target
// authorization, bot-missing, non-group, or Staff-Group refusal returns the uniform opaque
// alert settings_pick_unavailable so probing arbitrary chat IDs reveals nothing.

const (
	// settingsPageCopy is the p value of the Copy Settings page.
	settingsPageCopy = "cp"

	// Action codes of the Copy Settings page.
	settingsActCopyPage = "cg"
	settingsActCopyPick = "ck"
	settingsActCopyYes  = "cy"

	// settingsCopyCardKeyPrefix is the Redis key prefix for one-time copy confirmation cards,
	// outside xenobot:cache: so cache clears do not wipe in-flight confirmations.
	settingsCopyCardKeyPrefix = "xenobot:settings:copy:"
	// settingsCopyCardIDBytes is 96 bits of entropy (16 unpadded base64url chars).
	settingsCopyCardIDBytes = 12
	settingsCopyCardIDLen   = 16

	// settingsCopyWriteTimeout bounds one full 9-table copy execution.
	settingsCopyWriteTimeout = 15 * time.Second
)

// settingsCopyTokenRecord binds a one-time confirm card to one (user, source, target) triple.
type settingsCopyTokenRecord struct {
	UserID       int64 `json:"u"`
	SourceChatID int64 `json:"s"`
	TargetChatID int64 `json:"t"`
}

// copyTargetVerdict classifies live verification of the source/target pair.
type copyTargetVerdict int

const (
	copyTargetAllowed copyTargetVerdict = iota
	copyTargetUnavailable
	copyTargetCheckFailed
)

var (
	errSettingsCopyNoRedis    = errors.New("copy settings: Redis unavailable")
	errSettingsCopyTokenTaken = errors.New("copy settings: confirm token already exists")

	// settingsCopyConsumeScript atomically reads and deletes the one-time confirm token so
	// concurrent or replayed Yes presses never run applyCopySettings twice.
	settingsCopyConsumeScript = redis.NewScript(`
local v = redis.call("GET", KEYS[1])
if not v then return false end
redis.call("DEL", KEYS[1])
return v
`)

	// Test seams (production defaults never reassigned outside tests).
	settingsCopyRedisClient      = settingsCopyDefaultRedis
	settingsCopyStaffLookup      = settingsCopyDefaultStaffLookup
	settingsCopyStaffBatchLookup = settingsCopyDefaultStaffBatchLookup
	settingsCopyApply            = applyCopySettings
	settingsCopyHubLookup        = func(ctx context.Context, userID int64) ([]adminindex.ListedGroup, error) {
		return settingsHubLookup(ctx, userID)
	}
)

func settingsCopyDefaultRedis() *redis.Client {
	if cache.IsRedisAvailable() {
		return cache.GetRedisClient()
	}
	return nil
}

func settingsCopyDefaultStaffLookup(chatID int64) (*models.StaffGroup, error) {
	if db.DB == nil {
		return nil, errCopyNoDatabase
	}
	return staff.GetStaffGroupFresh(chatID)
}

func settingsCopyDefaultStaffBatchLookup(ctx context.Context, chatIDs []int64) (map[int64]bool, error) {
	if db.DB == nil {
		return nil, errCopyNoDatabase
	}
	return staff.StaffGroupChatIDsFresh(ctx, chatIDs)
}

func init() {
	private := func(check func(map[string]string) bool) func(map[string]string) bool {
		return func(fields map[string]string) bool { return fields["c"] != "" && check(fields) }
	}
	registerSettingsPage(settingsPage{
		Key:            settingsPageCopy,
		Order:          82,
		Icon:           "📋",
		TitleKey:       "settings_page_copy",
		MainState:      settingsCopyMainState,
		Open:           settingsCopyOpen,
		Hidden:         settingsCopyHidden,
		PrivateOnly:    true,
		PrivateNoteKey: "settings_copy_group_note",
		Actions: map[string]settingsPageAction{
			settingsActCopyPage: {
				Fields: []string{"v"}, Right: settingsRightAdmin, Run: settingsCopyRunPage,
				Validate: private(settingsCopyPageValid),
			},
			settingsActCopyPick: {
				Fields: []string{"t"}, Right: settingsRightAdmin, Run: settingsCopyRunPick,
				Validate: private(settingsCopyPickValid),
			},
			settingsActCopyYes: {
				Fields: []string{"k"}, Right: settingsRightAdmin, Run: settingsCopyRunYes,
				Validate: private(settingsCopyYesValid),
			},
		},
	})
}

func settingsCopyPageValid(fields map[string]string) bool {
	v, ok := settingsParseNumber(fields["v"], settingsHubPageDigits)
	return ok && v >= 1 && v <= settingsHubMaxPages && strconv.Itoa(v) == fields["v"]
}

func settingsCopyPickValid(fields map[string]string) bool {
	srcID, srcOK := parseStaffChatIDArg(fields["c"])
	dstID, dstOK := parseStaffChatIDArg(fields["t"])
	if !srcOK || !dstOK {
		return false
	}
	if strconv.FormatInt(srcID, 10) != fields["c"] || strconv.FormatInt(dstID, 10) != fields["t"] {
		return false
	}
	return dstID != srcID
}

func settingsCopyTokenValid(token string) bool {
	if len(token) != settingsCopyCardIDLen {
		return false
	}
	for i := 0; i < len(token); i++ {
		c := token[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func settingsCopyYesValid(fields map[string]string) bool {
	return settingsCopyTokenValid(fields["k"])
}

func settingsCopyTokenKey(token string) string {
	return settingsCopyCardKeyPrefix + token
}

func settingsCopyNewToken() (string, error) {
	raw := make([]byte, settingsCopyCardIDBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func settingsCopyPutToken(ctx context.Context, rec settingsCopyTokenRecord) (string, error) {
	client := settingsCopyRedisClient()
	if client == nil {
		return "", errSettingsCopyNoRedis
	}
	token, err := settingsCopyNewToken()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return "", err
	}
	ttl := settingsCardLifetime
	if ttl <= 0 {
		ttl = time.Millisecond
	}
	putCtx, cancel := context.WithTimeout(ctx, raidRedisTimeout)
	defer cancel()
	stored, err := client.SetNX(putCtx, settingsCopyTokenKey(token), payload, ttl).Result()
	if err != nil {
		return "", err
	}
	if !stored {
		return "", errSettingsCopyTokenTaken
	}
	return token, nil
}

func settingsCopyPeekToken(ctx context.Context, token string) (settingsCopyTokenRecord, bool, error) {
	client := settingsCopyRedisClient()
	if client == nil {
		return settingsCopyTokenRecord{}, false, errSettingsCopyNoRedis
	}
	return redisGetJSON[settingsCopyTokenRecord](ctx, client, settingsCopyTokenKey(token))
}

func settingsCopyConsumeToken(ctx context.Context, token string) (settingsCopyTokenRecord, bool, error) {
	client := settingsCopyRedisClient()
	if client == nil {
		return settingsCopyTokenRecord{}, false, errSettingsCopyNoRedis
	}
	consumeCtx, cancel := context.WithTimeout(ctx, raidRedisTimeout)
	defer cancel()
	raw, err := settingsCopyConsumeScript.Run(consumeCtx, client, []string{settingsCopyTokenKey(token)}).Text()
	if errors.Is(err, redis.Nil) {
		return settingsCopyTokenRecord{}, false, nil
	}
	if err != nil {
		return settingsCopyTokenRecord{}, false, err
	}
	var rec settingsCopyTokenRecord
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return settingsCopyTokenRecord{}, false, err
	}
	return rec, true, nil
}

// settingsCopyHidden hides Copy Settings in Staff Groups (definite fresh row).
func settingsCopyHidden(group *gotgbot.Chat) bool {
	row, err := settingsCopyStaffLookup(group.Id)
	return err == nil && row != nil
}

// settingsCopyMainState is static with 0 DB reads and 0 Telegram calls.
func settingsCopyMainState(_ *gotgbot.Bot, tr *i18n.Translator, _ *gotgbot.Chat) (string, bool) {
	return settingsText(tr, "settings_state_copy"), true
}

func settingsCopyTitleOrID(title string, chatID int64) string {
	collapsed := settingsHubCollapse(title)
	if collapsed == "" {
		return strconv.FormatInt(chatID, 10)
	}
	return collapsed
}

// settingsCopyLoadCandidates reads the presser's roster from adminindex (creating 0 rows),
// excludes the source group and any definite Staff Group in a single readCtx-bounded batch query,
// and sorts deterministically.
func settingsCopyLoadCandidates(ctx context.Context, userID, sourceChatID int64) ([]adminindex.ListedGroup, error) {
	readCtx, cancel := context.WithTimeout(ctx, settingsReadTimeout)
	defer cancel()
	groups, err := settingsCopyHubLookup(readCtx, userID)
	if err != nil {
		return nil, err
	}
	candidateIDs := make([]int64, 0, len(groups))
	for _, g := range groups {
		if g.ChatID != sourceChatID {
			candidateIDs = append(candidateIDs, g.ChatID)
		}
	}
	var staffIDs map[int64]bool
	if len(candidateIDs) > 0 {
		var staffErr error
		staffIDs, staffErr = settingsCopyStaffBatchLookup(readCtx, candidateIDs)
		if staffErr != nil && readCtx.Err() != nil {
			return nil, staffErr
		}
	}
	filtered := make([]adminindex.ListedGroup, 0, len(candidateIDs))
	for _, g := range groups {
		if g.ChatID == sourceChatID || staffIDs[g.ChatID] {
			continue
		}
		filtered = append(filtered, g)
	}
	return settingsHubSort(filtered), nil
}

// settingsCopyOpen draws page 1 of the target group picker (page cp).
func settingsCopyOpen(p *settingsPress, notice string) (settingsScreen, error) {
	groups, err := settingsCopyLoadCandidates(context.Background(), p.query.From.Id, p.group.Id)
	if err != nil {
		return settingsScreen{}, err
	}
	return renderSettingsCopyPicker(p.view(), groups, 1, notice), nil
}

// settingsCopyRunPage handles cg (paginated target picker).
func settingsCopyRunPage(p *settingsPress) *settingsScreen {
	page, ok := settingsParseNumber(p.fields["v"], settingsHubPageDigits)
	if !ok || page < 1 {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	groups, err := settingsCopyLoadCandidates(context.Background(), p.query.From.Id, p.group.Id)
	if err != nil {
		log.Warnf("[Settings] copy candidate lookup for user %d failed: %v", p.query.From.Id, err)
		p.cannotOpen()
		return nil
	}
	screen := renderSettingsCopyPicker(p.view(), groups, page, "")
	return &screen
}

// settingsCopyVerifySource performs the fresh non-Staff-Group check on the source group,
// failing closed on DB errors. Live admin + settingsGate + group type already passed verify().
func settingsCopyVerifySource(sourceID int64) copyTargetVerdict {
	staffRow, err := settingsCopyStaffLookup(sourceID)
	if err != nil {
		log.Warnf("[Settings] fresh staff check of copy source chat %d failed: %v", sourceID, err)
		return copyTargetCheckFailed
	}
	if staffRow != nil {
		return copyTargetUnavailable
	}
	return copyTargetAllowed
}

// settingsCopyVerifyTarget runs full live authorization on the target group:
// 1. Live lockdownLiveMember (4xx -> opaque unavailable; transient -> check_failed)
// 2. Creator or administrator
// 3. Target settingsGate (owner-only refusal -> opaque unavailable; transient -> check_failed)
// 4. Non-creator admin must hold BOTH can_change_info AND can_restrict_members (P29-02)
// 5. Live GetChatWithContext (group/supergroup only; 4xx/other chat type -> opaque unavailable)
// 6. Fresh non-Staff-Group check (DB error -> check_failed; Staff Group -> opaque unavailable)
func settingsCopyVerifyTarget(ctx context.Context, b *gotgbot.Bot, presser gotgbot.User, sourceID, targetID int64) (*gotgbot.Chat, copyTargetVerdict) {
	if targetID >= 0 || targetID == sourceID {
		return nil, copyTargetUnavailable
	}
	member, err := lockdownLiveMember(b, targetID, presser.Id)
	if err != nil {
		log.Warnf("[Settings] live check of user %d in copy target chat %d failed: %v", presser.Id, targetID, err)
		if isTelegramClientError(err) {
			return nil, copyTargetUnavailable
		}
		return nil, copyTargetCheckFailed
	}
	if member.Status != gotgbot.ChatMemberStatusCreator && member.Status != gotgbot.ChatMemberStatusAdministrator {
		return nil, copyTargetUnavailable
	}
	switch settingsGate(b, targetID, presser, member) {
	case settingsGateIn:
	case settingsGateOwnerOnly:
		return nil, copyTargetUnavailable
	default:
		return nil, copyTargetCheckFailed
	}
	if member.Status != gotgbot.ChatMemberStatusCreator && (!member.CanChangeInfo || !member.CanRestrictMembers) {
		return nil, copyTargetUnavailable
	}
	lookupCtx, cancel := context.WithTimeout(ctx, lockdownLiveCheckTimeout)
	full, err := b.GetChatWithContext(lookupCtx, targetID, nil)
	cancel()
	if err != nil || full == nil {
		log.Warnf("[Settings] getChat of copy target chat %d failed: %v", targetID, err)
		if err != nil && isTelegramClientError(err) {
			return nil, copyTargetUnavailable
		}
		return nil, copyTargetCheckFailed
	}
	targetChat := full.ToChat()
	if targetChat.Type != "group" && targetChat.Type != "supergroup" {
		return nil, copyTargetUnavailable
	}
	staffRow, err := settingsCopyStaffLookup(targetID)
	if err != nil {
		log.Warnf("[Settings] fresh staff check of copy target chat %d failed: %v", targetID, err)
		return nil, copyTargetCheckFailed
	}
	if staffRow != nil {
		return nil, copyTargetUnavailable
	}
	return &targetChat, copyTargetAllowed
}

func settingsCopyAnswerVerdict(p *settingsPress, verdict copyTargetVerdict) {
	switch verdict {
	case copyTargetUnavailable:
		p.answer(settingsText(p.tr, "settings_pick_unavailable"), true)
	default:
		p.answer(settingsText(p.tr, "settings_check_failed"), true)
	}
}

func settingsCopyRedrawWithNotice(p *settingsPress, notice string) *settingsScreen {
	screen, err := settingsCopyOpen(p, notice)
	if err != nil {
		log.Warnf("[Settings] copy redraw for user %d chat %d failed: %v", p.query.From.Id, p.group.Id, err)
		p.cannotOpen()
		return nil
	}
	return &screen
}

// settingsCopyRunPick handles ck (pick target -> live dual-group verify -> store one-time Redis
// confirm token -> draw confirm card).
func settingsCopyRunPick(p *settingsPress) *settingsScreen {
	targetID, ok := parseStaffChatIDArg(p.fields["t"])
	if !ok || targetID == p.group.Id {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	if verdict := settingsCopyVerifySource(p.group.Id); verdict != copyTargetAllowed {
		settingsCopyAnswerVerdict(p, verdict)
		return nil
	}
	targetChat, verdict := settingsCopyVerifyTarget(context.Background(), p.b, p.query.From, p.group.Id, targetID)
	if verdict != copyTargetAllowed {
		settingsCopyAnswerVerdict(p, verdict)
		return nil
	}
	token, err := settingsCopyPutToken(context.Background(), settingsCopyTokenRecord{
		UserID:       p.query.From.Id,
		SourceChatID: p.group.Id,
		TargetChatID: targetID,
	})
	if err != nil {
		log.Warnf("[Settings] copy token write for user %d source %d target %d failed: %v", p.query.From.Id, p.group.Id, targetID, err)
		p.answer(settingsText(p.tr, "settings_copy_unavailable"), true)
		return nil
	}
	screen := renderSettingsCopyConfirm(p.view(), p.group.Title, targetChat.Title, p.group.Id, targetID, token)
	return &screen
}

// settingsCopyRunYes handles cy (confirmed Yes -> peek & bind check -> live dual-group re-verify ->
// atomic Lua GET+DEL consume -> applyCopySettings -> redraw picker with copied notice).
func settingsCopyRunYes(p *settingsPress) *settingsScreen {
	token := p.fields["k"]
	rec, found, err := settingsCopyPeekToken(context.Background(), token)
	if err != nil {
		log.Warnf("[Settings] copy token peek for user %d chat %d failed: %v", p.query.From.Id, p.group.Id, err)
		p.answer(settingsText(p.tr, "settings_check_failed"), true)
		return nil
	}
	if !found {
		expired := settingsText(p.tr, "settings_card_expired")
		p.answer(expired, true)
		return settingsCopyRedrawWithNotice(p, settingsNoticeWarn(expired))
	}
	// Wrong user or wrong source chat must NOT delete or invalidate the real owner's token.
	if rec.UserID != p.query.From.Id || rec.SourceChatID != p.group.Id || rec.TargetChatID == rec.SourceChatID || rec.TargetChatID >= 0 {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	if verdict := settingsCopyVerifySource(p.group.Id); verdict != copyTargetAllowed {
		settingsCopyAnswerVerdict(p, verdict)
		return nil
	}
	targetChat, verdict := settingsCopyVerifyTarget(context.Background(), p.b, p.query.From, p.group.Id, rec.TargetChatID)
	if verdict != copyTargetAllowed {
		settingsCopyAnswerVerdict(p, verdict)
		return nil
	}
	consumed, found, err := settingsCopyConsumeToken(context.Background(), token)
	if err != nil {
		log.Warnf("[Settings] copy token consume for user %d chat %d failed: %v", p.query.From.Id, p.group.Id, err)
		p.answer(settingsText(p.tr, "settings_check_failed"), true)
		return nil
	}
	if !found {
		expired := settingsText(p.tr, "settings_card_expired")
		p.answer(expired, true)
		return settingsCopyRedrawWithNotice(p, settingsNoticeWarn(expired))
	}
	if consumed.UserID != p.query.From.Id || consumed.SourceChatID != p.group.Id || consumed.TargetChatID != rec.TargetChatID {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}

	writeCtx, cancel := context.WithTimeout(context.Background(), settingsCopyWriteTimeout)
	defer cancel()
	res := settingsCopyApply(writeCtx, consumed.SourceChatID, consumed.TargetChatID, p.query.From.Id)
	if res.Outcome != copySettingsSaved {
		return settingsCopyRedrawWithNotice(p, settingsNoticeWarn(settingsText(p.tr, "common_settings_save_failed")))
	}
	escapedTarget := html.EscapeString(settingsCopyTitleOrID(targetChat.Title, consumed.TargetChatID))
	var notice string
	if res.MessagesSkipped > 0 {
		notice = settingsNoticeOK(settingsText(p.tr, "settings_notice_copied_skipped", i18n.TranslationParams{
			"target": escapedTarget,
			"n":      res.MessagesSkipped,
		}))
	} else {
		notice = settingsNoticeOK(settingsText(p.tr, "settings_notice_copied", i18n.TranslationParams{
			"target": escapedTarget,
		}))
	}
	return settingsCopyRedrawWithNotice(p, notice)
}

// renderSettingsCopyPicker draws the paginated target group picker (cp / cg). Pure.
func renderSettingsCopyPicker(v settingsView, groups []adminindex.ListedGroup, page int, notice string) settingsScreen {
	k := newSettingsKeyboard(v)
	if len(groups) == 0 {
		k.navRow(false)
		body := []string{settingsText(v.Tr, "settings_copy_empty")}
		return k.screen(settingsCompose(v, "settings_page_copy", body, notice, false))
	}

	pages := settingsHubPages(len(groups))
	page = min(max(page, 1), pages)
	body := []string{settingsText(v.Tr, "settings_copy_intro")}
	if pages > 1 {
		body = append(body, "", settingsText(v.Tr, "staff_panel_page", i18n.TranslationParams{
			"page":  page,
			"pages": pages,
		}))
	}

	start := (page - 1) * settingsHubPageSize
	end := min(start+settingsHubPageSize, len(groups))
	for _, g := range groups[start:end] {
		fields := map[string]string{
			"a": settingsActCopyPick,
			"t": strconv.FormatInt(g.ChatID, 10),
		}
		if btn, ok := k.button(settingsHubLabel(g.Title, g.ChatID), fields); ok {
			k.row(btn)
		}
	}

	var paging []gotgbot.InlineKeyboardButton
	if page > 1 {
		if btn, ok := k.button(settingsText(v.Tr, "staff_panel_prev"), map[string]string{
			"a": settingsActCopyPage,
			"v": strconv.Itoa(page - 1),
		}); ok {
			paging = append(paging, btn)
		}
	}
	if page < pages {
		if btn, ok := k.button(settingsText(v.Tr, "staff_panel_next"), map[string]string{
			"a": settingsActCopyPage,
			"v": strconv.Itoa(page + 1),
		}); ok {
			paging = append(paging, btn)
		}
	}
	k.row(paging...)
	k.navRow(false)
	return k.screen(settingsCompose(v, "settings_page_copy", body, notice, false))
}

// renderSettingsCopyConfirm draws the pair-bound confirmation card naming escaped source & target
// titles, all 6 overwritten areas, excluded items, and one-time non-linking notice. Pure.
func renderSettingsCopyConfirm(v settingsView, sourceTitle, targetTitle string, sourceID, targetID int64, token string) settingsScreen {
	src := html.EscapeString(settingsCopyTitleOrID(sourceTitle, sourceID))
	dst := html.EscapeString(settingsCopyTitleOrID(targetTitle, targetID))
	body := []string{
		settingsText(v.Tr, "settings_copy_confirm_head", i18n.TranslationParams{
			"source": src,
			"target": dst,
		}),
		"",
		settingsText(v.Tr, "settings_copy_confirm_areas"),
		"",
		settingsText(v.Tr, "settings_copy_confirm_keep"),
	}

	k := newSettingsKeyboard(v)
	var actionRow []gotgbot.InlineKeyboardButton
	if yesBtn, ok := k.button(settingsText(v.Tr, "settings_btn_copy_yes"), map[string]string{
		"a": settingsActCopyYes,
		"k": token,
	}); ok {
		actionRow = append(actionRow, yesBtn)
	}
	if noBtn, ok := k.button(settingsText(v.Tr, "settings_btn_no"), map[string]string{
		"a": settingsActCancel,
		"p": settingsPageCopy,
	}); ok {
		actionRow = append(actionRow, noBtn)
	}
	k.row(actionRow...)
	k.navRow(false)
	return k.screen(settingsCompose(v, "settings_page_copy", body, "", false))
}
