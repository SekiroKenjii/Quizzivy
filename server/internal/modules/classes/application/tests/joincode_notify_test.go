//go:build integration

package application_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/classes/application"
	"quizzivy/internal/modules/classes/application/command"
	"quizzivy/internal/modules/classes/application/ports"
	"quizzivy/internal/modules/classes/domain"
	"quizzivy/internal/modules/classes/repositories"
	notificationsapp "quizzivy/internal/modules/notifications/application"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	notificationsrepo "quizzivy/internal/modules/notifications/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/cqrs"
)

type joinNotices struct {
	mu   sync.Mutex
	sent []notificationscommand.Notify
	err  error
}

func (j *joinNotices) Handle(_ context.Context, n notificationscommand.Notify) (cqrs.Nothing, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.err != nil {
		return cqrs.Nothing{}, j.err
	}
	j.sent = append(j.sent, n)
	return cqrs.Nothing{}, nil
}

func (j *joinNotices) count() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.sent)
}

func notifyingOver(pool *pgxpool.Pool, notifier ports.Notifier, log *bytes.Buffer) *application.Application {
	logger := slog.New(slog.DiscardHandler)
	if log != nil {
		logger = slog.New(slog.NewTextHandler(log, nil))
	}
	return application.New(repositories.NewPostgres(db.NewContext(pool)), nil, joinKeys).WithNotifier(notifier).WithLogger(logger)
}

