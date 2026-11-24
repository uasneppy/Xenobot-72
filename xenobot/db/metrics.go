package db

import (
	"errors"

	"gorm.io/gorm"

	"github.com/uasneppy/Xenobot-72/xenobot/utils/metrics"
)

// registerQueryMetrics counts every GORM statement in xenobot_db_queries_total,
// labelled by operation only.
func registerQueryMetrics(database *gorm.DB) error {
	count := func(operation string) func(*gorm.DB) {
		counter := metrics.DBQueries.WithLabelValues(operation)
		return func(*gorm.DB) { counter.Inc() }
	}
	cb := database.Callback()
	return errors.Join(
		cb.Query().After("gorm:query").Register("xenobot:metrics_query", count("query")),
		cb.Create().After("gorm:create").Register("xenobot:metrics_create", count("create")),
		cb.Update().After("gorm:update").Register("xenobot:metrics_update", count("update")),
		cb.Delete().After("gorm:delete").Register("xenobot:metrics_delete", count("delete")),
		cb.Row().After("gorm:row").Register("xenobot:metrics_row", count("row")),
		cb.Raw().After("gorm:raw").Register("xenobot:metrics_raw", count("raw")),
	)
}
