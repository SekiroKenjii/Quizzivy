//go:build integration

package repositories_test

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/audit/application"
	"quizzivy/internal/modules/audit/application/command"
	"quizzivy/internal/modules/audit/application/query"
	"quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/modules/audit/repositories"
	"quizzivy/internal/platform/db"
)

func appRoleApplication(t *testing.T) *application.Application {
	t.Helper()
	dsn := os.Getenv("APP_DATABASE_URL")
	if dsn == "" {
		t.Skip("APP_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var role string
	if err := pool.QueryRow(context.Background(), `SELECT current_user`).Scan(&role); err != nil || role != "quizzivy_app" {
		t.Fatalf("connected as %q: %v", role, err)
	}
	app := application.New(repositories.NewPostgres(db.NewContext(pool)))
	app.SetClock(func() time.Time { return march(15, 12, 0) })
	return app
}

func TestTheListTheLabelsAndTheExportAnswerTheSameToTheAppRole(t *testing.T) {
	w := newWorld(t)
	app := appRoleApplication(t)
	f := newFixtures(w)
	actorName := "Người làm " + w.nonce
	actorID := w.user("teacher", actorName, "")

	var kinds int
	for name, fixture := range f.all() {
		id := fixture.id("Nhãn " + name + " " + w.nonce)
		w.write(entry{actor: actorID, entity: fixture.entity, entityID: id, at: march(10, 9, 0), ip: "198.51.100.7",
			userAgent: "Mozilla/5.0 (X11; Linux x86_64) Firefox/120.0", diff: `{"name": {"old": "a", "new": "b"}, "code": "SECRET1234"}`})
		kinds++
	}
	ctx := context.Background()
	list := inMarch()
	list.Query = actorName

	viaOwner := w.list(list)
	viaApp, err := app.Queries.List.Handle(ctx, list)
	if err != nil {
		t.Fatalf("as the app role: %v", err)
	}

	if len(viaApp.Entries) != kinds || !slices.Equal(ids(viaApp.Entries), ids(viaOwner.Entries)) {
		t.Errorf("the app role reads %d entries, the owner %d, want %d", len(viaApp.Entries), len(viaOwner.Entries), kinds)
	}
	if viaApp.Facets != viaOwner.Facets {
		t.Errorf("facets %+v, want %+v", viaApp.Facets, viaOwner.Facets)
	}
	for _, e := range viaApp.Entries {
		if e.Label == nil {
			t.Errorf("a %s entry has no label for the app role", e.EntityType)
		}
	}

	plan, err := app.Commands.StartExport.Handle(ctx, command.StartExport{Actor: w.exporter(""), Period: domain.Year, Format: "csv", Query: actorName})
	if err != nil {
		t.Fatalf("export as the app role: %v", err)
	}
	if plan.Rows != kinds {
		t.Errorf("%d rows counted, want %d", plan.Rows, kinds)
	}
	batch, err := app.Queries.ExportBatch.Handle(ctx, query.ExportBatch{Filter: plan.Filter})
	if err != nil || len(batch.Entries) != kinds {
		t.Errorf("%d entries read as the app role: %v", len(batch.Entries), err)
	}
}
