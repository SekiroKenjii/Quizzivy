package application_test

import (
	"context"
	"errors"
	"io"
	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
	"strings"
	"testing"
	"time"
)

type replacementRepo struct {
	scriptedRepo
	target                                    domain.ReplacementTarget
	targetErr, replaceErr, refsErr, countsErr error
	replacements                              []domain.ReplaceInput
	cancel                                    context.CancelFunc
}

func (r *replacementRepo) FindReplacementTarget(_ context.Context, scope access.Scope, id string) (domain.ReplacementTarget, error) {
	r.finds = append(r.finds, found{scope: scope, id: id})
	return r.target, r.targetErr
}
func (r *replacementRepo) Replace(_ context.Context, in domain.ReplaceInput) (domain.ReplaceResult, error) {
	r.replacements = append(r.replacements, in)
	if r.cancel != nil {
		r.cancel()
	}
	if r.replaceErr != nil {
		return domain.ReplaceResult{}, r.replaceErr
	}
	return domain.ReplaceResult{Asset: domain.Asset{ID: in.Asset.ID, StorageKey: in.Asset.StorageKey, Kind: in.Asset.Kind, Bytes: in.Asset.Bytes, DisplayName: *in.Asset.DisplayName, DefaultMaxPlays: in.Asset.DefaultMaxPlays}, Repointed: domain.ReplacementCounts{Questions: 2, Groups: 1}, Left: domain.ReplacementCounts{Questions: 3, Groups: 2}}, nil
}
func (r *replacementRepo) ReferencesFor(context.Context, []string) (map[string][]domain.TestRef, error) {
	return r.versions, r.refsErr
}
func (r *replacementRepo) QuestionCounts(context.Context, []string) (map[string]int, error) {
	return r.questions, r.countsErr
}

type replacementStore struct {
	recordingStore
	deleteErr, signErr                        error
	cleanupLive, cleanupBounded, cleanupValue bool
}
type cleanupValueKey struct{}

func (s *replacementStore) Delete(ctx context.Context, key string) error {
	s.deletes = append(s.deletes, key)
	s.cleanupLive = ctx.Err() == nil
	deadline, ok := ctx.Deadline()
	s.cleanupBounded = ok && time.Until(deadline) > 0 && time.Until(deadline) <= 5*time.Second
	s.cleanupValue = ctx.Value(cleanupValueKey{}) == "keep"
	return s.deleteErr
}
func (s *replacementStore) SignedURL(context.Context, string, time.Duration) (string, error) {
	return "https://signed.test/new", s.signErr
}
func replacementBench() (*application.Application, *replacementRepo, *replacementStore) {
	r := &replacementRepo{target: domain.ReplacementTarget{Asset: domain.Asset{ID: "old-id", Kind: domain.KindImage, StorageKey: "image/old.png", DisplayName: "Tên hiện tại"}, OwnerID: "owner"}}
	s := &replacementStore{}
	return application.New(r, s, &scriptedAudio{durationMs: 1000}).WithImageProbe(&scriptedImage{width: 1200, height: 800}), r, s
}
func replacementCommand(t *testing.T) command.Replace {
	t.Helper()
	return command.Replace{ID: "old-id", Scope: access.Scope{UserID: "actor", All: true}, UploaderID: "actor", Filename: "../../mới.png", Body: imageOf(t, 32)}
}

