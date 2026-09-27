//go:build integration

package core_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/gen/openapi"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
)

func cataloguePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), db.TestDSN(t))
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func rolledBack(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context, tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	fn(ctx, tx)
}

func builtinRoleID(t *testing.T, tx pgx.Tx, builtin access.Builtin) string {
	t.Helper()
	var id string
	if err := tx.QueryRow(context.Background(), `SELECT id::text FROM app.roles WHERE builtin_key = $1`, string(builtin)).Scan(&id); err != nil {
		t.Fatalf("role %s: %v", builtin, err)
	}
	return id
}

func revision(t *testing.T, tx pgx.Tx, roleID string) int64 {
	t.Helper()
	var r int64
	if err := tx.QueryRow(context.Background(), `SELECT revision FROM app.roles WHERE id = $1`, roleID).Scan(&r); err != nil {
		t.Fatalf("revision: %v", err)
	}
	return r
}

func refusedBy(t *testing.T, err error, constraint string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("want a refusal by %s, got %v", constraint, err)
	}
	if pgErr.Code != "23514" || pgErr.ConstraintName != constraint {
		t.Fatalf("want 23514 from %s, got %s from %q: %s", constraint, pgErr.Code, pgErr.ConstraintName, pgErr.Message)
	}
}

func TestTheContractTheCodeAndTheDatabaseHoldOneCatalogue(t *testing.T) {
	spec, err := openapi.GetSpec()
	if err != nil {
		t.Fatal(err)
	}
	schema, ok := spec.Components.Schemas["PermissionKey"]
	if !ok || schema.Value == nil {
		t.Fatal("api/openapi.yaml has no PermissionKey schema")
	}
	var contract []access.Key
	for _, v := range schema.Value.Enum {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("PermissionKey enum holds %T", v)
		}
		contract = append(contract, access.Key(s))
	}

	pool := cataloguePool(t)
	rows, err := pool.Query(context.Background(), `
		SELECT key, in_matrix
		  FROM app.permissions
		 ORDER BY array_position(ARRAY['content', 'teaching', 'people', 'system'], group_key), ordinal`)
	if err != nil {
		t.Fatal(err)
	}
	var database []access.Key
	var hidden []access.Key
	for rows.Next() {
		var key string
		var inMatrix bool
		if err := rows.Scan(&key, &inMatrix); err != nil {
			t.Fatal(err)
		}
		database = append(database, access.Key(key))
		if !inMatrix {
			hidden = append(hidden, access.Key(key))
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	code := access.All()
	if !slices.Equal(contract, code) {
		t.Errorf("the contract's PermissionKey differs from access.All():\ncontract %v\ncode     %v", contract, code)
	}
	if !slices.Equal(database, code) {
		t.Errorf("app.permissions differs from access.All():\ndatabase %v\ncode     %v", database, code)
	}
	var codeHidden []access.Key
	for _, k := range code {
		if k.Hidden() {
			codeHidden = append(codeHidden, k)
		}
	}
	if !slices.Equal(hidden, codeHidden) {
		t.Errorf("hidden keys: database %v, code %v", hidden, codeHidden)
	}
}

func TestTheBuiltInGrantsAreTheMatrixCellByCell(t *testing.T) {
	want := map[access.Builtin][]access.Key{
		access.BuiltinAdmin: {},
		access.BuiltinTeacher: {
			access.ContentTestsWrite, access.ContentTestsPublish, access.ContentQuestionsWrite,
			access.ContentMediaWrite, access.ContentShare,
			access.TeachingClassesWrite, access.TeachingAssignmentsWrite, access.TeachingGrading,
			access.TeachingAttemptsIntervene, access.TeachingAttendance,
			access.PeopleStudentsRead, access.PeopleStudentsCreate, access.PeopleStudentsResetPassword,
		},
		access.BuiltinAssistant: {
			access.ContentTestsWrite, access.ContentQuestionsWrite, access.TeachingAssignmentsWrite,
			access.TeachingGrading, access.TeachingAttendance, access.PeopleStudentsRead,
		},
		access.BuiltinStudent: {access.LearningTakeTests},
	}
	pool := cataloguePool(t)
	ctx := context.Background()
	for builtin, keys := range want {
		rows, err := pool.Query(ctx, `
			SELECT rp.permission_key
			  FROM app.role_permissions rp
			  JOIN app.roles r ON r.id = rp.role_id
			 WHERE r.builtin_key = $1`, string(builtin))
		if err != nil {
			t.Fatal(err)
		}
		got, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		stored := make([]access.Key, 0, len(got))
		for _, k := range got {
			stored = append(stored, access.Key(k))
		}
		if gotKeys, wantKeys := access.NewSet(stored...).Keys(), access.NewSet(keys...).Keys(); !slices.Equal(gotKeys, wantKeys) || len(stored) != len(keys) {
			t.Errorf("%s grants = %v, want %v", builtin, gotKeys, wantKeys)
		}
	}
	rows, err := pool.Query(ctx, `SELECT name FROM app.roles WHERE builtin_key IS NOT NULL ORDER BY array_position(ARRAY['admin', 'teacher', 'assistant', 'student'], builtin_key)`)
	if err != nil {
		t.Fatal(err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(names, []string{"Quản trị viên", "Giáo viên", "Trợ giảng", "Học viên"}) {
		t.Errorf("built-in role names = %v", names)
	}
}

func TestTheAppRoleOnlyReadsTheCatalogue(t *testing.T) {
	pool := cataloguePool(t)
	for _, relation := range []string{"app.permissions", "app.roles", "app.role_permissions", "app.student_like_roles"} {
		for privilege, want := range map[string]bool{"SELECT": true, "INSERT": false, "UPDATE": false, "DELETE": false} {
			var got bool
			if err := pool.QueryRow(context.Background(),
				`SELECT has_table_privilege('quizzivy_app', $1, $2)`, relation, privilege).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Errorf("quizzivy_app %s on %s = %v, want %v", privilege, relation, got, want)
			}
		}
	}
}

func TestTheStudentKeepsTakeTests(t *testing.T) {
	pool := cataloguePool(t)
	rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
		student := builtinRoleID(t, tx, access.BuiltinStudent)
		_, err := tx.Exec(ctx, `DELETE FROM app.role_permissions WHERE role_id = $1 AND permission_key = 'learning.take_tests'`, student)
		refusedBy(t, err, "role_permissions_keep_student_take_tests")
	})
	rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
		student := builtinRoleID(t, tx, access.BuiltinStudent)
		_, err := tx.Exec(ctx, `UPDATE app.role_permissions SET permission_key = 'teaching.grading' WHERE role_id = $1 AND permission_key = 'learning.take_tests'`, student)
		refusedBy(t, err, "role_permissions_keep_student_take_tests")
	})
}

