package application_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application"
	"quizzivy/internal/modules/identity/application/command"
	"quizzivy/internal/modules/identity/domain"
	notificationscommand "quizzivy/internal/modules/notifications/application/command"
	notificationsdomain "quizzivy/internal/modules/notifications/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/cqrs"
)

type contentMover struct {
	mu     sync.Mutex
	calls  []domain.ContentTransfer
	reqs   []domain.WriteRequest
	fails  []error
	result domain.TransferredContent
}

func (m *contentMover) Move(_ context.Context, req domain.WriteRequest, in domain.ContentTransfer) (domain.TransferredContent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, in)
	m.reqs = append(m.reqs, req)
	if len(m.fails) > 0 {
		err := m.fails[0]
		m.fails = m.fails[1:]
		if err != nil {
			return domain.TransferredContent{}, err
		}
	}
	return m.result, nil
}

type noticeLog struct {
	mu   sync.Mutex
	sent []notificationscommand.Notify
}

func (n *noticeLog) Handle(_ context.Context, c notificationscommand.Notify) (cqrs.Nothing, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, c)
	return cqrs.Nothing{}, nil
}

var someContent = domain.TransferredContent{
	Moved:      domain.MovedContent{Classes: 2, Tests: 3, Questions: 40, QuestionGroups: 4, Media: 5, Assignments: 6},
	LeftBehind: domain.LeftBehind{Imports: 1},
}

func transferRig(t *testing.T) (*writeRig, *contentMover, *noticeLog) {
	t.Helper()
	r := newWriteRig("gone", "heir")
	mover, notices := &contentMover{result: someContent}, &noticeLog{}
	r.app.SetContentTransfer(mover, notices)
	if r.app.Commands.TransferUserContent == nil {
		t.Fatal("SetContentTransfer left the command nil")
	}
	return r, mover, notices
}

func transfer(r *writeRig, req domain.WriteRequest, from string, notify bool) (domain.TransferredContent, error) {
	return r.app.Commands.TransferUserContent.Handle(context.Background(), command.TransferUserContent{Request: req, ID: from, ToID: "heir", Notify: notify})
}

func TestATransferMovesOnceAndTellsTheRecipientWhoseContentItIs(t *testing.T) {
	r, mover, notices := transferRig(t)

	got, err := transfer(r, theAdmin, "gone", true)

	if err != nil {
		t.Fatal(err)
	}
	if got != someContent {
		t.Errorf("result %+v, want the mover's %+v", got, someContent)
	}
	if len(mover.calls) != 1 || mover.calls[0].FromID != "gone" || mover.calls[0].ToID != "heir" || mover.calls[0].Now.IsZero() {
		t.Errorf("moves %+v, want one from gone to heir at a time", mover.calls)
	}
	if mover.reqs[0].ActorID != "admin" || mover.reqs[0].IP != theAdmin.IP {
		t.Errorf("the move carried %+v, want the caller's request", mover.reqs[0])
	}
	want := notificationscommand.Notify{
		UserID:    "heir",
		Kind:      notificationsdomain.ContentTransferred,
		Params:    notificationsdomain.Transferred{TeacherName: "Tên gone", Count: 60},
		Target:    &notificationsdomain.Target{Route: notificationsdomain.RouteClasses},
		DedupeKey: "transferred:gone",
		Merge:     notificationsdomain.Replace,
	}
	if len(notices.sent) != 1 {
		t.Fatalf("sent %d notifications, want one", len(notices.sent))
	}
	sent := notices.sent[0]
	if sent.UserID != want.UserID || sent.Kind != want.Kind || sent.Params != want.Params || *sent.Target != *want.Target || sent.DedupeKey != want.DedupeKey || sent.Merge != want.Merge {
		t.Errorf("sent %+v, want %+v", sent, want)
	}
}

func TestATransferTellsNobodyUnlessAskedOrWhenNothingMoved(t *testing.T) {
	r, mover, notices := transferRig(t)
	if _, err := transfer(r, theAdmin, "gone", false); err != nil {
		t.Fatal(err)
	}
	mover.result = domain.TransferredContent{LeftBehind: domain.LeftBehind{Imports: 2}}
	if _, err := transfer(r, theAdmin, "gone", true); err != nil {
		t.Fatal(err)
	}
	if len(notices.sent) != 0 {
		t.Errorf("sent %+v, want nothing", notices.sent)
	}
}

func TestATransferIsRefusedForAnUnknownAccountAndOneBeyondTheCallerBeforeAnythingMoves(t *testing.T) {
	r, mover, notices := transferRig(t)
	r.principal.sets["strong"] = adminKeys
	manager := domain.WriteRequest{ActorID: "manager", Grants: access.NewSet(access.PeopleUsersManage)}

	if _, err := transfer(r, theAdmin, "ghost", true); !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("an unknown account: %v, want ErrUserNotFound", err)
	}
	if _, err := transfer(r, manager, "strong", true); !errors.Is(err, accessdomain.ErrTargetRoleNotAllowed) {
		t.Errorf("an Admin moved by a manager: %v, want ErrTargetRoleNotAllowed", err)
	}
	if len(mover.calls) != 0 || len(notices.sent) != 0 {
		t.Errorf("a refusal moved %v and sent %v", mover.calls, notices.sent)
	}
}

func TestATransferTheMoverRefusesIsAnsweredAsRefusedAndTellsNobody(t *testing.T) {
	for _, refusal := range []error{domain.ErrUserNotDisabled, domain.ErrInvalidRecipient} {
		r, mover, notices := transferRig(t)
		mover.fails = []error{refusal}

		_, err := transfer(r, theAdmin, "gone", true)

		if !errors.Is(err, refusal) || len(mover.calls) != 1 || len(notices.sent) != 0 {
			t.Errorf("%v: got %v after %d moves and %d notifications, want the refusal after one move and none", refusal, err, len(mover.calls), len(notices.sent))
		}
	}
}

func TestATransferThatLosesARaceIsMadeAgainAndAThirdLossIsRefused(t *testing.T) {
	contended := fmt.Errorf("%w: deadlock detected", domain.ErrWriteContended)
	r, mover, notices := transferRig(t)
	mover.fails = []error{domain.ErrRoleChanged, contended}

	if _, err := transfer(r, theAdmin, "gone", true); err != nil {
		t.Fatalf("two lost races then a move: %v", err)
	}
	if len(mover.calls) != 3 || len(notices.sent) != 1 {
		t.Errorf("%d moves and %d notifications, want three and one", len(mover.calls), len(notices.sent))
	}

	r, mover, notices = transferRig(t)
	mover.fails = []error{contended, contended, contended}
	if _, err := transfer(r, theAdmin, "gone", true); !errors.Is(err, domain.ErrWriteContended) {
		t.Errorf("three lost races: %v, want ErrWriteContended", err)
	}
	if len(mover.calls) != 3 || len(notices.sent) != 0 {
		t.Errorf("%d moves and %d notifications, want three and none", len(mover.calls), len(notices.sent))
	}
}

func TestTheTransferIsOpenOnlyWithAMoverAndTheAccountDirectory(t *testing.T) {
	r := newWriteRig()
	r.app.SetContentTransfer(nil, nil)
	if r.app.Commands.TransferUserContent != nil {
		t.Error("a nil mover opened the transfer")
	}
	bare := application.New(nil, nil, time.Hour, nil, nil)
	bare.SetContentTransfer(&contentMover{}, nil)
	if bare.Commands.TransferUserContent != nil {
		t.Error("the transfer opened without the account directory")
	}
}
