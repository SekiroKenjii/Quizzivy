package adapters_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/imports/application/ports"
	"quizzivy/internal/modules/imports/domain"
	"quizzivy/internal/platform/word"
)

func TestExtractionStageChunksAllSourceEvidenceAndVerifiesFileIdentities(t *testing.T) {
	data := evidenceWord(t, strings.Repeat(`<w:p><w:r><w:t>Vietnamese nghé</w:t></w:r></w:p>`, 205))
	ctx := context.Background()
	raw, err := word.Extract(ctx, bytes.NewReader(data), int64(len(data)), "normalized-identity", word.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	engine := adapters.ImportProcessing{WorkDir: root}
	out, err := engine.Extract(ctx, ports.DocumentInput{OriginalID: "original", Identity: raw.SourceID, Role: "exam", Format: "docx", Body: bytes.NewReader(data), Bytes: int64(len(data))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = out.Close() })
	var manifest domain.ExtractionManifest
	if err := json.Unmarshal(out.Plan.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Identity != raw.SourceID || manifest.BlockCount != len(raw.Blocks) || len(manifest.Chunks) < 3 || out.Plan.SourceID != "original" {
		t.Fatal("lost original/normalized lineage or chunk inventory")
	}
	files := map[string][]byte{}
	for _, spec := range out.Plan.Files {
		file, err := out.Open(spec.Name)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(body)
		if int64(len(body)) != spec.Bytes || !bytes.Equal(digest[:], spec.SHA256) {
			t.Fatal("reservation does not identify output bytes")
		}
		files[spec.Name] = body
	}
	var blocks []word.SourceBlock
	for _, chunk := range manifest.Chunks {
		var part []word.SourceBlock
		if err := json.Unmarshal(files[chunk.Name], &part); err != nil {
			t.Fatal(err)
		}
		if chunk.First != len(blocks) || chunk.Count != len(part) || chunk.Count > 100 {
			t.Fatal("noncontiguous source chunk")
		}
		blocks = append(blocks, part...)
	}
	if !reflect.DeepEqual(blocks, raw.Blocks) {
		t.Fatal("chunking changed source evidence")
	}
	var inventory word.Extraction
	if err := json.Unmarshal(files[manifest.InventoryFile], &inventory); err != nil || len(inventory.Blocks) != 0 || inventory.SourceID != raw.SourceID {
		t.Fatal("invalid inventory")
	}
	var evidence domain.EvidenceDocument
	if err := json.Unmarshal(files[manifest.EvidenceFile], &evidence); err != nil || evidence.SourceID != raw.SourceID || len(evidence.Blocks) != len(blocks) {
		t.Fatal("lost projected evidence")
	}
	if _, err := out.Open("../inventory.json"); err == nil {
		t.Fatal("opened unlisted artifact")
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("extraction stage leaked working files")
	}
}
