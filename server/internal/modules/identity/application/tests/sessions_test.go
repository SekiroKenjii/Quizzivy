//go:build integration

package application_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/application/query"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
)

// The signed-in devices (T-R4.9). Each test signs in through the real Login
// command, so a family is what a browser would hold, and reads the database
// for what the commands left behind.

type forgetCall struct {
	userID   string
	epoch    int
	revoked  int
	auditRow int
}

// spyPrincipals records what the database held at the moment Forget ran, which
// is the proof that it ran after the commit and not before.
type spyPrincipals struct {
	pool  *pgxpool.Pool
	mu    sync.Mutex
	calls []forgetCall
}

func (s *spyPrincipals) Resolve(_ context.Context, userID string) (access.Principal, error) {
	return access.Principal{UserID: userID}, nil
}

func (s *spyPrincipals) Forget(userID string) {
	call := forgetCall{userID: userID}
	ctx := context.Background()
	_ = s.pool.QueryRow(ctx, `SELECT session_epoch FROM app.users WHERE id = $1`, userID).Scan(&call.epoch)
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM app.refresh_tokens WHERE user_id = $1 AND revoked_at IS NOT NULL AND replaced_by IS NULL`, userID).Scan(&call.revoked)
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE actor_user_id = $1 AND action LIKE 'session.%'`, userID).Scan(&call.auditRow)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, call)
}

func (s *spyPrincipals) forgets() []forgetCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]forgetCall(nil), s.calls...)
}

func serviceWithSpy(t *testing.T, pool *pgxpool.Pool) (*application.Application, *spyPrincipals) {
	t.Helper()
	svc := newService(t, pool)
	spy := &spyPrincipals{pool: pool}
	svc.SetPrincipals(spy)
	return svc, spy
}

type device struct {
	userAgent string
	ip        string
	geoLabel  string
}

type signedIn struct {
	refresh string
	access  string
	family  string
}

func signInFrom(t *testing.T, svc *application.Application, pool *pgxpool.Pool, email string, d device) signedIn {
	t.Helper()
	session, err := svc.Commands.Login.Handle(context.Background(), command.Login{
		Email: email, Password: testPassword, UserAgent: d.userAgent, IP: d.ip, GeoLabel: d.geoLabel,
	})
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	return signedIn{refresh: session.RefreshToken, access: session.AccessToken, family: loadToken(t, pool, session.RefreshToken).familyID}
}

func sessionsOf(t *testing.T, svc *application.Application, userID, cookie string) []domain.Session {
	t.Helper()
	sessions, err := svc.Queries.ListSessions.Handle(context.Background(), query.ListSessions{UserID: userID, RefreshToken: cookie})
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	return sessions
}

func familiesOf(sessions []domain.Session) []string {
	out := make([]string, len(sessions))
	for i, s := range sessions {
		out[i] = s.FamilyID
	}
	return out
}

func epochOf(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	var epoch int
	if err := pool.QueryRow(context.Background(), `SELECT session_epoch FROM app.users WHERE id = $1`, userID).Scan(&epoch); err != nil {
		t.Fatalf("read the session epoch: %v", err)
	}
	return epoch
}

