package modules

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/uasneppy/fuku_robot/fuku/db/backup"
	"github.com/uasneppy/fuku_robot/fuku/i18n"
	"github.com/uasneppy/fuku_robot/fuku/utils/ratelimit"
)

// The Backup page (page ba, Phase 30, BKP-01..03, MSG-10): private-only Backup, Restore, and
// Reset controls inside /settings. Hidden in Staff Groups, with a static zero-I/O Main state.
// Module selection is stateless via a canonical 20-bit lowercase hex mask (m=0..fffff). Export
// (be) requires settingsRightAdminBotAdmin and delivers the .json document to the presser's
// private chat under AcquireExport. Import (bi -> private document wait -> bj) and Reset
// (bq -> bu) require live settingsRightOwnerBotAdmin, store cross-replica one-time Redis tokens
// (fuku:settings:backup:{import,reset}:<token>, TTL 5m, 2 MiB cap, P30-01), verify user/chat
// binding before atomic Lua GET+DEL consumption, and enforce AcquireImport / AcquireReset.

const (
	// settingsPageBackup is the p value of the Backup page.
	settingsPageBackup = "ba"

	// Action codes of the Backup page.
	settingsActBackupToggle       = "bv"
	settingsActBackupExport       = "be"
	settingsActBackupImport       = "bi"
	settingsActBackupImportCancel = "bl"
	settingsActBackupImportYes    = "bj"
	settingsActBackupResetAsk     = "bq"
	settingsActBackupResetYes     = "bu"

	// settingsBackupImportPart is the typed-reply wait part for uploading a backup JSON file.
	settingsBackupImportPart = "i"
)

var (
	// settingsBackupAllMask selects all exportable backup modules derived from backup.AllExportableModules().
	settingsBackupAllMask = computeSettingsBackupAllMask(len(backup.AllExportableModules()))

	// settingsBackupMaskMaxHex is the maximum lowercase hex length of settingsBackupAllMask.
	settingsBackupMaskMaxHex = len(strconv.FormatUint(uint64(settingsBackupAllMask), 16))

	// settingsBackupStaffLookup reads a chat's Staff Group row fresh; a variable only so tests can fail it.
	settingsBackupStaffLookup = settingsCopyDefaultStaffLookup
)

func computeSettingsBackupAllMask(count int) uint32 {
	if count <= 0 || count > 32 {
		panic(fmt.Sprintf("settings backup module count %d out of uint32 mask range [1..32]", count))
	}
	if count == 32 {
		return ^uint32(0)
	}
	return (uint32(1) << count) - 1
}

func init() {
	private := func(check func(map[string]string) bool) func(map[string]string) bool {
		return func(fields map[string]string) bool { return fields["c"] != "" && check(fields) }
	}
	registerSettingsPage(settingsPage{
		Key:            settingsPageBackup,
		Order:          83,
		Icon:           "💾",
		TitleKey:       "settings_page_backup",
		MainState:      settingsBackupMainState,
		Open:           settingsBackupOpen,
		Hidden:         settingsBackupHidden,
		PrivateOnly:    true,
		PrivateNoteKey: "settings_bkp_group_note",
		Actions: map[string]settingsPageAction{
			settingsActBackupToggle: {
				Fields: []string{"m"}, Right: settingsRightAdmin, Run: settingsBackupRunToggle,
				Validate: private(settingsBackupMaskFieldValid),
			},
			settingsActBackupExport: {
				Fields: []string{"m"}, Right: settingsRightAdminBotAdmin, Run: settingsBackupRunExport,
				Validate: private(settingsBackupMaskFieldValid),
			},
			settingsActBackupImport: {
				Right: settingsRightOwnerBotAdmin, Run: settingsBackupRunImportPrompt,
				Validate: private(func(map[string]string) bool { return true }),
			},
			settingsActBackupImportCancel: {
				Fields: []string{"t"}, Right: settingsRightAdmin, Run: settingsBackupRunImportCancel,
				Validate: private(func(f map[string]string) bool { return f["t"] == settingsBackupImportPart }),
			},
			settingsActBackupImportYes: {
				Fields: []string{"k"}, Right: settingsRightOwnerBotAdmin, Run: settingsBackupRunImportYes,
				Validate: private(settingsCopyYesValid),
			},
			settingsActBackupResetAsk: {
				Fields: []string{"m"}, Right: settingsRightOwnerBotAdmin, Run: settingsBackupRunResetAsk,
				Validate: private(settingsBackupMaskFieldValid),
			},
			settingsActBackupResetYes: {
				Fields: []string{"k"}, Right: settingsRightOwnerBotAdmin, Run: settingsBackupRunResetYes,
				Validate: private(settingsCopyYesValid),
			},
		},
	})
	settingsTypedPages[settingsPageBackup] = settingsTypedPage{
		Part:         settingsBackupImportPart,
		CancelCode:   settingsActBackupImportCancel,
		Right:        settingsRightOwnerBotAdmin,
		Typed:        settingsBackupImportTyped,
		WaitEndedKey: "settings_bkp_import_wait_ended",
	}
}

