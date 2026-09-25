package word

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
)

const (
	partUnknownXML          = "unknown_xml"
	elementPicture          = "pict"
	partStyles              = "styles"
	elementDrawing          = "drawing"
	styleResolutionRequired = "STYLE_RESOLUTION_REQUIRED"
)

// Inspect inventories a DOCX without executing objects or resolving external
// relationships. Its output retains teacher-only source content and requires
// semantic extraction, asset validation and review before assessment use.
func Inspect(ctx context.Context, src io.ReaderAt, size int64, limits Limits) (Inspection, error) {
	a, err := openArchive(ctx, src, size, limits)
	if err != nil {
		return Inspection{}, err
	}
	budget := &xmlBudget{remaining: limits.XMLNodes, bytes: limits.XMLTotalBytes, locators: limits.LocatorBytes}
	types, err := a.contentTypes(ctx, budget)
	if err != nil {
		return Inspection{}, err
	}
	rels, err := a.relationships(ctx, budget)
	if err != nil {
		return Inspection{}, err
	}
	main, err := mainPart(rels, types)
	if err != nil {
		return Inspection{}, err
	}
	out := Inspection{MainPart: main, Parts: []Part{}, Relationships: rels, Assets: []Asset{}, Findings: []Finding{}}
	for _, rel := range rels {
		if rel.External {
			out.Findings = append(out.Findings, Finding{Code: "EXTERNAL_RELATIONSHIP", Locator: Locator{Part: rel.Source, Path: rel.ID}})
		}
	}
	if err := a.inspectParts(ctx, main, types, budget, &out); err != nil {
		return Inspection{}, err
	}
	return out, nil
}

func (a *archive) inspectParts(ctx context.Context, main string, types map[string]string, budget *xmlBudget, out *Inspection) error {
	names := slices.Clone(a.order)
	slices.Sort(names)
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "[Content_Types].xml" || strings.HasSuffix(name, ".rels") || strings.HasSuffix(name, "/") {
			continue
		}
		if err := a.inspectPart(ctx, name, main, types[name], budget, out); err != nil {
			return err
		}
	}
	return nil
}

func (a *archive) inspectPart(ctx context.Context, name, main, mediaType string, budget *xmlBudget, out *Inspection) error {
	kind := partKind(mediaType)
	if kind == "asset" {
		out.Assets = append(out.Assets, Asset{Part: name, MediaType: mediaType, Bytes: a.files[name].UncompressedSize64})
		return nil
	}
	n, err := a.readXML(ctx, name, budget)
	if err != nil && kind == partUnknownXML && !errors.Is(err, ErrLimit) && ctx.Err() == nil {
		out.Findings = append(out.Findings, Finding{Code: "UNSUPPORTED_SOURCE_PART", Locator: Locator{Part: name}})
		return nil
	}
	if err != nil {
		return err
	}
	if name == main && (!n.word("document") || n.child("body") == nil) {
		return fmt.Errorf("%w: Word document body absent", ErrInvalidPackage)
	}
	part := Part{Name: name, Kind: kind, Paragraphs: []Paragraph{}, Structures: []Structure{}}
	switch kind {
	case partStyles, "numbering", "metadata", "settings", partUnknownXML:
		part.Properties = []Property{property(n)}
	}
	if kind == partStyles {
		out.Findings = append(out.Findings, Finding{Code: styleResolutionRequired, Locator: Locator{Part: name, Path: n.path}})
	}
	walkPart(n, walkState{paragraph: -1, run: -1}, &part, &out.Findings)
	finishRunText(&part)
	if kind == partUnknownXML {
		out.Findings = append(out.Findings, Finding{Code: "UNSUPPORTED_SOURCE_PART", Locator: Locator{Part: name, Path: n.path}})
	}
	if name == main && !hasText(part) {
		out.Findings = append(out.Findings, Finding{Code: "NO_EXTRACTABLE_TEXT", Locator: Locator{Part: name, Path: n.path}})
	}
	out.Parts = append(out.Parts, part)
	return nil
}

