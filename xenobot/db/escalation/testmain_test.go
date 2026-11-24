//go:build testtools

package escalation

import (
	"os"
	"testing"

	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m, &models.ChatEscalation{}, &models.ChatOffence{}))
}
