package wiring_test

import (
	"os"
	"path/filepath"
	"testing"

	"quizzivy/internal/core/wiring"
)

func TestPrivateWorkDirCreatesAMissingDirectoryAt0700(t *testing.T) {
	path := filepath.Join(t.TempDir(), "imports")
	if err := wiring.PrivateWorkDir(path); err != nil {
		t.Fatalf("PrivateWorkDir: %v", err)
	}
	assertMode(t, path, 0o700)
}

func TestPrivateWorkDirNarrowsADirectoryAnImageShippedWider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "imports")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := wiring.PrivateWorkDir(path); err != nil {
		t.Fatalf("PrivateWorkDir on a 0755 directory: %v", err)
	}
	assertMode(t, path, 0o700)
}

func TestPrivateWorkDirRefusesAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "imports")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := wiring.PrivateWorkDir(path); err == nil {
		t.Fatal("PrivateWorkDir accepted a file")
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode = %o, want %o", got, want)
	}
}
