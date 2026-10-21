// Package limits is the centre's usage limits (CR-002, D-060): the values an
// Admin stores in the settings document, the deploy's overrides and the
// hosting plans' figures they fall back to, how one value is resolved from the
// three, and the counts and guards the modules that enforce a limit share.
package limits

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"quizzivy/internal/shared/access"
)

// Querier is the surface a pool and a transaction share.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Name names one limit, as the contract and the audit row spell it.
type Name string

// The limits, in the order the settings page lists them.
const (
	Seats                 Name = "seats"
	Teachers              Name = "teachers"
	StorageBytes          Name = "storageBytes"
	MediaPerTeacherBytes  Name = "mediaPerTeacherBytes"
	ImportBytes           Name = "importBytes"
	ImportPerTeacherBytes Name = "importPerTeacherBytes"
	EmailsPerDay          Name = "emailsPerDay"
	EmailsPerMonth        Name = "emailsPerMonth"
	DatabaseBytes         Name = "databaseBytes"
	WarnAtPercent         Name = "warnAtPercent"
)

// Source says where a default comes from.
type Source string

// The sources of a default.
const (
	// SourceNone is no limit.
	SourceNone Source = "none"
	// SourceApp is the app's own figure, which no hosting plan sets.
	SourceApp Source = "app"
	// SourceDeploy is a deploy variable that overrides the hosting plan.
	SourceDeploy Source = "deploy"
	// SourceR2Free is Cloudflare R2's free tier.
	SourceR2Free Source = "r2_free"
	// SourceNeonLaunch is the warning line chosen for Neon's Launch plan.
	SourceNeonLaunch Source = "neon_launch"
	// SourceResendFree is Resend's free plan.
	SourceResendFree Source = "resend_free"
)

// Byte units, as R2 and the web's formatStorage count them.
const (
	MiB = int64(1) << 20
	GiB = int64(1) << 30
)

// R2FreeStorageBytes is the storage Cloudflare R2's free tier includes, 10 GB
// a month, a GB being 2^30 bytes; read on 2026-10-21. Media, import documents
// and exports share the account's buckets, so one limit covers them all.
const R2FreeStorageBytes = 10 * GiB

// NeonLaunchDatabaseBytes is the database size the console warns at on Neon's
// Launch plan, which bills storage by use and sets no cap; the owner confirmed
// Launch on 2026-10-21 and the line is 10 GB.
const NeonLaunchDatabaseBytes = 10 * GiB

// ResendFreeEmailsPerDay and ResendFreeEmailsPerMonth are what Resend's free
// plan sends, 100 emails a day and 3,000 a month; read on 2026-10-21.
const (
	ResendFreeEmailsPerDay   = 100
	ResendFreeEmailsPerMonth = 3000
)

// The app's own defaults: the deck's "of 5 GB" library (T-R4.17a), the import
// quotas of O-24, and the conventional warning point of the Overview (D-060).
const (
	AppMediaPerTeacherBytes  = 5 * GiB
	AppImportBytes           = 1 * GiB
	AppImportPerTeacherBytes = 256 * MiB
	AppWarnAtPercent         = 80
)

type bounds struct {
	column   string
	min, max int64
	fallback int64
	source   Source
}

var catalogue = []struct {
	name Name
	bounds
}{
	{Seats, bounds{"limit_seats", 1, 1_000_000, 0, SourceNone}},
	{Teachers, bounds{"limit_teachers", 1, 1_000_000, 0, SourceNone}},
	{StorageBytes, bounds{"limit_storage_bytes", MiB, 1 << 50, R2FreeStorageBytes, SourceR2Free}},
	{MediaPerTeacherBytes, bounds{"limit_media_per_teacher_bytes", MiB, 1 << 50, AppMediaPerTeacherBytes, SourceApp}},
	{ImportBytes, bounds{"limit_import_bytes", MiB, 1 << 50, AppImportBytes, SourceApp}},
	{ImportPerTeacherBytes, bounds{"limit_import_per_teacher_bytes", MiB, 1 << 50, AppImportPerTeacherBytes, SourceApp}},
	{EmailsPerDay, bounds{"limit_emails_per_day", 1, 100_000, ResendFreeEmailsPerDay, SourceResendFree}},
	{EmailsPerMonth, bounds{"limit_emails_per_month", 1, 10_000_000, ResendFreeEmailsPerMonth, SourceResendFree}},
	{DatabaseBytes, bounds{"limit_database_bytes", MiB, 1 << 50, NeonLaunchDatabaseBytes, SourceNeonLaunch}},
	{WarnAtPercent, bounds{"warn_at_percent", 50, 100, AppWarnAtPercent, SourceApp}},
}

