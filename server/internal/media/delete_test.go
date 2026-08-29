package media_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/media"
)

// TestDeleteSoftDeletesAndAudits: the row survives, the object survives, the
// asset leaves the library, and the teacher's action is recorded (§13.2, §15).
func TestDeleteSoftDeletesAndAudits(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	objects := newFakeStore()
	svc := media.NewService(media.NewStore(pool), objects)
	asset := upload(t, svc, uploader, "xoa.mp3")
	ctx := context.Background()

	if err := svc.Delete(ctx, media.DeleteInput{ID: asset.ID, ActorID: uploader}); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var deletedAt *string
	if err := pool.QueryRow(ctx,
		`SELECT deleted_at::text FROM app.media_assets WHERE id = $1`, asset.ID).Scan(&deletedAt); err != nil {
		t.Fatalf("row was hard-deleted, but soft delete was expected: %v", err)
	}
	if deletedAt == nil {
		t.Error("deleted_at is still null")
	}
	if len(objects.deletes) != 0 {
		t.Errorf("the stored object was removed: %v", objects.deletes)
	}

	var audited int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.audit_log
		  WHERE action = 'media.deleted' AND entity_id = $1 AND actor_user_id = $2`,
		asset.ID, uploader).Scan(&audited); err != nil {
		t.Fatal(err)
	}
	if audited != 1 {
		t.Errorf("audit rows for the delete: %d, want 1", audited)
	}

	// Gone from the library.
	assets, _, err := svc.List(ctx, media.ListInput{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range assets {
		if a.ID == asset.ID {
			t.Error("a deleted asset is still listed")
		}
	}
}

// TestDeleteTwiceIsNotFound: a stale library entry gets told the list is stale,
// rather than a silent success that looks like it deleted something.
func TestDeleteTwiceIsNotFound(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := media.NewService(media.NewStore(pool), newFakeStore())
	asset := upload(t, svc, uploader, "hai-lan.mp3")
	ctx := context.Background()

	if err := svc.Delete(ctx, media.DeleteInput{ID: asset.ID, ActorID: uploader}); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	err := svc.Delete(ctx, media.DeleteInput{ID: asset.ID, ActorID: uploader})
	if !errors.Is(err, media.ErrNotFound) {
		t.Errorf("second delete: %v, want ErrNotFound", err)
	}

	var audited int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM app.audit_log WHERE action = 'media.deleted' AND entity_id = $1`,
		asset.ID).Scan(&audited); err != nil {
		t.Fatal(err)
	}
	if audited != 1 {
		t.Errorf("the refused delete still wrote an audit row: %d rows", audited)
	}
}

// TestMintForStudentDeniesByDefault is the authorization property of §11.2: a
// student may not mint a URL for an asset merely by knowing its id.
//
// It passes today because ReachableByStudent denies everything, which is both
// fail-closed and, with no attempts table, exactly right. It keeps its meaning
// once that function is implemented: this student has no attempt on anything,
// so the answer must still be no.
func TestMintForStudentDeniesByDefault(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := media.NewService(media.NewStore(pool), newFakeStore())
	asset := upload(t, svc, uploader, "cua-nguoi-khac.mp3")

	student := makeStudent(t, pool)
	_, err := svc.MintForStudent(context.Background(), student, asset.ID)
	if !errors.Is(err, media.ErrForbidden) {
		t.Errorf("a student minted a URL for an asset they cannot reach: %v", err)
	}
}

// TestMintForStudentHidesWhetherTheAssetExists: the same answer for a real
// asset and a made-up id, so the endpoint is not an oracle for valid ids.
func TestMintForStudentHidesWhetherTheAssetExists(t *testing.T) {
	pool := newPool(t)
	svc := media.NewService(media.NewStore(pool), newFakeStore())
	student := makeStudent(t, pool)

	_, err := svc.MintForStudent(context.Background(), student,
		"00000000-0000-7000-8000-000000000000")
	if !errors.Is(err, media.ErrForbidden) {
		t.Errorf("a nonexistent asset answered %v, want the same ErrForbidden a real one gets", err)
	}
}