func settingsBackupModules() []string {
	return backup.AllExportableModules()
}

func settingsBackupFormatMask(mask uint32) string {
	return strconv.FormatUint(uint64(mask&settingsBackupAllMask), 16)
}

func settingsBackupParseMask(raw string) (uint32, bool) {
	if raw == "" || len(raw) > settingsBackupMaskMaxHex {
		return 0, false
	}
	v, err := strconv.ParseUint(raw, 16, 32)
	if err != nil || v > uint64(settingsBackupAllMask) {
		return 0, false
	}
	mask := uint32(v)
	if settingsBackupFormatMask(mask) != raw {
		return 0, false
	}
	return mask, true
}

func settingsBackupMaskFieldValid(fields map[string]string) bool {
	_, ok := settingsBackupParseMask(fields["m"])
	return ok
}

func settingsBackupMaskModules(mask uint32) []string {
	mods := settingsBackupModules()
	out := make([]string, 0, len(mods))
	for i, mod := range mods {
		if i < 32 && (mask&(uint32(1)<<uint(i))) != 0 {
			out = append(out, mod)
		}
	}
	return out
}

func settingsBackupModuleLabel(tr *i18n.Translator, mod string) string {
	if label := settingsText(tr, "backup_module_"+mod); label != "" {
		return label
	}
	return mod
}

// settingsBackupHidden hides Backup in Staff Groups (definite fresh row).
func settingsBackupHidden(group *gotgbot.Chat) bool {
	row, err := settingsBackupStaffLookup(group.Id)
	return err == nil && row != nil
}

// settingsBackupVerifyNonStaff enforces fresh non-Staff-Group status, failing closed on DB errors.
func settingsBackupVerifyNonStaff(p *settingsPress) bool {
	row, err := settingsBackupStaffLookup(p.group.Id)
	if err != nil {
		log.Warnf("[Settings] fresh staff check of backup chat %d failed: %v", p.group.Id, err)
		p.answer(settingsText(p.tr, "settings_check_failed"), true)
		return false
	}
	if row != nil {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return false
	}
	return true
}

// settingsBackupMainState is static with 0 DB reads and 0 Telegram calls.
func settingsBackupMainState(_ *gotgbot.Bot, tr *i18n.Translator, _ *gotgbot.Chat) (string, bool) {
	return settingsText(tr, "settings_state_backup"), true
}

// settingsBackupOpen draws the Backup page with all 20 modules selected.
func settingsBackupOpen(p *settingsPress, notice string) (settingsScreen, error) {
	return renderSettingsBackup(p.view(), settingsBackupAllMask, notice), nil
}

// settingsBackupRunToggle serves bv: stateless bitmask redraw.
func settingsBackupRunToggle(p *settingsPress) *settingsScreen {
	mask, ok := settingsBackupParseMask(p.fields["m"])
	if !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	screen := renderSettingsBackup(p.view(), mask, "")
	return &screen
}

