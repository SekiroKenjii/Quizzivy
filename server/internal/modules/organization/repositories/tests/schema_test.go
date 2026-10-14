//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"quizzivy/internal/modules/organization/repositories"
	"quizzivy/internal/platform/db"
)

func pgError(t *testing.T, err error) *pgconn.PgError {
	t.Helper()
	var pg *pgconn.PgError
	if !errors.As(err, &pg) {
		t.Fatalf("error = %v, want a PostgreSQL error", err)
	}
	return pg
}

func TestTheMigrationLeavesExactlyOneSettingsRow(t *testing.T) {
	pool := connect(t, migrateEnv)
	var rows int
	var onlyTrue bool
	if err := pool.QueryRow(context.Background(), `SELECT count(*), coalesce(bool_and(id), false) FROM app.organization_settings`).Scan(&rows, &onlyTrue); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || !onlyTrue {
		t.Fatalf("settings rows = %d (all keyed true: %v), want exactly one keyed true", rows, onlyTrue)
	}
}

func TestTheMigrationSeedsTheCentreWithItsDocumentedDefaults(t *testing.T) {
	pool := connect(t, migrateEnv)
	matches, err := filepath.Glob(filepath.Join(db.MigrationsDir(t), "*_create_organization_settings.sql"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("migration files = %v (err %v), want exactly one", matches, err)
	}
	source, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	insert := regexp.MustCompile(`(?m)^INSERT INTO app\.organization_settings[^;]*;`).FindString(string(source))
	if insert == "" {
		t.Fatalf("%s holds no INSERT INTO app.organization_settings", matches[0])
	}
	replayed := strings.Replace(insert, "app.organization_settings", "seed_copy", 1)

	rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
		if _, err := tx.Exec(ctx, `CREATE TEMP TABLE seed_copy (LIKE app.organization_settings INCLUDING ALL) ON COMMIT DROP`); err != nil {
			t.Fatalf("temp copy: %v", err)
		}
		if _, err := tx.Exec(ctx, replayed); err != nil {
			t.Fatalf("replaying the migration's INSERT: %v", err)
		}
		seeded := readRawTable(t, tx, "seed_copy")
		want := rawRow{
			name: "Quizzivy", google: true, password: true, force: true, joinCodes: true,
			domains: []string{}, keepFocus: 13, revision: 1, updatedAt: seeded.updatedAt,
		}
		if !seeded.same(want) {
			t.Errorf("seeded row = %+v, want %+v", seeded, want)
		}
		var stamped bool
		if err := tx.QueryRow(ctx, `SELECT updated_at = now() FROM seed_copy`).Scan(&stamped); err != nil || !stamped {
			t.Errorf("seeded updated_at is the insert time: %v (err %v)", stamped, err)
		}
	})
}

func TestAnUpdateStampsUpdatedAtWhateverTheStatementWrites(t *testing.T) {
	pool := connect(t, migrateEnv)
	rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
		var stamped bool
		if err := tx.QueryRow(ctx, `
			WITH changed AS (
			  UPDATE app.organization_settings SET updated_at = '2000-01-01T00:00:00Z' RETURNING updated_at
			)
			SELECT updated_at = now() FROM changed`).Scan(&stamped); err != nil {
			t.Fatal(err)
		}
		if !stamped {
			t.Error("updated_at kept the value the UPDATE wrote, not the time of the change")
		}
	})
}

func TestASecondSettingsRowCannotBeInserted(t *testing.T) {
	pool := connect(t, migrateEnv)
	cases := []struct {
		name       string
		sql        string
		code       string
		constraint string
	}{
		{"another row keyed true", `INSERT INTO app.organization_settings (name) VALUES ('Một trung tâm khác')`, pgerrcode.UniqueViolation, "organization_settings_pkey"},
		{"a row keyed false", `INSERT INTO app.organization_settings (id, name) VALUES (false, 'Một trung tâm khác')`, pgerrcode.CheckViolation, "organization_settings_singleton"},
		{"the row re-keyed false", `UPDATE app.organization_settings SET id = false`, pgerrcode.CheckViolation, "organization_settings_singleton"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
				_, err := tx.Exec(ctx, c.sql)
				pg := pgError(t, err)
				if pg.Code != c.code || pg.ConstraintName != c.constraint {
					t.Errorf("error = %s on %q, want %s on %q", pg.Code, pg.ConstraintName, c.code, c.constraint)
				}
			})
		})
	}
}

