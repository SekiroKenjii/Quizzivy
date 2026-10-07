package adapters

import (
	"context"
	"fmt"
	"golang.org/x/text/unicode/norm"
	"io"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/domain"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TextEvidenceVersion identifies the deterministic plain-text line projection.
const TextEvidenceVersion = "text-lines-v1"

const textFormat = "text"
const textParagraph = "paragraph"

// TextEvidence projects bounded plain text into paragraphs retaining normalized line numbers.
func TextEvidence(text, sourceID, role string) (domain.EvidenceDocument, error) {
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return domain.EvidenceDocument{}, domain.ErrInvalid
	}
	text = norm.NFC.String(text)
	if utf8.RuneCountInString(text) > domain.MaxPasteCharacters {
		return domain.EvidenceDocument{}, domain.ErrTooLarge
	}
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	out := domain.EvidenceDocument{SourceID: sourceID, Role: role, Version: TextEvidenceVersion, Findings: []domain.EvidenceFinding{{Code: "TEXT_MARKS_UNAVAILABLE", Main: true}}, Blocks: []domain.EvidenceBlock{}}
	for i, line := range strings.Split(text, "\n") {
		if strings.TrimFunc(line, unicode.IsSpace) == "" {
			continue
		}
		if len(out.Blocks) >= domain.MaxPasteLines {
			return domain.EvidenceDocument{}, domain.ErrTooLarge
		}
		out.Blocks = append(out.Blocks, domain.EvidenceBlock{ID: fmt.Sprintf("l%d", i+1), Kind: textParagraph, Main: true, Meaningful: true, Safe: true, Text: line, Spans: []domain.EvidenceSpan{}, Reasons: []string{}})
	}
	if len(out.Blocks) == 0 {
		return domain.EvidenceDocument{}, domain.ErrInvalid
	}
	return out, nil
}

func (p ImportProcessing) extractText(ctx context.Context, in ports.DocumentInput) (ports.StageOutput, error) {
	if in.Bytes < 1 || in.Bytes > 4*domain.MaxPasteCharacters {
		return ports.StageOutput{}, domain.ErrTooLarge
	}
	data := make([]byte, in.Bytes)
	if _, err := io.ReadFull(io.NewSectionReader(in.Body, 0, in.Bytes), data); err != nil {
		return ports.StageOutput{}, fmt.Errorf("read text source: %w", err)
	}
	evidence, err := TextEvidence(string(data), in.Identity, in.Role)
	if err != nil {
		return ports.StageOutput{}, err
	}
	inventory := struct {
		Version    string `json:"version"`
		Characters int    `json:"characters"`
		Lines      int    `json:"lines"`
	}{TextEvidenceVersion, utf8.RuneCountInString(string(data)), len(evidence.Blocks)}
	return stageExtraction(ctx, p.WorkDir, in, extracted[domain.EvidenceBlock]{component: p.ExtractionVersion(in.Format), version: TextEvidenceVersion, identity: in.Identity, mainPart: textFormat, evidence: evidence, blocks: evidence.Blocks, inventory: inventory})
}
