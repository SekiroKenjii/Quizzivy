package command

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/classes/application/internal/support"
	"quizzivy/internal/modules/classes/application/ports"
	"quizzivy/internal/modules/classes/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"

	"github.com/google/uuid"
)

const (
	legacyRotationNotice = "join_codes.rotated:legacy"
	notifiedClassNames   = 5
)

// RotateLegacyJoinCodes replaces every join code only a SHA-256 holds, and
// that can still be redeemed, with a sealed one: class by class, each in its
// own transaction, in the order the repository lists them. A class that fails
// does not stop the run, and one that lost a deadlock or a serialization
// conflict is tried once more after every other class. Each teacher is then
// told once, with how many of their classes were rotated and the names of the
// first five, in a notification a later run adds its count to; a class without
// a teacher is rotated and nobody is told, and a failed notification undoes
// nothing. Found, when set, is told how many
// classes hold such a code before the first is rotated. Problem, when set, is
// told of each class left unrotated and each teacher left untold, in an error
// that names the class or the teacher and never a code or a hint.
type RotateLegacyJoinCodes struct {
	Found   func(count int)
	Problem func(err error)
}

type RotateLegacyJoinCodesHandler struct {
	*support.Enrolment
	Notifier ports.Notifier
}

func (s RotateLegacyJoinCodesHandler) Handle(ctx context.Context, cmd RotateLegacyJoinCodes) (domain.LegacyRotation, error) {
	classes, err := s.Repo.LegacyCodeClasses(ctx, s.Now())
	if err != nil {
		return domain.LegacyRotation{}, err
	}
	if cmd.Found != nil {
		cmd.Found(len(classes))
	}

	run := &legacyRun{handler: s, problem: cmd.Problem, result: domain.LegacyRotation{Found: len(classes)}}
	teachers := byTeacher(classes)
	for _, teacher := range teachers {
		run.rotateAll(ctx, teacher)
		if !teacher.contended() {
			run.notify(ctx, teacher)
		}
	}
	for _, teacher := range teachers {
		if teacher.contended() {
			run.retry(ctx, teacher)
			run.notify(ctx, teacher)
		}
	}
	return run.result, nil
}

type legacyClass struct {
	domain.LegacyCodeClass
	rotated   bool
	contended bool
}

type teacherClasses struct {
	teacherID string
	classes   []*legacyClass
}

func (t *teacherClasses) contended() bool {
	for _, class := range t.classes {
		if class.contended {
			return true
		}
	}
	return false
}

func (t *teacherClasses) rotated() (int, []string) {
	count := 0
	names := make([]string, 0, notifiedClassNames)
	for _, class := range t.classes {
		if !class.rotated {
			continue
		}
		count++
		if len(names) < notifiedClassNames {
			names = append(names, class.ClassName)
		}
	}
	return count, names
}

func byTeacher(classes []domain.LegacyCodeClass) []*teacherClasses {
	var teachers []*teacherClasses
	index := map[string]*teacherClasses{}
	for _, class := range classes {
		teacherID := ""
		if class.TeacherID != nil {
			teacherID = *class.TeacherID
		}
		teacher, seen := index[teacherID]
		if !seen {
			teacher = &teacherClasses{teacherID: teacherID}
			index[teacherID] = teacher
			teachers = append(teachers, teacher)
		}
		teacher.classes = append(teacher.classes, &legacyClass{LegacyCodeClass: class})
	}
	return teachers
}

type legacyRun struct {
	handler RotateLegacyJoinCodesHandler
	problem func(err error)
	result  domain.LegacyRotation
}

func (r *legacyRun) rotateAll(ctx context.Context, teacher *teacherClasses) {
	for _, class := range teacher.classes {
		wrote, err := r.handler.rotate(ctx, class.ClassID)
		if errors.Is(err, domain.ErrRotationContended) {
			class.contended = true
			continue
		}
		r.settle(class, wrote, err)
	}
}

func (r *legacyRun) retry(ctx context.Context, teacher *teacherClasses) {
	for _, class := range teacher.classes {
		if !class.contended {
			continue
		}
		wrote, err := r.handler.rotate(ctx, class.ClassID)
		r.settle(class, wrote, err)
	}
}

func (r *legacyRun) settle(class *legacyClass, wrote bool, err error) {
	if err != nil {
		r.result.Failed++
		r.report(fmt.Errorf("rotate the legacy join code of class %s: %w", class.ClassID, err))
		return
	}
	if wrote {
		class.rotated = true
		r.result.Rotated++
	}
}

func (r *legacyRun) notify(ctx context.Context, teacher *teacherClasses) {
	count, names := teacher.rotated()
	if count == 0 || teacher.teacherID == "" || r.handler.Notifier == nil {
		return
	}
	_, err := r.handler.Notifier.Handle(ctx, notificationscommand.Notify{
		UserID:    teacher.teacherID,
		Kind:      notificationsdomain.JoinCodesRotated,
		Params:    notificationsdomain.CodesRotated{Count: count, ClassNames: names},
		Target:    &notificationsdomain.Target{Route: notificationsdomain.RouteClasses},
		DedupeKey: legacyRotationNotice,
		Merge:     notificationsdomain.Add,
	})
	if err != nil {
		r.result.NotifyFailed++
		r.report(fmt.Errorf("tell teacher %s of their rotated join codes: %w", teacher.teacherID, err))
		return
	}
	r.result.Teachers++
}

func (r *legacyRun) report(err error) {
	if r.problem != nil {
		r.problem(err)
	}
}

func (s RotateLegacyJoinCodesHandler) rotate(ctx context.Context, classID string) (bool, error) {
	code, err := domain.JoinCodes.Generate()
	if err != nil {
		return false, err
	}
	codeID, err := uuid.NewV7()
	if err != nil {
		return false, fmt.Errorf("join code id: %w", err)
	}
	sealed, err := s.Keys.Seal(classID, codeID.String(), code)
	if err != nil {
		return false, err
	}
	return s.Repo.RotateLegacyCode(ctx, domain.LegacyRotationInput{
		ClassID:    classID,
		CodeID:     codeID.String(),
		CodeHash:   s.Keys.Hash(code),
		Ciphertext: sealed,
		KeyID:      s.Keys.CurrentID(),
		Hint:       domain.JoinCodes.Hint(code),
		Now:        s.Now(),
	})
}