// settingsBackupRunExport serves be: exports chosen modules to the presser's private chat.
func settingsBackupRunExport(p *settingsPress) *settingsScreen {
	if !settingsBackupVerifyNonStaff(p) {
		return nil
	}
	mask, ok := settingsBackupParseMask(p.fields["m"])
	if !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	if mask == 0 {
		p.answer(settingsText(p.tr, "settings_bkp_pick_one"), true)
		return nil
	}
	var modules []string
	if mask != settingsBackupAllMask {
		modules = settingsBackupMaskModules(mask)
	}
	res := applyBackupExport(p.b, p.tr, p.group.Id, p.group.Title, p.query.From.Id, modules)
	var notice string
	switch res.Outcome {
	case backupExportSent:
		notice = settingsNoticeOK(settingsText(p.tr, "backup_export_success_text", i18n.TranslationParams{
			"modules": strconv.Itoa(res.ModuleCount),
		}))
	case backupExportRateLimited:
		notice = settingsNoticeWarn(settingsText(p.tr, "backup_export_rate_limited", i18n.TranslationParams{
			"time": ratelimit.FormatCooldown(res.Cooldown),
		}))
	case backupExportStartBot:
		notice = settingsNoticeWarn(settingsText(p.tr, "backup_export_start_bot"))
	case backupExportNoModules:
		notice = settingsNoticeWarn(settingsText(p.tr, "backup_export_no_modules"))
	default:
		notice = settingsNoticeWarn(settingsText(p.tr, "backup_export_failed"))
	}
	screen := renderSettingsBackup(p.view(), mask, notice)
	return &screen
}

// settingsBackupRunImportPrompt serves bi: starts the private JSON document wait.
func settingsBackupRunImportPrompt(p *settingsPress) *settingsScreen {
	if !settingsBackupVerifyNonStaff(p) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	nonce, err := newSettingsInputNonce()
	if err != nil {
		log.Errorf("[Settings] no nonce for backup import wait: %v", err)
		p.answer(settingsText(p.tr, "settings_edit_unavailable"), true)
		return nil
	}
	until := time.Now().Add(settingsInputTTL)
	previous, err := settingsInputSet(ctx, p.query.From.Id, settingsInputWait{
		GroupID:     p.group.Id,
		Page:        settingsPageBackup,
		Part:        settingsBackupImportPart,
		PromptMsgID: p.query.Message.GetMessageId(),
		Nonce:       nonce,
		ExpiresAt:   until,
	})
	if err != nil {
		if !errors.Is(err, errSettingsInputUnavailable) {
			log.Warnf("[Settings] backup import wait for user %d chat %d could not be written: %v", p.query.From.Id, p.group.Id, err)
		}
		p.answer(settingsText(p.tr, "settings_edit_unavailable"), true)
		return nil
	}
	screen := renderSettingsBackupImportPrompt(
		p.view(),
		until,
		settingsEditWaitReplaced(previous, p.group.Id, settingsPageBackup, settingsBackupImportPart),
	)
	return &screen
}

// settingsBackupRunImportCancel serves bl: cancels the active import wait and redraws page ba.
func settingsBackupRunImportCancel(p *settingsPress) *settingsScreen {
	ctx, cancel := context.WithTimeout(context.Background(), settingsReadTimeout)
	defer cancel()
	outcome, err := settingsInputCancel(ctx, p.query.From.Id, p.group.Id, settingsPageBackup, p.fields["t"], p.query.Message.GetMessageId())
	notice := ""
	switch {
	case errors.Is(err, errSettingsInputUnavailable):
		p.answer(settingsText(p.tr, "settings_edit_unavailable"), true)
	case err != nil:
		log.Warnf("[Settings] backup import wait for user %d chat %d could not be cancelled: %v", p.query.From.Id, p.group.Id, err)
		p.answer(settingsText(p.tr, "settings_edit_unavailable"), true)
		return nil
	case outcome == settingsInputCancelDeleted:
		notice = settingsNoticeOK(settingsText(p.tr, "settings_edit_cancelled"))
	case outcome == settingsInputCancelExpired:
		p.answer(settingsText(p.tr, "settings_edit_wait_expired"), true)
	default:
		p.answer(settingsText(p.tr, "settings_edit_prompt_stale"), true)
	}
	screen := renderSettingsBackup(p.view(), settingsBackupAllMask, notice)
	return &screen
}

