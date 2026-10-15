package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"quizzivy/internal/modules/organization/application"
	"quizzivy/internal/modules/organization/application/ports"
	"quizzivy/internal/modules/organization/domain"
)

type fakeRepo struct {
	mu       sync.Mutex
	settings domain.Settings
	reads    int
	saves    []domain.Save
	readErr  error
	impact   domain.Impact
	block    chan struct{}
	onRead   func(reads int, settings *domain.Settings)
}

func (f *fakeRepo) Read(context.Context) (domain.Settings, error) {
	f.mu.Lock()
	f.reads++
	if f.onRead != nil {
		f.onRead(f.reads, &f.settings)
	}
	err, settings, block := f.readErr, f.settings, f.block
	f.mu.Unlock()
	if block != nil {
		<-block
	}
	return settings, err
}

func (f *fakeRepo) Save(_ context.Context, in domain.Save) (domain.Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saves = append(f.saves, in)
	if in.ExpectedRevision != f.settings.Revision {
		return domain.Settings{}, domain.ErrStale
	}
	if f.impact.Lockout() {
		return domain.Settings{}, &domain.LockedOut{Impact: f.impact}
	}
	saved := in.Settings
	saved.Revision = f.settings.Revision + 1
	saved.UpdatedBy = in.ActorID
	saved.UpdatedAt = in.Now
	f.settings = saved
	return saved, nil
}

func (f *fakeRepo) Impact(_ context.Context, _ domain.SignIn, _ string, googleConfigured bool) (domain.Impact, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saves = append(f.saves, domain.Save{GoogleConfigured: googleConfigured})
	return f.impact, nil
}

func (f *fakeRepo) readCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads
}

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func stored() domain.Settings {
	return domain.Settings{
		Profile:   domain.Profile{Name: "Quizzivy"},
		SignIn:    domain.SignIn{Google: true, Password: true, ForcePasswordChange: true, JoinCodes: true, AllowedGoogleDomains: []string{}},
		Retention: domain.Retention{KeepFocusEventsMonths: 13},
		Revision:  1,
	}
}

func build(t *testing.T, repo *fakeRepo) (*application.Application, *clock) {
	t.Helper()
	c := &clock{now: time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)}
	if repo.impact == (domain.Impact{}) {
		repo.impact = domain.Impact{ActiveAdmins: 2}
	}
	app := application.New(repo, slog.New(slog.DiscardHandler), true)
	app.SetClock(c.Now)
	return app, c
}

type memoryStore struct {
	mu        sync.Mutex
	objects   map[string][]byte
	written   map[string]time.Time
	puts      int
	gets      int
	listErr   error
	deleteErr map[string]error
	deleted   []string
	listed    int
	onList    func()
}

func newStore() *memoryStore {
	return &memoryStore{objects: map[string][]byte{}, written: map[string]time.Time{}, deleteErr: map[string]error{}}
}

func (m *memoryStore) put(key string, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = []byte(key)
	m.written[key] = at
}

func (m *memoryStore) List(_ context.Context, prefix string, limit int) ([]ports.StoredLogo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listed = limit
	if m.onList != nil {
		m.onList()
	}
	if m.listErr != nil {
		return nil, m.listErr
	}
	var out []ports.StoredLogo
	for key := range m.objects {
		if strings.HasPrefix(key, prefix) {
			out = append(out, ports.StoredLogo{Key: key, WrittenAt: m.written[key]})
		}
	}
	slices.SortFunc(out, func(a, b ports.StoredLogo) int { return strings.Compare(a.Key, b.Key) })
	return out[:min(limit, len(out))], nil
}

func (m *memoryStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.deleteErr[key]; err != nil {
		return err
	}
	delete(m.objects, key)
	delete(m.written, key)
	m.deleted = append(m.deleted, key)
	return nil
}

func (m *memoryStore) Put(_ context.Context, key string, _ domain.LogoType, body, digest []byte) error {
	sum := sha256.Sum256(body)
	if !bytes.Equal(sum[:], digest) {
		return errors.New("digest does not match the body")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.puts++
	m.objects[key] = body
	m.written[key] = time.Now()
	return nil
}

func (m *memoryStore) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gets++
	body, ok := m.objects[key]
	if !ok {
		return nil, domain.ErrNoLogo
	}
	return body, nil
}

func (m *memoryStore) Exists(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objects[key]
	return ok, nil
}

type fixedChecker struct {
	file domain.LogoFile
	err  error
}

func (c fixedChecker) Check(_ context.Context, body io.Reader) (domain.LogoFile, error) {
	if _, err := io.ReadAll(body); err != nil {
		return domain.LogoFile{}, err
	}
	return c.file, c.err
}

func hexOf(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, v := range b {
		out = append(out, digits[v>>4], digits[v&15])
	}
	return string(out)
}
