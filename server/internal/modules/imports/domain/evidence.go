package domain

// EvidenceDocument is a bounded recognition projection; the original extraction artifact retains all raw XML evidence.
type EvidenceDocument struct {
	SourceID string          `json:"sourceId"`
	Role     string          `json:"role"`
	Version  string          `json:"version"`
	Findings []string        `json:"findings"`
	Blocks   []EvidenceBlock `json:"blocks"`
}

// EvidenceBlock preserves source offsets even when a private or ambiguous fragment cannot enter learner content.
type EvidenceBlock struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Main       bool           `json:"main"`
	Meaningful bool           `json:"meaningful"`
	Safe       bool           `json:"safe"`
	Text       string         `json:"text"`
	Numbering  string         `json:"numbering,omitempty"`
	Spans      []EvidenceSpan `json:"spans"`
	Reasons    []string       `json:"reasons"`
	TableID    string         `json:"tableId,omitempty"`
	Row        int            `json:"row,omitempty"`
	Column     int            `json:"column,omitempty"`
}

type EvidenceSpan struct {
	Start int      `json:"start"`
	End   int      `json:"end"`
	Marks []string `json:"marks"`
}

// SourceRef names Unicode-code-point offsets in immutable extracted text; generated labels are separate evidence.
type SourceRef struct {
	SourceID       string `json:"sourceId"`
	BlockID        string `json:"blockId"`
	Start          int    `json:"start"`
	End            int    `json:"end"`
	GeneratedLabel bool   `json:"generatedLabel,omitempty"`
}
