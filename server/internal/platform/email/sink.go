package email

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9-]`)

// Sink is the Sender outside production: it writes each message as a JSON
// file under Dir, readable only by the process's user, and logs one line that
// names the file and the recipient's domain. The live end-to-end suite reads
// set-password links from these files.
type Sink struct {
	Dir    string
	Logger *slog.Logger
}

type sunk struct {
	To             string            `json:"to"`
	Subject        string            `json:"subject"`
	Text           string            `json:"text"`
	HTML           string            `json:"html"`
	ReplyTo        string            `json:"replyTo,omitempty"`
	IdempotencyKey string            `json:"idempotencyKey"`
	Tags           map[string]string `json:"tags,omitempty"`
	WrittenAt      time.Time         `json:"writtenAt"`
}

// Send writes m to a new file and answers "sink:" followed by its name.
func (s Sink) Send(_ context.Context, m Message) (string, error) {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return "", &SendError{Kind: ErrTransient, Code: "sink_unwritable"}
	}
	payload, err := json.MarshalIndent(sunk{
		To: m.To, Subject: m.Subject, Text: m.Text, HTML: m.HTML, ReplyTo: m.ReplyTo,
		IdempotencyKey: m.IdempotencyKey, Tags: m.Tags, WrittenAt: time.Now().UTC(),
	}, "", "  ")
	if err != nil {
		return "", &SendError{Kind: ErrPermanent, Code: "encode"}
	}
	name := time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + unsafeName.ReplaceAllString(m.IdempotencyKey, "_") + ".json"
	if err := os.WriteFile(filepath.Join(s.Dir, name), payload, 0o600); err != nil {
		return "", &SendError{Kind: ErrTransient, Code: "sink_unwritable"}
	}
	if s.Logger != nil {
		s.Logger.Info("email written to the sink", "file", name, "toDomain", Domain(m.To))
	}
	return "sink:" + name, nil
}
