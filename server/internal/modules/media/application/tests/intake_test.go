package application_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/media/application"
	"quizzivy/internal/modules/media/application/command"
	"quizzivy/internal/modules/media/application/query"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/paging"
)

const copyBufferBytes = 32 << 10

var pngMagic = []byte("\x89PNG\r\n\x1a\n")

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type streamed struct {
	t    *testing.T
	body io.Reader
	read int64
}

func (s *streamed) Read(p []byte) (int, error) {
	if len(p) > copyBufferBytes {
		s.t.Errorf("the upload was asked for %d bytes at once, more than the copy's buffer of %d", len(p), copyBufferBytes)
	}
	n, err := s.body.Read(p)
	s.read += int64(n)
	return n, err
}

func audioOf(t *testing.T, size int64) *streamed {
	return &streamed{t: t, body: io.LimitReader(zeros{}, size)}
}

func imageOf(t *testing.T, size int64) *streamed {
	return &streamed{t: t, body: io.MultiReader(bytes.NewReader(pngMagic), io.LimitReader(zeros{}, size-int64(len(pngMagic))))}
}

type recordedPut struct {
	key, contentType string
	declared, read   int64
}

type recordingStore struct {
	puts    []recordedPut
	deletes []string
	putErr  error
}

func (s *recordingStore) Put(_ context.Context, key, contentType string, body io.Reader, size int64) error {
	if s.putErr != nil {
		return s.putErr
	}
	read, err := io.Copy(io.Discard, body)
	if err != nil {
		return err
	}
	s.puts = append(s.puts, recordedPut{key: key, contentType: contentType, declared: size, read: read})
	return nil
}

func (s *recordingStore) Delete(_ context.Context, key string) error {
	s.deletes = append(s.deletes, key)
	return nil
}

func (s *recordingStore) SignedURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://signed.test/" + key, nil
}

type scriptedAudio struct {
	durationMs int
	calls      int
}

func (p *scriptedAudio) Audio(io.ReaderAt, int64) (string, int, error) {
	p.calls++
	return "audio/mpeg", p.durationMs, nil
}

type scriptedImage struct {
	width, height int
	unreadable    bool
	calls         int
}

func (p *scriptedImage) Image(io.ReaderAt, int64) (int, int, bool) {
	p.calls++
	return p.width, p.height, !p.unreadable
}

type found struct {
	scope access.Scope
	id    string
}

type scriptedRepo struct {
	usage     domain.Usage
	usageOf   []access.Scope
	inserts   []domain.InsertInput
	insertErr error
	stored    domain.Asset
	finds     []found
	findErr   error
	updates   []domain.UpdateInput
	listed    []domain.ListInput
	totalled  []domain.ListInput
	faceted   []domain.ListInput
	questions map[string]int
	versions  map[string][]domain.TestRef
}

func (r *scriptedRepo) Insert(_ context.Context, in domain.InsertInput) (domain.Asset, error) {
	r.inserts = append(r.inserts, in)
	if r.insertErr != nil {
		return domain.Asset{}, r.insertErr
	}
	return domain.Asset{ID: in.ID, Kind: in.Kind, StorageKey: in.StorageKey, MimeType: in.MimeType, Bytes: in.Bytes, OriginalFilename: in.OriginalFilename}, nil
}

func (r *scriptedRepo) Get(context.Context, string) (domain.Asset, error) {
	return r.stored, nil
}

func (r *scriptedRepo) Find(_ context.Context, scope access.Scope, id string) (domain.Asset, error) {
	r.finds = append(r.finds, found{scope: scope, id: id})
	return r.stored, r.findErr
}

func (r *scriptedRepo) Update(_ context.Context, in domain.UpdateInput) (domain.Asset, error) {
	r.updates = append(r.updates, in)
	updated := r.stored
	if in.DisplayName != nil {
		updated.DisplayName = *in.DisplayName
	}
	if in.SetDefaultMaxPlays {
		updated.DefaultMaxPlays = in.DefaultMaxPlays
	}
	return updated, nil
}

