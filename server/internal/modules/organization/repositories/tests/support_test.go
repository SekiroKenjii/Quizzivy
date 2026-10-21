//go:build integration

package repositories_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/modules/organization/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/limits"
)

const (
	migrateEnv = "TEST_DATABASE_URL"
	appEnv     = "APP_DATABASE_URL"
)

func connect(t *testing.T, env string) *pgxpool.Pool { return connectMax(t, env, 0) }

func connectMax(t *testing.T, env string, maxConns int32) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(env)
	if dsn == "" {
		t.Skipf("%s is not set (TEST_DATABASE_URL or APP_DATABASE_URL)", env)
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pool config: %v", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func nonce(t *testing.T) string {
	t.Helper()
	return hex.EncodeToString(randomBytes(t, 8))
}

func uniqueInstant(t *testing.T) time.Time {
	t.Helper()
	micros := binary.BigEndian.Uint64(randomBytes(t, 8)) >> 24
	return time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(micros) * time.Microsecond)
}

func intp(n int) *int { return &n }

func digest(seed byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = seed + byte(i)
	}
	return out
}

type session struct {
	t      *testing.T
	tx     pgx.Tx
	repo   *repositories.Postgres
	nonce  string
	now    time.Time
	agent  string
	serial int
	custom string
}

func begin(t *testing.T, level pgx.TxIsoLevel, lockRow bool) *session {
	t.Helper()
	pool := connect(t, migrateEnv)
	ctx := context.Background()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: level})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rolling back the test's transaction: %v", err)
		}
	})
	s := &session{t: t, tx: tx, repo: repositories.NewPostgres(db.NewContext(tx)), nonce: nonce(t), now: uniqueInstant(t)}
	s.agent = "organization-test/" + s.nonce
	s.exec(`SET LOCAL lock_timeout = '30s'`)
	if lockRow {
		s.exec(`SELECT 1 FROM app.organization_settings FOR UPDATE`)
	}
	return s
}

func lockedSession(t *testing.T) *session { return begin(t, pgx.ReadCommitted, true) }

func snapshotSession(t *testing.T) *session { return begin(t, pgx.RepeatableRead, false) }

func (s *session) exec(sql string, args ...any) {
	s.t.Helper()
	if _, err := s.tx.Exec(context.Background(), sql, args...); err != nil {
		s.t.Fatalf("%s: %v", sql, err)
	}
}

func (s *session) roleID(key string) string {
	s.t.Helper()
	ctx := context.Background()
	var id string
	if key == "custom" {
		if s.custom == "" {
			if err := s.tx.QueryRow(ctx, `
				INSERT INTO app.roles (name, icon, color) VALUES ($1, 'user', 'gray') RETURNING id::text`,
				"Vai trò thử "+s.nonce).Scan(&id); err != nil {
				s.t.Fatalf("custom role: %v", err)
			}
			s.custom = id
		}
		return s.custom
	}
	if err := s.tx.QueryRow(ctx, `SELECT id::text FROM app.roles WHERE builtin_key = $1`, key).Scan(&id); err != nil {
		s.t.Fatalf("role %s: %v", key, err)
	}
	return id
}

type account struct {
	role     string
	password bool
	disabled bool
	google   string
}

func (s *session) addAccount(a account) string {
	s.t.Helper()
	ctx := context.Background()
	s.serial++
	var hash *string
	if a.password {
		h := "$argon2id$not-a-real-hash"
		hash = &h
	}
	var id string
	if err := s.tx.QueryRow(ctx, `
		INSERT INTO app.users (email, full_name, role_id, password_hash, disabled_at)
		VALUES ($1, 'Người dùng thử', $2::uuid, $3, CASE WHEN $4::boolean THEN now() END)
		RETURNING id::text`,
		fmt.Sprintf("org-%s-%d@example.com", s.nonce, s.serial), s.roleID(a.role), hash, a.disabled).Scan(&id); err != nil {
		s.t.Fatalf("account %+v: %v", a, err)
	}
	if a.google != "" {
		s.exec(`
			INSERT INTO app.user_identities (user_id, provider, provider_user_id, email_at_link)
			VALUES ($1::uuid, 'google', $2, $3)`,
			id, fmt.Sprintf("sub-%s-%d", s.nonce, s.serial), a.google)
	}
	return id
}

func (s *session) domains() []string {
	return []string{"hoa-mai-" + s.nonce + ".example", "truong-" + s.nonce + ".example"}
}

func (s *session) coverAdmin() string {
	s.t.Helper()
	return s.addAccount(account{role: "admin", password: true, google: "quan.tri@" + s.domains()[0]})
}

