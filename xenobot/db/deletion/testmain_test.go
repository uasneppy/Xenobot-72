//go:build testtools

package deletion

import (
	"os"
	"testing"

	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m,
		&models.DeleteRule{},
		&models.ScheduledDeletion{},
		&models.StaffGroup{},
		&models.StaffGroupLink{},
		&models.GreetingSettings{},
	))
}