func (r *scriptedRepo) CountByChecksum(context.Context, string, []byte) (int, error) { return 0, nil }

func (r *scriptedRepo) List(_ context.Context, in domain.ListInput) ([]domain.Asset, paging.Page, error) {
	r.listed = append(r.listed, in)
	return []domain.Asset{r.stored}, paging.Page{Number: 1, Size: 24, Total: 1}, nil
}

func (r *scriptedRepo) TotalBytes(_ context.Context, in domain.ListInput) (int64, error) {
	r.totalled = append(r.totalled, in)
	return r.stored.Bytes, nil
}

func (r *scriptedRepo) Facets(_ context.Context, in domain.ListInput) (domain.Facets, error) {
	r.faceted = append(r.faceted, in)
	return domain.Facets{All: 1}, nil
}

func (r *scriptedRepo) Usage(_ context.Context, scope access.Scope) (domain.Usage, error) {
	r.usageOf = append(r.usageOf, scope)
	return r.usage, nil
}

func (r *scriptedRepo) QuestionCounts(context.Context, []string) (map[string]int, error) {
	return r.questions, nil
}

func (r *scriptedRepo) SoftDelete(context.Context, domain.DeleteInput) error { return nil }

func (r *scriptedRepo) ReferencesFor(context.Context, []string) (map[string][]domain.TestRef, error) {
	return r.versions, nil
}

func (r *scriptedRepo) ReachableByStudent(context.Context, string, string) (bool, error) {
	return false, nil
}

func (r *scriptedRepo) Readable(context.Context, access.Scope, []string) (map[string]domain.Kind, error) {
	return nil, nil
}

type bench struct {
	repo   *scriptedRepo
	store  *recordingStore
	audio  *scriptedAudio
	images *scriptedImage
	app    *application.Application
}

func newBench() *bench {
	b := &bench{
		repo:   &scriptedRepo{},
		store:  &recordingStore{},
		audio:  &scriptedAudio{durationMs: 252_000},
		images: &scriptedImage{width: 1200, height: 800},
	}
	b.app = application.New(b.repo, b.store, b.audio).WithImageProbe(b.images)
	return b
}

const uploader = "01935000-0000-7000-8000-0000000000c3"

func (b *bench) upload(body io.Reader, plays *int) (domain.Asset, error) {
	return b.app.Commands.Upload.Handle(context.Background(), command.Upload{
		Filename: "thư mục/Bài nghe 1.mp3", Body: body, UploaderID: uploader, DefaultMaxPlays: plays, IP: "203.0.113.5", UserAgent: "kiểm thử",
	})
}

func (b *bench) nothingStored(t *testing.T) {
	t.Helper()
	if len(b.store.puts) != 0 || len(b.repo.inserts) != 0 {
		t.Errorf("a refused upload reached the object store %v and the repository %v", b.store.puts, b.repo.inserts)
	}
}

func plays(n int) *int { return &n }

