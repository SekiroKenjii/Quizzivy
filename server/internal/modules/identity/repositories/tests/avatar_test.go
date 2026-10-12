//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/modules/identity/repositories"
	"quizzivy/internal/platform/db"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func avatarWrite(w profileWorld, key *string) domain.AvatarRecord {
	return domain.AvatarRecord{UserID: w.id, Key: key, Now: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)}
}

func TestSettingReplacingAndClearingAPhotoReturnsTheKeyItReplacedAndAuditsEachChange(t *testing.T) {
	w := profileFixture(t)
	ctx := context.Background()
	first, second := "avatars/"+w.id+"/one.png", "avatars/"+w.id+"/two.png"

	set, err := w.users.SetAvatar(ctx, avatarWrite(w, &first))
	if err != nil {
		t.Fatal(err)
	}
	if set.PreviousKey != nil || set.User.AvatarKey == nil || *set.User.AvatarKey != first {
		t.Fatalf("first set answered %+v", set)
	}
	replaced, err := w.users.SetAvatar(ctx, avatarWrite(w, &second))
	if err != nil {
		t.Fatal(err)
	}
	if replaced.PreviousKey == nil || *replaced.PreviousKey != first || *replaced.User.AvatarKey != second {
		t.Fatalf("replacing answered %+v", replaced)
	}
	cleared, err := w.users.SetAvatar(ctx, avatarWrite(w, nil))
	if err != nil {
		t.Fatal(err)
	}
	if cleared.PreviousKey == nil || *cleared.PreviousKey != second || cleared.User.AvatarKey != nil {
		t.Fatalf("clearing answered %+v", cleared)
	}
	stored, err := w.users.FindUserByID(ctx, w.id)
	if err != nil || stored.AvatarKey != nil {
		t.Fatalf("stored user %+v, %v", stored, err)
	}

	var sets, removals int
	if err := w.tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE action = 'user.avatar_set'), count(*) FILTER (WHERE action = 'user.avatar_removed')
	  FROM app.audit_log WHERE entity = 'user' AND entity_id = $1::uuid`, w.id).Scan(&sets, &removals); err != nil {
		t.Fatal(err)
	}
	if sets != 2 || removals != 1 {
		t.Errorf("audit holds %d sets and %d removals, want 2 and 1", sets, removals)
	}
	for action, diff := range auditFields(t, w) {
		if strings.Contains(string(diff), "avatars/") {
			t.Errorf("%s audit diff %s names a storage key", action, diff)
		}
	}
}

func TestClearingAPhotoThatIsNotThereWritesNoAudit(t *testing.T) {
	w := profileFixture(t)
	ctx := context.Background()

	cleared, err := w.users.SetAvatar(ctx, avatarWrite(w, nil))
	if err != nil {
		t.Fatal(err)
	}
	if cleared.PreviousKey != nil || cleared.User.AvatarKey != nil || cleared.User.ID != w.id {
		t.Fatalf("clearing nothing answered %+v", cleared)
	}
	if n := auditCount(t, w); n != 0 {
		t.Errorf("clearing nothing wrote %d audit rows", n)
	}
}

func TestADisabledAccountKeepsItsPhotoAndAnUnknownOneIsNotFound(t *testing.T) {
	w := profileFixture(t)
	ctx := context.Background()
	kept, replacement := "avatars/"+w.id+"/kept.png", "avatars/"+w.id+"/new.png"
	if _, err := w.users.SetAvatar(ctx, avatarWrite(w, &kept)); err != nil {
		t.Fatal(err)
	}
	if _, err := w.tx.Exec(ctx, `UPDATE app.users SET disabled_at = now() WHERE id = $1::uuid`, w.id); err != nil {
		t.Fatal(err)
	}

	if _, err := w.users.SetAvatar(ctx, avatarWrite(w, &replacement)); !errors.Is(err, domain.ErrAccountDisabled) {
		t.Fatalf("setting on a disabled account answered %v, want ErrAccountDisabled", err)
	}
	if _, err := w.users.SetAvatar(ctx, avatarWrite(w, nil)); !errors.Is(err, domain.ErrAccountDisabled) {
		t.Fatalf("clearing on a disabled account answered %v, want ErrAccountDisabled", err)
	}
	var stored *string
	if err := w.tx.QueryRow(ctx, `SELECT avatar_key FROM app.users WHERE id = $1::uuid`, w.id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == nil || *stored != kept {
		t.Errorf("a disabled account's photo is %v, want %q", stored, kept)
	}

	missing := avatarWrite(w, &replacement)
	missing.UserID = "01935000-0000-7000-8000-00000000dead"
	if _, err := w.users.SetAvatar(ctx, missing); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("setting for an unknown user answered %v, want ErrUserNotFound", err)
	}
}

func TestConcurrentPhotoWritesChainEveryKeyToTheNextSoNoObjectIsLeftBehind(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	var id string
	t.Cleanup(func() {
		if id == "" {
			return
		}
		if _, err := pool.Exec(ctx, `DELETE FROM app.audit_log WHERE actor_user_id = $1::uuid`, id); err != nil {
			t.Errorf("cleanup audit: %v", err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM app.users WHERE id = $1::uuid`, id); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})
	if err := pool.QueryRow(ctx, `INSERT INTO app.users(email,full_name,role_id) VALUES ('avatar-race-'||uuidv7()::text||'@example.com','Race',(SELECT id FROM app.roles WHERE builtin_key='student')) RETURNING id::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}

	users := repositories.NewUsers(db.NewContext(pool))
	const writers = 8
	written := make([]string, writers)
	previous := make([]*string, writers)
	failures := make([]error, writers)
	var wg sync.WaitGroup
	for i := range writers {
		written[i] = fmt.Sprintf("avatars/%s/race-%d.png", id, i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := users.SetAvatar(ctx, domain.AvatarRecord{UserID: id, Key: &written[i], Now: time.Now()})
			previous[i], failures[i] = result.PreviousKey, err
		}()
	}
	wg.Wait()
	for i, err := range failures {
		if err != nil {
			t.Fatalf("writer %d: %v", i, err)
		}
	}

	var final string
	if err := pool.QueryRow(ctx, `SELECT avatar_key FROM app.users WHERE id = $1::uuid`, id).Scan(&final); err != nil {
		t.Fatal(err)
	}
	var replaced []string
	nils := 0
	for _, key := range previous {
		if key == nil {
			nils++
			continue
		}
		replaced = append(replaced, *key)
	}
	var want []string
	for _, key := range written {
		if key != final {
			want = append(want, key)
		}
	}
	sort.Strings(replaced)
	sort.Strings(want)
	if nils != 1 || strings.Join(replaced, ",") != strings.Join(want, ",") {
		t.Errorf("the writes replaced %v with %d empty, want exactly every key but the final %q: %v", replaced, nils, final, want)
	}
}
