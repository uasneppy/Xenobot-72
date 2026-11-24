package xenobot

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/uasneppy/Xenobot-72/xenobot/db"
)

type xenobotTestBotClient struct{}

func (xenobotTestBotClient) RequestWithContext(_ context.Context, _ string, method string, _ map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	if method == "getMe" {
		return json.RawMessage(`{"id":999,"is_bot":true,"first_name":"Xenobot-72","username":"XenobotTestBot"}`), nil
	}
	return json.RawMessage(`true`), nil
}

func (xenobotTestBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return "https://api.telegram.org"
}

func (xenobotTestBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return "https://api.telegram.org/file/bot" + token + "/" + path
}

var (
	xenobotMainDBOnce sync.Once
	xenobotMainDBErr  error
)

func setupXenobotMainDB(t *testing.T) {
	t.Helper()

	xenobotMainDBOnce.Do(func() {
		if db.DB == nil {
			db.DB, xenobotMainDBErr = gorm.Open(
				sqlite.Open("file:xenobot_main_test?mode=memory&cache=shared"),
				&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
			)
			if xenobotMainDBErr != nil {
				return
			}
		}
		xenobotMainDBErr = db.DB.AutoMigrate(&db.User{})
	})
	if xenobotMainDBErr != nil {
		t.Fatalf("setup xenobot main DB: %v", xenobotMainDBErr)
	}
}

func TestInitialChecksEnsuresBot(t *testing.T) {
	setupXenobotMainDB(t)
	bot := &gotgbot.Bot{
		Token:     "999:test",
		BotClient: xenobotTestBotClient{},
		User: gotgbot.User{
			Id:        999,
			IsBot:     true,
			FirstName: "Fallback",
			Username:  "fallback_bot",
		},
	}

	if err := InitialChecks(bot); err != nil {
		t.Fatalf("InitialChecks() error = %v", err)
	}

	var user db.User
	if err := db.DB.Where("user_id = ?", int64(999)).First(&user).Error; err != nil {
		t.Fatalf("bot user was not created: %v", err)
	}
	if user.UserName != "XenobotTestBot" || user.Name != "Xenobot-72" {
		t.Fatalf("bot user = username %q name %q, want XenobotTestBot/Xenobot-72", user.UserName, user.Name)
	}
}

func TestInitialChecksReturnsDatabaseError(t *testing.T) {
	originalDB := db.DB
	t.Cleanup(func() {
		db.DB = originalDB
	})

	var err error
	db.DB, err = gorm.Open(
		sqlite.Open("file:xenobot_main_missing_schema?mode=memory&cache=shared"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("open schema-less database: %v", err)
	}

	bot := &gotgbot.Bot{
		Token:     "999:test",
		BotClient: xenobotTestBotClient{},
		User:      gotgbot.User{Id: 999, IsBot: true, FirstName: "Xenobot-72"},
	}
	if err := InitialChecks(bot); err == nil {
		t.Fatal("InitialChecks() error = nil, want missing-schema error")
	}
}
