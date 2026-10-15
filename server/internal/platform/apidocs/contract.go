package apidocs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

// ContractJSON converts source, the contract as it is written in YAML, to
// indented JSON that keeps every key, value and the order they were written in,
// so the document a client downloads is the file the contract is authored in
// and not a model of it that the code generator has rewritten. A mapping with a
// repeated key, an alias, a merge key or a key that is not a scalar is refused:
// JSON has no reading of them that every consumer would share.
func ContractJSON(source []byte) ([]byte, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(source))
	var root, extra yaml.Node
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("apidocs: read contract: %w", err)
	}
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("apidocs: the contract is not one YAML document")
	}
	var compact bytes.Buffer
	if err := writeNode(&compact, root.Content[0]); err != nil {
		return nil, fmt.Errorf("apidocs: convert contract: %w", err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, compact.Bytes(), "", "  "); err != nil {
		return nil, fmt.Errorf("apidocs: convert contract: %w", err)
	}
	indented.WriteByte('\n')
	return indented.Bytes(), nil
}

func writeNode(w *bytes.Buffer, n *yaml.Node) error {
	switch n.Kind {
	case yaml.MappingNode:
		return writeMapping(w, n)
	case yaml.SequenceNode:
		w.WriteByte('[')
		for i, item := range n.Content {
			if i > 0 {
				w.WriteByte(',')
			}
			if err := writeNode(w, item); err != nil {
				return err
			}
		}
		w.WriteByte(']')
		return nil
	case yaml.ScalarNode:
		return writeScalar(w, n)
	case yaml.AliasNode:
		return fmt.Errorf("line %d: aliases are not supported", n.Line)
	}
	return fmt.Errorf("line %d: unsupported YAML node", n.Line)
}

func writeMapping(w *bytes.Buffer, n *yaml.Node) error {
	seen := make(map[string]bool, len(n.Content)/2)
	w.WriteByte('{')
	for i := 0; i < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		switch {
		case key.Kind != yaml.ScalarNode:
			return fmt.Errorf("line %d: a mapping key is not a scalar", key.Line)
		case key.ShortTag() == "!!merge":
			return fmt.Errorf("line %d: merge keys are not supported", key.Line)
		case seen[key.Value]:
			return fmt.Errorf("line %d: the key %q is repeated", key.Line, key.Value)
		}
		seen[key.Value] = true
		if i > 0 {
			w.WriteByte(',')
		}
		if err := writeString(w, key.Value); err != nil {
			return err
		}
		w.WriteByte(':')
		if err := writeNode(w, value); err != nil {
			return err
		}
	}
	w.WriteByte('}')
	return nil
}

func writeScalar(w *bytes.Buffer, n *yaml.Node) error {
	switch n.ShortTag() {
	case "!!str", "!!timestamp":
		return writeString(w, n.Value)
	case "!!null":
		w.WriteString("null")
		return nil
	case "!!bool", "!!int", "!!float":
		if json.Valid([]byte(n.Value)) && n.Value != "" && n.Value[0] != '"' {
			w.WriteString(n.Value)
			return nil
		}
		var decoded any
		if err := n.Decode(&decoded); err != nil {
			return fmt.Errorf("line %d: %w", n.Line, err)
		}
		encoded, err := json.Marshal(decoded)
		if err != nil {
			return fmt.Errorf("line %d: %w", n.Line, err)
		}
		w.Write(encoded)
		return nil
	}
	return fmt.Errorf("line %d: the tag %s is not supported", n.Line, n.ShortTag())
}

func writeString(w *bytes.Buffer, s string) error {
	encoded, err := json.Marshal(s)
	if err != nil {
		return err
	}
	w.Write(encoded)
	return nil
}
