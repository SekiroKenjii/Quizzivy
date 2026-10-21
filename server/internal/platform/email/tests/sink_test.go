package email_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"quizzivy/internal/platform/email"
)

func TestTheSinkWritesOneReadableFilePerMessageAndLogsOnlyTheRecipientsDomain(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sink")
	var logs strings.Builder
	sink := email.Sink{Dir: dir, Logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	m := sampleMessage()
	m.Text = "https://app.quizzivy.com/set-password#token=abc"
	id, err := sink.Send(context.Background(), m)
	if err != nil || !strings.HasPrefix(id, "sink:") {
		t.Fatalf("send: %q %v", id, err)
	}
	if _, err := sink.Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 2 {
		t.Fatalf("files: %v %v", files, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, strings.TrimPrefix(id, "sink:")))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil || got["to"] != "an@example.com" || got["text"] != m.Text {
		t.Fatalf("file: %v %v", got, err)
	}
	info, _ := os.Stat(filepath.Join(dir, strings.TrimPrefix(id, "sink:")))
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", info.Mode().Perm())
	}
	if line := logs.String(); !strings.Contains(line, `"toDomain":"example.com"`) || strings.Contains(line, "an@example.com") || strings.Contains(line, "token") {
		t.Errorf("the log line: %s", line)
	}
}