func sessionAudits(t *testing.T, pool *pgxpool.Pool, userID string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT action FROM app.audit_log WHERE actor_user_id = $1 AND action LIKE 'session.%' ORDER BY id`, userID)
	if err != nil {
		t.Fatalf("read the audit log: %v", err)
	}
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return actions
}

func geoLabelOf(t *testing.T, pool *pgxpool.Pool, refresh string) *string {
	t.Helper()
	var label *string
	if err := pool.QueryRow(context.Background(),
		`SELECT geo_label FROM app.refresh_tokens WHERE token_hash = $1`, hashOf(refresh)).Scan(&label); err != nil {
		t.Fatalf("read the label: %v", err)
	}
	return label
}

func hashOf(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func revokeSession(svc *application.Application, userID, family, cookie string) error {
	_, err := svc.Commands.RevokeSession.Handle(context.Background(), command.RevokeSession{
		UserID: userID, FamilyID: family, RefreshToken: cookie, IP: "198.51.100.7", UserAgent: "go-test",
	})
	return err
}

func revokeOthers(svc *application.Application, userID, cookie string) (int, error) {
	return svc.Commands.RevokeOtherSessions.Handle(context.Background(), command.RevokeOtherSessions{
		UserID: userID, RefreshToken: cookie, IP: "198.51.100.7", UserAgent: "go-test",
	})
}

var (
	macChrome  = device{userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36", ip: "203.0.113.5", geoLabel: "Ho Chi Minh City, VN"}
	iphone     = device{userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", ip: "203.0.113.6", geoLabel: "Hanoi, VN"}
	windowsEdg = device{userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0", ip: "203.0.113.7"}
)

func TestSignInStoresTheLocationAndItIsNullWithout(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)

	with := signInFrom(t, svc, pool, email, macChrome)
	without := signInFrom(t, svc, pool, email, windowsEdg)

	if got := geoLabelOf(t, pool, with.refresh); got == nil || *got != "Ho Chi Minh City, VN" {
		t.Errorf("stored label = %v, want Ho Chi Minh City, VN", got)
	}
	if got := geoLabelOf(t, pool, without.refresh); got != nil {
		t.Errorf("stored label = %q for a sign-in without one, want NULL and not an empty string", *got)
	}
}

func TestTheDatabaseRefusesALabelOfMoreThan80Characters(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)

	_, err := svc.Commands.Login.Handle(context.Background(), command.Login{
		Email: email, Password: testPassword, GeoLabel: strings.Repeat("ế", 81),
	})
	if err == nil {
		t.Fatal("a label of 81 characters was stored")
	}
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM app.refresh_tokens WHERE user_id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d tokens were stored by a sign-in that failed", n)
	}
	if _, err := svc.Commands.Login.Handle(context.Background(), command.Login{
		Email: email, Password: testPassword, GeoLabel: strings.Repeat("ế", 80),
	}); err != nil {
		t.Errorf("a label of 80 characters was refused: %v", err)
	}
}

func TestRotationCopiesTheLocationWhenTheRequestCarriesNone(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)
	first := signInFrom(t, svc, pool, email, macChrome)

	res, err := svc.Commands.Refresh.Handle(context.Background(), command.Refresh{Token: first.refresh})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if got := geoLabelOf(t, pool, res.RefreshToken); got == nil || *got != "Ho Chi Minh City, VN" {
		t.Errorf("successor label = %v, want the predecessor's", got)
	}
}

func TestRotationTakesTheLocationOfTheRequestThatCarriesOne(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	first := signInFrom(t, svc, pool, email, macChrome)

	res, err := svc.Commands.Refresh.Handle(context.Background(), command.Refresh{Token: first.refresh, GeoLabel: "Da Nang, VN", UserAgent: "Mozilla/5.0 (Windows NT 10.0) Firefox/127.0"})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if got := geoLabelOf(t, pool, res.RefreshToken); got == nil || *got != "Da Nang, VN" {
		t.Errorf("successor label = %v, want the request's Da Nang, VN", got)
	}
	sessions := sessionsOf(t, svc, id, res.RefreshToken)
	if len(sessions) != 1 || sessions[0].GeoLabel == nil || *sessions[0].GeoLabel != "Da Nang, VN" {
		t.Errorf("sessions = %+v, want one with the latest location", sessions)
	}
	if sessions[0].UserAgent == nil || !strings.Contains(*sessions[0].UserAgent, "Firefox") {
		t.Errorf("the list shows user agent %v, want the latest refresh's", sessions[0].UserAgent)
	}
}

func TestTheListHoldsTheLiveFamiliesOnly(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	laptop := signInFrom(t, svc, pool, email, macChrome)
	phone := signInFrom(t, svc, pool, email, iphone)
	signedOut := signInFrom(t, svc, pool, email, windowsEdg)
	if _, err := svc.Commands.Logout.Handle(context.Background(), command.Logout{Token: signedOut.refresh}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.Refresh.Handle(context.Background(), command.Refresh{Token: phone.refresh}); err != nil {
		t.Fatal(err)
	}

	sessions := sessionsOf(t, svc, id, "")

	if len(sessions) != 2 {
		t.Fatalf("sessions = %v, want the laptop and the phone once each, and not the signed-out one", familiesOf(sessions))
	}
	got := map[string]bool{sessions[0].FamilyID: true, sessions[1].FamilyID: true}
	if !got[laptop.family] || !got[phone.family] || got[signedOut.family] {
		t.Errorf("sessions = %v, want %s and %s", familiesOf(sessions), laptop.family, phone.family)
	}
}

func TestAnExpiredFamilyIsNotListedAndCannotBeRevoked(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	current := signInFrom(t, svc, pool, email, macChrome)
	old := signInFrom(t, svc, pool, email, iphone)
	if _, err := pool.Exec(context.Background(),
		`UPDATE app.refresh_tokens SET issued_at = now() - interval '40 days', expires_at = now() - interval '10 days' WHERE family_id = $1`, old.family); err != nil {
		t.Fatal(err)
	}

	sessions := sessionsOf(t, svc, id, current.refresh)

	if len(sessions) != 1 || sessions[0].FamilyID != current.family {
		t.Errorf("sessions = %v, want only the unexpired %s", familiesOf(sessions), current.family)
	}
	if err := revokeSession(svc, id, old.family, current.refresh); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("revoking an expired family = %v, want ErrSessionNotFound", err)
	}
	if n, err := revokeOthers(svc, id, current.refresh); err != nil || n != 0 {
		t.Errorf("revoke others = %d, %v, want 0: an expired family is not a session to end", n, err)
	}
	if got := epochOf(t, pool, id); got != 0 {
		t.Errorf("epoch = %d after refusals and a no-op, want it unmoved at 0", got)
	}
}

func TestTheCookiesFamilyIsCurrentAndFirst(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	_, otherEmail := makeUser(t, pool)
	older := signInFrom(t, svc, pool, email, macChrome)
	middle := signInFrom(t, svc, pool, email, iphone)
	signInFrom(t, svc, pool, email, windowsEdg)
	stranger := signInFrom(t, svc, pool, otherEmail, macChrome)
	staleOlder := older.refresh
	rotatedOlder, err := svc.Commands.Refresh.Handle(context.Background(), command.Refresh{Token: staleOlder})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		cookie      string
		wantCurrent string
	}{
		{"the head of the family", rotatedOlder.RefreshToken, older.family},
		{"a token the family already rotated", staleOlder, older.family},
		{"another family", middle.refresh, middle.family},
		{"no cookie", "", ""},
		{"an unknown token", "not-a-token-we-issued", ""},
		{"another user's cookie", stranger.refresh, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sessions := sessionsOf(t, svc, id, c.cookie)
			if len(sessions) != 3 {
				t.Fatalf("sessions = %v, want all three of the user's", familiesOf(sessions))
			}
			currents := 0
			for _, s := range sessions {
				if s.Current {
					currents++
					if s.FamilyID != c.wantCurrent {
						t.Errorf("current = %s, want %s", s.FamilyID, c.wantCurrent)
					}
				}
			}
			wantCurrents := 0
			if c.wantCurrent != "" {
				wantCurrents = 1
				if !sessions[0].Current {
					t.Errorf("the current session is not first: %v", familiesOf(sessions))
				}
			}
			if currents != wantCurrents {
				t.Errorf("%d current sessions, want %d", currents, wantCurrents)
			}
		})
	}
}

func TestACookieOfARevokedFamilyMakesNothingCurrent(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	signedOut := signInFrom(t, svc, pool, email, macChrome)
	signInFrom(t, svc, pool, email, iphone)
	if _, err := svc.Commands.Logout.Handle(context.Background(), command.Logout{Token: signedOut.refresh}); err != nil {
		t.Fatal(err)
	}

	for _, s := range sessionsOf(t, svc, id, signedOut.refresh) {
		if s.Current {
			t.Errorf("%s is current for the cookie of a revoked family", s.FamilyID)
		}
	}
}

func TestOtherSessionsFollowByLastUseThenFamilyAndTheListIsBounded(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	oldest := signInFrom(t, svc, pool, email, macChrome)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO app.refresh_tokens (user_id, family_id, token_hash, issued_at, expires_at)
		SELECT $1, gen_random_uuid(), sha256(convert_to($2 || g::text, 'UTF8')), now() - (g || ' minutes')::interval, now() + interval '20 days'
		  FROM generate_series(1, 105) g`, id, "bound-"+id); err != nil {
		t.Fatalf("insert families: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE app.refresh_tokens SET issued_at = now() - interval '2 days' WHERE family_id = $1`, oldest.family); err != nil {
		t.Fatal(err)
	}

	sessions := sessionsOf(t, svc, id, oldest.refresh)

	if len(sessions) != domain.MaxSessions {
		t.Fatalf("%d sessions, want the cap of %d", len(sessions), domain.MaxSessions)
	}
	if sessions[0].FamilyID != oldest.family || !sessions[0].Current {
		t.Errorf("the current session, though the oldest, is not first: %s", sessions[0].FamilyID)
	}
	for i := 2; i < len(sessions); i++ {
		if sessions[i].LastUsedAt.After(sessions[i-1].LastUsedAt) {
			t.Fatalf("session %d is newer than session %d: the rest are not newest first", i, i-1)
		}
	}
}