func (s *session) fullDocument() domain.Settings {
	return domain.Settings{
		Profile: domain.Profile{
			Name:           "Trung tâm Ngoại ngữ Hoa Mai",
			ShortName:      "Hoa Mai",
			Website:        "https://hoamai.example",
			ContactEmail:   "lienhe@hoamai.example",
			FrontDeskPhone: "+84 28 3822 0000",
			FrontDeskHours: "Thứ Hai đến Thứ Sáu, 8:00 - 17:00",
			Address:        "12 Nguyễn Huệ, Quận 1, Thành phố Hồ Chí Minh",
			LightLogo:      &domain.Logo{SHA256: digest(1), Type: domain.LogoPNG},
			DarkLogo:       &domain.Logo{SHA256: digest(40), Type: domain.LogoPNG},
		},
		SignIn: domain.SignIn{
			Google:               true,
			Password:             true,
			ForcePasswordChange:  false,
			JoinCodes:            false,
			AllowedGoogleDomains: s.domains(),
		},
		Retention: domain.Retention{
			KeepAnswersMonths:            intp(24),
			KeepFocusEventsMonths:        6,
			AnonymiseDisabledAfterMonths: intp(12),
		},
		Limits: limits.Stored{limits.Seats: 400, limits.EmailsPerDay: 80},
	}
}

func bareDocument() domain.Settings {
	return domain.Settings{
		Profile: domain.Profile{Name: "Chỉ có tên"},
		SignIn: domain.SignIn{
			Google:               true,
			Password:             true,
			ForcePasswordChange:  true,
			JoinCodes:            true,
			AllowedGoogleDomains: []string{},
		},
		Retention: domain.Retention{KeepFocusEventsMonths: domain.DefaultFocusMonths},
	}
}

func (s *session) save(doc domain.Settings, expected int64, actor string) domain.Save {
	return domain.Save{
		Settings:         doc,
		ExpectedRevision: expected,
		ActorID:          actor,
		IP:               "203.0.113.7",
		UserAgent:        s.agent,
		Now:              s.now,
		GoogleConfigured: true,
	}
}

func (s *session) stored() domain.Settings {
	s.t.Helper()
	got, err := s.repo.Read(context.Background())
	if err != nil {
		s.t.Fatalf("read: %v", err)
	}
	return got
}

func (s *session) raw() rawRow {
	s.t.Helper()
	return readRaw(s.t, s.tx)
}

func (s *session) dbNow() time.Time {
	s.t.Helper()
	var now time.Time
	if err := s.tx.QueryRow(context.Background(), `SELECT now()`).Scan(&now); err != nil {
		s.t.Fatalf("now: %v", err)
	}
	return now
}

type rawRow struct {
	name         string
	shortName    *string
	website      *string
	contactEmail *string
	phone        *string
	hours        *string
	address      *string
	logo         []byte
	logoType     *string
	logoDark     []byte
	logoDarkType *string
	google       bool
	password     bool
	force        bool
	joinCodes    bool
	domains      []string
	keepAnswers  *int16
	keepFocus    int16
	anonymise    *int16
	revision     int32
	updatedBy    *string
	updatedAt    time.Time
}

func (r rawRow) same(other rawRow) bool {
	a, b := r, other
	a.updatedAt, b.updatedAt = time.Time{}, time.Time{}
	return reflect.DeepEqual(a, b) && r.updatedAt.Equal(other.updatedAt)
}

func readRaw(t *testing.T, q db.Querier) rawRow {
	t.Helper()
	return readRawTable(t, q, "app.organization_settings")
}

func readRawTable(t *testing.T, q db.Querier, table string) rawRow {
	t.Helper()
	var r rawRow
	if err := q.QueryRow(context.Background(), `
		SELECT name, short_name, website, contact_email, front_desk_phone, front_desk_hours, address,
		       logo_sha256, logo_type, logo_dark_sha256, logo_dark_type,
		       google_sign_in, password_sign_in, force_password_change, join_codes_enabled, allowed_google_domains,
		       keep_answers_months, keep_focus_events_months, anonymise_disabled_after_months,
		       revision, updated_by::text, updated_at
		  FROM `+table).Scan(
		&r.name, &r.shortName, &r.website, &r.contactEmail, &r.phone, &r.hours, &r.address,
		&r.logo, &r.logoType, &r.logoDark, &r.logoDarkType,
		&r.google, &r.password, &r.force, &r.joinCodes, &r.domains,
		&r.keepAnswers, &r.keepFocus, &r.anonymise,
		&r.revision, &r.updatedBy, &r.updatedAt); err != nil {
		t.Fatalf("raw row of %s: %v", table, err)
	}
	return r
}

