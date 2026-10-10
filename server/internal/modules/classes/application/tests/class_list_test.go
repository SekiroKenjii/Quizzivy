//go:build integration

package application_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/query"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/classes/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"quizzivy/internal/shared/stats"
)

var joinKeyC = bytes.Repeat([]byte{0xc3}, domain.JoinCodeKeySize)

func extraClass(t *testing.T, pool *pgxpool.Pool, teacherID, name string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	t.Cleanup(func() {
		if id == "" {
			return
		}
		for _, stmt := range []string{
			`DELETE FROM app.class_join_codes WHERE class_id = $1::uuid`,
			`DELETE FROM app.audit_log WHERE entity_id = $1::uuid`,
			`DELETE FROM app.classes WHERE id = $1::uuid`,
		} {
			if _, err := pool.Exec(ctx, stmt, id); err != nil {
				t.Errorf("cleanup %q: %v", stmt, err)
			}
		}
	})
	if err := pool.QueryRow(ctx, `INSERT INTO app.classes (name, teacher_id) VALUES ($1, $2) RETURNING id::text`, name, teacherID).Scan(&id); err != nil {
		t.Fatalf("insert class %s: %v", name, err)
	}
	return id
}

type codeWorld struct {
	pool    *pgxpool.Pool
	marker  string
	teacher string
	other   string
	reader  *application.Application
	logs    *bytes.Buffer
	classes map[string]string
}

