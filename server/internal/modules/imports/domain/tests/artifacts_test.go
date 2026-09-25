package domain_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"quizzivy/internal/modules/imports/domain"
	"testing"
)

func plan() domain.ArtifactPlan {
	hash := sha256.Sum256([]byte("evidence"))
	return domain.ArtifactPlan{SourceID: "source", Role: "exam", Stage: "extraction", ComponentVersion: "extract-v1", Manifest: json.RawMessage(`{"b":{"z":1,"a":9007199254740993},"a":2}`), Files: []domain.ArtifactSpec{{Name: "blocks.json", Kind: "source_blocks", ContentType: "application/json", Bytes: 8, SHA256: hash[:]}}}
}
func TestArtifactIdentityPreservesExactNumbersAndCanonicalizesNestedManifest(t *testing.T) {
	p := plan()
	first, _, err := p.Digest()
	if err != nil {
		t.Fatal(err)
	}
	p.Manifest = json.RawMessage(`{ "a": 2, "b": {"a":9007199254740993,"z":1}}`)
	second, _, err := p.Digest()
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("equivalent manifest changed identity")
	}
	p.Manifest = json.RawMessage(`{"a":2,"b":{"a":9007199254740992,"z":1}}`)
	third, _, err := p.Digest()
	if err != nil || bytes.Equal(first, third) {
		t.Fatal("manifest integer rounded in digest")
	}
}
func TestArtifactPlanRejectsPathsUnknownTypesAndUnboundedEvidence(t *testing.T) {
	for name, change := range map[string]func(*domain.ArtifactPlan){
		"path":              func(p *domain.ArtifactPlan) { p.Files[0].Name = "../private" },
		"duplicate":         func(p *domain.ArtifactPlan) { p.Files = append(p.Files, p.Files[0]) },
		"mime":              func(p *domain.ArtifactPlan) { p.Files[0].ContentType = "text/html" },
		"digest":            func(p *domain.ArtifactPlan) { p.Files[0].SHA256 = []byte{1} },
		"size":              func(p *domain.ArtifactPlan) { p.Files[0].Bytes = domain.MaxArtifactBytes + 1 },
		"array manifest":    func(p *domain.ArtifactPlan) { p.Manifest = json.RawMessage(`[]`) },
		"trailing manifest": func(p *domain.ArtifactPlan) { p.Manifest = json.RawMessage(`{} {}`) },
		"unknown stage":     func(p *domain.ArtifactPlan) { p.Stage = "arbitrary" },
	} {
		t.Run(name, func(t *testing.T) {
			p := plan()
			change(&p)
			if _, _, err := p.Digest(); err == nil {
				t.Fatal("invalid artifact plan accepted")
			}
		})
	}
}
