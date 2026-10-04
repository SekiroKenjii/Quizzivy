package router_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/core/router"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	mediaapp "quizzivy/internal/modules/media/application"
	mediadomain "quizzivy/internal/modules/media/domain"
	mediahttp "quizzivy/internal/modules/media/http"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/paging"
)

const libraryAsset = "01935000-0000-7000-8000-00000000a001"

type mediaLibrary struct {
	mu       sync.Mutex
	stored   mediadomain.Asset
	findErr  error
	inserts  []mediadomain.InsertInput
	updates  []mediadomain.UpdateInput
	listed   []mediadomain.ListInput
	totalled []mediadomain.ListInput
	faceted  []mediadomain.ListInput
	usageOf  []access.Scope
	puts     []int64
}

func (m *mediaLibrary) Insert(_ context.Context, in mediadomain.InsertInput) (mediadomain.Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inserts = append(m.inserts, in)
	return mediadomain.Asset{ID: in.ID, Kind: in.Kind, StorageKey: in.StorageKey, MimeType: in.MimeType, Bytes: in.Bytes,
		DurationMs: in.DurationMs, OriginalFilename: in.OriginalFilename, CreatedAt: in.Now}, nil
}

func (m *mediaLibrary) Get(context.Context, string) (mediadomain.Asset, error) { return m.stored, nil }

func (m *mediaLibrary) Find(context.Context, access.Scope, string) (mediadomain.Asset, error) {
	return m.stored, m.findErr
}

func (m *mediaLibrary) Update(_ context.Context, in mediadomain.UpdateInput) (mediadomain.Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates = append(m.updates, in)
	updated := m.stored
	if in.DisplayName != nil {
		updated.DisplayName = *in.DisplayName
	}
	if in.SetDefaultMaxPlays {
		updated.DefaultMaxPlays = in.DefaultMaxPlays
	}
	return updated, nil
}

func (m *mediaLibrary) CountByChecksum(context.Context, string, []byte) (int, error) { return 0, nil }

func (m *mediaLibrary) List(_ context.Context, in mediadomain.ListInput) ([]mediadomain.Asset, paging.Page, error) {
	m.listed = append(m.listed, in)
	return []mediadomain.Asset{m.stored}, paging.Page{Number: 1, Size: 24, Total: 1}, nil
}

func (m *mediaLibrary) TotalBytes(_ context.Context, in mediadomain.ListInput) (int64, error) {
	m.totalled = append(m.totalled, in)
	return m.stored.Bytes, nil
}

func (m *mediaLibrary) Facets(_ context.Context, in mediadomain.ListInput) (mediadomain.Facets, error) {
	m.faceted = append(m.faceted, in)
	return mediadomain.Facets{All: 8, Audio: 5, Image: 3, Unused: 1}, nil
}

func (m *mediaLibrary) Usage(_ context.Context, scope access.Scope) (mediadomain.Usage, error) {
	m.usageOf = append(m.usageOf, scope)
	return mediadomain.Usage{AudioBytes: 1_127_428_915, ImageBytes: 214_748_364}, nil
}

func (m *mediaLibrary) QuestionCounts(context.Context, []string) (map[string]int, error) {
	return map[string]int{libraryAsset: 4}, nil
}

func (m *mediaLibrary) SoftDelete(context.Context, mediadomain.DeleteInput) error { return nil }

func (m *mediaLibrary) ReferencesFor(context.Context, []string) (map[string][]mediadomain.TestRef, error) {
	return nil, nil
}

func (m *mediaLibrary) ReachableByStudent(context.Context, string, string) (bool, error) {
	return false, nil
}

func (m *mediaLibrary) Readable(context.Context, access.Scope, []string) (map[string]mediadomain.Kind, error) {
	return nil, nil
}

func (m *mediaLibrary) Put(_ context.Context, _, _ string, body io.Reader, _ int64) error {
	read, err := io.Copy(io.Discard, body)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.puts = append(m.puts, read)
	return err
}

func (m *mediaLibrary) Delete(context.Context, string) error { return nil }

