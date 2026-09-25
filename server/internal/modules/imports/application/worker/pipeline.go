package worker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
)

// PipelineVersion pins the default deterministic workflow; configuration-specific output versions also fence stage reuse.
const PipelineVersion = "word-pipeline-v1"

// Pipeline joins private source reads, durable stages and conservative recognition; only Runner may complete its live claim.
type Pipeline struct {
	Sources   ports.ProcessingSources
	Artifacts domain.Artifacts
	Store     ports.ArtifactStore
	Engine    ports.ProcessingEngine
	Quotas    domain.ArtifactQuotas
	WorkDir   string
}

type pipelineSource struct {
	source                    domain.Source
	normalization, extraction domain.ArtifactSet
	identity                  string
}

func (p Pipeline) Process(ctx context.Context, run domain.Run, progress func(string) error) (domain.Outcome, error) {
	result, err := p.process(ctx, run, progress)
	return result, pipelineFailure(err)
}

func (p Pipeline) process(ctx context.Context, run domain.Run, progress func(string) error) (domain.Outcome, error) {
	if run.PipelineVersion != PipelineVersion {
		return domain.Outcome{}, domain.ErrInvalidResult
	}
	sources, err := p.Sources.Sources(ctx, run.ImportID, run.SourceRevision)
	if err != nil {
		return domain.Outcome{}, err
	}
	if len(sources) < 1 || len(sources) > 2 {
		return domain.Outcome{}, domain.ErrInvalid
	}
	if err := progress("normalization"); err != nil {
		return domain.Outcome{}, err
	}
	prepared, err := p.normalizeSources(ctx, run.Claim(), sources)
	if err != nil {
		return domain.Outcome{}, err
	}
	if err := progress("extraction"); err != nil {
		return domain.Outcome{}, err
	}
	documents, err := p.extractSources(ctx, run.Claim(), prepared)
	if err != nil {
		return domain.Outcome{}, err
	}
	if err := progress("recognition"); err != nil {
		return domain.Outcome{}, err
	}
	candidate, err := recognition.Recognize(ctx, documents, run.Profile)
	if err != nil {
		return domain.Outcome{}, err
	}
	if err := addRenditionFindings(&candidate, prepared); err != nil {
		return domain.Outcome{}, err
	}
	if err := progress("validation"); err != nil {
		return domain.Outcome{}, err
	}
	return p.saveCandidate(ctx, run.Claim(), candidate, prepared)
}

func (p Pipeline) normalizeSources(ctx context.Context, claim domain.Claim, sources []domain.Source) ([]pipelineSource, error) {
	prepared := make([]pipelineSource, 0, len(sources))
	for _, source := range sources {
		if !p.Engine.Converts() {
			if source.Format != "docx" {
				return nil, Failure{Code: "LEGACY_CONVERSION_UNAVAILABLE"}
			}
			prepared = append(prepared, pipelineSource{source: source, identity: source.ID})
			continue
		}
		set, err := p.normalize(ctx, claim, source)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, pipelineSource{source: source, normalization: set, identity: source.ID})
	}
	return prepared, nil
}

func (p Pipeline) extractSources(ctx context.Context, claim domain.Claim, prepared []pipelineSource) ([]domain.EvidenceDocument, error) {
	documents := make([]domain.EvidenceDocument, 0, len(prepared))
	for i := range prepared {
		s := &prepared[i]
		if err := p.extract(ctx, claim, s); err != nil {
			return nil, err
		}
		evidence, err := p.evidence(ctx, *s)
		if err != nil {
			return nil, err
		}
		documents = append(documents, evidence)
	}
	return documents, nil
}

func (p Pipeline) normalize(ctx context.Context, c domain.Claim, s domain.Source) (domain.ArtifactSet, error) {
	return p.stage(ctx, c, s, "normalization", p.Engine.NormalizationVersion(), func() (ports.StageOutput, error) {
		file, err := p.fetch(ctx, s.StorageKey, s.Bytes, s.SHA256)
		if err != nil {
			return ports.StageOutput{}, err
		}
		defer removeStaging(file)
		return p.Engine.Normalize(ctx, ports.DocumentInput{OriginalID: s.ID, Identity: s.ID, Role: s.Role, Format: s.Format, Body: file, Bytes: s.Bytes})
	})
}

func (p Pipeline) extract(ctx context.Context, c domain.Claim, s *pipelineSource) error {
	key, size, digest := s.source.StorageKey, s.source.Bytes, s.source.SHA256
	if s.source.Format == "doc" {
		file, err := artifactKind(s.normalization, "normalized_docx")
		if err != nil {
			return err
		}
		key, size, digest, s.identity = file.StorageKey, file.Bytes, file.SHA256, file.ID
	}
	version := componentIdentity(p.Engine.ExtractionVersion(), s.identity)
	set, err := p.stage(ctx, c, s.source, "extraction", version, func() (ports.StageOutput, error) {
		file, err := p.fetch(ctx, key, size, digest)
		if err != nil {
			return ports.StageOutput{}, err
		}
		defer removeStaging(file)
		return p.Engine.Extract(ctx, ports.DocumentInput{OriginalID: s.source.ID, Identity: s.identity, Role: s.source.Role, Format: "docx", Body: file, Bytes: size})
	})
	s.extraction = set
	return err
}

