//go:build integration

package db_test

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const superuserDSN = "TEST_SUPERUSER_DATABASE_URL"

var provisionedRoles = []string{"quizzivy_migrate", "quizzivy_app", "quizzivy_maintenance"}

func provisionScript(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller for the script path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "scripts", "provision-db.sh")
}

func randomName(t *testing.T, prefix string) string {
	t.Helper()
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	return prefix + hex.EncodeToString(raw)
}

type roleState struct {
	Password string
	Super    bool
	CreateDB bool
	CreateRL bool
	Login    bool
	ConnLim  int
}

func roleStates(t *testing.T, admin *sql.DB) map[string]roleState {
	t.Helper()
	rows, err := admin.Query(`SELECT rolname, coalesce(rolpassword, ''), rolsuper, rolcreatedb, rolcreaterole, rolcanlogin, rolconnlimit
	  FROM pg_authid WHERE rolname = ANY($1) ORDER BY rolname`, provisionedRoles)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	states := map[string]roleState{}
	for rows.Next() {
		var name string
		var s roleState
		if err := rows.Scan(&name, &s.Password, &s.Super, &s.CreateDB, &s.CreateRL, &s.Login, &s.ConnLim); err != nil {
			t.Fatal(err)
		}
		states[name] = s
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return states
}

func TestTheProvisioningScriptRunsAsAManagedAdminThatIsNotASuperuser(t *testing.T) {
	dsn := os.Getenv(superuserDSN)
	if dsn == "" && os.Getenv("CI") != "" {
		t.Fatalf("%s is not set in CI; the provisioning script must be proved against a real non-superuser admin", superuserDSN)
	}
	if dsn == "" {
		t.Skipf("%s is not set; skipping the provisioning script test", superuserDSN)
	}
	super, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = super.Close() })
	var isSuper bool
	if err := super.QueryRow(`SELECT rolsuper FROM pg_roles WHERE rolname = current_user`).Scan(&isSuper); err != nil || !isSuper {
		t.Fatalf("%s must name a superuser: super=%v err=%v", superuserDSN, isSuper, err)
	}
	before := roleStates(t, super)
	if len(before) != len(provisionedRoles) {
		t.Fatalf("the three roles must exist before the test (scripts/provision-db.sh): %v", before)
	}

	admin, name, password := randomName(t, "qv_admin_"), randomName(t, "qv_prov_"), randomName(t, "pw")
	t.Cleanup(func() {
		if _, err := super.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`); err != nil {
			t.Error(err)
		}
		if _, err := super.Exec(`DROP OWNED BY ` + admin); err != nil {
			t.Error(err)
		}
		if _, err := super.Exec(`DROP ROLE IF EXISTS ` + admin); err != nil {
			t.Error(err)
		}
	})
	if _, err := super.Exec(`CREATE ROLE ` + admin + ` LOGIN PASSWORD '` + password + `' CREATEROLE CREATEDB`); err != nil {
		t.Fatal(err)
	}
	if _, err := super.Exec(`GRANT ` + strings.Join(provisionedRoles, ", ") + ` TO ` + admin + ` WITH ADMIN OPTION, INHERIT FALSE, SET FALSE`); err != nil {
		t.Fatal(err)
	}

	adminURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	adminURL.User = url.UserPassword(admin, password)
	adminURL.Path = "/postgres"
	cmd := exec.Command("bash", provisionScript(t), adminURL.String())
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "QUIZZIVY_MANAGED=1", "QUIZZIVY_DB_NAME=" + name}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("scripts/provision-db.sh as a CREATEROLE CREATEDB admin that is not a superuser: %v\n%s", err, out)
	}
	for _, role := range provisionedRoles {
		if !strings.Contains(string(out), "password of "+role+" left as it was") {
			t.Errorf("the script did not say it left the password of %s as it was:\n%s", role, out)
		}
	}

	var owner string
	if err := super.QueryRow(`SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname = $1`, name).Scan(&owner); err != nil || owner != "quizzivy_migrate" {
		t.Errorf("database %s owner = %q, err = %v, want quizzivy_migrate", name, owner, err)
	}
	after := roleStates(t, super)
	for _, role := range provisionedRoles {
		if before[role] != after[role] {
			t.Errorf("the script changed %s: before %+v, after %+v", role, before[role], after[role])
		}
	}
}