func lookup(n Name) (bounds, bool) {
	for _, c := range catalogue {
		if c.name == n {
			return c.bounds, true
		}
	}
	return bounds{}, false
}

// Names returns every limit in the settings page's order.
func Names() []Name {
	out := make([]Name, len(catalogue))
	for i, c := range catalogue {
		out[i] = c.name
	}
	return out
}

// Column returns the organization_settings column that stores n.
func Column(n Name) string {
	b, _ := lookup(n)
	return b.column
}

// Range returns the smallest and largest value n may be set to.
func Range(n Name) (lowest, highest int64) {
	b, _ := lookup(n)
	return b.min, b.max
}

// Stored is what an Admin set, by name. A name that is absent takes its
// default, which for Seats and Teachers is no limit.
type Stored map[Name]int64

// Equal reports whether s and other set the same names to the same values.
func (s Stored) Equal(other Stored) bool {
	if len(s) != len(other) {
		return false
	}
	for n, v := range s {
		if w, ok := other[n]; !ok || w != v {
			return false
		}
	}
	return true
}

// Change is one limit a save changed; a nil side is the default.
type Change struct {
	Name   Name   `json:"name"`
	Before *int64 `json:"before"`
	After  *int64 `json:"after"`
}

// Changes lists, in Names order, the limits next sets differently from s.
func (s Stored) Changes(next Stored) []Change {
	var out []Change
	for _, n := range Names() {
		before, had := s[n]
		after, has := next[n]
		if had == has && before == after {
			continue
		}
		c := Change{Name: n}
		if had {
			c.Before = &before
		}
		if has {
			c.After = &after
		}
		out = append(out, c)
	}
	return out
}

// Refused names a limit set outside its range.
func (s Stored) Refused() []Name {
	var out []Name
	for _, n := range Names() {
		v, ok := s[n]
		if !ok {
			continue
		}
		if lo, hi := Range(n); v < lo || v > hi {
			out = append(out, n)
		}
	}
	for n := range s {
		if _, known := lookup(n); !known {
			out = append(out, n)
		}
	}
	return out
}

// Env is what the deploy's variables set, by name: a name present overrides
// its hosting-plan default and is overridden by a stored value.
type Env map[Name]int64

// Default is the value a limit takes when nothing is stored: nil for no limit.
type Default struct {
	Value  *int64
	Source Source
}

// Limit is one resolved limit. Value is nil for no limit; Custom says an
// Admin stored it.
type Limit struct {
	Value   *int64
	Default Default
	Custom  bool
}

// Limits is every limit resolved.
type Limits map[Name]Limit

// DefaultOf returns n's default under env.
func DefaultOf(n Name, env Env) Default {
	if v, ok := env[n]; ok {
		return Default{Value: &v, Source: SourceDeploy}
	}
	b, _ := lookup(n)
	if b.source == SourceNone {
		return Default{Source: SourceNone}
	}
	v := b.fallback
	return Default{Value: &v, Source: b.source}
}

// Resolve applies the precedence stored, then env, then the hosting plan.
func Resolve(stored Stored, env Env) Limits {
	out := make(Limits, len(catalogue))
	for _, n := range Names() {
		l := Limit{Default: DefaultOf(n, env)}
		if v, ok := stored[n]; ok {
			l.Value, l.Custom = &v, true
		} else {
			l.Value = l.Default.Value
		}
		out[n] = l
	}
	return out
}

// Of returns n's value and whether there is a limit at all.
func (l Limits) Of(n Name) (int64, bool) {
	v := l[n].Value
	if v == nil {
		return 0, false
	}
	return *v, true
}

// WarnPercent is the share of a limit at which the console warns.
func (l Limits) WarnPercent() int {
	if v, ok := l.Of(WarnAtPercent); ok {
		return int(v)
	}
	return AppWarnAtPercent
}

