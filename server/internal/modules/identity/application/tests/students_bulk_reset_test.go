package application_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
)

type bulkShelf struct {
	domain.Students
	mu     sync.Mutex
	known  map[string]domain.Student
	refuse map[string]error
	fail   map[string]error
	lost   map[string]error
	panics map[string]bool
	delay  map[string]time.Duration
	tried  []string
	batch  map[string]string
	done   []string
	hashes map[string]string
}

func newBulkShelf(ids ...string) *bulkShelf {
	shelf := &bulkShelf{
		known:  map[string]domain.Student{},
		refuse: map[string]error{},
		fail:   map[string]error{},
		lost:   map[string]error{},
		panics: map[string]bool{},
		delay:  map[string]time.Duration{},
		hashes: map[string]string{},
		batch:  map[string]string{},
	}
	for _, id := range ids {
		shelf.known[id] = domain.Student{ID: id, FullName: "Học sinh " + id, Email: id + "@example.test"}
	}
	return shelf
}

func (b *bulkShelf) Get(_ context.Context, _ access.Scope, id string) (domain.Student, error) {
	student, ok := b.known[id]
	if !ok {
		return domain.Student{}, domain.ErrStudentNotFound
	}
	return student, nil
}

func (b *bulkShelf) CheckResettable(_ context.Context, _ domain.WriteRequest, id string) error {
	return b.refuse[id]
}

func (b *bulkShelf) ResetPassword(_ context.Context, req domain.WriteRequest, id, hash string, _ time.Time) error {
	b.mu.Lock()
	b.tried = append(b.tried, id)
	b.batch[id] = req.BatchID
	b.mu.Unlock()
	time.Sleep(b.delay[id])
	if b.panics[id] {
		panic("the connection pool closed under " + id)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.fail[id]; err != nil {
		return err
	}
	b.done = append(b.done, id)
	b.hashes[id] = hash
	return b.lost[id]
}

func (b *bulkShelf) reset(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.hashes[id]
	return ok
}

func (b *bulkShelf) attempted() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := slices.Clone(b.tried)
	slices.Sort(out)
	return out
}

func (b *bulkShelf) committed() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := slices.Clone(b.done)
	slices.Sort(out)
	return out
}

type forgetAfterCommit struct {
	shelf     *bulkShelf
	sets      map[string]access.Set
	unknown   map[string]bool
	mu        sync.Mutex
	forgotten []string
	early     []string
}

func (f *forgetAfterCommit) Resolve(_ context.Context, id string) (access.Principal, error) {
	if f.unknown[id] {
		return access.Principal{}, accessdomain.ErrUnknownUser
	}
	return access.Principal{UserID: id, Permissions: f.sets[id]}, nil
}

func (f *forgetAfterCommit) Forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forgotten = append(f.forgotten, id)
	if !f.shelf.reset(id) {
		f.early = append(f.early, id)
	}
}

func (f *forgetAfterCommit) forgottenIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := slices.Clone(f.forgotten)
	slices.Sort(out)
	return out
}

type passwordSource struct {
	calls    atomic.Int64
	inFlight atomic.Int64
	peak     atomic.Int64
	failOn   int64
	onCall   func(n int64)
}

func (p *passwordSource) make(_ context.Context) (string, string, error) {
	n := p.calls.Add(1)
	now := p.inFlight.Add(1)
	defer p.inFlight.Add(-1)
	for {
		peak := p.peak.Load()
		if now <= peak || p.peak.CompareAndSwap(peak, now) {
			break
		}
	}
	if p.onCall != nil {
		p.onCall(n)
	}
	time.Sleep(time.Millisecond)
	if n == p.failOn {
		return "", "", errors.New("the entropy source was refused")
	}
	return fmt.Sprintf("secret-%d", n), fmt.Sprintf("hash-%d", n), nil
}

func (p *passwordSource) waitForInFlight(want int64, within time.Duration) {
	deadline := time.Now().Add(within)
	for p.inFlight.Load() < want && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
}

type bulkRig struct {
	app       *application.Application
	shelf     *bulkShelf
	principal *forgetAfterCommit
	passwords *passwordSource
	logs      *bytes.Buffer
}