func partKind(mediaType string) string {
	for _, kind := range []string{"document.main", "header", "footer", "footnotes", "endnotes", "comments", partStyles, "numbering", "settings"} {
		if mediaType == "application/vnd.openxmlformats-officedocument.wordprocessingml."+kind+"+xml" {
			return strings.TrimSuffix(kind, ".main")
		}
	}
	if strings.Contains(mediaType, "properties+xml") {
		return "metadata"
	}
	if strings.HasSuffix(mediaType, "+xml") || mediaType == "application/xml" || mediaType == "text/xml" {
		return partUnknownXML
	}
	return "asset"
}

type walkState struct {
	paragraph  int
	run        int
	containers []Locator
	inObject   bool
}

func walkPart(n *element, state walkState, part *Part, findings *[]Finding) {
	loc := Locator{Part: part.Name, Path: n.path}
	if isContainer(n) {
		state.containers = append(slices.Clone(state.containers), loc)
		part.Structures = append(part.Structures, Structure{Locator: loc, SourceRange: n.SourceRange, Kind: n.name.Local, Attributes: attributes(n), Properties: containerProperties(n)})
	}
	if !state.inObject && isUnresolvedObject(n) {
		part.Objects = append(part.Objects, Object{Locator: loc, SourceRange: n.SourceRange, Content: property(n)})
		state.inObject = true
	}
	if code := findingCode(n); code != "" {
		*findings = append(*findings, Finding{Code: code, Locator: loc})
	}
	if n.word("p") {
		state.paragraph, state.run = len(part.Paragraphs), -1
		part.Paragraphs = append(part.Paragraphs, Paragraph{Locator: loc, SourceRange: n.SourceRange, Containers: slices.Clone(state.containers), Properties: properties(n.child("pPr")), Runs: []Run{}})
	}
	if n.word("r") && state.paragraph >= 0 {
		p := &part.Paragraphs[state.paragraph]
		state.run = len(p.Runs)
		p.Runs = append(p.Runs, Run{Locator: loc, Properties: properties(n.child("rPr")), Contexts: slices.Clone(state.containers)})
	}
	if text, present := sourceText(n); present {
		if state.paragraph >= 0 && state.run >= 0 {
			r := &part.Paragraphs[state.paragraph].Runs[state.run]
			r.Fragments = append(r.Fragments, Fragment{Locator: loc, SourceRange: n.SourceRange, Kind: n.name.Local, Text: text})
		} else if strings.TrimSpace(text) != "" {
			part.Unassigned = append(part.Unassigned, Fragment{Locator: loc, SourceRange: n.SourceRange, Kind: n.name.Local, Text: text})
			*findings = append(*findings, Finding{Code: "UNASSIGNED_SOURCE_TEXT", Locator: loc})
		}
	}
	for _, child := range n.children {
		walkPart(child, state, part, findings)
	}
}

func sourceText(n *element) (string, bool) {
	if !n.word(n.name.Local) {
		return "", false
	}
	switch n.name.Local {
	case "t", "delText", elementInstruction:
		return n.text.String(), true
	case elementTab:
		return "\t", true
	case "br", "cr":
		return "\n", true
	case "noBreakHyphen":
		return "\u2011", true
	case "softHyphen":
		return "\u00ad", true
	}
	return "", false
}

func finishRunText(part *Part) {
	for i := range part.Paragraphs {
		for j := range part.Paragraphs[i].Runs {
			r := &part.Paragraphs[i].Runs[j]
			var text strings.Builder
			for _, fragment := range r.Fragments {
				text.WriteString(fragment.Text)
			}
			r.Text = text.String()
		}
	}
}

func containerProperties(n *element) []Property {
	var out []Property
	for _, child := range n.children {
		for _, local := range []string{"tblPr", "tblGrid", "trPr", "tcPr", "sdtPr"} {
			if child.word(local) {
				out = append(out, property(child))
			}
		}
	}
	return out
}