func TestAudioOfExactlyTheLimitIsStored(t *testing.T) {
	b := newBench()
	body := audioOf(t, domain.MaxAudioBytes)
	asset, err := b.upload(body, nil)
	if err != nil {
		t.Fatalf("audio of exactly %d bytes: %v", domain.MaxAudioBytes, err)
	}
	if len(b.store.puts) != 1 || len(b.repo.inserts) != 1 {
		t.Fatalf("puts %v and inserts %v, want one of each", b.store.puts, b.repo.inserts)
	}
	put, in := b.store.puts[0], b.repo.inserts[0]
	if put.declared != domain.MaxAudioBytes || put.read != domain.MaxAudioBytes || put.contentType != "audio/mpeg" {
		t.Errorf("the object was put as %+v, want %d bytes of audio/mpeg, all of them read", put, domain.MaxAudioBytes)
	}
	if want := "audio/" + in.ID + ".mp3"; in.ID == "" || put.key != want || in.StorageKey != want || asset.StorageKey != want {
		t.Errorf("the object key is %q and the row's %q, want %q for both", put.key, in.StorageKey, want)
	}
	if in.Kind != domain.KindAudio || in.MimeType != "audio/mpeg" || in.Bytes != domain.MaxAudioBytes || in.DurationMs == nil || *in.DurationMs != 252_000 {
		t.Errorf("the row is %s %s of %d bytes lasting %v, want the audio the probe measured", in.Kind, in.MimeType, in.Bytes, in.DurationMs)
	}
	if in.UploaderID != uploader || in.OwnerID != "" || in.OriginalFilename != "Bài nghe 1.mp3" || len(in.ChecksumSHA256) != 32 {
		t.Errorf("the row names uploader %q, owner %q, file %q and a %d-byte checksum", in.UploaderID, in.OwnerID, in.OriginalFilename, len(in.ChecksumSHA256))
	}
	if in.IP == nil || *in.IP != "203.0.113.5" || in.UserAgent == nil || *in.UserAgent != "kiểm thử" || in.Now.IsZero() {
		t.Errorf("the row's audit context is %v, %v at %v", in.IP, in.UserAgent, in.Now)
	}
	if in.QuotaBytes != domain.DefaultOwnerQuotaBytes {
		t.Errorf("the row is measured against %d bytes, want the default quota %d", in.QuotaBytes, domain.DefaultOwnerQuotaBytes)
	}
	if in.DefaultMaxPlays != nil || in.DisplayName != nil || in.Width != nil || in.Height != nil {
		t.Errorf("audio uploaded with no limit carries plays %v, name %v and size %v by %v", in.DefaultMaxPlays, in.DisplayName, in.Width, in.Height)
	}
	if len(b.store.deletes) != 0 || b.images.calls != 0 {
		t.Errorf("a stored audio file deleted %v and was measured as an image %d times", b.store.deletes, b.images.calls)
	}
	if !slices.Equal(b.repo.usageOf, []access.Scope{{UserID: uploader}}) {
		t.Errorf("the library measured before the upload is %+v, want the uploader's own", b.repo.usageOf)
	}
}

func TestAudioOneByteOverTheLimitIsRefusedBeforeItIsIdentified(t *testing.T) {
	b := newBench()
	body := audioOf(t, domain.MaxAudioBytes+1024)
	_, err := b.upload(body, nil)
	if !errors.Is(err, domain.ErrTooLarge) || errors.Is(err, domain.ErrImageTooLarge) {
		t.Fatalf("error = %v, want the file refused as too large and not as an image", err)
	}
	if b.audio.calls != 0 || b.images.calls != 0 {
		t.Errorf("an oversized body was probed %d and %d times before it was refused", b.audio.calls, b.images.calls)
	}
	if body.read != domain.MaxAudioBytes+1 {
		t.Errorf("%d bytes were read, want the limit and one more", body.read)
	}
	b.nothingStored(t)

	b = newBench()
	if _, err := b.upload(audioOf(t, domain.MaxAudioBytes+1), nil); !errors.Is(err, domain.ErrTooLarge) {
		t.Errorf("one byte over the limit: %v, want it refused as too large", err)
	}
	b.nothingStored(t)

	b = newBench()
	if _, err := b.upload(imageOf(t, domain.MaxAudioBytes+1), nil); !errors.Is(err, domain.ErrTooLarge) || errors.Is(err, domain.ErrImageTooLarge) || b.images.calls != 0 {
		t.Errorf("an unidentified body over the audio limit: %v after %d image probes, want the audio refusal", err, b.images.calls)
	}
}

