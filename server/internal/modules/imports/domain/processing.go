package domain

// ProcessingResult is the private run envelope; full source evidence and candidates live in separately authorized artifact sets.
type ProcessingResult struct {
	Version        string            `json:"version"`
	CandidateSetID string            `json:"candidateSetId"`
	Sources        []ProcessedSource `json:"sources"`
}

// ProcessedSource maps source coordinates to an original and its complete normalization/extraction lineage.
type ProcessedSource struct {
	SourceID           string `json:"sourceId"`
	Identity           string `json:"identity"`
	Role               string `json:"role"`
	NormalizationSetID string `json:"normalizationSetId"`
	ExtractionSetID    string `json:"extractionSetId"`
}

type BlockChunk struct {
	Name  string `json:"name"`
	First int    `json:"first"`
	Count int    `json:"count"`
}

// ExtractionManifest indexes bounded source-block files without embedding source contents in a queue row.
type ExtractionManifest struct {
	Version       string       `json:"version"`
	Identity      string       `json:"identity"`
	MainPart      string       `json:"mainPart"`
	BlockCount    int          `json:"blockCount"`
	EvidenceFile  string       `json:"evidenceFile"`
	InventoryFile string       `json:"inventoryFile"`
	Chunks        []BlockChunk `json:"chunks"`
}