func (m *mediaLibrary) SignedURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://signed.test/" + key, nil
}

func (m *mediaLibrary) Audio(io.ReaderAt, int64) (string, int, error) {
	return "audio/mpeg", 252_000, nil
}

func cambridge() mediadomain.Asset {
	return mediadomain.Asset{
		ID: libraryAsset, Kind: mediadomain.KindAudio, StorageKey: "audio/" + libraryAsset + ".mp3", MimeType: "audio/mpeg",
		Bytes: 3_984_588, DurationMs: new(252_000), OriginalFilename: "cam15-t2-p1.mp3", DisplayName: "Cambridge 15 · Test 2 · Part 1.mp3",
		DefaultMaxPlays: new(2), CreatedAt: time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC),
	}
}

func mediaRouter(t *testing.T, issuer *identitytoken.Issuer) (http.Handler, *mediaLibrary) {
	t.Helper()
	library := &mediaLibrary{stored: cambridge()}
	modules := router.Modules{Media: mediahttp.NewMedia(mediaapp.New(library, library, library))}
	h, err := router.New(router.Deps{Modules: modules, Principals: rolePrincipals(), DB: fakeDB{}, Tokens: issuer},
		slog.New(slog.NewTextHandler(io.Discard, nil)), []string{"https://app.quizzivy.com"}, "")
	if err != nil {
		t.Fatal(err)
	}
	return h, library
}

type nothing struct{}

func (nothing) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type formPart struct {
	field, filename string
	bytes           int64
}

func streamUpload(t *testing.T, h http.Handler, issuer *identitytoken.Issuer, query string, parts ...formPart) *httptest.ResponseRecorder {
	t.Helper()
	body, wire := io.Pipe()
	form := multipart.NewWriter(wire)
	go func() {
		for _, p := range parts {
			header := textproto.MIMEHeader{}
			disposition := `form-data; name="` + p.field + `"`
			if p.filename != "" {
				disposition += `; filename="` + p.filename + `"`
				header.Set("Content-Type", "application/octet-stream")
			}
			header.Set("Content-Disposition", disposition)
			part, err := form.CreatePart(header)
			if err != nil {
				return
			}
			if _, err := io.CopyN(part, nothing{}, p.bytes); err != nil {
				return
			}
		}
		_ = form.Close()
		_ = wire.Close()
	}()
	req := httptest.NewRequest(http.MethodPost, "/teacher/media"+query, body)
	req.ContentLength = -1
	req.Header.Set("Content-Type", form.FormDataContentType())
	token, err := issuer.Issue(teacherUser, "admin", 0)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	_ = body.CloseWithError(io.ErrClosedPipe)
	return rec
}

func allocatedDuring(run func()) uint64 {
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	run()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestAudioOverTheLimitAnswers413ThroughTheRouterWithoutBeingHeld(t *testing.T) {
	issuer := testIssuer(t)
	h, library := mediaRouter(t, issuer)

	var rec *httptest.ResponseRecorder
	allocated := allocatedDuring(func() {
		rec = streamUpload(t, h, issuer, "", formPart{field: "file", filename: "khong-lo.mp3", bytes: 51 << 20})
	})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("51 MiB of audio answered %d, want 413: %s", rec.Code, rec.Body.String())
	}
	if code, message := errorCodeAndMessage(t, rec); code != "MEDIA_TOO_LARGE" || message != "Tệp vượt quá 50 MB. Vui lòng nén hoặc cắt ngắn tệp." {
		t.Errorf("answered %s %q, want MEDIA_TOO_LARGE naming 50 MB", code, message)
	}
	if len(library.puts) != 0 || len(library.inserts) != 0 {
		t.Errorf("a refused upload was put %v and inserted %v", library.puts, library.inserts)
	}
	if limit := uint64(4 << 20); allocated > limit {
		t.Errorf("the server allocated %.1f MiB for a 51 MiB upload, want under %d MiB: the body is being held in memory", float64(allocated)/(1<<20), limit>>20)
	}
}

