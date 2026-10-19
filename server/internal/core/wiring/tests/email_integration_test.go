//go:build integration

package wiring_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/core/wiring"
	"quizzivy/internal/platform/config"
	"quizzivy/internal/platform/db"
)

func TestOrganizationBrandingHeadsTheMailWithTheStoredShortName(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil {
			t.Errorf("rollback: %v", err)
		}
	})
	if _, err := tx.Exec(ctx, `UPDATE app.organization_settings SET name = 'Trung tâm Anh ngữ Ánh Dương', short_name = 'Ánh Dương'`); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{Email: config.Email{SinkDir: t.TempDir()}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dbx := db.NewContext(tx)
	mail, err := wiring.BuildMail(cfg, dbx, wiring.OrganizationBranding(cfg, dbx, logger), logger)
	if err != nil {
		t.Fatalf("BuildMail: %v", err)
	}
	got, err := mail.Dispatcher.Org(ctx)
	if err != nil {
		t.Fatalf("Org: %v", err)
	}
	if got != "Ánh Dương" {
		t.Errorf("Org = %q, want the stored short name", got)
	}
}