func TestRevokeSessionEndsTheFamilyMovesTheEpochAuditsAndForgetsAfterTheCommit(t *testing.T) {
	pool := newPool(t)
	svc, spy := serviceWithSpy(t, pool)
	id, email := makeUser(t, pool)
	current := signInFrom(t, svc, pool, email, macChrome)
	target := signInFrom(t, svc, pool, email, iphone)
	bystander := signInFrom(t, svc, pool, email, windowsEdg)

	if err := revokeSession(svc, id, target.family, current.refresh); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	if n := liveTokensInFamily(t, pool, target.family); n != 0 {
		t.Errorf("the revoked family still has %d live tokens", n)
	}
	if n := liveTokensInFamily(t, pool, current.family); n != 1 {
		t.Errorf("the calling family has %d live tokens, want 1", n)
	}
	if n := liveTokensInFamily(t, pool, bystander.family); n != 1 {
		t.Errorf("another family has %d live tokens, want 1", n)
	}
	if got := epochOf(t, pool, id); got != 1 {
		t.Errorf("epoch = %d, want it moved by exactly one", got)
	}
	if got := sessionAudits(t, pool, id); len(got) != 1 || got[0] != "session.revoked" {
		t.Errorf("audit rows = %v, want one session.revoked", got)
	}
	var entity, entityID string
	var diff *string
	var userAgent *string
	if err := pool.QueryRow(context.Background(),
		`SELECT entity, entity_id::text, diff::text, user_agent FROM app.audit_log WHERE actor_user_id = $1 AND action = 'session.revoked'`, id).Scan(&entity, &entityID, &diff, &userAgent); err != nil {
		t.Fatal(err)
	}
	if entity != "refresh_token_family" || entityID != target.family {
		t.Errorf("audited %s %s, want refresh_token_family %s", entity, entityID, target.family)
	}
	if diff != nil {
		t.Errorf("the audit row carries a diff %s: it holds no device or location label", *diff)
	}
	calls := spy.forgets()
	if len(calls) != 1 || calls[0].userID != id {
		t.Fatalf("Forget calls = %+v, want one for the user", calls)
	}
	if calls[0].epoch != 1 || calls[0].auditRow != 1 {
		t.Errorf("at Forget the database held epoch %d and %d audit rows, want 1 and 1: Forget ran before the commit", calls[0].epoch, calls[0].auditRow)
	}
}