func TestTheAppRoleMayReadAndUpdateTheSettingsButNeverInsertOrDeleteThem(t *testing.T) {
	pool := connect(t, appEnv)

	t.Run("select", func(t *testing.T) {
		rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
			var name string
			if err := tx.QueryRow(ctx, `SELECT name FROM app.organization_settings`).Scan(&name); err != nil {
				t.Fatalf("select as the app role: %v", err)
			}
		})
	})
	t.Run("update", func(t *testing.T) {
		rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
			tag, err := tx.Exec(ctx, `UPDATE app.organization_settings SET revision = revision`)
			if err != nil || tag.RowsAffected() != 1 {
				t.Fatalf("update as the app role: %d rows, err %v", tag.RowsAffected(), err)
			}
		})
	})
	t.Run("insert", func(t *testing.T) {
		rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
			_, err := tx.Exec(ctx, `INSERT INTO app.organization_settings (name) VALUES ('Một trung tâm khác')`)
			if pg := pgError(t, err); pg.Code != pgerrcode.InsufficientPrivilege {
				t.Errorf("insert error = %s, want %s", pg.Code, pgerrcode.InsufficientPrivilege)
			}
		})
	})
	t.Run("delete", func(t *testing.T) {
		rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
			_, err := tx.Exec(ctx, `DELETE FROM app.organization_settings`)
			if pg := pgError(t, err); pg.Code != pgerrcode.InsufficientPrivilege {
				t.Errorf("delete error = %s, want %s", pg.Code, pgerrcode.InsufficientPrivilege)
			}
		})
	})
	t.Run("truncate", func(t *testing.T) {
		rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
			_, err := tx.Exec(ctx, `TRUNCATE app.organization_settings`)
			if pg := pgError(t, err); pg.Code != pgerrcode.InsufficientPrivilege {
				t.Errorf("truncate error = %s, want %s", pg.Code, pgerrcode.InsufficientPrivilege)
			}
		})
	})
	t.Run("the repository reads through the app role", func(t *testing.T) {
		got, err := repositories.NewPostgres(db.NewContext(pool)).Read(context.Background())
		if err != nil {
			t.Fatalf("read as the app role: %v", err)
		}
		if got.Revision < 1 || got.Profile.Name == "" {
			t.Errorf("document read as the app role = %+v", got)
		}
	})
}

type refusal struct {
	name       string
	set        string
	code       string
	constraint string
	column     string
}

func checkRefusal(name, set, constraint string) refusal {
	return refusal{name: name, set: set, code: pgerrcode.CheckViolation, constraint: "organization_settings_" + constraint}
}

func nullRefusal(column string) refusal {
	return refusal{name: column + " null", set: column + " = NULL", code: pgerrcode.NotNullViolation, column: column}
}

