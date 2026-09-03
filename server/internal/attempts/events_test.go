package attempts_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/attempts"
)

func flushOf(w world, s attempts.Session, seq int, kind string) attempts.FlushInput {
	return attempts.FlushInput{
		AttemptID: s.Attempt.ID,
		SessionID: s.SessionID,
		StudentID: w.student,
		Events: []attempts.Event{
			{Kind: kind, OccurredAt: time.Now(), ClientSeq: seq},
		},
	}
}

func TestABearerFlushAppendsEvents(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)

	if err := svc.Flush(context.Background(), flushOf(w, session, 1, "window_blur")); err != nil {
		t.Fatalf("flush: %v", err)
	}
	kinds := eventKinds(t, pool, session.Attempt.ID)
	if !containsKind(kinds, "window_blur") {
		t.Errorf("events %v do not include the flushed one", kinds)
	}
}

// [D-01] The regression test for the uniqueness key. clientSeq lives in
// sessionStorage and restarts at 0 on a new session, so keying on
// (attempt, client_seq) alone would make every event of a resumed session a
// duplicate-key failure -- silently losing exactly the resume timeline the
// teacher needs.
func TestTheSameClientSeqFromTwoSessionsBothPersist(t *testing.T) {
	pool := newPool(t)
	svc, w, first := started(t, pool)
	ctx := context.Background()

	if err := svc.Flush(ctx, flushOf(w, first, 0, "tab_hidden")); err != nil {
		t.Fatalf("first session: %v", err)
	}
	second, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if err := svc.Flush(ctx, flushOf(w, second, 0, "tab_visible")); err != nil {
		t.Fatalf("second session, same clientSeq: %v", err)
	}

	got := count(t, pool, `
		SELECT count(*) FROM app.attempt_events
		 WHERE attempt_id = $1::uuid AND client_seq = 0`, first.Attempt.ID)
	if got != 2 {
		t.Errorf("%d events at clientSeq 0, want 2 — one session's events were lost", got)
	}
}

func TestADuplicateBatchInsertsNothingAndSucceeds(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()
	same := flushOf(w, session, 7, "paste")

	if err := svc.Flush(ctx, same); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := svc.Flush(ctx, same); err != nil {
		t.Fatalf("replay must succeed, got %v", err)
	}
	if n := count(t, pool, `
		SELECT count(*) FROM app.attempt_events
		 WHERE attempt_id = $1::uuid AND kind = 'paste'`, session.Attempt.ID); n != 1 {
		t.Errorf("%d paste events after a replay, want 1", n)
	}
}

// §10.1 is explicit that the kind list will grow, and a newer client must not
// fail against an older server.
func TestAnUnknownKindIsStoredRatherThanRejected(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)

	in := flushOf(w, session, 11, "quantum_tunnelling_detected")
	if err := svc.Flush(context.Background(), in); err != nil {
		t.Fatalf("an unknown kind was rejected: %v", err)
	}
	if !containsKind(eventKinds(t, pool, session.Attempt.ID), "quantum_tunnelling_detected") {
		t.Error("the unknown kind was not stored")
	}
}