// SelectList is the columns that store the limits, in Names order.
func SelectList() string {
	cols := make([]string, len(catalogue))
	for i, c := range catalogue {
		cols[i] = c.column
	}
	return strings.Join(cols, ", ")
}

// ScanTargets returns one destination per column of SelectList, and the
// function that gathers them into a Stored once the row is scanned.
func ScanTargets() ([]any, func() Stored) {
	values := make([]*int64, len(catalogue))
	targets := make([]any, len(catalogue))
	for i := range values {
		targets[i] = &values[i]
	}
	return targets, func() Stored {
		out := Stored{}
		for i, c := range catalogue {
			if values[i] != nil {
				out[c.name] = *values[i]
			}
		}
		return out
	}
}

// Arguments returns s as one argument per column of SelectList, nil for a
// default.
func (s Stored) Arguments() []any {
	out := make([]any, len(catalogue))
	for i, c := range catalogue {
		if v, ok := s[c.name]; ok {
			out[i] = v
		}
	}
	return out
}

// ReadStored reads the stored limits.
func ReadStored(ctx context.Context, q Querier) (Stored, error) {
	targets, gather := ScanTargets()
	if err := q.QueryRow(ctx, `SELECT `+SelectList()+` FROM app.organization_settings`).Scan(targets...); err != nil {
		return nil, fmt.Errorf("limits: read the stored limits: %w", err)
	}
	return gather(), nil
}

// Read reads the stored limits and resolves them under env.
func Read(ctx context.Context, q Querier, env Env) (Limits, error) {
	stored, err := ReadStored(ctx, q)
	if err != nil {
		return nil, err
	}
	return Resolve(stored, env), nil
}

// Reader reads the resolved limits from one database under one deploy's
// variables.
type Reader struct {
	DB  Querier
	Env Env
}

// Read resolves the limits, on q when it is not nil and on the reader's
// database otherwise.
func (r Reader) Read(ctx context.Context, q Querier) (Limits, error) {
	if q == nil {
		q = r.DB
	}
	return Read(ctx, q, r.Env)
}

// ErrSeatLimit and ErrTeacherLimit refuse an account that would take the
// centre past its seat or teacher limit. ErrStorageLimit refuses a file that
// would take the centre's stored files past StorageBytes.
var (
	ErrSeatLimit    = errors.New("limits: the centre has no seat left")
	ErrTeacherLimit = errors.New("limits: the centre has no teacher place left")
	ErrStorageLimit = errors.New("limits: the centre's file storage is full")
)

// TeacherRole is the condition that a role id, the SQL expression role, opens
// the teacher workspace, as access.Workspaces decides it from the role's keys.
func TeacherRole(role string) string {
	prefixes := access.TeacherPrefixes()
	likes := make([]string, len(prefixes))
	for i, p := range prefixes {
		likes[i] = fmt.Sprintf("rp.permission_key LIKE '%s%%'", p)
	}
	return `EXISTS (SELECT 1 FROM app.role_permissions rp WHERE rp.role_id = ` + role + ` AND (` + strings.Join(likes, " OR ") + `))`
}

// TeacherRoles returns the ids of the roles that take a teacher place.
func TeacherRoles(ctx context.Context, q Querier) (map[string]struct{}, error) {
	var ids []string
	if err := q.QueryRow(ctx, `SELECT coalesce(array_agg(r.id::text), '{}') FROM app.roles r WHERE `+TeacherRole("r.id")).Scan(&ids); err != nil {
		return nil, fmt.Errorf("limits: teacher roles: %w", err)
	}
	out := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out, nil
}

// Accounts is how many seats and teacher places the centre uses: a seat is an
// enabled account (an anonymised one is disabled), and a teacher an enabled
// account whose role opens the teacher workspace (D-052).
type Accounts struct {
	Seats    int
	Teachers int
}

