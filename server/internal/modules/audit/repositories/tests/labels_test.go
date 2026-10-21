//go:build integration

package repositories_test

import (
	"context"
	"testing"

	"quizzivy/internal/modules/audit/domain"
)

func TestEveryKindOfRowIsNamedByItsOwnLabel(t *testing.T) {
	w := newWorld(t)
	f := newFixtures(w)
	var refs []domain.EntityRef
	want := map[domain.EntityRef]string{}
	for name, fixture := range f.all() {
		label := "Nhãn " + name + " " + w.nonce
		ref := domain.EntityRef{Type: fixture.entity, ID: fixture.id(label)}
		refs = append(refs, ref)
		want[ref] = label
	}

	got, err := w.repo.Labels(context.Background(), refs)
	if err != nil {
		t.Fatal(err)
	}

	for _, ref := range refs {
		if got[ref] != want[ref] {
			t.Errorf("%s: label %q, want %q", describe(ref.ID, ref.Type), got[ref], want[ref])
		}
	}
	if len(got) != len(want) {
		t.Errorf("%d labels for %d rows", len(got), len(want))
	}
}

func TestMediaIsNamedByItsDisplayNameElseItsFile(t *testing.T) {
	w := newWorld(t)
	f := newFixtures(w)
	shown := f.media("Ảnh minh hoạ", "raw-name.png")
	bare := f.media(nil, "raw-name-only.png")
	refs := []domain.EntityRef{{Type: "media_asset", ID: shown}, {Type: "media_asset", ID: bare}}

	got, err := w.repo.Labels(context.Background(), refs)
	if err != nil {
		t.Fatal(err)
	}

	if got[refs[0]] != "Ảnh minh hoạ" || got[refs[1]] != "raw-name-only.png" {
		t.Errorf("labels %v", got)
	}
}

func TestARowThatIsGoneHasNoLabel(t *testing.T) {
	w := newWorld(t)
	teacher := w.user("teacher", "Giáo viên "+w.nonce, "")
	class := w.class("Lớp sắp xoá", teacher)
	w.exec(`DELETE FROM app.classes WHERE id = $1::uuid`, class)
	gone := domain.EntityRef{Type: "class", ID: class}
	neverThere := domain.EntityRef{Type: "test", ID: "0195c4b0-0000-7000-8000-00000000dead"}

	got, err := w.repo.Labels(context.Background(), []domain.EntityRef{gone, neverThere})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 0 {
		t.Errorf("labels %v, want none", got)
	}
}

func TestAnEntityNoSourceNamesHasNoLabelAndAsksNoQuery(t *testing.T) {
	w := newWorld(t)
	got, err := w.repo.Labels(context.Background(), []domain.EntityRef{
		{Type: "maintenance_window", ID: "0195c4b0-0000-7000-8000-000000000001"},
		{Type: "question", ID: "0195c4b0-0000-7000-8000-000000000002"},
		{Type: "refresh_token_family", ID: "0195c4b0-0000-7000-8000-000000000003"},
	})
	if err != nil || len(got) != 0 {
		t.Errorf("labels %v, %v", got, err)
	}
	if empty, err := w.repo.Labels(context.Background(), nil); err != nil || len(empty) != 0 {
		t.Errorf("no refs: %v, %v", empty, err)
	}
}

func TestATestStaysNamedAfterItIsDeleted(t *testing.T) {
	w := newWorld(t)
	f := newFixtures(w)
	id := f.test("Đề đã xoá mềm " + w.nonce)
	w.exec(`UPDATE app.tests SET deleted_at = now() WHERE id = $1::uuid`, id)
	ref := domain.EntityRef{Type: "test", ID: id}

	got, err := w.repo.Labels(context.Background(), []domain.EntityRef{ref})
	if err != nil {
		t.Fatal(err)
	}
	if got[ref] != "Đề đã xoá mềm "+w.nonce {
		t.Errorf("label %q: an audit entry about a deleted test should still name it", got[ref])
	}
}

func TestOneRowTwoEntitiesBothGetItsName(t *testing.T) {
	w := newWorld(t)
	f := newFixtures(w)
	id := f.all()["class"].id("Lớp dùng chung " + w.nonce)
	asClass := domain.EntityRef{Type: "class", ID: id}
	asMember := domain.EntityRef{Type: "class_member", ID: id}

	got, err := w.repo.Labels(context.Background(), []domain.EntityRef{asClass, asMember})
	if err != nil {
		t.Fatal(err)
	}
	if got[asClass] != "Lớp dùng chung "+w.nonce || got[asMember] != got[asClass] {
		t.Errorf("labels %v", got)
	}
}