func TestAnImageOverItsOwnLimitIsRefusedOnceIdentified(t *testing.T) {
	b := newBench()
	_, err := b.upload(imageOf(t, domain.MaxImageBytes+1), nil)
	if !errors.Is(err, domain.ErrImageTooLarge) {
		t.Fatalf("error = %v, want the image refused for its own limit", err)
	}
	if b.audio.calls != 0 {
		t.Errorf("an image was probed as audio %d times", b.audio.calls)
	}
	b.nothingStored(t)

	b = newBench()
	if _, err := b.upload(imageOf(t, domain.MaxImageBytes), nil); err != nil {
		t.Fatalf("an image of exactly %d bytes: %v", domain.MaxImageBytes, err)
	}
	if len(b.store.puts) != 1 || b.store.puts[0].read != domain.MaxImageBytes || b.store.puts[0].contentType != "image/png" {
		t.Errorf("puts = %+v, want the whole image as image/png", b.store.puts)
	}

	b = newBench()
	if _, err := b.upload(audioOf(t, domain.MaxImageBytes+1), nil); err != nil {
		t.Errorf("audio one byte over the image limit: %v, want it stored", err)
	}
}

func TestADefaultPlayLimitReachesTheRowForAudio(t *testing.T) {
	for name, limit := range map[string]*int{"two plays": plays(2), "unlimited": plays(0), "three plays": plays(domain.MaxDefaultPlays)} {
		b := newBench()
		if _, err := b.upload(audioOf(t, 4096), limit); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := b.repo.inserts[0].DefaultMaxPlays; got == nil || *got != *limit {
			t.Errorf("%s reached the row as %v, want %d", name, got, *limit)
		}
	}

	b := newBench()
	if _, err := b.upload(audioOf(t, 4096), plays(domain.MaxDefaultPlays+1)); !errors.Is(err, domain.ErrInvalidPlayLimit) {
		t.Errorf("a limit of four plays: %v, want it refused as out of range", err)
	}
	b.nothingStored(t)
}

func TestADefaultPlayLimitOnAnImageIsRefusedBeforeTheObjectIsPut(t *testing.T) {
	for name, limit := range map[string]*int{"two plays": plays(2), "unlimited": plays(0)} {
		b := newBench()
		_, err := b.upload(imageOf(t, 4096), limit)
		if !errors.Is(err, domain.ErrPlayLimitOnImage) {
			t.Fatalf("%s on an image: %v, want the play limit refused", name, err)
		}
		if len(b.store.puts) != 0 {
			t.Errorf("%s on an image put %v before it was refused", name, b.store.puts)
		}
		if len(b.repo.inserts) != 0 || len(b.store.deletes) != 0 {
			t.Errorf("%s on an image reached the repository %v and deleted %v", name, b.repo.inserts, b.store.deletes)
		}
	}
}

func TestAFailedInsertDeletesTheObjectThatWasPut(t *testing.T) {
	for name, failure := range map[string]error{
		"a failed insert":                errors.New("the row could not be written"),
		"a quota refusal under the lock": domain.ErrQuotaExceeded,
	} {
		b := newBench()
		b.repo.insertErr = failure
		_, err := b.upload(audioOf(t, 4096), nil)
		if !errors.Is(err, failure) {
			t.Fatalf("%s: %v, want the repository's own error", name, err)
		}
		if len(b.store.puts) != 1 {
			t.Fatalf("%s: puts = %v, want the object stored before the row", name, b.store.puts)
		}
		if !slices.Equal(b.store.deletes, []string{b.store.puts[0].key}) {
			t.Errorf("%s left the object behind: put %q, deleted %v", name, b.store.puts[0].key, b.store.deletes)
		}
	}
}