func textOrNil(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func logoColumns(l *domain.Logo) ([]byte, *string) {
	if l == nil {
		return nil, nil
	}
	kind := string(l.Type)
	return l.SHA256, &kind
}

func months16(m *int) *int16 {
	if m == nil {
		return nil
	}
	v := int16(*m)
	return &v
}

func rawOf(d domain.Settings, revision int32, updatedBy string, updatedAt time.Time) rawRow {
	light, lightType := logoColumns(d.Profile.LightLogo)
	dark, darkType := logoColumns(d.Profile.DarkLogo)
	return rawRow{
		name:         d.Profile.Name,
		shortName:    textOrNil(d.Profile.ShortName),
		website:      textOrNil(d.Profile.Website),
		contactEmail: textOrNil(d.Profile.ContactEmail),
		phone:        textOrNil(d.Profile.FrontDeskPhone),
		hours:        textOrNil(d.Profile.FrontDeskHours),
		address:      textOrNil(d.Profile.Address),
		logo:         light,
		logoType:     lightType,
		logoDark:     dark,
		logoDarkType: darkType,
		google:       d.SignIn.Google,
		password:     d.SignIn.Password,
		force:        d.SignIn.ForcePasswordChange,
		joinCodes:    d.SignIn.JoinCodes,
		domains:      d.SignIn.AllowedGoogleDomains,
		keepAnswers:  months16(d.Retention.KeepAnswersMonths),
		keepFocus:    int16(d.Retention.KeepFocusEventsMonths),
		anonymise:    months16(d.Retention.AnonymiseDisabledAfterMonths),
		revision:     revision,
		updatedBy:    textOrNil(updatedBy),
		updatedAt:    updatedAt,
	}
}

func sameSettings(a, b domain.Settings) bool {
	x, y := a, b
	x.UpdatedAt, y.UpdatedAt = time.Time{}, time.Time{}
	x.Limits, y.Limits = nil, nil
	return reflect.DeepEqual(x, y) && a.UpdatedAt.Equal(b.UpdatedAt) && a.Limits.Equal(b.Limits)
}

func sameDocument(t *testing.T, what string, got, want domain.Settings) {
	t.Helper()
	if !reflect.DeepEqual(got.Profile, want.Profile) {
		t.Errorf("%s: profile = %+v, want %+v", what, got.Profile, want.Profile)
	}
	if !reflect.DeepEqual(got.SignIn, want.SignIn) {
		t.Errorf("%s: sign-in = %+v, want %+v", what, got.SignIn, want.SignIn)
	}
	if !reflect.DeepEqual(got.Retention, want.Retention) {
		t.Errorf("%s: retention = %+v, want %+v", what, got.Retention, want.Retention)
	}
	if !got.Limits.Equal(want.Limits) {
		t.Errorf("%s: limits = %v, want %v", what, got.Limits, want.Limits)
	}
}

type auditRow struct {
	actor      *string
	action     string
	entity     string
	entityID   *string
	occurredAt time.Time
	ip         *string
	userAgent  *string
	diff       []byte
}

func (s *session) audits() []auditRow {
	s.t.Helper()
	rows, err := s.tx.Query(context.Background(), `
		SELECT actor_user_id::text, action, entity, entity_id::text, occurred_at, host(ip), user_agent, diff
		  FROM app.audit_log
		 WHERE entity = 'organization' AND occurred_at = $1
		 ORDER BY id`, s.now)
	if err != nil {
		s.t.Fatalf("audit rows: %v", err)
	}
	defer rows.Close()
	var out []auditRow
	for rows.Next() {
		var a auditRow
		if err := rows.Scan(&a.actor, &a.action, &a.entity, &a.entityID, &a.occurredAt, &a.ip, &a.userAgent, &a.diff); err != nil {
			s.t.Fatalf("audit row: %v", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		s.t.Fatalf("audit rows: %v", err)
	}
	return out
}

type auditDiff struct {
	Fields   []string        `json:"fields"`
	Limits   []limits.Change `json:"limits"`
	Revision int64           `json:"revision"`
}

func decodeDiff(t *testing.T, raw []byte) auditDiff {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var d auditDiff
	if err := dec.Decode(&d); err != nil {
		t.Fatalf("audit diff %s: %v", raw, err)
	}
	if d.Fields == nil {
		t.Fatalf("audit diff %s: fields is not an array", raw)
	}
	return d
}

func rolledBack(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Errorf("rollback: %v", err)
		}
	}()
	if _, err := tx.Exec(ctx, `SET LOCAL lock_timeout = '30s'`); err != nil {
		t.Fatalf("lock timeout: %v", err)
	}
	fn(ctx, tx)
}
