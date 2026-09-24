package word

import (
	"slices"
	"strings"
	"unicode"
)

type propertyLayer struct {
	part  string
	props []Property
}

type styleResult struct {
	complete  bool
	layers    []propertyLayer
	ids       []string
	numbering []Property
}

type styleIndex struct {
	part              string
	valid             bool
	definitions       map[string]Property
	duplicates        map[string]bool
	cache             map[string]styleResult
	defaultParagraph  string
	runDefaults       propertyLayer
	paragraphDefaults []Property
}

func (r *resolver) initStyles() error {
	part, valid := r.linkedPart(partStyles)
	r.styles = styleIndex{part: part.Name, valid: valid, definitions: map[string]Property{}, duplicates: map[string]bool{}, cache: map[string]styleResult{}}
	if part.Name == "" {
		return nil
	}
	root := prop(part.Properties, partStyles)
	if root.Name == "" {
		r.styles.valid = false
		r.finding("INVALID_STYLES_ROOT", Locator{Part: part.Name})
		return nil
	}
	defaults := prop(root.Children, "docDefaults")
	r.styles.runDefaults = propertyLayer{part: part.Name, props: prop(prop(defaults.Children, "rPrDefault").Children, propertyRun).Children}
	r.styles.paragraphDefaults = prop(prop(defaults.Children, "pPrDefault").Children, propertyParagraph).Children
	if duplicateNamedProperty(root.Children, "docDefaults") || duplicateProperties(defaults.Children) || duplicateProperties(r.styles.paragraphDefaults) {
		r.styles.valid = false
		r.finding("AMBIGUOUS_DOCUMENT_DEFAULTS", Locator{Part: part.Name, Path: defaults.Path})
	}
	for _, child := range root.Children {
		if err := r.spend(1 + len(child.Children)); err != nil {
			return err
		}
		if wordName(child.Name, "style") {
			r.addStyle(child)
		}
	}
	return nil
}

func (r *resolver) addStyle(p Property) {
	id := attr(p, "styleId")
	if _, exists := r.styles.definitions[id]; exists || id == "" {
		r.styles.duplicates[id] = true
		r.finding("DUPLICATE_OR_MISSING_STYLE_ID", Locator{Part: r.styles.part, Path: p.Path})
	}
	r.styles.definitions[id] = p
	isDefault, _ := onOff(attr(p, "default"))
	if attr(p, "default") == "" || !isDefault || attr(p, "type") != styleParagraph {
		return
	}
	if r.styles.defaultParagraph != "" {
		r.styles.valid = false
		r.finding("AMBIGUOUS_DEFAULT_STYLE", Locator{Part: r.styles.part, Path: p.Path})
	}
	r.styles.defaultParagraph = id
}

func (r *resolver) style(id, kind string) (styleResult, error) {
	if err := r.spend(len(id) + 1); err != nil {
		return styleResult{}, err
	}
	key := kind + ":" + id
	if cached, ok := r.styles.cache[key]; ok {
		return cached, nil
	}
	result := styleResult{complete: r.styles.valid}
	var chain []Property
	seen := map[string]bool{}
	for id != "" {
		if err := r.spend(len(id) + 1); err != nil {
			return styleResult{}, err
		}
		definition, found := r.styles.definitions[id]
		if !found || attr(definition, "type") != kind || r.styles.duplicates[id] || seen[id] || len(chain) >= 64 {
			result.complete = false
			break
		}
		seen[id] = true
		chain = append(chain, definition)
		result.ids = append(result.ids, id)
		id = val(definition.Children, "basedOn")
	}
	slices.Reverse(chain)
	for _, definition := range chain {
		run := prop(definition.Children, propertyRun)
		paragraph := prop(definition.Children, propertyParagraph)
		if duplicateProperties(definition.Children) || duplicateProperties(paragraph.Children) || duplicateProperties(prop(paragraph.Children, propertyNumbering).Children) {
			result.complete = false
		}
		if err := r.spend(len(run.Children) + len(paragraph.Children)); err != nil {
			return styleResult{}, err
		}
		result.layers = append(result.layers, propertyLayer{part: r.styles.part, props: run.Children})
		result.numbering = mergeNumProps(result.numbering, prop(paragraph.Children, propertyNumbering).Children)
	}
	r.styles.cache[key] = result
	return result, nil
}

func mergeNumProps(base, next []Property) []Property {
	var out []Property
	for _, name := range []string{propertyNumberID, propertyLevel} {
		p := prop(next, name)
		if p.Name == "" {
			p = prop(base, name)
		}
		if p.Name != "" {
			out = append(out, p)
		}
	}
	return out
}

var semanticMarkNames = []string{"b", "i", "u", "strike", "dstrike", propertyVertical, propertyHidden, propertyWebHidden, "caps", "smallCaps"}

