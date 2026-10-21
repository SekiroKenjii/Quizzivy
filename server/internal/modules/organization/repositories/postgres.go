// Package repositories reads and writes the centre's one row of app.organization_settings, judges a sign-in policy against the active accounts, and counts what a retention policy would remove.
package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"quizzivy/internal/shared/retention"
)

// Postgres stores the settings in the database.
type Postgres struct{ db.Repository }

// NewPostgres builds the repository on the database context.
func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

var _ domain.Repository = (*Postgres)(nil)

const columns = `
	name, short_name, website, contact_email, front_desk_phone, front_desk_hours, address,
	logo_sha256, logo_type, logo_dark_sha256, logo_dark_type,
	google_sign_in, password_sign_in, force_password_change, join_codes_enabled, allowed_google_domains,
	keep_answers_months, keep_focus_events_months, anonymise_disabled_after_months,
	revision, updated_by, updated_at`

// Read returns the stored document.
func (p *Postgres) Read(ctx context.Context) (domain.Settings, error) {
	return scan(p.QueryRow(ctx, `SELECT `+columns+` FROM app.organization_settings`))
}

// Save stores in.Settings over in.ExpectedRevision, judging its sign-in
// policy inside the transaction that holds the row. A document equal to the
// stored one changes nothing, not even the revision, and writes no audit row.
func (p *Postgres) Save(ctx context.Context, in domain.Save) (domain.Settings, error) {
	var out domain.Settings
	err := p.InTx(ctx, "organization: save settings", func(tx pgx.Tx) error {
		stored, err := scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM app.organization_settings FOR UPDATE`))
		if err != nil {
			return err
		}
		if stored.Revision != in.ExpectedRevision {
			return domain.ErrStale
		}
		if len(stored.ChangedFields(in.Settings)) == 0 {
			out = stored
			return nil
		}
		impact, err := impactOf(ctx, tx, in.Settings.SignIn, in.ActorID, in.GoogleConfigured)
		if err != nil {
			return err
		}
		if impact.Lockout() {
			return &domain.LockedOut{Impact: impact}
		}
		out, err = write(ctx, tx, in)
		if err != nil {
			return err
		}
		return record(ctx, tx, in, stored.ChangedFields(out), out.Revision)
	})
	if err != nil {
		return domain.Settings{}, err
	}
	return out, nil
}

func write(ctx context.Context, tx pgx.Tx, in domain.Save) (domain.Settings, error) {
	s := in.Settings
	domains := s.SignIn.AllowedGoogleDomains
	if domains == nil {
		domains = []string{}
	}
	light, lightType := logoColumns(s.Profile.LightLogo)
	dark, darkType := logoColumns(s.Profile.DarkLogo)
	return scan(tx.QueryRow(ctx, `
		UPDATE app.organization_settings
		   SET name = $1, short_name = $2, website = $3, contact_email = $4, front_desk_phone = $5,
		       front_desk_hours = $6, address = $7,
		       logo_sha256 = $8, logo_type = $9, logo_dark_sha256 = $10, logo_dark_type = $11,
		       google_sign_in = $12, password_sign_in = $13, force_password_change = $14,
		       join_codes_enabled = $15, allowed_google_domains = $16,
		       keep_answers_months = $17, keep_focus_events_months = $18, anonymise_disabled_after_months = $19,
		       revision = revision + 1, updated_by = $20
		RETURNING `+columns,
		s.Profile.Name, opt.String(s.Profile.ShortName), opt.String(s.Profile.Website), opt.String(s.Profile.ContactEmail),
		opt.String(s.Profile.FrontDeskPhone), opt.String(s.Profile.FrontDeskHours), opt.String(s.Profile.Address),
		light, lightType, dark, darkType,
		s.SignIn.Google, s.SignIn.Password, s.SignIn.ForcePasswordChange, s.SignIn.JoinCodes, domains,
		s.Retention.KeepAnswersMonths, s.Retention.KeepFocusEventsMonths, s.Retention.AnonymiseDisabledAfterMonths,
		opt.String(in.ActorID)))
}

func logoColumns(l *domain.Logo) ([]byte, *string) {
	if l == nil {
		return nil, nil
	}
	kind := string(l.Type)
	return l.SHA256, &kind
}

func record(ctx context.Context, tx pgx.Tx, in domain.Save, fields []string, revision int64) error {
	if fields == nil {
		fields = []string{}
	}
	diff, err := json.Marshal(struct {
		Fields   []string `json:"fields"`
		Revision int64    `json:"revision"`
	}{fields, revision})
	if err != nil {
		return fmt.Errorf("organization: encode audit diff: %w", err)
	}
	return audit.Write(ctx, tx, audit.Entry{
		ActorUserID: opt.String(in.ActorID), Action: "organization.updated", Entity: "organization",
		OccurredAt: in.Now, IP: opt.String(in.IP), UserAgent: opt.String(in.UserAgent), Diff: diff,
	})
}

// Impact judges policy against the active accounts for actorID.
func (p *Postgres) Impact(ctx context.Context, policy domain.SignIn, actorID string, googleConfigured bool) (domain.Impact, error) {
	return impactOf(ctx, p, policy, actorID, googleConfigured)
}

func impactOf(ctx context.Context, q db.Querier, policy domain.SignIn, actorID string, googleConfigured bool) (domain.Impact, error) {
	domains := policy.AllowedGoogleDomains
	if domains == nil {
		domains = []string{}
	}
	var out domain.Impact
	err := q.QueryRow(ctx, `
		WITH accounts AS (
		  SELECT u.id, r.builtin_key IS NOT DISTINCT FROM 'admin' AS admin,
		         ($1 AND u.password_hash IS NOT NULL)
		         OR ($2 AND EXISTS (
		              SELECT 1 FROM app.user_identities i
		               WHERE i.user_id = u.id AND i.provider = 'google'
		                 AND (cardinality($3::text[]) = 0
		                      OR lower(substring(i.email_at_link FROM '@([^@]*)$')) = ANY ($3::text[])))) AS has_way
		    FROM app.users u
		    JOIN app.roles r ON r.id = u.role_id
		   WHERE u.disabled_at IS NULL
		)
		SELECT count(*) FILTER (WHERE NOT has_way),
		       count(*) FILTER (WHERE admin AND NOT has_way),
		       count(*) FILTER (WHERE admin),
		       coalesce(bool_or(id = $4::uuid AND NOT has_way), false)
		  FROM accounts`,
		policy.Password, policy.Google && googleConfigured, domains, opt.String(actorID)).
		Scan(&out.AffectedUsers, &out.AffectedAdmins, &out.ActiveAdmins, &out.ActorLocked)
	if err != nil {
		return domain.Impact{}, fmt.Errorf("organization: judge the sign-in policy: %w", err)
	}
	return out, nil
}

// RetentionImpact counts what the retention sweep would do today under
// policy, with the queries its dry run runs, in one read-only transaction.
func (p *Postgres) RetentionImpact(ctx context.Context, policy domain.Retention) (domain.RetentionImpact, error) {
	var out domain.RetentionImpact
	err := p.InTx(ctx, "organization: count the retention policy's impact", func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SET TRANSACTION READ ONLY`); err != nil {
			return err
		}
		if m := policy.KeepAnswersMonths; m != nil {
			out.Answers = &domain.AnswersImpact{}
			if err := countBefore(ctx, tx, *m, retention.CountAnswers, &out.Answers.Attempts, &out.Answers.Answers); err != nil {
				return err
			}
		}
		if err := countBefore(ctx, tx, policy.KeepFocusEventsMonths, retention.CountEvents, &out.FocusEvents); err != nil {
			return err
		}
		if m := policy.AnonymiseDisabledAfterMonths; m != nil {
			var inProgress int64
			out.Accounts = &domain.AccountsImpact{}
			return countBefore(ctx, tx, *m, retention.CountAccounts, &out.Accounts.Anonymise, &out.Accounts.SkippedOwners, &inProgress)
		}
		return nil
	})
	if err != nil {
		return domain.RetentionImpact{}, fmt.Errorf("organization: count the retention policy's impact: %w", err)
	}
	return out, nil
}