// settingsBackupImportTyped handles private messages arriving during an active bi import wait.
func settingsBackupImportTyped(r *settingsInputReply) {
	row, err := settingsBackupStaffLookup(r.group.Id)
	if err != nil {
		log.Warnf("[Settings] fresh staff check of backup import chat %d failed: %v", r.group.Id, err)
		r.keep("settings_check_failed")
		return
	}
	if row != nil {
		r.end("settings_edit_refuse_access_lost")
		return
	}
	if r.msg.Document == nil {
		r.refuse(r.text("settings_bkp_import_need_file"))
		return
	}
	bkp, errText := downloadAndParseBackupFile(r.b, r.msg.Document, r.tr)
	if bkp == nil {
		r.refuse(errText)
		return
	}
	var importModules []string
	if caption := strings.TrimSpace(r.msg.Caption); caption != "" {
		var parseErr error
		importModules, parseErr = parseImportModules("/import "+caption, bkp.Data)
		if parseErr != nil {
			r.refuse(r.text("backup_import_invalid_file"))
			return
		}
	}
	if len(importModules) == 0 {
		importModules = bkp.Modules
	}
	if len(importModules) == 0 {
		r.refuse(r.text("backup_import_invalid_file"))
		return
	}
	claim, claimed := r.claimWait()
	if !claimed {
		return
	}
	token, err := settingsBackupPutImportToken(context.Background(), settingsBackupImportRecord{
		UserID:  r.user.Id,
		ChatID:  r.wait.GroupID,
		Modules: importModules,
		Backup:  bkp,
	})
	if err != nil {
		r.releaseWait(claim)
		if errors.Is(err, errSettingsBackupPayloadTooLarge) {
			r.refuse(r.text("settings_bkp_import_file_too_large"))
			return
		}
		log.Warnf("[Settings] backup import token write for user %d chat %d failed: %v", r.user.Id, r.wait.GroupID, err)
		r.refuse(r.text("settings_bkp_unavailable"))
		return
	}
	r.finishWait(claim)
	r.closePrompt()
	screen := renderSettingsBackupImportConfirm(r.view(), importModules, token)
	r.send(screen.Text, &screen.Keyboard, false)
}

func settingsBackupFormatFailNotice(tr *i18n.Translator, key string) string {
	msg := settingsText(tr, key)
	if strings.Contains(msg, "%s") {
		return fmt.Sprintf(msg, settingsText(tr, "common_settings_save_failed"))
	}
	return msg
}

func settingsBackupHandleCardReadErr(p *settingsPress, kind, step string, err error) {
	if errors.Is(err, errSettingsCopyNoRedis) {
		p.answer(settingsText(p.tr, "settings_bkp_unavailable"), true)
		return
	}
	log.Warnf("[Settings] backup %s token %s for user %d chat %d failed: %v", kind, step, p.query.From.Id, p.group.Id, err)
	p.answer(settingsText(p.tr, "settings_check_failed"), true)
}

