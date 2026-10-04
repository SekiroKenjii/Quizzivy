package application_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/cqrs"
)

const (
	teacherLan = "0195b000-0000-7000-8000-0000000000a1"
	teacherMai = "0195b000-0000-7000-8000-0000000000a2"
)

var sealingKeys = func() domain.JoinCodeKeys {
	keys, err := domain.NewJoinCodeKeys(bytes.Repeat([]byte{0xc3}, domain.JoinCodeKeySize), nil)
	if err != nil {
		panic(err)
	}
	return keys
}()

var rotationInstant = time.Date(2026, 10, 12, 2, 30, 0, 0, time.UTC)

func classID(n int) string {
	return fmt.Sprintf("0195a000-0000-7000-8000-0000000000%02x", n)
}

func legacyClass(n int, name, teacher string) domain.LegacyCodeClass {
	c := domain.LegacyCodeClass{ClassID: classID(n), ClassName: name}
	if teacher != "" {
		c.TeacherID = &teacher
	}
	return c
}

type rotationOutcome struct {
	wrote bool
	err   error
}

type rotationStore struct {
	domain.Repository
	classes  []domain.LegacyCodeClass
	listErr  error
	listedAt time.Time
	outcomes map[string][]rotationOutcome
	inputs   []domain.LegacyRotationInput
	events   *[]string
}

func (s *rotationStore) LegacyCodeClasses(_ context.Context, now time.Time) ([]domain.LegacyCodeClass, error) {
	s.listedAt = now
	*s.events = append(*s.events, "list")
	return s.classes, s.listErr
}

func (s *rotationStore) RotateLegacyCode(_ context.Context, in domain.LegacyRotationInput) (bool, error) {
	s.inputs = append(s.inputs, in)
	*s.events = append(*s.events, "rotate "+in.ClassID)
	queued := s.outcomes[in.ClassID]
	if len(queued) == 0 {
		return true, nil
	}
	s.outcomes[in.ClassID] = queued[1:]
	return queued[0].wrote, queued[0].err
}

func (s *rotationStore) tries(class string) int {
	n := 0
	for _, in := range s.inputs {
		if in.ClassID == class {
			n++
		}
	}
	return n
}

type rotationWorld struct {
	store    *rotationStore
	notices  []notificationscommand.Notify
	refuse   map[string]error
	events   []string
	problems []error
	app      *application.Application
}

func rotating(classes ...domain.LegacyCodeClass) *rotationWorld {
	w := &rotationWorld{refuse: map[string]error{}}
	w.store = &rotationStore{classes: classes, outcomes: map[string][]rotationOutcome{}, events: &w.events}
	w.app = application.New(w.store, nil, sealingKeys).WithNotifier(
		cqrs.HandlerFunc[notificationscommand.Notify, cqrs.Nothing](func(_ context.Context, n notificationscommand.Notify) (cqrs.Nothing, error) {
			w.events = append(w.events, "notify "+n.UserID)
			if err := w.refuse[n.UserID]; err != nil {
				return cqrs.Nothing{}, err
			}
			w.notices = append(w.notices, n)
			return cqrs.Nothing{}, nil
		}))
	w.app.SetClock(func() time.Time { return rotationInstant })
	return w
}

func (w *rotationWorld) run(t *testing.T) domain.LegacyRotation {
	t.Helper()
	got, err := w.app.Commands.RotateLegacyJoinCodes.Handle(context.Background(), command.RotateLegacyJoinCodes{
		Found:   func(count int) { w.events = append(w.events, fmt.Sprintf("found %d", count)) },
		Problem: func(err error) { w.problems = append(w.problems, err) },
	})
	if err != nil {
		t.Fatalf("the run failed: %v", err)
	}
	return got
}

func (w *rotationWorld) noticeFor(t *testing.T, teacher string) notificationscommand.Notify {
	t.Helper()
	var found []notificationscommand.Notify
	for _, n := range w.notices {
		if n.UserID == teacher {
			found = append(found, n)
		}
	}
	if len(found) != 1 {
		t.Fatalf("teacher %s was told %d times, want exactly once", teacher, len(found))
	}
	return found[0]
}

func rotatedParams(t *testing.T, n notificationscommand.Notify) notificationsdomain.CodesRotated {
	t.Helper()
	params, ok := n.Params.(notificationsdomain.CodesRotated)
	if !ok {
		t.Fatalf("the params are %T, want CodesRotated", n.Params)
	}
	return params
}