func TestNoRoleIsGrantedAHiddenKey(t *testing.T) {
	pool := cataloguePool(t)
	for _, builtin := range []access.Builtin{access.BuiltinAdmin, access.BuiltinTeacher, access.BuiltinStudent} {
		for _, key := range []access.Key{access.ScopeAll, access.SystemAPIReference, access.SystemDataExport, access.SystemLeads} {
			rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
				role := builtinRoleID(t, tx, builtin)
				_, err := tx.Exec(ctx, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1, $2)`, role, string(key))
				refusedBy(t, err, "role_permissions_guard")
			})
		}
	}
}

func TestTheAdminStoresOnlyTakeTests(t *testing.T) {
	pool := cataloguePool(t)
	rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
		admin := builtinRoleID(t, tx, access.BuiltinAdmin)
		_, err := tx.Exec(ctx, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1, 'teaching.grading')`, admin)
		refusedBy(t, err, "role_permissions_guard")
	})
	rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
		admin := builtinRoleID(t, tx, access.BuiltinAdmin)
		if _, err := tx.Exec(ctx, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1, 'learning.take_tests')`, admin); err != nil {
			t.Fatalf("turning the Admin's Take tests on: %v", err)
		}
	})
}

func TestEveryGrantChangeBumpsTheRoleRevision(t *testing.T) {
	pool := cataloguePool(t)
	rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
		teacher := builtinRoleID(t, tx, access.BuiltinTeacher)
		assistant := builtinRoleID(t, tx, access.BuiltinAssistant)
		start := revision(t, tx, teacher)
		if _, err := tx.Exec(ctx, `DELETE FROM app.role_permissions WHERE role_id = $1 AND permission_key = 'content.share'`, teacher); err != nil {
			t.Fatal(err)
		}
		if got := revision(t, tx, teacher); got != start+1 {
			t.Fatalf("after a delete: revision %d, want %d", got, start+1)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1, 'content.share')`, teacher); err != nil {
			t.Fatal(err)
		}
		if got := revision(t, tx, teacher); got != start+2 {
			t.Fatalf("after an insert: revision %d, want %d", got, start+2)
		}
		assistantStart := revision(t, tx, assistant)
		if _, err := tx.Exec(ctx, `UPDATE app.role_permissions SET role_id = $2 WHERE role_id = $1 AND permission_key = 'content.share'`, teacher, assistant); err != nil {
			t.Fatal(err)
		}
		if got := revision(t, tx, teacher); got != start+3 {
			t.Errorf("after moving a grant away: revision %d, want %d", got, start+3)
		}
		if got := revision(t, tx, assistant); got != assistantStart+1 {
			t.Errorf("after receiving a grant: revision %d, want %d", got, assistantStart+1)
		}
	})
}