func TestAFullLibraryRefusesTheFileBeforeItIsStored(t *testing.T) {
	const quota = 10_000
	for name, c := range map[string]struct {
		usage   domain.Usage
		size    int64
		refused bool
	}{
		"room for exactly this file": {domain.Usage{AudioBytes: 4000, ImageBytes: 2000}, 4000, false},
		"one byte short":             {domain.Usage{AudioBytes: 4000, ImageBytes: 2001}, 4000, true},
		"full of audio":              {domain.Usage{AudioBytes: quota}, 64, true},
		"full of images":             {domain.Usage{ImageBytes: quota}, 64, true},
	} {
		b := newBench()
		b.app.WithOwnerQuota(quota)
		b.repo.usage = c.usage
		_, err := b.upload(audioOf(t, c.size), nil)
		if c.refused != errors.Is(err, domain.ErrQuotaExceeded) || (!c.refused && err != nil) {
			t.Errorf("%s: %v, want refused %v", name, err, c.refused)
			continue
		}
		if c.refused {
			b.nothingStored(t)
			continue
		}
		if len(b.repo.inserts) != 1 || b.repo.inserts[0].QuotaBytes != quota {
			t.Errorf("%s: the row is measured against %+v, want the configured quota %d", name, b.repo.inserts, quota)
		}
	}
}

func TestTheConfiguredQuotaIsTheOneTheLibraryReports(t *testing.T) {
	b := newBench()
	b.repo.usage = domain.Usage{AudioBytes: 7, ImageBytes: 5}
	scope := access.Scope{UserID: uploader}
	for _, c := range []struct {
		set, want int64
	}{{0, domain.DefaultOwnerQuotaBytes}, {-1, domain.DefaultOwnerQuotaBytes}, {4096, 4096}} {
		b.app.WithOwnerQuota(c.set)
		usage, err := b.app.Queries.Usage.Handle(context.Background(), query.Usage{Scope: scope})
		if err != nil {
			t.Fatal(err)
		}
		if usage != (domain.Usage{AudioBytes: 7, ImageBytes: 5, QuotaBytes: c.want}) {
			t.Errorf("a quota set to %d reports %+v, want the library's bytes beside %d", c.set, usage, c.want)
		}
	}
	if b.repo.usageOf[0] != scope {
		t.Errorf("usage was read in %+v, want %+v", b.repo.usageOf[0], scope)
	}
}

func TestAnImagesSizeReachesTheRowAndAudioHasNone(t *testing.T) {
	b := newBench()
	if _, err := b.upload(imageOf(t, 4096), nil); err != nil {
		t.Fatal(err)
	}
	in := b.repo.inserts[0]
	if in.Kind != domain.KindImage || in.MimeType != "image/png" || in.DurationMs != nil {
		t.Errorf("the row is %s %s lasting %v, want a png with no duration", in.Kind, in.MimeType, in.DurationMs)
	}
	if in.Width == nil || in.Height == nil || *in.Width != 1200 || *in.Height != 800 {
		t.Errorf("the row is %v by %v, want 1200 by 800", in.Width, in.Height)
	}
	if want := "image/" + in.ID + ".png"; in.StorageKey != want {
		t.Errorf("the image's key is %q, want %q", in.StorageKey, want)
	}

	b = newBench()
	if _, err := b.upload(audioOf(t, 4096), nil); err != nil {
		t.Fatal(err)
	}
	if in := b.repo.inserts[0]; in.Width != nil || in.Height != nil || b.images.calls != 0 {
		t.Errorf("audio is %v by %v after %d image probes, want no size and no probe", in.Width, in.Height, b.images.calls)
	}

	b = newBench()
	b.images.unreadable = true
	if _, err := b.upload(imageOf(t, 4096), nil); err != nil {
		t.Fatalf("an image whose header cannot be read: %v, want it stored", err)
	}
	if in := b.repo.inserts[0]; in.Width != nil || in.Height != nil || len(b.store.puts) != 1 {
		t.Errorf("an unreadable header gave %v by %v and puts %v, want the image stored without a size", in.Width, in.Height, b.store.puts)
	}

	repo, store := &scriptedRepo{}, &recordingStore{}
	bare := application.New(repo, store, &scriptedAudio{durationMs: 1000})
	if _, err := bare.Commands.Upload.Handle(context.Background(), command.Upload{Filename: "hinh.png", Body: imageOf(t, 4096), UploaderID: uploader}); err != nil {
		t.Fatalf("an image with no image probe wired: %v, want it stored", err)
	}
	if in := repo.inserts[0]; in.Width != nil || in.Height != nil {
		t.Errorf("no image probe gave %v by %v", in.Width, in.Height)
	}
}