func TestTheLegacyRotationTakesClassesTeacherByTeacherAndTellsEachOnce(t *testing.T) {
	w := rotating(
		legacyClass(1, "Lớp 10A1", teacherLan),
		legacyClass(2, "Lớp 10A2", teacherLan),
		legacyClass(3, "Lớp 11B", teacherMai),
	)
	got := w.run(t)

	if want := (domain.LegacyRotation{Found: 3, Rotated: 3, Teachers: 2}); got != want {
		t.Errorf("the run answered %+v, want %+v", got, want)
	}
	wantEvents := []string{
		"list", "found 3",
		"rotate " + classID(1), "rotate " + classID(2), "notify " + teacherLan,
		"rotate " + classID(3), "notify " + teacherMai,
	}
	if !slices.Equal(w.events, wantEvents) {
		t.Errorf("the run went\n%v\nwant\n%v", w.events, wantEvents)
	}
	if !w.store.listedAt.Equal(rotationInstant) {
		t.Errorf("the classes were listed as of %s, want the clock's %s", w.store.listedAt, rotationInstant)
	}

	lan := w.noticeFor(t, teacherLan)
	if lan.Kind != notificationsdomain.JoinCodesRotated {
		t.Errorf("kind = %q, want join_codes.rotated", lan.Kind)
	}
	if params := rotatedParams(t, lan); params.Count != 2 || !slices.Equal(params.ClassNames, []string{"Lớp 10A1", "Lớp 10A2"}) {
		t.Errorf("params = %+v, want 2 classes named in order", params)
	}
	if lan.Target == nil || *lan.Target != (notificationsdomain.Target{Route: notificationsdomain.RouteClasses}) {
		t.Errorf("target = %+v, want the classes route and nothing else", lan.Target)
	}
	if lan.DedupeKey != "join_codes.rotated:legacy" {
		t.Errorf("dedupe key = %q, want join_codes.rotated:legacy", lan.DedupeKey)
	}
	if lan.Merge != notificationsdomain.Add {
		t.Errorf("merge = %d, want Add, so that two runs sum", lan.Merge)
	}
	if err := (notificationsdomain.Notice{UserID: lan.UserID, Kind: lan.Kind, Params: lan.Params, Target: lan.Target, DedupeKey: lan.DedupeKey, Merge: lan.Merge}).Validate(); err != nil {
		t.Errorf("the notifications module would refuse the notice: %v", err)
	}
	if params := rotatedParams(t, w.noticeFor(t, teacherMai)); params.Count != 1 || !slices.Equal(params.ClassNames, []string{"Lớp 11B"}) {
		t.Errorf("the second teacher's params = %+v", params)
	}
	if len(w.problems) != 0 {
		t.Errorf("a clean run reported %v", w.problems)
	}
}

func TestATeachersClassesAreToldTogetherHoweverTheListIsOrdered(t *testing.T) {
	w := rotating(
		legacyClass(1, "Lớp 10A1", teacherLan),
		legacyClass(3, "Lớp 11B", teacherMai),
		legacyClass(2, "Lớp 10A2", teacherLan),
	)
	got := w.run(t)

	if got.Teachers != 2 || len(w.notices) != 2 {
		t.Fatalf("%d notices for two teachers (%+v)", len(w.notices), got)
	}
	if params := rotatedParams(t, w.noticeFor(t, teacherLan)); params.Count != 2 || !slices.Equal(params.ClassNames, []string{"Lớp 10A1", "Lớp 10A2"}) {
		t.Errorf("params = %+v, want both classes in one notice", params)
	}
}

func TestEightRotatedClassesAreCountedAndTheFirstFiveNamed(t *testing.T) {
	var classes []domain.LegacyCodeClass
	var names []string
	for i := range 8 {
		name := fmt.Sprintf("Lớp %c", 'A'+i)
		classes = append(classes, legacyClass(i+1, name, teacherLan))
		names = append(names, name)
	}
	w := rotating(classes...)
	got := w.run(t)

	if got.Rotated != 8 || got.Teachers != 1 {
		t.Errorf("the run answered %+v, want 8 rotated and one teacher told", got)
	}
	notice := w.noticeFor(t, teacherLan)
	if params := rotatedParams(t, notice); params.Count != 8 || !slices.Equal(params.ClassNames, names[:5]) {
		t.Errorf("params = %+v, want a count of 8 and the first five names", params)
	}
	if _, err := notificationsdomain.Encode(notice.Params); err != nil {
		t.Errorf("the params are outside the contract's bounds: %v", err)
	}
}

