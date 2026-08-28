package media_test

import (
	"context"
	"testing"

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
	svc := media.NewService(media.NewStore(pool), newFakeStore())

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

	var got []string
	seen := map[string]int{}
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > total+2 {
			t.Fatal("pagination did not terminate")
		}
		assets, next, err := svc.List(context.Background(), media.ListInput{Limit: 1, Cursor: cursor})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		for _, a := range assets {
			// Restrict to this test's own uploads: the table is shared.
			if contains(want, a.ID) {
				got = append(got, a.ID)
				seen[a.ID]++
			}
		}
		if next == "" {
			break
		}
		cursor = next
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

// TestListRejectsForgedCursor keeps a malformed cursor a 400 rather than a 500
// or, worse, a query built from attacker-supplied text.
func TestListRejectsForgedCursor(t *testing.T) {
	pool := newPool(t)
	svc := media.NewService(media.NewStore(pool), newFakeStore())

	for _, bad := range []string{
		"not-base64!!",
		"YWJj",                         // "abc": no separator
		"MjAyNC0wMS0wMXxub3QtYS11dWlk", // valid time, id is not a uuid
		"eHx5",                         // "x|y": neither half parses
	} {
		_, _, err := svc.List(context.Background(), media.ListInput{Cursor: bad})
		if err == nil {
			t.Errorf("cursor %q was accepted", bad)
		}
	}
}