func newBulkRig(ids ...string) *bulkRig {
	shelf := newBulkShelf(ids...)
	rig := &bulkRig{
		shelf:     shelf,
		principal: &forgetAfterCommit{shelf: shelf, sets: map[string]access.Set{}, unknown: map[string]bool{}},
		passwords: &passwordSource{},
		logs:      &bytes.Buffer{},
	}
	rig.app = application.New(nil, nil, 0, shelf, noFigures{})
	rig.app.SetPrincipals(rig.principal)
	rig.app.SetTemporaryPasswords(rig.passwords.make)
	rig.app.SetLogger(slog.New(slog.NewTextHandler(rig.logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return rig
}

func (r *bulkRig) reset(ctx context.Context, req domain.WriteRequest, ids ...string) domain.BulkReset {
	result, err := r.app.Commands.ResetStudentsPasswords.Handle(ctx, command.ResetStudentsPasswords{Request: req, IDs: ids})
	if err != nil {
		panic("a bulk reset answered an error: " + err.Error())
	}
	return result
}

func resetIDs(result domain.BulkReset) []string {
	out := make([]string, 0, len(result.Reset))
	for _, r := range result.Reset {
		out = append(out, r.StudentID)
	}
	return out
}

func failureOf(result domain.BulkReset) []string {
	out := make([]string, 0, len(result.Failed))
	for _, f := range result.Failed {
		out = append(out, f.StudentID)
	}
	return out
}

func reasonFor(result domain.BulkReset, id string) error {
	for _, f := range result.Failed {
		if f.StudentID == id {
			return f.Reason
		}
	}
	return nil
}

var teacherRequest = domain.WriteRequest{ActorID: "teacher", Grants: access.NewSet(access.PeopleStudentsResetPassword)}

func TestABulkResetAnswersEachStudentInTheOrderAskedWithItsOwnReason(t *testing.T) {
	rig := newBulkRig("a", "grader", "shared", "b", "broken", "disabled", "gone", "c")
	rig.principal.sets["grader"] = access.NewSet(access.LearningTakeTests, access.TeachingGrading)
	rig.principal.sets["hidden"] = access.NewSet(access.LearningTakeTests, access.TeachingGrading)
	rig.principal.unknown["gone"] = true
	rig.shelf.fail["shared"] = domain.ErrStudentShared
	rig.shelf.fail["broken"] = errors.New("pq: deadlock detected")
	rig.shelf.delay["a"] = 30 * time.Millisecond
	disabled := time.Now()
	student := rig.shelf.known["disabled"]
	student.DisabledAt = &disabled
	rig.shelf.known["disabled"] = student

	result := rig.reset(context.Background(), teacherRequest, "a", "missing", "hidden", "grader", "shared", "b", "broken", "disabled", "gone", "c")

	if got, want := resetIDs(result), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("reset %v, want %v in the order asked, though a finished last", got, want)
	}
	if got, want := failureOf(result), []string{"missing", "hidden", "grader", "shared", "broken", "disabled", "gone"}; !slices.Equal(got, want) {
		t.Errorf("failed %v, want %v in the order asked", got, want)
	}
	for id, want := range map[string]error{
		"missing":  domain.ErrStudentNotFound,
		"hidden":   domain.ErrStudentNotFound,
		"grader":   domain.ErrForbidden,
		"shared":   domain.ErrStudentShared,
		"broken":   domain.ErrResetFailed,
		"disabled": domain.ErrStudentNotFound,
		"gone":     domain.ErrStudentNotFound,
	} {
		if got := reasonFor(result, id); !errors.Is(got, want) {
			t.Errorf("%s failed with %v, want %v", id, got, want)
		}
	}
	if got := rig.passwords.calls.Load(); got != 5 {
		t.Errorf("%d passwords were made, want 5: one for each student who passed the guards, none for a refused one", got)
	}
	seen := map[string]bool{}
	for _, r := range result.Reset {
		if r.TemporaryPassword == "" || seen[r.TemporaryPassword] {
			t.Errorf("%s got the password %q, want a non-empty one of its own", r.StudentID, r.TemporaryPassword)
		}
		seen[r.TemporaryPassword] = true
		if r.FullName != "Học sinh "+r.StudentID || r.Email != r.StudentID+"@example.test" {
			t.Errorf("%s is named %q <%s>", r.StudentID, r.FullName, r.Email)
		}
		if rig.shelf.hashes[r.StudentID] == "" || strings.Contains(rig.shelf.hashes[r.StudentID], r.TemporaryPassword) {
			t.Errorf("%s was stored as %q for the password %q", r.StudentID, rig.shelf.hashes[r.StudentID], r.TemporaryPassword)
		}
	}
	if got, want := rig.shelf.committed(), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("committed %v, want %v", got, want)
	}
	if got, want := rig.principal.forgottenIDs(), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("forgot %v, want exactly the students who were reset, %v", got, want)
	}
	if len(rig.principal.early) != 0 {
		t.Errorf("forgot %v before their reset was stored", rig.principal.early)
	}
}

