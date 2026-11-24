//go:build testtools

package modtuning

import (
	"os"
	"testing"

	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m, &models.ChatManualDefaults{}, &models.ChatReasonShortcut{}))
}
