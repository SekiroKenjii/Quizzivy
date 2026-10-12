//go:build integration

package db_test

import (
	"database/sql"
	"testing"
	"time"
)

func dropScratchDatabase(t *testing.T, admin *sql.DB, name string) {
	t.Helper()
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		if _, err = admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`); err == nil {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Errorf("dropping %s: %v", name, err)
}
