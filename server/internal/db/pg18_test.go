package db_test

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"quizzivy/internal/db"
)

// Pins the PostgreSQL 18 behaviour spec §13 depends on.
//
// §13.1 requires version-specific behaviour to be checked against the docs
// rather than recalled. This is the executable form of that check: if an
// assumption ever stops holding -- a minor upgrade, a different image, a
// misconfigured server -- it fails here in Phase 0 rather than in Phase 3 with
// a schema already built on it.
//
// Two of these exist because the first draft of the plan got them WRONG:
//   - pg_trgm was assumed to fold accents. It does not.
//   - Virtual generated columns were assumed to reject NOT NULL and CHECK.
//     They accept both.
//
// Each assertion names the docs section it encodes.

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("pgx", db.TestDSN(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := conn.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	return conn
}

// scratch runs stmts in a transaction that is always rolled back, so these
// tests leave no trace in a database that also holds migrations.
func scratch(t *testing.T, conn *sql.DB, fn func(tx *sql.Tx)) {
	t.Helper()
	tx, err := conn.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	fn(tx)
}

func TestServerIsPostgres18(t *testing.T) {
	conn := openDB(t)
	var version string
	if err := conn.QueryRow("SHOW server_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "18") {
		t.Fatalf("server_version = %s; spec §13 targets PostgreSQL 18 and this schema "+
			"does not degrade gracefully on 16/17 -- uuidv7() and virtual generated "+
			"columns simply do not exist", version)
	}
}

// https://www.postgresql.org/docs/18/functions-uuid.html
func TestUuidV7IsBuiltInAndTimeOrdered(t *testing.T) {
	conn := openDB(t)

	var version int
	if err := conn.QueryRow("SELECT uuid_extract_version(uuidv7())").Scan(&version); err != nil {
		t.Fatalf("uuidv7() is unavailable: %v", err)
	}
	if version != 7 {
		t.Errorf("uuid_extract_version = %d, want 7", version)
	}

	// §13.2's whole reason for choosing v7: time-ordered keys keep B-tree
	// inserts local instead of scattering them the way random v4 does.
	var ordered bool
	if err := conn.QueryRow(`
		WITH g AS (SELECT uuidv7() AS u FROM generate_series(1, 500))
		SELECT count(*) = 500 FROM (
			SELECT u, lag(u) OVER (ORDER BY uuid_extract_timestamp(u)) AS prev FROM g
		) s WHERE prev IS NULL OR u >= prev`).Scan(&ordered); err != nil {
		t.Fatal(err)
	}
	if !ordered {
		t.Error("uuidv7() values are not monotonically ordered; §13.8's keyset pagination assumes they are")
	}
}

// https://www.postgresql.org/docs/18/ddl-generated-columns.html
func TestVirtualGeneratedColumns(t *testing.T) {
	conn := openDB(t)

	t.Run("VIRTUAL is the default kind", func(t *testing.T) {
		scratch(t, conn, func(tx *sql.Tx) {
			mustExec(t, tx, `CREATE TEMP TABLE g_default (x int, y int GENERATED ALWAYS AS (x * 2))`)
			var kind string
			if err := tx.QueryRow(`
				SELECT attgenerated FROM pg_attribute
				WHERE attrelid = 'g_default'::regclass AND attname = 'y'`).Scan(&kind); err != nil {
				t.Fatal(err)
			}
			if kind != "v" {
				t.Errorf("attgenerated = %q, want \"v\" -- the docs say virtual is the default", kind)
			}
		})
	})

	t.Run("reflects base-column updates on read", func(t *testing.T) {
		scratch(t, conn, func(tx *sql.Tx) {
			mustExec(t, tx, `CREATE TEMP TABLE g (a numeric(8,2), b numeric(8,2),
				final numeric(8,2) GENERATED ALWAYS AS (coalesce(b, a)) VIRTUAL)`)
			mustExec(t, tx, `INSERT INTO g (a, b) VALUES (5.00, NULL)`)

			var got string
			if err := tx.QueryRow(`SELECT final::text FROM g`).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != "5.00" {
				t.Errorf("final = %s, want 5.00", got)
			}

			mustExec(t, tx, `UPDATE g SET b = 7.00`)
			if err := tx.QueryRow(`SELECT final::text FROM g`).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != "7.00" {
				t.Errorf("final = %s after update, want 7.00 -- §13.3 relies on manual "+
					"score taking precedence over auto without ever going stale", got)
			}
		})
	})

	// The exact rejection set, verified on 18.6. attempt_answers.final_score is
	// a virtual column, so this is what may and may not be done to it.
	t.Run("rejection set", func(t *testing.T) {
		cases := []struct {
			name    string
			stmt    string
			wantErr string // "" means it must succeed
		}{
			{"CREATE INDEX", `CREATE INDEX ON v (g)`, "virtual generated column"},
			{"UNIQUE", `ALTER TABLE v ADD CONSTRAINT v_uq UNIQUE (g)`, "virtual generated column"},
			{"PRIMARY KEY", `ALTER TABLE v ADD PRIMARY KEY (g)`, "virtual generated column"},
			{"CREATE STATISTICS", `CREATE STATISTICS v_st ON a, g FROM v`, "virtual generated column"},

			// These two the plan originally claimed were rejected. They are not.
			{"SET NOT NULL", `ALTER TABLE v ALTER COLUMN g SET NOT NULL`, ""},
			{"CHECK referencing it", `ALTER TABLE v ADD CONSTRAINT v_ck CHECK (g >= 0)`, ""},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				scratch(t, conn, func(tx *sql.Tx) {
					mustExec(t, tx, `CREATE TEMP TABLE v (a int, b int,
						g int GENERATED ALWAYS AS (coalesce(b, a)) VIRTUAL)`)
					_, err := tx.Exec(tc.stmt)
					switch {
					case tc.wantErr == "" && err != nil:
						t.Errorf("%s was rejected but should be allowed: %v", tc.name, err)
					case tc.wantErr != "" && err == nil:
						t.Errorf("%s succeeded but should be rejected", tc.name)
					case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
						t.Errorf("%s failed with an unexpected error: %v", tc.name, err)
					}
				})
			})
		}
	})

	t.Run("aggregates work, which is what grading actually needs", func(t *testing.T) {
		scratch(t, conn, func(tx *sql.Tx) {
			mustExec(t, tx, `CREATE TEMP TABLE v (a int, b int,
				g int GENERATED ALWAYS AS (coalesce(b, a)) VIRTUAL)`)
			mustExec(t, tx, `INSERT INTO v (a, b) VALUES (3, NULL), (10, 4)`)
			var sum int
			if err := tx.QueryRow(`SELECT sum(g) FROM v`).Scan(&sum); err != nil {
				t.Fatal(err)
			}
			if sum != 7 {
				t.Errorf("sum(g) = %d, want 7", sum)
			}
		})
	})
}

