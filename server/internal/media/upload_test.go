package media_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/media"
	"quizzivy/internal/media/probe"
)

// §11.1's rules, exercised end to end against a real database and a fake
// object store. What the object store does is not interesting here -- what
// matters is WHEN it is called relative to the row.

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func makeUploader(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	email := "uploader-" + hex.EncodeToString(nonce) + "@example.com"

	var id string
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO app.users (email, full_name, role) VALUES ($1,'Giáo viên','admin') RETURNING id::text`,
		email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM app.audit_log WHERE actor_user_id = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM app.media_assets WHERE uploaded_by = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM app.users WHERE id = $1`, id)
	})
	return id
}

// fakeStore records what it was asked to do, so the tests can assert ORDER --
// that the object lands before the row, and is removed when the row fails.
type fakeStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	puts    []string
	deletes []string
	putErr  error
}

func newFakeStore() *fakeStore { return &fakeStore{objects: map[string][]byte{}} }

func (f *fakeStore) Put(_ context.Context, key, _ string, body io.Reader, _ int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.putErr != nil {
		return f.putErr
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.objects[key] = data
	f.puts = append(f.puts, key)
	return nil
}

func (f *fakeStore) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, key)
	f.deletes = append(f.deletes, key)
	return nil
}

func (f *fakeStore) SignedURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://example.test/" + key, nil
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../media/probe/testdata/" + name)
	if err != nil {
		// Run from internal/media, so the probe corpus is one level across.
		data, err = os.ReadFile("probe/testdata/" + name)
		if err != nil {
			t.Fatalf("fixture %s: %v", name, err)
		}
	}
	return data
}

func newService(t *testing.T, pool *pgxpool.Pool, object media.ObjectStore) *media.Service {
	t.Helper()
	return media.NewService(media.NewStore(pool), object)
}