func TestABuiltInKeyNeverChanges(t *testing.T) {
	pool := cataloguePool(t)
	rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
		_, err := tx.Exec(ctx, `UPDATE app.roles SET builtin_key = NULL WHERE builtin_key = 'teacher'`)
		refusedBy(t, err, "roles_builtin_key_immutable")
	})
	rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
		if _, err := tx.Exec(ctx, `UPDATE app.roles SET name = 'Giáo viên chính' WHERE builtin_key = 'teacher'`); err != nil {
			t.Fatalf("renaming a built-in role: %v", err)
		}
	})
}

func TestTheStudentLikeViewFollowsTheGrants(t *testing.T) {
	pool := cataloguePool(t)
	rolledBack(t, pool, func(ctx context.Context, tx pgx.Tx) {
		var empty, takesTests, grades string
		for name, id := range map[string]*string{"Vai trò trống": &empty, "Chỉ làm bài": &takesTests, "Chấm bài": &grades} {
			if err := tx.QueryRow(ctx, `INSERT INTO app.roles (name, icon, color) VALUES ($1, 'user', 'gray') RETURNING id::text`, name).Scan(id); err != nil {
				t.Fatal(err)
			}
		}
		admin := builtinRoleID(t, tx, access.BuiltinAdmin)
		for _, grant := range [][2]string{
			{takesTests, "learning.take_tests"},
			{grades, "learning.take_tests"},
			{grades, "teaching.grading"},
			{admin, "learning.take_tests"},
		} {
			if _, err := tx.Exec(ctx, `INSERT INTO app.role_permissions (role_id, permission_key) VALUES ($1, $2)`, grant[0], grant[1]); err != nil {
				t.Fatal(err)
			}
		}
		rows, err := tx.Query(ctx, `
			SELECT coalesce(r.builtin_key, r.name)
			  FROM app.student_like_roles s
			  JOIN app.roles r ON r.id = s.id`)
		if err != nil {
			t.Fatal(err)
		}
		got, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(got)
		want := []string{"student", "Chỉ làm bài", "Vai trò trống"}
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("student_like_roles = %v, want %v", got, want)
		}
	})
}
