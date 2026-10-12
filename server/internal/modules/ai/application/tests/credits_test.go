//go:build integration

package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/ai/application/command"
	"quizzivy/internal/modules/ai/application/ports"
	"quizzivy/internal/modules/ai/domain"
)

func (w *world) reserve(userID string, feature domain.Feature, ceiling int64) (domain.Reservation, error) {
	return w.app.Commands.Reserve.Handle(context.Background(), command.Reserve{Actor: asActor(userID), Feature: feature, Ceiling: ceiling})
}

func (w *world) mustReserve(userID string, feature domain.Feature, ceiling int64) domain.Reservation {
	w.t.Helper()
	r, err := w.reserve(userID, feature, ceiling)
	if err != nil {
		w.t.Fatalf("reserve %d: %v", ceiling, err)
	}
	return r
}

func (w *world) settle(r domain.Reservation, input, output int64) domain.Settled {
	w.t.Helper()
	settled, err := w.app.Commands.Settle.Handle(context.Background(), command.Settle{
		Reservation: r, Tier: ports.TierCapable, Usage: ports.Usage{InputTokens: input, OutputTokens: output}, RequestID: "req_" + nonce(w.t),
	})
	if err != nil {
		w.t.Fatalf("settle: %v", err)
	}
	return settled
}

func TestAReservationThatWouldCrossThePoolIsRefusedByTheConstraintAndNothingIsWritten(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	w.period(month, 500)
	first, second := w.teacher(), w.teacher()

	w.mustReserve(first, domain.FeatureGenerate, 300)
	if _, err := w.reserve(first, domain.FeatureGenerate, 300); !isScope(err, domain.ScopePool) {
		t.Fatalf("a second 300 against a pool of 500: %v, want the pool's refusal", err)
	}
	if p := w.periodRow(month); p.reserved != 300 || p.used != 0 {
		t.Errorf("the pool holds %+v after a refusal, want only the first 300 reserved", p)
	}
	if tr := w.teacherRow(month, first); tr.reserved != 300 {
		t.Errorf("the teacher holds %+v after a refusal", tr)
	}

	if _, err := w.reserve(second, domain.FeatureGenerate, 300); !isScope(err, domain.ScopePool) {
		t.Fatalf("a first use that the pool cannot cover: %v", err)
	}
	if w.hasTeacherRow(month, second) {
		t.Error("a refused first use left the teacher's month behind")
	}
	if n := w.reservations(month); n != 1 {
		t.Errorf("%d reservations after two refusals, want only the first", n)
	}

	w.mustReserve(second, domain.FeatureScore, 200)
	if p := w.periodRow(month); p.reserved != 500 {
		t.Errorf("a reservation that fits to the credit was refused: %+v", p)
	}
}

func TestAReservationPastTheAllowanceIsTheTeachersRefusalEvenWhenThePoolIsAlsoShort(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	w.period(month, 1_500)
	teacher := w.teacher()
	w.allowance(month, teacher, 1_000)

	w.mustReserve(teacher, domain.FeatureVariant, 600)
	if _, err := w.reserve(teacher, domain.FeatureVariant, 600); !isScope(err, domain.ScopeTeacher) {
		t.Fatalf("600 past an allowance of 1,000 with the pool holding room: %v", err)
	}
	if p := w.periodRow(month); p.reserved != 600 {
		t.Errorf("the pool holds %+v, want the teacher's refusal to leave it alone", p)
	}

	other := w.teacher()
	w.mustReserve(other, domain.FeatureVariant, 300)
	if _, err := w.reserve(teacher, domain.FeatureVariant, 700); !isScope(err, domain.ScopeTeacher) {
		t.Fatalf("the allowance and the pool both short: %v, want the teacher told first", err)
	}
	if _, err := w.reserve(other, domain.FeatureVariant, 700); !isScope(err, domain.ScopePool) {
		t.Fatalf("the pool short for a teacher with room: %v", err)
	}
}

