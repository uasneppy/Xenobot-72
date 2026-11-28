package backup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
)

const (
	BackupFormatVersion = "1.1"
	legacyFormatVersion = "1.0"
)

type BackupFormat struct {
	Version    string                 `json:"version"`
	ExportedAt time.Time              `json:"exported_at"`
	BotName    string                 `json:"bot_name"`
	ChatID     int64                  `json:"chat_id"`
	ChatName   string                 `json:"chat_name"`
	ExportedBy int64                  `json:"exported_by"`
	Modules    []string               `json:"modules"`
	Data       map[string]interface{} `json:"data"`
}

func NewBackupFormat(chatID int64, chatName string, exportedBy int64, modules []string) *BackupFormat {
	return &BackupFormat{
		Version:    BackupFormatVersion,
		ExportedAt: time.Now().UTC(),
		BotName:    "Xenobot-72",
		ChatID:     chatID,
		ChatName:   chatName,
		ExportedBy: exportedBy,
		Modules:    modules,
		Data:       make(map[string]interface{}),
	}
}

// Validate checks the structure of a backup before it is imported. The bot name is informational
// only: it must be present, and it must stay unchecked against any constant, so that exports made
// under the bot's earlier name keep importing.
func (b *BackupFormat) Validate() error {
	if b == nil {
		return fmt.Errorf("backup cannot be nil")
	}
	if b.Version == "" {
		return fmt.Errorf("backup version is required")
	}
	if b.BotName == "" {
		return fmt.Errorf("bot name is required")
	}
	if b.ChatID == 0 {
		return fmt.Errorf("chat ID is required")
	}
	if len(b.Modules) == 0 {
		return fmt.Errorf("at least one module must be specified")
	}
	if b.Data == nil {
		return fmt.Errorf("data field cannot be nil")
	}
	seen := make(map[string]struct{}, len(b.Modules))
	for _, module := range b.Modules {
		if !IsValidModule(module) {
			return fmt.Errorf("unknown module: %s", module)
		}
		if _, dup := seen[module]; dup {
			return fmt.Errorf("duplicate module: %s", module)
		}
		seen[module] = struct{}{}
		if _, ok := b.Data[module]; !ok {
			return fmt.Errorf("missing data for module: %s", module)
		}
	}
	return nil
}

func (b *BackupFormat) IsCompatibleVersion() bool {
	return b.Version == BackupFormatVersion || b.Version == legacyFormatVersion
}

func (b *BackupFormat) ToJSON() ([]byte, error) {
	return json.MarshalIndent(b, "", "  ")
}

func BackupFormatFromJSON(data []byte) (*BackupFormat, error) {
	var backup BackupFormat
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&backup); err != nil {
		return nil, fmt.Errorf("failed to parse backup file: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("failed to parse backup file: trailing data")
	}
	return &backup, nil
}

const (
	BackupModuleAdmin       = "admin"
	BackupModuleAntiflood   = "antiflood"
	BackupModuleAntiraid    = "antiraid"
	BackupModuleApprovals   = "approvals"
	BackupModuleBlacklists  = "blacklists"
	BackupModuleCaptcha     = "captcha"
	BackupModuleConnections = "connections"
	BackupModuleDisabling   = "disabling"
	BackupModuleFilters     = "filters"
	BackupModuleGreetings   = "greetings"
	BackupModuleLocks       = "locks"
	BackupModuleNotes       = "notes"
	BackupModulePins        = "pins"
	BackupModuleReactions   = "reactions"
	BackupModuleReports     = "reports"
	BackupModuleRules       = "rules"
	BackupModuleWarns       = "warns"
	BackupModuleFederations = "federations"
	BackupModuleLogChannels = "logchannels"
	BackupModuleMessages    = "messages"
	BackupModulePresence    = "presence"
)

func AllExportableModules() []string {
	return []string{
		BackupModuleAdmin,
		BackupModuleAntiflood,
		BackupModuleAntiraid,
		BackupModuleApprovals,
		BackupModuleBlacklists,
		BackupModuleCaptcha,
		BackupModuleConnections,
		BackupModuleDisabling,
		BackupModuleFilters,
		BackupModuleGreetings,
		BackupModuleLocks,
		BackupModuleNotes,
		BackupModulePins,
		BackupModuleReactions,
		BackupModuleReports,
		BackupModuleRules,
		BackupModuleWarns,
		BackupModuleFederations,
		BackupModuleLogChannels,
		BackupModuleMessages,
		BackupModulePresence,
	}
}