func newCodeWorld(t *testing.T) *codeWorld {
	t.Helper()
	pool := newPool(t)
	w := &codeWorld{pool: pool, marker: "dm" + nonce(t), logs: &bytes.Buffer{}, classes: map[string]string{}}
	_, w.teacher, _ = makeClassRow(t, pool)
	_, w.other, _ = makeClassRow(t, pool)
	for _, name := range []string{"sealed", "legacy", "none", "lost", "tampered"} {
		w.classes[name] = extraClass(t, pool, w.teacher, w.marker+" "+name)
	}
	issueCode(t, withKeys(pool, mustJoinCodeKeys(joinKeyA, nil)), w.classes["sealed"], w.teacher)
	legacyCode(t, pool, w.classes["legacy"], w.teacher)
	issueCode(t, withKeys(pool, mustJoinCodeKeys(joinKeyC, nil)), w.classes["lost"], w.teacher)
	issueCode(t, withKeys(pool, mustJoinCodeKeys(joinKeyA, nil)), w.classes["tampered"], w.teacher)
	if _, err := pool.Exec(context.Background(), `
		UPDATE app.class_join_codes
		   SET code_ciphertext = set_byte(code_ciphertext, 20, get_byte(code_ciphertext, 20) # 1)
		 WHERE class_id = $1 AND revoked_at IS NULL`, w.classes["tampered"]); err != nil {
		t.Fatal(err)
	}
	w.reader = withKeys(pool, mustJoinCodeKeys(joinKeyB, joinKeyA))
	w.reader.WithLogger(slog.New(slog.NewTextHandler(w.logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return w
}

func (w *codeWorld) list(t *testing.T, scope access.Scope, openCodes bool) map[string]domain.ListedClass {
	t.Helper()
	result, err := w.reader.Queries.List.Handle(context.Background(), query.List{Input: domain.ListInput{Query: w.marker, Limit: 100, Status: "all", Scope: scope}, OpenCodes: openCodes})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	out := map[string]domain.ListedClass{}
	for _, item := range result.Items {
		out[item.ID] = item
	}
	return out
}

func TestTheListOpensACodeExactlyWhereReadingItByIdWould(t *testing.T) {
	w := newCodeWorld(t)
	callers := map[string]access.Scope{
		"the class's teacher": {UserID: w.teacher},
		"scope.all":           everyone,
		"another teacher":     {UserID: w.other},
		"no one":              {},
	}
	for who, scope := range callers {
		listed := w.list(t, scope, true)
		for label, classID := range w.classes {
			active, err := w.reader.Queries.ActiveCode.Handle(context.Background(), query.ActiveCode{Scope: scope, ClassID: classID})
			item, present := listed[classID]
			switch {
			case errors.Is(err, domain.ErrClassNotFound):
				if present {
					t.Errorf("%s: the list shows the %s class, which reading its code answers as missing", who, label)
				}
			case errors.Is(err, domain.ErrNoActiveCode):
				if !present || item.JoinCode != nil || item.Code != "" {
					t.Errorf("%s: the %s class has no code to read, and the list shows %+v", who, label, item.JoinCode)
				}
			case err == nil:
				if !present || item.JoinCode == nil || item.Code != active.Code || item.JoinCode.Legacy != active.Legacy || item.JoinCode.Hint != active.Hint {
					t.Errorf("%s: the %s class reads as code %q (legacy %v), and the list as %q with %+v", who, label, active.Code, active.Legacy, item.Code, item.JoinCode)
				}
			case label == "tampered":
				if !present || item.Code != "" {
					t.Errorf("%s: a code that does not authenticate must not take the page down, and it must show none: %+v", who, item)
				}
			default:
				t.Fatalf("%s: reading the %s class's code: %v", who, label, err)
			}
		}
	}

	owner := w.list(t, callers["the class's teacher"], true)
	if owner[w.classes["sealed"]].Code == "" || owner[w.classes["legacy"]].Code != "" || owner[w.classes["lost"]].Code != "" {
		t.Errorf("the teacher's list opens the sealed code (%q), and neither the legacy (%q) nor the lost (%q)",
			owner[w.classes["sealed"]].Code, owner[w.classes["legacy"]].Code, owner[w.classes["lost"]].Code)
	}
	if !owner[w.classes["legacy"]].JoinCode.Legacy || owner[w.classes["sealed"]].JoinCode.Legacy {
		t.Error("the legacy flag does not follow the stored scheme")
	}
	if logs := w.logs.String(); !strings.Contains(logs, w.classes["tampered"]) || strings.Contains(logs, owner[w.classes["sealed"]].Code) {
		t.Errorf("the tampered code must be logged by class, and no code may be logged:\n%s", logs)
	}
}

func TestTheListOpensNoCodeUnlessAskedAndShowsTheSameClasses(t *testing.T) {
	w := newCodeWorld(t)
	scope := access.Scope{UserID: w.teacher}
	asked, quiet := w.list(t, scope, true), w.list(t, scope, false)
	if len(asked) != len(quiet) || len(quiet) != len(w.classes) {
		t.Fatalf("asked lists %d classes and quiet %d, want %d each", len(asked), len(quiet), len(w.classes))
	}
	for id, item := range quiet {
		if item.Code != "" {
			t.Errorf("class %s opened a code when none was asked for", id)
		}
		if got := asked[id]; !reflect.DeepEqual(got.JoinCode, item.JoinCode) {
			t.Errorf("class %s: the code's metadata differs with the ask: %+v and %+v", id, got.JoinCode, item.JoinCode)
		}
	}
}

type scoreSource struct {
	scores map[string]stats.ClassScore
	err    error
	asked  [][]string
}

func (s *scoreSource) ClassScores(_ context.Context, ids []string) (map[string]stats.ClassScore, error) {
	s.asked = append(s.asked, ids)
	return s.scores, s.err
}

func TestEveryTeacherReadOfAClassCarriesItsAverage(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	_, teacherID, _ := makeClassRow(t, pool)
	graded := extraClass(t, pool, teacherID, "dmavg graded "+nonce(t))
	empty := extraClass(t, pool, teacherID, "dmavg empty "+nonce(t))
	source := &scoreSource{scores: map[string]stats.ClassScore{graded: {Earned: 19, Total: 30, PendingManual: 1}}}
	svc := application.New(repositories.NewPostgres(db.NewContext(pool)), nil, joinKeys).WithClassScores(source)
	scope := access.Scope{UserID: teacherID}
	want := &stats.ClassScore{Earned: 19, Total: 30, PendingManual: 1}
	same := func(label string, got *stats.ClassScore, expect *stats.ClassScore) {
		t.Helper()
		if (got == nil) != (expect == nil) || (got != nil && *got != *expect) {
			t.Errorf("%s: average %v, want %v", label, got, expect)
		}
	}

	got, err := svc.Queries.Get.Handle(ctx, query.Get{ClassID: graded, Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	same("get", got.AverageScore, want)
	got, err = svc.Queries.Get.Handle(ctx, query.Get{ClassID: empty, Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	same("get of an empty class", got.AverageScore, nil)

	listed, err := svc.Queries.List.Handle(ctx, query.List{Input: domain.ListInput{Query: "dmavg", Limit: 100, Scope: scope}})
	if err != nil || len(listed.Items) != 2 {
		t.Fatalf("list: %d items (%v)", len(listed.Items), err)
	}
	for _, item := range listed.Items {
		if item.ID == graded {
			same("list", item.AverageScore, want)
		} else {
			same("list of an empty class", item.AverageScore, nil)
		}
	}
	if last := source.asked[len(source.asked)-1]; len(last) != 2 {
		t.Errorf("a page of two classes asked for %d scores, want one read of both", len(last))
	}

	updated, err := svc.Commands.Update.Handle(ctx, command.Update{ClassID: graded, Input: domain.UpdateInput{Room: domain.TextPatch{Set: true, Value: text("B2")}}, Scope: scope})
	if err != nil {
		t.Fatal(err)
	}
	same("update", updated.AverageScore, want)
	archived, err := svc.Commands.Archive.Handle(ctx, command.Archive{ClassID: graded, Archived: true, Actor: actor.Actor{ID: teacherID, Scope: scope}})
	if err != nil {
		t.Fatal(err)
	}
	same("archive", archived.AverageScore, want)

	if _, err := newSvc(t, pool).Queries.Get.Handle(ctx, query.Get{ClassID: graded, Scope: scope}); err != nil {
		t.Errorf("without a score source a read fails: %v", err)
	}
	source.err = errors.New("the scores cannot be read")
	if _, err := svc.Queries.Get.Handle(ctx, query.Get{ClassID: graded, Scope: scope}); err == nil {
		t.Error("a score source that fails must fail the read, not show a class with no average")
	}
}
