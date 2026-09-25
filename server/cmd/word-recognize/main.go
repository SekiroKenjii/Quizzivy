// Command word-recognize evaluates local native DOCX recognition without external services or assessment writes.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
	"quizzivy/internal/platform/word"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	output := flag.String("json", "", "write private candidate to a new 0600 file; never overwrite")
	key := flag.String("key", "", "optional local DOCX answer key")
	paper := flag.Int("paper", 0, "paper number to read from a multi-paper answer key")
	flag.Parse()
	if flag.NArg() != 1 {
		return fmt.Errorf("usage: word-recognize [-key answers.docx] [-json candidate.json] exam.docx")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	started := time.Now()
	exam, err := extract(ctx, flag.Arg(0), "exam")
	if err != nil {
		return err
	}
	docs := []domain.EvidenceDocument{exam}
	if *key != "" {
		answers, err := extract(ctx, *key, "answer_key")
		if err != nil {
			return err
		}
		docs = append(docs, answers)
	}
	candidate, err := recognition.Recognize(ctx, docs, domain.RecognitionProfile{KeyPaper: *paper})
	if err != nil {
		return err
	}
	if *output != "" {
		if err := writeCandidate(*output, candidate); err != nil {
			return err
		}
	}
	return json.NewEncoder(os.Stdout).Encode(summary(candidate, time.Since(started).Milliseconds()))
}

func extract(ctx context.Context, name, role string) (domain.EvidenceDocument, error) {
	f, err := os.Open(name)
	if err != nil {
		return domain.EvidenceDocument{}, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return domain.EvidenceDocument{}, err
	}
	limits := word.DefaultLimits()
	if !info.Mode().IsRegular() || info.Size() > limits.CompressedBytes {
		return domain.EvidenceDocument{}, domain.ErrTooLarge
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, io.NewSectionReader(f, 0, info.Size())); err != nil {
		return domain.EvidenceDocument{}, err
	}
	raw, err := word.Extract(ctx, f, info.Size(), fmt.Sprintf("sha256:%x", hash.Sum(nil)), limits)
	if err != nil {
		return domain.EvidenceDocument{}, err
	}
	return adapters.ImportEvidence(raw, role), nil
}

func writeCandidate(name string, draft domain.Draft) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	writeErr := json.NewEncoder(f).Encode(draft)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(name)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return nil
}

type recognitionSummary struct {
	Version   string         `json:"version"`
	Title     string         `json:"title"`
	Sections  int            `json:"sections"`
	Groups    int            `json:"groups"`
	Questions int            `json:"questions"`
	Types     map[string]int `json:"types"`
	Answers   map[string]int `json:"answers"`
	Notices   map[string]int `json:"notices"`
	ElapsedMS int64          `json:"elapsedMs"`
}

func summary(d domain.Draft, elapsed int64) recognitionSummary {
	out := recognitionSummary{Version: recognition.Version, Title: d.Title, Sections: len(d.Sections), Types: map[string]int{}, Answers: map[string]int{}, Notices: map[string]int{}, ElapsedMS: elapsed}
	for _, s := range d.Sections {
		for _, it := range s.Items {
			if it.Group != nil {
				out.Groups++
			}
		}
	}
	for _, q := range d.Questions() {
		out.Questions++
		out.Types[q.Type]++
		out.Answers[string(q.Answer.State)]++
	}
	for _, n := range d.Notices {
		out.Notices[n.Code+"/"+string(n.Severity)] += 1
	}
	return out
}
