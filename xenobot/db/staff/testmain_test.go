//go:build testtools

package staff

import (
	"os"
	"testing"

	"github.com/uasneppy/Xenobot-72/xenobot/db/models"
	"github.com/uasneppy/Xenobot-72/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Run(m, &models.StaffGroup{}, &models.StaffGroupLink{}, &models.StaffAction{}, &models.StaffActionGroup{}, &models.StaffForwardBlock{}, &models.StaffForwardBlockSettings{}))
}
