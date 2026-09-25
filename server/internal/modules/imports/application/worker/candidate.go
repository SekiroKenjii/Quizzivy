package worker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/google/uuid"
	"io"
	"os"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/modules/imports/domain/recognition"
	"strings"
)

const candidateFilename = "candidate.json"

func addRenditionFindings(draft *domain.Draft, sources []pipelineSource) error {
	for _, s := range sources {
		if s.normalization.ID == "" {
			continue
		}
		var manifest struct {
			Findings []string `json:"findings"`
		}
		if err := json.Unmarshal(s.normalization.Manifest, &manifest); err != nil {
			return domain.ErrInvalidResult
		}
		for _, code := range manifest.Findings {
			id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(s.identity+"/"+code)).String()
			draft.Notices = append(draft.Notices, domain.Finding{ID: id, Code: domain.CodeSourceObject, Severity: domain.ReviewRequired, Target: s.identity, Field: code, Count: 1, Evidence: []domain.SourceRef{{SourceID: s.identity}}})
		}
	}
	return nil
}

func (p Pipeline) saveCandidate(ctx context.Context, c domain.Claim, candidate domain.Draft, sources []pipelineSource) (domain.Outcome, error) {
	result := domain.ProcessingResult{Version: "word-run-result-v1", Sources: []domain.ProcessedSource{}}
	lineage := []string{}
	var exam domain.Source
	for _, s := range sources {
		result.Sources = append(result.Sources, domain.ProcessedSource{SourceID: s.source.ID, Identity: s.identity, Role: s.source.Role, NormalizationSetID: s.normalization.ID, ExtractionSetID: s.extraction.ID})
		lineage = append(lineage, s.normalization.ID, s.extraction.ID)
		if s.source.Role == "exam" {
			exam = s.source
		}
	}
	if exam.ID == "" {
		return domain.Outcome{}, domain.ErrInvalidResult
	}
	version := componentIdentity(recognition.Version, strings.Join(lineage, "/"))
	set, err := p.stage(ctx, c, exam, "recognition", version, func() (ports.StageOutput, error) { return p.candidateOutput(exam, candidate) })
	if err != nil {
		return domain.Outcome{}, err
	}
	result.CandidateSetID = set.ID
	envelope, err := json.Marshal(result)
	if err != nil {
		return domain.Outcome{}, err
	}
	draft, err := json.Marshal(candidate)
	return domain.Outcome{Result: envelope, Draft: draft}, err
}

func (p Pipeline) candidateOutput(source domain.Source, candidate domain.Draft) (ports.StageOutput, error) {
	raw, err := json.Marshal(candidate)
	if err != nil {
		return ports.StageOutput{}, err
	}
	if err := domain.ValidateRunResult(raw); err != nil {
		return ports.StageOutput{}, err
	}
	f, err := os.CreateTemp(p.WorkDir, "candidate-*")
	if err != nil {
		return ports.StageOutput{}, err
	}
	_, writeErr := f.Write(raw)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(f.Name())
		if writeErr != nil {
			return ports.StageOutput{}, writeErr
		}
		return ports.StageOutput{}, closeErr
	}
	digest := sha256.Sum256(raw)
	manifest, err := json.Marshal(map[string]string{"version": domain.DraftVersion, "candidateFile": candidateFilename})
	if err != nil {
		_ = os.Remove(f.Name())
		return ports.StageOutput{}, err
	}
	plan := domain.ArtifactPlan{SourceID: source.ID, Role: source.Role, Stage: "recognition", Manifest: manifest, Files: []domain.ArtifactSpec{{Name: candidateFilename, Kind: "candidate", ContentType: "application/json", Bytes: int64(len(raw)), SHA256: digest[:]}}}
	return ports.StageOutput{Plan: plan, Open: func(name string) (io.ReadSeekCloser, error) {
		if name != candidateFilename {
			return nil, domain.ErrNotFound
		}
		return os.Open(f.Name())
	}, Close: func() error { return os.Remove(f.Name()) }}, nil
}
