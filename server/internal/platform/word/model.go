package word

// Inspection is private source evidence, not normalized exam content or a learner payload.
type Inspection struct {
	MainPart      string         `json:"mainPart"`
	Parts         []Part         `json:"parts"`
	Relationships []Relationship `json:"relationships"`
	Assets        []Asset        `json:"assets"`
	Findings      []Finding      `json:"findings"`
}

// Locator identifies a structural position within one immutable source revision.
type Locator struct {
	Part string `json:"part"`
	Path string `json:"path"`
}

// Part retains ordered paragraphs and the source's structural containers.
type Part struct {
	Name       string      `json:"name"`
	Kind       string      `json:"kind"`
	Paragraphs []Paragraph `json:"paragraphs"`
	Structures []Structure `json:"structures"`
	Properties []Property  `json:"properties,omitempty"`
	Unassigned []Fragment  `json:"unassigned,omitempty"`
	Objects    []Object    `json:"objects,omitempty"`
}

// Fragment retains text that is outside an understood paragraph/run structure.
type Fragment struct {
	Locator
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// Structure records the original attributes of a table, revision or other contextual object.
type Structure struct {
	Locator
	Kind       string            `json:"kind"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Properties []Property        `json:"properties,omitempty"`
}

// Object retains a source subtree whose layout or semantics are not yet resolved.
type Object struct {
	Locator
	Content Property `json:"content"`
}

// Paragraph retains raw run evidence and ancestor identities without inferring a question boundary.
type Paragraph struct {
	Locator
	Containers []Locator  `json:"containers,omitempty"`
	Properties []Property `json:"properties,omitempty"`
	Runs       []Run      `json:"runs"`
}

// Run retains explicit properties; inherited styles and numbering require semantic resolution.
type Run struct {
	Locator
	Text       string     `json:"text"`
	Fragments  []Fragment `json:"fragments,omitempty"`
	Properties []Property `json:"properties,omitempty"`
	Contexts   []Locator  `json:"contexts,omitempty"`
}

// Property retains a namespaced property and its attributes for the semantic resolver.
type Property struct {
	Path       string            `json:"path,omitempty"`
	Name       string            `json:"name"`
	Text       string            `json:"text,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Children   []Property        `json:"children,omitempty"`
}

// Relationship is package metadata only; external targets are never fetched.
type Relationship struct {
	Source         string `json:"source"`
	ID             string `json:"id"`
	Type           string `json:"type"`
	Target         string `json:"target"`
	OriginalTarget string `json:"originalTarget"`
	External       bool   `json:"external"`
}

// Asset inventories an embedded object without exposing it as publishable media.
type Asset struct {
	Part      string `json:"part"`
	MediaType string `json:"mediaType"`
	Bytes     uint64 `json:"bytes"`
}

// Finding names unresolved source semantics and the evidence requiring a later decision.
type Finding struct {
	Code string `json:"code"`
	Locator
}