func TestReplacementPreflightDoesNotReadOrStoreRefusedBody(t *testing.T) {
	app, r, s := replacementBench()
	r.targetErr = domain.ErrNotFound
	body := imageOf(t, 32)
	cmd := replacementCommand(t)
	cmd.Body = body
	_, err := app.Commands.Replace.Handle(context.Background(), cmd)
	if !errors.Is(err, domain.ErrNotFound) || body.read != 0 || len(s.puts) != 0 || len(r.replacements) != 0 {
		t.Fatalf("preflight err=%v read=%d puts=%v replacements=%v", err, body.read, s.puts, r.replacements)
	}
}
func TestReplacementKindMismatchPrecedesObjectPut(t *testing.T) {
	app, r, s := replacementBench()
	r.target.Asset.Kind = domain.KindAudio
	_, err := app.Commands.Replace.Handle(context.Background(), replacementCommand(t))
	if !errors.Is(err, domain.ErrKindMismatch) || len(s.puts) != 0 || len(r.replacements) != 0 {
		t.Fatalf("mismatch err=%v puts=%v replacements=%v", err, s.puts, r.replacements)
	}
}
func TestReplacementUsesFreshIdentityAndReturnsFourCounts(t *testing.T) {
	app, r, s := replacementBench()
	max := 4
	r.target.Asset.DefaultMaxPlays = &max
	out, err := app.Commands.Replace.Handle(context.Background(), replacementCommand(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.replacements) != 1 || len(s.puts) != 1 || len(s.deletes) != 0 {
		t.Fatalf("calls=%v puts=%v deletes=%v", r.replacements, s.puts, s.deletes)
	}
	in := r.replacements[0]
	if in.Asset.ID == "old-id" || s.puts[0].key == r.target.Asset.StorageKey || !strings.HasPrefix(s.puts[0].key, "image/"+in.Asset.ID) || in.Asset.OwnerID != "owner" || in.Asset.UploaderID != "actor" || in.Asset.OriginalFilename != "mới.png" || *in.Asset.DisplayName != "Tên hiện tại" || *in.Asset.DefaultMaxPlays != 4 {
		t.Fatalf("input=%+v", in)
	}
	if out.Repointed != (domain.ReplacementCounts{Questions: 2, Groups: 1}) || out.Left != (domain.ReplacementCounts{Questions: 3, Groups: 2}) || out.Asset.URL == "" {
		t.Fatalf("result=%+v", out)
	}
}
func TestReplacementOutcomeControlsFreshObjectCompensation(t *testing.T) {
	cause := errors.New("operation failed")
	for _, tc := range []struct {
		name   string
		err    error
		remove bool
	}{{"not committed", &domain.ReplacementError{Outcome: domain.ReplacementNotCommitted, Cause: cause}, true}, {"unknown", &domain.ReplacementError{Outcome: domain.ReplacementUnknown, Cause: cause}, false}, {"committed", &domain.ReplacementError{Outcome: domain.ReplacementCommitted, Cause: cause}, false}, {"plain", cause, false}, {"rollback failed", &domain.ReplacementError{Outcome: domain.ReplacementNotCommitted, Cause: cause, RollbackError: errors.New("rollback")}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			app, r, s := replacementBench()
			r.replaceErr = tc.err
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), cleanupValueKey{}, "keep"))
			defer cancel()
			r.cancel = cancel
			_, err := app.Commands.Replace.Handle(ctx, replacementCommand(t))
			if !errors.Is(err, cause) {
				t.Fatalf("lost cause: %v", err)
			}
			if len(s.puts) != 1 {
				t.Fatal("object was not put")
			}
			want := 0
			if tc.remove {
				want = 1
			}
			if len(s.deletes) != want {
				t.Fatalf("deletes=%v want %d", s.deletes, want)
			}
			if tc.remove && (s.deletes[0] != s.puts[0].key || !s.cleanupLive || !s.cleanupBounded || !s.cleanupValue) {
				t.Fatalf("cleanup key/context wrong: %+v", s)
			}
			var failure *domain.ReplacementError
			if !errors.As(err, &failure) {
				t.Fatalf("missing outcome: %v", err)
			}
			if tc.name == "plain" && failure.Outcome != domain.ReplacementUnknown {
				t.Fatalf("plain outcome=%v", failure.Outcome)
			}
		})
	}
}
func TestReplacementPreservesCleanupAndRollbackErrors(t *testing.T) {
	app, r, s := replacementBench()
	cause := domain.ErrQuotaExceeded
	rollback := errors.New("rollback failed")
	cleanup := errors.New("delete failed")
	r.replaceErr = &domain.ReplacementError{Outcome: domain.ReplacementNotCommitted, Cause: cause, RollbackError: rollback}
	s.deleteErr = cleanup
	_, err := app.Commands.Replace.Handle(context.Background(), replacementCommand(t))
	var failure *domain.ReplacementCleanupError
	if !errors.As(err, &failure) || !errors.Is(err, cause) || !errors.Is(err, rollback) || !errors.Is(err, cleanup) {
		t.Fatalf("errors lost: %v", err)
	}
}
func TestReplacementPutFailureCompensatesOnlyFreshKey(t *testing.T) {
	app, _, s := replacementBench()
	cause := errors.New("put ambiguous")
	s.putErr = cause
	_, err := app.Commands.Replace.Handle(context.Background(), replacementCommand(t))
	if !errors.Is(err, cause) || len(s.deletes) != 1 || s.deletes[0] == "image/old.png" || !s.cleanupLive || !s.cleanupBounded {
		t.Fatalf("put cleanup err=%v deletes=%v", err, s.deletes)
	}
}
func TestReplacementConfirmedCommitResponseFailureRetainsObject(t *testing.T) {
	for _, stage := range []string{"references", "counts", "sign"} {
		t.Run(stage, func(t *testing.T) {
			app, r, s := replacementBench()
			cause := errors.New(stage)
			switch stage {
			case "references":
				r.refsErr = cause
			case "counts":
				r.countsErr = cause
			case "sign":
				s.signErr = cause
			}
			_, err := app.Commands.Replace.Handle(context.Background(), replacementCommand(t))
			var failure *domain.ReplacementError
			if !errors.As(err, &failure) || failure.Outcome != domain.ReplacementCommitted || !errors.Is(err, cause) || len(s.puts) != 1 || len(s.deletes) != 0 {
				t.Fatalf("committed response err=%v deletes=%v", err, s.deletes)
			}
		})
	}
}
func TestReplacementReceiveFailureDoesNotPut(t *testing.T) {
	app, _, s := replacementBench()
	cmd := replacementCommand(t)
	cmd.Body = io.LimitReader(strings.NewReader("x"), 1)
	_, err := app.Commands.Replace.Handle(context.Background(), cmd)
	if !errors.Is(err, domain.ErrUnsupportedType) || len(s.puts) != 0 {
		t.Fatalf("receive err=%v puts=%v", err, s.puts)
	}
}
