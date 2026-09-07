//go:build integration

package application_test

import (
	"context"
	"errors"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"strings"
	"testing"

	"quizzivy/internal/modules/identity/domain"
)

func TestRenamingKeepsEverythingElseAboutTheAccount(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	ctx := context.Background()

	before, err := svc.Queries.CurrentUser.Handle(ctx, query.CurrentUser{UserID: id})
	if err != nil {
		t.Fatal(err)
	}

	after, err := svc.Commands.Rename.Handle(ctx, command.Rename{
		UserID:    id,
		FullName:  "  Nguyễn Đức Minh  ",
		IP:        "203.0.113.4",
		UserAgent: "go-test",
	})
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}

	if after.FullName != "Nguyễn Đức Minh" {
		t.Errorf("name is %q; the surrounding spaces should have gone", after.FullName)
	}
	if after.Email != before.Email || after.Role != before.Role {
		t.Errorf("rename moved more than the name: %+v -> %+v", before, after)
	}

	// And it is the stored name, not just the returned one.
	reread, err := svc.Queries.CurrentUser.Handle(ctx, query.CurrentUser{UserID: id})
	if err != nil {
		t.Fatal(err)
	}
	if reread.FullName != "Nguyễn Đức Minh" {
		t.Errorf("re-read name is %q", reread.FullName)
	}

	// The password still works: renaming is not a credential change.
	if _, err := svc.Commands.Login.Handle(ctx, command.Login{Email: email, Password: testPassword}); err != nil {
		t.Errorf("login broke after a rename: %v", err)
	}
}

func TestAnEmptyOrOversizedNameIsRefused(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, _ := makeUser(t, pool)
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		want error
	}{
		{"", domain.ErrNameRequired},
		{"   ", domain.ErrNameRequired},
		{strings.Repeat("a", domain.MaxFullNameLength+1), domain.ErrNameTooLong},
	} {
		if _, err := svc.Commands.Rename.Handle(ctx, command.Rename{UserID: id, FullName: tc.name}); !errors.Is(err, tc.want) {
			t.Errorf("name %q: got %v, want %v", tc.name, err, tc.want)
		}
	}

	// A name at the limit is fine, and counted in runes rather than bytes: 200
	// Vietnamese letters are 200 characters and rather more bytes.
	long := strings.Repeat("ữ", domain.MaxFullNameLength)
	if _, err := svc.Commands.Rename.Handle(ctx, command.Rename{UserID: id, FullName: long}); err != nil {
		t.Errorf("a name of exactly %d runes was refused: %v", domain.MaxFullNameLength, err)
	}
}

func TestRenamingLeavesATrail(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, _ := makeUser(t, pool)
	ctx := context.Background()

	if _, err := svc.Commands.Rename.Handle(ctx, command.Rename{UserID: id, FullName: "Tên Mới", IP: "203.0.113.9"}); err != nil {
		t.Fatal(err)
	}

	var action, diff string
	if err := pool.QueryRow(ctx, `
		SELECT action, diff::text FROM app.audit_log
		 WHERE entity = 'user' AND entity_id = $1::uuid AND action = 'user.renamed'
		 ORDER BY occurred_at DESC LIMIT 1`, id).Scan(&action, &diff); err != nil {
		t.Fatalf("no audit row for the rename: %v", err)
	}
	if !strings.Contains(diff, "Tên Mới") {
		t.Errorf("the trail does not say what the name became: %s", diff)
	}
}
