//go:build integration

package http_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"quizzivy/gen/openapi"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/core/router"
	identitytoken "quizzivy/internal/modules/identity/application/token"
	"quizzivy/internal/modules/imports/application"
	"quizzivy/internal/modules/imports/domain"
	importshttp "quizzivy/internal/modules/imports/http"
	"quizzivy/internal/modules/imports/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/platform/storage"
	"strings"
	"testing"
	"time"
)

type intake struct {
	handler               http.Handler
	token, student, actor string
	pool                  *pgxpool.Pool
	repo                  *repositories.Postgres
	store                 *storage.Client
}

func setup(t *testing.T) intake {
	t.Helper()
	ctx := context.Background()
	if os.Getenv("S3_ENDPOINT") == "" {
		t.Skip("S3_ENDPOINT is required for real private intake")
	}
	pool, err := pgxpool.New(ctx, db.TestDSN(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	conn := pool
	if dsn := os.Getenv("TEST_APP_DATABASE_URL"); dsn != "" {
		conn, err = pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(conn.Close)
	}
	var actor string
	if err := pool.QueryRow(ctx, `INSERT INTO app.users(email,full_name,role) VALUES($1,'Private intake teacher','admin') RETURNING id::text`, uuid.NewString()+"@example.test").Scan(&actor); err != nil {
		t.Fatal(err)
	}
	store, err := storage.New(ctx, storage.Config{Endpoint: os.Getenv("S3_ENDPOINT"), Region: os.Getenv("S3_REGION"), Bucket: os.Getenv("S3_BUCKET"), AccessKeyID: os.Getenv("S3_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"), ForcePathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	repo := repositories.NewPostgres(db.NewContext(conn))
	app := application.New(application.Dependencies{Repo: repo, Drafts: repo, Runs: repo, Artifacts: repo, Store: store, Inspector: adapters.ImportInspector{}, WorkDir: t.TempDir(), Quotas: domain.DefaultQuotas()})
	issuer, err := identitytoken.NewIssuer([]byte(strings.Repeat("k", 32)), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	token, err := issuer.Issue(actor, "admin")
	if err != nil {
		t.Fatal(err)
	}
	student, err := issuer.Issue(uuid.NewString(), "student")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := router.New(router.Deps{Tokens: issuer, Modules: router.Modules{Imports: importshttp.New(app)}}, slog.New(slog.NewTextHandler(io.Discard, nil)), []string{"http://localhost:4173"}, "")
	if err != nil {
		t.Fatal(err)
	}
	h := intake{handler: handler, token: token, student: student, actor: actor, pool: pool, repo: repo, store: store}
	t.Cleanup(func() { h.cleanup(t) })
	return h
}
func (h intake) cleanup(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	rows, err := h.pool.Query(ctx, `SELECT storage_key FROM app.word_import_sources WHERE uploaded_by=$1`, h.actor)
	if err != nil {
		t.Error(err)
		return
	}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Error(err)
			continue
		}
		if err := h.store.Delete(ctx, key); err != nil {
			t.Error(err)
		}
	}
	rows.Close()
	for _, sql := range []string{
		`UPDATE app.word_imports SET source_revision=NULL WHERE created_by=$1`,
		`DELETE FROM app.word_import_source_set_items WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
		`DELETE FROM app.word_import_sources WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
		`DELETE FROM app.word_import_source_sets WHERE import_id IN (SELECT id FROM app.word_imports WHERE created_by=$1)`,
		`DELETE FROM app.word_imports WHERE created_by=$1`,
	} {
		if _, err := h.pool.Exec(ctx, sql, h.actor); err != nil {
			t.Error(err)
		}
	}
}
func (h intake) request(method, path, contentType string, body io.Reader, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec
}
func (h intake) create(t *testing.T) openapi.WordImport {
	t.Helper()
	body := `{"requestId":"` + uuid.NewString() + `","title":"Đề kiểm tra"}`
	rec := h.request(http.MethodPost, "/admin/imports", "application/json", strings.NewReader(body), h.token)
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var v openapi.WordImport
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func docx(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	parts := map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Synthetic answer evidence</w:t></w:r></w:p></w:body></w:document>`,
	}
	for name, data := range parts {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func multipartBody(t *testing.T, data []byte, extra bool) (*bytes.Buffer, string) {
	t.Helper()
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	part, err := writer.CreateFormFile("file", "Đề.docx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if extra {
		if err := writer.WriteField("unexpected", "must reject"); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &b, writer.FormDataContentType()
}

func TestPrivateIntakeRoundTripsThroughRouterPostgresAndMinIO(t *testing.T) {
	h := setup(t)
	v := h.create(t)
	data := docx(t)
	path := "/admin/imports/" + v.Id.String() + "/sources?role=exam&uploadId=" + uuid.NewString() + "&expectedRevision=1"
	bad, kind := multipartBody(t, data, true)
	invalid := h.request(http.MethodPost, path, kind, bad, h.token)
	if invalid.Code != 400 {
		t.Fatalf("additional part accepted: %d %s", invalid.Code, invalid.Body.String())
	}
	body, kind := multipartBody(t, data, false)
	rec := h.request(http.MethodPost, path, kind, body, h.token)
	if rec.Code != 201 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	var receipt openapi.ImportUploadReceipt
	if err := json.Unmarshal(rec.Body.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	body, kind = multipartBody(t, data, false)
	retry := h.request(http.MethodPost, path, kind, body, h.token)
	if retry.Code != 201 {
		t.Fatalf("retry: %d %s", retry.Code, retry.Body.String())
	}
	var replay openapi.ImportUploadReceipt
	if err := json.Unmarshal(retry.Body.Bytes(), &replay); err != nil {
		t.Fatal(err)
	}
	if replay.Source.Id != receipt.Source.Id || replay.Import.Revision != 2 || replay.SourceRevision != 1 {
		t.Fatal("retry created another source")
	}
	downloadPath := "/admin/imports/" + v.Id.String() + "/sources/" + receipt.Source.Id.String() + "/download"
	denied := h.request(http.MethodGet, downloadPath, "", nil, h.student)
	if denied.Code != 403 {
		t.Fatalf("student download: %d", denied.Code)
	}
	other := h.create(t)
	wrong := strings.Replace(downloadPath, v.Id.String(), other.Id.String(), 1)
	if rec := h.request(http.MethodGet, wrong, "", nil, h.token); rec.Code != 404 {
		t.Fatalf("cross-import access: %d", rec.Code)
	}
	access := h.request(http.MethodGet, downloadPath, "", nil, h.token)
	if access.Code != 200 || access.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("download access: %d", access.Code)
	}
	var link openapi.ImportSourceDownload
	if err := json.Unmarshal(access.Body.Bytes(), &link); err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(link.Url)
	if err != nil {
		t.Fatal("authorized source download failed")
	}
	defer response.Body.Close()
	downloaded, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != 200 || !bytes.Equal(downloaded, data) {
		t.Fatal("original bytes did not survive intake")
	}
	if !strings.HasPrefix(response.Header.Get("Content-Disposition"), "attachment") {
		t.Fatal("original can be interpreted as inline content")
	}
	current, err := h.repo.Get(context.Background(), v.Id.String())
	if err != nil || current.PendingUploads != 0 || len(current.Sources) != 1 {
		t.Fatalf("source state: %+v %v", current, err)
	}
	var mediaCount int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM app.media_assets WHERE id=$1`, receipt.Source.Id).Scan(&mediaCount); err != nil || mediaCount != 0 {
		t.Fatal("source leaked into learner media")
	}
}