func (p Pipeline) stage(ctx context.Context, c domain.Claim, source domain.Source, stage, version string, produce func() (ports.StageOutput, error)) (domain.ArtifactSet, error) {
	previous, err := p.Artifacts.ReusableArtifacts(ctx, c, source.Role, stage, version)
	if err == nil {
		return previous, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.ArtifactSet{}, err
	}
	output, err := produce()
	if err != nil {
		return domain.ArtifactSet{}, err
	}
	if output.Close == nil || output.Open == nil {
		return domain.ArtifactSet{}, domain.ErrInvalidResult
	}
	if output.Plan.SourceID != source.ID || output.Plan.Role != source.Role || output.Plan.Stage != stage {
		_ = output.Close()
		return domain.ArtifactSet{}, domain.ErrInvalidResult
	}
	output.Plan.ComponentVersion = version
	writer := ArtifactWriter{Repo: p.Artifacts, Store: p.Store, Quotas: p.Quotas}
	set, writeErr := writer.Write(ctx, c, output.Plan, output.Open)
	closeErr := output.Close()
	if writeErr != nil {
		return set, writeErr
	}
	return set, closeErr
}

func (p Pipeline) evidence(ctx context.Context, s pipelineSource) (domain.EvidenceDocument, error) {
	var manifest domain.ExtractionManifest
	if err := json.Unmarshal(s.extraction.Manifest, &manifest); err != nil || manifest.Identity != s.identity {
		return domain.EvidenceDocument{}, domain.ErrInvalidResult
	}
	artifact, err := artifactName(s.extraction, manifest.EvidenceFile)
	if err != nil {
		return domain.EvidenceDocument{}, err
	}
	var evidence domain.EvidenceDocument
	if err := p.readJSON(ctx, artifact, &evidence); err != nil {
		return evidence, err
	}
	if evidence.SourceID != s.identity || evidence.Role != s.source.Role {
		return evidence, domain.ErrInvalidResult
	}
	return evidence, nil
}

func (p Pipeline) readJSON(ctx context.Context, a domain.Artifact, dest any) error {
	file, err := p.fetch(ctx, a.StorageKey, a.Bytes, a.SHA256)
	if err != nil {
		return err
	}
	defer removeStaging(file)
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return domain.ErrInvalidResult
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return domain.ErrInvalidResult
	}
	return nil
}

func (p Pipeline) fetch(ctx context.Context, key string, size int64, digest []byte) (*os.File, error) {
	file, err := FetchPrivate(ctx, p.Store, p.WorkDir, key, size, digest)
	if errors.Is(err, ErrPrivateIdentity) {
		return nil, Failure{Code: "STORAGE_INTEGRITY_FAILED"}
	}
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return nil, Failure{Code: "STORAGE_UNAVAILABLE", Retryable: true}
	}
	return file, err
}

func artifactName(set domain.ArtifactSet, name string) (domain.Artifact, error) {
	for _, f := range set.Files {
		if f.Name == name && f.Ready {
			return f, nil
		}
	}
	return domain.Artifact{}, domain.ErrInvalidResult
}
func artifactKind(set domain.ArtifactSet, kind string) (domain.Artifact, error) {
	var out domain.Artifact
	for _, f := range set.Files {
		if f.Kind == kind && f.Ready {
			if out.ID != "" {
				return out, domain.ErrInvalidResult
			}
			out = f
		}
	}
	if out.ID == "" {
		return out, domain.ErrInvalidResult
	}
	return out, nil
}
func removeStaging(file *os.File) { _ = file.Close(); _ = os.Remove(file.Name()) }
func componentIdentity(version, identity string) string {
	return fmt.Sprintf("%s:%x", version, sha256.Sum256([]byte(identity)))
}

func pipelineFailure(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrTooLarge):
		return Failure{Code: "SOURCE_TOO_LARGE"}
	case errors.Is(err, domain.ErrUnsupported):
		return Failure{Code: "SOURCE_UNSUPPORTED"}
	case errors.Is(err, domain.ErrInvalid):
		return Failure{Code: "SOURCE_INVALID"}
	case errors.Is(err, domain.ErrInvalidResult):
		return Failure{Code: processorOutputInvalid}
	case errors.Is(err, domain.ErrQuota):
		return Failure{Code: "STORAGE_QUOTA_EXCEEDED"}
	default:
		return err
	}
}
