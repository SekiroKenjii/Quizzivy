//go:build integration

package repositories_test

import (
	"errors"
	"github.com/google/uuid"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/shared/access"
	"reflect"
	"sort"
	"testing"
)

func TestGradingQueueOrderingAndIdentity(t *testing.T) {
	f := newQueueFixture(t)
	p := f.paper(f.teacher, "Equal title", "short_answer", "short_answer")
	first := f.attempt(p, f.students[0], "submitted", queueTime(1), 1)
	later := f.attempt(p, f.students[0], "timed_out", queueTime(2), 2)
	tied := f.attempt(p, f.students[0], "submitted", queueTime(1), 3)
	second := f.attempt(p, f.students[1], "submitted", queueTime(1), 1)
	for _, at := range []string{first, later, tied, second} {
		for _, q := range p.questions {
			f.answer(at, q, true, nil)
		}
	}
	repeated := p
	repeated.assignment = f.assignment(f.teacher, p)
	other := f.attempt(repeated, f.students[1], "submitted", queueTime(3), 1)
	f.answer(other, p.questions[0], true, nil)
	nullPaper := f.paper(f.teacher, "Equal title", "short_answer")
	nullAttempt := f.attempt(nullPaper, f.students[2], "timed_out", nil, 1)
	f.answer(nullAttempt, nullPaper.questions[0], true, nil)
	section := f.id("test_version_sections")
	f.exec(`INSERT INTO app.test_version_sections(id,test_version_id,ordinal,title) VALUES($1,$2,1,'Phần 2')`, section, p.version)
	third := f.id("test_version_questions")
	f.exec(`INSERT INTO app.test_version_questions(id,test_version_section_id,ordinal,type,prompt,points) VALUES($1,$2,0,'short_answer','Second section question',1)`, third, section)
	f.answer(first, third, true, nil)
	unknownTime := f.attempt(p, f.students[0], "timed_out", nil, 4)
	f.answer(unknownTime, p.questions[0], true, nil)
	student := f.read(domain.GradingQueueQuery{Scope: f.own()})
	names := append([]string(nil), f.students[:2]...)
	sort.Strings(names)
	if student.AnswersRemaining != 12 || student.StudentsWaiting != 3 || len(student.Groups) != 3 {
		t.Fatalf("full counts/groups=%+v", student)
	}
	if student.Groups[0].Key != names[0] || student.Groups[1].Key != names[1] || student.Groups[2].Key != f.students[2] || student.Groups[0].Label != "Trùng tên" || student.Groups[1].Label != "Trùng tên" {
		t.Fatalf("UUID/NULL grouping=%+v", student.Groups)
	}
	early := []string{first, tied}
	sort.Strings(early)
	firstItems := map[string][]string{first: {first + ":" + p.questions[0], first + ":" + p.questions[1], first + ":" + third}, tied: {tied + ":" + p.questions[0], tied + ":" + p.questions[1]}}
	ownItems := append(append(append([]string{}, firstItems[early[0]]...), firstItems[early[1]]...), later+":"+p.questions[0], later+":"+p.questions[1], unknownTime+":"+p.questions[0])
	want := map[string][]string{f.students[0]: ownItems, f.students[1]: {second + ":" + p.questions[0], second + ":" + p.questions[1], other + ":" + p.questions[0]}, f.students[2]: {nullAttempt + ":" + nullPaper.questions[0]}}

	var expected []string
	for _, group := range student.Groups {
		expected = append(expected, want[group.Key]...)
	}
	if !reflect.DeepEqual(queueItemIDs(student), expected) {
		t.Fatalf("student paper order got=%v want=%v", queueItemIDs(student), expected)
	}
	question := f.read(domain.GradingQueueQuery{Scope: f.own(), Mode: "question"})
	if len(question.Groups) != 5 || question.Groups[4].Key != nullPaper.assignment+":"+nullPaper.questions[0] {
		t.Fatalf("question identity/NULL group: %+v", question.Groups)
	}
	keys := []string{p.assignment + ":" + p.questions[0], p.assignment + ":" + p.questions[1], p.assignment + ":" + third}
	sort.Strings(keys)
	if question.Groups[0].Key != keys[0] || question.Groups[1].Key != keys[1] || question.Groups[2].Key != keys[2] || question.Groups[3].Key != repeated.assignment+":"+p.questions[0] {
		t.Fatalf("composite group ties: %+v", question.Groups)
	}
	labels := map[string]string{p.assignment + ":" + p.questions[0]: "1", p.assignment + ":" + p.questions[1]: "2", p.assignment + ":" + third: "3", repeated.assignment + ":" + p.questions[0]: "1", nullPaper.assignment + ":" + nullPaper.questions[0]: "1"}
	for _, group := range question.Groups {
		if group.Kind != "question" || group.Label != labels[group.Key] || group.Sub != "Equal title" {
			t.Fatalf("raw labels/sub=%+v", group)
		}
	}
}