func TestAudioOfExactlyTheLimitPassesTheTransportCap(t *testing.T) {
	issuer := testIssuer(t)
	h, library := mediaRouter(t, issuer)

	rec := streamUpload(t, h, issuer, "?defaultMaxPlays=2", formPart{field: "file", filename: "vua-du.mp3", bytes: mediadomain.MaxAudioBytes})
	if rec.Code != http.StatusCreated {
		t.Fatalf("audio of exactly the limit answered %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if len(library.puts) != 1 || library.puts[0] != mediadomain.MaxAudioBytes {
		t.Errorf("puts = %v, want the whole %d bytes", library.puts, mediadomain.MaxAudioBytes)
	}
	if len(library.inserts) != 1 || library.inserts[0].DefaultMaxPlays == nil || *library.inserts[0].DefaultMaxPlays != 2 || library.inserts[0].UploaderID != teacherUser {
		t.Errorf("inserts = %+v, want one by the caller with the play limit from the query", library.inserts)
	}
}

func TestFramingPastTheBodyCapAnswersTheFiles413(t *testing.T) {
	issuer := testIssuer(t)
	h, library := mediaRouter(t, issuer)

	rec := streamUpload(t, h, issuer, "",
		formPart{field: "note", bytes: 53 << 20},
		formPart{field: "file", filename: "sau-cung.mp3", bytes: 1024})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("a body over the transport cap answered %d, want 413: %s", rec.Code, rec.Body.String())
	}
	if code, _ := errorCodeAndMessage(t, rec); code != "MEDIA_TOO_LARGE" {
		t.Errorf("answered %s, want MEDIA_TOO_LARGE", code)
	}
	if len(library.puts) != 0 {
		t.Errorf("puts = %v, want none", library.puts)
	}
}

func TestAnUploadsPlayLimitIsCheckedBeforeTheBodyIsRead(t *testing.T) {
	issuer := testIssuer(t)
	h, library := mediaRouter(t, issuer)
	for _, query := range []string{"?defaultMaxPlays=4", "?defaultMaxPlays=-1", "?defaultMaxPlays=hai", "?defaultMaxPlays=1.5"} {
		rec := streamUpload(t, h, issuer, query, formPart{field: "file", filename: "nghe.mp3", bytes: 1024})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s answered %d, want 400: %s", query, rec.Code, rec.Body.String())
		}
	}
	if len(library.puts) != 0 || len(library.inserts) != 0 {
		t.Errorf("a refused play limit still stored %v and %v", library.puts, library.inserts)
	}
}

func sendPatch(t *testing.T, h http.Handler, issuer *identitytoken.Issuer, user, body string) *httptest.ResponseRecorder {
	t.Helper()
	return sendAs(t, h, issuer, http.MethodPatch, "/teacher/media/"+libraryAsset, user, body)
}

func TestUpdateMediaTakesANameAPlayLimitOrBoth(t *testing.T) {
	issuer := testIssuer(t)
	for name, c := range map[string]struct {
		body  string
		name  *string
		set   bool
		plays *int
	}{
		"a name":                {`{"displayName":"  Bài nghe sân bay  "}`, new("Bài nghe sân bay"), false, nil},
		"a play limit":          {`{"defaultMaxPlays":3}`, nil, true, new(3)},
		"unlimited plays":       {`{"defaultMaxPlays":0}`, nil, true, new(0)},
		"the limit cleared":     {`{"defaultMaxPlays":null}`, nil, true, nil},
		"both":                  {`{"displayName":"Tên mới","defaultMaxPlays":1}`, new("Tên mới"), true, new(1)},
		"a name of 200 letters": {`{"displayName":"` + strings.Repeat("ế", 200) + `"}`, new(strings.Repeat("ế", 200)), false, nil},
	} {
		h, library := mediaRouter(t, issuer)
		rec := sendPatch(t, h, issuer, teacherUser, c.body)
		if rec.Code != http.StatusOK {
			t.Errorf("%s answered %d, want 200: %s", name, rec.Code, rec.Body.String())
			continue
		}
		if len(library.updates) != 1 {
			t.Errorf("%s wrote %d updates, want one", name, len(library.updates))
			continue
		}
		in := library.updates[0]
		if (in.DisplayName == nil) != (c.name == nil) || (c.name != nil && *in.DisplayName != *c.name) {
			t.Errorf("%s wrote the name %v, want %v", name, in.DisplayName, c.name)
		}
		if in.SetDefaultMaxPlays != c.set || (in.DefaultMaxPlays == nil) != (c.plays == nil) || (c.plays != nil && *in.DefaultMaxPlays != *c.plays) {
			t.Errorf("%s wrote the limit set=%v %v, want set=%v %v", name, in.SetDefaultMaxPlays, in.DefaultMaxPlays, c.set, c.plays)
		}
		if in.ID != libraryAsset || in.ActorID != teacherUser || in.All {
			t.Errorf("%s ran on %s as %s with All=%v, want the path's file as the teacher", name, in.ID, in.ActorID, in.All)
		}
	}
}

