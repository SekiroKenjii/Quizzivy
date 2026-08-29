package auth_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/auth"
)

// Rotation and reuse detection (§5.2). These run against a real database
// because the property being tested is a concurrency property: it lives in the
// row lock, not in the Go code, and an in-memory fake would assert nothing.

// login returns a live refresh token for a fresh user.
func login(t *testing.T, svc *auth.Service, email string) string {
	t.Helper()
	session, err := svc.Login(context.Background(), auth.LoginInput{
		Email: email, Password: testPassword, IP: "203.0.113.7", UserAgent: "go-test",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return session.RefreshToken
}

type tokenRow struct {
	id         string
	familyID   string
	revoked    bool
	replacedBy *string
}

func loadToken(t *testing.T, pool *pgxpool.Pool, token string) tokenRow {
	t.Helper()
	sum := sha256.Sum256([]byte(token))
	var r tokenRow
	var revokedAt *time.Time
	err := pool.QueryRow(context.Background(),
		`SELECT id::text, family_id::text, revoked_at, replaced_by::text
		   FROM app.refresh_tokens WHERE token_hash = $1`, sum[:],
	).Scan(&r.id, &r.familyID, &revokedAt, &r.replacedBy)
	if err != nil {
		t.Fatalf("load token row: %v", err)
	}
	r.revoked = revokedAt != nil
	return r
}

func liveTokensInFamily(t *testing.T, pool *pgxpool.Pool, familyID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.refresh_tokens
		  WHERE family_id = $1 AND revoked_at IS NULL`, familyID).Scan(&n); err != nil {
		t.Fatalf("count live tokens: %v", err)
	}
	return n
}

func TestRotationIssuesASuccessorInTheSameFamilyWithReplacedBySet(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)

	first := login(t, svc, email)
	before := loadToken(t, pool, first)

	res, err := svc.Refresh(context.Background(), auth.RefreshInput{
		Token: first, IP: "203.0.113.9", UserAgent: "go-test",
	})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if res.RefreshToken == first {
		t.Fatal("rotation returned the same token; nothing was rotated")
	}
	if res.AccessToken == "" {
		t.Error("no access token issued")
	}

	after := loadToken(t, pool, first)
	successor := loadToken(t, pool, res.RefreshToken)

	if !after.revoked {
		t.Error("predecessor was not revoked")
	}
	if after.replacedBy == nil || *after.replacedBy != successor.id {
		t.Errorf("predecessor.replaced_by = %v, want the successor %s", after.replacedBy, successor.id)
	}
	if successor.familyID != before.familyID {
		t.Errorf("successor family = %s, want the predecessor's %s -- a rotation that "+
			"changes family cannot be traced back and reuse detection would never fire",
			successor.familyID, before.familyID)
	}
	if successor.revoked {
		t.Error("successor was born revoked")
	}
	if n := liveTokensInFamily(t, pool, before.familyID); n != 1 {
		t.Errorf("live tokens in family = %d, want exactly 1", n)
	}
}

func TestReplayingARotatedTokenRevokesEveryTokenInTheFamily(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)
	ctx := context.Background()

	first := login(t, svc, email)
	family := loadToken(t, pool, first).familyID

	// Rotate twice, so the family has a history rather than a single link.
	second, err := svc.Refresh(ctx, auth.RefreshInput{Token: first})
	if err != nil {
		t.Fatal(err)
	}
	third, err := svc.Refresh(ctx, auth.RefreshInput{Token: second.RefreshToken})
	if err != nil {
		t.Fatal(err)
	}
	if n := liveTokensInFamily(t, pool, family); n != 1 {
		t.Fatalf("precondition: live tokens = %d, want 1", n)
	}

	// Replay the ORIGINAL, long-rotated token.
	_, err = svc.Refresh(ctx, auth.RefreshInput{Token: first})
	if !errors.Is(err, auth.ErrRefreshReused) {
		t.Fatalf("replay error = %v, want ErrRefreshReused", err)
	}

	if n := liveTokensInFamily(t, pool, family); n != 0 {
		t.Errorf("live tokens after reuse = %d, want 0 -- §5.2 revokes the WHOLE family", n)
	}
	if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: third.RefreshToken}); err == nil {
		t.Error("the newest token still works after its family was revoked")
	}
}

func TestConcurrentRefreshesOfOneTokenElectExactlyOneWinner(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)

	first := login(t, svc, email)
	family := loadToken(t, pool, first).familyID

	const racers = 8
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		wins    int
		reuse   int
		others  []error
		release = make(chan struct{})
	)
	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-release // line them all up on the same starting gun
			_, err := svc.Refresh(context.Background(), auth.RefreshInput{Token: first})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				wins++
			case errors.Is(err, auth.ErrRefreshReused):
				reuse++
			default:
				others = append(others, err)
			}
		}()
	}
	close(release)
	wg.Wait()

	if len(others) > 0 {
		t.Fatalf("unexpected errors from concurrent refresh: %v", others)
	}
	if wins != 1 {
		t.Fatalf("%d of %d concurrent refreshes succeeded, want exactly 1", wins, racers)
	}
	if reuse != racers-1 {
		t.Errorf("reuse detections = %d, want %d", reuse, racers-1)
	}
	if n := liveTokensInFamily(t, pool, family); n != 0 {
		t.Errorf("live tokens = %d, want 0 after reuse detection fired", n)
	}
}

func TestAnExpiredTokenIsRejectedButIsNotTreatedAsReuse(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)

	past := time.Now().Add(-90 * 24 * time.Hour)
	svc.SetClock(func() time.Time { return past })
	token := login(t, svc, email)
	family := loadToken(t, pool, token).familyID

	svc.SetClock(time.Now) // the token's 30 days elapsed long ago
	_, err := svc.Refresh(context.Background(), auth.RefreshInput{Token: token})
	if !errors.Is(err, auth.ErrRefreshRejected) {
		t.Fatalf("expired token error = %v, want ErrRefreshRejected", err)
	}
	if errors.Is(err, auth.ErrRefreshReused) {
		t.Error("expiry was reported as reuse")
	}
	row := loadToken(t, pool, token)
	if row.revoked {
		t.Error("an expired token was revoked; expiry is not a security event")
	}
	_ = family
}

func TestAnUnknownTokenIsRejected(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)

	_, err := svc.Refresh(context.Background(), auth.RefreshInput{Token: "not-a-token-we-ever-issued"})
	if !errors.Is(err, auth.ErrRefreshRejected) {
		t.Fatalf("error = %v, want ErrRefreshRejected", err)
	}
}

func TestAnEmptyTokenIsRejectedWithoutTouchingTheDatabase(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)

	if _, err := svc.Refresh(context.Background(), auth.RefreshInput{Token: ""}); !errors.Is(err, auth.ErrRefreshRejected) {
		t.Fatalf("error = %v, want ErrRefreshRejected", err)
	}
}

func TestASuspendedUserCannotRefresh(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	ctx := context.Background()

	token := login(t, svc, email)
	family := loadToken(t, pool, token).familyID

	if _, err := pool.Exec(ctx,
		`UPDATE app.users SET disabled_at = now() WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: token}); !errors.Is(err, auth.ErrRefreshRejected) {
		t.Fatalf("error = %v, want ErrRefreshRejected", err)
	}
	if n := liveTokensInFamily(t, pool, family); n != 0 {
		t.Errorf("live tokens = %d, want 0 -- suspending an account ends its sessions", n)
	}
}