func TestAStudentTheResetWouldRefuseCostsNoHash(t *testing.T) {
	rig := newBulkRig("a", "shared", "b", "vanished", "fault", "c")
	rig.shelf.refuse["shared"] = domain.ErrStudentShared
	rig.shelf.refuse["vanished"] = domain.ErrStudentNotFound
	rig.shelf.refuse["fault"] = errors.New("pq: the connection was reset")
	rig.shelf.fail["c"] = domain.ErrStudentShared

	result := rig.reset(context.Background(), teacherRequest, "a", "shared", "b", "vanished", "fault", "c")

	if got, want := resetIDs(result), []string{"a", "b"}; !slices.Equal(got, want) {
		t.Errorf("reset %v, want %v", got, want)
	}
	for id, want := range map[string]error{
		"shared":   domain.ErrStudentShared,
		"vanished": domain.ErrStudentNotFound,
		"fault":    domain.ErrResetFailed,
		"c":        domain.ErrStudentShared,
	} {
		if got := reasonFor(result, id); !errors.Is(got, want) {
			t.Errorf("%s failed with %v, want %v", id, got, want)
		}
	}
	if got := rig.passwords.calls.Load(); got != 3 {
		t.Errorf("%d passwords were made, want 3: one for each of a and b and one for c, whose refusal only the transaction could give", got)
	}
	if got, want := rig.shelf.attempted(), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Errorf("the transaction ran for %v, want %v and not a student refused before the hash", got, want)
	}
}

func TestASingleResetOfAStudentItWouldRefuseCostsNoHash(t *testing.T) {
	app, shelf := guarded()
	passwords := &passwordSource{}
	app.SetTemporaryPasswords(passwords.make)
	shelf.refuse = map[string]error{"pupil": domain.ErrStudentShared}
	reset := command.ResetStudentPassword{Request: teacherRequest, ID: "pupil"}

	if _, err := app.Commands.ResetStudentPassword.Handle(context.Background(), reset); !errors.Is(err, domain.ErrStudentShared) {
		t.Fatalf("a shared student answered %v", err)
	}
	if got := passwords.calls.Load(); got != 0 || len(shelf.writes) != 0 {
		t.Errorf("a refused student cost %d passwords and %v writes, want none", got, shelf.writes)
	}

	shelf.refuse = nil
	if _, err := app.Commands.ResetStudentPassword.Handle(context.Background(), reset); err != nil {
		t.Fatal(err)
	}
	if got := passwords.calls.Load(); got != 1 || len(shelf.writes) != 1 {
		t.Errorf("a student who proceeds cost %d passwords and %v writes, want one of each", got, shelf.writes)
	}
}

type forgetLog struct {
	heldPermissions
	mu  sync.Mutex
	ids []string
}

func (f *forgetLog) Forget(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ids = append(f.ids, id)
}

func (f *forgetLog) forgotten() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.ids)
}

func TestAResetThatMayHaveCommittedForgetsTheStudentAndOneThatCannotHaveDoesNot(t *testing.T) {
	for name, c := range map[string]struct {
		answer error
		want   bool
	}{
		"it committed":                       {nil, true},
		"the request was cancelled":          {fmt.Errorf("students: commit reset: %w", context.Canceled), true},
		"the request's deadline passed":      {fmt.Errorf("students: reset password: timeout: %w", context.DeadlineExceeded), true},
		"the student was shared":             {domain.ErrStudentShared, false},
		"the student was gone":               {domain.ErrStudentNotFound, false},
		"the database refused the statement": {errors.New("pq: deadlock detected"), false},
	} {
		t.Run(name, func(t *testing.T) {
			shelf := &guardedShelf{visible: map[string]bool{"pupil": true}, failed: map[string]error{"pupil": c.answer}}
			log := &forgetLog{heldPermissions: heldPermissions{sets: map[string]access.Set{"pupil": access.NewSet(access.LearningTakeTests)}}}
			app := application.New(nil, nil, 0, shelf, noFigures{})
			app.SetPrincipals(log)
			app.SetTemporaryPasswords((&passwordSource{}).make)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err := app.Commands.ResetStudentPassword.Handle(ctx, command.ResetStudentPassword{Request: teacherRequest, ID: "pupil"})

			if (err == nil) != (c.answer == nil) || (c.answer != nil && !errors.Is(err, c.answer)) {
				t.Errorf("answered %v for the reset's %v", err, c.answer)
			}
			if got := log.forgotten(); c.want != (len(got) == 1 && got[0] == "pupil") || len(got) > 1 {
				t.Errorf("forgot %v, want the student forgotten exactly when %v", got, c.want)
			}
		})
	}
}

