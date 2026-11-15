package store

import (
	"database/sql/driver"
	"fmt"
	"time"
	_ "time/tzdata" // embed zone rules so the binary does not depend on the host

	"modernc.org/sqlite"
)

// Local is the station's time zone. Instants are stored in UTC; calendar days
// (records, date-only query parameters) are days in this zone.
var Local = mustLoadLocation("America/Chicago")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("store: load %s: %v", name, err))
	}
	return loc
}

// LocalDay formats the station-local calendar day of t as "2006-01-02".
func LocalDay(t time.Time) string { return t.In(Local).Format("2006-01-02") }

// SQLite has no time zone database, so local_day(obs_time) is a SQL function
// backed by Go's. Registered once; every connection of the "sqlite" driver
// opened afterwards can call it.
func init() {
	sqlite.MustRegisterDeterministicScalarFunction("local_day", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			ts, ok := args[0].(int64)
			if !ok {
				return nil, fmt.Errorf("local_day: want integer unix seconds, got %T", args[0])
			}
			return LocalDay(time.Unix(ts, 0)), nil
		})
}
