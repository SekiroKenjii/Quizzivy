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

func TestNextDueIsNoLaterThanAPendingRowsLinkExpires(t *testing.T) {
	w := newWorld(t, day(12))
	w.enqueue(t, email.Mail{To: addr(t, "held"), Params: email.SignInInviteParams{}, DeliverAfter: day(30), ExpiresAt: day(25)})
	due, pending, err := w.disp.NextDue(context.Background())
	if err != nil || !pending || due.After(day(25)) {
		t.Fatalf("next due = %v %v %v, want no later than the link's expiry", due, pending, err)
	}
}

func TestARowOfATemplateThisBinaryDoesNotSendStaysPendingWithItsSecret(t *testing.T) {
	w := newWorld(t, day(13))
	longAgo := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var futureID string
	if err := w.pool.QueryRow(context.Background(), `
		INSERT INTO app.email_outbox (to_address, template, locale, params, secret_ciphertext, deliver_after)
		VALUES ($1, 'future_template', 'vi', '{"fullName":"An"}', '\x00', $2)
		RETURNING id::text`, addr(t, "future"), longAgo).Scan(&futureID); err != nil {
		t.Fatal(err)
	}
	w.ids = append(w.ids, futureID)
	newer := addr(t, "newer")
	newerID := w.enqueue(t, email.Mail{To: newer, Params: email.SignInInviteParams{}})
	if _, err := w.pool.Exec(context.Background(), `
		UPDATE app.email_outbox SET params = '{"fullName":"An","addedLater":true}' WHERE id = $1::uuid`, newerID); err != nil {
		t.Fatal(err)
	}
	batch, err := w.disp.Dispatch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r := read(t, w.pool, newerID); r.sentAt == nil || r.lastError != "" {
		t.Errorf("a row with a field added later: %+v", r)
	}
	if r := read(t, w.pool, futureID); r.sentAt != nil || r.failedAt != nil || r.attempts != 0 || !r.hasSecret || r.lastError != "" || !r.deliverAfter.Equal(longAgo) {
		t.Errorf("a row of a template this binary does not send: %+v", r)
	}
	if batch.Failed != 0 {
		t.Errorf("batch: %+v", batch)
	}
	if due, pending, err := w.disp.NextDue(context.Background()); err != nil || (pending && !due.After(longAgo)) {
		t.Errorf("the unknown row is counted as due: %v %v %v", due, pending, err)
	}
}