func TestTwoConcurrentReservationsAgainstAPoolWithRoomForOneLeaveExactlyOne(t *testing.T) {
	w := newWorld(t)
	first, second := w.teacher(), w.teacher()
	for round := range 25 {
		month := w.newMonth()
		w.period(month, 300)
		results := make(chan error, 2)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for _, user := range []string{first, second} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, err := w.reserve(user, domain.FeatureGenerate, 200)
				results <- err
			}()
		}
		close(start)
		wg.Wait()
		close(results)

		granted, refused := 0, 0
		for err := range results {
			switch {
			case err == nil:
				granted++
			case isScope(err, domain.ScopePool):
				refused++
			default:
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if granted != 1 || refused != 1 {
			t.Fatalf("round %d: %d granted and %d refused, want exactly one of each", round, granted, refused)
		}
		if p := w.periodRow(month); p.reserved != 200 {
			t.Fatalf("round %d: the pool holds %d, want 200", round, p.reserved)
		}
		if n := w.reservations(month); n != 1 {
			t.Fatalf("round %d: %d reservations, want 1", round, n)
		}
	}
}

func TestManyReservationsRacingForOneTeachersAllowanceNeverHoldMoreThanItIs(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	w.period(month, 100_000)
	teacher := w.teacher()
	w.allowance(month, teacher, 1_000)

	var wg sync.WaitGroup
	var mu sync.Mutex
	granted := 0
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := w.reserve(teacher, domain.FeatureScore, 100); err == nil {
				mu.Lock()
				granted++
				mu.Unlock()
			} else if !isScope(err, domain.ScopeTeacher) {
				t.Errorf("unexpected: %v", err)
			}
		}()
	}
	wg.Wait()
	if granted != 10 {
		t.Errorf("%d of 16 reservations of 100 were granted against 1,000, want 10", granted)
	}
	if tr := w.teacherRow(month, teacher); tr.reserved != 1_000 {
		t.Errorf("the teacher holds %d, want 1,000", tr.reserved)
	}
	if p := w.periodRow(month); p.reserved != 1_000 {
		t.Errorf("the pool holds %d, want 1,000", p.reserved)
	}
}

