//go:build integration

package application_test

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/attempts/application/command"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
)

func submittedBucket(t *testing.T, key, assignment string) int64 {
	t.Helper()
	prefix := "submitted:" + assignment + ":"
	if !strings.HasPrefix(key, prefix) {
		t.Fatalf("dedupe key %q, want %s{15-minute bucket}", key, prefix)
	}
	bucket, err := strconv.ParseInt(strings.TrimPrefix(key, prefix), 10, 64)
	if err != nil {
		t.Fatalf("dedupe key %q: %v", key, err)
	}
	return bucket
}

func TestHandingInAPaperTellsTheTeachersWhoReachItAndNoOtherTeacher(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	second := addSecondClass(t, pool, w)
	tell := &told{}
	svc := announcing(pool, tell, nil)
	title := titleOf(t, pool, w)

	hand(t, svc, startFor(t, svc, w, w.student), w.student)
	sent := tell.of(notificationsdomain.AttemptSubmitted)
	if len(sent) != 1 || sent[0].UserID != w.admin {
		t.Fatalf("a student of the first class handed in; the notices went to %+v, want the creator and class teacher once", sent)
	}
	n := sent[0]
	if want := (notificationsdomain.Submitted{Title: title, Count: 1, ToGrade: 0}); n.Params != want {
		t.Errorf("params %+v, want %+v", n.Params, want)
	}
	if want := (notificationsdomain.Target{Route: notificationsdomain.RouteAssignment, AssignmentID: w.assignment}); *n.Target != want {
		t.Errorf("target %+v, want %+v", *n.Target, want)
	}
	if n.Merge != notificationsdomain.Add {
		t.Errorf("merge %d, want Add: papers in a bucket count together", n.Merge)
	}
	if bucket, now := submittedBucket(t, n.DedupeKey, w.assignment), time.Now().Unix()/900; bucket < now-1 || bucket > now {
		t.Errorf("bucket %d, want the current fifteen minutes %d", bucket, now)
	}

	hand(t, svc, startFor(t, svc, w, second.student), second.student)
	got := tell.users(notificationsdomain.AttemptSubmitted)
	if len(tell.of(notificationsdomain.AttemptSubmitted)) != 3 || !got[w.admin] || !got[second.teacher] {
		t.Errorf("after a student of the second class handed in: %+v, want the creator and the second teacher told as well", tell.of(notificationsdomain.AttemptSubmitted))
	}
	for _, n := range tell.of(notificationsdomain.AttemptSubmitted)[1:] {
		if n.UserID != w.admin && n.UserID != second.teacher {
			t.Errorf("a notice went to %s", n.UserID)
		}
	}
}

func TestAPaperWithAnAnswerToMarkCountsAsWaiting(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	tell := &told{}
	svc := announcing(pool, tell, nil)
	session := startFor(t, svc, w, w.student)
	save := domain.SaveInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
		Answers: []domain.Answer{{QuestionID: w.essay, Payload: []byte(`{"type":"text","value":"Tôi dậy lúc 6 giờ."}`)}},
	}
	if _, err := svc.Commands.Save.Handle(context.Background(), command.Save{Input: save}); err != nil {
		t.Fatal(err)
	}
	hand(t, svc, session, w.student)
	sent := tell.of(notificationsdomain.AttemptSubmitted)
	if len(sent) != 1 || sent[0].Params.(notificationsdomain.Submitted).ToGrade != 1 {
		t.Errorf("a paper with an essay produced %+v, want toGrade 1", sent)
	}
}

func TestAPaperAnsweredByMachineAloneIsNotWaiting(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	tell := &told{}
	svc := announcing(pool, tell, nil)
	session := startFor(t, svc, w, w.student)
	save := domain.SaveInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
		Answers: []domain.Answer{{QuestionID: w.choice, Payload: []byte(`{"type":"choice","optionIds":["` + correctOption(t, pool, w.choice) + `"]}`)}},
	}
	if _, err := svc.Commands.Save.Handle(context.Background(), command.Save{Input: save}); err != nil {
		t.Fatal(err)
	}
	hand(t, svc, session, w.student)
	sent := tell.of(notificationsdomain.AttemptSubmitted)
	if len(sent) != 1 || sent[0].Params.(notificationsdomain.Submitted).ToGrade != 0 {
		t.Errorf("a paper of choices produced %+v, want toGrade 0", sent)
	}
}

