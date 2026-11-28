//go:build testtools

package membermem

import (
	"os"
	"testing"

	"github.com/uasneppy/Xenobot-72/internal/testdb"
	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m, &models.ChatMemberMemory{}))
}