func TestWhatIsNotAcceptedStoresNothing(t *testing.T) {
	b := newBench()
	b.audio.durationMs = domain.MaxDurationMs + 1
	if _, err := b.upload(audioOf(t, 4096), nil); !errors.Is(err, domain.ErrTooLong) {
		t.Errorf("audio a millisecond over the limit: %v, want it refused as too long", err)
	}
	b.nothingStored(t)

	b = newBench()
	b.audio.durationMs = domain.MaxDurationMs
	if _, err := b.upload(audioOf(t, 4096), nil); err != nil {
		t.Errorf("audio of exactly the longest duration: %v, want it stored", err)
	}

	b = newBench()
	if _, err := b.upload(audioOf(t, 0), nil); !errors.Is(err, domain.ErrUnsupportedType) {
		t.Errorf("an empty file: %v, want it refused as unsupported", err)
	}
	b.nothingStored(t)

	b = newBench()
	b.store.putErr = errors.New("the bucket is away")
	if _, err := b.upload(audioOf(t, 4096), nil); !errors.Is(err, b.store.putErr) {
		t.Errorf("a failed put: %v, want the store's error", err)
	}
	if len(b.repo.inserts) != 0 {
		t.Errorf("a row was written for an object that was never stored: %v", b.repo.inserts)
	}
}

const asset = "01935000-0000-7000-8000-00000000a001"

func (b *bench) update(in domain.UpdateInput) (domain.Asset, error) {
	in.ID, in.ActorID = asset, uploader
	return b.app.Commands.Update.Handle(context.Background(), command.Update{Input: in})
}

func name(s string) *string { return &s }

func TestAnUpdateIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	audio := domain.Asset{ID: asset, Kind: domain.KindAudio, StorageKey: "audio/a.mp3"}
	image := domain.Asset{ID: asset, Kind: domain.KindImage, StorageKey: "image/a.png"}
	for label, c := range map[string]struct {
		stored domain.Asset
		in     domain.UpdateInput
		want   error
		found  int
	}{
		"a blank name":                    {audio, domain.UpdateInput{DisplayName: name(" \t ")}, domain.ErrInvalidName, 0},
		"an empty name":                   {audio, domain.UpdateInput{DisplayName: name("")}, domain.ErrInvalidName, 0},
		"a name of 201 characters":        {audio, domain.UpdateInput{DisplayName: name(strings.Repeat("ê", domain.MaxDisplayNameLength+1))}, domain.ErrInvalidName, 0},
		"no change at all":                {audio, domain.UpdateInput{}, domain.ErrNothingToUpdate, 0},
		"a limit left unset but named":    {audio, domain.UpdateInput{DefaultMaxPlays: plays(2)}, domain.ErrNothingToUpdate, 0},
		"a play limit on an image":        {image, domain.UpdateInput{SetDefaultMaxPlays: true, DefaultMaxPlays: plays(2)}, domain.ErrPlayLimitOnImage, 1},
		"unlimited plays on an image":     {image, domain.UpdateInput{SetDefaultMaxPlays: true, DefaultMaxPlays: plays(0)}, domain.ErrPlayLimitOnImage, 1},
		"a name and a limit, an image":    {image, domain.UpdateInput{DisplayName: name("Bản đồ"), SetDefaultMaxPlays: true, DefaultMaxPlays: plays(1)}, domain.ErrPlayLimitOnImage, 1},
		"four plays":                      {audio, domain.UpdateInput{SetDefaultMaxPlays: true, DefaultMaxPlays: plays(domain.MaxDefaultPlays + 1)}, domain.ErrInvalidPlayLimit, 1},
		"a file that is not the caller's": {audio, domain.UpdateInput{DisplayName: name("Tên mới")}, domain.ErrNotFound, 1},
	} {
		b := newBench()
		b.repo.stored = c.stored
		if c.want == domain.ErrNotFound {
			b.repo.findErr = domain.ErrNotFound
		}
		if _, err := b.update(c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", label, err, c.want)
		}
		if len(b.repo.updates) != 0 {
			t.Errorf("%s was written all the same: %+v", label, b.repo.updates)
		}
		if len(b.repo.finds) != c.found {
			t.Errorf("%s looked the file up %d times, want %d", label, len(b.repo.finds), c.found)
		}
	}
}

