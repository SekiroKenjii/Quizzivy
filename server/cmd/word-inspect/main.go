// Command word-inspect inventories local DOCX source evidence without recognition
// or network access. Full evidence is opt-in and may contain private answer keys.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	output := flag.String("json", "", "write private source evidence to a new file (0600); never overwrite")
	flag.Parse()
	if flag.NArg() != 1 {
		return fmt.Errorf("usage: word-inspect [-json private-evidence.json] source.docx")
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
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source must be a regular file")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	started := time.Now()
	inspection, err := word.Inspect(ctx, f, info.Size(), word.DefaultLimits())
	if err != nil {
		return err
	}
	resolution, err := word.Resolve(ctx, inspection)
	if err != nil {
		return err
	}
	if *output != "" {
		if err := writeEvidence(*output, evidence{Inspection: inspection, Resolution: resolution}); err != nil {
			return err
		}
	}
	return json.NewEncoder(os.Stdout).Encode(summarize(inspection, resolution, time.Since(started).Milliseconds()))
}

type evidence struct {
	word.Inspection
	Resolution word.Resolution `json:"resolution"`
}

type summary struct {
	Parts              int            `json:"parts"`
	Paragraphs         int            `json:"paragraphs"`
	Runs               int            `json:"runs"`
	Assets             int            `json:"assets"`
	Findings           map[string]int `json:"findings"`
	ResolvedLabels     int            `json:"resolvedLabels"`
	UnresolvedLabels   int            `json:"unresolvedLabels"`
	ResolvedMarks      int            `json:"resolvedMarks"`
	ResolutionFindings map[string]int `json:"resolutionFindings"`
	ElapsedMS          int64          `json:"elapsedMs"`
}

func summarize(inspection word.Inspection, resolution word.Resolution, elapsed int64) summary {
	report := summary{Parts: len(inspection.Parts), Assets: len(inspection.Assets), Findings: map[string]int{}, ResolutionFindings: map[string]int{}, ElapsedMS: elapsed}
	for _, part := range inspection.Parts {
		report.Paragraphs += len(part.Paragraphs)
		for _, p := range part.Paragraphs {
			report.Runs += len(p.Runs)
		}
	}
	for _, finding := range inspection.Findings {
		report.Findings[finding.Code]++
	}
	for _, finding := range resolution.Findings {
		report.ResolutionFindings[finding.Code]++
	}
	for _, paragraph := range resolution.Paragraphs {
		report.addParagraph(paragraph)
	}
	return report
}

func (s *summary) addParagraph(p word.ResolvedParagraph) {
	if p.Numbering != nil {
		if p.Numbering.Resolved {
			s.ResolvedLabels++
		} else {
			s.UnresolvedLabels++
		}
	}
	for _, run := range p.Runs {
		for _, mark := range run.Marks {
			if mark.Resolved {
				s.ResolvedMarks++
			}
		}
	}
}

func writeEvidence(name string, inspection evidence) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	encodeErr := json.NewEncoder(f).Encode(inspection)
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