func TestReuseDetectionIsRecordedInTheAuditLog(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	id, email := makeUser(t, pool)
	ctx := context.Background()

	first := login(t, svc, email)
	family := loadToken(t, pool, first).familyID
	if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: first}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(ctx, auth.RefreshInput{
		Token: first, IP: "198.51.100.23", UserAgent: "replayer/1.0",
	}); !errors.Is(err, auth.ErrRefreshReused) {
		t.Fatal("expected reuse detection")
	}

	var action, entity, entityID, ip, userAgent string
	err := pool.QueryRow(ctx,
		`SELECT action, entity, entity_id::text, host(ip), user_agent
		   FROM app.audit_log WHERE actor_user_id = $1 ORDER BY id DESC LIMIT 1`, id,
	).Scan(&action, &entity, &entityID, &ip, &userAgent)
	if err != nil {
		t.Fatalf("no audit row was written for the family revocation: %v", err)
	}
	if action != "refresh_token.reuse_detected" {
		t.Errorf("action = %q", action)
	}
	if entity != "refresh_token_family" || entityID != family {
		t.Errorf("entity = %q/%s, want refresh_token_family/%s", entity, entityID, family)
	}
	if ip != "198.51.100.23" || userAgent != "replayer/1.0" {
		t.Errorf("audit row recorded ip=%q ua=%q, want the REPLAYER's, not the victim's", ip, userAgent)
	}
}

func TestLogoutRevokesTheWholeFamilyNotJustTheCurrentToken(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)
	ctx := context.Background()

	first := login(t, svc, email)
	family := loadToken(t, pool, first).familyID
	second, err := svc.Refresh(ctx, auth.RefreshInput{Token: first})
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Logout(ctx, second.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if n := liveTokensInFamily(t, pool, family); n != 0 {
		t.Errorf("live tokens after logout = %d, want 0", n)
	}
	if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: second.RefreshToken}); err == nil {
		t.Error("the token still refreshes after logout")
	}
}

func TestLogoutIsIdempotentAndForgivingOfUnknownTokens(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)
	ctx := context.Background()

	token := login(t, svc, email)
	for i := range 3 {
		if err := svc.Logout(ctx, token); err != nil {
			t.Fatalf("logout %d: %v", i+1, err)
		}
	}
	if err := svc.Logout(ctx, "a-token-that-was-never-issued"); err != nil {
		t.Errorf("logout with an unknown token = %v, want nil", err)
	}
}