// https://www.postgresql.org/docs/18/dml-returning.html
func TestOldNewInReturningInsideDataModifyingCTE(t *testing.T) {
	conn := openDB(t)
	scratch(t, conn, func(tx *sql.Tx) {
		mustExec(t, tx, `CREATE TEMP TABLE att (id int PRIMARY KEY, deadline_at timestamptz)`)
		mustExec(t, tx, `CREATE TEMP TABLE audit (entity_id int, diff jsonb)`)
		mustExec(t, tx, `INSERT INTO att VALUES (1, '2026-01-01T10:00:00Z')`)

		// §13.4 wants the audit diff captured in the same statement as the
		// mutation. A bare UPDATE ... RETURNING followed by an INSERT is still
		// a read-then-write with a race, so the CTE form is the one that has to
		// work -- and it is the one T-4.2 uses.
		mustExec(t, tx, `
			WITH updated AS (
			  UPDATE att SET deadline_at = deadline_at + interval '15 min'
			   WHERE id = 1
			  RETURNING id, old.deadline_at AS prev, new.deadline_at AS next
			)
			INSERT INTO audit (entity_id, diff)
			SELECT id, jsonb_build_object('old', prev, 'new', next) FROM updated`)

		var diff string
		if err := tx.QueryRow(`SELECT diff::text FROM audit`).Scan(&diff); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"10:00:00", "10:15:00"} {
			if !strings.Contains(diff, want) {
				t.Errorf("diff %s is missing %s -- both the old and new value must be captured", diff, want)
			}
		}
	})
}