// §13.3 wants both clocks. occurred_at is the client's, corrected against the
// serverTime it is handed on every save; received_at is ours and is the one
// that cannot be wrong.
func TestBothClocksAreStored(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	claimed := time.Now().Add(-90 * time.Second).UTC().Truncate(time.Millisecond)

	in := flushOf(w, session, 21, "network_offline")
	in.Events[0].OccurredAt = claimed
	if err := svc.Flush(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	var occurred, received time.Time
	if err := pool.QueryRow(context.Background(), `
		SELECT occurred_at, received_at FROM app.attempt_events
		 WHERE attempt_id = $1::uuid AND kind = 'network_offline'`,
		session.Attempt.ID).Scan(&occurred, &received); err != nil {
		t.Fatal(err)
	}
	if !occurred.UTC().Truncate(time.Millisecond).Equal(claimed) {
		t.Errorf("occurred_at %v, want the client's %v", occurred, claimed)
	}
	if !received.After(occurred) {
		t.Errorf("received_at %v is not after occurred_at %v", received, occurred)
	}
}

func TestABeaconTokenAppendsEvents(t *testing.T) {
	pool := newPool(t)
	svc, _, session := started(t, pool)

	in := attempts.FlushInput{
		AttemptID:   session.Attempt.ID,
		SessionID:   session.SessionID,
		BeaconToken: session.BeaconToken,
		Events:      []attempts.Event{{Kind: "page_hide", OccurredAt: time.Now(), ClientSeq: 99}},
	}
	if err := svc.Flush(context.Background(), in); err != nil {
		t.Fatalf("beacon flush: %v", err)
	}
	if !containsKind(eventKinds(t, pool, session.Attempt.ID), "page_hide") {
		t.Error("the beacon's event was not stored")
	}
}

func TestAnExpiredBeaconTokenIsRejected(t *testing.T) {
	pool := newPool(t)
	svc, _, session := started(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `
		UPDATE app.attempts
		   SET started_at = now() - interval '2 hours', deadline_at = now() - interval '1 hour'
		 WHERE id = $1::uuid`, session.Attempt.ID); err != nil {
		t.Fatal(err)
	}

	in := attempts.FlushInput{
		AttemptID:   session.Attempt.ID,
		SessionID:   session.SessionID,
		BeaconToken: session.BeaconToken,
		Events:      []attempts.Event{{Kind: "page_hide", OccurredAt: time.Now(), ClientSeq: 99}},
	}
	if err := svc.Flush(ctx, in); !errors.Is(err, attempts.ErrBeaconExpired) {
		t.Fatalf("got %v, want ErrBeaconExpired", err)
	}
	if containsKind(eventKinds(t, pool, session.Attempt.ID), "page_hide") {
		t.Error("an expired beacon still wrote")
	}
}

// The bearer path deliberately does NOT check the deadline: a page_hide landing
// a moment after time runs out is exactly the event worth keeping.
func TestABearerFlushStillWorksAfterTheDeadline(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `
		UPDATE app.attempts
		   SET started_at = now() - interval '2 hours', deadline_at = now() - interval '1 hour'
		 WHERE id = $1::uuid`, session.Attempt.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Flush(ctx, flushOf(w, session, 31, "page_hide")); err != nil {
		t.Fatalf("a late bearer flush was refused: %v", err)
	}
}

func TestAWrongOrMissingCredentialIsRefused(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool)
	ctx := context.Background()

	events := []attempts.Event{{Kind: "paste", OccurredAt: time.Now(), ClientSeq: 5}}
	cases := []struct {
		name string
		in   attempts.FlushInput
	}{
		{"no credential at all", attempts.FlushInput{
			AttemptID: session.Attempt.ID, SessionID: session.SessionID, Events: events}},
		{"another student's bearer", attempts.FlushInput{
			AttemptID: session.Attempt.ID, SessionID: session.SessionID,
			StudentID: w.outsider, Events: events}},
		{"a beacon token that is not this attempt's", attempts.FlushInput{
			AttemptID: session.Attempt.ID, SessionID: session.SessionID,
			BeaconToken: "not-the-token", Events: events}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := svc.Flush(ctx, c.in); !errors.Is(err, attempts.ErrForbidden) {
				t.Fatalf("got %v, want ErrForbidden", err)
			}
		})
	}
	if containsKind(eventKinds(t, pool, session.Attempt.ID), "paste") {
		t.Error("a refused flush still wrote")
	}
}

