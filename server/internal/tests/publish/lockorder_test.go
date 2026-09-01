package publish_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/questions"
)

// audioAsset inserts a usable audio asset and returns its id.
//
// The storage key carries a random suffix rather than the test name: once a
// publish freezes a reference, test_version_questions holds it with ON DELETE
// RESTRICT, so the cleanup below cannot remove the row and a fixed key would
// collide on the next run.
func audioAsset(t *testing.T, pool *pgxpool.Pool, owner, label string) string {
	t.Helper()
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	key := "audio/" + label + "-" + hex.EncodeToString(nonce) + ".mp3"

	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO app.media_assets
		        (kind, storage_key, mime_type, bytes, duration_ms,
		         original_filename, checksum_sha256, uploaded_by)
		 VALUES ('audio', $1, 'audio/mpeg', 1024, 10000, 'nghe.mp3',
		         repeat('c', 32)::bytea, $2)
		 RETURNING id::text`, key, owner).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM app.media_assets WHERE id = $1`, id)
	})
	return id
}

// listeningQuestion attaches an audio asset with a full policy, which is what
// makes the snapshot take that asset's lock.
func (b *builder) listeningQuestion(prompt, assetID string) string {
	b.t.Helper()
	allow := false
	show := true
	return b.question(questions.Input{
		Type: questions.ShortAnswer, Prompt: prompt, Points: "1.00",
		MediaAssetID: &assetID,
		Audio: &questions.AudioPolicy{
			AllowSeek: allow, ShowTranscriptAfterSubmit: show,
		},
	})
}

// TestConcurrentPublishesSharingAssetsDoNotDeadlock is the deadlock the sort
// exists to prevent.
//
// Two tests that reuse the same listening files -- ordinary in a school -- were
// locked in document order, so publishing both at once could take asset 1 then
// asset 2 in one transaction and asset 2 then asset 1 in the other. Postgres
// breaks the cycle by aborting one with 40P01, and the teacher who loses gets
// an error on a deliberate action that says nothing they can act on.
func TestConcurrentPublishesSharingAssetsDoNotDeadlock(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)

	// Ids are sorted, not created, order -- so the fixture cannot accidentally
	// arrange the two tests in the same order and pass by luck.
	first := audioAsset(t, pool, author, "lock-order-1")
	second := audioAsset(t, pool, author, "lock-order-2")

	q1 := b.listeningQuestion("Nghe tệp A", first)
	q2 := b.listeningQuestion("Nghe tệp B", second)

	// One test names them in one order, the other in the opposite.
	testA := b.draft("Đề A", q1, q2)
	testB := b.draft("Đề B", q2, q1)

	for attempt := range 6 {
		var wg sync.WaitGroup
		errs := make([]error, 2)

		wg.Add(2)
		go func() {
			defer wg.Done()
			_, errs[0] = b.publish(testA.ID)
		}()
		go func() {
			defer wg.Done()
			_, errs[1] = b.publish(testB.ID)
		}()
		wg.Wait()

		for i, err := range errs {
			if err == nil {
				continue
			}
			if strings.Contains(err.Error(), "deadlock") || strings.Contains(err.Error(), "40P01") {
				t.Fatalf("attempt %d, publish %d deadlocked: %v", attempt, i, err)
			}
			t.Fatalf("attempt %d, publish %d failed: %v", attempt, i, err)
		}
	}
}

// The lock is still taken, and still refuses an asset that has been deleted --
// hoisting it out of the loop must not have dropped the check it was there for.
func TestPublishStillRefusesADeletedAsset(t *testing.T) {
	pool := newPool(t)
	author := makeAuthor(t, pool)
	b := newBuilder(t, pool, author)

	asset := audioAsset(t, pool, author, "deleted")
	q := b.listeningQuestion("Nghe tệp đã xoá", asset)
	draft := b.draft("Đề dùng tệp đã xoá", q)

	if _, err := pool.Exec(context.Background(),
		`UPDATE app.media_assets SET deleted_at = now() WHERE id = $1`, asset); err != nil {
		t.Fatal(err)
	}

	if _, err := b.publish(draft.ID); err == nil {
		t.Error("publishing froze a reference to a soft-deleted asset")
	}
}