// settingsBackupClaimCard peeks a pending Redis confirm card, verifies caller/chat binding before
// consuming (so cross-user or cross-chat presses never invalidate the real owner's card), runs
// fresh Staff Group refusal, and consumes the card atomically via Lua GET+DEL.
func settingsBackupClaimCard[T any](
	p *settingsPress,
	kind string,
	peek, consume func(context.Context, string) (T, bool, error),
	valid func(T) bool,
) (T, bool, *settingsScreen) {
	var zero T
	token := p.fields["k"]
	rec, found, err := peek(context.Background(), token)
	if err != nil {
		settingsBackupHandleCardReadErr(p, kind, "peek", err)
		return zero, false, nil
	}
	if !found {
		expired := settingsText(p.tr, "settings_card_expired")
		p.answer(expired, true)
		screen := renderSettingsBackup(p.view(), settingsBackupAllMask, settingsNoticeWarn(expired))
		return zero, false, &screen
	}
	if !valid(rec) {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return zero, false, nil
	}
	if !settingsBackupVerifyNonStaff(p) {
		return zero, false, nil
	}
	consumed, found, err := consume(context.Background(), token)
	if err != nil {
		settingsBackupHandleCardReadErr(p, kind, "consume", err)
		return zero, false, nil
	}
	if !found {
		expired := settingsText(p.tr, "settings_card_expired")
		p.answer(expired, true)
		screen := renderSettingsBackup(p.view(), settingsBackupAllMask, settingsNoticeWarn(expired))
		return zero, false, &screen
	}
	if !valid(consumed) {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return zero, false, nil
	}
	return consumed, true, nil
}

// settingsBackupRunImportYes serves bj: claims the one-time Redis import card atomically via Lua
// GET+DEL, checks AcquireImport, runs applyBackupImport, and redraws page ba.
func settingsBackupRunImportYes(p *settingsPress) *settingsScreen {
	consumed, ok, fallback := settingsBackupClaimCard(
		p,
		"import",
		settingsBackupPeekImportToken,
		settingsBackupConsumeImportToken,
		func(rec settingsBackupImportRecord) bool {
			return rec.UserID == p.query.From.Id && rec.ChatID == p.group.Id && rec.Backup != nil && len(rec.Modules) > 0
		},
	)
	if !ok {
		return fallback
	}
	limiter := ratelimit.GetBackupRateLimiter()
	if allowed, cooldown := limiter.AcquireImport(p.group.Id); !allowed {
		notice := settingsNoticeWarn(settingsText(p.tr, "backup_import_rate_limited", i18n.TranslationParams{
			"time": ratelimit.FormatCooldown(cooldown),
		}))
		screen := renderSettingsBackup(p.view(), settingsBackupAllMask, notice)
		return &screen
	}
	report, err := applyBackupImport(p.group.Id, consumed.Backup, consumed.Modules)
	if err != nil {
		notice := settingsNoticeWarn(settingsBackupFormatFailNotice(p.tr, "backup_import_failed"))
		screen := renderSettingsBackup(p.view(), settingsBackupAllMask, notice)
		return &screen
	}
	notice := settingsNoticeOK(formatBackupImportSuccess(p.tr, consumed.Modules, report))
	screen := renderSettingsBackup(p.view(), settingsBackupAllMask, notice)
	return &screen
}

// settingsBackupRunResetAsk serves bq: stores a one-time Redis reset token and draws the Reset confirm card.
func settingsBackupRunResetAsk(p *settingsPress) *settingsScreen {
	if !settingsBackupVerifyNonStaff(p) {
		return nil
	}
	mask, ok := settingsBackupParseMask(p.fields["m"])
	if !ok {
		p.answer(settingsText(p.tr, "common_callback_invalid_request"), true)
		return nil
	}
	if mask == 0 {
		p.answer(settingsText(p.tr, "settings_bkp_pick_one"), true)
		return nil
	}
	modules := settingsBackupMaskModules(mask)
	token, err := settingsBackupPutResetToken(context.Background(), settingsBackupResetRecord{
		UserID:  p.query.From.Id,
		ChatID:  p.group.Id,
		Modules: modules,
	})
	if err != nil {
		if !errors.Is(err, errSettingsCopyNoRedis) {
			log.Warnf("[Settings] backup reset token write for user %d chat %d failed: %v", p.query.From.Id, p.group.Id, err)
		}
		p.answer(settingsText(p.tr, "settings_bkp_unavailable"), true)
		return nil
	}
	screen := renderSettingsBackupResetConfirm(p.view(), modules, token)
	return &screen
}

