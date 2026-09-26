package worker

import (
	"context"
	"encoding/json"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/domain"
)

// EvidenceReader reads what a succeeded run extracted, for the teacher's side-by-side source view.
type EvidenceReader struct {
	Artifacts domain.Artifacts
	Store     ports.ArtifactStore
	WorkDir   string
}

// Read returns the extracted evidence of the source with role in run; ErrNotFound when that role was not processed.
func (r EvidenceReader) Read(ctx context.Context, run domain.Run, role string) (domain.EvidenceDocument, error) {
	var result domain.ProcessingResult
	if err := json.Unmarshal(run.Result, &result); err != nil {
		return domain.EvidenceDocument{}, domain.ErrInvalidResult
	}
	for _, source := range result.Sources {
		if source.Role != role {
			continue
		}
		set, err := r.Artifacts.ArtifactSet(ctx, run.ImportID, source.ExtractionSetID)
		if err != nil {
			return domain.EvidenceDocument{}, err
		}
		p := Pipeline{Store: r.Store, WorkDir: r.WorkDir}
		return p.evidence(ctx, pipelineSource{source: domain.Source{ID: source.SourceID, Role: role}, extraction: set, identity: source.Identity})
	}
	return domain.EvidenceDocument{}, domain.ErrNotFound
}