func TestPruningDeletesExpiredTokensAndLeavesLiveOnes(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)
	ctx := context.Background()

	past := time.Now().Add(-90 * 24 * time.Hour)
	svc.SetClock(func() time.Time { return past })
	stale := login(t, svc, email)

	svc.SetClock(time.Now)
	live := login(t, svc, email)

	if _, err := svc.PruneExpiredTokens(ctx); err != nil {
		t.Fatalf("prune: %v", err)
	}

	var n int
	staleHash := sha256.Sum256([]byte(stale))
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.refresh_tokens WHERE token_hash = $1`, staleHash[:]).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("the expired token survived pruning")
	}
	loadToken(t, pool, live) // fatals if the live token was pruned too
}

func TestLoggingOutIsNotReportedAsReuse(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)
	ctx := context.Background()

	token := login(t, svc, email)
	if err := svc.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Refresh(ctx, auth.RefreshInput{Token: token})
	if errors.Is(err, auth.ErrRefreshReused) {
		t.Fatal("refreshing after logout was reported as token REUSE; the student " +
			"would be told someone else used their session")
	}
	if !errors.Is(err, auth.ErrRefreshRejected) {
		t.Fatalf("error = %v, want ErrRefreshRejected", err)
	}
}

func TestAVictimOfSomeoneElsesReplayIsNotAccusedOfReuse(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)
	ctx := context.Background()

	first := login(t, svc, email)
	current, err := svc.Refresh(ctx, auth.RefreshInput{Token: first})
	if err != nil {
		t.Fatal(err)
	}

	// The replay, by someone else.
	if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: first}); !errors.Is(err, auth.ErrRefreshReused) {
		t.Fatalf("replay error = %v, want ErrRefreshReused", err)
	}

	// The victim, holding a token that was revoked but never rotated.
	_, err = svc.Refresh(ctx, auth.RefreshInput{Token: current.RefreshToken})
	if errors.Is(err, auth.ErrRefreshReused) {
		t.Error("the victim's own token was reported as reused")
	}
	if !errors.Is(err, auth.ErrRefreshRejected) {
		t.Errorf("victim error = %v, want ErrRefreshRejected", err)
	}
}

func TestReplayingAnAlreadyReplayedTokenStaysReuse(t *testing.T) {
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)
	ctx := context.Background()

	first := login(t, svc, email)
	if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: first}); err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: first}); !errors.Is(err, auth.ErrRefreshReused) {
			t.Fatalf("replay %d: error = %v, want ErrRefreshReused", i+1, err)
		}
	}
}

func TestPruningNeverBreaksARotationChainItLeavesBehind(t *testing.T) {
	pool := newPool(t)
	_, email := makeUser(t, pool)
	ctx := context.Background()

	long := newService(t, pool) // 30-day tokens
	first := login(t, long, email)
	family := loadToken(t, pool, first).familyID
	short := auth.NewService(auth.NewStore(pool), mustIssuer(t), time.Hour)
	if _, err := short.Refresh(ctx, auth.RefreshInput{Token: first}); err != nil {
		t.Fatal(err)
	}
	short.SetClock(func() time.Time { return time.Now().Add(2 * time.Hour) })
	if _, err := short.PruneExpiredTokens(ctx); err != nil {
		t.Fatalf("prune: %v", err)
	}

	row := loadToken(t, pool, first) // fatals if the predecessor was pruned too
	if row.replacedBy == nil {
		t.Fatal("pruning nulled the predecessor's replaced_by: replaying this token " +
			"would now be treated as a plain rejection and the family would survive")
	}

	// The property that link protects, asserted end to end.
	if _, err := long.Refresh(ctx, auth.RefreshInput{Token: first}); !errors.Is(err, auth.ErrRefreshReused) {
		t.Fatalf("replay after pruning = %v, want ErrRefreshReused", err)
	}
	if n := liveTokensInFamily(t, pool, family); n != 0 {
		t.Errorf("live tokens = %d, want 0 -- reuse detection did not revoke the family", n)
	}
}

func TestPruningRemovesAFullyExpiredFamily(t *testing.T) {
	// The other half: once nothing in the family can authenticate, it all goes.
	pool := newPool(t)
	svc := newService(t, pool)
	_, email := makeUser(t, pool)
	ctx := context.Background()

	past := time.Now().Add(-90 * 24 * time.Hour)
	svc.SetClock(func() time.Time { return past })
	stale := login(t, svc, email)
	if _, err := svc.Refresh(ctx, auth.RefreshInput{Token: stale}); err != nil {
		t.Fatal(err)
	}
	family := loadToken(t, pool, stale).familyID

	svc.SetClock(time.Now)
	live := login(t, svc, email) // a separate, current family

	if _, err := svc.PruneExpiredTokens(ctx); err != nil {
		t.Fatalf("prune: %v", err)
	}

	var n int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.refresh_tokens WHERE family_id = $1`, family).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d rows of the expired family survived pruning", n)
	}
	loadToken(t, pool, live) // fatals if the live family was pruned
}

func mustIssuer(t *testing.T) *auth.TokenIssuer {
	t.Helper()
	issuer, err := auth.NewTokenIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return issuer
}
