package word

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"
)

func (a *archive) contentTypes(ctx context.Context, budget *xmlBudget) (map[string]string, error) {
	root, err := a.readXML(ctx, "[Content_Types].xml", budget)
	if err != nil {
		return nil, err
	}
	if root.name.Space != contentNamespace || root.name.Local != "Types" {
		return nil, fmt.Errorf("%w: content-type manifest", ErrInvalidPackage)
	}
	defaults, overrides := map[string]string{}, map[string]string{}
	for _, n := range root.children {
		if err := addContentType(n, defaults, overrides); err != nil {
			return nil, err
		}
	}
	types := make(map[string]string, len(a.files))
	for name := range a.files {
		if name == "[Content_Types].xml" || strings.HasSuffix(name, "/") {
			continue
		}
		kind := overrides[name]
		if kind == "" {
			kind = defaults[strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))]
		}
		if kind == "" {
			return nil, fmt.Errorf("%w: undeclared part content type", ErrInvalidPackage)
		}
		types[name] = kind
	}
	return types, nil
}

func addContentType(n *element, defaults, overrides map[string]string) error {
	if n.name.Space != contentNamespace {
		return fmt.Errorf("%w: content-type namespace", ErrInvalidPackage)
	}
	kind := n.attr("ContentType")
	if kind == "" {
		return fmt.Errorf("%w: absent content type", ErrInvalidPackage)
	}
	lower := strings.ToLower(kind)
	for _, active := range []string{"macroenabled", "vbaproject", "activex", "oleobject"} {
		if strings.Contains(lower, active) {
			return ErrActiveContent
		}
	}
	var key string
	target := defaults
	switch n.name.Local {
	case "Default":
		key = strings.ToLower(n.attr("Extension"))
		if key == "" || strings.ContainsAny(key, "/\\.:") {
			return fmt.Errorf("%w: invalid content-type extension", ErrInvalidPackage)
		}
	case "Override":
		u, err := url.Parse(n.attr("PartName"))
		if err != nil || u.Scheme != "" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/") {
			return fmt.Errorf("%w: invalid content-type part", ErrInvalidPackage)
		}
		key, target = strings.TrimPrefix(u.Path, "/"), overrides
		if !safePartName(key) {
			return fmt.Errorf("%w: invalid content-type path", ErrInvalidPackage)
		}
	default:
		return fmt.Errorf("%w: unknown content-type declaration", ErrInvalidPackage)
	}
	if _, duplicate := target[key]; duplicate {
		return fmt.Errorf("%w: duplicate content-type declaration", ErrInvalidPackage)
	}
	target[key] = kind
	return nil
}

func (a *archive) relationships(ctx context.Context, budget *xmlBudget) ([]Relationship, error) {
	if _, ok := a.files["_rels/.rels"]; !ok {
		return nil, fmt.Errorf("%w: root relationships absent", ErrInvalidPackage)
	}
	out := []Relationship{}
	names := slices.Clone(a.order)
	slices.Sort(names)
	for _, name := range names {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		source, err := relationshipSource(name)
		if err != nil {
			return nil, err
		}
		if source != "" && a.files[source] == nil {
			return nil, fmt.Errorf("%w: relationship owner absent", ErrInvalidPackage)
		}
		root, err := a.readXML(ctx, name, budget)
		if err != nil {
			return nil, err
		}
		if root.name.Space != relationNamespace || root.name.Local != "Relationships" {
			return nil, fmt.Errorf("%w: relationships manifest", ErrInvalidPackage)
		}
		seen := map[string]bool{}
		for _, n := range root.children {
			rel, err := a.readRelationship(n, source, seen)
			if err != nil {
				return nil, err
			}
			out = append(out, rel)
		}
	}
	return out, nil
}

func (a *archive) readRelationship(n *element, source string, seen map[string]bool) (Relationship, error) {
	id, kind, target, mode := n.attr("Id"), n.attr("Type"), n.attr("Target"), n.attr("TargetMode")
	if n.name.Space != relationNamespace || n.name.Local != "Relationship" || id == "" || kind == "" || target == "" || seen[id] || (mode != "" && mode != "Internal" && mode != "External") {
		return Relationship{}, fmt.Errorf("%w: invalid relationship", ErrInvalidPackage)
	}
	seen[id] = true
	for _, active := range []string{"/oleObject", "/vbaProject", "/control", "/attachedTemplate", "/package"} {
		if strings.HasSuffix(kind, active) {
			return Relationship{}, ErrActiveContent
		}
	}
	rel := Relationship{Source: source, ID: id, Type: kind, Target: target, OriginalTarget: target, External: mode == "External"}
	if rel.External {
		return rel, nil
	}
	resolved, err := resolveTarget(source, target)
	if err != nil {
		return Relationship{}, err
	}
	if a.files[resolved] == nil {
		return Relationship{}, fmt.Errorf("%w: relationship target absent", ErrInvalidPackage)
	}
	rel.Target = resolved
	return rel, nil
}

func resolveTarget(source, target string) (string, error) {
	u, err := url.Parse(target)
	if err != nil || u.Scheme != "" || u.Host != "" || u.RawQuery != "" || strings.ContainsAny(u.Path, "\\:\x00") {
		return "", fmt.Errorf("%w: unsafe internal relationship", ErrInvalidPackage)
	}
	resolved := source
	if u.Path != "" {
		resolved = path.Join(path.Dir(source), u.Path)
		if strings.HasPrefix(u.Path, "/") {
			resolved = strings.TrimPrefix(u.Path, "/")
		}
	}
	if !safePartName(resolved) {
		return "", fmt.Errorf("%w: relationship escapes package", ErrInvalidPackage)
	}
	return resolved, nil
}

func mainPart(rels []Relationship, types map[string]string) (string, error) {
	main := ""
	for _, rel := range rels {
		if rel.Source != "" || (rel.Type != "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" && rel.Type != "http://purl.oclc.org/ooxml/officeDocument/relationships/officeDocument") {
			continue
		}
		if main != "" || rel.External || types[rel.Target] != "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml" {
			return "", fmt.Errorf("%w: main document relationship", ErrInvalidPackage)
		}
		main = rel.Target
	}
	if main == "" {
		return "", fmt.Errorf("%w: main document absent", ErrInvalidPackage)
	}
	return main, nil
}
