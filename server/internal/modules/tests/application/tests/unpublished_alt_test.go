//go:build integration

package application_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"quizzivy/internal/modules/tests/application/query"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func altAuthorWithImage(t *testing.T, pool *pgxpool.Pool) (author, asset string) {
	t.Helper()
	t.Cleanup(func() {
		if author == "" {
			return
		}
		for _, statement := range []string{
			`DELETE FROM app.audit_log WHERE actor_user_id = $1`,
			`DELETE FROM app.test_versions WHERE published_by = $1`,
			`DELETE FROM app.tests WHERE created_by = $1`,
			`DELETE FROM app.questions WHERE created_by = $1`,
			`DELETE FROM app.media_assets WHERE uploaded_by = $1`,
			`DELETE FROM app.users WHERE id = $1`,
		} {
			if _, err := pool.Exec(context.Background(), statement, author); err != nil {
				t.Errorf("cleanup %q: %v", statement, err)
			}
		}
	})
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO app.users (email, full_name, role_id)
		 VALUES ($1,'Giáo viên',(SELECT id FROM app.roles WHERE builtin_key = 'teacher')) RETURNING id::text`,
		"alt-"+hex.EncodeToString(nonce)+"@example.com").Scan(&author); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(context.Background(), `INSERT INTO app.media_assets
		(kind,storage_key,mime_type,bytes,original_filename,checksum_sha256,uploaded_by,owner_id)
		VALUES ('image',$1,'image/png',100,'meo.png',$2,$3,$3) RETURNING id::text`,
		"image/alt-"+hex.EncodeToString(nonce), make([]byte, 32), author).Scan(&asset); err != nil {
		t.Fatal(err)
	}
	return author, asset
}

func TestAnAltTextEditRaisesTheUnpublishedChangesAndTheDiffNamesTheMedia(t *testing.T) {
	pool := newPool(t)
	author, asset := altAuthorWithImage(t, pool)
	b := newBuilder(t, pool, author)
	original := "Một chú mèo ngồi trên ghế"
	question := b.question(imageInput(asset, &original))
	draft := b.draft("Đề có ảnh", question)
	b.mustPublish(draft.ID)
	if got := b.get(draft.ID).UnpublishedChanges; got == nil || *got != 0 {
		t.Fatalf("a draft just published counts %v, want 0", got)
	}

	for _, step := range []struct {
		name string
		alt  *string
		want int
	}{
		{"edited", groupValue("Một chú chó nằm trên thảm"), 1},
		{"cleared", nil, 1},
		{"back to what was published", &original, 0},
	} {
		b.setAlt(question, asset, step.alt)
		got := b.get(draft.ID).UnpublishedChanges
		if got == nil || *got != step.want {
			t.Fatalf("alt text %s: unpublishedChanges = %v, want %d", step.name, got, step.want)
		}
	}

	b.setAlt(question, asset, groupValue("Một chú chó nằm trên thảm"))
	changes := b.mustDiff(draft.ID, 1, againstDraft).Changes
	if len(changes) != 1 {
		t.Fatalf("changes = %+v, want one", changes)
	}
	if c := changes[0]; c.Kind != domain.ChangeChanged || c.QuestionID != question || !reflect.DeepEqual(c.Fields, []domain.ChangedField{domain.FieldMedia}) {
		t.Fatalf("change = %+v, want the question changed in its media alone", c)
	}

	b.mustPublish(draft.ID)
	if got := b.get(draft.ID).UnpublishedChanges; got == nil || *got != 0 {
		t.Fatalf("after publishing the edit the draft counts %v, want 0", got)
	}
	for version, want := range map[int]string{1: original, 2: "Một chú chó nằm trên thảm"} {
		preview, err := b.tests.Queries.Preview.Handle(context.Background(), query.Preview{TestID: draft.ID, Version: version, Scope: access.Scope{UserID: author}})
		if err != nil {
			t.Fatal(err)
		}
		if len(preview.Questions) != 1 {
			t.Fatalf("version %d previews %d questions, want 1", version, len(preview.Questions))
		}
		wantAlt(t, fmt.Sprintf("the preview of version %d", version), preview.Questions[0].MediaAlt, want)
	}
}