func (r *resolver) resolveRun(p Paragraph, run Run, paragraphStyle styleResult, structures map[Locator]Structure) (ResolvedRun, error) {
	character, err := r.style(val(run.Properties, "rStyle"), styleCharacter)
	if err != nil {
		return ResolvedRun{}, err
	}
	result := ResolvedRun{Locator: run.Locator, Complete: paragraphStyle.complete && character.complete, Marks: []ResolvedMark{}}
	if !result.Complete {
		r.finding("STYLE_CHAIN_REQUIRES_REVIEW", run.Locator)
	}
	if r.unsupportedRunContext(p, run, structures) {
		result.Complete = false
	}
	layers := []propertyLayer{r.styles.runDefaults}
	layers = append(layers, paragraphStyle.layers...)
	layers = append(layers, character.layers...)
	marks := make(map[string]ResolvedMark, len(semanticMarkNames))
	for _, layer := range layers {
		if unsafeMarkLayer(layer.props) {
			result.Complete = false
		}
		if err := r.applyMarks(marks, layer, false); err != nil {
			return ResolvedRun{}, err
		}
	}
	if unsafeMarkLayer(run.Properties) {
		result.Complete = false
	}
	if err := r.applyMarks(marks, propertyLayer{part: run.Part, props: run.Properties}, true); err != nil {
		return ResolvedRun{}, err
	}
	return r.finishMarks(result, marks)
}

func (r *resolver) finishMarks(result ResolvedRun, marks map[string]ResolvedMark) (ResolvedRun, error) {
	contextComplete := result.Complete
	for _, name := range semanticMarkNames {
		mark, exists := marks[name]
		if !exists {
			continue
		}
		if !contextComplete {
			mark.Resolved = false
		}
		if !mark.Resolved {
			mark.Value = ""
			result.Complete = false
		}
		result.Marks = append(result.Marks, mark)
		if err := r.retainLocations(mark.Sources); err != nil {
			return ResolvedRun{}, err
		}
	}
	if !result.Complete {
		r.finding("SEMANTIC_MARK_REQUIRES_REVIEW", result.Locator)
	}
	return result, nil
}

func duplicateProperties(props []Property) bool {
	seen := make(map[string]bool, len(props))
	for _, p := range props {
		if seen[p.Name] {
			return true
		}
		seen[p.Name] = true
	}
	return false
}

func duplicateNamedProperty(props []Property, name string) bool {
	found := false
	for _, p := range props {
		if !wordName(p.Name, name) {
			continue
		}
		if found {
			return true
		}
		found = true
	}
	return false
}

func unsafeMarkLayer(props []Property) bool {
	if duplicateProperties(props) {
		return true
	}
	for _, name := range []string{"cs", "rtl"} {
		p := prop(props, name)
		enabled, valid := onOff(attr(p, propertyValue))
		if p.Name != "" && (enabled || !valid) {
			return true
		}
	}
	return false
}

func (r *resolver) applyMarks(marks map[string]ResolvedMark, layer propertyLayer, direct bool) error {
	if err := r.spend(len(layer.props) + len(semanticMarkNames)); err != nil {
		return err
	}
	for _, p := range layer.props {
		name := ""
		for _, candidate := range semanticMarkNames {
			if wordName(p.Name, candidate) {
				name = candidate
				break
			}
		}
		if name == "" {
			continue
		}
		old, exists := marks[name]
		value, valid := markValue(name, p)
		if exists && !direct && name != "u" && name != propertyVertical {
			valid = false
		}
		sources := append(slices.Clone(old.Sources), Locator{Part: layer.part, Path: p.Path})
		marks[name] = ResolvedMark{Name: name, Value: value, Resolved: valid, Sources: sources}
	}
	return nil
}

func markValue(name string, p Property) (string, bool) {
	value := attr(p, propertyValue)
	switch name {
	case "u":
		return value, slices.Contains([]string{"none", "single", "words", "double", "thick", "dotted", "dottedHeavy", "dash", "dashedHeavy", "dashLong", "dashLongHeavy", "dotDash", "dashDotHeavy", "dotDotDash", "dashDotDotHeavy", "wave", "wavyHeavy", "wavyDouble"}, value)
	case propertyVertical:
		return value, slices.Contains([]string{"baseline", "subscript", "superscript"}, value)
	default:
		enabled, valid := onOff(value)
		if enabled {
			return "on", valid
		}
		return "off", valid
	}
}

func (r *resolver) unsupportedRunContext(p Paragraph, run Run, structures map[Locator]Structure) bool {
	unsupported := false
	for _, loc := range p.Containers {
		if structures[loc].Kind == elementTable {
			r.finding("TABLE_STYLE_REQUIRES_REVIEW", run.Locator)
			unsupported = true
			break
		}
	}
	if strings.ContainsFunc(run.Text, func(c rune) bool { return unicode.IsLetter(c) && !unicode.Is(unicode.Latin, c) }) {
		r.finding("SCRIPT_MARKS_REQUIRES_REVIEW", run.Locator)
		unsupported = true
	}
	return unsupported
}