func TestAClassAnotherRunRotatedIsNeitherCountedNorNamed(t *testing.T) {
	w := rotating(
		legacyClass(1, "Lớp 10A1", teacherLan),
		legacyClass(2, "Lớp 10A2", teacherLan),
		legacyClass(3, "Lớp 10A3", teacherLan),
		legacyClass(4, "Lớp 11B", teacherMai),
	)
	w.store.outcomes[classID(2)] = []rotationOutcome{{wrote: false}}
	w.store.outcomes[classID(4)] = []rotationOutcome{{wrote: false}}
	got := w.run(t)

	if want := (domain.LegacyRotation{Found: 4, Rotated: 2, Teachers: 1}); got != want {
		t.Errorf("the run answered %+v, want %+v", got, want)
	}
	if params := rotatedParams(t, w.noticeFor(t, teacherLan)); params.Count != 2 || !slices.Equal(params.ClassNames, []string{"Lớp 10A1", "Lớp 10A3"}) {
		t.Errorf("params = %+v, want only the two classes this run rotated", params)
	}
	if slices.Contains(w.events, "notify "+teacherMai) {
		t.Error("a teacher none of whose classes this run rotated was told")
	}
	if w.store.tries(classID(2)) != 1 {
		t.Errorf("a class another run rotated was tried %d times, want once", w.store.tries(classID(2)))
	}
}

func TestAFailingClassIsNotRetriedAndDoesNotStopTheRun(t *testing.T) {
	away := errors.New("the database is away")
	w := rotating(
		legacyClass(1, "Lớp 10A1", teacherLan),
		legacyClass(2, "Lớp 10A2", teacherLan),
		legacyClass(3, "Lớp 11B", teacherMai),
	)
	w.store.outcomes[classID(1)] = []rotationOutcome{{err: away}}
	got := w.run(t)

	if want := (domain.LegacyRotation{Found: 3, Rotated: 2, Failed: 1, Teachers: 2}); got != want {
		t.Errorf("the run answered %+v, want %+v", got, want)
	}
	if n := w.store.tries(classID(1)); n != 1 {
		t.Errorf("a class that failed for another reason was tried %d times, want once", n)
	}
	if params := rotatedParams(t, w.noticeFor(t, teacherLan)); params.Count != 1 || !slices.Equal(params.ClassNames, []string{"Lớp 10A2"}) {
		t.Errorf("params = %+v, want only the class that was rotated", params)
	}
	if len(w.problems) != 1 || !errors.Is(w.problems[0], away) || !strings.Contains(w.problems[0].Error(), classID(1)) {
		t.Errorf("the problems are %v, want the failure, naming class %s", w.problems, classID(1))
	}
}

func TestAContendedClassIsRetriedOnceAfterEveryOtherClass(t *testing.T) {
	w := rotating(
		legacyClass(1, "Lớp 10A1", teacherLan),
		legacyClass(2, "Lớp 10A2", teacherLan),
		legacyClass(3, "Lớp 11B", teacherMai),
	)
	w.store.outcomes[classID(1)] = []rotationOutcome{{err: fmt.Errorf("%w: deadlock detected", domain.ErrRotationContended)}}
	got := w.run(t)

	if want := (domain.LegacyRotation{Found: 3, Rotated: 3, Teachers: 2}); got != want {
		t.Errorf("the run answered %+v, want %+v", got, want)
	}
	wantEvents := []string{
		"list", "found 3",
		"rotate " + classID(1), "rotate " + classID(2),
		"rotate " + classID(3), "notify " + teacherMai,
		"rotate " + classID(1), "notify " + teacherLan,
	}
	if !slices.Equal(w.events, wantEvents) {
		t.Errorf("the run went\n%v\nwant\n%v", w.events, wantEvents)
	}
	if params := rotatedParams(t, w.noticeFor(t, teacherLan)); params.Count != 2 || !slices.Equal(params.ClassNames, []string{"Lớp 10A1", "Lớp 10A2"}) {
		t.Errorf("params = %+v, want the retried class counted and named in its place", params)
	}
	if len(w.problems) != 0 {
		t.Errorf("a class rotated on its retry was reported: %v", w.problems)
	}
}