func TestAValidUploadStoresTheObjectAndThenTheRow(t *testing.T) {
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	objects := newFakeStore()
	svc := newService(t, pool, objects)

	asset, err := svc.Upload(context.Background(), media.UploadInput{
		Filename:   "bài nghe 1.mp3",
		Body:       bytes.NewReader(fixture(t, "cbr-128k.mp3")),
		UploaderID: uploader,
		IP:         "203.0.113.5",
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if asset.Kind != media.KindAudio || asset.MimeType != "audio/mpeg" {
		t.Errorf("kind/mime = %s/%s", asset.Kind, asset.MimeType)
	}
	if asset.DurationMs == nil || *asset.DurationMs < 9900 || *asset.DurationMs > 10100 {
		t.Errorf("durationMs = %v, want about 10005", asset.DurationMs)
	}
	// §11.2's layout, with the id in the key.
	if want := "audio/" + asset.ID + ".mp3"; asset.StorageKey != want {
		t.Errorf("storage key = %q, want %q", asset.StorageKey, want)
	}
	if len(objects.puts) != 1 || objects.puts[0] != asset.StorageKey {
		t.Errorf("puts = %v", objects.puts)
	}
	if len(objects.deletes) != 0 {
		t.Errorf("a successful upload deleted something: %v", objects.deletes)
	}
}

func TestAWavRenamedMp3IsRejectedOnItsBytes(t *testing.T) {
	// The reason identification never consults the extension (§11.1). Storing
	// this would put a file in the library that no browser plays as audio.
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	objects := newFakeStore()
	svc := newService(t, pool, objects)

	_, err := svc.Upload(context.Background(), media.UploadInput{
		Filename:   "bai-nghe.mp3",
		Body:       bytes.NewReader(fixture(t, "wav-renamed.mp3")),
		UploaderID: uploader,
	})
	if !errors.Is(err, probe.ErrUnsupportedType) {
		t.Fatalf("error = %v, want ErrUnsupportedType", err)
	}
	// Nothing was stored -- not the object, and not a row.
	if len(objects.puts) != 0 {
		t.Errorf("a rejected file reached the object store: %v", objects.puts)
	}
	var rows int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM app.media_assets WHERE uploaded_by = $1`, uploader).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Errorf("%d rows written for a rejected file", rows)
	}
}

func TestAnOverlongRecordingIsRejected(t *testing.T) {
	// §11.1: five minutes. Six minutes of CBR 128k is about 5.7 MB, so this is
	// rejected on DURATION rather than on size -- which is the check being
	// tested.
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	objects := newFakeStore()
	svc := newService(t, pool, objects)

	_, err := svc.Upload(context.Background(), media.UploadInput{
		Filename:   "bai-giang-dai.mp3",
		Body:       bytes.NewReader(sixMinuteMP3()),
		UploaderID: uploader,
	})
	if !errors.Is(err, media.ErrTooLong) {
		t.Fatalf("error = %v, want ErrTooLong", err)
	}
	if len(objects.puts) != 0 {
		t.Errorf("an over-long file reached the object store: %v", objects.puts)
	}
}

func TestAnOversizedUploadIsCutOffBeforeAnythingParsesIt(t *testing.T) {
	// The order §11.1 specifies: size first. A 50 MB upload must not become a
	// parsing problem, and must not be held in full anywhere.
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	objects := newFakeStore()
	svc := newService(t, pool, objects)

	_, err := svc.Upload(context.Background(), media.UploadInput{
		Filename:   "khong-lo.mp3",
		Body:       bytes.NewReader(make([]byte, media.MaxBytes+1024)),
		UploaderID: uploader,
	})
	if !errors.Is(err, media.ErrTooLarge) {
		t.Fatalf("error = %v, want ErrTooLarge", err)
	}
}

func TestTheSameFileTwiceMakesTwoRowsWithTwoKeys(t *testing.T) {
	// [D-06] and §11.1: a re-upload never overwrites. That is what lets a
	// frozen test version reference an asset without copying the file. The
	// equal checksums are what the "you already uploaded this" warning reads.
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	objects := newFakeStore()
	svc := newService(t, pool, objects)
	data := fixture(t, "cbr-128k.mp3")

	first, err := svc.Upload(context.Background(), media.UploadInput{
		Filename: "bai-nghe.mp3", Body: bytes.NewReader(data), UploaderID: uploader})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Upload(context.Background(), media.UploadInput{
		Filename: "bai-nghe.mp3", Body: bytes.NewReader(data), UploaderID: uploader})
	if err != nil {
		t.Fatal(err)
	}

	if first.ID == second.ID {
		t.Error("the second upload reused the first row")
	}
	if first.StorageKey == second.StorageKey {
		t.Error("the second upload overwrote the first object")
	}
	if !bytes.Equal(first.ChecksumSHA256, second.ChecksumSHA256) {
		t.Error("identical bytes produced different checksums")
	}
	if len(objects.puts) != 2 {
		t.Errorf("puts = %v, want two distinct keys", objects.puts)
	}
}

func TestAFailedRowInsertRemovesTheObject(t *testing.T) {
	// An object with no row is a file nothing references, nothing lists, and
	// nothing will ever clean up. The uploader id is bogus, so the FK rejects
	// the row after the object has already landed.
	pool := newPool(t)
	objects := newFakeStore()
	svc := newService(t, pool, objects)

	_, err := svc.Upload(context.Background(), media.UploadInput{
		Filename:   "bai-nghe.mp3",
		Body:       bytes.NewReader(fixture(t, "cbr-128k.mp3")),
		UploaderID: "01935000-0000-7000-8000-00000000ffff",
	})
	if err == nil {
		t.Fatal("an upload with an unknown uploader succeeded")
	}
	if len(objects.puts) != 1 {
		t.Fatalf("expected the object to have been written first, puts = %v", objects.puts)
	}
	if len(objects.deletes) != 1 || objects.deletes[0] != objects.puts[0] {
		t.Errorf("the orphaned object was not removed: puts=%v deletes=%v", objects.puts, objects.deletes)
	}
}

func TestAFilenameNeverBecomesAPath(t *testing.T) {
	// The name is a label in the library and nothing else -- the key comes from
	// the asset id. This asserts it cannot carry a directory separator anyway.
	pool := newPool(t)
	uploader := makeUploader(t, pool)
	svc := newService(t, pool, newFakeStore())

	asset, err := svc.Upload(context.Background(), media.UploadInput{
		Filename:   "../../etc/passwd.mp3",
		Body:       bytes.NewReader(fixture(t, "cbr-128k.mp3")),
		UploaderID: uploader,
	})
	if err != nil {
		t.Fatal(err)
	}
	if asset.OriginalFilename != "passwd.mp3" {
		t.Errorf("filename = %q, want the base name only", asset.OriginalFilename)
	}
	if want := "audio/" + asset.ID + ".mp3"; asset.StorageKey != want {
		t.Errorf("storage key = %q; it must come from the id, not the name", asset.StorageKey)
	}
}

// sixMinuteMP3 builds a CBR 128 kbps stream just over §11.1's five-minute cap.
func sixMinuteMP3() []byte {
	const (
		sampleRate  = 44100
		samples     = 1152
		frameLen    = 144*128*1000/sampleRate + 0
		targetFrame = 6 * 60 * sampleRate / samples // ~13781
	)
	out := make([]byte, 0, frameLen*targetFrame)
	header := []byte{0xFF, 0xFB, 0x90, 0x00}
	for range targetFrame {
		f := make([]byte, frameLen)
		copy(f, header)
		out = append(out, f...)
	}
	return out
}
