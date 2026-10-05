//go:build integration

package application_test

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"strings"
	"testing"
)

func TestReplacementPoolRepointsOnlyEditableOwnedGraphAndCountsDistinctGroups(t *testing.T) {
	w := newReplacementWorld(t, nil)
	old := w.asset(t, w.a, domain.KindImage, 20)
	standalone := w.question(t, w.a, old, domain.KindImage, nil)
	deleted := w.question(t, w.a, old, domain.KindImage, nil)
	w.exec(t, `UPDATE app.questions SET deleted_at=now() WHERE id=$1`, deleted)
	bank := w.group(t, w.a, nil)
	member := w.question(t, w.a, old, domain.KindImage, &bank)
	stimulus := w.stimulus(t, bank, old)
	archived := w.group(t, w.a, nil)
	w.exec(t, `UPDATE app.question_groups SET archived_at=now() WHERE id=$1`, archived)
	w.stimulus(t, archived, old)
	parent, section := w.section(t, w.a)
	draft := w.group(t, w.a, &section)
	w.stimulus(t, draft, old)
	w.exec(t, `UPDATE app.tests SET status='archived' WHERE id=$1`, parent)
	removedParent, removedSection := w.section(t, w.a)
	excluded := w.group(t, w.a, &removedSection)
	w.stimulus(t, excluded, old)
	excludedQ := w.question(t, w.a, old, domain.KindImage, &excluded)
	w.exec(t, `UPDATE app.tests SET deleted_at=now() WHERE id=$1`, removedParent)
	foreign := w.group(t, w.b, nil)
	w.stimulus(t, foreign, old)
	foreignQ := w.question(t, w.b, old, domain.KindImage, &foreign)
	foreignStandalone := w.question(t, w.b, old, domain.KindImage, nil)
	in := w.input(old, w.a, domain.KindImage, 30, 30)
	out, err := w.repo.Replace(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Repointed != (domain.ReplacementCounts{Questions: 2, Groups: 3}) || out.Left != (domain.ReplacementCounts{Questions: 2, Groups: 1}) {
		t.Fatalf("counts repointed=%+v left=%+v", out.Repointed, out.Left)
	}
	for _, id := range []string{standalone, member} {
		if got := w.text(t, `SELECT media_asset_id::text FROM app.questions WHERE id=$1`, id); got != out.Asset.ID {
			t.Fatalf("owned question=%s", got)
		}
	}
	for _, id := range []string{deleted, excludedQ, foreignQ, foreignStandalone} {
		if got := w.text(t, `SELECT media_asset_id::text FROM app.questions WHERE id=$1`, id); got != old {
			t.Fatalf("ineligible question=%s", got)
		}
	}
	for _, id := range []string{bank, archived, draft} {
		if got := w.scalar(t, `SELECT revision FROM app.question_groups WHERE id=$1`, id); got != 2 {
			t.Fatalf("revision=%d", got)
		}
	}
	for _, id := range []string{foreign, excluded} {
		if got := w.scalar(t, `SELECT revision FROM app.question_groups WHERE id=$1`, id); got != 1 {
			t.Fatalf("ineligible revision=%d", got)
		}
	}
	raw := w.text(t, `SELECT content::text FROM app.group_stimuli WHERE id=$1`, stimulus)
	if !strings.Contains(raw, out.Asset.ID) || strings.Contains(raw, old) || !strings.Contains(raw, "Giữ nguyên") || !strings.Contains(raw, "bold") {
		t.Fatalf("content=%s", raw)
	}
	if owner := w.text(t, `SELECT owner_id::text FROM app.media_assets WHERE id=$1`, out.Asset.ID); owner != w.a {
		t.Fatalf("owner=%s", owner)
	}
	if uploader := w.text(t, `SELECT uploaded_by::text FROM app.media_assets WHERE id=$1`, out.Asset.ID); uploader != w.actor {
		t.Fatalf("uploader=%s", uploader)
	}
	if got := w.text(t, `SELECT replaced_by::text FROM app.media_assets WHERE id=$1`, old); got != out.Asset.ID {
		t.Fatalf("replacement link=%s", got)
	}
	if out.Asset.DisplayName != "Tên gốc" || out.Asset.DefaultMaxPlays != nil {
		t.Fatalf("metadata=%+v", out.Asset)
	}
	if _, err := w.repo.FindReplacementTarget(context.Background(), access.Scope{All: true}, old); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("replaced target=%v", err)
	}
	if got, err := w.repo.Get(context.Background(), old); err != nil || got.ID != old {
		t.Fatalf("old immutable get=%+v error=%v", got, err)
	}
	if got := w.scalar(t, `SELECT count(*) FROM app.audit_log WHERE entity_id=$1 AND action='media.replaced' AND actor_user_id=$2`, old, w.actor); got != 1 {
		t.Fatalf("audit=%d", got)
	}
}
func TestReplacementPoolCopiesLockedAudioPolicyAndRecordingIdentity(t *testing.T) {
	w := newReplacementWorld(t, nil)
	old := w.asset(t, w.a, domain.KindAudio, 20)
	g := w.group(t, w.a, nil)
	recording := w.id(t, `INSERT INTO app.group_recordings(group_id,media_asset_id,allow_seek,show_transcript_after_submit) VALUES($1,$2,true,false) RETURNING id::text`, g, old)
	w.question(t, w.a, old, domain.KindAudio, &g)
	out, err := w.repo.Replace(context.Background(), w.input(old, w.a, domain.KindAudio, 30, 30))
	if err != nil {
		t.Fatal(err)
	}
	if out.Repointed != (domain.ReplacementCounts{Questions: 1, Groups: 1}) || out.Asset.DefaultMaxPlays == nil || *out.Asset.DefaultMaxPlays != 3 {
		t.Fatalf("result=%+v", out)
	}
	if got := w.text(t, `SELECT media_asset_id::text FROM app.group_recordings WHERE id=$1 AND allow_seek AND NOT show_transcript_after_submit`, recording); got != out.Asset.ID {
		t.Fatalf("recording=%s", got)
	}
	if revision := w.scalar(t, `SELECT revision FROM app.question_groups WHERE id=$1`, g); revision != 2 {
		t.Fatalf("revision=%d", revision)
	}
}
func TestReplacementPoolKnownRefusalsRollBackAllRowsAndLinks(t *testing.T) {
	for _, reason := range []string{"foreign", "deleted", "kind", "quota"} {
		t.Run(reason, func(t *testing.T) {
			w := newReplacementWorld(t, nil)
			old := w.asset(t, w.a, domain.KindImage, 20)
			in := w.input(old, w.a, domain.KindImage, 30, 30)
			want := domain.ErrNotFound
			switch reason {
			case "foreign":
				in.Scope = access.Scope{UserID: w.b}
			case "deleted":
				w.exec(t, `UPDATE app.media_assets SET deleted_at=now() WHERE id=$1`, old)
			case "kind":
				in.Asset.Kind = domain.KindAudio
				in.Asset.DurationMs = new(1000)
				want = domain.ErrKindMismatch
			case "quota":
				in.Asset.QuotaBytes = 29
				want = domain.ErrQuotaExceeded
			}
			_, err := w.repo.Replace(context.Background(), in)
			var failure *domain.ReplacementError
			if !errors.Is(err, want) || !errors.As(err, &failure) || failure.Outcome != domain.ReplacementNotCommitted || failure.RollbackError != nil {
				t.Fatalf("refusal=%v", err)
			}
			if rows := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE id=$1`, in.Asset.ID); rows != 0 {
				t.Fatalf("new rows=%d", rows)
			}
			if links := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE id=$1 AND replaced_by IS NOT NULL`, old); links != 0 {
				t.Fatalf("links=%d", links)
			}
		})
	}
}