func TestSettleMovesExactlyTheMeteredAmountAndAppendsTheLedger(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	teacher := w.teacher()
	r := w.mustReserve(teacher, domain.FeatureGenerate, 200)

	settled, err := w.app.Commands.Settle.Handle(context.Background(), command.Settle{
		Reservation: r, Tier: ports.TierCapable, RequestID: "req_exact",
		Usage: ports.Usage{InputTokens: 100, OutputTokens: 20, CacheReadTokens: 100, CacheWriteTokens: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := int64(100 + 40 + 10 + 3); settled.Charged != want || settled.Metered != want {
		t.Errorf("charged %d, metered %d; want %d (100 in, 20 out at two, 100 cached at a tenth, 3 written as input)", settled.Charged, settled.Metered, want)
	}
	tr, p := w.teacherRow(month, teacher), w.periodRow(month)
	if tr.used != 153 || tr.reserved != 0 || p.used != 153 || p.reserved != 0 {
		t.Errorf("teacher %+v, pool %+v; want 153 used and nothing held", tr, p)
	}
	if n := w.reservations(month); n != 0 {
		t.Errorf("%d reservations left", n)
	}
	var feature, tier, request string
	var in, out, read, write, credits int64
	var user *string
	if err := w.pool.QueryRow(context.Background(), `
		SELECT user_id::text, feature, tier, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, credits, request_id
		  FROM app.ai_usage_ledger WHERE month = $1::date`, month).Scan(&user, &feature, &tier, &in, &out, &read, &write, &credits, &request); err != nil {
		t.Fatal(err)
	}
	if user == nil || *user != teacher || feature != "generate" || tier != "capable" || in != 100 || out != 20 || read != 100 || write != 3 || credits != 153 || request != "req_exact" {
		t.Errorf("ledger: %v %s %s %d %d %d %d %d %s", user, feature, tier, in, out, read, write, credits, request)
	}
}

func TestSettleNeverChargesMoreThanTheReservationAndTheLedgerKeepsTheTokens(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	w.period(month, 300)
	teacher := w.teacher()
	r := w.mustReserve(teacher, domain.FeatureScore, 100)

	settled := w.settle(r, 150, 0)
	if settled.Charged != 100 || settled.Metered != 150 {
		t.Errorf("settled %+v, want 100 charged of 150 metered", settled)
	}
	if p := w.periodRow(month); p.used != 100 || p.reserved != 0 {
		t.Errorf("pool %+v", p)
	}
	var credits, tokens int64
	if err := w.pool.QueryRow(context.Background(), `SELECT credits, input_tokens FROM app.ai_usage_ledger WHERE month = $1::date`, month).Scan(&credits, &tokens); err != nil {
		t.Fatal(err)
	}
	if credits != 100 || tokens != 150 {
		t.Errorf("ledger holds %d credits for %d tokens, want 100 credits for the 150 the provider reported", credits, tokens)
	}
}

func TestReleaseReturnsTheCeilingAndAnotherReleaseDoesNothing(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	w.period(month, 300)
	teacher := w.teacher()

	r := w.mustReserve(teacher, domain.FeatureCheck, 300)
	if _, err := w.reserve(teacher, domain.FeatureCheck, 1); !isScope(err, domain.ScopePool) {
		t.Fatalf("the pool is full: %v", err)
	}
	for range 2 {
		if _, err := w.app.Commands.Release.Handle(context.Background(), command.Release{Reservation: r}); err != nil {
			t.Fatal(err)
		}
	}
	if tr, p := w.teacherRow(month, teacher), w.periodRow(month); tr.reserved != 0 || p.reserved != 0 || tr.used != 0 {
		t.Errorf("teacher %+v, pool %+v after a release", tr, p)
	}
	if n := w.count(`SELECT count(*) FROM app.ai_usage_ledger WHERE month = $1::date`, month); n != 0 {
		t.Errorf("a release wrote %d ledger rows", n)
	}
	w.mustReserve(teacher, domain.FeatureCheck, 300)
}

func TestAPausedTeacherIsRefusedBeforeAnyLockIsTaken(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	w.period(month, 5_000)
	paused, active := w.teacher(), w.teacher()
	w.allowance(month, paused, 2_000)
	w.allowance(month, active, 2_000)
	w.exec(`UPDATE app.ai_teacher_credits SET paused = true WHERE month = $1::date AND user_id = $2::uuid`, month, paused)

	held, err := w.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := held.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("releasing the lock: %v", err)
		}
	})
	if _, err := held.Exec(context.Background(), `SELECT 1 FROM app.ai_credit_periods WHERE month = $1::date FOR UPDATE`, month); err != nil {
		t.Fatal(err)
	}

	budget, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	began := time.Now()
	_, err = w.app.Commands.Reserve.Handle(budget, command.Reserve{Actor: asActor(paused), Feature: domain.FeatureScore, Ceiling: 30})
	if !errors.Is(err, domain.ErrPaused) {
		t.Fatalf("a paused teacher: %v", err)
	}
	if took := time.Since(began); took > time.Second {
		t.Errorf("the refusal took %s with the pool's row locked, so it waited for the lock", took)
	}

	blocked, cancelBlocked := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancelBlocked()
	if _, err := w.app.Commands.Reserve.Handle(blocked, command.Reserve{Actor: asActor(active), Feature: domain.FeatureScore, Ceiling: 30}); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a teacher who is not paused: %v, want it to wait for the held lock", err)
	}
}