func TestPapersHandedInTogetherAreOneNotificationThatIsUnreadAgain(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	second := addSecondClass(t, pool, w)
	notifications := realNotifications(pool)
	svc := announcing(pool, notifications.Commands.Notify, nil)

	hand(t, svc, startFor(t, svc, w, w.student), w.student)
	var id string
	if err := pool.QueryRow(t.Context(), `SELECT id::text FROM app.notifications WHERE user_id = $1::uuid AND kind = 'attempt.submitted'`, w.admin).Scan(&id); err != nil {
		t.Fatalf("the teacher holds no submission notice: %v", err)
	}
	if _, err := notifications.Commands.MarkRead.Handle(t.Context(), notificationscommand.MarkRead{UserID: w.admin, IDs: []string{id}}); err != nil {
		t.Fatal(err)
	}

	hand(t, svc, startFor(t, svc, w, second.student), second.student)
	var count int
	var unread bool
	var rows int
	if err := pool.QueryRow(t.Context(), `
		SELECT count(*), max((params->>'count')::int), bool_and(read_at IS NULL)
		  FROM app.notifications WHERE user_id = $1::uuid AND kind = 'attempt.submitted'`, w.admin).Scan(&rows, &count, &unread); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || count != 2 || !unread {
		t.Errorf("the teacher holds %d rows counting %d, unread = %v; want one row counting 2, unread again", rows, count, unread)
	}
	var other int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM app.notifications WHERE user_id = $1::uuid`, second.teacher).Scan(&other); err != nil || other != 1 {
		t.Errorf("the second class's teacher holds %d notices, want the one for their own student: %v", other, err)
	}
}

func TestAnAttemptThatRanOutOfTimeIsHandedInOnceHoweverOftenItIsRead(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	tell := &told{}
	svc := announcing(pool, tell, nil)
	session := startFor(t, svc, w, w.student)
	expire(t, pool, session.Attempt.ID)

	for range 3 {
		if _, err := svc.Queries.Get.Handle(t.Context(), query.Get{AttemptID: session.Attempt.ID, StudentID: w.student}); err != nil {
			t.Fatal(err)
		}
	}
	if sent := tell.of(notificationsdomain.AttemptSubmitted); len(sent) != 1 || sent[0].UserID != w.admin {
		t.Errorf("a timed-out paper read three times produced %+v, want one notice", sent)
	}
}

func TestAnAttemptTimedOutByTheTeachersMonitorIsHandedInToo(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	tell := &told{}
	svc := announcing(pool, tell, nil)
	session := startFor(t, svc, w, w.student)
	expire(t, pool, session.Attempt.ID)

	if _, err := svc.Commands.ExpireDue.Handle(t.Context(), command.ExpireDue{AssignmentID: w.assignment, Scope: everyone}); err != nil {
		t.Fatal(err)
	}
	if sent := tell.of(notificationsdomain.AttemptSubmitted); len(sent) != 1 {
		t.Errorf("expiring the due attempts produced %+v, want one notice", sent)
	}
}

func TestThePolicyFlagsAPaperOnceAndTheTeachersAreToldOnce(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, focusLimit(1, "flag"))
	tell := &told{}
	svc := announcing(pool, tell, nil)
	session := startFor(t, svc, w, w.student)
	title := titleOf(t, pool, w)
	flush := func(seq int) {
		t.Helper()
		if _, err := svc.Commands.Flush.Handle(t.Context(), command.Flush{Input: flushEvents(w, session, away(seq, 4000))}); err != nil {
			t.Fatal(err)
		}
	}

	flush(1)
	if sent := tell.of(notificationsdomain.AttemptFlagged); len(sent) != 0 {
		t.Fatalf("one absence is within the limit of one, and %+v was sent", sent)
	}
	flush(2)
	sent := tell.of(notificationsdomain.AttemptFlagged)
	if len(sent) != 1 || sent[0].UserID != w.admin {
		t.Fatalf("the second absence crossed the limit; sent %+v, want one notice to the teacher", sent)
	}
	n := sent[0]
	if want := (notificationsdomain.Flagged{StudentName: "Người dùng", Title: title, FocusLost: 2}); n.Params != want {
		t.Errorf("params %+v, want %+v", n.Params, want)
	}
	if want := (notificationsdomain.Target{Route: notificationsdomain.RouteAttempt, AttemptID: session.Attempt.ID, AssignmentID: w.assignment}); *n.Target != want {
		t.Errorf("target %+v, want %+v", *n.Target, want)
	}
	if n.DedupeKey != "flagged:"+session.Attempt.ID {
		t.Errorf("dedupe key %q", n.DedupeKey)
	}
	flush(3)
	flush(4)
	if sent := tell.of(notificationsdomain.AttemptFlagged); len(sent) != 1 {
		t.Errorf("more absences on a flagged paper sent %d notices in all, want the one", len(sent))
	}
}

func TestABeaconThatCrossesTheLimitFlagsThePaperAndTellsTheTeacher(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, focusLimit(1, "flag"))
	tell := &told{}
	svc := announcing(pool, tell, nil)
	session := startFor(t, svc, w, w.student)
	beacon := domain.FlushInput{
		AttemptID: session.Attempt.ID, SessionID: session.SessionID, BeaconToken: session.BeaconToken,
		Events: []domain.Event{away(1, 4000), away(2, 4000)},
	}
	if _, err := svc.Commands.Flush.Handle(t.Context(), command.Flush{Input: beacon}); err != nil {
		t.Fatal(err)
	}
	if sent := tell.of(notificationsdomain.AttemptFlagged); len(sent) != 1 || sent[0].UserID != w.admin {
		t.Errorf("a beacon with two absences sent %+v, want the one flag notice", sent)
	}
}

func TestAWarningPolicyAndATeachersOwnFlagTellNobody(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, focusLimit(1, "warn"))
	tell := &told{}
	svc := announcing(pool, tell, nil)
	session := startFor(t, svc, w, w.student)
	for seq := 1; seq <= 3; seq++ {
		if _, err := svc.Commands.Flush.Handle(t.Context(), command.Flush{Input: flushEvents(w, session, away(seq, 4000))}); err != nil {
			t.Fatal(err)
		}
	}
	var flagged bool
	if err := pool.QueryRow(t.Context(), `SELECT flagged FROM app.attempts WHERE id = $1::uuid`, session.Attempt.ID).Scan(&flagged); err != nil || flagged {
		t.Fatalf("a warn policy flagged the paper: %v, %v", flagged, err)
	}
	if _, err := svc.Commands.Flag.Handle(t.Context(), command.Flag{Request: teacher(w), AttemptID: session.Attempt.ID, Flagged: true, Reason: "Nghi ngờ"}); err != nil {
		t.Fatal(err)
	}
	if tell.count() != 0 {
		t.Errorf("a warning and a teacher's own flag sent %+v", tell.sent)
	}
}

func TestAnAutoSubmittedPaperIsBothHandedInAndFlagged(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, focusLimit(1, "auto_submit"))
	tell := &told{}
	svc := announcing(pool, tell, nil)
	session := startFor(t, svc, w, w.student)
	save := domain.SaveInput{
		AttemptID: session.Attempt.ID, StudentID: w.student, SessionID: session.SessionID,
		Events: []domain.Event{away(1, 4000), away(2, 4000)},
	}
	if _, err := svc.Commands.Save.Handle(t.Context(), command.Save{Input: save}); err != nil {
		t.Fatal(err)
	}
	if got := len(tell.of(notificationsdomain.AttemptSubmitted)); got != 1 {
		t.Errorf("the limit closed the paper and %d submission notices were sent, want 1", got)
	}
	if got := len(tell.of(notificationsdomain.AttemptFlagged)); got != 1 {
		t.Errorf("the limit flagged the paper and %d flag notices were sent, want 1", got)
	}
}

func TestAPaperVoidedBeforeALateFlagIsNotAnnounced(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, focusLimit(1, "flag"))
	tell := &told{}
	svc := announcing(pool, tell, nil)
	session := startFor(t, svc, w, w.student)
	if _, err := svc.Commands.Void.Handle(t.Context(), command.Void{Request: teacher(w), AttemptID: session.Attempt.ID, Reason: "Làm lại"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Commands.Flush.Handle(t.Context(), command.Flush{Input: flushEvents(w, session, away(1, 4000), away(2, 4000))}); err != nil {
		t.Fatal(err)
	}
	if tell.count() != 0 {
		t.Errorf("a voided paper was announced: %+v", tell.sent)
	}
}

func TestAFailingNotifierNeverFailsTheStudentsSubmission(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	tell := &told{err: errNotifierDown}
	var log bytes.Buffer
	svc := announcing(pool, tell, &log)
	session := startFor(t, svc, w, w.student)

	submitted, err := svc.Commands.Submit.Handle(t.Context(), command.Submit{AttemptID: session.Attempt.ID, StudentID: w.student, Reason: domain.Manual})
	if err != nil || submitted.Status != domain.Submitted {
		t.Fatalf("submitting with the notifier down: %+v, %v", submitted, err)
	}
	if !strings.Contains(log.String(), "announcement not delivered") || !strings.Contains(log.String(), errNotifierDown.Error()) {
		t.Errorf("the failure was logged as %q", log.String())
	}
}

func TestARequestThatEndsWhileTheTeachersAreBeingToldStillTellsThemAll(t *testing.T) {
	pool := newPool(t)
	w := seedWorld(t, pool, openAssignment())
	second := addSecondClass(t, pool, w)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	tell := &told{onFirst: cancel}
	svc := announcing(pool, tell, nil)
	session := startFor(t, svc, w, second.student)

	if _, err := svc.Commands.Submit.Handle(ctx, command.Submit{AttemptID: session.Attempt.ID, StudentID: second.student, Reason: domain.Manual}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if ctx.Err() == nil {
		t.Fatal("the notifier never ended the request, so the test proves nothing")
	}
	got := tell.users(notificationsdomain.AttemptSubmitted)
	if !got[w.admin] || !got[second.teacher] {
		t.Errorf("after the request ended the notices went to %v, want both teachers", got)
	}
}