// https://www.postgresql.org/docs/18/sql-altertable.html
func TestNotNullNotValid(t *testing.T) {
	conn := openDB(t)

	t.Run("rejected in CREATE TABLE", func(t *testing.T) {
		// This is why every table in 20-data-model.md declares NOT NULL inline
		// instead: the construct simply is not available there.
		scratch(t, conn, func(tx *sql.Tx) {
			if _, err := tx.Exec(`CREATE TEMP TABLE nn_bad (id int, val text NOT NULL NOT VALID)`); err == nil {
				t.Error("CREATE TABLE accepted NOT NULL NOT VALID; the plan says it cannot")
			}
		})
	})

	t.Run("rejected via SET NOT NULL", func(t *testing.T) {
		scratch(t, conn, func(tx *sql.Tx) {
			mustExec(t, tx, `CREATE TEMP TABLE nn (id int, val text)`)
			if _, err := tx.Exec(`ALTER TABLE nn ALTER COLUMN val SET NOT NULL NOT VALID`); err == nil {
				t.Error("SET NOT NULL accepted NOT VALID; only ADD CONSTRAINT does")
			}
		})
	})

	t.Run("works via ADD CONSTRAINT, and VALIDATE enforces it", func(t *testing.T) {
		scratch(t, conn, func(tx *sql.Tx) {
			mustExec(t, tx, `CREATE TEMP TABLE nn (id int, val text)`)
			mustExec(t, tx, `INSERT INTO nn VALUES (1, NULL), (2, 'x')`)

			// Adding it does not scan, so it succeeds despite the NULL.
			mustExec(t, tx, `ALTER TABLE nn ADD CONSTRAINT nn_val_nn NOT NULL val NOT VALID`)

			if _, err := tx.Exec(`ALTER TABLE nn VALIDATE CONSTRAINT nn_val_nn`); err == nil {
				t.Error("VALIDATE succeeded while a NULL was present")
			} else if !strings.Contains(err.Error(), "null") {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	})
}

// The two facts the plan originally got wrong about Vietnamese text search.
func TestAccentFolding(t *testing.T) {
	conn := openDB(t)

	t.Run("pg_trgm does NOT fold accents", func(t *testing.T) {
		// The original D-11 claimed trigram matching handled this. It does not:
		// pg_trgm is case-insensitive but not accent-insensitive, which is why
		// app.immutable_unaccent() exists at all.
		var matches bool
		if err := conn.QueryRow(`SELECT 'nghé' ILIKE '%nghe%'`).Scan(&matches); err != nil {
			t.Fatal(err)
		}
		if matches {
			t.Error("ILIKE now folds accents; D-11's unaccent wrapper may be unnecessary -- re-check before removing it")
		}
	})

	t.Run("both unaccent() forms are STABLE, not IMMUTABLE", func(t *testing.T) {
		rows, err := conn.Query(`
			SELECT pg_get_function_identity_arguments(p.oid), p.provolatile
			FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
			WHERE p.proname = 'unaccent'`)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()

		found := 0
		for rows.Next() {
			var args, volatility string
			if err := rows.Scan(&args, &volatility); err != nil {
				t.Fatal(err)
			}
			found++
			if volatility != "s" {
				t.Errorf("unaccent(%s) is %q, expected STABLE -- if it became IMMUTABLE, "+
					"app.immutable_unaccent() is no longer needed", args, volatility)
			}
		}
		if found == 0 {
			t.Fatal("unaccent extension is not installed")
		}
	})

	// Each of the next three runs in its own transaction. A failed statement
	// aborts the surrounding transaction in Postgres, so a negative assertion
	// and a positive one cannot share it.
	t.Run("a bare unaccent() is rejected in an index expression", func(t *testing.T) {
		scratch(t, conn, func(tx *sql.Tx) {
			mustExec(t, tx, `CREATE TEMP TABLE q1 (prompt text)`)
			_, err := tx.Exec(`CREATE INDEX ON q1 USING gin (unaccent(prompt) gin_trgm_ops)`)
			if err == nil {
				t.Fatal("a bare unaccent() was accepted; app.immutable_unaccent() would be unnecessary")
			}
			if !strings.Contains(err.Error(), "IMMUTABLE") {
				t.Errorf("rejected for an unexpected reason: %v", err)
			}
		})
	})

	t.Run("the wrapper is accepted in an index expression", func(t *testing.T) {
		scratch(t, conn, func(tx *sql.Tx) {
			mustExec(t, tx, `CREATE TEMP TABLE q2 (prompt text)`)
			mustExec(t, tx, `CREATE INDEX ON q2 USING gin (app.immutable_unaccent(lower(prompt)) gin_trgm_ops)`)
		})
	})

	t.Run("the wrapper folds Vietnamese, including the Đ stroke", func(t *testing.T) {
		// Đ is the one Vietnamese character plain Unicode decomposition misses:
		// it is a stroke, not a combining diacritic.
		for _, tc := range []struct{ in, want string }{
			{"Nghé", "nghe"},
			{"phát âm", "phat am"},
			{"Đường", "duong"},
			{"tiếng Việt", "tieng viet"},
		} {
			var got string
			if err := conn.QueryRow(`SELECT app.immutable_unaccent(lower($1))`, tc.in).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("immutable_unaccent(lower(%q)) = %q, want %q", tc.in, got, tc.want)
			}
		}
	})
}

// mustExec runs a statement that is expected to succeed. Shared with
// constraints_test.go, hence the variadic args.
func mustExec(t *testing.T, tx *sql.Tx, stmt string, args ...any) {
	t.Helper()
	if _, err := tx.Exec(stmt, args...); err != nil {
		t.Fatalf("should have succeeded (%.80s): %v", strings.TrimSpace(stmt), err)
	}
}
