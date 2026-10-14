package http_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/audit/application"
	"quizzivy/internal/modules/audit/domain"
	audithttp "quizzivy/internal/modules/audit/http"
	"quizzivy/internal/platform/httpx"
)

const (
	viewer    = "01935000-0000-7000-8000-0000000000c3"
	teacherID = "01935000-0000-7000-8000-0000000000a1"
)

var vietnam = domain.LocationOf(domain.DefaultZone)

func march(day, hour, minute int) time.Time {
	return time.Date(2001, time.March, day, hour, minute, 0, 0, vietnam)
}

type log struct {
	mu      sync.Mutex
	records []domain.Record
	labels  map[domain.EntityRef]string

	exports       []domain.Export
	countRows     *int
	countErr      error
	failBatchFrom int
	batches       int
	batchSawAudit []bool
}

func (l *log) matching(f domain.Filter) []domain.Record {
	var out []domain.Record
	entities := map[string]bool{}
	if f.Category != "" {
		for _, e := range domain.EntitiesOf(f.Category) {
			entities[e] = true
		}
	}
	for _, r := range l.records {
		switch {
		case f.Window.From != nil && r.OccurredAt.Before(*f.Window.From):
		case f.Window.To != nil && !r.OccurredAt.Before(*f.Window.To):
		case f.Through > 0 && r.ID > f.Through:
		case f.Category != "" && !entities[r.EntityType]:
		case f.Query != "" && !strings.Contains(domain.Fold(r.Action+" "+nameOf(r)), domain.Fold(f.Query)) && !slices.Contains(f.Actions, r.Action):
		default:
			out = append(out, r)
		}
	}
	slices.SortFunc(out, func(a, b domain.Record) int {
		if c := b.OccurredAt.Compare(a.OccurredAt); c != 0 {
			return c
		}
		return int(b.ID - a.ID)
	})
	return out
}

func nameOf(r domain.Record) string {
	if r.Actor == nil {
		return ""
	}
	return r.Actor.Name
}

func (l *log) List(_ context.Context, f domain.Filter, offset, limit int) ([]domain.Record, error) {
	rows := l.matching(f)
	if offset >= len(rows) {
		return nil, nil
	}
	return rows[offset:min(offset+limit, len(rows))], nil
}

func (l *log) Facets(_ context.Context, f domain.Filter) (domain.Facets, error) {
	f.Category = ""
	per := map[string]int{}
	for _, r := range l.matching(f) {
		per[r.EntityType]++
	}
	return domain.FoldFacets(per), nil
}

func (l *log) Labels(_ context.Context, refs []domain.EntityRef) (map[domain.EntityRef]string, error) {
	out := map[domain.EntityRef]string{}
	for _, ref := range refs {
		if label, ok := l.labels[ref]; ok {
			out[ref] = label
		}
	}
	return out, nil
}

func (l *log) CountExport(_ context.Context, f domain.Filter) (domain.ExportCount, error) {
	if l.countErr != nil {
		return domain.ExportCount{}, l.countErr
	}
	if l.countRows != nil {
		return domain.ExportCount{Rows: *l.countRows, Through: 1 << 40}, nil
	}
	rows := l.matching(f)
	count := domain.ExportCount{Rows: len(rows)}
	for _, r := range rows {
		count.Through = max(count.Through, r.ID)
	}
	return count, nil
}

func (l *log) ExportBatch(_ context.Context, f domain.Filter, after *domain.Cursor, limit int) ([]domain.Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.batches++
	l.batchSawAudit = append(l.batchSawAudit, len(l.exports) > 0)
	if l.failBatchFrom > 0 && l.batches >= l.failBatchFrom {
		return nil, errors.New("the database went away")
	}
	var rows []domain.Record
	for _, r := range l.matching(f) {
		if after == nil || r.OccurredAt.Before(after.OccurredAt) || (r.OccurredAt.Equal(after.OccurredAt) && r.ID < after.ID) {
			rows = append(rows, r)
		}
	}
	return rows[:min(limit, len(rows))], nil
}

func (l *log) RecordExport(_ context.Context, e domain.Export) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.exports = append(l.exports, e)
	return nil
}

var _ domain.Repository = (*log)(nil)

type photos struct{}

func (photos) AvatarURL(_ context.Context, key string) (string, error) {
	return "https://photos.example/" + key, nil
}

func (l *log) transport() audithttp.Audit {
	app := application.New(l).WithAvatars(photos{})
	app.SetClock(func() time.Time { return march(15, 12, 0) })
	return audithttp.NewAudit(app).WithLogger(slog.New(slog.DiscardHandler))
}

type serving func(ctx context.Context, w http.ResponseWriter) error

func answered(t *testing.T, acceptLanguage string, serve serving) (rec *httptest.ResponseRecorder, aborted bool) {
	t.Helper()
	handler := httpx.WithRequestMeta(func(*http.Request) string { return "203.0.113.9" })(
		httpx.RequireAuth(nil, func(string) (httpx.Principal, error) {
			return httpx.Principal{UserID: viewer}, nil
		})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					if recovered != http.ErrAbortHandler {
						panic(recovered)
					}
					aborted = true
				}
			}()
			if err := serve(r.Context(), w); err != nil {
				t.Fatal(err)
			}
		})))
	request := httptest.NewRequest(http.MethodGet, "/admin/audit-log", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	request.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) Firefox/120.0")
	if acceptLanguage != "" {
		request.Header.Set("Accept-Language", acceptLanguage)
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, request)
	return rec, aborted
}

func record(id int64, at time.Time, action, entity string) domain.Record {
	r := domain.Record{ID: id, OccurredAt: at, Action: action, EntityType: entity}
	return r
}

func ptr[T any](v T) *T { return &v }