func IsValidModule(module string) bool {
	for _, m := range AllExportableModules() {
		if m == module {
			return true
		}
	}
	return false
}

type AdminBackup struct {
	AdminSettings      *models.AdminSettings          `json:"admin_settings,omitempty"`
	AntifloodSettings  *models.AntifloodSettings      `json:"antiflood_settings,omitempty"`
	BlacklistMode      string                         `json:"blacklist_mode,omitempty"`
	CaptchaSettings    *models.CaptchaSettings        `json:"captcha_settings,omitempty"`
	ConnectionSettings *models.ConnectionChatSettings `json:"connection_settings,omitempty"`
}

type SettingBackup[T any] struct {
	Settings *T `json:"settings,omitempty"`
}

type AntifloodBackup = SettingBackup[models.AntifloodSettings]

type BlacklistsBackup struct {
	Settings      *models.BlacklistSettings  `json:"settings,omitempty"`
	BlacklistMode string                     `json:"blacklist_mode,omitempty"`
	Entries       []models.BlacklistSettings `json:"entries,omitempty"`
}

type CaptchaBackup = SettingBackup[models.CaptchaSettings]

type ConnectionsBackup = SettingBackup[models.ConnectionChatSettings]

type DisablingBackup struct {
	ChatSettings *models.DisableChatSettings `json:"chat_settings,omitempty"`
	Commands     []models.DisableSettings    `json:"commands,omitempty"`
}

type FiltersBackup struct {
	Filters []models.ChatFilters `json:"filters,omitempty"`
}

type GreetingsBackup = SettingBackup[models.GreetingSettings]

type LocksBackup struct {
	Locks []models.LockSettings `json:"locks,omitempty"`
}

type NotesBackup struct {
	Settings *models.NotesSettings `json:"settings,omitempty"`
	Notes    []models.Notes        `json:"notes,omitempty"`
}

type PinsBackup = SettingBackup[models.PinSettings]

type ReportsBackup = SettingBackup[models.ReportChatSettings]

type RulesBackup = SettingBackup[models.RulesSettings]

type WarnsBackup struct {
	WarnSettings *models.WarnSettings `json:"warn_settings,omitempty"`
	Warns        []models.Warns       `json:"warns,omitempty"`
}

type AntiraidBackup = SettingBackup[models.AntiRaidSettings]

type ApprovalsBackup struct {
	ApprovedUsers []models.ApprovedUsers `json:"approved_users,omitempty"`
}

type ReactionsBackup struct {
	Reactions []models.Reactions `json:"reactions,omitempty"`
}

type FederationsBackup struct {
	Membership *models.FederationChat `json:"membership,omitempty"`
}

type LogChannelsBackup = SettingBackup[models.LogChannel]

type MessagesBackup struct {
	Messages []models.ChatEventMessage       `json:"messages,omitempty"`
	Options  *models.ChatEventMessageOptions `json:"options,omitempty"`
}

// PresencePortableBackup holds the portable fields of chat_presence, explicitly
// omitting archive_chat_id and archive_thread_id so destination targets never leak
// or get overwritten across chats.
type PresencePortableBackup struct {
	ConsensusEnabled   bool   `json:"consensus_enabled"`
	ConsensusThreshold int    `json:"consensus_threshold"`
	RetractEnabled     bool   `json:"retract_enabled"`
	AIReactMode        string `json:"ai_react_mode"`
	CatStickerEnabled  bool   `json:"cat_sticker_enabled"`
	ArchiveEnabled     bool   `json:"archive_enabled"`
	ArchiveThreshold   int    `json:"archive_threshold"`
	ArchiveEmojiMode   string `json:"archive_emoji_mode"`
	ArchiveDelivery    string `json:"archive_delivery"`
	AmbientContext     bool   `json:"ambient_context"`
	VocativeEnabled    bool   `json:"vocative_enabled"`
	PMBriefReact       bool   `json:"pm_brief_react"`
	ReferendumEnabled  bool   `json:"referendum_enabled"`
	InspectEnabled     bool   `json:"inspect_enabled"`
	// The personality fields are omitted at their defaults, so a group that never set them exports
	// the file it always did and an old file imports them as defaults.
	ReactTextFreq    int    `json:"react_text_freq,omitempty"`
	ReactStickerFreq int    `json:"react_sticker_freq,omitempty"`
	ReactPhotoFreq   int    `json:"react_photo_freq,omitempty"`
	ReactTopics      string `json:"react_topics,omitempty"`
	BotNames         string `json:"bot_names,omitempty"`
}

type PresenceBackup = SettingBackup[PresencePortableBackup]
