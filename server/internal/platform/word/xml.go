package word

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	wordNamespace       = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	strictWordNamespace = "http://purl.oclc.org/ooxml/wordprocessingml/main"
	contentNamespace    = "http://schemas.openxmlformats.org/package/2006/content-types"
	relationNamespace   = "http://schemas.openxmlformats.org/package/2006/relationships"
)

type element struct {
	name     xml.Name
	attrs    []xml.Attr
	text     strings.Builder
	children []*element
	path     string
	counts   map[xml.Name]int
}

type xmlBudget struct {
	remaining int
	bytes     int64
	locators  int64
}

func parseXML(ctx context.Context, data []byte, budget *xmlBudget, depthLimit int) (*element, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var root *element
	var stack []*element
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: invalid XML", ErrInvalidPackage)
		}
		switch token := token.(type) {
		case xml.StartElement:
			n, err := pushElement(token, stack, budget, depthLimit)
			if err != nil {
				return nil, err
			}
			if len(stack) == 0 {
				if root != nil {
					return nil, fmt.Errorf("%w: multiple XML roots", ErrInvalidPackage)
				}
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("%w: XML nesting", ErrInvalidPackage)
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(token)
			} else if strings.TrimSpace(string(token)) != "" {
				return nil, fmt.Errorf("%w: text outside XML root", ErrInvalidPackage)
			}
		case xml.Directive:
			return nil, fmt.Errorf("%w: XML directives are not permitted", ErrInvalidPackage)
		case xml.ProcInst:
			if token.Target != "xml" || root != nil {
				return nil, fmt.Errorf("%w: XML processing instruction", ErrInvalidPackage)
			}
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, fmt.Errorf("%w: incomplete XML", ErrInvalidPackage)
	}
	return root, nil
}

func pushElement(token xml.StartElement, stack []*element, budget *xmlBudget, depthLimit int) (*element, error) {
	budget.remaining--
	if budget.remaining < 0 || len(stack) >= depthLimit {
		return nil, fmt.Errorf("%w: XML complexity", ErrLimit)
	}
	seen := make(map[xml.Name]struct{}, len(token.Attr))
	for _, attr := range token.Attr {
		if _, duplicate := seen[attr.Name]; duplicate {
			return nil, fmt.Errorf("%w: duplicate XML attribute", ErrInvalidPackage)
		}
		seen[attr.Name] = struct{}{}
	}
	n := &element{name: token.Name, attrs: token.Attr, counts: make(map[xml.Name]int)}
	length := int64(len(token.Name.Space)) + int64(len(token.Name.Local)) + 6
	var parent *element
	ordinal := "1"
	if len(stack) > 0 {
		parent = stack[len(stack)-1]
		parent.counts[token.Name]++
		ordinal = strconv.Itoa(parent.counts[token.Name])
		length += int64(len(parent.path)) + int64(len(ordinal)) - 1
	}
	budget.locators -= length
	if budget.locators < 0 {
		return nil, fmt.Errorf("%w: locator bytes", ErrLimit)
	}
	identity := "{" + token.Name.Space + "}" + token.Name.Local
	if len(stack) == 0 {
		n.path = "/" + identity + "[1]"
		return n, nil
	}
	n.path = parent.path + "/" + identity + "[" + ordinal + "]"
	parent.children = append(parent.children, n)
	return n, nil
}

func (e *element) word(local string) bool {
	return e.name.Local == local && (e.name.Space == wordNamespace || e.name.Space == strictWordNamespace)
}

func (e *element) attr(local string) string {
	for _, attr := range e.attrs {
		if attr.Name.Local == local && (attr.Name.Space == "" || attr.Name.Space == wordNamespace || attr.Name.Space == strictWordNamespace) {
			return attr.Value
		}
	}
	return ""
}

func (e *element) child(local string) *element {
	for _, child := range e.children {
		if child.word(local) {
			return child
		}
	}
	return nil
}