func TestAPauseThatLandsBetweenTheReadAndTheLockStillRefuses(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	w.period(month, 5_000)
	teacher := w.teacher()
	w.allowance(month, teacher, 2_000)

	held, err := w.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Rollback(context.Background()) })
	if _, err := held.Exec(context.Background(), `SELECT 1 FROM app.ai_credit_periods WHERE month = $1::date FOR UPDATE`, month); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		_, err := w.reserve(teacher, domain.FeatureScore, 30)
		result <- err
	}()
	time.Sleep(200 * time.Millisecond)
	if _, err := held.Exec(context.Background(), `UPDATE app.ai_teacher_credits SET paused = true WHERE month = $1::date AND user_id = $2::uuid`, month, teacher); err != nil {
		t.Fatal(err)
	}
	if err := held.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, domain.ErrPaused) {
			t.Errorf("a reservation that waited while the teacher was paused: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the reservation never returned")
	}
	if tr := w.teacherRow(month, teacher); tr.reserved != 0 {
		t.Errorf("a paused teacher holds %d", tr.reserved)
	}
}

func TestTheMonthRollsOverAtMidnightInTheCentresZone(t *testing.T) {
	w := newWorld(t)
	march, april := time.Date(2400, 3, 1, 0, 0, 0, 0, time.UTC), time.Date(2400, 4, 1, 0, 0, 0, 0, time.UTC)
	w.track(march, april)
	teacher := w.teacher()

	w.at(time.Date(2400, 3, 31, 16, 59, 59, 0, time.UTC))
	r1 := w.mustReserve(teacher, domain.FeatureScore, 30)
	w.at(time.Date(2400, 3, 31, 17, 0, 0, 0, time.UTC))
	r2 := w.mustReserve(teacher, domain.FeatureScore, 50)

	if !r1.Month.Equal(march) || !r2.Month.Equal(april) {
		t.Fatalf("months %v and %v, want March and April", r1.Month, r2.Month)
	}
	if a, b := w.teacherRow(march, teacher), w.teacherRow(april, teacher); a.reserved != 30 || b.reserved != 50 {
		t.Errorf("March holds %d and April %d, want 30 and 50", a.reserved, b.reserved)
	}
	if a, b := w.periodRow(march), w.periodRow(april); a.reserved != 30 || b.reserved != 50 {
		t.Errorf("the pools hold %d and %d", a.reserved, b.reserved)
	}

	w.at(time.Date(2400, 4, 20, 0, 0, 0, 0, time.UTC))
	w.settle(r1, 25, 0)
	if a, b := w.periodRow(march), w.periodRow(april); a.used != 25 || a.reserved != 0 || b.used != 0 || b.reserved != 50 {
		t.Errorf("a call that began in March and ended in April was charged to %+v and %+v", a, b)
	}
}

func TestACustomAllowanceAndAPauseFollowTheTeacherIntoTheNextMonth(t *testing.T) {
	w := newWorld(t)
	first := w.newMonth()
	second := first.AddDate(0, 1, 0)
	w.track(second)
	teacher := w.teacher()
	admin := domain.Author{UserID: w.teacher()}
	yes := true

	if _, err := w.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: admin, UserID: teacher, Custom: &yes, Credits: 30_000, Paused: &yes}); err != nil {
		t.Fatal(err)
	}
	w.exec(`
		UPDATE app.ai_teacher_credits SET allowance_credits = allowance_credits + 5000, granted_credits = granted_credits + 5000
		 WHERE month = $1::date AND user_id = $2::uuid`, first, teacher)

	w.at(second.Add(48 * time.Hour))
	if _, err := w.reserve(teacher, domain.FeatureScore, 30); !errors.Is(err, domain.ErrPaused) {
		t.Fatalf("a teacher paused last month, reserving in a month with no row: %v", err)
	}
	if w.hasTeacherRow(second, teacher) {
		t.Error("a refusal created the month's row")
	}
	usage, err := w.app.Queries.Usage.Handle(context.Background(), usageOf(teacher))
	if err != nil || !usage.Paused || usage.Allowance != 30_000 {
		t.Fatalf("usage in the new month: %+v, %v; want the custom 30,000 and the pause, without last month's grant", usage, err)
	}

	no := false
	if _, err := w.app.Commands.SetAllowance.Handle(context.Background(), command.SetAllowance{By: admin, UserID: teacher, Paused: &no}); err != nil {
		t.Fatal(err)
	}
	if tr := w.teacherRow(second, teacher); tr.allowance != 30_000 || tr.granted != 0 || !tr.custom || tr.paused {
		t.Errorf("the new month's row is %+v, want a custom 30,000 with no grant and no pause", tr)
	}
	w.mustReserve(teacher, domain.FeatureScore, 30)
}

