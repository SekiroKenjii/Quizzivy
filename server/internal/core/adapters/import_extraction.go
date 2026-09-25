package adapters

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/word"
)

func (p ImportProcessing) Extract(ctx context.Context, in ports.DocumentInput) (ports.StageOutput, error) {
	if in.Format == pdfFormat {
		return p.extractPDF(ctx, in)
	}
	raw, err := word.Extract(ctx, in.Body, in.Bytes, in.Identity, word.DefaultLimits())
	if err != nil {
		return ports.StageOutput{}, extractionFailure(err)
	}
	evidence := ImportEvidence(raw, in.Role)
	blocks := raw.Blocks
	raw.Blocks = nil
	return stageExtraction(ctx, p.WorkDir, in, extracted[word.SourceBlock]{component: p.ExtractionVersion(in.Format), version: raw.Version, identity: raw.SourceID, mainPart: raw.MainPart, evidence: evidence, blocks: blocks, inventory: raw})
}

type extracted[T any] struct {
	component                   string
	version, identity, mainPart string
	evidence                    domain.EvidenceDocument
	blocks                      []T
	inventory                   any
}

func stageExtraction[T any](ctx context.Context, workDir string, in ports.DocumentInput, out extracted[T]) (ports.StageOutput, error) {
	job, err := os.MkdirTemp(workDir, "extraction-")
	if err != nil {
		return ports.StageOutput{}, err
	}
	stage := &extractionFiles{ctx: ctx, root: job, allowed: map[string]bool{}, plan: domain.ArtifactPlan{SourceID: in.OriginalID, Role: in.Role, Stage: "extraction", ComponentVersion: out.component}}
	if err := writeExtraction(stage, out); err != nil {
		_ = stage.close()
		return ports.StageOutput{}, err
	}
	return ports.StageOutput{Plan: stage.plan, Open: stage.open, Close: stage.close}, nil
}

func extractionFailure(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, word.ErrLimit):
		return worker.Failure{Code: "SOURCE_TOO_LARGE"}
	case errors.Is(err, word.ErrActiveContent), errors.Is(err, word.ErrLegacyOrLocked):
		return worker.Failure{Code: "SOURCE_UNSUPPORTED"}
	default:
		return worker.Failure{Code: "SOURCE_INVALID"}
	}
}

type extractionFiles struct {
	ctx     context.Context
	root    string
	plan    domain.ArtifactPlan
	allowed map[string]bool
	bytes   int64
}

func writeExtraction[T any](s *extractionFiles, out extracted[T]) error {
	if len(out.blocks) > 20000 {
		return domain.ErrTooLarge
	}
	manifest := domain.ExtractionManifest{Version: out.version, Identity: out.identity, MainPart: out.mainPart, BlockCount: len(out.blocks), EvidenceFile: "evidence.json", InventoryFile: "inventory.json", Chunks: []domain.BlockChunk{}}
	if err := s.add(manifest.EvidenceFile, out.evidence); err != nil {
		return err
	}
	for start := 0; start < len(out.blocks); start += 100 {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		end := min(start+100, len(out.blocks))
		name := fmt.Sprintf("blocks-%04d.json", len(manifest.Chunks)+1)
		if err := s.add(name, out.blocks[start:end]); err != nil {
			return err
		}
		manifest.Chunks = append(manifest.Chunks, domain.BlockChunk{Name: name, First: start, Count: end - start})
	}
	if err := s.add(manifest.InventoryFile, out.inventory); err != nil {
		return err
	}
	encoded, err := json.Marshal(manifest)
	s.plan.Manifest = encoded
	return err
}

func (s *extractionFiles) add(name string, value any) error {
	f, err := os.OpenFile(filepath.Join(s.root, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	sink := &artifactSink{ctx: s.ctx, writer: io.MultiWriter(f, hash)}
	writeErr := json.NewEncoder(sink).Encode(value)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	s.bytes += sink.bytes
	if s.bytes > domain.MaxArtifactSetBytes {
		return domain.ErrTooLarge
	}
	s.plan.Files = append(s.plan.Files, domain.ArtifactSpec{Name: name, Kind: "source_blocks", ContentType: "application/json", Bytes: sink.bytes, SHA256: hash.Sum(nil)})
	s.allowed[name] = true
	return nil
}

func (s *extractionFiles) open(name string) (io.ReadSeekCloser, error) {
	if !s.allowed[name] {
		return nil, domain.ErrNotFound
	}
	return os.Open(filepath.Join(s.root, name))
}
func (s *extractionFiles) close() error { return os.RemoveAll(s.root) }

type artifactSink struct {
	ctx    context.Context
	writer io.Writer
	bytes  int64
}

func (w *artifactSink) Write(data []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(data)) > domain.MaxArtifactBytes-w.bytes {
		return 0, domain.ErrTooLarge
	}
	n, err := w.writer.Write(data)
	w.bytes += int64(n)
	return n, err
}