// CountAccounts counts the seats and teachers in use.
func CountAccounts(ctx context.Context, q Querier) (Accounts, error) {
	var out Accounts
	err := q.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE `+TeacherRole("u.role_id")+`)
		  FROM app.users u
		 WHERE u.disabled_at IS NULL`).Scan(&out.Seats, &out.Teachers)
	if err != nil {
		return Accounts{}, fmt.Errorf("limits: count accounts: %w", err)
	}
	return out, nil
}

// Admission is one write that may take a seat or a teacher place: Seat when it
// makes an account enabled that was not (a new account, or one enabled again),
// RoleID the role the account has afterwards, and FromRoleID the role it had
// while it was already enabled, empty when Seat is true.
type Admission struct {
	Seat       bool
	RoleID     string
	FromRoleID string
}

// Admit refuses a with ErrSeatLimit or ErrTeacherLimit when it would take the
// centre past a limit an Admin set, counted with every other admission held
// back by a transaction-scoped advisory lock, so q must be the transaction that
// then writes. With no limit set that a touches, it takes no lock.
func Admit(ctx context.Context, q Querier, a Admission) error {
	stored, err := ReadStored(ctx, q)
	if err != nil {
		return err
	}
	seats, seatLimited := stored[Seats]
	teachers, teacherLimited := stored[Teachers]
	seatGain := a.Seat && seatLimited
	teacherGain := false
	if teacherLimited && a.RoleID != "" {
		var becomes bool
		err := q.QueryRow(ctx, `SELECT `+TeacherRole("$1::uuid")+` AND NOT (`+TeacherRole("$2::uuid")+`)`,
			a.RoleID, nullable(a.FromRoleID)).Scan(&becomes)
		if err != nil {
			return fmt.Errorf("limits: class the role: %w", err)
		}
		teacherGain = becomes
	}
	if !seatGain && !teacherGain {
		return nil
	}
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(73819, 45)`); err != nil {
		return fmt.Errorf("limits: account lock: %w", err)
	}
	used, err := CountAccounts(ctx, q)
	if err != nil {
		return err
	}
	if seatGain && int64(used.Seats) >= seats {
		return ErrSeatLimit
	}
	if teacherGain && int64(used.Teachers) >= teachers {
		return ErrTeacherLimit
	}
	return nil
}

// Held is what the centre keeps in its buckets, as the database records it:
// media files not yet purged, the source documents and read artifacts of the
// imports whose files are kept, and the exports ready to download.
type Held struct {
	Media   int64
	Imports int64
	Exports int64
}

// Total is every byte Held counts.
func (h Held) Total() int64 { return h.Media + h.Imports + h.Exports }

// CountHeld sums what the centre keeps in its buckets.
func CountHeld(ctx context.Context, q Querier) (Held, error) {
	var out Held
	err := q.QueryRow(ctx, `
		SELECT (SELECT coalesce(sum(bytes), 0)::bigint FROM app.media_assets WHERE purged_at IS NULL),
		       (SELECT coalesce(sum(s.bytes), 0)::bigint
		          FROM app.word_import_sources s JOIN app.word_imports i ON i.id = s.import_id
		         WHERE i.files_removed_at IS NULL)
		     + (SELECT coalesce(sum(a.bytes), 0)::bigint
		          FROM app.word_import_artifact_sets a JOIN app.word_imports i ON i.id = a.import_id
		         WHERE i.files_removed_at IS NULL),
		       (SELECT coalesce(sum(bytes), 0)::bigint FROM app.data_exports WHERE status = 'ready')`).
		Scan(&out.Media, &out.Imports, &out.Exports)
	if err != nil {
		return Held{}, fmt.Errorf("limits: count stored files: %w", err)
	}
	return out, nil
}

// RequireStorage refuses with ErrStorageLimit adding bytes that would take
// what the centre keeps past limit, the resolved StorageBytes. It holds a
// transaction-scoped advisory lock, so q must be the transaction that then
// records the file.
func RequireStorage(ctx context.Context, q Querier, limit, bytes int64) error {
	if _, err := q.Exec(ctx, `SELECT pg_advisory_xact_lock(73819, 46)`); err != nil {
		return fmt.Errorf("limits: storage lock: %w", err)
	}
	held, err := CountHeld(ctx, q)
	if err != nil {
		return err
	}
	if held.Total()+bytes > limit {
		return ErrStorageLimit
	}
	return nil
}

// DatabaseSize is pg_database_size of the current database, which
// approximates and is not what Neon bills.
func DatabaseSize(ctx context.Context, q Querier) (int64, error) {
	var out int64
	if err := q.QueryRow(ctx, `SELECT pg_database_size(current_database())`).Scan(&out); err != nil {
		return 0, fmt.Errorf("limits: database size: %w", err)
	}
	return out, nil
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