func outsider(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	t.Cleanup(func() {
		if id == "" {
			return
		}
		for _, q := range []string{
			`DELETE FROM app.audit_log WHERE actor_user_id = $1::uuid`,
			`DELETE FROM app.class_members WHERE user_id = $1::uuid`,
			`DELETE FROM app.users WHERE id = $1::uuid`,
		} {
			if _, err := pool.Exec(context.Background(), q, id); err != nil {
				t.Errorf("cleanup %q: %v", q, err)
			}
		}
	})
	if err := pool.QueryRow(context.Background(), `INSERT INTO app.users (email, full_name, role_id)
		VALUES ($1, 'Học viên mới', (SELECT id FROM app.roles WHERE builtin_key = 'student')) RETURNING id::text`,
		"outsider-"+nonce(t)+"@example.com").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func className(t *testing.T, pool *pgxpool.Pool, classID string) string {
	t.Helper()
	var name string
	if err := pool.QueryRow(context.Background(), `SELECT name FROM app.classes WHERE id = $1::uuid`, classID).Scan(&name); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestAStudentWhoSignsUpWithACodeTellsTheClassTeacher(t *testing.T) {
	pool := newPool(t)
	notices := &joinNotices{}
	svc := notifyingOver(pool, notices, nil)
	classID, teacherID, _ := makeClassRow(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	m := newMember(t)
	dropUser(t, pool, m.Email)

	result, err := svc.Commands.EnrolNewMember.Handle(context.Background(), command.EnrolNewMember{Member: m, Code: code})
	if err != nil || result.Outcome != domain.PreviewOK {
		t.Fatalf("enrol: %+v, %v", result, err)
	}
	if notices.count() != 1 {
		t.Fatalf("%d notices, want one for the teacher: %+v", notices.count(), notices.sent)
	}
	n := notices.sent[0]
	if n.UserID != teacherID || n.Kind != notificationsdomain.ClassJoined {
		t.Errorf("notice %+v, want class.joined for the class teacher %s", n, teacherID)
	}
	if want := (notificationsdomain.Joined{StudentName: "Trần Thị B", ClassName: className(t, pool, classID)}); n.Params != want {
		t.Errorf("params %+v, want %+v", n.Params, want)
	}
	if *n.Target != (notificationsdomain.Target{Route: notificationsdomain.RouteClasses}) {
		t.Errorf("target %+v, want the classes page", *n.Target)
	}
	if n.DedupeKey != "joined:"+classID+":"+result.UserID || n.Merge != notificationsdomain.Replace {
		t.Errorf("key %q, merge %d", n.DedupeKey, n.Merge)
	}
}

func TestASignedInStudentWhoJoinsWithACodeTellsTheClassTeacher(t *testing.T) {
	pool := newPool(t)
	notices := &joinNotices{}
	svc := notifyingOver(pool, notices, nil)
	classID, teacherID, _ := makeClassRow(t, pool)
	studentID := outsider(t, pool)
	code := issueCode(t, svc, classID, teacherID)

	result, err := svc.Commands.EnrolExisting.Handle(context.Background(), command.EnrolExisting{UserID: studentID, Code: code})
	if err != nil || result.Outcome != domain.PreviewOK || result.AlreadyMember {
		t.Fatalf("enrol: %+v, %v", result, err)
	}
	if notices.count() != 1 || notices.sent[0].UserID != teacherID || notices.sent[0].Params.(notificationsdomain.Joined).StudentName != "Học viên mới" {
		t.Errorf("notices %+v, want the teacher told that Học viên mới joined", notices.sent)
	}
}

func TestJoiningAClassYouAreAlreadyInTellsNobody(t *testing.T) {
	pool := newPool(t)
	notices := &joinNotices{}
	svc := notifyingOver(pool, notices, nil)
	classID, teacherID, _ := makeClassRow(t, pool)
	studentID := outsider(t, pool)
	code := issueCode(t, svc, classID, teacherID)

	enrol := func() domain.EnrolResult {
		result, err := svc.Commands.EnrolExisting.Handle(context.Background(), command.EnrolExisting{UserID: studentID, Code: code})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	enrol()
	if again := enrol(); !again.AlreadyMember {
		t.Fatalf("the second join was not recognised as a repeat: %+v", again)
	}
	if notices.count() != 1 {
		t.Errorf("%d notices after joining twice, want only the first join's", notices.count())
	}
}

func TestACodeThatIsNotAcceptedTellsNobody(t *testing.T) {
	pool := newPool(t)
	notices := &joinNotices{}
	svc := notifyingOver(pool, notices, nil)
	makeClassRow(t, pool)
	studentID := outsider(t, pool)

	result, err := svc.Commands.EnrolExisting.Handle(context.Background(), command.EnrolExisting{UserID: studentID, Code: "KHONG-CO-MA-NAY"})
	if err != nil || result.Outcome == domain.PreviewOK {
		t.Fatalf("a code nobody holds: %+v, %v", result, err)
	}
	if notices.count() != 0 {
		t.Errorf("a refused code told %+v", notices.sent)
	}
}

func TestAFailingNotifierNeverFailsAJoin(t *testing.T) {
	pool := newPool(t)
	notices := &joinNotices{err: errors.New("the notifications store is away")}
	var log bytes.Buffer
	svc := notifyingOver(pool, notices, &log)
	classID, teacherID, _ := makeClassRow(t, pool)
	studentID := outsider(t, pool)
	code := issueCode(t, svc, classID, teacherID)

	result, err := svc.Commands.EnrolExisting.Handle(context.Background(), command.EnrolExisting{UserID: studentID, Code: code})
	if err != nil || result.Outcome != domain.PreviewOK {
		t.Fatalf("a join with the notifier down: %+v, %v", result, err)
	}
	if memberCount(t, pool, classID) != 2 {
		t.Error("the student was not enrolled")
	}
	if !strings.Contains(log.String(), "announcement not delivered") || !strings.Contains(log.String(), "the notifications store is away") {
		t.Errorf("the failure was logged as %q", log.String())
	}
}

func TestAJoinReachesATeacherWhoSwitchedEveryEventOff(t *testing.T) {
	pool := newPool(t)
	notifications := notificationsapp.New(notificationsrepo.NewPostgres(db.NewContext(pool)))
	svc := notifyingOver(pool, notifications.Commands.Notify, nil)
	classID, teacherID, _ := makeClassRow(t, pool)
	studentID := outsider(t, pool)
	code := issueCode(t, svc, classID, teacherID)
	for _, event := range notificationsdomain.Events() {
		if _, err := pool.Exec(context.Background(), `INSERT INTO app.notification_preferences (user_id, event, in_app) VALUES ($1::uuid, $2, false)`, teacherID, string(event)); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := svc.Commands.EnrolExisting.Handle(context.Background(), command.EnrolExisting{UserID: studentID, Code: code}); err != nil {
		t.Fatal(err)
	}
	var held int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM app.notifications WHERE user_id = $1::uuid AND kind = 'class.joined'`, teacherID).Scan(&held); err != nil || held != 1 {
		t.Errorf("the teacher holds %d join notices, want 1: a join has no switch: %v", held, err)
	}
}
