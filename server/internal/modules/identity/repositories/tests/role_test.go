//go:build integration

package repositories_test

import (
	"context"
	"testing"
)

func TestAUserIsReadWithTheRoleTheyHold(t *testing.T) {
	w := profileFixture(t)
	ctx := context.Background()

	var studentID, studentName string
	if err := w.tx.QueryRow(ctx, `SELECT id::text, name FROM app.roles WHERE builtin_key = 'student'`).Scan(&studentID, &studentName); err != nil {
		t.Fatal(err)
	}
	builtin, err := w.users.FindUserByID(ctx, w.id)
	if err != nil {
		t.Fatal(err)
	}
	if builtin.Role.ID != studentID || builtin.Role.Name != studentName || builtin.Role.BuiltinKey == nil || *builtin.Role.BuiltinKey != "student" {
		t.Fatalf("a built-in role is read as %+v, want the Student role %s %q", builtin.Role, studentID, studentName)
	}

	var customID, email string
	if err := w.tx.QueryRow(ctx, `INSERT INTO app.roles (name, icon, color) VALUES ('Trợ lý '||uuidv7()::text, 'star', 'blue') RETURNING id::text`).Scan(&customID); err != nil {
		t.Fatal(err)
	}
	if err := w.tx.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role_id) VALUES ('r5-role-'||uuidv7()::text||'@example.com', 'Có vai trò riêng', $1::uuid) RETURNING email`, customID).Scan(&email); err != nil {
		t.Fatal(err)
	}
	custom, err := w.users.FindUserByEmail(ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	if custom.Role.ID != customID || custom.Role.BuiltinKey != nil || custom.Role.Name == "" {
		t.Fatalf("a custom role is read as %+v, want id %s and no built-in key", custom.Role, customID)
	}

	if _, err := w.tx.Exec(ctx, `UPDATE app.roles SET name = 'Học viên '||uuidv7()::text WHERE builtin_key = 'student'`); err != nil {
		t.Fatal(err)
	}
	renamed, err := w.users.FindUserByID(ctx, w.id)
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Role.Name == studentName || renamed.Role.BuiltinKey == nil || *renamed.Role.BuiltinKey != "student" {
		t.Fatalf("a renamed built-in role is read as %+v, want the new name under the same key", renamed.Role)
	}
}