func TestARevokedFamilyIsRefusedAsRevokedNotAsReuseAndNothingElseIsTouched(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	current := signInFrom(t, svc, pool, email, macChrome)
	target := signInFrom(t, svc, pool, email, iphone)
	if err := revokeSession(svc, id, target.family, current.refresh); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Commands.Refresh.Handle(context.Background(), command.Refresh{Token: target.refresh})

	if !errors.Is(err, domain.ErrRefreshRejected) {
		t.Fatalf("refresh with the revoked device's cookie = %v, want ErrRefreshRejected", err)
	}
	if got := sessionAudits(t, pool, id); len(got) != 1 {
		t.Errorf("audit rows = %v: a refresh of a revoked family must not add one", got)
	}
	var reuse int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.audit_log WHERE actor_user_id = $1 AND action = 'refresh_token.reuse_detected'`, id).Scan(&reuse); err != nil {
		t.Fatal(err)
	}
	if reuse != 0 {
		t.Errorf("%d reuse detections recorded for a plain revocation", reuse)
	}
	if n := liveTokensInFamily(t, pool, current.family); n != 1 {
		t.Errorf("the calling family has %d live tokens after the revoked device refreshed", n)
	}
}

func TestAStaleCookieOfARevokedFamilyDoesNotTouchTheUsersOtherFamilies(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	current := signInFrom(t, svc, pool, email, macChrome)
	target := signInFrom(t, svc, pool, email, iphone)
	if _, err := svc.Commands.Refresh.Handle(context.Background(), command.Refresh{Token: target.refresh}); err != nil {
		t.Fatal(err)
	}
	if err := revokeSession(svc, id, target.family, current.refresh); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Commands.Refresh.Handle(context.Background(), command.Refresh{Token: target.refresh})

	if !errors.Is(err, domain.ErrRefreshReused) {
		t.Fatalf("refresh with a stale cookie of the revoked family = %v, want the reuse answer it has always given", err)
	}
	if n := liveTokensInFamily(t, pool, current.family); n != 1 {
		t.Errorf("the calling family has %d live tokens: reuse detection reached another family", n)
	}
	if _, err := svc.Commands.Refresh.Handle(context.Background(), command.Refresh{Token: current.refresh}); err != nil {
		t.Errorf("the calling family cannot refresh: %v", err)
	}
}

func TestAnotherUsersFamilyAnswersNotFoundAndChangesNothing(t *testing.T) {
	pool := newPool(t)
	svc, spy := serviceWithSpy(t, pool)
	callerID, callerEmail := makeUser(t, pool)
	ownerID, ownerEmail := makeUser(t, pool)
	caller := signInFrom(t, svc, pool, callerEmail, macChrome)
	owner := signInFrom(t, svc, pool, ownerEmail, iphone)

	for _, family := range []string{owner.family, "019535d9-3df7-79fb-b466-fa907fa17f9e"} {
		if err := revokeSession(svc, callerID, family, caller.refresh); !errors.Is(err, domain.ErrSessionNotFound) {
			t.Errorf("revoking %s = %v, want ErrSessionNotFound", family, err)
		}
	}

	if n := liveTokensInFamily(t, pool, owner.family); n != 1 {
		t.Errorf("the other user's family has %d live tokens, want it untouched", n)
	}
	if got := epochOf(t, pool, ownerID); got != 0 {
		t.Errorf("the other user's epoch = %d", got)
	}
	if got := epochOf(t, pool, callerID); got != 0 {
		t.Errorf("the caller's epoch = %d: a refused revoke moved it", got)
	}
	if got := sessionAudits(t, pool, callerID); len(got) != 0 {
		t.Errorf("audit rows = %v, want none", got)
	}
	if calls := spy.forgets(); len(calls) != 0 {
		t.Errorf("Forget ran %d times for refusals", len(calls))
	}
	for _, s := range sessionsOf(t, svc, callerID, caller.refresh) {
		if s.FamilyID == owner.family {
			t.Error("the other user's family is in the caller's list")
		}
	}
}

func TestARevokedFamilyAnswersNotFoundAsAnUnknownOneDoes(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	current := signInFrom(t, svc, pool, email, macChrome)
	target := signInFrom(t, svc, pool, email, iphone)
	if err := revokeSession(svc, id, target.family, current.refresh); err != nil {
		t.Fatal(err)
	}

	if err := revokeSession(svc, id, target.family, current.refresh); !errors.Is(err, domain.ErrSessionNotFound) {
		t.Errorf("revoking it twice = %v, want ErrSessionNotFound", err)
	}
	if got := epochOf(t, pool, id); got != 1 {
		t.Errorf("epoch = %d, want only the first revoke to have moved it", got)
	}
}

func TestTheCurrentSessionCannotBeRevoked(t *testing.T) {
	pool := newPool(t)
	svc, spy := serviceWithSpy(t, pool)
	id, email := makeUser(t, pool)
	current := signInFrom(t, svc, pool, email, macChrome)
	signInFrom(t, svc, pool, email, iphone)

	err := revokeSession(svc, id, current.family, current.refresh)

	if !errors.Is(err, domain.ErrSessionIsCurrent) {
		t.Fatalf("revoking the current session = %v, want ErrSessionIsCurrent", err)
	}
	if n := liveTokensInFamily(t, pool, current.family); n != 1 {
		t.Errorf("the current family has %d live tokens", n)
	}
	if got := epochOf(t, pool, id); got != 0 {
		t.Errorf("epoch = %d, want unmoved", got)
	}
	if got := sessionAudits(t, pool, id); len(got) != 0 {
		t.Errorf("audit rows = %v, want none", got)
	}
	if calls := spy.forgets(); len(calls) != 0 {
		t.Errorf("Forget ran %d times", len(calls))
	}
}

func TestWithoutACurrentSessionNothingIsRevokedAndTheCookieIsCheckedBeforeTheId(t *testing.T) {
	pool := newPool(t)
	svc, spy := serviceWithSpy(t, pool)
	id, email := makeUser(t, pool)
	_, strangerEmail := makeUser(t, pool)
	current := signInFrom(t, svc, pool, email, macChrome)
	target := signInFrom(t, svc, pool, email, iphone)
	signedOut := signInFrom(t, svc, pool, email, windowsEdg)
	stranger := signInFrom(t, svc, pool, strangerEmail, macChrome)
	if _, err := svc.Commands.Logout.Handle(context.Background(), command.Logout{Token: signedOut.refresh}); err != nil {
		t.Fatal(err)
	}

	cookies := map[string]string{
		"no cookie":             "",
		"an unknown token":      "not-a-token-we-issued",
		"another user's cookie": stranger.refresh,
		"a revoked family":      signedOut.refresh,
	}
	for name, cookie := range cookies {
		t.Run(name, func(t *testing.T) {
			if err := revokeSession(svc, id, target.family, cookie); !errors.Is(err, domain.ErrNoCurrentSession) {
				t.Errorf("revoke one = %v, want ErrNoCurrentSession", err)
			}
			if err := revokeSession(svc, id, "019535d9-3df7-79fb-b466-fa907fa17f9e", cookie); !errors.Is(err, domain.ErrNoCurrentSession) {
				t.Errorf("revoke an unknown id = %v, want ErrNoCurrentSession: the cookie is checked before the id is looked up", err)
			}
			if n, err := revokeOthers(svc, id, cookie); !errors.Is(err, domain.ErrNoCurrentSession) || n != 0 {
				t.Errorf("revoke others = %d, %v, want 0 and ErrNoCurrentSession", n, err)
			}
		})
	}

	for _, family := range []string{current.family, target.family, stranger.family} {
		if n := liveTokensInFamily(t, pool, family); n != 1 {
			t.Errorf("family %s has %d live tokens, want it untouched", family, n)
		}
	}
	if got := epochOf(t, pool, id); got != 0 {
		t.Errorf("epoch = %d, want unmoved", got)
	}
	if calls := spy.forgets(); len(calls) != 0 {
		t.Errorf("Forget ran %d times", len(calls))
	}
}

func TestTheCurrentFamilyRefreshesAfterTheEpochMoves(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	issuer := mustIssuer(t)
	current := signInFrom(t, svc, pool, email, macChrome)
	target := signInFrom(t, svc, pool, email, iphone)
	if err := revokeSession(svc, id, target.family, current.refresh); err != nil {
		t.Fatal(err)
	}

	res, err := svc.Commands.Refresh.Handle(context.Background(), command.Refresh{Token: current.refresh})

	if err != nil {
		t.Fatalf("the calling family cannot refresh after the epoch moved: %v", err)
	}
	claims, err := issuer.Verify(res.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Epoch != epochOf(t, pool, id) || claims.Epoch != 1 {
		t.Errorf("the new access token carries epoch %d, the user's is %d, want both 1", claims.Epoch, epochOf(t, pool, id))
	}
	if _, err := svc.Commands.Refresh.Handle(context.Background(), command.Refresh{Token: target.refresh}); !errors.Is(err, domain.ErrRefreshRejected) {
		t.Errorf("the revoked family refreshed: %v", err)
	}
}

func TestRevokeOtherSessionsKeepsTheCurrentFamilyAndCountsTheRest(t *testing.T) {
	pool := newPool(t)
	svc, spy := serviceWithSpy(t, pool)
	id, email := makeUser(t, pool)
	strangerID, strangerEmail := makeUser(t, pool)
	current := signInFrom(t, svc, pool, email, macChrome)
	phone := signInFrom(t, svc, pool, email, iphone)
	tablet := signInFrom(t, svc, pool, email, windowsEdg)
	expired := signInFrom(t, svc, pool, email, windowsEdg)
	stranger := signInFrom(t, svc, pool, strangerEmail, macChrome)
	if _, err := pool.Exec(context.Background(),
		`UPDATE app.refresh_tokens SET issued_at = now() - interval '40 days', expires_at = now() - interval '10 days' WHERE family_id = $1`, expired.family); err != nil {
		t.Fatal(err)
	}

	revoked, err := revokeOthers(svc, id, current.refresh)

	if err != nil {
		t.Fatalf("revoke others: %v", err)
	}
	if revoked != 2 {
		t.Errorf("revoked = %d, want the phone and the tablet, not the expired family", revoked)
	}
	for name, family := range map[string]string{"phone": phone.family, "tablet": tablet.family} {
		if n := liveTokensInFamily(t, pool, family); n != 0 {
			t.Errorf("the %s still has %d live tokens", name, n)
		}
	}
	if n := liveTokensInFamily(t, pool, current.family); n != 1 {
		t.Errorf("the current family has %d live tokens, want it kept", n)
	}
	if n := liveTokensInFamily(t, pool, stranger.family); n != 1 {
		t.Errorf("another user's family has %d live tokens", n)
	}
	if got := epochOf(t, pool, id); got != 1 {
		t.Errorf("epoch = %d, want one move", got)
	}
	if got := epochOf(t, pool, strangerID); got != 0 {
		t.Errorf("another user's epoch = %d", got)
	}
	var diff string
	if err := pool.QueryRow(context.Background(),
		`SELECT diff::text FROM app.audit_log WHERE actor_user_id = $1 AND action = 'session.revoked_others'`, id).Scan(&diff); err != nil {
		t.Fatalf("read the audit row: %v", err)
	}
	if diff != `{"revoked": 2}` {
		t.Errorf("audit diff = %s, want {\"revoked\": 2} and no device or location label", diff)
	}
	calls := spy.forgets()
	if len(calls) != 1 || calls[0].epoch != 1 || calls[0].auditRow != 1 {
		t.Errorf("Forget calls = %+v, want one after the commit, at epoch 1 with the audit row written", calls)
	}
	if _, err := svc.Commands.Refresh.Handle(context.Background(), command.Refresh{Token: current.refresh}); err != nil {
		t.Errorf("the current family cannot refresh after the others were revoked: %v", err)
	}
}

func TestRevokeOtherSessionsWithNothingToRevokeWritesNothing(t *testing.T) {
	pool := newPool(t)
	svc, spy := serviceWithSpy(t, pool)
	id, email := makeUser(t, pool)
	current := signInFrom(t, svc, pool, email, macChrome)

	revoked, err := revokeOthers(svc, id, current.refresh)

	if err != nil || revoked != 0 {
		t.Fatalf("revoke others = %d, %v, want 0 and no error", revoked, err)
	}
	if got := epochOf(t, pool, id); got != 0 {
		t.Errorf("epoch = %d, want unmoved: a no-op must not sign the caller's tabs out", got)
	}
	if got := sessionAudits(t, pool, id); len(got) != 0 {
		t.Errorf("audit rows = %v, want none", got)
	}
	if calls := spy.forgets(); len(calls) != 0 {
		t.Errorf("Forget ran %d times", len(calls))
	}
}

func TestARevocationWaitsForARotationInFlightAndRevokesItsSuccessor(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	current := signInFrom(t, svc, pool, email, macChrome)
	target := signInFrom(t, svc, pool, email, iphone)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rotation, pid := heldRotation(t, pool, id, target.refresh)

	revoked := make(chan error, 1)
	go func() { revoked <- revokeSession(svc, id, target.family, current.refresh) }()
	waitUntilRevocationIsBlockedBy(t, pool, pid, revoked)

	if err := rotation.Commit(ctx); err != nil {
		t.Fatalf("commit the rotation: %v", err)
	}
	select {
	case err := <-revoked:
		if err != nil {
			t.Fatalf("revoke: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the revocation did not return after the rotation committed")
	}

	if n := liveTokensInFamily(t, pool, target.family); n != 0 {
		t.Errorf("the revoked family has %d live tokens: the successor of the rotation in flight survived", n)
	}
	if n := liveTokensInFamily(t, pool, current.family); n != 1 {
		t.Errorf("the calling family has %d live tokens", n)
	}
}

func TestTwoRevocationsOfOneSessionMoveTheEpochOnce(t *testing.T) {
	pool := newPool(t)
	svc, spy := serviceWithSpy(t, pool)
	id, email := makeUser(t, pool)
	current := signInFrom(t, svc, pool, email, macChrome)
	target := signInFrom(t, svc, pool, email, iphone)

	ctx := context.Background()
	holder, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := holder.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("roll back the lock holder: %v", err)
		}
	})
	var pid int
	if err := holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	var locked bool
	if err := holder.QueryRow(ctx, `SELECT true FROM app.users WHERE id = $1 FOR SHARE`, id).Scan(&locked); err != nil {
		t.Fatal(err)
	}

	results := make(chan error, 2)
	for range 2 {
		go func() { results <- revokeSession(svc, id, target.family, current.refresh) }()
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting int
		if err := pool.QueryRow(ctx, `
			SELECT count(*) FROM pg_stat_activity
			 WHERE datname = current_database() AND pid <> $1 AND wait_event_type = 'Lock'
			   AND query LIKE '%FOR NO KEY UPDATE%'`, pid).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of the two revocations waited for the user's lock", waiting)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := holder.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	var succeeded, notFound int
	for range 2 {
		select {
		case err := <-results:
			switch {
			case err == nil:
				succeeded++
			case errors.Is(err, domain.ErrSessionNotFound):
				notFound++
			default:
				t.Errorf("a revocation answered %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("a revocation never returned")
		}
	}
	if succeeded != 1 || notFound != 1 {
		t.Errorf("%d succeeded and %d answered not found, want one of each", succeeded, notFound)
	}
	if got := epochOf(t, pool, id); got != 1 {
		t.Errorf("epoch = %d, want a single move", got)
	}
	if got := sessionAudits(t, pool, id); len(got) != 1 {
		t.Errorf("audit rows = %v, want one", got)
	}
	if calls := spy.forgets(); len(calls) != 1 {
		t.Errorf("Forget ran %d times, want once", len(calls))
	}
}