func TestABulkResetForgetsEveryStudentWhoseCommitMayHaveHappened(t *testing.T) {
	rig := newBulkRig("a", "cancelled", "late", "b", "shared", "broken")
	rig.shelf.lost["cancelled"] = fmt.Errorf("students: commit reset: %w", context.Canceled)
	rig.shelf.lost["late"] = fmt.Errorf("students: reset password: %w", context.DeadlineExceeded)
	rig.shelf.fail["shared"] = domain.ErrStudentShared
	rig.shelf.fail["broken"] = errors.New("pq: deadlock detected")

	result := rig.reset(context.Background(), teacherRequest, "a", "cancelled", "late", "b", "shared", "broken")

	if got, want := resetIDs(result), []string{"a", "b"}; !slices.Equal(got, want) {
		t.Errorf("reset %v, want %v", got, want)
	}
	for _, id := range []string{"cancelled", "late", "broken"} {
		if got := reasonFor(result, id); !errors.Is(got, domain.ErrResetFailed) {
			t.Errorf("%s failed with %v, want the reset-failed reason: the password cannot be handed out", id, got)
		}
	}
	if got, want := rig.principal.forgottenIDs(), []string{"a", "b", "cancelled", "late"}; !slices.Equal(got, want) {
		t.Errorf("forgot %v, want %v: the students reset and the two whose commit the request could not read, not the refused or the faulted", got, want)
	}
	if len(rig.principal.early) != 0 {
		t.Errorf("forgot %v before their reset was stored", rig.principal.early)
	}
}

func TestEveryStudentOfOneBulkResetIsWrittenUnderOneBatchIDAndAnotherResetUsesAnother(t *testing.T) {
	ids := make([]string, 12)
	for i := range ids {
		ids[i] = fmt.Sprintf("s%02d", i)
	}
	rig := newBulkRig(ids...)
	rig.shelf.fail["s03"] = domain.ErrStudentShared

	first := rig.reset(context.Background(), teacherRequest, ids...)
	batches := maps.Clone(rig.shelf.batch)
	second := rig.reset(context.Background(), teacherRequest, ids[:4]...)

	if len(first.Reset) != 11 || len(second.Reset) != 3 {
		t.Fatalf("reset %d and %d students, want 11 and 3", len(first.Reset), len(second.Reset))
	}
	if len(batches) != len(ids) {
		t.Fatalf("%d students were written, want %d: a student the transaction refuses is still written under the batch", len(batches), len(ids))
	}
	one := batches[ids[0]]
	if parsed, err := uuid.Parse(one); err != nil || parsed.Version() != 7 {
		t.Fatalf("the batch id is %q (%v), want a version 7 uuid", one, err)
	}
	for id, got := range batches {
		if got != one {
			t.Errorf("%s was written under the batch %q, want %q: one call is one batch", id, got, one)
		}
	}
	for _, id := range ids[:4] {
		if got := rig.shelf.batch[id]; got == one || got == "" {
			t.Errorf("%s was written under the batch %q again, want a batch of its own for the second call", id, got)
		}
	}
	if teacherRequest.BatchID != "" {
		t.Errorf("the caller's request was given the batch %q, want it left as it was", teacherRequest.BatchID)
	}
}

