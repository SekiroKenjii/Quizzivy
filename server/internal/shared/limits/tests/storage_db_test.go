//go:build integration

package limits_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/shared/limits"
)

func rolledBack(t *testing.T) pgx.Tx {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.BeginTx(context.Background(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}

func TestStorageCountsWhatTheBucketsKeepAndRefusesPastTheLimit(t *testing.T) {
	tx := rolledBack(t)
	ctx := context.Background()
	before, err := limits.CountHeld(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	var owner string
	if err := tx.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role_id)
		VALUES ('kho-' || gen_random_uuid() || '@example.test', 'Giáo viên', (SELECT id FROM app.roles WHERE builtin_key = 'teacher'))
		RETURNING id::text`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	for _, purged := range []bool{false, true} {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app.media_assets (kind, display_name, original_filename, mime_type, bytes, checksum_sha256, storage_key, uploaded_by, owner_id, unreferenced_at, purged_at)
			VALUES ('image', 'Ảnh', 'a.png', 'image/png', 4096, sha256(gen_random_uuid()::text::bytea), 'media/' || gen_random_uuid(), $1, $1,
			        CASE WHEN $2 THEN now() END, CASE WHEN $2 THEN now() END)`, owner, purged); err != nil {
			t.Fatal(err)
		}
	}
	held, err := limits.CountHeld(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	if got := held.Media - before.Media; got != 4096 {
		t.Errorf("media moved by %d, want 4096: a purged file is no longer kept", got)
	}
	limit := held.Total() + 100
	if err := limits.RequireStorage(ctx, tx, limit, 100); err != nil {
		t.Errorf("a file that fills the limit exactly: %v, want it taken", err)
	}
	if err := limits.RequireStorage(ctx, tx, limit, 101); !errors.Is(err, limits.ErrStorageLimit) {
		t.Errorf("a file one byte past the limit: %v, want ErrStorageLimit", err)
	}
	if err := limits.RequireStorage(ctx, tx, held.Total()-1, 0); !errors.Is(err, limits.ErrStorageLimit) {
		t.Errorf("a replace over a lowered limit: %v, want ErrStorageLimit", err)
	}
}

func TestTheStoredLimitsReadBackAndADefaultIsNull(t *testing.T) {
	tx := rolledBack(t)
	ctx := context.Background()
	if _, err := tx.Exec(ctx, `UPDATE app.organization_settings SET limit_storage_bytes = $1, warn_at_percent = 75, limit_seats = NULL`, 20*limits.GiB); err != nil {
		t.Fatal(err)
	}
	got, err := limits.Read(ctx, tx, limits.Env{limits.EmailsPerDay: 50})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := got.Of(limits.StorageBytes); v != 20*limits.GiB || !got[limits.StorageBytes].Custom {
		t.Errorf("storage = %+v, want the stored 20 GiB", got[limits.StorageBytes])
	}
	if got.WarnPercent() != 75 {
		t.Errorf("warn at = %d, want 75", got.WarnPercent())
	}
	if v, _ := got.Of(limits.EmailsPerDay); v != 50 {
		t.Errorf("emails per day = %d, want the deploy's 50", v)
	}
	if _, ok := got.Of(limits.Seats); ok {
		t.Error("a null seat limit must be no limit")
	}
	for _, c := range []struct {
		column string
		value  int64
	}{
		{"limit_seats", 0}, {"limit_storage_bytes", limits.MiB - 1}, {"warn_at_percent", 49}, {"limit_emails_per_day", 100_001},
	} {
		_, err := tx.Exec(ctx, `SAVEPOINT bad`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE app.organization_settings SET `+c.column+` = $1`, c.value); err == nil {
			t.Errorf("%s = %d was stored, want the range check to refuse it", c.column, c.value)
		}
		if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT bad`); err != nil {
			t.Fatal(err)
		}
	}
}
