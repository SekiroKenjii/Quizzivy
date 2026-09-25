package word

import (
	"context"
	"fmt"
	"strings"
)

const (
	styleParagraph    = "paragraph"
	styleCharacter    = "character"
	propertyRun       = "rPr"
	propertyParagraph = "pPr"
	propertyNumbering = "numPr"
	propertyNumberID  = "numId"
	propertyLevel     = "ilvl"
	propertyValue     = "val"
	elementTab        = "tab"
	elementTextBox    = "txbxContent"
	propertyVertical  = "vertAlign"
)

// Resolution adds conservative semantic-mark and list-label evidence to an
// Inspection. It is private, is not a renderer, and does not replace source findings.
type Resolution struct {
	Paragraphs []ResolvedParagraph `json:"paragraphs"`
	Findings   []Finding           `json:"findings"`
}

// ResolvedParagraph keeps automatic labels separate from the original run text.
type ResolvedParagraph struct {
	Locator
	Numbering *NumberingLabel `json:"numbering,omitempty"`
	Runs      []ResolvedRun   `json:"runs"`
}

// NumberingLabel is a generated label only when Resolved is true; sources identify
// the instance and level definitions, while the paragraph locator binds its order.
type NumberingLabel struct {
	ID       string    `json:"id"`
	Level    int       `json:"level"`
	Text     string    `json:"text"`
	Suffix   string    `json:"suffix"`
	Resolved bool      `json:"resolved"`
	Sources  []Locator `json:"sources,omitempty"`
}

// ResolvedRun reports only marks supported by the resolver; an absent mark is
// unspecified, and incomplete style/context resolution never asserts a value.
type ResolvedRun struct {
	Locator
	Complete bool           `json:"complete"`
	Marks    []ResolvedMark `json:"marks"`
}

// ResolvedMark records a named source property and its inheritance evidence.
// An unresolved mark has an empty Value, rather than a guessed on/off state.
type ResolvedMark struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Resolved bool      `json:"resolved"`
	Direct   bool      `json:"direct,omitempty"`
	Sources  []Locator `json:"sources"`
}

type resolver struct {
	ctx          context.Context
	source       Inspection
	out          Resolution
	parts        map[string]Part
	styles       styleIndex
	numbering    numberingIndex
	remaining    int
	locatorBytes int
}

// Resolve derives bounded source evidence from an unmodified Inspection produced
// by Inspect. Unsupported styles, numbering and ambiguous revisions remain findings;
// no source text is rewritten, accepted as an answer, or made learner-safe.
func Resolve(ctx context.Context, source Inspection) (Resolution, error) {
	r := resolver{ctx: ctx, source: source, remaining: 2000000, locatorBytes: 32 << 20, parts: map[string]Part{},
		out: Resolution{Paragraphs: []ResolvedParagraph{}, Findings: []Finding{}}}
	for _, part := range source.Parts {
		if err := r.spend(1); err != nil {
			return Resolution{}, err
		}
		r.parts[part.Name] = part
	}
	if err := r.initStyles(); err != nil {
		return Resolution{}, err
	}
	if err := r.initNumbering(); err != nil {
		return Resolution{}, err
	}
	for _, part := range source.Parts {
		if err := r.resolvePart(part); err != nil {
			return Resolution{}, err
		}
	}
	if err := r.spend(0); err != nil {
		return Resolution{}, err
	}
	return r.out, nil
}

func (r *resolver) spend(n int) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	r.remaining -= n
	if r.locatorBytes < 0 {
		return fmt.Errorf("%w: resolved source locators", ErrLimit)
	}
	if r.remaining < 0 {
		return fmt.Errorf("%w: source resolution work", ErrLimit)
	}
	return nil
}

func (r *resolver) finding(code string, loc Locator) {
	r.out.Findings = append(r.out.Findings, Finding{Code: code, Locator: loc})
	r.locatorBytes -= len(loc.Part) + len(loc.Path)
}

func (r *resolver) retainLocations(locations []Locator) error {
	for _, loc := range locations {
		r.locatorBytes -= len(loc.Part) + len(loc.Path)
	}
	if r.locatorBytes < 0 {
		return fmt.Errorf("%w: resolved source locators", ErrLimit)
	}
	return nil
}

func (r *resolver) linkedPart(kind string) (Part, bool) {
	var target string
	for _, rel := range r.source.Relationships {
		if rel.Source != r.source.MainPart || !officeRelationship(rel.Type, kind) {
			continue
		}
		if target != "" || rel.External {
			r.finding("AMBIGUOUS_"+strings.ToUpper(kind)+"_RELATIONSHIP", Locator{Part: rel.Source, Path: rel.ID})
			return Part{}, false
		}
		target = rel.Target
	}
	part, found := r.parts[target]
	if target != "" && (!found || part.Kind != kind) {
		r.finding("INVALID_"+strings.ToUpper(kind)+"_PART", Locator{Part: target})
		return Part{}, false
	}
	return part, true
}

func officeRelationship(value, kind string) bool {
	return value == "http://schemas.openxmlformats.org/officeDocument/2006/relationships/"+kind || value == "http://purl.oclc.org/ooxml/officeDocument/relationships/"+kind
}

func wordName(name, local string) bool {
	return name == "{"+wordNamespace+"}"+local || name == "{"+strictWordNamespace+"}"+local
}

func prop(props []Property, local string) Property {
	for _, p := range props {
		if wordName(p.Name, local) {
			return p
		}
	}
	return Property{}
}

func attr(p Property, local string) string {
	if value, ok := p.Attributes["{"+wordNamespace+"}"+local]; ok {
		return value
	}
	return p.Attributes["{"+strictWordNamespace+"}"+local]
}

func val(props []Property, local string) string { return attr(prop(props, local), propertyValue) }

func onOff(value string) (bool, bool) {
	switch value {
	case "", "1", "true", "on":
		return true, true
	case "0", "false", "off":
		return false, true
	default:
		return false, false
	}
}

func (r *resolver) resolvePart(part Part) error {
	structures := make(map[Locator]Structure, len(part.Structures))
	for _, structure := range part.Structures {
		structures[structure.Locator] = structure
	}
	for _, p := range part.Paragraphs {
		if err := r.resolveParagraph(p, structures); err != nil {
			return err
		}
	}
	return nil
}

func (r *resolver) resolveParagraph(p Paragraph, structures map[Locator]Structure) error {
	if err := r.spend(1 + len(p.Properties)); err != nil {
		return err
	}
	styleID := val(p.Properties, "pStyle")
	if styleID == "" {
		styleID = r.styles.defaultParagraph
	}
	style, err := r.style(styleID, styleParagraph)
	if err != nil {
		return err
	}
	style.complete = style.complete && !duplicateProperties(p.Properties)
	resolved := ResolvedParagraph{Locator: p.Locator, Runs: []ResolvedRun{}}
	if err := r.retainLocations([]Locator{p.Locator}); err != nil {
		return err
	}
	resolved.Numbering, err = r.resolveNumbering(p, style, structures)
	if err != nil {
		return err
	}
	for _, run := range p.Runs {
		if err := r.retainLocations([]Locator{run.Locator}); err != nil {
			return err
		}
		marks, err := r.resolveRun(p, run, style, structures)
		if err != nil {
			return err
		}
		resolved.Runs = append(resolved.Runs, marks)
	}
	r.out.Paragraphs = append(r.out.Paragraphs, resolved)
	return nil
}
