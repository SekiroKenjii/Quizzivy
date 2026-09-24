//go:build integration

package repositories_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"quizzivy/internal/modules/imports/domain"
	"testing"
)

func artifactQuotas() domain.ArtifactQuotas {
	return domain.ArtifactQuotas{ActorBytes: 1 << 30, GlobalBytes: 1 << 40, SetsPerImport: 200}
}
func (h harness) artifactPlan(t *testing.T, run domain.Run) domain.ArtifactPlan {
	t.Helper()
	parent, err := h.repo.Get(context.Background(), run.ImportID)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("synthetic artifact"))
	return domain.ArtifactPlan{SourceID: parent.Sources[0].ID, Role: "exam", Stage: "extraction", ComponentVersion: "extract-v1", Manifest: json.RawMessage(`{"schema":"evidence-v1"}`), Files: []domain.ArtifactSpec{
		{Name: "blocks.json", Kind: "source_blocks", ContentType: "application/json", Bytes: 18, SHA256: digest[:]},
		{Name: "page-1.png", Kind: "source_page", ContentType: "image/png", Bytes: 20, SHA256: digest[:]},
	}}
}
func (h harness) reserveArtifacts(t *testing.T, run domain.Run, p domain.ArtifactPlan) domain.ArtifactSet {
	t.Helper()
	out, err := h.repo.ReserveArtifacts(context.Background(), run.Claim(), p, artifactQuotas())
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func (h harness) finishArtifacts(t *testing.T, run domain.Run, set domain.ArtifactSet) domain.ArtifactSet {
	t.Helper()
	for _, f := range set.Files {
		if err := h.repo.ArtifactStored(context.Background(), run.Claim(), set.ID, f.ID); err != nil {
			t.Fatal(err)
		}
	}
	out, err := h.repo.FinishArtifacts(context.Background(), run.Claim(), set.ID)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestArtifactsPublishOnlyCompleteSetsAndRetainImmutableReplay(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	h.schedule(t, version, 3)
	run := h.claim(t, policy(version))
	p := h.artifactPlan(t, run)
	set := h.reserveArtifacts(t, run, p)
	if len(set.Files) != 2 || set.Ready || set.Bytes != 38 {
		t.Fatalf("bad reservation: %+v", set)
	}
	if _, err := h.repo.ArtifactSet(ctx, run.ImportID, set.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("pending set exposed: %v", err)
	}
	if _, err := h.repo.FinishArtifacts(ctx, run.Claim(), set.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("incomplete publish: %v", err)
	}
	if _, err := h.pool.Exec(ctx, `UPDATE app.word_import_artifact_sets SET ready=true,completed_at=now() WHERE id=$1`, set.ID); err == nil {
		t.Fatal("database allowed incomplete set completion")
	}
	replay := h.reserveArtifacts(t, run, p)
	if replay.ID != set.ID || replay.Files[0].StorageKey != set.Files[0].StorageKey {
		t.Fatal("replay duplicated object reservations")
	}
	p.Files[0].Bytes++
	if _, err := h.repo.ReserveArtifacts(ctx, run.Claim(), p, artifactQuotas()); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("replay changed identity: %v", err)
	}
	set = h.finishArtifacts(t, run, set)
	if !set.Ready || set.CompletedAt == nil {
		t.Fatal("finished set unavailable")
	}
	if _, err := h.repo.ArtifactSet(ctx, uuid.NewString(), set.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("cross-import read")
	}
	for _, sql := range []string{`UPDATE app.word_import_artifact_sets SET ready=false,completed_at=NULL WHERE id=$1`, `UPDATE app.word_import_artifacts SET ready=false WHERE set_id=$1`} {
		if _, err := h.pool.Exec(ctx, sql, set.ID); err == nil {
			t.Fatal("completed evidence mutable")
		}
	}
}

func TestArtifactGlobalQuotaOrdersReservationsFromDifferentParents(t *testing.T) {
	h := setup(t)
	other := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	h.schedule(t, version, 3)
	first := h.claim(t, policy(version))
	otherVersion := uuid.NewString()
	other.schedule(t, otherVersion, 3)
	second := other.claim(t, policy(otherVersion))
	q := artifactQuotas()
	if err := h.pool.QueryRow(ctx, `SELECT coalesce(sum(bytes),0)+38 FROM app.word_import_artifact_sets`).Scan(&q.GlobalBytes); err != nil {
		t.Fatal(err)
	}
	p := h.artifactPlan(t, first)
	p2 := other.artifactPlan(t, second)
	start := make(chan struct{})
	done := make(chan error, 2)
	go func() { <-start; _, err := h.repo.ReserveArtifacts(ctx, first.Claim(), p, q); done <- err }()
	go func() { <-start; _, err := other.repo.ReserveArtifacts(ctx, second.Claim(), p2, q); done <- err }()
	close(start)
	success, limited := 0, 0
	for range 2 {
		err := <-done
		if err == nil {
			success++
		} else if errors.Is(err, domain.ErrQuota) {
			limited++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || limited != 1 {
		t.Fatalf("global quota raced: %d/%d", success, limited)
	}
}

func TestArtifactTakeoverRejectsLateWritesAndReusesOnlyCompleteCompatibleEvidence(t *testing.T) {
	h := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	h.schedule(t, version, 3)
	first := h.claim(t, policy(version))
	p := h.artifactPlan(t, first)
	complete := h.finishArtifacts(t, first, h.reserveArtifacts(t, first, p))
	p.Stage = "normalization"
	pending := h.reserveArtifacts(t, first, p)
	h.expire(t, first.ID)
	second := h.claim(t, policy(version))
	for _, err := range []error{
		h.repo.ArtifactStored(ctx, first.Claim(), pending.ID, pending.Files[0].ID),
		func() error { _, err := h.repo.FinishArtifacts(ctx, first.Claim(), pending.ID); return err }(),
		func() error { _, err := h.repo.ReserveArtifacts(ctx, first.Claim(), p, artifactQuotas()); return err }(),
	} {
		if !errors.Is(err, domain.ErrLeaseLost) {
			t.Fatalf("stale artifact write: %v", err)
		}
	}
	reused, err := h.repo.ReusableArtifacts(ctx, second.Claim(), "exam", "extraction", "extract-v1")
	if err != nil || reused.ID != complete.ID {
		t.Fatalf("completed evidence lost: %v", err)
	}
	for _, args := range [][3]string{{"exam", "normalization", "extract-v1"}, {"exam", "extraction", "extract-v2"}, {"answer_key", "extraction", "extract-v1"}} {
		if _, err := h.repo.ReusableArtifacts(ctx, second.Claim(), args[0], args[1], args[2]); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("incompatible/pending reuse: %v", err)
		}
	}
	replacement := h.reserveArtifacts(t, second, p)
	if replacement.ID == pending.ID || replacement.Files[0].StorageKey == pending.Files[0].StorageKey {
		t.Fatal("takeover shares writable old object")
	}
	if err := h.repo.ArtifactStored(ctx, second.Claim(), pending.ID, pending.Files[0].ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("new claim adopted incomplete old set")
	}
	if err := h.repo.Fail(ctx, domain.RunFailure{Claim: second.Claim(), Code: "SYNTHETIC_FAILURE"}); err != nil {
		t.Fatal(err)
	}
	parent, err := h.repo.Get(ctx, second.ImportID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.repo.Schedule(ctx, domain.Schedule{ImportID: parent.ID, RequestID: uuid.NewString(), PipelineVersion: version, ExpectedRevision: parent.Revision, SourceRevision: parent.SourceRevision, Actor: h.actor, MaxAttempts: 3})
	if err != nil {
		t.Fatal(err)
	}
	next := h.claim(t, policy(version))
	reused, err = h.repo.ReusableArtifacts(ctx, next.Claim(), "exam", "extraction", "extract-v1")
	if err != nil || reused.ID != complete.ID {
		t.Fatalf("explicit retry lost reusable evidence: %v", err)
	}
}

func TestArtifactReservationsEnforceSourceOwnershipAndAtomicPendingQuota(t *testing.T) {
	h := setup(t)
	other := setup(t)
	ctx := context.Background()
	version := uuid.NewString()
	h.schedule(t, version, 3)
	run := h.claim(t, policy(version))
	p := h.artifactPlan(t, run)
	otherVersion := uuid.NewString()
	other.schedule(t, otherVersion, 3)
	foreign := other.claim(t, policy(otherVersion))
	foreignPlan := other.artifactPlan(t, foreign)
	wrong := p
	wrong.SourceID = foreignPlan.SourceID
	if _, err := h.repo.ReserveArtifacts(ctx, run.Claim(), wrong, artifactQuotas()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("foreign source accepted: %v", err)
	}
	wrong = p
	wrong.Role = "answer_key"
	if _, err := h.repo.ReserveArtifacts(ctx, run.Claim(), wrong, artifactQuotas()); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("source role mismatch: %v", err)
	}
	q := artifactQuotas()
	q.ActorBytes = 38
	start := make(chan struct{})
	done := make(chan error, 2)
	for _, stage := range []string{"extraction", "normalization"} {
		go func() {
			<-start
			in := p
			in.Stage = stage
			_, err := h.repo.ReserveArtifacts(ctx, run.Claim(), in, q)
			done <- err
		}()
	}
	close(start)
	passed, limited := 0, 0
	for range 2 {
		err := <-done
		if err == nil {
			passed++
		} else if errors.Is(err, domain.ErrQuota) {
			limited++
		} else {
			t.Fatal(err)
		}
	}
	if passed != 1 || limited != 1 {
		t.Fatalf("non-atomic pending quota: %d/%d", passed, limited)
	}
	for _, table := range []string{"word_import_artifact_sets", "word_import_artifacts"} {
		var mutable bool
		if err := h.pool.QueryRow(ctx, `SELECT has_table_privilege('quizzivy_app',$1,'UPDATE,DELETE')`, "app."+table).Scan(&mutable); err != nil || mutable {
			t.Fatalf("artifact grants: %v", err)
		}
	}
}
