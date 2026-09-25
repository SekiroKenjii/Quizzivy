package wordconvert_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/platform/wordconvert"
)

func TestConverterRefusesMutableImageOrSharedWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, image := range []string{"debian:latest", "converter:development", "", "sha256:bad"} {
		if _, err := wordconvert.New(root, "/usr/bin/docker", image, time.Minute); !errors.Is(err, wordconvert.ErrConfiguration) {
			t.Fatalf("mutable runtime accepted: %v", err)
		}
	}
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := wordconvert.New(root, "/usr/bin/docker", "sha256:"+strings.Repeat("a", 64), time.Minute); !errors.Is(err, wordconvert.ErrConfiguration) {
		t.Fatal("shared workspace accepted")
	}
}

func TestConverterRefusesInvalidInputBeforeInvokingRuntime(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("runtime deliberately rejects root workers")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	converter, err := wordconvert.New(root, filepath.Join(root, "nonexistent-docker"), "sha256:"+strings.Repeat("a", 64), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"doc", "docx", "pdf", "../docx"} {
		if _, err := converter.Convert(context.Background(), bytes.NewReader([]byte("not a word source")), format); !errors.Is(err, wordconvert.ErrSource) {
			t.Fatalf("invalid input reached runtime: %v", err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid-input staging leaked")
	}
}

func TestConverterRefusesSymlinkWorkspace(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := wordconvert.New(link, "/usr/bin/docker", "sha256:"+strings.Repeat("a", 64), time.Minute); !errors.Is(err, wordconvert.ErrConfiguration) {
		t.Fatal("symlink workspace accepted")
	}
}

func TestConverterDiskWatchdogStopsAndCleansOversizedJob(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("runtime deliberately rejects root workers")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	runtime := filepath.Join(t.TempDir(), "fake-docker")
	script := "#!" + python + ` 
import pathlib,sys,time
if sys.argv[1]=='run':
 mount=next(arg for arg in sys.argv if arg.startswith('type=bind,src=') and arg.endswith(',dst=/work'))
 work=pathlib.Path(mount[len('type=bind,src='):-len(',dst=/work')])
 with (work/'oversized').open('wb') as f: f.truncate(257*1024*1024)
 time.sleep(10)
`
	if err := os.WriteFile(runtime, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	converter, err := wordconvert.New(root, runtime, "sha256:"+strings.Repeat("a", 64), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	input := []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}
	if _, err := converter.Convert(context.Background(), bytes.NewReader(input), "doc"); !errors.Is(err, wordconvert.ErrLimit) {
		t.Fatalf("disk limit ignored: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("oversized job retained")
	}
}