func TestABulkResetPassesTheGuardsTheSingleResetPasses(t *testing.T) {
	app, shelf := guarded()
	ids := []string{"pupil", "grader", "stranger", "gone"}
	result, err := app.Commands.ResetStudentsPasswords.Handle(context.Background(), command.ResetStudentsPasswords{Request: teacherRequest, IDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	if got := resetIDs(result); !slices.Equal(got, []string{"pupil"}) {
		t.Errorf("reset %v, want only the student the single reset also reaches", got)
	}
	for _, id := range ids {
		_, single := app.Commands.ResetStudentPassword.Handle(context.Background(), command.ResetStudentPassword{Request: teacherRequest, ID: id})
		bulk := reasonFor(result, id)
		if (single == nil) != (bulk == nil) || (single != nil && !errors.Is(single, bulk)) {
			t.Errorf("%s: the single reset answered %v and the bulk reset %v", id, single, bulk)
		}
	}
	if len(shelf.writes) != 2 {
		t.Errorf("wrote %v, want one reset from each command and nothing for a refused student", shelf.writes)
	}
}

func TestAFaultIsLoggedWithTheStudentAndTheCauseButNeverThePassword(t *testing.T) {
	rig := newBulkRig("a", "broken", "panics", "b")
	rig.shelf.fail["broken"] = errors.New("pq: deadlock detected")
	rig.shelf.panics["panics"] = true

	result := rig.reset(context.Background(), teacherRequest, "a", "broken", "panics", "b")

	if got := resetIDs(result); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("reset %v, want the two students who had no fault", got)
	}
	for _, id := range []string{"broken", "panics"} {
		if got := reasonFor(result, id); !errors.Is(got, domain.ErrResetFailed) {
			t.Errorf("%s failed with %v, want the reset-failed reason", id, got)
		}
	}
	logs := rig.logs.String()
	for _, want := range []string{"student_id=broken", "deadlock detected", "student_id=panics", "closed under panics"} {
		if !strings.Contains(logs, want) {
			t.Errorf("the log lacks %q:\n%s", want, logs)
		}
	}
	for _, r := range result.Reset {
		if strings.Contains(logs, r.TemporaryPassword) {
			t.Errorf("the log holds %s's temporary password:\n%s", r.StudentID, logs)
		}
	}
	if strings.Contains(logs, "secret-") {
		t.Errorf("the log holds a temporary password:\n%s", logs)
	}
}

func TestAPasswordThatCannotBeMadeFailsOnlyItsStudent(t *testing.T) {
	rig := newBulkRig("a", "b", "c", "d")
	rig.passwords.failOn = 2

	result := rig.reset(context.Background(), teacherRequest, "a", "b", "c", "d")

	if len(result.Reset) != 3 || len(result.Failed) != 1 || !errors.Is(result.Failed[0].Reason, domain.ErrResetFailed) {
		t.Fatalf("reset %v and failed %v, want three reset and one reset-failed", resetIDs(result), failureOf(result))
	}
	failed := result.Failed[0].StudentID
	if rig.shelf.reset(failed) {
		t.Errorf("%s was stored although its password could not be made", failed)
	}
	if !strings.Contains(rig.logs.String(), "student_id="+failed) || !strings.Contains(rig.logs.String(), "the entropy source was refused") {
		t.Errorf("the log does not name %s and the cause:\n%s", failed, rig.logs.String())
	}
	if got := rig.principal.forgottenIDs(); len(got) != 3 || slices.Contains(got, failed) {
		t.Errorf("forgot %v, want the three students who were reset and not %s", got, failed)
	}
}

func TestABulkResetHoldsAtMostTwoHashSlots(t *testing.T) {
	ids := make([]string, domain.MaxBulkReset)
	for i := range ids {
		ids[i] = fmt.Sprintf("s%02d", i)
	}
	rig := newBulkRig(ids...)
	rig.passwords.onCall = func(n int64) {
		if n <= 8 {
			rig.passwords.waitForInFlight(3, 60*time.Millisecond)
		}
	}

	result := rig.reset(context.Background(), teacherRequest, ids...)

	if len(result.Reset) != len(ids) {
		t.Fatalf("reset %d of %d students", len(result.Reset), len(ids))
	}
	if peak := rig.passwords.peak.Load(); peak != 2 {
		t.Errorf("%d passwords were made at once, want 2: the call may hold two of the process's hash slots and the workers must both use them", peak)
	}
}

func TestACancelledBulkResetStartsNoMoreStudentsAndKeepsWhatItDid(t *testing.T) {
	ids := make([]string, domain.MaxBulkReset)
	for i := range ids {
		ids[i] = fmt.Sprintf("s%02d", i)
	}
	rig := newBulkRig(ids...)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	rig.passwords.onCall = func(n int64) {
		if n == 6 {
			cancel()
		}
	}

	result := rig.reset(ctx, teacherRequest, ids...)

	if attempted := rig.passwords.calls.Load(); attempted < 6 || attempted > 6+2 {
		t.Errorf("%d students were started, want the six before the cancel and at most the two in flight beside it", attempted)
	}
	if got, want := int64(len(result.Reset)), rig.passwords.calls.Load(); got != want {
		t.Errorf("the answer holds %d resets for %d started students: what was committed must be reported", got, want)
	}
	if got := rig.shelf.committed(); !slices.Equal(got, slices.Sorted(slices.Values(resetIDs(result)))) {
		t.Errorf("committed %v, answered %v", got, resetIDs(result))
	}
	if got := rig.principal.forgottenIDs(); !slices.Equal(got, slices.Sorted(slices.Values(resetIDs(result)))) {
		t.Errorf("forgot %v, want every student who was reset", got)
	}
	if !strings.Contains(rig.logs.String(), "bulk password reset cut short") {
		t.Errorf("a cut-short reset was not logged:\n%s", rig.logs.String())
	}
}
