//go:build integration

package wordconvert_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/platform/word"
	"quizzivy/internal/platform/wordconvert"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for _, entry := range []struct{ name, text string }{
		{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`},
		{"_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`},
		{"word/document.xml", `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Question 1. Choose the underlined sound.</w:t></w:r></w:p><w:p><w:r><w:t>A. </w:t></w:r><w:r><w:rPr><w:u w:val="single"/></w:rPr><w:t>ou</w:t></w:r></w:p><w:p><w:r><w:t>B. ea</w:t></w:r></w:p></w:body></w:document>`},
	} {
		f, err := z.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, entry.text); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestDockerLegacyConversionKeepsOriginalAndNormalizesThroughNativeParser(t *testing.T) {
	c, _ := configured(t, 90*time.Second)
	legacy := legacyFixture(t, false)
	before := bytes.Clone(legacy)
	result, err := c.Convert(context.Background(), bytes.NewReader(legacy), "doc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = result.Close() }()
	if !bytes.Equal(before, legacy) || result.Manifest.Normalized == nil || result.Manifest.SourceFormat != "doc" {
		t.Fatal("legacy source lineage lost")
	}
	f, err := result.Open(*result.Manifest.Normalized)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	extracted, err := word.Extract(context.Background(), bytes.NewReader(data), int64(len(data)), "converted-source", word.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	underline := false
	for _, b := range extracted.Blocks {
		if b.Paragraph == nil {
			continue
		}
		for _, r := range b.Paragraph.Runs {
			for _, f := range r.Fragments {
				text.WriteString(f.Text)
			}
			for _, m := range r.Marks {
				if m.Name == "u" && m.Resolved && m.Value == "single" {
					underline = true
				}
			}
		}
	}
	if !strings.Contains(text.String(), "Question 1. Choose the underlined sound.") || !underline {
		t.Fatal("legacy conversion lost text or meaningful underline")
	}
}

func legacyFixture(t *testing.T, encrypted bool) []byte {
	t.Helper()
	job := t.TempDir()
	for _, name := range []string{"input", "work", "fallback"} {
		if err := os.Mkdir(filepath.Join(job, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(job, "input", "source.docx"), fixture(t), 0o600); err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("testdata/make_legacy.py")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	args := []string{"run", "--rm", "--pull=never", "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--memory=512m", "--memory-swap=512m", "--cpus=1", "--pids-limit=64", "--user", strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid()), "--mount", "type=bind,src=" + filepath.Join(job, "input") + ",dst=/input,readonly", "--mount", "type=bind,src=" + filepath.Join(job, "work") + ",dst=/work", "--mount", "type=bind,src=" + filepath.Join(job, "fallback") + ",dst=/tmp", "--mount", "type=bind,src=" + script + ",dst=/make_legacy.py,readonly", "--entrypoint", "python3", os.Getenv("TEST_WORD_CONVERTER_IMAGE"), "/make_legacy.py"}
	if encrypted {
		args = append(args, "encrypted")
	}
	if output, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput(); err != nil {
		t.Fatalf("synthetic legacy fixture: %v %s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(job, "work", "fixture.doc"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func configured(t *testing.T, timeout time.Duration) (*wordconvert.Converter, string) {
	t.Helper()
	image := os.Getenv("TEST_WORD_CONVERTER_IMAGE")
	if image == "" {
		t.Skip("TEST_WORD_CONVERTER_IMAGE not set")
	}
	binary, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	c, err := wordconvert.New(root, binary, image, timeout)
	if err != nil {
		t.Fatal(err)
	}
	return c, root
}

func TestDockerRenditionProducesBoundedPrivatePagesAndRemovesJob(t *testing.T) {
	c, root := configured(t, 90*time.Second)
	result, err := c.Convert(context.Background(), bytes.NewReader(fixture(t)), "docx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = result.Close() })
	if result.Manifest.Normalized != nil || len(result.Manifest.Pages) != 1 || result.Manifest.SourceFormat != "docx" || len(result.SourceSHA256) != 64 || len(result.Artifacts) != 2 {
		t.Fatalf("incomplete rendition: %+v", result.Manifest)
	}
	page, err := result.Open(result.Manifest.Pages[0])
	if err != nil {
		t.Fatal(err)
	}
	im, err := png.Decode(page)
	_ = page.Close()
	if err != nil || im.Bounds().Dx() > 1600 || im.Bounds().Dy() > 1600 {
		t.Fatalf("invalid page: %v", err)
	}
	for _, name := range []string{"../../.env", "/etc/passwd", "manifest.json"} {
		if _, err := result.Open(name); !errors.Is(err, wordconvert.ErrSource) {
			t.Fatal("unlisted artifact readable")
		}
	}
	if err := result.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("transient job retained after Close")
	}
}

func TestDockerConversionTimeoutAndSingleSlot(t *testing.T) {
	c, root := configured(t, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := fixture(t)
	finished := make(chan error, 1)
	go func() {
		result, err := c.Convert(ctx, bytes.NewReader(source), "docx")
		if result != nil {
			_ = result.Close()
		}
		finished <- err
	}()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := c.Convert(ctx, bytes.NewReader(source), "docx"); !errors.Is(err, wordconvert.ErrBusy) {
		t.Fatalf("second conversion was not bounded: %v", err)
	}
	if err := <-finished; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout not enforced: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("timed-out job not cleaned")
	}
	binary, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(binary, "ps", "--filter", "label=quizzivy.word-converter=true", "--format", "{{.Names}}").Output()
	if err != nil || strings.TrimSpace(string(output)) != "" {
		t.Fatalf("converter survived timeout: %v", err)
	}
}

func TestDockerCancellationDoesNotLeaveContainerOrStaging(t *testing.T) {
	c, root := configured(t, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	source := fixture(t)
	go func() {
		result, err := c.Convert(ctx, bytes.NewReader(source), "docx")
		if result != nil {
			_ = result.Close()
		}
		done <- err
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		output, err := exec.Command("docker", "ps", "--filter", "label=quizzivy.word-converter=true", "--format", "{{.Names}}").Output()
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(output)) != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation not preserved: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("cancelled job staging retained")
	}
	output, err := exec.Command("docker", "ps", "--filter", "label=quizzivy.word-converter=true", "--format", "{{.Names}}").Output()
	if err != nil || strings.TrimSpace(string(output)) != "" {
		t.Fatal("container survived cancellation")
	}
}

func TestDockerExistingGlobalSlotIsNotKilledByAnotherWorker(t *testing.T) {
	c, root := configured(t, time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "docker", "run", "--detach", "--pull=never", "--name", "quizzivy-word-converter-slot", "--label", "quizzivy.word-converter=true", "--label", "quizzivy.word-converter-owner=other-worker", "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--memory=32m", "--memory-swap=32m", "--pids-limit=8", "--user", "65532:65532", "--entrypoint", "python3", os.Getenv("TEST_WORD_CONVERTER_IMAGE"), "-c", "import time; time.sleep(60)").Output()
	if err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSpace(string(output))
	t.Cleanup(func() {
		bounded, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = exec.CommandContext(bounded, "docker", "rm", "--force", id).Run()
	})
	if _, err := c.Convert(context.Background(), bytes.NewReader(fixture(t)), "docx"); !errors.Is(err, wordconvert.ErrBusy) {
		t.Fatalf("global slot was not enforced: %v", err)
	}
	state, err := exec.CommandContext(ctx, "docker", "inspect", id, "--format", "{{.State.Running}}").Output()
	if err != nil || strings.TrimSpace(string(state)) != "true" {
		t.Fatal("another worker's container was killed")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("busy job staging leaked")
	}
}

func TestDockerEncryptedLegacySourceDoesNotProduceEmptySuccess(t *testing.T) {
	c, root := configured(t, 30*time.Second)
	legacy := legacyFixture(t, true)
	result, err := c.Convert(context.Background(), bytes.NewReader(legacy), "doc")
	if result != nil {
		_ = result.Close()
		t.Fatal("locked source was accepted")
	}
	if !errors.Is(err, wordconvert.ErrConversion) {
		t.Fatalf("locked source did not fail cleanly: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("failed conversion left partial artifacts")
	}
}

func TestDockerCompletedOrphanSlotCanBeReclaimed(t *testing.T) {
	c, _ := configured(t, time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	args := []string{"run", "--pull=never", "--name", "quizzivy-word-converter-slot", "--label", "quizzivy.word-converter=true", "--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--memory=32m", "--memory-swap=32m", "--pids-limit=8", "--user", "65532:65532", "--entrypoint", "python3", os.Getenv("TEST_WORD_CONVERTER_IMAGE"), "-c", "pass"}
	if err := exec.CommandContext(ctx, "docker", args...).Run(); err != nil {
		t.Fatal(err)
	}
	result, err := c.Convert(context.Background(), bytes.NewReader(fixture(t)), "docx")
	if err != nil {
		t.Fatal(err)
	}
	if err := result.Close(); err != nil {
		t.Fatal(err)
	}
}