func TestAnUpdateWritesTheTrimmedNameAndTheLimit(t *testing.T) {
	b := newBench()
	b.repo.stored = domain.Asset{ID: asset, Kind: domain.KindAudio, StorageKey: "audio/a.mp3", DisplayName: "cũ.mp3"}
	b.repo.questions = map[string]int{asset: 4}
	b.repo.versions = map[string][]domain.TestRef{asset: {{ID: "t1", Title: "Đề 1", Version: 2}}}

	updated, err := b.update(domain.UpdateInput{DisplayName: name("  Cambridge 15 · Test 2 · Part 1  "), SetDefaultMaxPlays: true, DefaultMaxPlays: plays(2), All: true, IP: "203.0.113.5", UserAgent: "kiểm thử"})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.repo.updates) != 1 {
		t.Fatalf("updates = %+v, want one", b.repo.updates)
	}
	in := b.repo.updates[0]
	if in.DisplayName == nil || *in.DisplayName != "Cambridge 15 · Test 2 · Part 1" {
		t.Errorf("the name was written as %v, want it trimmed", in.DisplayName)
	}
	if !in.SetDefaultMaxPlays || in.DefaultMaxPlays == nil || *in.DefaultMaxPlays != 2 {
		t.Errorf("the limit was written as set=%v %v, want 2", in.SetDefaultMaxPlays, in.DefaultMaxPlays)
	}
	if in.ID != asset || in.ActorID != uploader || !in.All || in.IP != "203.0.113.5" || in.UserAgent != "kiểm thử" || in.Now.IsZero() {
		t.Errorf("the update ran as %+v, want the caller's identity, scope and a time", in)
	}
	if want := (found{scope: access.Scope{UserID: uploader, All: true}, id: asset}); len(b.repo.finds) != 1 || b.repo.finds[0] != want {
		t.Errorf("the file was looked up as %+v, want %+v", b.repo.finds, want)
	}
	if updated.DisplayName != "Cambridge 15 · Test 2 · Part 1" || updated.DefaultMaxPlays == nil || *updated.DefaultMaxPlays != 2 {
		t.Errorf("the answer is named %q with limit %v", updated.DisplayName, updated.DefaultMaxPlays)
	}
	if updated.URL != "https://signed.test/audio/a.mp3" || updated.QuestionCount != 4 || updated.UsageCount != 1 || len(updated.UsedIn) != 1 {
		t.Errorf("the answer carries URL %q, %d questions and %d versions, want what the library lists", updated.URL, updated.QuestionCount, updated.UsageCount)
	}
}

