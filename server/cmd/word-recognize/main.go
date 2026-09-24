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
	candidate, err := recognition.Recognize(ctx, docs, domain.RecognitionProfile{Version: "auto-v1"})
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

func writeCandidate(name string, candidate domain.Candidate) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	writeErr := json.NewEncoder(f).Encode(candidate)
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
	Version      string         `json:"version"`
	Sections     int            `json:"sections"`
	Questions    int            `json:"questions"`
	Answers      map[string]int `json:"answers"`
	Issues       map[string]int `json:"issues"`
	SourceBlocks int            `json:"sourceBlocks"`
	ElapsedMS    int64          `json:"elapsedMs"`
}

func summary(c domain.Candidate, elapsed int64) recognitionSummary {
	out := recognitionSummary{Version: c.RecognizerVersion, Sections: len(c.Sections), Questions: len(c.Questions), Answers: map[string]int{}, Issues: map[string]int{}, SourceBlocks: len(c.Coverage), ElapsedMS: elapsed}
	for _, q := range c.Questions {
		out.Answers[q.Answer.State]++
	}
	for _, i := range c.Issues {
		out.Issues[i.Code]++
	}
	return out
}