func countBefore(ctx context.Context, tx pgx.Tx, months int, count string, into ...any) error {
	var cutoff time.Time
	if err := tx.QueryRow(ctx, `SELECT `+retention.Cutoff("$1"), months).Scan(&cutoff); err != nil {
		return err
	}
	return tx.QueryRow(ctx, count, cutoff).Scan(into...)
}

func scan(row pgx.Row) (domain.Settings, error) {
	var (
		s                                                                         domain.Settings
		shortName, website, email, phone, hours, address, lightType, darkType, by *string
		light, dark                                                               []byte
	)
	err := row.Scan(&s.Profile.Name, &shortName, &website, &email, &phone, &hours, &address,
		&light, &lightType, &dark, &darkType,
		&s.SignIn.Google, &s.SignIn.Password, &s.SignIn.ForcePasswordChange, &s.SignIn.JoinCodes, &s.SignIn.AllowedGoogleDomains,
		&s.Retention.KeepAnswersMonths, &s.Retention.KeepFocusEventsMonths, &s.Retention.AnonymiseDisabledAfterMonths,
		&s.Revision, &by, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Settings{}, errors.New("organization: the settings row is missing")
	}
	if err != nil {
		return domain.Settings{}, fmt.Errorf("organization: read settings: %w", err)
	}
	s.Profile.ShortName, s.Profile.Website, s.Profile.ContactEmail = opt.Deref(shortName), opt.Deref(website), opt.Deref(email)
	s.Profile.FrontDeskPhone, s.Profile.FrontDeskHours, s.Profile.Address = opt.Deref(phone), opt.Deref(hours), opt.Deref(address)
	s.Profile.LightLogo = logoOf(light, lightType)
	s.Profile.DarkLogo = logoOf(dark, darkType)
	s.UpdatedBy = opt.Deref(by)
	if s.SignIn.AllowedGoogleDomains == nil {
		s.SignIn.AllowedGoogleDomains = []string{}
	}
	return s, nil
}

func logoOf(digest []byte, kind *string) *domain.Logo {
	if digest == nil || kind == nil || domain.LogoType(*kind) != domain.LogoPNG {
		return nil
	}
	return &domain.Logo{SHA256: digest, Type: domain.LogoType(*kind)}
}
