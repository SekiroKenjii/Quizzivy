package adapters

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/application/worker"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/pdftext"
	"quizzivy/internal/platform/word"
	"quizzivy/internal/platform/wordconvert"
)

const conversionFailed = "CONVERSION_FAILED"

// ImportProcessing bridges the isolated converter and the native Word and PDF
// extractors to durable worker stages. Without PDF, PDF sources fail as unsupported.
type ImportProcessing struct {
	Converter *wordconvert.Converter
	PDF       *pdftext.Reader
	ImageID   string
	WorkDir   string
}

func (p ImportProcessing) Converts() bool { return p.Converter != nil }

func (p ImportProcessing) NormalizationVersion() string { return "rendition-v1:" + p.ImageID }
func (p ImportProcessing) ExtractionVersion(format string) string {
	if format == pdfFormat {
		return pdftext.Version + ":projection-v1"
	}
	return word.ExtractionVersion + ":projection-v1"
}

func (p ImportProcessing) Normalize(ctx context.Context, in ports.DocumentInput) (ports.StageOutput, error) {
	if p.Converter == nil {
		return ports.StageOutput{}, worker.Failure{Code: conversionFailed}
	}
	result, err := p.Converter.Convert(ctx, in.Body, in.Format)
	if err != nil {
		return ports.StageOutput{}, conversionFailure(err)
	}
	if result.ImageID != p.ImageID {
		_ = result.Close()
		return ports.StageOutput{}, worker.Failure{Code: conversionFailed}
	}
	manifest, err := json.Marshal(struct {
		wordconvert.Manifest
		ImageID      string `json:"imageId"`
		SourceSHA256 string `json:"sourceSha256"`
	}{result.Manifest, result.ImageID, result.SourceSHA256})
	if err != nil {
		_ = result.Close()
		return ports.StageOutput{}, err
	}
	plan := domain.ArtifactPlan{SourceID: in.OriginalID, Role: in.Role, Stage: "normalization", ComponentVersion: p.NormalizationVersion(), Manifest: manifest, Files: []domain.ArtifactSpec{}}
	for _, f := range result.Artifacts {
		kind, mime := renditionType(f.Kind)
		digest, err := hex.DecodeString(f.SHA256)
		if err != nil || kind == "" {
			_ = result.Close()
			return ports.StageOutput{}, domain.ErrInvalid
		}
		plan.Files = append(plan.Files, domain.ArtifactSpec{Name: f.Name, Kind: kind, ContentType: mime, Bytes: f.Bytes, SHA256: digest})
	}
	return ports.StageOutput{Plan: plan, Open: result.Open, Close: result.Close}, nil
}

func renditionType(kind string) (string, string) {
	switch kind {
	case "normalized":
		return "normalized_docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "rendition_pdf":
		return "source_pdf", "application/pdf"
	case "rendition_page":
		return "source_page", "image/png"
	default:
		return "", ""
	}
}

func conversionFailure(err error) error {
	switch {
	case errors.Is(err, wordconvert.ErrBusy):
		return worker.Failure{Code: "CONVERSION_BUSY", Retryable: true}
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, wordconvert.ErrTimeout):
		return worker.Failure{Code: "CONVERSION_TIMEOUT"}
	case errors.Is(err, wordconvert.ErrLimit):
		return worker.Failure{Code: "SOURCE_TOO_LARGE"}
	case errors.Is(err, wordconvert.ErrSource):
		return worker.Failure{Code: "SOURCE_INVALID"}
	case errors.Is(err, wordconvert.ErrCleanup):
		return worker.Failure{Code: "CONVERSION_CLEANUP_FAILED"}
	default:
		return worker.Failure{Code: conversionFailed}
	}
}
