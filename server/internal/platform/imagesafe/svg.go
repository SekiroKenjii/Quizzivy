package imagesafe

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ErrSVGNotAllowed is why CheckSVG refuses an SVG that is well formed: a
// DOCTYPE or entity, a processing instruction, a root other than svg, an element
// outside the allowlist, an event attribute, a reference that leaves the
// document, or a shape a renderer could be made to expand without bound, which
// is also ErrSVGTooComplex.
var ErrSVGNotAllowed = errors.New("imagesafe: svg holds an element, attribute or reference that is not allowed")

// ErrSVGTooComplex is why CheckSVG refuses an SVG whose elements are all
// allowed but whose shape a renderer could be made to expand without bound:
// more than maxSVGElements elements, nesting deeper than maxSVGDepth, or a use
// that references an element holding a use. It is returned together with
// ErrSVGNotAllowed, so a caller that does not tell the two apart still sees a
// refused SVG.
var ErrSVGTooComplex = errors.New("imagesafe: svg is too complex to draw safely")

const (
	svgNamespace = "http://www.w3.org/2000/svg"
	xmlNamespace = "http://www.w3.org/XML/1998/namespace"
	spaceChars   = " \t\r\n\f"
	maxSVGSide   = math.MaxInt32

	maxEscapeDigits = 6

	maxSVGElements = 10000
	maxSVGDepth    = 64

	elementSVG   = "svg"
	elementStyle = "style"
	attrHref     = "href"
	attrWidth    = "width"
	attrHeight   = "height"
	attrID       = "id"
	elementUse   = "use"
)

var utf8BOM = []byte("\xef\xbb\xbf")

var svgElements = map[string]bool{
	elementSVG: true, "g": true, "path": true, "rect": true, "circle": true, "ellipse": true,
	"line": true, "polyline": true, "polygon": true, "defs": true, "linearGradient": true,
	"radialGradient": true, "stop": true, "clipPath": true, "mask": true, "symbol": true,
	elementUse: true, "title": true, "desc": true, "text": true, "tspan": true, elementStyle: true,
}