func TestGradingQueueEligibilityAndFilters(t *testing.T) {
	f := newQueueFixture(t)
	p := f.paper(f.teacher, "Owned", "short_answer", "true_false", "short_answer")
	handed := f.attempt(p, f.students[0], "submitted", queueTime(1), 1)
	f.answer(handed, p.questions[0], true, nil)
	f.answer(handed, p.questions[1], true, nil)
	zero := 0.0
	f.answer(handed, p.questions[2], true, &zero)
	for i, status := range []string{"in_progress", "voided", "graded"} {
		at := f.attempt(p, f.students[1], status, nil, i+1)
		f.answer(at, p.questions[0], true, nil)
	}
	f.attempt(p, f.students[2], "submitted", queueTime(1), 1)
	timed := f.attempt(p, f.students[2], "timed_out", queueTime(2), 2)
	f.answer(timed, p.questions[0], true, nil)
	f.answer(timed, p.questions[1], false, nil)
	foreign := f.paper(f.foreign, "Foreign", "short_answer")
	foreignStudent := f.user("student", "Foreign student", f.foreign)
	foreignAt := f.attempt(foreign, foreignStudent, "submitted", queueTime(0), 1)
	f.answer(foreignAt, foreign.questions[0], true, nil)
	own := f.read(domain.GradingQueueQuery{Scope: f.own()})
	if own.AnswersRemaining != 3 || own.StudentsWaiting != 2 || len(own.Items) != 3 {
		t.Fatalf("eligibility=%+v", own)
	}
	expected := []string{handed + ":" + p.questions[0], handed + ":" + p.questions[1], timed + ":" + p.questions[0]}
	if !reflect.DeepEqual(queueItemIDs(own), expected) {
		t.Fatalf("eligibility IDs=%v", queueItemIDs(own))
	}
	f.assertAll("student")
	f.assertAll("question")
	all := f.read(domain.GradingQueueQuery{Scope: access.Scope{All: true}, AssignmentID: &foreign.assignment})
	if all.AnswersRemaining != 1 || all.StudentsWaiting != 1 || len(all.Items) != 1 || all.Items[0].AttemptID != foreignAt {
		t.Fatalf("wide All excludes known foreign work: %+v", all)
	}
	empty := f.paper(f.teacher, "Empty", "short_answer")
	for _, q := range []domain.GradingQueueQuery{{Scope: f.own(), AssignmentID: &empty.assignment}, {Scope: f.own(), StudentID: &f.students[1]}, {Scope: f.own(), AssignmentID: &empty.assignment, StudentID: &f.students[0]}} {
		out := f.read(q)
		if out.AnswersRemaining != 0 || out.StudentsWaiting != 0 || len(out.Items) != 0 || len(out.Groups) != 0 {
			t.Fatalf("independent empty filter=%+v", out)
		}
	}
	for _, id := range []string{foreign.assignment, uuid.NewString()} {
		if _, err := f.repo.GradingQueue(f.ctx, domain.GradingQueueQuery{Scope: f.own(), AssignmentID: &id}); !errors.Is(err, domain.ErrPaperNotFound) {
			t.Fatalf("foreign/missing assignment=%v", err)
		}
	}
	for _, id := range []string{foreignStudent, uuid.NewString()} {
		if _, err := f.repo.GradingQueue(f.ctx, domain.GradingQueueQuery{Scope: f.own(), StudentID: &id}); !errors.Is(err, domain.ErrPaperNotFound) {
			t.Fatalf("foreign/missing taker=%v", err)
		}
	}
	reached := f.read(domain.GradingQueueQuery{Scope: f.own(), StudentID: &f.students[0]})
	if len(reached.Items) != 2 {
		t.Fatalf("student filter=%+v", reached)
	}
}