// The beacon token buys append-only access and nothing else.
//
// The strong form of this is structural -- Flush returns only an error, and no
// read method accepts a token -- which a runtime test cannot observe. What it
// can observe is that holding the token gets you no payload, whatever the
// error happens to be, so the assertion is on the absence of data rather than
// on a particular code.
func TestABeaconTokenBuysNoRead(t *testing.T) {
	pool := newPool(t)
	svc, _, session := started(t, pool)
	ctx := context.Background()

	// It appends, so it is a real credential and not merely a wrong string.
	appended := attempts.FlushInput{
		AttemptID:   session.Attempt.ID,
		SessionID:   session.SessionID,
		BeaconToken: session.BeaconToken,
		Events:      []attempts.Event{{Kind: "page_hide", OccurredAt: time.Now(), ClientSeq: 41}},
	}
	if err := svc.Flush(ctx, appended); err != nil {
		t.Fatalf("the token should append: %v", err)
	}

	got, err := svc.Get(ctx, session.Attempt.ID, session.BeaconToken)
	if err == nil {
		t.Fatal("the beacon token read the attempt")
	}
	if len(got.Questions) != 0 || got.BeaconToken != "" {
		t.Errorf("a failed read still handed back data: %+v", got)
	}
}

func containsKind(kinds []string, want string) bool {
	for _, k := range kinds {
		if k == want {
			return true
		}
	}
	return false
}

// One away episode as the client records it: the return signal carrying how
// long the student was gone. The leave signal is recorded too but carries
// nothing, so it is the return that counts.
func away(seq int, awayMs int) attempts.Event {
	return attempts.Event{
		Kind: "window_focus", OccurredAt: time.Now(), ClientSeq: seq,
		Meta: []byte(fmt.Sprintf(`{"awayMs": %d}`, awayMs)),
	}
}

func flushEvents(w world, s attempts.Session, events ...attempts.Event) attempts.FlushInput {
	return attempts.FlushInput{
		AttemptID: s.Attempt.ID, SessionID: s.SessionID, StudentID: w.student, Events: events,
	}
}

func focusState(t *testing.T, pool *pgxpool.Pool, attemptID string) (count int, flagged bool) {
	t.Helper()
	if err := pool.QueryRow(context.Background(),
		`SELECT focus_loss_count, flagged FROM app.attempts WHERE id = $1::uuid`, attemptID).
		Scan(&count, &flagged); err != nil {
		t.Fatal(err)
	}
	return count, flagged
}

func startWith(t *testing.T, pool *pgxpool.Pool, o worldOpts) (*attempts.Service, world, attempts.Session) {
	t.Helper()
	w := seedWorld(t, pool, o)
	svc := newService(t, pool)
	session, err := svc.StartOrResume(context.Background(), w.assignment, w.student)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	return svc, w, session
}

// §10.2's `flag`: "attempt marked for the admin, student told". The client
// tells; this is the marking. And §10.1's threshold: the count is episodes at
// or over minAwayMs, not events.
func TestAwayEpisodesOverTheThresholdAreCountedAndExceedingTheLimitFlags(t *testing.T) {
	pool := newPool(t)
	svc, w, session := startWith(t, pool, focusLimit(2, "flag"))
	ctx := context.Background()

	// Two episodes: at the limit, not over it. "Quá 2 lần" means more than two.
	if err := svc.Flush(ctx, flushEvents(w, session, away(1, 3502), away(2, 90_000))); err != nil {
		t.Fatal(err)
	}
	if n, flagged := focusState(t, pool, session.Attempt.ID); n != 2 || flagged {
		t.Fatalf("after two episodes: count %d flagged %v, want 2 and not flagged", n, flagged)
	}

	// A notification, not a search: below the threshold, so not an episode.
	if err := svc.Flush(ctx, flushEvents(w, session, away(3, 2000))); err != nil {
		t.Fatal(err)
	}
	if n, flagged := focusState(t, pool, session.Attempt.ID); n != 2 || flagged {
		t.Fatalf("after a 2s blur: count %d flagged %v, want still 2 and not flagged", n, flagged)
	}

	// The third is the one over the limit.
	if err := svc.Flush(ctx, flushEvents(w, session, away(4, 3000))); err != nil {
		t.Fatal(err)
	}
	if n, flagged := focusState(t, pool, session.Attempt.ID); n != 3 || !flagged {
		t.Fatalf("after three episodes: count %d flagged %v, want 3 and flagged", n, flagged)
	}

	// What the student sees on the next resume is the same number.
	resumed, err := svc.StartOrResume(ctx, w.assignment, w.student)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Attempt.FocusLossCount != 3 || !resumed.Attempt.Flagged {
		t.Errorf("resume payload: count %d flagged %v, want 3 and flagged",
			resumed.Attempt.FocusLossCount, resumed.Attempt.Flagged)
	}
}

