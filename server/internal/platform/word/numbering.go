package word

import (
	"strconv"
	"strings"
)

const (
	partNumbering      = "numbering"
	elementAbstractNum = "abstractNum"
	elementAbstractID  = "abstractNumId"
	elementListLevel   = "lvl"
	elementStart       = "start"
	formatDecimal      = "decimal"
)

type numberingIndex struct {
	part          string
	valid         bool
	ambiguousBody bool
	tainted       bool
	instances     map[string]Property
	abstracts     map[string]Property
	duplicates    map[string]bool
	cache         map[string]*listDefinition
	counters      map[string]*listCounter
}

type listDefinition struct {
	valid     bool
	levels    [9]listLevel
	counter   *listCounter
	sources   []Locator
	overrides [9]bool
}

type listCounter struct {
	counts [9]int64
	seen   [9]bool
}

type listLevel struct {
	present bool
	valid   bool
	start   int64
	restart int
	format  string
	pattern string
	suffix  string
	style   string
	legal   bool
	sources []Locator
}

func (r *resolver) initNumbering() error {
	part, valid := r.linkedPart(partNumbering)
	r.numbering = numberingIndex{part: part.Name, valid: valid, instances: map[string]Property{}, abstracts: map[string]Property{}, duplicates: map[string]bool{}, cache: map[string]*listDefinition{}, counters: map[string]*listCounter{}}
	for _, finding := range r.source.Findings {
		if finding.Part == r.source.MainPart && (finding.Code == "TRACKED_CHANGE_REQUIRES_REVIEW" || finding.Code == "ALTERNATE_CONTENT_REQUIRES_RESOLUTION") {
			r.numbering.ambiguousBody = true
		}
	}
	if part.Name == "" {
		return nil
	}
	root := prop(part.Properties, partNumbering)
	if root.Name == "" {
		r.numbering.valid = false
		r.finding("INVALID_NUMBERING_ROOT", Locator{Part: part.Name})
	}
	for _, child := range root.Children {
		if err := r.spend(1 + len(child.Children)); err != nil {
			return err
		}
		switch {
		case wordName(child.Name, "num"):
			r.addNumberDefinition(child, propertyNumberID, r.numbering.instances)
		case wordName(child.Name, elementAbstractNum):
			r.addNumberDefinition(child, elementAbstractID, r.numbering.abstracts)
		}
	}
	return nil
}

func (r *resolver) addNumberDefinition(p Property, attribute string, destination map[string]Property) {
	id, valid := numberID(attr(p, attribute))
	if _, exists := destination[id]; exists || !valid {
		r.numbering.duplicates[attribute+":"+id] = true
		r.finding("DUPLICATE_OR_INVALID_NUMBERING_ID", Locator{Part: r.numbering.part, Path: p.Path})
	}
	destination[id] = p
}

func numberID(value string) (string, bool) {
	n, err := strconv.ParseInt(value, 10, 32)
	return strconv.FormatInt(n, 10), err == nil && n >= 0
}

func (r *resolver) list(id string) (*listDefinition, error) {
	if cached, ok := r.numbering.cache[id]; ok {
		return cached, nil
	}
	instance, found := r.numbering.instances[id]
	abstractID, idValid := numberID(val(instance.Children, elementAbstractID))
	abstract, abstractFound := r.numbering.abstracts[abstractID]
	result := &listDefinition{valid: r.numbering.valid && found && idValid && abstractFound && !r.numbering.duplicates[propertyNumberID+":"+id] && !r.numbering.duplicates[elementAbstractID+":"+abstractID]}
	result.sources = []Locator{{Part: r.numbering.part, Path: instance.Path}, {Part: r.numbering.part, Path: abstract.Path}}
	if duplicateNamedProperty(instance.Children, elementAbstractID) {
		result.valid = false
	}
	if prop(abstract.Children, "numStyleLink").Name != "" || unsupportedNumberAttributes(abstract) {
		result.valid = false
	}
	for _, child := range abstract.Children {
		if err := r.spend(1 + len(child.Children)); err != nil {
			return nil, err
		}
		if wordName(child.Name, elementListLevel) {
			r.addLevel(result, child)
		}
	}
	overridden := false
	for _, child := range instance.Children {
		if err := r.spend(1 + len(child.Children)); err != nil {
			return nil, err
		}
		if wordName(child.Name, "lvlOverride") {
			r.overrideLevel(result, child)
			overridden = true
		}
	}
	result.counter = r.counter(abstractID, overridden)
	r.numbering.cache[id] = result
	return result, nil
}