type replacementPoolWrapper struct{ db.Conn }

func TestReplacementPoolBoundaryRejectsRawAcquiredTransactionAndWrapperBeforeBegin(t *testing.T) {
	w := newReplacementWorld(t, nil)
	old := w.asset(t, w.a, domain.KindImage, 20)
	ctx := context.Background()
	acquired, err := w.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer acquired.Release()
	tx, err := acquired.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Error(err)
		}
	}()
	for _, c := range []db.Conn{acquired.Conn(), acquired, tx, replacementPoolWrapper{Conn: w.pool}} {
		in := w.input(old, w.a, domain.KindImage, 30, 30)
		_, err := repositories.NewPostgres(db.NewContext(c)).Replace(ctx, in)
		var failure *domain.ReplacementError
		if !errors.As(err, &failure) || failure.Outcome != domain.ReplacementNotCommitted || failure.RollbackError != nil {
			t.Fatalf("boundary %T=%v", c, err)
		}
		if got := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE id=$1`, in.Asset.ID); got != 0 {
			t.Fatalf("boundary wrote %d rows", got)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if linked := w.scalar(t, `SELECT count(*) FROM app.media_assets WHERE id=$1 AND replaced_by IS NOT NULL`, old); linked != 0 {
		t.Fatal("nested refusal changed old link")
	}
}
