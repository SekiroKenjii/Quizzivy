// Command word-extract extracts local private DOCX blocks without recognition or external services.
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
	"time"

	"quizzivy/internal/platform/word"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	output := flag.String("json", "", "write private source blocks to a new file (0600); never overwrite")
	flag.Parse()
	if flag.NArg() != 1 {
		return fmt.Errorf("usage: word-extract [-json private-blocks.json] source.docx")
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	limits := word.DefaultLimits()
	if !info.Mode().IsRegular() || info.Size() > limits.CompressedBytes {
		return fmt.Errorf("source must be a regular file within the compressed size limit")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	hash := sha256.New()
	if _, err := io.Copy(hash, io.NewSectionReader(f, 0, info.Size())); err != nil {
		return err
	}
	started := time.Now()
	result, err := word.Extract(ctx, f, info.Size(), fmt.Sprintf("sha256:%x", hash.Sum(nil)), limits)
	if err != nil {
		return err
	}
	if *output != "" {
		if err := writeBlocks(*output, result); err != nil {
			return err
		}
	}
	return json.NewEncoder(os.Stdout).Encode(summarize(result, time.Since(started).Milliseconds()))
}

type extractionSummary struct {
	Version       string         `json:"version"`
	Blocks        int            `json:"blocks"`
	Meaningful    int            `json:"meaningful"`
	Paragraphs    int            `json:"paragraphs"`
	Fragments     int            `json:"fragments"`
	Cells         int            `json:"cells"`
	Assets        int            `json:"assets"`
	Findings      map[string]int `json:"findings"`
	ReviewReasons map[string]int `json:"reviewReasons"`
	ElapsedMS     int64          `json:"elapsedMs"`
}

func summarize(source word.Extraction, elapsed int64) extractionSummary {
	out := extractionSummary{Version: source.Version, Blocks: len(source.Blocks), Assets: len(source.Assets), Findings: map[string]int{}, ReviewReasons: map[string]int{}, ElapsedMS: elapsed}
	for _, b := range source.Blocks {
		out.add(b)
	}
	for _, f := range source.Findings {
		out.Findings[f.Code]++
	}
	return out
}

func (s *extractionSummary) add(b word.SourceBlock) {
	if b.Meaningful {
		s.Meaningful++
	}
	if b.Cell != nil {
		s.Cells++
	}
	if b.Paragraph != nil {
		s.Paragraphs++
		for _, r := range b.Paragraph.Runs {
			s.Fragments += len(r.Fragments)
		}
	}
	for _, reason := range b.ReviewReasons {
		s.ReviewReasons[reason]++
	}
}

func writeBlocks(name string, source word.Extraction) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	encodeErr := json.NewEncoder(f).Encode(source)
	closeErr := f.Close()
	if encodeErr != nil || closeErr != nil {
		_ = os.Remove(name)
		if encodeErr != nil {
			return encodeErr
		}
		return closeErr
	}
	return nil
}
