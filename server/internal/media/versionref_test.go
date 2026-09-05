package media_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/media"
)

// publishVersionUsing freezes a version question that names the asset, taking
// the lock the publish routine must take.
func publishVersionUsing(ctx context.Context, pool *pgxpool.Pool, author, assetID string, lock bool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if lock {
		if err := media.LockForVersionUse(ctx, tx, assetID); err != nil {
			return err
		}
	}

	var testID, versionID, sectionID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.tests (title, status, current_version, created_by)
		 VALUES ('Đề đã xuất bản', 'published', 1, $1) RETURNING id::text`, author).Scan(&testID); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.test_versions (test_id, version, total_points, published_by)
		 VALUES ($1, 1, 10.00, $2) RETURNING id::text`, testID, author).Scan(&versionID); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.test_version_sections (test_version_id, ordinal, title)
		 VALUES ($1, 0, 'Phần nghe') RETURNING id::text`, versionID).Scan(&sectionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO app.test_version_questions
		        (test_version_section_id, ordinal, type, prompt, points,
		         media_asset_id, media_asset_kind, audio_allow_seek, audio_show_transcript_after)
		 VALUES ($1, 0, 'short_answer', 'Nghe và trả lời', 5.00, $2, 'audio', false, true)`,
		sectionID, assetID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// The 409 T-2.4 specified and could not implement: an asset a published version
// uses cannot be deleted from the library.
func TestDeletingAnAssetAPublishedVersionUsesIsRefused(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := media.NewService(media.NewStore(pool), newFakeStore())
	ctx := context.Background()

	asset := upload(t, svc, uploader, "dang-dung.mp3")
	if err := publishVersionUsing(ctx, pool, uploader, asset.ID, true); err != nil {
		t.Fatalf("publish: %v", err)
	}

	err := svc.Delete(ctx, media.DeleteInput{ID: asset.ID, ActorID: uploader})
	if !errors.Is(err, media.ErrReferenced) {
		t.Fatalf("delete returned %v, want ErrReferenced", err)
	}
	var blocked *media.ReferencedError
	if !errors.As(err, &blocked) || len(blocked.Tests) != 1 ||
		blocked.Tests[0].Title != "Đề đã xuất bản" || blocked.Tests[0].Version != 1 {
		t.Errorf("the refusal names %+v, want \"Đề đã xuất bản\" v1", blocked)
	}

	listed, _, err := svc.List(ctx, media.ListInput{Limit: media.MaxLimit})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range listed {
		if a.ID == asset.ID && (a.UsageCount != 1 || len(a.UsedIn) != 1 || a.UsedIn[0].Version != 1) {
			t.Errorf("the listing carries usage %d and %+v, want 1 and the version", a.UsageCount, a.UsedIn)
		}
	}

	// Refused, not half-done.
	if _, err := svc.Get(ctx, asset.ID); err != nil {
		t.Errorf("the asset was deleted anyway: %v", err)
	}
}

func TestDeletingAnUnreferencedAssetStillWorks(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := media.NewService(media.NewStore(pool), newFakeStore())
	ctx := context.Background()

	used := upload(t, svc, uploader, "dang-dung-2.mp3")
	unused := upload(t, svc, uploader, "khong-ai-dung.mp3")
	if err := publishVersionUsing(ctx, pool, uploader, used.ID, true); err != nil {
		t.Fatal(err)
	}

	if err := svc.Delete(ctx, media.DeleteInput{ID: unused.ID, ActorID: uploader}); err != nil {
		t.Errorf("an unreferenced asset was refused: %v", err)
	}
}

// The usage count the library screen shows, so a referenced asset can be
// rendered as undeletable before the teacher tries.
func TestUsageCountReflectsPublishedVersions(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := media.NewService(media.NewStore(pool), newFakeStore())
	ctx := context.Background()

	asset := upload(t, svc, uploader, "dem-luot-dung.mp3")
	if err := publishVersionUsing(ctx, pool, uploader, asset.ID, true); err != nil {
		t.Fatal(err)
	}

	listed, _, err := svc.List(ctx, media.ListInput{Limit: media.MaxLimit})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range listed {
		if a.ID == asset.ID && a.UsageCount != 1 {
			t.Errorf("usageCount is %d, want 1", a.UsageCount)
		}
	}
}

// The race AGENTS.md flagged as still open for media. A FOR UPDATE on the asset
// row does not block an INSERT into test_version_questions, so without the
// publish side taking the same lock a publish can claim the asset between the
// count and the update -- leaving a published version pointing at a
// soft-deleted asset.
func TestLockForVersionUseSerialisesAgainstDelete(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := media.NewService(media.NewStore(pool), newFakeStore())
	ctx := context.Background()

	for attempt := range 8 {
		asset := upload(t, svc, uploader, "tranh-chap.mp3")

		var wg sync.WaitGroup
		var deleteErr, publishErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			deleteErr = svc.Delete(ctx, media.DeleteInput{ID: asset.ID, ActorID: uploader})
		}()
		go func() {
			defer wg.Done()
			publishErr = publishVersionUsing(ctx, pool, uploader, asset.ID, true)
		}()
		wg.Wait()

		if deleteErr == nil && publishErr == nil {
			t.Fatalf("attempt %d: the asset was soft-deleted AND frozen into a published "+
				"version; the version now points at a deleted asset", attempt)
		}
	}
}

func TestLockForVersionUseRefusesADeletedAsset(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := media.NewService(media.NewStore(pool), newFakeStore())
	ctx := context.Background()

	asset := upload(t, svc, uploader, "da-xoa.mp3")
	if err := svc.Delete(ctx, media.DeleteInput{ID: asset.ID, ActorID: uploader}); err != nil {
		t.Fatal(err)
	}

	if err := publishVersionUsing(ctx, pool, uploader, asset.ID, true); !errors.Is(err, media.ErrNotFound) {
		t.Errorf("publishing against a deleted asset returned %v, want ErrNotFound", err)
	}
}