func TestTheChecksRefuseWhatTheDocumentMayNotHold(t *testing.T) {
	pool := connect(t, migrateEnv)
	hash31 := `decode(repeat('ab', 31), 'hex')`
	hash32 := `decode(repeat('ab', 32), 'hex')`
	hash33 := `decode(repeat('ab', 33), 'hex')`
	cases := []refusal{
		checkRefusal("name blank", `name = '   '`, "name_check"),
		checkRefusal("name of 121 characters", `name = repeat('a', 121)`, "name_check"),
		checkRefusal("name of 121 multi-byte characters", `name = repeat('ế', 121)`, "name_check"),
		checkRefusal("short name blank", `short_name = ' '`, "short_name_check"),
		checkRefusal("short name of 41 characters", `short_name = repeat('s', 41)`, "short_name_check"),
		checkRefusal("website empty", `website = ''`, "website_check"),
		checkRefusal("website of 201 characters", `website = repeat('w', 201)`, "website_check"),
		checkRefusal("contact email of 2 characters", `contact_email = 'ab'`, "contact_email_check"),
		checkRefusal("contact email of 255 characters", `contact_email = repeat('e', 255)`, "contact_email_check"),
		checkRefusal("phone with letters", `front_desk_phone = 'call me'`, "front_desk_phone_check"),
		checkRefusal("phone of 5 characters", `front_desk_phone = '12345'`, "front_desk_phone_check"),
		checkRefusal("phone of 31 characters", `front_desk_phone = repeat('1', 31)`, "front_desk_phone_check"),
		checkRefusal("hours empty", `front_desk_hours = ''`, "front_desk_hours_check"),
		checkRefusal("hours of 121 characters", `front_desk_hours = repeat('h', 121)`, "front_desk_hours_check"),
		checkRefusal("address empty", `address = ''`, "address_check"),
		checkRefusal("address of 301 characters", `address = repeat('a', 301)`, "address_check"),
		checkRefusal("logo hash of 31 bytes", `logo_sha256 = `+hash31+`, logo_type = 'image/png'`, "logo_sha256_check"),
		checkRefusal("logo hash of 33 bytes", `logo_sha256 = `+hash33+`, logo_type = 'image/png'`, "logo_sha256_check"),
		checkRefusal("dark logo hash of 31 bytes", `logo_dark_sha256 = `+hash31+`, logo_dark_type = 'image/png'`, "logo_dark_sha256_check"),
		checkRefusal("logo type gif", `logo_sha256 = `+hash32+`, logo_type = 'image/gif'`, "logo_type_check"),
		checkRefusal("dark logo type jpeg", `logo_dark_sha256 = `+hash32+`, logo_dark_type = 'image/jpeg'`, "logo_dark_type_check"),
		checkRefusal("logo hash without a type", `logo_sha256 = `+hash32+`, logo_type = NULL`, "logo_pair"),
		checkRefusal("logo type without a hash", `logo_sha256 = NULL, logo_type = 'image/png'`, "logo_pair"),
		checkRefusal("dark logo hash without a type", `logo_dark_sha256 = `+hash32+`, logo_dark_type = NULL`, "logo_dark_pair"),
		checkRefusal("dark logo type without a hash", `logo_dark_sha256 = NULL, logo_dark_type = 'image/svg+xml'`, "logo_dark_pair"),
		checkRefusal("both sign-in methods off", `google_sign_in = false, password_sign_in = false`, "a_sign_in_method"),
		checkRefusal("an upper-case domain", `allowed_google_domains = ARRAY['Example.com']`, "allowed_google_domains_check"),
		checkRefusal("51 domains", `allowed_google_domains = ARRAY(SELECT 'd' || g || '.example' FROM generate_series(1, 51) AS g)`, "allowed_google_domains_check"),
		checkRefusal("a null among the domains", `allowed_google_domains = ARRAY['a.example', NULL]::text[]`, "allowed_google_domains_check"),
		checkRefusal("keep answers 6 months", `keep_answers_months = 6`, "keep_answers_months_check"),
		checkRefusal("keep answers 0 months", `keep_answers_months = 0`, "keep_answers_months_check"),
		checkRefusal("keep focus events 7 months", `keep_focus_events_months = 7`, "keep_focus_events_months_check"),
		checkRefusal("keep focus events 0 months", `keep_focus_events_months = 0`, "keep_focus_events_months_check"),
		checkRefusal("anonymise after 6 months", `anonymise_disabled_after_months = 6`, "anonymise_disabled_after_months_check"),
		checkRefusal("revision 0", `revision = 0`, "revision_check"),
		nullRefusal("name"),
		nullRefusal("google_sign_in"),
		nullRefusal("password_sign_in"),
		nullRefusal("force_password_change"),
		nullRefusal("join_codes_enabled"),
		nullRefusal("allowed_google_domains"),
		nullRefusal("keep_focus_events_months"),
		nullRefusal("revision"),
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
				_, err := tx.Exec(ctx, `UPDATE app.organization_settings SET `+c.set)
				pg := pgError(t, err)
				if pg.Code != c.code || pg.ConstraintName != c.constraint || pg.ColumnName != c.column {
					t.Errorf("error = %s constraint %q column %q, want %s constraint %q column %q",
						pg.Code, pg.ConstraintName, pg.ColumnName, c.code, c.constraint, c.column)
				}
			})
		})
	}
}