var svgNumber = regexp.MustCompile(`^[+-]?(?:[0-9]+\.?[0-9]*|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

var errSVGSize = fmt.Errorf("%w: the svg has no square viewBox, or width and height in pixels", ErrDimensions)

// CheckSVG reads at most maxBytes from body and returns it unchanged, with the
// width and height of its drawing rounded to whole pixels, at least 1, if it is
// an SVG that can neither run script nor reach anything beyond itself.
//
// The document is read token by token with encoding/xml, which expands no
// entity, and is refused with ErrSVGNotAllowed for a DOCTYPE, entity or any
// directive; a processing instruction other than the leading XML declaration; a
// root other than svg, or a second root; an element outside the allowlist or in
// a namespace other than SVG's; an attribute whose name begins with "on"; an
// href, in any namespace, that does not begin with "#"; xml:base; and a CSS
// value, in a style element or in any attribute, that holds @import or a url()
// whose argument does not begin with "#". Comments and escapes in the CSS are
// looked through, so a split or escaped keyword is refused as well.
//
// A renderer expands every use, so a chain of them multiplies: a file of a
// kilobyte whose uses reference groups of uses draws billions of instances. A
// use may therefore not reference a use, or an element that holds one, which
// also refuses a use that references its own ancestor. The document may hold at
// most maxSVGElements elements nested at most maxSVGDepth deep.
//
// The size is the viewBox's width and height, or, without a viewBox, the root's
// width and height attributes in pixels. It fails with ErrTooLarge,
// ErrUnreadable if the bytes are not well-formed UTF-8 XML, ErrSVGNotAllowed,
// and ErrDimensions if there is no size or the sides differ by more than 2% of
// the longer one. A failure to read body is returned wrapped.
func CheckSVG(body io.Reader, maxBytes int64) (data []byte, width, height int, err error) {
	data, err = readBounded(body, maxBytes)
	if err != nil {
		return nil, 0, 0, err
	}
	if !utf8.Valid(data) {
		return nil, 0, 0, fmt.Errorf("%w: the svg is not utf-8", ErrUnreadable)
	}
	root, err := auditSVG(bytes.TrimPrefix(data, utf8BOM))
	if err != nil {
		return nil, 0, 0, err
	}
	w, h, err := svgSize(root)
	if err != nil {
		return nil, 0, 0, err
	}
	if !nearSquare(w, h) {
		return nil, 0, 0, fmt.Errorf("%w: the svg is %g by %g, not square", ErrDimensions, w, h)
	}
	return data, sideOf(w), sideOf(h), nil
}

type svgWalker struct {
	decoder    *xml.Decoder
	depth      int
	rooted     bool
	rootAttrs  []xml.Attr
	styleDepth int
	css        strings.Builder
	elements   int
	openIDs    []string
	holdsUse   map[string]bool
	useTargets []string
}

func auditSVG(doc []byte) ([]xml.Attr, error) {
	walker := svgWalker{decoder: xml.NewDecoder(bytes.NewReader(doc)), holdsUse: map[string]bool{}}
	walker.decoder.Strict = true
	for first := true; ; first = false {
		token, err := walker.decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %.120s", ErrUnreadable, err)
		}
		if err := walker.accept(token, first); err != nil {
			return nil, err
		}
	}
	if !walker.rooted {
		return nil, fmt.Errorf("%w: the svg has no root element", ErrUnreadable)
	}
	if err := walker.checkUses(); err != nil {
		return nil, err
	}
	return walker.rootAttrs, nil
}

func (w *svgWalker) accept(token xml.Token, first bool) error {
	switch t := token.(type) {
	case xml.StartElement:
		return w.start(t)
	case xml.EndElement:
		return w.end(t)
	case xml.CharData:
		return w.text(t)
	case xml.Comment:
		return nil
	case xml.ProcInst:
		if first && t.Target == "xml" {
			return nil
		}
		return notAllowed("processing instruction", t.Target)
	default:
		return notAllowed("directive", "")
	}
}

func (w *svgWalker) start(el xml.StartElement) error {
	if w.depth == 0 && w.rooted {
		return notAllowed("second root element", el.Name.Local)
	}
	if !allowedElement(el.Name) {
		return notAllowed("element", el.Name.Local)
	}
	if w.depth == 0 {
		if el.Name.Local != elementSVG {
			return notAllowed("root element", el.Name.Local)
		}
		w.rooted, w.rootAttrs = true, el.Attr
	}
	if w.elements++; w.elements > maxSVGElements {
		return tooComplex("more elements than", strconv.Itoa(maxSVGElements))
	}
	if w.depth >= maxSVGDepth {
		return tooComplex("nesting deeper than", strconv.Itoa(maxSVGDepth))
	}
	for _, attr := range el.Attr {
		if err := checkAttribute(attr); err != nil {
			return err
		}
	}
	w.trackUse(el)
	if el.Name.Local == elementStyle {
		w.styleDepth++
	}
	w.depth++
	return nil
}

func (w *svgWalker) trackUse(el xml.StartElement) {
	id, _ := plainAttribute(el.Attr, attrID)
	w.openIDs = append(w.openIDs, id)
	if el.Name.Local != elementUse {
		return
	}
	for _, open := range w.openIDs {
		if open != "" {
			w.holdsUse[open] = true
		}
	}
	for _, attr := range el.Attr {
		if strings.ToLower(attr.Name.Local) == attrHref {
			w.useTargets = append(w.useTargets, strings.TrimPrefix(strings.Trim(attr.Value, spaceChars), "#"))
		}
	}
}

func (w *svgWalker) checkUses() error {
	for _, target := range w.useTargets {
		if w.holdsUse[target] {
			return tooComplex("use of an element that holds a use", target)
		}
	}
	return nil
}

func (w *svgWalker) end(el xml.EndElement) error {
	w.depth--
	w.openIDs = w.openIDs[:len(w.openIDs)-1]
	if el.Name.Local != elementStyle {
		return nil
	}
	w.styleDepth--
	if w.styleDepth > 0 {
		return nil
	}
	css := w.css.String()
	w.css.Reset()
	return checkCSS(css)
}

func (w *svgWalker) text(data xml.CharData) error {
	if w.styleDepth > 0 {
		w.css.Write(data)
		return nil
	}
	if w.depth == 0 && len(bytes.Trim(data, spaceChars)) > 0 {
		return fmt.Errorf("%w: text outside the root element", ErrUnreadable)
	}
	return nil
}

func allowedElement(name xml.Name) bool {
	return (name.Space == "" || name.Space == svgNamespace) && svgElements[name.Local]
}

func checkAttribute(attr xml.Attr) error {
	name := strings.ToLower(attr.Name.Local)
	switch {
	case strings.HasPrefix(name, "on"):
		return notAllowed("event attribute", attr.Name.Local)
	case name == attrHref && !fragmentReference(attr.Value):
		return notAllowed("reference beyond the document", attr.Value)
	case name == "base" && attr.Name.Space == xmlNamespace:
		return notAllowed("base URI", attr.Value)
	}
	return checkCSS(attr.Value)
}

func fragmentReference(value string) bool {
	return strings.HasPrefix(strings.TrimLeft(value, spaceChars), "#")
}

func quotedFragmentReference(argument string) bool {
	argument = strings.TrimLeft(argument, spaceChars)
	if argument != "" && (argument[0] == '"' || argument[0] == '\'') {
		argument = argument[1:]
	}
	return fragmentReference(argument)
}

func checkCSS(css string) error {
	bare := stripCSSComments(css)
	for _, form := range []string{css, bare, unescapeCSS(css), unescapeCSS(bare)} {
		if err := checkCSSForm(strings.ToLower(form)); err != nil {
			return err
		}
	}
	return nil
}

func checkCSSForm(css string) error {
	if strings.Contains(css, "@import") {
		return notAllowed("@import", css)
	}
	for rest := css; ; {
		_, after, found := strings.Cut(rest, "url(")
		if !found {
			return nil
		}
		if !quotedFragmentReference(after) {
			return notAllowed("url() beyond the document", after)
		}
		rest = after
	}
}

func stripCSSComments(css string) string {
	if !strings.Contains(css, "/*") {
		return css
	}
	var out strings.Builder
	for {
		before, rest, found := strings.Cut(css, "/*")
		out.WriteString(before)
		if !found {
			return out.String()
		}
		if _, css, found = strings.Cut(rest, "*/"); !found {
			return out.String()
		}
	}
}

func unescapeCSS(css string) string {
	if !strings.Contains(css, `\`) {
		return css
	}
	var out strings.Builder
	for i := 0; i < len(css); {
		if css[i] != '\\' {
			out.WriteByte(css[i])
			i++
			continue
		}
		i = writeEscape(&out, css, i+1)
	}
	return out.String()
}

func writeEscape(out *strings.Builder, css string, i int) int {
	if i >= len(css) {
		return i
	}
	switch c := css[i]; {
	case c == '\n' || c == '\r' || c == '\f':
		return i + 1
	case !isHexDigit(c):
		r, size := utf8.DecodeRuneInString(css[i:])
		out.WriteRune(r)
		return i + size
	}
	code, end := 0, i
	for ; end < len(css) && end-i < maxEscapeDigits && isHexDigit(css[end]); end++ {
		code = code*16 + hexValue(css[end])
	}
	if code == 0 || !utf8.ValidRune(rune(code)) {
		code = utf8.RuneError
	}
	out.WriteRune(rune(code))
	if end < len(css) && strings.IndexByte(spaceChars, css[end]) >= 0 {
		if css[end] == '\r' && end+1 < len(css) && css[end+1] == '\n' {
			end++
		}
		end++
	}
	return end
}

func isHexDigit(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func hexValue(c byte) int {
	switch {
	case c >= 'a':
		return int(c-'a') + 10
	case c >= 'A':
		return int(c-'A') + 10
	default:
		return int(c - '0')
	}
}

func svgSize(root []xml.Attr) (width, height float64, err error) {
	if box, ok := plainAttribute(root, "viewBox"); ok {
		return viewBoxSize(box)
	}
	width, widthOK := pixelLength(plainAttribute(root, attrWidth))
	height, heightOK := pixelLength(plainAttribute(root, attrHeight))
	if !widthOK || !heightOK {
		return 0, 0, errSVGSize
	}
	return width, height, nil
}

func plainAttribute(attrs []xml.Attr, name string) (string, bool) {
	for _, attr := range attrs {
		if attr.Name.Space == "" && attr.Name.Local == name {
			return attr.Value, true
		}
	}
	return "", false
}

func viewBoxSize(box string) (width, height float64, err error) {
	fields := strings.FieldsFunc(box, func(r rune) bool { return r == ',' || strings.ContainsRune(spaceChars, r) })
	if len(fields) != 4 {
		return 0, 0, errSVGSize
	}
	var numbers [4]float64
	for i, field := range fields {
		number, ok := parseNumber(field)
		if !ok {
			return 0, 0, errSVGSize
		}
		numbers[i] = number
	}
	if numbers[2] <= 0 || numbers[3] <= 0 || numbers[2] > maxSVGSide || numbers[3] > maxSVGSide {
		return 0, 0, errSVGSize
	}
	return numbers[2], numbers[3], nil
}

func pixelLength(value string, present bool) (float64, bool) {
	if !present {
		return 0, false
	}
	number, ok := parseNumber(strings.TrimSuffix(strings.Trim(value, spaceChars), "px"))
	return number, ok && number > 0 && number <= maxSVGSide
}

func parseNumber(s string) (float64, bool) {
	if !svgNumber.MatchString(s) {
		return 0, false
	}
	number, err := strconv.ParseFloat(s, 64)
	return number, err == nil
}

func sideOf(length float64) int {
	return max(1, int(math.Round(length)))
}

func tooComplex(what, subject string) error {
	return fmt.Errorf("%w: %s %.40q: %w", ErrSVGTooComplex, what, subject, ErrSVGNotAllowed)
}

func notAllowed(what, subject string) error {
	return fmt.Errorf("%w: %s %.40q", ErrSVGNotAllowed, what, subject)
}