func (r *resolver) counter(abstractID string, overridden bool) *listCounter {
	if overridden {
		return &listCounter{}
	}
	shared, ok := r.numbering.counters[abstractID]
	if !ok {
		shared = &listCounter{}
		r.numbering.counters[abstractID] = shared
	}
	return shared
}

func listIndex(value string) (int, bool) {
	n, err := strconv.Atoi(value)
	return n, err == nil && n >= 0 && n < 9
}

func (r *resolver) addLevel(list *listDefinition, p Property) {
	index, valid := listIndex(attr(p, propertyLevel))
	if !valid || list.levels[index].present {
		list.valid = false
		return
	}
	list.levels[index] = r.readLevel(p, index)
}

func (r *resolver) readLevel(p Property, index int) listLevel {
	start, err := strconv.ParseInt(val(p.Children, elementStart), 10, 32)
	if prop(p.Children, elementStart).Name == "" {
		start, err = 0, nil
	}
	level := listLevel{present: true, valid: err == nil && start >= 0, start: start, restart: index - 1,
		format: val(p.Children, "numFmt"), pattern: val(p.Children, "lvlText"), suffix: val(p.Children, "suff"), style: val(p.Children, "pStyle"),
		sources: []Locator{{Part: r.numbering.part, Path: p.Path}}}
	if prop(p.Children, "numFmt").Name == "" {
		level.format = formatDecimal
	}
	if duplicateProperties(p.Children) || attr(prop(p.Children, "numFmt"), "format") != "" || attr(prop(p.Children, "lvlText"), "null") != "" || symbolNumberFont(p) {
		level.valid = false
	}
	if level.suffix == "" {
		level.suffix = elementTab
	}
	if prop(p.Children, "lvlText").Name == "" || len(level.pattern) > 1024 || prop(p.Children, "lvlPicBulletId").Name != "" {
		level.valid = false
	}
	if level.suffix != elementTab && level.suffix != "space" && level.suffix != "nothing" {
		level.valid = false
	}
	if prop(p.Children, "isLgl").Name != "" {
		var valid bool
		level.legal, valid = onOff(val(p.Children, "isLgl"))
		level.valid = level.valid && valid
	}
	readLevelRestart(&level, p, index)
	return level
}

func symbolNumberFont(p Property) bool {
	fonts := prop(prop(p.Children, propertyRun).Children, "rFonts")
	for _, slot := range []string{"ascii", "hAnsi"} {
		switch strings.ToLower(attr(fonts, slot)) {
		case "symbol", "wingdings", "wingdings 2", "wingdings 3", "webdings":
			return true
		}
	}
	return false
}

func readLevelRestart(level *listLevel, p Property, index int) {
	restart := prop(p.Children, "lvlRestart")
	if restart.Name == "" {
		return
	}
	n, err := strconv.ParseInt(attr(restart, propertyValue), 10, 32)
	if err != nil || n < 0 {
		level.valid = false
		return
	}
	if n <= int64(index) {
		level.restart = int(n) - 1
	}
}

func (r *resolver) overrideLevel(list *listDefinition, p Property) {
	index, valid := listIndex(attr(p, propertyLevel))
	if !valid || list.overrides[index] || duplicateProperties(p.Children) {
		list.valid = false
		return
	}
	list.overrides[index] = true
	replacement := prop(p.Children, elementListLevel)
	if replacement.Name != "" {
		replacementIndex, ok := listIndex(attr(replacement, propertyLevel))
		if !ok || replacementIndex != index {
			list.valid = false
			return
		}
		list.levels[index] = r.readLevel(replacement, index)
	}
	level := &list.levels[index]
	level.sources = append(level.sources, Locator{Part: r.numbering.part, Path: p.Path})
	if override := prop(p.Children, "startOverride"); override.Name != "" {
		n, err := strconv.ParseInt(attr(override, propertyValue), 10, 32)
		if err != nil || n < 0 {
			level.valid = false
			return
		}
		level.start = n
	}
}

func unsupportedNumberAttributes(p Property) bool {
	for name, value := range p.Attributes {
		if strings.HasSuffix(name, "}restartNumberingAfterBreak") {
			enabled, valid := onOff(value)
			if enabled || !valid {
				return true
			}
		}
	}
	return false
}
