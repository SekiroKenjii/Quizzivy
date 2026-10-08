package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"quizzivy/internal/modules/imports/application/command"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/actor"
	"testing"
)

type capDraftStore struct {
	domain.Drafts
	draft    domain.StoredDraft
	previous *domain.Commit
	reads    int
}

func (s *capDraftStore) Commit(context.Context, access.Scope, string) (domain.Commit, error) {
	if s.previous != nil {
		return *s.previous, nil
	}
	return domain.Commit{}, domain.ErrNotFound
}
func (s *capDraftStore) Draft(context.Context, access.Scope, string) (domain.StoredDraft, error) {
	s.reads++
	return s.draft, nil
}

type capImportStore struct{ domain.Repository }

func (capImportStore) Get(context.Context, access.Scope, string) (domain.Import, error) {
	return domain.Import{}, nil
}

type capMaterializer struct{ calls int }

func (m *capMaterializer) Materialize(context.Context, domain.CommitPlan, string, actor.Actor, func(context.Context, domain.CommitStore, string) error) (string, error) {
	m.calls++
	return "", errors.New("materializer must not run")
}

func TestOversizedImportPlanRefusesBeforeMaterializerAndReplayStillWins(t *testing.T) {
	q := domain.DraftQuestion{ID: "q", Type: "single_choice", Points: "1", Prompt: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Pick","marks":[]}]}]}`), Answer: domain.DraftAnswer{State: domain.AnswerKnown, OptionIDs: []string{"o0"}}}
	for i := range 9 {
		id := uuid.NewString()
		if i == 0 {
			id = "o0"
		}
		q.Options = append(q.Options, domain.DraftOption{ID: id, Label: id, Content: json.RawMessage(`{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Option","marks":[]}]}]}`)})
	}
	store := &capDraftStore{draft: domain.StoredDraft{Status: "needs_review", Revision: 1, Draft: domain.Draft{Version: domain.DraftVersion, Sections: []domain.DraftSection{{ID: "s", Title: "Part", Items: []domain.DraftItem{{Question: &q}}}}}}}
	materializer := &capMaterializer{}
	handler := command.CommitHandler{Repo: capImportStore{}, Drafts: store, Materializer: materializer}
	req := command.Commit{ImportID: "i", RequestID: "request", DraftRevision: 1}
	if _, err := handler.Handle(context.Background(), req); !errors.Is(err, domain.ErrNotReady) || materializer.calls != 0 {
		t.Fatalf("plan must refuse before materialization: %v calls=%d", err, materializer.calls)
	}
	testID := uuid.NewString()
	store.previous = &domain.Commit{RequestID: req.RequestID, TestID: &testID}
	result, err := handler.Handle(context.Background(), req)
	if err != nil || result.TestID != testID || materializer.calls != 0 || store.reads != 1 {
		t.Fatalf("replay was revalidated: %+v %v calls=%d reads=%d", result, err, materializer.calls, store.reads)
	}
}
