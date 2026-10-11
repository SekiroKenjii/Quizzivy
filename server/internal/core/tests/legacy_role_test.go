package core_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var legacyRolePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bapp\.user_role\b`),
	regexp.MustCompile(`(?i)\busers\.role\b`),
	regexp.MustCompile(`(?i)\brole\s*=\s*'(admin|student)'`),
	regexp.MustCompile(`(?i)insert\s+into\s+app\.users\s*\([^)]*\brole\b`),
}

var legacyRoleExempt = []string{
	"server/gen/",
	"server/internal/platform/db/tests/",
	"server/internal/core/tests/legacy_role_test.go",
}

func legacyRoleHits(text string) []string {
	var hits []string
	for _, pattern := range legacyRolePatterns {
		for _, at := range pattern.FindAllStringIndex(text, -1) {
			hits = append(hits, strings.TrimSpace(text[at[0]:at[1]]))
		}
	}
	return hits
}

func TestNoCodeSeedOrScriptNamesTheLegacyRole(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	for _, dir := range []string{"server", "seed", "docker"} {
		if _, err := os.Stat(filepath.Join(root, dir)); err != nil {
			t.Fatalf("the walk cannot see %s: %v", dir, err)
		}
	}
	walked := 0
	for _, dir := range []string{"server", "seed", "docker"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
			if d.IsDir() {
				if d.Name() == "node_modules" || d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".sql") && !strings.HasSuffix(path, ".sh") {
				return nil
			}
			for _, exempt := range legacyRoleExempt {
				if strings.HasPrefix(rel, exempt) {
					return nil
				}
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			walked++
			for _, hit := range legacyRoleHits(string(raw)) {
				t.Errorf("%s names the legacy role: %q", rel, hit)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if walked < 100 {
		t.Errorf("the walk read %d files; it is not looking where it should", walked)
	}
}

func TestTheLegacyRoleScanSeesWhatItIsMeantToSee(t *testing.T) {
	for _, text := range []string{
		"CAST($1 AS app.user_role)",
		"SELECT u.id FROM app.users u WHERE app.users.role = $1",
		"UPDATE app.users SET x = 1 WHERE role = 'admin'",
		"WHERE u.ROLE = 'student'",
		"INSERT INTO app.users (email, full_name, role) VALUES ($1, $2, $3)",
		"INSERT INTO app.users\n  (id, email,\n   role, password_hash)",
	} {
		if len(legacyRoleHits(text)) == 0 {
			t.Errorf("the scan missed %q", text)
		}
	}
	for _, text := range []string{
		"INSERT INTO app.users (email, full_name, role_id) VALUES ($1, $2, $3)",
		"SELECT u.role_id FROM app.users u",
		"WHERE r.builtin_key = 'admin'",
		"app.users_sync_role_id",
		"WHERE source.role = 'exam'",
	} {
		if hits := legacyRoleHits(text); len(hits) != 0 {
			t.Errorf("the scan flagged %q: %v", text, hits)
		}
	}
}