func TestAClassContendedTwiceIsTriedExactlyTwiceAndFails(t *testing.T) {
	contended := fmt.Errorf("%w: deadlock detected", domain.ErrRotationContended)
	w := rotating(legacyClass(1, "Lớp 10A1", teacherLan), legacyClass(2, "Lớp 11B", teacherMai))
	w.store.outcomes[classID(1)] = []rotationOutcome{{err: contended}, {err: contended}, {err: contended}, {err: contended}}
	got := w.run(t)

	if want := (domain.LegacyRotation{Found: 2, Rotated: 1, Failed: 1, Teachers: 1}); got != want {
		t.Errorf("the run answered %+v, want %+v", got, want)
	}
	if n := w.store.tries(classID(1)); n != 2 {
		t.Errorf("the contended class was tried %d times, want exactly 2", n)
	}
	if slices.Contains(w.events, "notify "+teacherLan) {
		t.Error("a teacher whose only class is still unrotated was told")
	}
	if len(w.problems) != 1 || !errors.Is(w.problems[0], domain.ErrRotationContended) {
		t.Errorf("the problems are %v, want the second refusal and nothing else", w.problems)
	}
}

func TestAFailingNotifierIsCountedAndTheRotationStands(t *testing.T) {
	full := errors.New("the notifications table is away")
	w := rotating(legacyClass(1, "Lớp 10A1", teacherLan), legacyClass(2, "Lớp 11B", teacherMai))
	w.refuse[teacherLan] = full
	got := w.run(t)

	if want := (domain.LegacyRotation{Found: 2, Rotated: 2, Teachers: 1, NotifyFailed: 1}); got != want {
		t.Errorf("the run answered %+v, want %+v", got, want)
	}
	if !slices.Contains(w.events, "rotate "+classID(2)) || !slices.Contains(w.events, "notify "+teacherMai) {
		t.Errorf("the run stopped at the failed notification: %v", w.events)
	}
	if len(w.problems) != 1 || !errors.Is(w.problems[0], full) || !strings.Contains(w.problems[0].Error(), teacherLan) {
		t.Errorf("the problems are %v, want the failed notification, naming teacher %s", w.problems, teacherLan)
	}
}

func TestWithoutANotifierTheCommandRotatesAndTellsNobody(t *testing.T) {
	for name, build := range map[string]func(domain.Repository) *application.Application{
		"never given one": func(repo domain.Repository) *application.Application {
			return application.New(repo, nil, sealingKeys)
		},
		"given none": func(repo domain.Repository) *application.Application {
			return application.New(repo, nil, sealingKeys).WithNotifier(nil)
		},
	} {
		t.Run(name, func(t *testing.T) {
			var events []string
			store := &rotationStore{classes: []domain.LegacyCodeClass{legacyClass(1, "Lớp 10A1", teacherLan)}, events: &events}
			got, err := build(store).Commands.RotateLegacyJoinCodes.Handle(context.Background(), command.RotateLegacyJoinCodes{})
			if err != nil {
				t.Fatal(err)
			}
			if want := (domain.LegacyRotation{Found: 1, Rotated: 1}); got != want {
				t.Errorf("the run answered %+v, want %+v", got, want)
			}
		})
	}
}

func TestAClassWithoutATeacherIsRotatedAndNobodyIsTold(t *testing.T) {
	w := rotating(legacyClass(1, "Lớp mồ côi", ""), legacyClass(2, "Lớp mồ côi 2", ""), legacyClass(3, "Lớp 11B", teacherMai))
	got := w.run(t)

	if want := (domain.LegacyRotation{Found: 3, Rotated: 3, Teachers: 1}); got != want {
		t.Errorf("the run answered %+v, want %+v", got, want)
	}
	if len(w.notices) != 1 || w.notices[0].UserID != teacherMai {
		t.Errorf("the notices went to %+v, want only the one class's teacher", w.notices)
	}
}