func TestTheChecksAcceptTheLimitsOfWhatTheDocumentMayHold(t *testing.T) {
	pool := connect(t, migrateEnv)
	hash32 := `decode(repeat('cd', 32), 'hex')`
	cases := []struct{ name, set string }{
		{"name of 120 characters", `name = repeat('a', 120)`},
		{"name of 120 multi-byte characters", `name = repeat('ế', 120)`},
		{"short name of 40 characters", `short_name = repeat('s', 40)`},
		{"website of 200 characters", `website = repeat('w', 200)`},
		{"contact email of 3 characters", `contact_email = 'a@b'`},
		{"contact email of 254 characters", `contact_email = repeat('e', 244) || '@x.example'`},
		{"phone of 6 characters", `front_desk_phone = '123456'`},
		{"phone of 30 characters", `front_desk_phone = repeat('1', 30)`},
		{"phone with the allowed punctuation", `front_desk_phone = '+84 (28) 3822.0000-1'`},
		{"hours of 120 characters", `front_desk_hours = repeat('h', 120)`},
		{"address of 300 characters", `address = repeat('a', 300)`},
		{"png logo", `logo_sha256 = ` + hash32 + `, logo_type = 'image/png'`},
		{"svg logo", `logo_sha256 = ` + hash32 + `, logo_type = 'image/svg+xml'`},
		{"dark png logo", `logo_dark_sha256 = ` + hash32 + `, logo_dark_type = 'image/png'`},
		{"dark svg logo", `logo_dark_sha256 = ` + hash32 + `, logo_dark_type = 'image/svg+xml'`},
		{"no logos", `logo_sha256 = NULL, logo_type = NULL, logo_dark_sha256 = NULL, logo_dark_type = NULL`},
		{"google only", `google_sign_in = true, password_sign_in = false`},
		{"password only", `google_sign_in = false, password_sign_in = true`},
		{"50 domains", `allowed_google_domains = ARRAY(SELECT 'd' || g || '.example' FROM generate_series(1, 50) AS g)`},
		{"no domains", `allowed_google_domains = '{}'`},
		{"keep answers 12 months", `keep_answers_months = 12`},
		{"keep answers 24 months", `keep_answers_months = 24`},
		{"keep answers for ever", `keep_answers_months = NULL`},
		{"keep focus events 6 months", `keep_focus_events_months = 6`},
		{"keep focus events 12 months", `keep_focus_events_months = 12`},
		{"keep focus events 13 months", `keep_focus_events_months = 13`},
		{"keep focus events 24 months", `keep_focus_events_months = 24`},
		{"anonymise after 12 months", `anonymise_disabled_after_months = 12`},
		{"anonymise after 24 months", `anonymise_disabled_after_months = 24`},
		{"never anonymise", `anonymise_disabled_after_months = NULL`},
		{"every optional text absent", `short_name = NULL, website = NULL, contact_email = NULL, front_desk_phone = NULL, front_desk_hours = NULL, address = NULL`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
				if _, err := tx.Exec(ctx, `UPDATE app.organization_settings SET `+c.set); err != nil {
					t.Errorf("UPDATE SET %s: %v", c.set, err)
				}
			})
		})
	}
}

func TestADeletedAdminOrUserLeavesTheSettingsAuthorEmpty(t *testing.T) {
	s := lockedSession(t)
	author := s.addAccount(account{role: "teacher", password: true})
	s.exec(`UPDATE app.organization_settings SET updated_by = $1::uuid`, author)
	if got := s.stored().UpdatedBy; got != author {
		t.Fatalf("updated_by = %q, want %q", got, author)
	}
	s.exec(`DELETE FROM app.users WHERE id = $1::uuid`, author)
	if got := s.stored().UpdatedBy; got != "" {
		t.Errorf("updated_by after the author was deleted = %q, want empty", got)
	}
	if !reflect.DeepEqual(s.raw().updatedBy, (*string)(nil)) {
		t.Errorf("updated_by column is not NULL after the author was deleted")
	}
}