func TestUpdateMediaAnswersTheFileAsTheLibraryListsIt(t *testing.T) {
	issuer := testIssuer(t)
	h, library := mediaRouter(t, issuer)
	rec := sendPatch(t, h, issuer, adminUser, `{"displayName":"Unit 4 · Airport announcements.mp3"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"id": libraryAsset, "kind": "audio", "mimeType": "audio/mpeg", "bytes": float64(3_984_588), "durationMs": float64(252_000),
		"originalFilename": "cam15-t2-p1.mp3", "displayName": "Unit 4 · Airport announcements.mp3", "defaultMaxPlays": float64(2),
		"width": nil, "height": nil, "questionCount": float64(4), "usageCount": float64(0),
		"url": "https://signed.test/audio/" + libraryAsset + ".mp3", "createdAt": "2026-10-01T08:00:00Z",
	}
	for field, value := range want {
		got, present := body[field]
		if !present || got != value {
			t.Errorf("%s = %v (present %v), want %v", field, got, present, value)
		}
	}
	if !library.updates[0].All {
		t.Error("the Admin's update ran without scope.all")
	}
}

func TestUpdateMediaRefusesWhatTheContractDoesNotAllow(t *testing.T) {
	issuer := testIssuer(t)
	h, library := mediaRouter(t, issuer)
	for name, body := range map[string]string{
		"an empty body":           `{}`,
		"an empty name":           `{"displayName":""}`,
		"a name of 201 letters":   `{"displayName":"` + strings.Repeat("ế", 201) + `"}`,
		"four plays":              `{"defaultMaxPlays":4}`,
		"a negative limit":        `{"defaultMaxPlays":-1}`,
		"a limit written as text": `{"defaultMaxPlays":"2"}`,
		"a fractional limit":      `{"defaultMaxPlays":1.5}`,
		"a property it lacks":     `{"originalFilename":"khac.mp3"}`,
		"a name that is not text": `{"displayName":7}`,
		"a name of spaces only":   `{"displayName":"   "}`,
	} {
		rec := sendPatch(t, h, issuer, teacherUser, body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s answered %d, want 400: %s", name, rec.Code, rec.Body.String())
			continue
		}
		if code := errorCode(t, rec); code != "VALIDATION_FAILED" {
			t.Errorf("%s answered %s, want VALIDATION_FAILED", name, code)
		}
	}
	if len(library.updates) != 0 {
		t.Errorf("a refused body was written: %+v", library.updates)
	}
}

func TestUpdateMediaIsForThoseWhoMayWriteMedia(t *testing.T) {
	issuer := testIssuer(t)
	h, library := mediaRouter(t, issuer)
	for user, want := range map[string]int{"": http.StatusUnauthorized, studentUser: http.StatusForbidden, assistantUser: http.StatusForbidden} {
		if rec := sendPatch(t, h, issuer, user, `{"displayName":"Tên mới"}`); rec.Code != want {
			t.Errorf("%q answered %d, want %d", user, rec.Code, want)
		}
	}
	if len(library.updates) != 0 {
		t.Errorf("a caller without the permission wrote %+v", library.updates)
	}

	library.findErr = mediadomain.ErrNotFound
	rec := sendPatch(t, h, issuer, teacherUser, `{"displayName":"Tên mới"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("a file outside the caller's library answered %d, want 404: %s", rec.Code, rec.Body.String())
	}
	if code, message := errorCodeAndMessage(t, rec); code != "NOT_FOUND" || message != "Không tìm thấy tệp." {
		t.Errorf("answered %s %q, want the not-found deleteMedia gives", code, message)
	}
}

func TestListMediaTakesItsFiltersFromTheQuery(t *testing.T) {
	issuer := testIssuer(t)
	h, library := mediaRouter(t, issuer)
	rec := sendAs(t, h, issuer, http.MethodGet, "/teacher/media?q=s%C3%A2n+bay&kind=audio&unused=true&page=2&limit=10", adminUser, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	own := access.Scope{UserID: adminUser}
	for name, got := range map[string][]mediadomain.ListInput{"the list": library.listed, "the total": library.totalled, "the facets": library.faceted} {
		if len(got) != 1 || got[0].Scope != own || got[0].Query != "sân bay" || !got[0].Unused || got[0].Kind == nil || *got[0].Kind != mediadomain.KindAudio {
			t.Errorf("%s read %+v, want the Admin's own rows, the search, audio and unused", name, got)
		}
	}
	if library.listed[0].Page != 2 || library.listed[0].Limit != 10 {
		t.Errorf("the list read page %d of %d, want page 2 of 10", library.listed[0].Page, library.listed[0].Limit)
	}
	if len(library.usageOf) != 1 || library.usageOf[0] != own {
		t.Errorf("usage was read in %+v, want %+v", library.usageOf, own)
	}

	var body struct {
		Facets map[string]float64 `json:"facets"`
		Usage  map[string]float64 `json:"usage"`
		Total  float64            `json:"total"`
		Items  []map[string]any   `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Facets["all"] != 8 || body.Facets["audio"] != 5 || body.Facets["image"] != 3 || body.Facets["unused"] != 1 || len(body.Facets) != 4 {
		t.Errorf("facets = %v", body.Facets)
	}
	if body.Usage["audioBytes"] != 1_127_428_915 || body.Usage["imageBytes"] != 214_748_364 || body.Usage["quotaBytes"] != float64(mediadomain.DefaultOwnerQuotaBytes) || len(body.Usage) != 3 {
		t.Errorf("usage = %v, want the library's bytes beside the default quota", body.Usage)
	}
	if len(body.Items) != 1 || body.Items[0]["displayName"] != "Cambridge 15 · Test 2 · Part 1.mp3" || body.Items[0]["questionCount"] != float64(4) || body.Items[0]["defaultMaxPlays"] != float64(2) {
		t.Errorf("items = %v", body.Items)
	}

	for _, query := range []string{"?unused=maybe", "?kind=video", "?q=" + strings.Repeat("a", 201)} {
		if rec := sendAs(t, h, issuer, http.MethodGet, "/teacher/media"+query, teacherUser, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s answered %d, want 400", query[:12], rec.Code)
		}
	}

	h, library = mediaRouter(t, issuer)
	if rec := sendAs(t, h, issuer, http.MethodGet, "/teacher/media", teacherUser, ""); rec.Code != http.StatusOK {
		t.Fatalf("the bare list answered %d", rec.Code)
	}
	if in := library.listed[0]; in.Query != "" || in.Unused || in.Kind != nil || in.Scope != (access.Scope{UserID: teacherUser}) {
		t.Errorf("the bare list read %+v, want no filter in the teacher's own scope", in)
	}
	h, library = mediaRouter(t, issuer)
	if rec := sendAs(t, h, issuer, http.MethodGet, "/teacher/media?unused=false", teacherUser, ""); rec.Code != http.StatusOK || library.listed[0].Unused {
		t.Errorf("unused=false answered %d and filtered %v, want the whole library", rec.Code, library.listed[0].Unused)
	}
}