func TestTheSweepReturnsOnlyWhatIsStaleAndASettlementAfterItFindsNothing(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	w.period(month, 1_000)
	teacher := w.teacher()

	stale := w.mustReserve(teacher, domain.FeatureGenerate, 200)
	fresh := w.mustReserve(teacher, domain.FeatureScore, 30)
	w.exec(`UPDATE app.ai_reservations SET created_at = now() - interval '11 minutes' WHERE id = $1::uuid`, stale.ID)
	w.at(time.Now())

	released, err := w.app.Commands.ReleaseStale.Handle(context.Background(), command.ReleaseStale{})
	if err != nil {
		t.Fatal(err)
	}
	if released.Reservations < 1 || released.Credits < 200 {
		t.Errorf("released %+v, want at least the stale 200", released)
	}
	if tr, p := w.teacherRow(month, teacher), w.periodRow(month); tr.reserved != 30 || p.reserved != 30 {
		t.Errorf("teacher %+v, pool %+v; want only the fresh 30 still held", tr, p)
	}
	if n := w.reservations(month); n != 1 {
		t.Errorf("%d reservations left, want the fresh one", n)
	}

	_, err = w.app.Commands.Settle.Handle(context.Background(), command.Settle{Reservation: stale, Tier: ports.TierCapable, Usage: ports.Usage{InputTokens: 50}})
	if !errors.Is(err, domain.ErrReservationGone) {
		t.Fatalf("settling a swept reservation: %v", err)
	}
	if n := w.count(`SELECT count(*) FROM app.ai_usage_ledger WHERE month = $1::date`, month); n != 0 {
		t.Errorf("a settlement that found nothing wrote %d ledger rows", n)
	}
	if p := w.periodRow(month); p.used != 0 || p.reserved != 30 {
		t.Errorf("pool %+v", p)
	}

	again, err := w.app.Commands.ReleaseStale.Handle(context.Background(), command.ReleaseStale{})
	if err != nil {
		t.Fatal(err)
	}
	if w.count(`SELECT count(*) FROM app.ai_reservations WHERE id = $1::uuid`, fresh.ID) != 1 || again.Credits >= 200 {
		t.Errorf("a second sweep released %+v or took the fresh reservation", again)
	}
}

func TestFinishSettlesARefusalTheProviderBilledAndReleasesAnOutage(t *testing.T) {
	w := newWorld(t)
	month := w.newMonth()
	w.period(month, 1_000)
	teacher := w.teacher()

	billedCall := w.mustReserve(teacher, domain.FeatureGenerate, 200)
	settled, err := w.app.Commands.Finish.Handle(context.Background(), command.Finish{
		Reservation: billedCall, Tier: ports.TierCapable,
		Response: ports.Response{Usage: ports.Usage{InputTokens: 80, OutputTokens: 10}, RequestID: "req_refused"}, Err: ports.ErrRefused,
	})
	if err != nil || settled.Charged != 100 {
		t.Fatalf("a refusal: %+v, %v", settled, err)
	}
	outage := w.mustReserve(teacher, domain.FeatureGenerate, 200)
	if _, err := w.app.Commands.Finish.Handle(context.Background(), command.Finish{Reservation: outage, Tier: ports.TierCapable, Err: ports.ErrUnavailable}); err != nil {
		t.Fatal(err)
	}
	if tr, p := w.teacherRow(month, teacher), w.periodRow(month); tr.used != 100 || tr.reserved != 0 || p.used != 100 || p.reserved != 0 {
		t.Errorf("teacher %+v, pool %+v; want the refusal charged and the outage returned", tr, p)
	}
	if n := w.count(`SELECT count(*) FROM app.ai_usage_ledger WHERE month = $1::date`, month); n != 1 {
		t.Errorf("%d ledger rows, want one", n)
	}
}