// settingsBackupRunResetYes serves bu: claims the one-time Redis reset card atomically via Lua
// GET+DEL, checks AcquireReset, runs applyBackupReset, and redraws page ba.
func settingsBackupRunResetYes(p *settingsPress) *settingsScreen {
	consumed, ok, fallback := settingsBackupClaimCard(
		p,
		"reset",
		settingsBackupPeekResetToken,
		settingsBackupConsumeResetToken,
		func(rec settingsBackupResetRecord) bool {
			return rec.UserID == p.query.From.Id && rec.ChatID == p.group.Id && len(rec.Modules) > 0
		},
	)
	if !ok {
		return fallback
	}
	limiter := ratelimit.GetBackupRateLimiter()
	if allowed, cooldown := limiter.AcquireReset(p.group.Id); !allowed {
		notice := settingsNoticeWarn(settingsText(p.tr, "backup_reset_rate_limited", i18n.TranslationParams{
			"time": ratelimit.FormatCooldown(cooldown),
		}))
		screen := renderSettingsBackup(p.view(), settingsBackupAllMask, notice)
		return &screen
	}
	if err := applyBackupReset(p.group.Id, consumed.Modules); err != nil {
		notice := settingsNoticeWarn(settingsBackupFormatFailNotice(p.tr, "backup_reset_failed"))
		screen := renderSettingsBackup(p.view(), settingsBackupAllMask, notice)
		return &screen
	}
	notice := settingsNoticeOK(formatBackupResetSuccess(p.tr, consumed.Modules))
	screen := renderSettingsBackup(p.view(), settingsBackupAllMask, notice)
	return &screen
}

// renderSettingsBackup draws the private Backup page with a stateless 20-module selector. Pure.
func renderSettingsBackup(v settingsView, mask uint32, notice string) settingsScreen {
	mods := settingsBackupModules()
	selected := len(settingsBackupMaskModules(mask))
	body := []string{
		settingsText(v.Tr, "settings_bkp_intro"),
		"",
		settingsText(v.Tr, "settings_bkp_selected", i18n.TranslationParams{
			"selected": strconv.Itoa(selected),
			"total":    strconv.Itoa(len(mods)),
		}),
	}

	k := newSettingsKeyboard(v)
	var pair []gotgbot.InlineKeyboardButton
	for i, mod := range mods {
		bit := uint32(1) << uint(i)
		icon := "⬜"
		if (mask & bit) != 0 {
			icon = "✅"
		}
		label := icon + " " + settingsBackupModuleLabel(v.Tr, mod)
		if btn, ok := k.button(label, map[string]string{
			"a": settingsActBackupToggle,
			"m": settingsBackupFormatMask(mask ^ bit),
		}); ok {
			pair = append(pair, btn)
			if len(pair) == 2 {
				k.row(pair...)
				pair = nil
			}
		}
	}
	if len(pair) > 0 {
		k.row(pair...)
	}

	var selectRow []gotgbot.InlineKeyboardButton
	if allBtn, ok := k.button(settingsText(v.Tr, "settings_btn_bkp_all"), map[string]string{
		"a": settingsActBackupToggle,
		"m": settingsBackupFormatMask(settingsBackupAllMask),
	}); ok {
		selectRow = append(selectRow, allBtn)
	}
	if noneBtn, ok := k.button(settingsText(v.Tr, "settings_btn_bkp_none"), map[string]string{
		"a": settingsActBackupToggle,
		"m": settingsBackupFormatMask(0),
	}); ok {
		selectRow = append(selectRow, noneBtn)
	}
	k.row(selectRow...)

	maskStr := settingsBackupFormatMask(mask)
	var ioRow []gotgbot.InlineKeyboardButton
	if expBtn, ok := k.button(settingsText(v.Tr, "settings_btn_bkp_export"), map[string]string{
		"a": settingsActBackupExport,
		"m": maskStr,
	}); ok {
		ioRow = append(ioRow, expBtn)
	}
	if impBtn, ok := k.button(settingsText(v.Tr, "settings_btn_bkp_import"), map[string]string{
		"a": settingsActBackupImport,
	}); ok {
		ioRow = append(ioRow, impBtn)
	}
	k.row(ioRow...)

	if resetBtn, ok := k.button(settingsText(v.Tr, "settings_btn_bkp_reset"), map[string]string{
		"a": settingsActBackupResetAsk,
		"m": maskStr,
	}); ok {
		k.row(resetBtn)
	}

	k.navRow(false)
	return k.screen(settingsCompose(v, "settings_page_backup", body, notice, false))
}

