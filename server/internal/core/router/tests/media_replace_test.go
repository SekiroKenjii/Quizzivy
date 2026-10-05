package router_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"quizzivy/internal/modules/media/domain"
	"quizzivy/internal/shared/access"
	"strings"
	"testing"
)

func (m *mediaLibrary) FindReplacementTarget(ctx context.Context, scope access.Scope, id string) (domain.ReplacementTarget, error) {
	a, err := m.Find(ctx, scope, id)
	return domain.ReplacementTarget{Asset: a, OwnerID: scope.UserID}, err
}
func (m *mediaLibrary) Replace(ctx context.Context, in domain.ReplaceInput) (domain.ReplaceResult, error) {
	a, err := m.Insert(ctx, in.Asset)
	return domain.ReplaceResult{Asset: a, Repointed: domain.ReplacementCounts{Questions: 2, Groups: 1}, Left: domain.ReplacementCounts{Questions: 3, Groups: 4}}, err
}

type unreadReplacementBody struct {
	t     *testing.T
	reads int
}

func (b *unreadReplacementBody) Read([]byte) (int, error) {
	b.reads++
	b.t.Error("refused replacement read multipart bytes")
	return 0, io.EOF
}
func TestReplacementRouterMissingAndForeignPreflightLeaveBodyUnread(t *testing.T) {
	for _, id := range []string{libraryAsset, "01935000-0000-7000-8000-00000000ffff"} {
		t.Run(id, func(t *testing.T) {
			issuer := testIssuer(t)
			h, m := mediaRouter(t, issuer)
			m.findErr = domain.ErrNotFound
			body := &unreadReplacementBody{t: t}
			req := httptest.NewRequest(http.MethodPost, "/teacher/media/"+id+"/replace", body)
			req.ContentLength = -1
			req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
			token, err := issuer.Issue(teacherUser, "admin", 0)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != 404 || body.reads != 0 || len(m.puts) != 0 || len(m.inserts) != 0 {
				t.Fatalf("code=%d reads=%d puts=%v inserts=%v body=%s", rec.Code, body.reads, m.puts, m.inserts, rec.Body.String())
			}
		})
	}
}
func TestReplacementRouterKindRefusalIsLocalizedAndDoesNotPut(t *testing.T) {
	issuer := testIssuer(t)
	h, m := mediaRouter(t, issuer)
	for _, lang := range []string{"vi", "en"} {
		raw := "--test\r\nContent-Disposition: form-data; name=\"file\"; filename=\"hinh.png\"\r\nContent-Type: application/octet-stream\r\n\r\n" + string([]byte{137, 80, 78, 71, 13, 10, 26, 10}) + strings.Repeat("x", 24) + "\r\n--test--\r\n"
		req := httptest.NewRequest(http.MethodPost, "/teacher/media/"+libraryAsset+"/replace", strings.NewReader(raw))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
		req.Header.Set("Accept-Language", lang)
		token, err := issuer.Issue(teacherUser, "admin", 0)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		code, message := errorCodeAndMessage(t, rec)
		want := "Tệp thay thế phải có cùng loại với tệp cũ."
		if lang == "en" {
			want = "The replacement must have the same media kind as the old file."
		}
		if rec.Code != 415 || code != "MEDIA_KIND_MISMATCH" || message != want || len(m.puts) != 0 || len(m.inserts) != 0 {
			t.Fatalf("%s code=%d error=%s %q calls=%v/%v", lang, rec.Code, code, message, m.puts, m.inserts)
		}
	}
}
