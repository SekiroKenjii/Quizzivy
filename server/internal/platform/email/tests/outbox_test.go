//go:build integration

package email_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/platform/email"
)

func day(n int) time.Time {
	return time.Date(2031, 3, n, 12, 0, 0, 0, time.FixedZone("ICT", 7*3600))
}

func addr(t *testing.T, label string) string { return label + "-" + nonce(t) + "@example.com" }

func TestARolledBackEnqueueSendsNothing(t *testing.T) {
	w := newWorld(t, day(1))
	to := addr(t, "rollback")
	tx, err := w.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	id, wrote, err := w.outbox.Enqueue(context.Background(), tx, invite(to))
	if err != nil || !wrote {
		t.Fatalf("enqueue: %v %v", wrote, err)
	}
	if err := tx.Rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := w.disp.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := w.sender.to(to); len(got) != 0 {
		t.Fatalf("a rolled-back enqueue sent %d message(s)", len(got))
	}
	var n int
	if err := w.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.email_outbox WHERE id = $1::uuid`, id).Scan(&n); err != nil || n != 0 {
		t.Fatalf("the row survived the rollback: %d %v", n, err)
	}
}

func TestACommittedMessageIsRenderedWithItsSecretSentOnceAndLosesTheSecret(t *testing.T) {
	w := newWorld(t, day(2))
	to := addr(t, "sent")
	id := w.enqueue(t, invite(to))

	var stored []byte
	if err := w.pool.QueryRow(context.Background(), `SELECT secret_ciphertext FROM app.email_outbox WHERE id = $1::uuid`, id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if len(stored) == 0 || strings.Contains(string(stored), "tok-") {
		t.Fatalf("the secret is not sealed: %q", stored)
	}
	var params string
	if err := w.pool.QueryRow(context.Background(), `SELECT params::text FROM app.email_outbox WHERE id = $1::uuid`, id).Scan(&params); err != nil || strings.Contains(params, "tok-") {
		t.Fatalf("params carry the secret: %s %v", params, err)
	}

	batch, err := w.disp.Dispatch(context.Background())
	if err != nil || batch.Sent < 1 {
		t.Fatalf("dispatch: %+v %v", batch, err)
	}
	if _, err := w.disp.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	sent := w.sender.to(to)
	if len(sent) != 1 {
		t.Fatalf("sent %d times, want once", len(sent))
	}
	m := sent[0]
	if m.IdempotencyKey != id || !strings.Contains(m.Text, "https://app.quizzivy.com/set-password#token=tok-"+to) || !strings.Contains(m.Subject, "Trung tâm Ánh Dương") {
		t.Errorf("message: %+v", m)
	}
	got := read(t, w.pool, id)
	if got.sentAt == nil || got.failedAt != nil || got.hasSecret || got.attempts != 1 || got.provider != "fake:"+id || got.locale != "vi" {
		t.Errorf("row after sending: %+v", got)
	}
}

func TestTwoDispatchersNeverSendOneRowTwice(t *testing.T) {
	w := newWorld(t, day(3))
	w.sender.delay = 20 * time.Millisecond
	var recipients []string
	for range 8 {
		to := addr(t, "race")
		recipients = append(recipients, to)
		w.enqueue(t, invite(to))
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 3 {
				if _, err := w.disp.Dispatch(context.Background()); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	for _, to := range recipients {
		if n := len(w.sender.to(to)); n != 1 {
			t.Errorf("%s was sent %d times", to, n)
		}
	}
}

func TestOnceTheDaysBudgetIsSpentTheRestMoveToTheNextLocalMidnight(t *testing.T) {
	w := newWorld(t, day(4))
	w.disp.DailyLimit = 2
	var ids []string
	for range 4 {
		ids = append(ids, w.enqueue(t, email.Mail{To: addr(t, "budget"), Params: email.SignInInviteParams{FullName: "An"}}))
	}
	if _, err := w.disp.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	first, err := w.disp.Dispatch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Deferred < 2 {
		t.Fatalf("the second dispatch deferred %d row(s), want at least 2: %+v", first.Deferred, first)
	}
	var sent, deferred int
	midnight := time.Date(2031, 3, 5, 0, 0, 0, 0, time.FixedZone("ICT", 7*3600))
	for _, id := range ids {
		r := read(t, w.pool, id)
		switch {
		case r.sentAt != nil:
			sent++
		case r.deliverAfter.Equal(midnight):
			deferred++
		default:
			t.Errorf("row %s is neither sent nor moved to %v: %+v", id, midnight, r)
		}
	}
	if sent != 2 || deferred != 2 {
		t.Fatalf("sent %d and deferred %d, want 2 and 2", sent, deferred)
	}
	w.disp.Now = func() time.Time { return midnight.Add(time.Minute) }
	if _, err := w.disp.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if read(t, w.pool, id).sentAt == nil {
			t.Errorf("row %s was not sent on the next day", id)
		}
	}
}

func TestATransientFailureBacksOffAndTheTenthAttemptFailsTheRowWithoutItsSecret(t *testing.T) {
	w := newWorld(t, day(6))
	to := addr(t, "retry")
	w.sender.failWith[to] = &email.SendError{Kind: email.ErrTransient, Code: "http_503"}
	id := w.enqueue(t, invite(to))

	now := day(6)
	w.disp.Now = func() time.Time { return now }
	for attempt, wait := range []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, time.Hour, time.Hour, time.Hour, time.Hour, time.Hour} {
		if _, err := w.disp.Dispatch(context.Background()); err != nil {
			t.Fatal(err)
		}
		r := read(t, w.pool, id)
		if r.attempts != attempt+1 || r.failedAt != nil || r.sentAt != nil || !r.hasSecret || r.lastError != "http_503" {
			t.Fatalf("after attempt %d: %+v", attempt+1, r)
		}
		if !r.deliverAfter.Equal(now.Add(wait)) {
			t.Fatalf("after attempt %d the next try is at %v, want %v", attempt+1, r.deliverAfter, now.Add(wait))
		}
		if _, err := w.disp.Dispatch(context.Background()); err != nil {
			t.Fatal(err)
		}
		if read(t, w.pool, id).attempts != attempt+1 {
			t.Fatalf("attempt %d was retried before its time", attempt+1)
		}
		now = now.Add(wait)
	}
	if _, err := w.disp.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := read(t, w.pool, id)
	if r.attempts != 10 || r.failedAt == nil || r.hasSecret || r.sentAt != nil {
		t.Fatalf("after the tenth attempt: %+v", r)
	}
}

func TestAPermanentFailureFailsTheRowAtOnceAndAMessageThatCannotBeOpenedFailsClosed(t *testing.T) {
	w := newWorld(t, day(7))
	refused := addr(t, "refused")
	w.sender.failWith[refused] = &email.SendError{Kind: email.ErrPermanent, Code: "http_422"}
	refusedID := w.enqueue(t, invite(refused))
	tampered := addr(t, "tampered")
	tamperedID := w.enqueue(t, invite(tampered))
	if _, err := w.pool.Exec(context.Background(), `
		UPDATE app.email_outbox SET secret_ciphertext = (SELECT secret_ciphertext FROM app.email_outbox WHERE id = $1::uuid) WHERE id = $2::uuid`, refusedID, tamperedID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.disp.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := read(t, w.pool, refusedID); r.failedAt == nil || r.hasSecret || r.attempts != 1 || r.lastError != "http_422" {
		t.Errorf("refused: %+v", r)
	}
	if r := read(t, w.pool, tamperedID); r.failedAt == nil || r.hasSecret || r.lastError != "secret_unreadable" {
		t.Errorf("a secret sealed for another row: %+v", r)
	}
	if len(w.sender.to(tampered)) != 0 {
		t.Error("a message whose secret could not be opened was sent")
	}
}

func TestADedupeKeyWritesTheMessageOnce(t *testing.T) {
	w := newWorld(t, day(8))
	to := addr(t, "dedupe")
	key := "invite:" + nonce(t)
	first := w.enqueue(t, email.Mail{To: to, Params: email.SignInInviteParams{}, DedupeKey: key})
	tx, err := w.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	id, wrote, err := w.outbox.Enqueue(context.Background(), tx, email.Mail{To: to, Params: email.SignInInviteParams{}, DedupeKey: key})
	if err != nil || wrote || id != first {
		t.Fatalf("second enqueue: %q %v %v, want the first row %q and no write", id, wrote, err, first)
	}
}

func TestTheRecipientsLocaleIsUsedUnlessTheCallerNamesOne(t *testing.T) {
	w := newWorld(t, day(9))
	english := newUser(t, w.pool, "en")
	none := newUser(t, w.pool, "vi")
	if _, err := w.pool.Exec(context.Background(), `UPDATE app.users SET locale = NULL WHERE id = $1::uuid`, none); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		mail email.Mail
		want string
	}{
		"the user's own":      {email.Mail{ToUserID: english, To: addr(t, "en"), Params: email.SignInInviteParams{}}, "en"},
		"a user with none":    {email.Mail{ToUserID: none, To: addr(t, "none"), Params: email.SignInInviteParams{}}, "vi"},
		"no user":             {email.Mail{To: addr(t, "nouser"), Params: email.SignInInviteParams{}}, "vi"},
		"named by the caller": {email.Mail{ToUserID: english, To: addr(t, "named"), Locale: "vi", Params: email.SignInInviteParams{}}, "vi"},
	} {
		if got := read(t, w.pool, w.enqueue(t, c.mail)).locale; got != c.want {
			t.Errorf("%s: locale %q, want %q", name, got, c.want)
		}
	}
}

func TestEnqueueRefusesWhatCouldNeverBeSentWell(t *testing.T) {
	w := newWorld(t, day(10))
	bare := email.NewOutbox(nil)
	for name, c := range map[string]struct {
		outbox *email.Outbox
		mail   email.Mail
		want   error
	}{
		"no params":           {w.outbox, email.Mail{To: "a@example.com"}, email.ErrUnknownTemplate},
		"two addresses":       {w.outbox, email.Mail{To: "a@example.com, b@example.com", Params: email.SignInInviteParams{}}, email.ErrInvalidAddress},
		"a display name":      {w.outbox, email.Mail{To: "An <a@example.com>", Params: email.SignInInviteParams{}}, email.ErrInvalidAddress},
		"no address":          {w.outbox, email.Mail{Params: email.SignInInviteParams{}}, email.ErrInvalidAddress},
		"an unknown locale":   {w.outbox, email.Mail{To: "a@example.com", Locale: "fr", Params: email.SignInInviteParams{}}, email.ErrInvalidLocale},
		"a missing secret":    {w.outbox, email.Mail{To: "a@example.com", Params: email.AccountInviteParams{}}, email.ErrSecretRequired},
		"a secret not needed": {w.outbox, email.Mail{To: "a@example.com", Params: email.SignInInviteParams{}, Secret: "x"}, email.ErrSecretUnused},
		"no secrets key":      {bare, invite("a@example.com"), email.ErrNoSecretsKey},
	} {
		_, _, err := c.outbox.Enqueue(context.Background(), w.pool, c.mail)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", name, err, c.want)
		}
	}
}

func TestTheTableRefusesARowWithTwoOutcomesAndAParamsListOrAnUnknownLocale(t *testing.T) {
	w := newWorld(t, day(11))
	for name, c := range map[string]struct{ sql, constraint string }{
		"sent and failed":      {`sent_at, failed_at) VALUES ('a@example.com', 'x_y', 'vi', now(), now()`, "email_outbox_one_outcome"},
		"a params list":        {`params) VALUES ('a@example.com', 'x_y', 'vi', '[]'::jsonb`, "email_outbox_params_shape"},
		"a locale":             {`params) VALUES ('a@example.com', 'x_y', 'fr', '{}'::jsonb`, "email_outbox_locale_check"},
		"a template with caps": {`params) VALUES ('a@example.com', 'X', 'vi', '{}'::jsonb`, "email_outbox_template_check"},
		"an address too short": {`params) VALUES ('a@', 'x_y', 'vi', '{}'::jsonb`, "email_outbox_to_address_length"},
	} {
		_, err := w.pool.Exec(context.Background(), `INSERT INTO app.email_outbox (to_address, template, locale, `+c.sql+`)`)
		if err == nil || !strings.Contains(err.Error(), c.constraint) {
			t.Errorf("%s: %v, want %s", name, err, c.constraint)
		}
	}
}

func TestNextDueIsNoLaterThanTheEarliestPendingRow(t *testing.T) {
	w := newWorld(t, day(12))
	w.enqueue(t, email.Mail{To: addr(t, "late"), Params: email.SignInInviteParams{}, DeliverAfter: day(30)})
	w.enqueue(t, email.Mail{To: addr(t, "soon"), Params: email.SignInInviteParams{}, DeliverAfter: day(29)})
	due, pending, err := w.disp.NextDue(context.Background())
	if err != nil || !pending || due.After(day(29)) {
		t.Fatalf("next due: %v %v %v", due, pending, err)
	}
}