// renderSettingsBackupImportPrompt draws the private wait prompt asking for a backup .json document. Pure.
func renderSettingsBackupImportPrompt(v settingsView, until time.Time, replaced bool) settingsScreen {
	return renderSettingsTypedPrompt(v, "settings_page_backup", "settings_bkp_import_wait", "settings_bkp_import_wait_rules",
		settingsActBackupImportCancel, settingsBackupImportPart, until, replaced)
}

// renderSettingsTypedPrompt draws a single-value typed-reply prompt: the wait text, its rules, the end time, the
// replaced note when another wait was replaced, and one Cancel button. Pure.
func renderSettingsTypedPrompt(v settingsView, pageKey, waitKey, rulesKey, cancelAct, part string, until time.Time, replaced bool) settingsScreen {
	body := []string{
		settingsText(v.Tr, waitKey),
		settingsText(v.Tr, rulesKey),
		"",
		settingsText(v.Tr, "settings_edit_wait_footer", i18n.TranslationParams{
			"time": formatChatTime(until, chatClockOf(v.GroupID), "15:04"),
		}),
	}
	if replaced {
		body = append(body, "", settingsText(v.Tr, "settings_edit_replaced_note"))
	}
	k := newSettingsKeyboard(v)
	if cancelBtn, ok := k.button(settingsText(v.Tr, "settings_btn_edit_cancel"), map[string]string{"a": cancelAct, "t": part}); ok {
		k.row(cancelBtn)
	}
	return k.screen(settingsCompose(v, pageKey, body, "", false))
}

// renderSettingsBackupConfirm draws a two-tap Import or Reset confirmation card. Pure.
func renderSettingsBackupConfirm(v settingsView, bodyKey, yesBtnKey, yesAct string, modules []string, token string) settingsScreen {
	body := []string{
		settingsText(v.Tr, bodyKey, i18n.TranslationParams{
			"modules": strconv.Itoa(len(modules)),
			"list":    buildModuleList(modules),
		}),
	}
	k := newSettingsKeyboard(v)
	var actionRow []gotgbot.InlineKeyboardButton
	if yesBtn, ok := k.button(settingsText(v.Tr, yesBtnKey), map[string]string{
		"a": yesAct,
		"k": token,
	}); ok {
		actionRow = append(actionRow, yesBtn)
	}
	if noBtn, ok := k.button(settingsText(v.Tr, "settings_btn_no"), map[string]string{
		"a": settingsActCancel,
		"p": settingsPageBackup,
	}); ok {
		actionRow = append(actionRow, noBtn)
	}
	k.row(actionRow...)
	k.navRow(false)
	return k.screen(settingsCompose(v, "settings_page_backup", body, "", false))
}

// renderSettingsBackupImportConfirm draws the two-tap Import confirmation card. Pure.
func renderSettingsBackupImportConfirm(v settingsView, modules []string, token string) settingsScreen {
	return renderSettingsBackupConfirm(v, "backup_import_confirm", "settings_btn_bkp_import_yes", settingsActBackupImportYes, modules, token)
}

// renderSettingsBackupResetConfirm draws the two-tap Reset confirmation card. Pure.
func renderSettingsBackupResetConfirm(v settingsView, modules []string, token string) settingsScreen {
	return renderSettingsBackupConfirm(v, "backup_reset_confirm", "settings_btn_bkp_reset_yes", settingsActBackupResetYes, modules, token)
}
