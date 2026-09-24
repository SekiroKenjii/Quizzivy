package word

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"slices"
	"strings"
)

// ExtractionVersion identifies the private source-block schema and ordering policy.
const ExtractionVersion = "ooxml-blocks-v1"

// Extraction is ordered private evidence bound to one immutable source identity; it is never a learner payload.
type Extraction struct {
	Version       string         `json:"version"`
	SourceID      string         `json:"sourceId"`
	MainPart      string         `json:"mainPart"`
	Blocks        []SourceBlock  `json:"blocks"`
	Assets        []Asset        `json:"assets"`
	Relationships []Relationship `json:"relationships"`
	Findings      []Finding      `json:"findings"`
}

// SourceBlock retains document order, structural ancestry and unresolved evidence without choosing a question or answer interpretation.
type SourceBlock struct {
	Locator
	SourceRange
	ID            string            `json:"id"`
	ParentID      string            `json:"parentId,omitempty"`
	Kind          string            `json:"kind"`
	PartKind      string            `json:"partKind"`
	Meaningful    bool              `json:"meaningful"`
	ReviewReasons []string          `json:"reviewReasons,omitempty"`
	Properties    []Property        `json:"properties,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	Paragraph     *TextEvidence     `json:"paragraph,omitempty"`
	Object        *Property         `json:"object,omitempty"`
	Unassigned    *Fragment         `json:"unassigned,omitempty"`
	Cell          *CellEvidence     `json:"cell,omitempty"`
}

// TextEvidence keeps numbering outside source text and uses Unicode code point offsets across the original fragments, including private/deleted text.
type TextEvidence struct {
	Numbering *NumberingLabel `json:"numbering,omitempty"`
	Runs      []RunEvidence   `json:"runs"`
}

// RunEvidence retains properties, supported inherited marks and precisely located text fragments; Complete is not a learner-safety assertion.
type RunEvidence struct {
	Locator
	Contexts      []Locator      `json:"contexts,omitempty"`
	Complete      bool           `json:"complete"`
	Marks         []ResolvedMark `json:"marks"`
	Properties    []Property     `json:"properties,omitempty"`
	ReviewReasons []string       `json:"reviewReasons,omitempty"`
	Fragments     []TextFragment `json:"fragments"`
}

// TextFragment is a source text range; field instructions and deleted text remain distinguishable from ordinary text.
type TextFragment struct {
	Fragment
	Start     int `json:"start"`
	EndOffset int `json:"endOffset"`
}

// CellEvidence identifies a cell's table grid position; unresolved merges retain their raw properties and require review.
type CellEvidence struct {
	TableID       string `json:"tableId"`
	Row           int    `json:"row"`
	Column        int    `json:"column"`
	ColumnSpan    int    `json:"columnSpan"`
	VerticalMerge string `json:"verticalMerge,omitempty"`
	MergeOriginID string `json:"mergeOriginId,omitempty"`
	Resolved      bool   `json:"resolved"`
}

// Extract inventories and resolves one immutable native DOCX, retaining all source branches for later coverage and teacher review.
func Extract(ctx context.Context, src io.ReaderAt, size int64, sourceID string, limits Limits) (Extraction, error) {
	if sourceID == "" || len(sourceID) > 128 || strings.ContainsRune(sourceID, 0) {
		return Extraction{}, fmt.Errorf("%w: source identity", ErrInvalidPackage)
	}
	source, err := Inspect(ctx, src, size, limits)
	if err != nil {
		return Extraction{}, err
	}
	resolved, err := Resolve(ctx, source)
	if err != nil {
		return Extraction{}, err
	}
	return extractBlocks(ctx, sourceID, source, resolved)
}

func evidenceID(sourceID string, loc Locator) string {
	return fmt.Sprintf("b_%x", sha256.Sum256([]byte(sourceID+"\x00"+loc.Part+"\x00"+loc.Path)))
}

func extractBlocks(ctx context.Context, sourceID string, source Inspection, resolved Resolution) (Extraction, error) {
	out := Extraction{Version: ExtractionVersion, SourceID: sourceID, MainPart: source.MainPart, Blocks: []SourceBlock{}, Assets: source.Assets, Relationships: source.Relationships, Findings: slices.Concat(source.Findings, resolved.Findings)}
	paragraphs := make(map[Locator]ResolvedParagraph, len(resolved.Paragraphs))
	for _, p := range resolved.Paragraphs {
		paragraphs[p.Locator] = p
	}
	for _, part := range source.Parts {
		if err := ctx.Err(); err != nil {
			return Extraction{}, err
		}
		blocks := partBlocks(part, paragraphs)
		slices.SortFunc(blocks, func(a, b SourceBlock) int { return a.Order - b.Order })
		linkBlocks(sourceID, source.MainPart, part, blocks)
		markComplexFields(blocks)
		resolveCells(blocks)
		out.Blocks = append(out.Blocks, blocks...)
	}
	return out, ctx.Err()
}

func partBlocks(part Part, resolved map[Locator]ResolvedParagraph) []SourceBlock {
	blocks := make([]SourceBlock, 0, len(part.Structures)+len(part.Paragraphs)+len(part.Objects)+len(part.Unassigned)+1)
	indexed := make(map[Locator]int, len(part.Structures))
	contexts := make(map[Locator]string, len(part.Structures))
	for _, s := range part.Structures {
		contexts[s.Locator] = s.Kind
		indexed[s.Locator] = len(blocks)
		blocks = append(blocks, SourceBlock{Locator: s.Locator, SourceRange: s.SourceRange, Kind: s.Kind, Properties: s.Properties, Attributes: s.Attributes})
	}
	for _, p := range part.Paragraphs {
		text, meaningful := paragraphEvidence(p, resolved[p.Locator], contexts)
		blocks = append(blocks, SourceBlock{Locator: p.Locator, SourceRange: p.SourceRange, Kind: "paragraph", Properties: p.Properties, Paragraph: &text, Meaningful: meaningful})
	}
	for _, o := range part.Objects {
		if i, found := indexed[o.Locator]; found {
			blocks[i].Object = &o.Content
			blocks[i].Meaningful = true
		} else {
			blocks = append(blocks, SourceBlock{Locator: o.Locator, SourceRange: o.SourceRange, Kind: elementObject, Object: &o.Content, Meaningful: true})
		}
	}
	for _, f := range part.Unassigned {
		blocks = append(blocks, SourceBlock{Locator: f.Locator, SourceRange: f.SourceRange, Kind: "unassigned", Unassigned: &f, Meaningful: true})
	}
	if len(part.Properties) > 0 {
		blocks = append(blocks, SourceBlock{Locator: Locator{Part: part.Name, Path: ""}, Kind: "part_metadata", Properties: part.Properties, Meaningful: part.Kind == partUnknownXML})
	}
	return blocks
}

func linkBlocks(sourceID, mainPart string, part Part, blocks []SourceBlock) {
	stack := make([]int, 0, 16)
	for i := range blocks {
		b := &blocks[i]
		b.ID, b.PartKind = evidenceID(sourceID, b.Locator), part.Kind
		for len(stack) > 0 && blocks[stack[len(stack)-1]].End < b.Order {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			parent := &blocks[stack[len(stack)-1]]
			b.ParentID = parent.ID
			b.ReviewReasons = slices.Clone(parent.ReviewReasons)
		}
		b.ReviewReasons = appendReasons(b.ReviewReasons, blockReasons(*b)...)
		if part.Name != mainPart {
			b.ReviewReasons = appendReasons(b.ReviewReasons, "ANCILLARY_CONTENT_REQUIRES_REVIEW")
		}
		if b.End > b.Order {
			stack = append(stack, i)
		}
	}
}
