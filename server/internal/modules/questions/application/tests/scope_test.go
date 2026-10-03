//go:build integration

package application_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"quizzivy/internal/core/adapters"
	mediaapp "quizzivy/internal/modules/media/application"
	mediarepo "quizzivy/internal/modules/media/repositories"
	"quizzivy/internal/modules/questions/application"
	"quizzivy/internal/modules/questions/application/command"
	"quizzivy/internal/modules/questions/application/query"
	"quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/questions/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

type bankWorld struct {
	tx          pgx.Tx
	svc         *application.Application
	a, b, admin string
	marker      string
}

func newBankWorld(t *testing.T) *bankWorld {
	t.Helper()
	pool := newPool(t)
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	kinds := adapters.MediaKinds{Media: mediaapp.New(mediarepo.NewPostgres(db.NewContext(tx)), nil, nil)}
	w := &bankWorld{tx: tx, svc: application.New(repositories.NewPostgres(db.NewContext(tx)), kinds), marker: "pham-vi-" + uuid.NewString()[:8]}
	w.a, w.b, w.admin = w.user(t, "teacher"), w.user(t, "teacher"), w.user(t, "admin")
	return w
}

func (w *bankWorld) id(t *testing.T, sql string, args ...any) string {
	t.Helper()
	var id string
	if err := w.tx.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (w *bankWorld) user(t *testing.T, builtin string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.users (email, full_name, role_id) VALUES ($1, 'Bank scope', (SELECT id FROM app.roles WHERE builtin_key = $2)) RETURNING id::text`,
		uuid.NewString()+"@example.test", builtin)
}

func (w *bankWorld) image(t *testing.T, owner string) string {
	t.Helper()
	return w.id(t, `INSERT INTO app.media_assets (kind, storage_key, mime_type, bytes, original_filename, checksum_sha256, uploaded_by, owner_id)
		VALUES ('image', $1, 'image/png', 1, 'hinh.png', sha256(convert_to($1, 'UTF8')), $2, $2) RETURNING id::text`, "scope/"+uuid.NewString(), owner)
}

func (w *bankWorld) draftUsing(t *testing.T, owner, question string) string {
	t.Helper()
	test := w.id(t, `INSERT INTO app.tests (title, created_by, owner_id) VALUES ($1, $2, $2) RETURNING id::text`, "Đề của "+owner, owner)
	section := w.id(t, `INSERT INTO app.test_sections (test_id, ordinal, title) VALUES ($1, 0, 'Phần 1') RETURNING id::text`, test)
	if _, err := w.tx.Exec(context.Background(), `INSERT INTO app.test_section_questions (test_section_id, ordinal, question_id) VALUES ($1, 0, $2)`, section, question); err != nil {
		t.Fatal(err)
	}
	return test
}

func request(actor string, all bool, id string, in domain.Input) domain.WriteRequest {
	return domain.WriteRequest{ID: id, Input: in, ActorID: actor, All: all}
}

func (w *bankWorld) input(prompt string, asset *string) domain.Input {
	return domain.Input{Type: domain.ShortAnswer, Prompt: prompt, Points: "1.00", Tags: []string{w.marker, w.marker + "-" + prompt}, MediaAssetID: asset}
}

func (w *bankWorld) create(t *testing.T, actor, prompt string, asset *string) domain.Question {
	t.Helper()
	q, err := w.svc.Commands.Create.Handle(context.Background(), command.Create{Request: request(actor, false, "", w.input(prompt, asset))})
	if err != nil {
		t.Fatalf("%s creating %q: %v", actor, prompt, err)
	}
	return q
}

func (w *bankWorld) get(scope access.Scope, id string) (domain.Question, error) {
	return w.svc.Queries.Get.Handle(context.Background(), query.Get{ID: id, Scope: scope})
}

func (w *bankWorld) owner(t *testing.T, id string) (owner, deletedAt, tags string) {
	t.Helper()
	if err := w.tx.QueryRow(context.Background(), `SELECT owner_id::text, coalesce(deleted_at::text, ''), array_to_string(tags, ',') FROM app.questions WHERE id = $1`, id).
		Scan(&owner, &deletedAt, &tags); err != nil {
		t.Fatal(err)
	}
	return owner, deletedAt, tags
}

func questionIDs(questions []domain.Question) []string {
	out := make([]string, len(questions))
	for i, q := range questions {
		out[i] = q.ID
	}
	slices.Sort(out)
	return out
}

func sorted(ids ...string) []string {
	out := slices.Clone(ids)
	slices.Sort(out)
	return out
}

func TestACreatedQuestionBelongsToItsAuthor(t *testing.T) {
	w := newBankWorld(t)
	q := w.create(t, w.b, "cua-b", nil)
	if owner, _, _ := w.owner(t, q.ID); owner != w.b {
		t.Errorf("B's new question belongs to %s", owner)
	}
}

func TestTheBankListsFacetsAndCountsOnlyTheCallersOwnQuestions(t *testing.T) {
	w := newBankWorld(t)
	a1, a2 := w.create(t, w.a, "a1", nil), w.create(t, w.a, "a2", nil)
	b1 := w.create(t, w.b, "b1", nil)
	ctx := context.Background()

	for name, c := range map[string]struct {
		scope access.Scope
		want  []string
	}{
		"A":              {access.Scope{UserID: w.a}, sorted(a1.ID, a2.ID)},
		"B":              {access.Scope{UserID: w.b}, sorted(b1.ID)},
		"the zero scope": {access.Scope{}, []string{}},
		"scope.all":      {access.Scope{UserID: w.admin, All: true}, sorted(a1.ID, a2.ID, b1.ID)},
	} {
		t.Run(name, func(t *testing.T) {
			in := domain.ListInput{Tags: []string{w.marker}, Limit: repositories.MaxLimit, Scope: c.scope}
			listed, err := w.svc.Queries.List.Handle(ctx, query.List{Input: in})
			if err != nil {
				t.Fatal(err)
			}
			if got := questionIDs(listed.Items); !slices.Equal(got, c.want) || listed.Page.Total != len(c.want) {
				t.Errorf("lists %v with total %d, want %v", got, listed.Page.Total, c.want)
			}
			facets, err := w.svc.Queries.Facets.Handle(ctx, query.Facets{Input: in})
			if err != nil || facets.All != len(c.want) {
				t.Errorf("facets count %d (%v), want %d", facets.All, err, len(c.want))
			}
			counts, err := w.svc.Queries.Counts.Handle(ctx, query.Counts{Input: in})
			if err != nil || counts.Filtered != len(c.want) {
				t.Errorf("filtered count %d (%v), want %d", counts.Filtered, err, len(c.want))
			}
			if !c.scope.All && (counts.Total != len(c.want)) {
				t.Errorf("the bank total is %d, want the caller's own %d", counts.Total, len(c.want))
			}
			rail, err := w.svc.Queries.Tags.Handle(ctx, query.Tags{Input: in})
			if err != nil {
				t.Fatal(err)
			}
			for _, q := range []domain.Question{a1, a2, b1} {
				own := slices.Contains(c.want, q.ID)
				if slices.Contains(rail, q.Tags[1]) != own {
					t.Errorf("the tag rail offers %q: %v, want %v", q.Tags[1], !own, own)
				}
			}
		})
	}
}

func TestAnotherTeachersQuestionAnswersAsAMissingOne(t *testing.T) {
	w := newBankWorld(t)
	ctx := context.Background()
	theirs := w.create(t, w.a, "cua-a", nil)
	w.draftUsing(t, w.a, theirs.ID)
	mine := w.create(t, w.b, "cua-b", nil)

	for label, id := range map[string]string{"A's": theirs.ID, "a missing": uuid.NewString()} {
		if _, err := w.get(access.Scope{UserID: w.b}, id); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B opening %s question: %v, want not found", label, err)
		}
		if _, err := w.svc.Commands.Update.Handle(ctx, command.Update{Request: request(w.b, false, id, w.input("sua", nil))}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B updating %s question: %v, want not found", label, err)
		}
		if _, err := w.svc.Commands.Delete.Handle(ctx, command.Delete{Request: request(w.b, false, id, domain.Input{})}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B deleting %s question, which A's draft uses: %v, want not found and never the drafts", label, err)
		}
		if _, err := w.svc.Commands.Duplicate.Handle(ctx, command.Duplicate{Request: request(w.b, false, id, domain.Input{})}); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("B duplicating %s question: %v, want not found", label, err)
		}
	}
	tagged, err := w.svc.Commands.AddTags.Handle(ctx, command.AddTags{IDs: []string{theirs.ID, mine.ID}, Tags: []string{"the-cua-b"}, Scope: access.Scope{UserID: w.b}})
	if err != nil || tagged != 1 {
		t.Errorf("B tagging A's and B's questions tagged %d (%v), want only B's", tagged, err)
	}
	if owner, deletedAt, tags := w.owner(t, theirs.ID); owner != w.a || deletedAt != "" || slices.Contains(strings.Split(tags, ","), "the-cua-b") || strings.Contains(tags, "-sua") {
		t.Errorf("A's question now belongs to %s, deleted %q, tags %q", owner, deletedAt, tags)
	}
	var audited int
	if err := w.tx.QueryRow(ctx, `SELECT count(*) FROM app.audit_log WHERE actor_user_id = $1 AND entity_id = $2`, w.b, theirs.ID).Scan(&audited); err != nil || audited != 0 {
		t.Errorf("B's refused writes left %d audit rows (%v)", audited, err)
	}

	var refused *domain.ReferencedError
	if _, err := w.svc.Commands.Delete.Handle(ctx, command.Delete{Request: request(w.a, false, theirs.ID, domain.Input{})}); !errors.As(err, &refused) || len(refused.Tests) != 1 {
		t.Errorf("A deleting A's question in A's draft: %v, want the refusal naming A's draft", err)
	}
}

func TestScopeAllReachesEveryQuestionWithoutTakingIt(t *testing.T) {
	w := newBankWorld(t)
	ctx := context.Background()
	anyone := access.Scope{UserID: w.admin, All: true}
	asset := w.image(t, w.a)
	theirs := w.create(t, w.a, "cua-a", &asset)

	if _, err := w.get(anyone, theirs.ID); err != nil {
		t.Errorf("scope.all opening A's question: %v", err)
	}
	if _, err := w.svc.Commands.Update.Handle(ctx, command.Update{Request: request(w.admin, true, theirs.ID, w.input("sua-boi-admin", &asset))}); err != nil {
		t.Errorf("scope.all updating A's question: %v", err)
	}
	if tagged, err := w.svc.Commands.AddTags.Handle(ctx, command.AddTags{IDs: []string{theirs.ID}, Tags: []string{"admin-tag"}, Scope: anyone}); err != nil || tagged != 1 {
		t.Errorf("scope.all tagging A's question tagged %d (%v)", tagged, err)
	}
	if owner, _, _ := w.owner(t, theirs.ID); owner != w.a {
		t.Errorf("A's question belongs to %s after scope.all edited it", owner)
	}
	copied, err := w.svc.Commands.Duplicate.Handle(ctx, command.Duplicate{Request: request(w.admin, true, theirs.ID, domain.Input{})})
	if err != nil {
		t.Fatalf("scope.all duplicating A's question: %v", err)
	}
	if owner, _, _ := w.owner(t, copied.ID); owner != w.admin || copied.MediaAssetID == nil || *copied.MediaAssetID != asset {
		t.Errorf("the copy belongs to %s bound to %v, want the actor %s and A's asset", owner, copied.MediaAssetID, w.admin)
	}
	if _, err := w.svc.Commands.Delete.Handle(ctx, command.Delete{Request: request(w.admin, true, theirs.ID, domain.Input{})}); err != nil {
		t.Errorf("scope.all deleting A's question: %v", err)
	}
}

func TestUsedInNamesOnlyTestsTheViewerCanRead(t *testing.T) {
	w := newBankWorld(t)
	q := w.create(t, w.a, "dung-chung", nil)
	own := w.draftUsing(t, w.a, q.ID)
	foreign := w.draftUsing(t, w.b, q.ID)

	for name, c := range map[string]struct {
		scope access.Scope
		want  []string
	}{
		"the owner": {access.Scope{UserID: w.a}, []string{own}},
		"scope.all": {access.Scope{UserID: w.admin, All: true}, sorted(own, foreign)},
	} {
		got, err := w.get(c.scope, q.ID)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, ref := range got.UsedIn {
			ids = append(ids, ref.ID)
		}
		slices.Sort(ids)
		if !slices.Equal(ids, c.want) {
			t.Errorf("%s sees it used in %v, want %v", name, ids, c.want)
		}
	}
}

func TestAQuestionBindsOnlyAssetsItsWriterCanRead(t *testing.T) {
	w := newBankWorld(t)
	ctx := context.Background()
	theirs := w.image(t, w.a)
	missing := uuid.NewString()
	mine := w.create(t, w.b, "cua-b", nil)

	for label, asset := range map[string]string{"A's": theirs, "a missing": missing} {
		in := w.input("gan-"+label, &asset)
		if _, err := w.svc.Commands.Create.Handle(ctx, command.Create{Request: request(w.b, false, "", in)}); !errors.Is(err, domain.ErrMediaNotFound) {
			t.Errorf("B creating with %s asset: %v, want the media not-found", label, err)
		}
		if _, err := w.svc.Commands.Update.Handle(ctx, command.Update{Request: request(w.b, false, mine.ID, in)}); !errors.Is(err, domain.ErrMediaNotFound) {
			t.Errorf("B updating to %s asset: %v, want the media not-found", label, err)
		}
		in.Audio = &domain.AudioPolicy{AllowSeek: true}
		if _, err := w.svc.Commands.Create.Handle(ctx, command.Create{Request: request(w.b, false, "", in)}); !errors.Is(err, domain.ErrMediaNotFound) {
			t.Errorf("B binding %s image with an audio policy: %v, want the media not-found and no check of its kind", label, err)
		}
	}
	own := strings.ToUpper(w.image(t, w.b))
	w.create(t, w.b, "anh-cua-b", &own)

	if _, err := w.svc.Commands.Update.Handle(ctx, command.Update{Request: request(w.admin, true, mine.ID, w.input("cua-b", &theirs))}); err != nil {
		t.Fatalf("scope.all binding A's asset into B's question: %v", err)
	}
	if _, err := w.svc.Commands.Update.Handle(ctx, command.Update{Request: request(w.b, false, mine.ID, w.input("cua-b-sua", &theirs))}); err != nil {
		t.Errorf("B re-saving the question with the asset scope.all bound: %v", err)
	}
	if _, err := w.svc.Commands.Duplicate.Handle(ctx, command.Duplicate{Request: request(w.b, false, mine.ID, domain.Input{})}); err != nil {
		t.Errorf("B duplicating that question: %v", err)
	}
}