func TestOnceTheMonthsBudgetIsSpentTheRestWaitForTheFirstAndAnExpiredLinkIsNeverSent(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	now := time.Date(2033, 7, 31, 22, 0, 0, 0, ict)
	w := newWorld(t, now)
	w.disp.Now = func() time.Time { return now }
	w.disp.DailyLimit, w.disp.MonthlyLimit = 50, 2
	mail := func(label string, minutesAgo int, expires time.Time) string {
		m := invite(addr(t, label))
		m.DeliverAfter, m.ExpiresAt = now.Add(-time.Duration(minutesAgo)*time.Minute), expires
		return w.enqueue(t, m)
	}
	first, second := mail("thang-1", 3, time.Time{}), mail("thang-2", 2, time.Time{})
	expiring := mail("het-han", 1, time.Date(2033, 7, 31, 23, 30, 0, 0, ict))
	later := mail("de-sau", 1, time.Date(2033, 8, 8, 0, 0, 0, 0, ict))

	for range 2 {
		if _, err := w.disp.Dispatch(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	firstOfAugust := time.Date(2033, 8, 1, 0, 0, 0, 0, ict)
	for _, id := range []string{first, second} {
		if r := read(t, w.pool, id); r.sentAt == nil {
			t.Errorf("row %s was not sent within the month's budget: %+v", id, r)
		}
	}
	for _, id := range []string{expiring, later} {
		if r := read(t, w.pool, id); r.sentAt != nil || !r.deliverAfter.Equal(firstOfAugust) {
			t.Errorf("row %s = %+v, want it moved to %v", id, r, firstOfAugust)
		}
	}

	now = firstOfAugust.Add(time.Minute)
	batch, err := w.disp.Dispatch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if batch.Expired != 1 {
		t.Errorf("expired = %d, want 1", batch.Expired)
	}
	if r := read(t, w.pool, expiring); r.sentAt != nil || r.failedAt == nil || r.lastError != "expired" || r.hasSecret {
		t.Errorf("the link that expired while it waited = %+v, want failed as expired with its secret erased", r)
	}
	if by := heldBy(t, w, expiring); by != "month" {
		t.Errorf("the expired message is held by %q, want month kept as the record of why it was not sent", by)
	}
	if r := read(t, w.pool, later); r.sentAt == nil {
		t.Errorf("the link still valid on the 1st = %+v, want it sent", r)
	}
}

func TestMailHeldByTheMonthsCapGoesFirstOnceTheCapIsRaised(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	now := time.Date(2033, 7, 20, 10, 0, 0, 0, ict)
	w := newWorld(t, now)
	w.disp.Now = func() time.Time { return now }
	w.disp.DailyLimit, w.disp.MonthlyLimit = 50, 1
	mail := func(label string) string {
		m := invite(addr(t, label))
		m.DeliverAfter = now.Add(-time.Minute)
		return w.enqueue(t, m)
	}
	sent, held := mail("gui-truoc"), mail("bi-giu")
	for range 2 {
		if _, err := w.disp.Dispatch(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	firstOfAugust := time.Date(2033, 8, 1, 0, 0, 0, 0, ict)
	if r := read(t, w.pool, sent); r.sentAt == nil {
		t.Fatalf("the month's one message was not sent: %+v", r)
	}
	if r := read(t, w.pool, held); r.sentAt != nil || !r.deliverAfter.Equal(firstOfAugust) {
		t.Fatalf("the message past the cap = %+v, want it held for %v", r, firstOfAugust)
	}

	newer := mail("moi-hon")
	w.disp.MonthlyLimit = 2
	batch, err := w.disp.Dispatch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if batch.Released != 1 || batch.Sent != 1 {
		t.Errorf("batch = %+v, want the held message released and one sent", batch)
	}
	if r := read(t, w.pool, held); r.sentAt == nil {
		t.Errorf("the held message = %+v, want it sent first once the cap allows one more", r)
	}
	if r := read(t, w.pool, newer); r.sentAt != nil {
		t.Errorf("the newer message = %+v, want it to wait behind the held one", r)
	}
}

func TestMailHeldByTheDaysCapGoesFirstOnceTheCapIsRaised(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	now := time.Date(2033, 7, 20, 10, 0, 0, 0, ict)
	w := newWorld(t, now)
	w.disp.Now = func() time.Time { return now }
	w.disp.DailyLimit, w.disp.MonthlyLimit = 1, 3000
	mail := func(label string) string {
		m := invite(addr(t, label))
		m.DeliverAfter = now.Add(-time.Minute)
		return w.enqueue(t, m)
	}
	mail("gui-truoc")
	held := mail("bi-giu")
	for range 2 {
		if _, err := w.disp.Dispatch(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if by := heldBy(t, w, held); by != "day" {
		t.Fatalf("the message past the day's cap is held by %q, want day", by)
	}

	newer := mail("moi-hon")
	w.disp.DailyLimit = 2
	if _, err := w.disp.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := read(t, w.pool, held); r.sentAt == nil {
		t.Errorf("the held message = %+v, want it sent first once the day's cap allows one more", r)
	}
	if by := heldBy(t, w, held); by != "" {
		t.Errorf("a sent message is still held by %q", by)
	}
	if r := read(t, w.pool, newer); r.sentAt != nil {
		t.Errorf("the newer message = %+v, want it to wait behind the held one", r)
	}
}

func TestAMessageEnqueuedForLaterIsNotHeldAndIsNotReleasedEarly(t *testing.T) {
	ict := time.FixedZone("ICT", 7*3600)
	now := time.Date(2033, 7, 20, 10, 0, 0, 0, ict)
	w := newWorld(t, now)
	w.disp.Now = func() time.Time { return now }
	w.disp.DailyLimit, w.disp.MonthlyLimit = 50, 3000
	midnight := time.Date(2033, 7, 21, 0, 0, 0, 0, ict)
	m := invite(addr(t, "hen-gio"))
	m.DeliverAfter = midnight
	later := w.enqueue(t, m)

	batch, err := w.disp.Dispatch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if batch.Released != 0 {
		t.Errorf("released %d, want none: nothing was held", batch.Released)
	}
	if r := read(t, w.pool, later); r.sentAt != nil || !r.deliverAfter.Equal(midnight) {
		t.Errorf("a message enqueued for midnight = %+v, want it left for %v", r, midnight)
	}
	if by := heldBy(t, w, later); by != "" {
		t.Errorf("a message enqueued for later is held by %q, want none", by)
	}
}

func heldBy(t *testing.T, w *world, id string) string {
	t.Helper()
	var by *string
	if err := w.pool.QueryRow(context.Background(), `SELECT held_by FROM app.email_outbox WHERE id = $1::uuid`, id).Scan(&by); err != nil {
		t.Fatal(err)
	}
	if by == nil {
		return ""
	}
	return *by
}