func TestGradingQueueHistoricalPaperTakerAndBothReachPredicates(t *testing.T) {
	f := newQueueFixture(t)
	p := f.paper(f.teacher, "Historical", "short_answer")
	promoted := f.user("student", "Historical staff", "")
	at := f.attempt(p, promoted, "submitted", queueTime(1), 1)
	f.answer(at, p.questions[0], true, nil)
	f.exec(`UPDATE app.users SET role_id=(SELECT id FROM app.roles WHERE builtin_key='teacher') WHERE id=$1`, promoted)
	out := f.read(domain.GradingQueueQuery{Scope: f.own(), StudentID: &promoted})
	if len(out.Items) != 1 || out.Items[0].StudentID != promoted {
		t.Fatalf("historical role-independent paper=%+v", out)
	}
	if _, err := f.repo.GradingQueue(f.ctx, domain.GradingQueueQuery{Scope: access.Scope{All: true}, StudentID: &f.foreign}); !errors.Is(err, domain.ErrPaperNotFound) {
		t.Fatalf("privileged current no-history account=%v", err)
	}
	f.exec(`UPDATE app.attempts SET status='graded',graded_at=now() WHERE id=$1`, at)
	out = f.read(domain.GradingQueueQuery{Scope: f.own(), StudentID: &promoted})
	if out.AnswersRemaining != 0 {
		t.Fatal("historical precheck used pending eligibility")
	}
	inaccessible := f.paper(f.foreign, "Unreachable", "short_answer")
	other := f.user("teacher", "Unreached history", "")
	f.attempt(inaccessible, other, "voided", nil, 1)
	if _, err := f.repo.GradingQueue(f.ctx, domain.GradingQueueQuery{Scope: f.own(), StudentID: &other}); !errors.Is(err, domain.ErrPaperNotFound) {
		t.Fatalf("unscoped history expanded reach=%v", err)
	}
	class := f.id("classes")
	f.exec(`INSERT INTO app.classes(id,name,teacher_id) VALUES($1,'Owned shared class',$2)`, class, f.teacher)
	f.exec(`INSERT INTO app.assignment_classes(assignment_id,class_id) VALUES($1,$2)`, inaccessible.assignment, class)
	if _, err := f.repo.GradingQueue(f.ctx, domain.GradingQueueQuery{Scope: f.own(), StudentID: &other}); !errors.Is(err, domain.ErrPaperNotFound) {
		t.Fatalf("assignment reach without Papers granted history=%v", err)
	}
	paperOnly := f.user("teacher", "Paper reach only", f.teacher)
	f.attempt(f.paper(f.foreign, "Paper without assignment reach", "short_answer"), paperOnly, "voided", nil, 1)
	if _, err := f.repo.GradingQueue(f.ctx, domain.GradingQueueQuery{Scope: f.own(), StudentID: &paperOnly}); !errors.Is(err, domain.ErrPaperNotFound) {
		t.Fatalf("Papers reach without assignment granted history=%v", err)
	}
	onlyPaperStudent := f.user("student", "Known student", f.teacher)
	f.attempt(f.paper(f.foreign, "Wrong assignment", "short_answer"), onlyPaperStudent, "graded", nil, 1)
	out = f.read(domain.GradingQueueQuery{Scope: f.own(), StudentID: &onlyPaperStudent})
	if out.AnswersRemaining != 0 {
		t.Fatal("current-empty strict student not reached")
	}
	removed := f.user("student", "Removed member", "")
	f.exec(`INSERT INTO app.class_members(class_id,user_id,joined_via,added_by) VALUES($1,$2,'admin',$3)`, class, removed, f.teacher)
	f.exec(`INSERT INTO app.assignment_classes(assignment_id,class_id) VALUES($1,$2)`, p.assignment, class)
	old := f.attempt(p, removed, "timed_out", nil, 1)
	f.answer(old, p.questions[0], true, nil)
	f.exec(`DELETE FROM app.class_members WHERE class_id=$1 AND user_id=$2`, class, removed)
	f.exec(`UPDATE app.users SET disabled_at=now() WHERE id=$1`, removed)
	out = f.read(domain.GradingQueueQuery{Scope: f.own(), StudentID: &removed})
	if len(out.Items) != 1 || out.Items[0].AttemptID != old {
		t.Fatalf("removed disabled historical class member=%+v", out)
	}
}

func TestGradingQueueFullCountsBeforeCap(t *testing.T) {
	f := newQueueFixture(t)
	p := f.paper(f.teacher, "Large", "short_answer")
	known := make([]string, 0, 202)
	for i := 1; i <= 202; i++ {
		at := f.attempt(p, f.students[0], "submitted", queueTime(1), i)
		f.answer(at, p.questions[0], true, nil)
		known = append(known, at+":"+p.questions[0])
	}
	after := f.attempt(p, f.students[1], "submitted", queueTime(2), 1)
	f.answer(after, p.questions[0], true, nil)
	out := f.read(domain.GradingQueueQuery{Scope: f.own()})
	if out.AnswersRemaining != 203 || out.StudentsWaiting != 2 || len(out.Items) != 200 || len(out.Groups) != 1 || out.Groups[0].Remaining != 202 {
		t.Fatalf("complete counts/split cap=%+v", out)
	}
	sort.Strings(known)
	if !reflect.DeepEqual(queueItemIDs(out), known[:200]) {
		t.Fatalf("exact student prefix got=%v want=%v", queueItemIDs(out), known[:200])
	}
	seen := map[string]bool{}
	for _, id := range queueItemIDs(out) {
		if seen[id] {
			t.Fatalf("duplicate prefix=%s", id)
		}
		seen[id] = true
	}
	narrowed := f.read(domain.GradingQueueQuery{Scope: f.own(), StudentID: &f.students[1]})
	if len(narrowed.Items) != 1 || narrowed.Items[0].AttemptID != after || narrowed.AnswersRemaining != 1 || narrowed.StudentsWaiting != 1 {
		t.Fatalf("filter was applied after global cap: %+v", narrowed)
	}
	question := f.read(domain.GradingQueueQuery{Scope: f.own(), Mode: "question"})
	if !reflect.DeepEqual(queueItemIDs(question), known[:200]) {
		t.Fatalf("exact question prefix got=%v want=%v", queueItemIDs(question), known[:200])
	}
	if len(question.Groups) != 1 || question.Groups[0].Remaining != 203 || len(question.Items) != 200 {
		t.Fatalf("question group full remaining=%+v", question)
	}
	f.assertAll("student")
	f.assertAll("question")
}
