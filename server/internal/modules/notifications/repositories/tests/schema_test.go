//go:build integration

package repositories_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/platform/db"
)

func TestTheTableRefusesWhatNoReaderCouldBeSent(t *testing.T) {
	pool := newPool(t)
	userID := newUser(t, pool, "teacher")
	insert := func(kind, params string, target *string, key string) error {
		_, err := pool.Exec(context.Background(), `
			INSERT INTO app.notifications (user_id, kind, params, target, dedupe_key)
			VALUES ($1::uuid, $2, $3::text::jsonb, $4::text::jsonb, $5)`, userID, kind, params, target, key)
		return err
	}
	const (
		kind   = "result.ready"
		params = `{"title":"Đề giữa kỳ"}`
	)
	padded := func(stored int) string {
		return `{"title":"` + strings.Repeat("a", stored-len(`{"title": ""}`)) + `"}`
	}
	for label, c := range map[string]struct {
		kind, params string
		target       *string
		key          string
		constraint   string
	}{
		"params that are a list":        {kind, `[]`, nil, "k1", "notifications_params_shape"},
		"params that are a string":      {kind, `"<b>Đề</b>"`, nil, "k2", "notifications_params_shape"},
		"params over four kilobytes":    {kind, padded(4097), nil, "k3", "notifications_params_shape"},
		"a target that is JSON null":    {kind, params, new("null"), "k4", "notifications_target_shape"},
		"a target that is a string":     {kind, params, new(`"/teacher/classes"`), "k5", "notifications_target_shape"},
		"a target over one kilobyte":    {kind, params, new(padded(1025)), "k6", "notifications_target_shape"},
		"a kind without a dot":          {"submitted", params, nil, "k7", "notifications_kind_check"},
		"a kind in capitals":            {"Attempt.Submitted", params, nil, "k8", "notifications_kind_check"},
		"a kind over 64 characters":     {"attempt." + strings.Repeat("a", 57), params, nil, "k9", "notifications_kind_check"},
		"an empty dedupe key":           {kind, params, nil, "", "notifications_dedupe_key_check"},
		"a dedupe key over 200":         {kind, params, nil, strings.Repeat("k", 201), "notifications_dedupe_key_check"},
		"a dedupe key of 201 letters ế": {kind, params, nil, strings.Repeat("ế", 201), "notifications_dedupe_key_check"},
	} {
		if err := insert(c.kind, c.params, c.target, c.key); !db.IsCheckViolation(err, c.constraint) {
			t.Errorf("%s: err = %v, want %s to refuse it", label, err, c.constraint)
		}
	}
	for label, c := range map[string]struct {
		kind, params string
		target       *string
		key          string
	}{
		"params of exactly four kilobytes": {kind, padded(4096), nil, "ok1"},
		"a target of exactly one kilobyte": {kind, params, new(padded(1024)), "ok2"},
		"no target":                        {kind, params, nil, "ok3"},
		"a kind of 64 characters":          {"attempt." + strings.Repeat("a", 56), params, nil, "ok4"},
		"a kind with an underscore":        {"join_codes.rotated", `{}`, nil, "ok5"},
		"a dedupe key of 200 letters ế":    {kind, params, nil, strings.Repeat("ế", domain.MaxDedupeKey)},
	} {
		if err := insert(c.kind, c.params, c.target, c.key); err != nil {
			t.Errorf("%s: refused with %v", label, err)
		}
	}
	if err := insert(kind, params, nil, "ok3"); !db.IsUniqueViolation(err, "notifications_user_dedupe_key") {
		t.Errorf("a second row under the same user and key: err = %v, want notifications_user_dedupe_key to refuse it", err)
	}
}

func TestASwitchIsOneOfTheFiveEvents(t *testing.T) {
	pool := newPool(t)
	userID := newUser(t, pool, "teacher")
	insert := func(event string) error {
		_, err := pool.Exec(context.Background(), `
			INSERT INTO app.notification_preferences (user_id, event, in_app) VALUES ($1::uuid, $2, true)`, userID, event)
		return err
	}
	for _, event := range []string{"class.joined", "assignment.opened", "summary.weekly", ""} {
		if err := insert(event); !db.IsCheckViolation(err, "notification_preferences_event_check") {
			t.Errorf("event %q: err = %v, want notification_preferences_event_check to refuse it", event, err)
		}
	}
	for _, event := range domain.Events() {
		if err := insert(string(event)); err != nil {
			t.Errorf("event %s: refused with %v", event, err)
		}
	}
	if err := insert(string(domain.EventResultReady)); !db.IsUniqueViolation(err, "notification_preferences_pkey") {
		t.Errorf("a second row for one switch: err = %v, want the primary key to refuse it", err)
	}
	var email bool
	if err := pool.QueryRow(context.Background(), `
		SELECT email FROM app.notification_preferences WHERE user_id = $1::uuid AND event = 'result.ready'`, userID).Scan(&email); err != nil || email {
		t.Errorf("email defaults to %v (%v), want false", email, err)
	}
}

func TestAUsersNotificationsAndSwitchesGoWithTheUser(t *testing.T) {
	pool := newPool(t)
	app := over(pool)
	ctx := context.Background()
	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Người sắp bị xoá', (SELECT id FROM app.roles WHERE builtin_key = 'student'))
		RETURNING id::text`, "bi-xoa-"+nonce(t)+"@example.com").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM app.users WHERE id = $1::uuid`, userID) })
	fill(t, app, userID, 2)
	save(t, app, userID, domain.WithDefaults(nil))
	if _, err := pool.Exec(ctx, `DELETE FROM app.users WHERE id = $1::uuid`, userID); err != nil {
		t.Fatalf("deleting a user who holds notifications and switches: %v", err)
	}
	if left := len(rowsOf(t, pool, userID)); left != 0 || savedRows(t, pool, userID) != 0 {
		t.Errorf("%d notifications and %d switches outlived their user", left, savedRows(t, pool, userID))
	}
}

func TestTheIndexesTheQueriesLeanOnExist(t *testing.T) {
	pool := newPool(t)
	rows, err := pool.Query(context.Background(), `
		SELECT indexname || ': ' || regexp_replace(indexdef, '^.* USING ', '')
		  FROM pg_indexes WHERE schemaname = 'app' AND tablename IN ('notifications', 'notification_preferences')`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	want := []string{
		"notification_preferences_pkey: btree (user_id, event)",
		"notifications_created_at_idx: btree (created_at)",
		"notifications_pkey: btree (id)",
		"notifications_unread_idx: btree (user_id) WHERE (read_at IS NULL)",
		"notifications_user_dedupe_key: btree (user_id, dedupe_key)",
		"notifications_user_recent_idx: btree (user_id, id DESC)",
	}
	if !slices.Equal(got, want) {
		t.Errorf("the indexes are\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}