func isUnresolvedObject(n *element) bool {
	if n.name.Space == "http://schemas.openxmlformats.org/officeDocument/2006/math" || n.name.Space == "http://purl.oclc.org/ooxml/officeDocument/math" || n.name.Space == "http://schemas.openxmlformats.org/markup-compatibility/2006" {
		return true
	}
	for _, local := range []string{elementDrawing, elementPicture, elementObject, "altChunk", "sym", "sectPr", "br", "fldChar", "footnoteReference", "endnoteReference", "commentReference", "commentRangeStart", "commentRangeEnd", "bookmarkStart", "bookmarkEnd"} {
		if n.word(local) {
			return true
		}
	}
	return false
}

func isContainer(n *element) bool {
	for _, name := range []string{elementTable, "tr", "tc", elementInsertion, elementDeletion, elementMoveFrom, elementMoveTo, elementTextBox, "footnote", "endnote", "comment", "hyperlink", "sdt", elementField, elementDrawing, elementPicture} {
		if n.word(name) {
			return true
		}
	}
	return false
}

func findingCode(n *element) string {
	if n.name.Local == "AlternateContent" && n.name.Space == "http://schemas.openxmlformats.org/markup-compatibility/2006" {
		return "ALTERNATE_CONTENT_REQUIRES_RESOLUTION"
	}
	if (n.name.Local == "oMath" || n.name.Local == "oMathPara") && isUnresolvedObject(n) {
		return "EQUATION_REQUIRES_RESOLUTION"
	}
	for _, local := range []string{elementInsertion, elementDeletion, elementMoveFrom, elementMoveTo, "pPrChange", "rPrChange", "numPrChange"} {
		if n.word(local) {
			return "TRACKED_CHANGE_REQUIRES_REVIEW"
		}
	}
	if n.word(n.name.Local) {
		return sourceFindings[n.name.Local]
	}
	return ""
}

var sourceFindings = map[string]string{
	"pStyle": styleResolutionRequired, "rStyle": styleResolutionRequired, "numPr": "NUMBERING_RESOLUTION_REQUIRED",
	propertyHidden: "HIDDEN_TEXT_REQUIRES_REVIEW", propertyWebHidden: "HIDDEN_TEXT_REQUIRES_REVIEW", "txbxContent": "TEXTBOX_ORDER_REQUIRES_REVIEW",
	elementField: "FIELD_REQUIRES_REVIEW", elementInstruction: "FIELD_REQUIRES_REVIEW", "altChunk": "UNSUPPORTED_DOCUMENT_OBJECT",
	"sym": "SYMBOL_FONT_REQUIRES_REVIEW", "cols": "COLUMN_ORDER_REQUIRES_REVIEW", elementDrawing: "DRAWING_REQUIRES_RESOLUTION",
	elementPicture: "DRAWING_REQUIRES_RESOLUTION", elementObject: "UNSUPPORTED_DOCUMENT_OBJECT",
}

func properties(n *element) []Property {
	if n == nil {
		return nil
	}
	out := make([]Property, 0, len(n.children))
	for _, child := range n.children {
		out = append(out, property(child))
	}
	return out
}

func property(n *element) Property {
	return Property{Path: n.path, Name: "{" + n.name.Space + "}" + n.name.Local, Text: n.text.String(), Attributes: attributes(n), Children: properties(n)}
}

func attributes(n *element) map[string]string {
	out := make(map[string]string, len(n.attrs))
	for _, attr := range n.attrs {
		if attr.Name.Space != "xmlns" && attr.Name.Local != "xmlns" {
			out["{"+attr.Name.Space+"}"+attr.Name.Local] = attr.Value
		}
	}
	return out
}

func hasText(part Part) bool {
	for _, p := range part.Paragraphs {
		for _, r := range p.Runs {
			if strings.TrimSpace(r.Text) != "" {
				return true
			}
		}
	}
	return false
}

func relationshipSource(name string) (string, error) {
	if name == "_rels/.rels" {
		return "", nil
	}
	dir, base := path.Split(name)
	if path.Base(strings.TrimSuffix(dir, "/")) != "_rels" || !strings.HasSuffix(base, ".rels") {
		return "", fmt.Errorf("%w: relationship part location", ErrInvalidPackage)
	}
	return path.Join(path.Dir(strings.TrimSuffix(dir, "/")), strings.TrimSuffix(base, ".rels")), nil
}
