package domain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"regexp"
	"time"
)

const MaxArtifactBytes int64 = 64 << 20
const MaxArtifactSetBytes int64 = 256 << 20

// ArtifactSpec describes immutable private bytes, never an authorized learner asset.
type ArtifactSpec struct {
	Name, Kind, ContentType string
	Bytes                   int64
	SHA256                  []byte
}

type Artifact struct {
	ArtifactSpec
	ID, SetID, ImportID, StorageKey string
	Ready                           bool
}

// ArtifactPlan pins a component's complete output list and lineage to one original source; ComponentVersion must identify all relevant processor configuration and upstream artifacts.
type ArtifactPlan struct {
	SourceID, Role, Stage, ComponentVersion string
	Manifest                                json.RawMessage
	Files                                   []ArtifactSpec
}

type ArtifactSet struct {
	ID, ImportID, RunID                     string
	ClaimToken, SourceRevision              int64
	SourceID, Role, Stage, ComponentVersion string
	Manifest                                json.RawMessage
	PlanDigest                              []byte
	Bytes                                   int64
	FileCount                               int
	Ready                                   bool
	CreatedAt                               time.Time
	CompletedAt                             *time.Time
	Files                                   []Artifact
}

// ArtifactQuotas bound retained pending and ready evidence independently of original-source quotas.
type ArtifactQuotas struct {
	ActorBytes, GlobalBytes int64
	SetsPerImport           int
}

// Artifacts journals every object before storage and exposes completed sets only after all files are acknowledged by the current claim.
type Artifacts interface {
	ReserveArtifacts(context.Context, Claim, ArtifactPlan, ArtifactQuotas) (ArtifactSet, error)
	ArtifactStored(context.Context, Claim, string, string) error
	FinishArtifacts(context.Context, Claim, string) (ArtifactSet, error)
	ReusableArtifacts(context.Context, Claim, string, string, string) (ArtifactSet, error)
	ArtifactSet(context.Context, string, string) (ArtifactSet, error)
}

var artifactName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,119}$`)

const artifactJSON = "application/json"

var artifactTypes = map[string]string{
	"normalized_docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"source_pdf":      "application/pdf", "source_page": "image/png", "source_image": "image/png",
	"source_blocks": artifactJSON, "candidate": artifactJSON, "validation": artifactJSON,
}

// Digest validates a bounded output plan and hashes its canonical manifest and ordered file identities.
func (p ArtifactPlan) Digest() ([]byte, int64, error) {
	if p.SourceID == "" || (p.Role != "exam" && p.Role != "answer_key") || len(p.ComponentVersion) == 0 || len(p.ComponentVersion) > 200 || len(p.Manifest) > 60<<10 || len(p.Files) == 0 || len(p.Files) > 512 {
		return nil, 0, ErrInvalid
	}
	switch p.Stage {
	case "normalization", "extraction", "recognition", "validation":
	default:
		return nil, 0, ErrInvalid
	}
	var manifest map[string]any
	decoder := json.NewDecoder(bytes.NewReader(p.Manifest))
	decoder.UseNumber()
	if err := decoder.Decode(&manifest); err != nil || manifest == nil || !json.Valid(p.Manifest) {
		return nil, 0, ErrInvalid
	}
	p.Manifest, _ = json.Marshal(manifest)
	var total int64
	names := make(map[string]bool, len(p.Files))
	for _, f := range p.Files {
		if !artifactName.MatchString(f.Name) || names[f.Name] || artifactTypes[f.Kind] == "" || artifactTypes[f.Kind] != f.ContentType || f.Bytes <= 0 || f.Bytes > MaxArtifactBytes || len(f.SHA256) != sha256.Size {
			return nil, 0, ErrInvalid
		}
		names[f.Name] = true
		total += f.Bytes
	}
	if total > MaxArtifactSetBytes {
		return nil, 0, ErrTooLarge
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, 0, ErrInvalid
	}
	digest := sha256.Sum256(data)
	return digest[:], total, nil
}
