package application_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	fail   map[string]error
	panics map[string]bool
	delay  map[string]time.Duration
	done   []string
	hashes map[string]string
}

func newBulkShelf(ids ...string) *bulkShelf {
	shelf := &bulkShelf{
		known:  map[string]domain.Student{},
		fail:   map[string]error{},
		panics: map[string]bool{},
		delay:  map[string]time.Duration{},
		hashes: map[string]string{},
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

func (b *bulkShelf) ResetPassword(_ context.Context, _ domain.WriteRequest, id, hash string, _ time.Time) error {
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
	return nil
}

func (b *bulkShelf) reset(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.hashes[id]
	return ok
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