func TestEveryGeneratedCodeIsSealedForTheClassAndCodeIdItIsStoredUnder(t *testing.T) {
	w := rotating(legacyClass(1, "Lớp 10A1", teacherLan), legacyClass(2, "Lớp 10A2", teacherLan))
	w.store.outcomes[classID(1)] = []rotationOutcome{{err: fmt.Errorf("%w: deadlock detected", domain.ErrRotationContended)}}
	w.run(t)

	if len(w.store.inputs) != 3 {
		t.Fatalf("the store was asked %d times, want 3: two classes and one retry", len(w.store.inputs))
	}
	codes, ids := map[string]bool{}, map[string]bool{}
	for _, in := range w.store.inputs {
		id, err := uuid.Parse(in.CodeID)
		if err != nil || id.Version() != 7 {
			t.Fatalf("code id %q is not a uuidv7 (%v)", in.CodeID, err)
		}
		if in.KeyID != sealingKeys.CurrentID() {
			t.Errorf("key id = %d, want the current key's %d", in.KeyID, sealingKeys.CurrentID())
		}
		code, err := sealingKeys.Open(in.ClassID, in.CodeID, in.KeyID, in.Ciphertext)
		if err != nil {
			t.Fatalf("the ciphertext does not open under its own class and code ids: %v", err)
		}
		if len(code) != domain.Length || domain.JoinCodes.Normalize(code) != code {
			t.Errorf("the sealed code %q is not a canonical join code", code)
		}
		if in.Hint != domain.JoinCodes.Hint(code) {
			t.Errorf("hint = %q, want the code's last four", in.Hint)
		}
		if !bytes.Equal(in.CodeHash, sealingKeys.Hash(code)) || bytes.Equal(in.CodeHash, domain.JoinCodes.Hash(code)) {
			t.Error("the lookup hash is not the keyed hash of the sealed code")
		}
		if !in.Now.Equal(rotationInstant) {
			t.Errorf("now = %s, want the clock's %s", in.Now, rotationInstant)
		}
		codes[code], ids[in.CodeID] = true, true
	}
	if len(codes) != 3 || len(ids) != 3 {
		t.Errorf("%d codes under %d ids for three attempts: every attempt, a retry included, takes a new code and a new id", len(codes), len(ids))
	}
}

func TestAProblemNamesTheClassAndNeverItsCodeOrHint(t *testing.T) {
	w := rotating(legacyClass(1, "Lớp 10A1", teacherLan))
	w.store.outcomes[classID(1)] = []rotationOutcome{{err: errors.New("the database is away")}}
	w.run(t)

	if len(w.problems) != 1 || len(w.store.inputs) != 1 {
		t.Fatalf("%d problems after %d attempts, want one of each", len(w.problems), len(w.store.inputs))
	}
	in := w.store.inputs[0]
	code, err := sealingKeys.Open(in.ClassID, in.CodeID, in.KeyID, in.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	said := w.problems[0].Error()
	if !strings.Contains(said, in.ClassID) {
		t.Errorf("the problem %q does not name the class", said)
	}
	for _, secret := range []string{code, domain.JoinCodes.Format(code), in.Hint, code[:4]} {
		if strings.Contains(said, secret) {
			t.Errorf("the problem %q carries %q of the code", said, secret)
		}
	}
}

func TestAListingFailureEndsTheRunBeforeAnythingIsWritten(t *testing.T) {
	away := errors.New("the database is away")
	w := rotating(legacyClass(1, "Lớp 10A1", teacherLan))
	w.store.listErr = away
	got, err := w.app.Commands.RotateLegacyJoinCodes.Handle(context.Background(), command.RotateLegacyJoinCodes{
		Found: func(int) { t.Error("a run that could not list announced a count") },
	})
	if !errors.Is(err, away) {
		t.Errorf("err = %v, want the listing's", err)
	}
	if !reflect.DeepEqual(got, domain.LegacyRotation{}) || len(w.store.inputs) != 0 || len(w.notices) != 0 {
		t.Errorf("the run answered %+v after %d rotations and %d notices, want nothing", got, len(w.store.inputs), len(w.notices))
	}
}

func TestARunThatFindsNothingStillAnnouncesItsCount(t *testing.T) {
	w := rotating()
	got := w.run(t)

	if got != (domain.LegacyRotation{}) {
		t.Errorf("the run answered %+v, want zeroes", got)
	}
	if !slices.Equal(w.events, []string{"list", "found 0"}) {
		t.Errorf("the run went %v, want the listing and its count", w.events)
	}
}
