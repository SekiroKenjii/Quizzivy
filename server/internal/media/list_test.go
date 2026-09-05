package media_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/media"
)

// upload puts one asset in the library and returns it.
func upload(t *testing.T, svc *media.Service, uploader, name string) media.Asset {
	t.Helper()
	asset, err := svc.Upload(context.Background(), media.UploadInput{
		Filename:   name,
		Body:       bytesReader(fixture(t, "cbr-128k.mp3")),
		UploaderID: uploader,
	})
	if err != nil {
		t.Fatalf("upload %s: %v", name, err)
	}
	return asset
}

// TestListPagesWithoutRepeatingOrSkipping is the property keyset pagination
// exists for. Walking the whole library one row at a time must yield every
// asset exactly once, in strict recency order.
func TestListPagesWithoutRepeatingOrSkipping(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	// Other packages insert into app.media_assets while this walks it, and an
	// OFFSET walk over a moving table repeats or skips a row. REPEATABLE READ
	// pins one snapshot for every page; rolling back is also the cleanup.
	tx, err := pool.BeginTx(context.Background(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	svc := media.NewService(media.NewStore(tx), newFakeStore())

	const total = 5
	want := make([]string, 0, total)
	for i := range total {
		a := upload(t, svc, uploader, string(rune('a'+i))+".mp3")
		want = append(want, a.ID)
	}
	// Newest first.
	for i, j := 0, len(want)-1; i < j; i, j = i+1, j-1 {
		want[i], want[j] = want[j], want[i]
	}

	var live int
	if err := tx.QueryRow(context.Background(),
		`SELECT count(*) FROM app.media_assets WHERE deleted_at IS NULL`).Scan(&live); err != nil {
		t.Fatal(err)
	}
	maxPages := live + 10

	var got []string
	seen := map[string]int{}
	for number := 1; ; number++ {
		if number > maxPages {
			t.Fatalf("pagination did not terminate after %d pages for %d live assets", number, live)
		}
		assets, page, err := svc.List(context.Background(), media.ListInput{Limit: 1, Page: number})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if page.Total < total {
			t.Fatalf("page %d reports total %d, below this test's %d uploads", number, page.Total, total)
		}
		for _, a := range assets {
			// Restrict to this test's own uploads: the table is shared.
			if contains(want, a.ID) {
				got = append(got, a.ID)
				seen[a.ID]++
			}
		}
		if len(assets) == 0 {
			break
		}
	}

	for id, n := range seen {
		if n != 1 {
			t.Errorf("asset %s appeared %d times across pages, want exactly 1", id, n)
		}
	}
	if len(got) != total {
		t.Fatalf("saw %d of this test's %d assets across all pages", len(got), total)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %s, want %s (recency order broken)", i, got[i], want[i])
		}
	}
}

// TestListFiltersByKindAndSignsEveryItem covers the two things the library
// screen needs from a page: only the kind it asked for, and a usable URL on
// every row, since the bucket is private (§11.2).
func TestListFiltersByKindAndSignsEveryItem(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := media.NewService(media.NewStore(pool), newFakeStore())
	mine := upload(t, svc, uploader, "nghe.mp3").ID

	image := media.KindImage
	assets, _, err := svc.List(context.Background(), media.ListInput{Kind: &image, Limit: 100})
	if err != nil {
		t.Fatalf("list images: %v", err)
	}
	for _, a := range assets {
		if a.ID == mine {
			t.Error("an audio asset was returned when filtering for images")
		}
		if a.Kind != media.KindImage {
			t.Errorf("kind filter returned a %s asset", a.Kind)
		}
	}

	audio := media.KindAudio
	assets, _, err = svc.List(context.Background(), media.ListInput{Kind: &audio, Limit: 100})
	if err != nil {
		t.Fatalf("list audio: %v", err)
	}
	var found bool
	for _, a := range assets {
		if a.URL == "" {
			t.Errorf("asset %s came back with no signed URL", a.ID)
		}
		if a.ID == mine {
			found = true
		}
	}
	if !found {
		t.Error("the uploaded audio asset was not in the audio listing")
	}
}

// TestAPagePastTheEndIsEmptyWithTheSameTotal: the client draws its page count
// from `total`, so the number must not vanish on the page nothing is on.
func TestAPagePastTheEndIsEmptyWithTheSameTotal(t *testing.T) {
	pool := newPool(t)
	svc := media.NewService(media.NewStore(pool), newFakeStore())
	// One asset of its own, so the total it reasons about is never zero.
	upload(t, svc, makeUploader(t, pool), "beyond.mp3")

	_, page, err := svc.List(context.Background(), media.ListInput{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	requested := page.Total + 50
	beyond, far, err := svc.List(context.Background(), media.ListInput{Limit: 1, Page: requested})
	if err != nil {
		t.Fatal(err)
	}
	if len(beyond) != 0 {
		t.Errorf("%d rows on a page past the end", len(beyond))
	}
	// A lower bound, not equality: the other tests in this package insert and
	// delete assets in parallel, so the count moves between the two calls. The
	// regression this guards is `total` reading 0 on the page nothing is on,
	// which would collapse the client's page count to nothing.
	if far.Total < 1 || far.Number != requested || far.Size != 1 {
		t.Errorf("page past the end reports %+v, want a non-zero total and page %d at size 1", far, requested)
	}
}

func TestTotalBytesSumsTheWholeShelfNotThePage(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := media.NewService(media.NewStore(pool), newFakeStore())
	ctx := context.Background()

	before, err := svc.TotalBytes(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	a := upload(t, svc, uploader, "shelf.mp3")
	after, err := svc.TotalBytes(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if after < before+a.Bytes {
		t.Errorf("total moved from %d to %d after a %d-byte upload", before, after, a.Bytes)
	}
	audio := media.Kind("audio")
	if byKind, err := svc.TotalBytes(ctx, &audio); err != nil || byKind < a.Bytes {
		t.Errorf("audio total %d (%v), want at least %d", byKind, err, a.Bytes)
	}
}