func TestAnUpdateLeavesAloneWhatItDoesNotName(t *testing.T) {
	b := newBench()
	b.repo.stored = domain.Asset{ID: asset, Kind: domain.KindAudio, StorageKey: "audio/a.mp3"}
	if _, err := b.update(domain.UpdateInput{DisplayName: name("Chỉ đổi tên")}); err != nil {
		t.Fatal(err)
	}
	if in := b.repo.updates[0]; in.SetDefaultMaxPlays || in.DefaultMaxPlays != nil {
		t.Errorf("a rename also wrote the limit: set=%v %v", in.SetDefaultMaxPlays, in.DefaultMaxPlays)
	}
	if want := (access.Scope{UserID: uploader}); b.repo.finds[0].scope != want {
		t.Errorf("a teacher's update looked the file up in %+v, want %+v", b.repo.finds[0].scope, want)
	}

	b = newBench()
	b.repo.stored = domain.Asset{ID: asset, Kind: domain.KindAudio, StorageKey: "audio/a.mp3"}
	if _, err := b.update(domain.UpdateInput{SetDefaultMaxPlays: true}); err != nil {
		t.Fatalf("clearing the limit: %v", err)
	}
	if in := b.repo.updates[0]; in.DisplayName != nil || !in.SetDefaultMaxPlays || in.DefaultMaxPlays != nil {
		t.Errorf("clearing the limit wrote name %v, set=%v, limit %v", in.DisplayName, in.SetDefaultMaxPlays, in.DefaultMaxPlays)
	}

	b = newBench()
	b.repo.stored = domain.Asset{ID: asset, Kind: domain.KindImage, StorageKey: "image/a.png"}
	if _, err := b.update(domain.UpdateInput{SetDefaultMaxPlays: true}); err != nil {
		t.Errorf("clearing the limit of an image: %v, want it allowed", err)
	}
	if _, err := b.update(domain.UpdateInput{DisplayName: name("Bản đồ chỉ đường")}); err != nil {
		t.Errorf("renaming an image: %v", err)
	}
}

func TestTheLibrarysFiguresAreReadWithTheListsOwnFilters(t *testing.T) {
	b := newBench()
	b.repo.stored = domain.Asset{ID: asset, Kind: domain.KindAudio, StorageKey: "audio/a.mp3", Bytes: 42}
	b.repo.questions = map[string]int{asset: 3}
	b.repo.versions = map[string][]domain.TestRef{asset: {{ID: "t1", Title: "Đề 1", Version: 1}, {ID: "t1", Title: "Đề 1", Version: 2}}}
	audio := domain.KindAudio
	in := domain.ListInput{Scope: access.Scope{UserID: uploader}, Kind: &audio, Query: "nghe", Unused: true, Page: 2, Limit: 10}
	ctx := context.Background()

	listed, err := b.app.Queries.List.Handle(ctx, query.List{Input: in})
	if err != nil {
		t.Fatal(err)
	}
	if got := listed.Items[0]; got.QuestionCount != 3 || got.UsageCount != 2 || len(got.UsedIn) != 2 || got.URL != "https://signed.test/audio/a.mp3" {
		t.Errorf("the row lists %d questions, %d versions and URL %q", got.QuestionCount, got.UsageCount, got.URL)
	}
	if total, err := b.app.Queries.TotalBytes.Handle(ctx, query.TotalBytes{Scope: in.Scope, Kind: in.Kind, Query: in.Query, Unused: in.Unused}); err != nil || total != 42 {
		t.Errorf("total bytes = %d (%v)", total, err)
	}
	if facets, err := b.app.Queries.Facets.Handle(ctx, query.Facets{Input: in}); err != nil || facets.All != 1 {
		t.Errorf("facets = %+v (%v)", facets, err)
	}
	same := func(got domain.ListInput) bool {
		return got.Scope == in.Scope && got.Kind == in.Kind && got.Query == in.Query && got.Unused == in.Unused
	}
	if len(b.repo.listed) != 1 || !same(b.repo.listed[0]) || b.repo.listed[0].Page != 2 || b.repo.listed[0].Limit != 10 {
		t.Errorf("the list read %+v, want %+v", b.repo.listed, in)
	}
	if len(b.repo.totalled) != 1 || !same(b.repo.totalled[0]) {
		t.Errorf("the total read %+v, want the list's scope, kind, search and unused filter", b.repo.totalled)
	}
	if len(b.repo.faceted) != 1 || !same(b.repo.faceted[0]) {
		t.Errorf("the facets read %+v, want the list's input", b.repo.faceted)
	}
}