func TestWarnCountsButNeverFlags(t *testing.T) {
	pool := newPool(t)
	svc, w, session := startWith(t, pool, focusLimit(1, "warn"))

	if err := svc.Flush(context.Background(),
		flushEvents(w, session, away(1, 5000), away(2, 5000), away(3, 5000))); err != nil {
		t.Fatal(err)
	}
	if n, flagged := focusState(t, pool, session.Attempt.ID); n != 3 || flagged {
		t.Errorf("count %d flagged %v, want 3 and not flagged: warn is dialog only", n, flagged)
	}
}

func TestNoLimitCountsButNeverFlags(t *testing.T) {
	pool := newPool(t)
	svc, w, session := started(t, pool) // §10.3's default: maxFocusLoss 0

	if err := svc.Flush(context.Background(),
		flushEvents(w, session, away(1, 5000), away(2, 5000))); err != nil {
		t.Fatal(err)
	}
	if n, flagged := focusState(t, pool, session.Attempt.ID); n != 2 || flagged {
		t.Errorf("count %d flagged %v, want 2 and not flagged: 0 means unlimited", n, flagged)
	}
}

// The threshold is the assignment's, not a constant.
func TestTheAssignmentsOwnThresholdDecidesAnEpisode(t *testing.T) {
	pool := newPool(t)
	o := focusLimit(0, "flag")
	o.minAwayMs = 10_000
	svc, w, session := startWith(t, pool, o)

	if err := svc.Flush(context.Background(),
		flushEvents(w, session, away(1, 5000), away(2, 10_000))); err != nil {
		t.Fatal(err)
	}
	if n, _ := focusState(t, pool, session.Attempt.ID); n != 1 {
		t.Errorf("count %d, want 1: only the 10s episode reaches a 10s threshold", n)
	}
}

// A retried batch is absorbed by ON CONFLICT; the recount must not see it twice.
func TestARetriedBatchDoesNotDoubleCount(t *testing.T) {
	pool := newPool(t)
	svc, w, session := startWith(t, pool, focusLimit(5, "flag"))
	ctx := context.Background()

	in := flushEvents(w, session, away(1, 4000), away(2, 4000))
	for range 3 {
		if err := svc.Flush(ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := focusState(t, pool, session.Attempt.ID); n != 2 {
		t.Errorf("count %d after the same batch three times, want 2", n)
	}
}

func TestTheBeaconPathCountsToo(t *testing.T) {
	pool := newPool(t)
	svc, _, session := startWith(t, pool, focusLimit(1, "flag"))

	in := attempts.FlushInput{
		AttemptID: session.Attempt.ID, SessionID: session.SessionID,
		BeaconToken: session.BeaconToken,
		Events:      []attempts.Event{away(1, 4000), away(2, 4000)},
	}
	if err := svc.Flush(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if n, flagged := focusState(t, pool, session.Attempt.ID); n != 2 || !flagged {
		t.Errorf("count %d flagged %v via the beacon, want 2 and flagged", n, flagged)
	}
}

// Events ride with the answers in one transaction (§10.6), so a bad byte in
// telemetry must not roll back the answer it travelled with.
func TestAMalformedAwayMsIsSkippedNotFatal(t *testing.T) {
	pool := newPool(t)
	svc, w, session := startWith(t, pool, focusLimit(1, "flag"))

	junk := attempts.Event{
		Kind: "window_focus", OccurredAt: time.Now(), ClientSeq: 1,
		Meta: []byte(`{"awayMs": "soon"}`),
	}
	if err := svc.Flush(context.Background(), flushEvents(w, session, junk, away(2, 4000))); err != nil {
		t.Fatalf("a malformed awayMs failed the flush: %v", err)
	}
	if n, _ := focusState(t, pool, session.Attempt.ID); n != 1 {
		t.Errorf("count %d, want 1: the junk is skipped and the real episode counted", n)
	}
}
