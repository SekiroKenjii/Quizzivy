package imagesafe_test

import (
	"bytes"
	"errors"
	"fmt"
	"quizzivy/internal/platform/imagesafe"
	"strings"
	"testing"
)

const svgLimit = 256 << 10

func checkSVG(doc string) ([]byte, int, int, error) {
	return imagesafe.CheckSVG(strings.NewReader(doc), svgLimit)
}

func inSVG(inner string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 24 24">` + inner + `</svg>`
}

func inSVGWithAttrs(attrs string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 24 24" ` + attrs + `/>`
}

const illustratorExport = `<?xml version="1.0" encoding="utf-8"?>
<!-- Generator: Adobe Illustrator 27.1.1, SVG Export Plug-In . SVG Version: 6.00 Build 0)  -->
<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
	 viewBox="0 0 512 512" style="enable-background:new 0 0 512 512;" xml:space="preserve">
<title>Quizzivy</title>
<desc>The logo of the centre</desc>
<style type="text/css">
	.st0{fill:url(#SVGID_1_);}
	.st1{fill:#FFFFFF;stroke:#1D1D1B;stroke-width:4;}
	.st2{clip-path:url(#SVGID_3_);fill:url('#SVGID_2_');}
	.label{font-family:'Be Vietnam Pro',sans-serif;font-size:32px;}
</style>
<defs>
	<path id="a" d="M256,32C132.3,32,32,132.3,32,256s100.3,224,224,224s224-100.3,224-224S379.7,32,256,32z"/>
	<linearGradient id="SVGID_1_" gradientUnits="userSpaceOnUse" x1="32" y1="32" x2="480" y2="480">
		<stop offset="0" style="stop-color:#00AEEF"/>
		<stop offset="1" style="stop-color:#2E3192"/>
	</linearGradient>
	<radialGradient id="SVGID_2_" xlink:href="#SVGID_1_" cx="256" cy="256" r="224" gradientUnits="userSpaceOnUse"/>
	<clipPath id="SVGID_3_"><use xlink:href="#a" style="overflow:visible;"/></clipPath>
	<mask id="m"><rect width="512" height="512" fill="#FFFFFF"/></mask>
	<symbol id="s" viewBox="0 0 10 10"><circle cx="5" cy="5" r="4"/></symbol>
</defs>
<g mask="url(#m)">
	<use xlink:href="#a" class="st0"/>
	<use href="#s" x="100" y="100" width="40" height="40"/>
	<ellipse class="st1" cx="256" cy="256" rx="100" ry="80"/>
	<line x1="0" y1="0" x2="512" y2="512" stroke="url(#SVGID_1_)"/>
	<polyline points="0,0 10,10 20,0" fill="none"/>
	<polygon points="0,0 10,10 20,0" fill="url( &quot;#SVGID_1_&quot; )"/>
	<text class="label" x="40" y="60"><tspan>Quizzivy</tspan> &amp; Co</text>
</g>
</svg>
`

const figmaExport = `<svg width="48" height="48" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg">
<g clip-path="url(#clip0_12_34)">
<rect width="48" height="48" rx="12" fill="url(#paint0_linear_12_34)"/>
<path d="M14 24L22 32L34 16" stroke="white" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"/>
</g>
<defs>
<linearGradient id="paint0_linear_12_34" x1="0" y1="0" x2="48" y2="48" gradientUnits="userSpaceOnUse">
<stop stop-color="#0F766E"/>
<stop offset="1" stop-color="#134E4A"/>
</linearGradient>
<clipPath id="clip0_12_34">
<rect width="48" height="48" fill="white"/>
</clipPath>
</defs>
</svg>
`

func TestAnSVGFromADesignToolIsReturnedAsItCameInWithItsSize(t *testing.T) {
	cases := map[string]struct {
		doc           string
		width, height int
	}{
		"illustrator":           {illustratorExport, 512, 512},
		"figma":                 {figmaExport, 48, 48},
		"a byte order mark":     {"\xef\xbb\xbf" + figmaExport, 48, 48},
		"windows line ends":     {strings.ReplaceAll(illustratorExport, "\n", "\r\n"), 512, 512},
		"no declaration at all": {`<svg viewBox="0 0 64 64"><circle cx="32" cy="32" r="30"/></svg>`, 64, 64},
	}
	for name, c := range cases {
		out, width, height, err := checkSVG(c.doc)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if string(out) != c.doc {
			t.Errorf("%s: the bytes were changed", name)
		}
		if width != c.width || height != c.height {
			t.Errorf("%s: answered %d x %d, want %d x %d", name, width, height, c.width, c.height)
		}
	}
}

func TestTheSizeOfAnSVGComesFromTheViewBoxAndElseFromItsWidthAndHeight(t *testing.T) {
	cases := []struct {
		name          string
		root          string
		width, height int
	}{
		{"a viewBox", `viewBox="0 0 24 24"`, 24, 24},
		{"a viewBox with commas", `viewBox="0,0,32,32"`, 32, 32},
		{"a viewBox with a lot of space", "viewBox=\"  -4\t-4\n 40   40 \"", 40, 40},
		{"a viewBox that wins over the width and height", `viewBox="0 0 24 24" width="100" height="100"`, 24, 24},
		{"fractions rounded", `viewBox="0 0 24.4 24.4"`, 24, 24},
		{"fractions rounded up", `viewBox="0 0 24.5 24.5"`, 25, 25},
		{"a size under one pixel is one", `viewBox="0 0 0.4 0.4"`, 1, 1},
		{"an exponent", `viewBox="0 0 1e2 1e2"`, 100, 100},
		{"unitless width and height", `width="48" height="48"`, 48, 48},
		{"pixel width and height", `width="48px" height="48px"`, 48, 48},
		{"within two percent", `viewBox="0 0 100 102"`, 100, 102},
		{"exactly two percent", `viewBox="0 0 100 98"`, 100, 98},
	}
	for _, c := range cases {
		doc := `<svg xmlns="http://www.w3.org/2000/svg" ` + c.root + `><path d="M0 0"/></svg>`
		_, width, height, err := checkSVG(doc)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if width != c.width || height != c.height {
			t.Errorf("%s: answered %d x %d, want %d x %d", c.name, width, height, c.width, c.height)
		}
	}
}

func TestAnSVGThatIsNotSquareOrHasNoSizeIsRefusedForItsDimensions(t *testing.T) {
	roots := map[string]string{
		"a wide viewBox":                      `viewBox="0 0 100 50"`,
		"a tall viewBox":                      `viewBox="0 0 50 100"`,
		"just over two percent":               `viewBox="0 0 100 97"`,
		"a viewBox of three numbers":          `viewBox="0 0 24"`,
		"a viewBox of five numbers":           `viewBox="0 0 24 24 24"`,
		"a viewBox that is not numbers":       `viewBox="0 0 a b"`,
		"a viewBox of zero":                   `viewBox="0 0 0 0"`,
		"a negative viewBox":                  `viewBox="0 0 -24 -24"`,
		"an infinite viewBox":                 `viewBox="0 0 1e999 1e999"`,
		"a viewBox beyond any integer":        `viewBox="0 0 1e30 1e30"`,
		"an empty viewBox":                    `viewBox=""`,
		"a malformed viewBox beside a width":  `viewBox="x" width="24" height="24"`,
		"no size at all":                      ``,
		"only a width":                        `width="24"`,
		"a wide width and height":             `width="100" height="50"`,
		"percent width and height":            `width="100%" height="100%"`,
		"width and height in millimetres":     `width="24mm" height="24mm"`,
		"width and height of zero":            `width="0" height="0"`,
		"a number with a sign and nothing":    `width="+" height="+"`,
		"a hexadecimal number":                `width="0x18" height="0x18"`,
		"a number with an underscore":         `width="2_4" height="2_4"`,
		"a viewBox on a nested svg only":      ``,
		"a size with the wrong case of attrs": `Width="24" Height="24"`,
	}
	for name, root := range roots {
		inner := `<path d="M0 0"/>`
		if name == "a viewBox on a nested svg only" {
			inner = `<svg viewBox="0 0 24 24"/>`
		}
		doc := `<svg xmlns="http://www.w3.org/2000/svg" ` + root + `>` + inner + `</svg>`
		if _, _, _, err := checkSVG(doc); !errors.Is(err, imagesafe.ErrDimensions) {
			t.Errorf("%s answered %v, want ErrDimensions", name, err)
		}
	}
}

func TestAnSVGWithScriptOrAnythingOutsideTheAllowlistIsRefused(t *testing.T) {
	cases := map[string]string{
		"a script":                            inSVG(`<script>alert(1)</script>`),
		"a script with CDATA":                 inSVG(`<script><![CDATA[alert(1)]]></script>`),
		"a script in the html space":          `<svg xmlns="http://www.w3.org/2000/svg" xmlns:html="http://www.w3.org/1999/xhtml" viewBox="0 0 24 24"><html:script>alert(1)</html:script></svg>`,
		"a script under an undeclared prefix": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><x:script>alert(1)</x:script></svg>`,
		"a foreignObject":                     inSVG(`<foreignObject width="10" height="10"><div xmlns="http://www.w3.org/1999/xhtml">hi</div></foreignObject>`),
		"an html default namespace":           inSVG(`<g xmlns="http://www.w3.org/1999/xhtml"><rect/></g>`),
		"an image":                            inSVG(`<image href="#a" width="1" height="1"/>`),
		"an anchor":                           inSVG(`<a href="#a"><rect width="1" height="1"/></a>`),
		"an animation":                        inSVG(`<rect id="r" width="1" height="1"><animate attributeName="width" to="2"/></rect>`),
		"a set":                               inSVG(`<set attributeName="href" to="javascript:alert(1)"/>`),
		"a filter":                            inSVG(`<filter id="f"><feImage href="#a"/></filter>`),
		"an iframe":                           inSVG(`<iframe src="https://example.com"/>`),
		"a pattern":                           inSVG(`<pattern id="p"/>`),
		"a marker":                            inSVG(`<marker id="m"/>`),
		"a switch":                            inSVG(`<switch/>`),
		"the wrong case of an element":        inSVG(`<Rect width="1" height="1"/>`),
		"a nested script":                     inSVG(`<g><g><g><script>alert(1)</script></g></g></g>`),
		"a script after the root":             inSVG(``) + `<script>alert(1)</script>`,
	}
	for name, doc := range cases {
		if _, _, _, err := checkSVG(doc); !errors.Is(err, imagesafe.ErrSVGNotAllowed) {
			t.Errorf("%s answered %v, want ErrSVGNotAllowed", name, err)
		}
	}
}

func TestAnSVGWithAnEventAttributeIsRefused(t *testing.T) {
	cases := map[string]string{
		"onload on the root": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" onload="alert(1)"/>`,
		"onclick on a path":  inSVG(`<path d="M0 0" onclick="alert(1)"/>`),
		"the wrong case":     inSVG(`<path d="M0 0" OnMouseOver="alert(1)"/>`),
		"all capitals":       inSVG(`<path d="M0 0" ONLOAD="alert(1)"/>`),
		"onbegin":            inSVG(`<g onbegin="alert(1)"/>`),
		"a prefixed handler": inSVG(`<g xlink:onload="alert(1)"/>`),
		"a handler under a prefix bound to xmlns": `<svg xmlns="http://www.w3.org/2000/svg" xmlns:x="xmlns" viewBox="0 0 24 24" x:onclick="alert(1)"/>`,
		"a handler with no value":                 inSVG(`<g onfocus=""/>`),
		"a handler on a deep child":               inSVG(`<g><g><g><text onerror="alert(1)">x</text></g></g></g>`),
		"a handler on the last":                   inSVGWithAttrs(`id="a" class="b" onload="alert(1)"`),
	}
	for name, doc := range cases {
		if _, _, _, err := checkSVG(doc); !errors.Is(err, imagesafe.ErrSVGNotAllowed) {
			t.Errorf("%s answered %v, want ErrSVGNotAllowed", name, err)
		}
	}
}

func TestAnSVGReferenceMustStayInsideTheDocument(t *testing.T) {
	refused := map[string]string{
		"an external href":                      inSVG(`<use href="https://example.com/x.svg#a"/>`),
		"an external xlink:href":                inSVG(`<use xlink:href="https://example.com/x.svg#a"/>`),
		"a protocol-relative href":              inSVG(`<use href="//example.com/x.svg#a"/>`),
		"a relative href":                       inSVG(`<use href="other.svg#a"/>`),
		"a javascript href":                     inSVG(`<use href="javascript:alert(1)"/>`),
		"a data href":                           inSVG(`<use href="data:image/svg+xml;base64,PHN2Zy8+"/>`),
		"an empty href":                         inSVG(`<use href=""/>`),
		"an href with a space before":           inSVG(`<use href=" https://example.com/#a"/>`),
		"an href with a no-break space":         inSVG("<use href=\" #a\"/>"),
		"an href in the wrong case":             inSVG(`<use HREF="https://example.com/x.svg#a"/>`),
		"an href under another prefix":          `<svg xmlns="http://www.w3.org/2000/svg" xmlns:x="urn:x" viewBox="0 0 24 24"><use x:href="https://example.com/#a"/></svg>`,
		"an href under no declaration":          `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><use xlink:href="https://example.com/#a"/></svg>`,
		"a gradient href":                       inSVG(`<linearGradient xlink:href="https://example.com/g.svg#a"/>`),
		"an href under a prefix bound to xmlns": `<svg xmlns="http://www.w3.org/2000/svg" xmlns:x="xmlns" viewBox="0 0 24 24"><use x:href="https://example.com/#a"/></svg>`,
		"an xml:base":                           inSVG(`<g xml:base="https://example.com/"><use href="#a"/></g>`),
		"an xml:base on the root":               `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" xml:base="https://example.com/"/>`,
	}
	for name, doc := range refused {
		if _, _, _, err := checkSVG(doc); !errors.Is(err, imagesafe.ErrSVGNotAllowed) {
			t.Errorf("%s answered %v, want ErrSVGNotAllowed", name, err)
		}
	}
	accepted := map[string]string{
		"an href to a fragment":                  inSVG(`<path id="a" d="M0 0"/><use href="#a"/>`),
		"an xlink:href to a fragment":            inSVG(`<path id="a" d="M0 0"/><use xlink:href="#a"/>`),
		"an href with space around the fragment": inSVG(`<path id="a" d="M0 0"/><use href=" #a "/>`),
		"a fragment written as a character ref":  inSVG(`<path id="a" d="M0 0"/><use href="&#35;a"/>`),
		"xml:space":                              inSVG(`<text xml:space="preserve">a</text>`),
	}
	for name, doc := range accepted {
		if _, _, _, err := checkSVG(doc); err != nil {
			t.Errorf("%s answered %v, want success", name, err)
		}
	}
}

func TestAStyleThatImportsOrLoadsAnythingIsRefused(t *testing.T) {
	cases := map[string]string{
		"an @import in a style element":              inSVG(`<style>@import url(https://example.com/a.css);</style>`),
		"an @import of a string":                     inSVG(`<style>@import "https://example.com/a.css";</style>`),
		"an @import in CDATA":                        inSVG(`<style><![CDATA[@import 'https://example.com/a.css';]]></style>`),
		"an @import in the wrong case":               inSVG(`<style>@IMPORT "https://example.com/a.css";</style>`),
		"an @import split by a comment":              inSVG(`<style>@im/**/port "https://example.com/a.css";</style>`),
		"an @import with a CSS escape":               inSVG(`<style>@\69mport "https://example.com/a.css";</style>`),
		"an @import with an escaped at sign":         inSVG(`<style>\40import "https://example.com/a.css";</style>`),
		"an @import split by sections":               inSVG(`<style>@imp<![CDATA[ort]]> "https://example.com/a.css";</style>`),
		"an @import split by an XML comment":         inSVG(`<style>@imp<!-- x -->ort "https://example.com/a.css";</style>`),
		"an @import after a first style":             inSVG(`<style>.a{fill:red}</style><style>@import "https://example.com/a.css";</style>`),
		"an @import in a style attribute":            inSVG(`<g style="@import 'https://example.com/a.css'"/>`),
		"a url in a style attribute":                 inSVG(`<g style="fill:url(http://example.com/x.svg#g)"/>`),
		"a quoted url in a style attribute":          inSVG(`<g style="fill:url('https://example.com/x.svg#g')"/>`),
		"a double-quoted url":                        inSVG(`<g style='fill:url("https://example.com/x.svg#g")'/>`),
		"a protocol-relative url":                    inSVG(`<g style="fill:url(//example.com/x.svg#g)"/>`),
		"a url with space in front":                  inSVG(`<g style="fill:url(  https://example.com/x.svg#g  )"/>`),
		"a relative url":                             inSVG(`<g style="fill:url(x.svg#g)"/>`),
		"a data url":                                 inSVG(`<g style="fill:url(data:image/png;base64,AAAA)"/>`),
		"the wrong case of url":                      inSVG(`<g style="fill:URL(https://example.com/x.svg#g)"/>`),
		"an escaped url":                             inSVG(`<g style="fill:\75rl(https://example.com/x.svg#g)"/>`),
		"an escaped url with a space":                inSVG(`<g style="fill:\75 rl(https://example.com/x.svg#g)"/>`),
		"an escaped scheme":                          inSVG(`<g style="fill:url(\68ttps://example.com/x.svg#g)"/>`),
		"a url split by a comment":                   inSVG(`<g style="fill:u/**/rl(https://example.com/x.svg#g)"/>`),
		"an escaped quote that is not a quote":       inSVG(`<g style="fill:url(\22#x.svg)"/>`),
		"a url hidden after a string with a comment": inSVG(`<g style="a:url(&quot;/*&quot;);fill:url(https://example.com/x.svg#g);b:url(&quot;*/&quot;)"/>`),
		"a second url that leaves":                   inSVG(`<style>.a{fill:url(#g)} .b{fill:url(https://example.com/x.svg#g)}</style>`),
		"a font from elsewhere":                      inSVG(`<style>@font-face{font-family:x;src:url(https://example.com/x.woff)}</style>`),
		"a url in a presentation attribute":          inSVG(`<rect width="1" height="1" fill="url(https://example.com/x.svg#g)"/>`),
		"a filter in a presentation attribute":       inSVG(`<rect width="1" height="1" filter="url(https://example.com/x.svg#f)"/>`),
		"a url in a mask attribute":                  inSVG(`<rect width="1" height="1" mask="url(x.svg#f)"/>`),
		"a url with a newline escape":                inSVG("<g style=\"fill:url(\\\n https://example.com/x.svg#g)\"/>"),
		"a fragment written with a CSS escape":       inSVG(`<style>.a{fill:url(\23 g)}</style>`),
		"a url without an argument":                  inSVG(`<g style="fill:url()"/>`),
	}
	for name, doc := range cases {
		if _, _, _, err := checkSVG(doc); !errors.Is(err, imagesafe.ErrSVGNotAllowed) {
			t.Errorf("%s answered %v, want ErrSVGNotAllowed", name, err)
		}
	}
}

func TestAStyleWithOnlyLocalUrlsIsAccepted(t *testing.T) {
	cases := map[string]string{
		"a class with a gradient":           inSVG(`<style>.a{fill:url(#g)}</style><linearGradient id="g"/><rect class="a" width="1" height="1"/>`),
		"quoted fragments":                  inSVG(`<style>.a{fill:url("#g")} .b{fill:url('#g')} .c{fill:url( "#g" )}</style><linearGradient id="g"/>`),
		"a media query":                     inSVG(`<style>@media (prefers-color-scheme: dark){.a{fill:#fff}}</style>`),
		"a keyframes block":                 inSVG(`<style>@keyframes k{from{opacity:0}to{opacity:1}}.a{animation:k 1s}</style>`),
		"a comment about an import":         inSVG(`<style>/* the sheet has no import */ .a{fill:red}</style>`),
		"a class that needs an escape":      inSVG(`<style>.a\:b{fill:red}.\31 0{fill:blue}</style>`),
		"a font by name":                    inSVG(`<style>.a{font-family:'Be Vietnam Pro',sans-serif}</style>`),
		"an empty style":                    inSVG(`<style/>`),
		"a style attribute with a url":      inSVG(`<g style="fill:url(#g);stroke:url(#g)"/><linearGradient id="g"/>`),
		"a url in a presentation attribute": inSVG(`<rect width="1" height="1" fill="url(#g)"/><linearGradient id="g"/>`),
		"a url in a clip-path":              inSVG(`<g clip-path="url(#c)"/><clipPath id="c"/>`),
		"text that says url":                inSVG(`<text>url(https://example.com) and @import</text>`),
		"a title that says url":             inSVG(`<title>url(https://example.com)</title>`),
		"a comment that says import":        inSVG(`<!-- @import url(https://example.com) -->`),
		"an on-prefix word that is not":     inSVG(`<g id="onion"/>`),
	}
	for name, doc := range cases {
		if _, _, _, err := checkSVG(doc); err != nil {
			t.Errorf("%s answered %v, want success", name, err)
		}
	}
}

func TestAnSVGWithADoctypeOrAnEntityIsRefusedWithoutExpandingIt(t *testing.T) {
	billion := `<?xml version="1.0"?>
<!DOCTYPE lolz [
 <!ENTITY lol "lol">
 <!ENTITY lol1 "&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;&lol;">
 <!ENTITY lol2 "&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;&lol1;">
 <!ENTITY lol3 "&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;&lol2;">
 <!ENTITY lol4 "&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;&lol3;">
 <!ENTITY lol5 "&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;&lol4;">
 <!ENTITY lol6 "&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;&lol5;">
 <!ENTITY lol7 "&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;&lol6;">
 <!ENTITY lol8 "&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;&lol7;">
 <!ENTITY lol9 "&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;&lol8;">
]>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><text>&lol9;</text></svg>`
	cases := map[string]string{
		"a billion laughs":                billion,
		"a plain doctype":                 `<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd">` + inSVG(``),
		"a doctype with a dtd":            `<!DOCTYPE svg [ <!ENTITY xxe SYSTEM "file:///etc/passwd"> ]>` + inSVG(`<text>&xxe;</text>`),
		"a doctype in the wrong case":     `<!doctype svg>` + inSVG(``),
		"an entity after the root begins": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><!ENTITY a "b"></svg>`,
		"an element declaration":          `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><!ELEMENT a ANY></svg>`,
	}
	for name, doc := range cases {
		if _, _, _, err := checkSVG(doc); !errors.Is(err, imagesafe.ErrSVGNotAllowed) {
			t.Errorf("%s answered %v, want ErrSVGNotAllowed", name, err)
		}
	}
}

func TestAProcessingInstructionOtherThanTheLeadingDeclarationIsRefused(t *testing.T) {
	cases := map[string]string{
		"a stylesheet":             `<?xml version="1.0"?><?xml-stylesheet href="https://example.com/a.css" type="text/css"?>` + inSVG(``),
		"a stylesheet first":       `<?xml-stylesheet href="https://example.com/a.css" type="text/css"?>` + inSVG(``),
		"a php block":              inSVG(`<?php echo 1; ?>`),
		"a declaration twice":      `<?xml version="1.0"?><?xml version="1.0"?>` + inSVG(``),
		"a declaration inside":     `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><?xml version="1.0"?></svg>`,
		"a declaration after text": "\n" + `<?xml version="1.0"?>` + inSVG(``),
		"the wrong case":           `<?XML version="1.0"?>` + inSVG(``),
	}
	for name, doc := range cases {
		if _, _, _, err := checkSVG(doc); !errors.Is(err, imagesafe.ErrSVGNotAllowed) {
			t.Errorf("%s answered %v, want ErrSVGNotAllowed", name, err)
		}
	}
}

func TestAnSVGWhoseRootIsNotAnSVGIsRefused(t *testing.T) {
	cases := map[string]string{
		"an html page":        `<html xmlns="http://www.w3.org/1999/xhtml"><body/></html>`,
		"a group":             `<g xmlns="http://www.w3.org/2000/svg"><rect/></g>`,
		"a path":              `<path xmlns="http://www.w3.org/2000/svg" d="M0 0"/>`,
		"a script":            `<script xmlns="http://www.w3.org/2000/svg">alert(1)</script>`,
		"an svg in xhtml":     `<svg xmlns="http://www.w3.org/1999/xhtml" viewBox="0 0 24 24"/>`,
		"an svg in a prefix":  `<s:svg xmlns:s="urn:something-else" viewBox="0 0 24 24"/>`,
		"an unknown root":     `<note><to>x</to></note>`,
		"the wrong case":      `<SVG xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"/>`,
		"two svgs":            inSVG(``) + inSVG(``),
		"an svg then an html": inSVG(``) + `<html xmlns="http://www.w3.org/1999/xhtml"/>`,
	}
	for name, doc := range cases {
		if _, _, _, err := checkSVG(doc); !errors.Is(err, imagesafe.ErrSVGNotAllowed) {
			t.Errorf("%s answered %v, want ErrSVGNotAllowed", name, err)
		}
	}
}

func TestAnSVGInAPrefixedSVGNamespaceIsAccepted(t *testing.T) {
	doc := `<s:svg xmlns:s="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><s:path d="M0 0"/></s:svg>`

	out, _, _, err := checkSVG(doc)

	if err != nil || string(out) != doc {
		t.Errorf("answered %v, want the document back", err)
	}
}

func TestADocumentThatIsNotWellFormedXMLIsUnreadable(t *testing.T) {
	cases := map[string]string{
		"nothing":                    ``,
		"only whitespace":            " \n ",
		"only a declaration":         `<?xml version="1.0"?>`,
		"only a comment":             `<!-- nothing -->`,
		"plain text":                 `hello`,
		"a png":                      "\x89PNG\r\n\x1a\n",
		"an unclosed root":           `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">`,
		"a mismatched end":           `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><g></svg>`,
		"an unquoted attribute":      `<svg xmlns="http://www.w3.org/2000/svg" viewBox=0 0 24 24/>`,
		"an undefined entity":        inSVG(`<text>&nbsp;</text>`),
		"an entity with no end":      inSVG(`<text>&amp</text>`),
		"a bare ampersand":           inSVG(`<text>a & b</text>`),
		"a less-than in a value":     inSVG(`<g id="a<b"/>`),
		"text after the root":        inSVG(``) + `hello`,
		"text before the root":       `hello` + inSVG(``),
		"another encoding":           `<?xml version="1.0" encoding="ISO-8859-1"?>` + inSVG(``),
		"an unknown xml version":     `<?xml version="2.0"?>` + inSVG(``),
		"a control character":        inSVG("<text>a\x01b</text>"),
		"invalid utf-8 in text":      inSVG("<text>a\xffb</text>"),
		"invalid utf-8 in a comment": inSVG("<!-- a\xffb -->"),
		"a lone surrogate":           inSVG("<text>a\xed\xa0\x80b</text>"),
		"utf-16":                     "\xff\xfe<\x00s\x00v\x00g\x00/\x00>\x00",
	}
	for name, doc := range cases {
		if _, _, _, err := checkSVG(doc); !errors.Is(err, imagesafe.ErrUnreadable) {
			t.Errorf("%s answered %v, want ErrUnreadable", name, err)
		}
	}
}

func TestAnSVGOverTheLimitIsRefusedAfterReadingNoMoreThanOneByteOverIt(t *testing.T) {
	body := &countingReader{r: endless{}}

	_, _, _, err := imagesafe.CheckSVG(body, svgLimit)

	if !errors.Is(err, imagesafe.ErrTooLarge) {
		t.Fatalf("answered %v, want ErrTooLarge", err)
	}
	if limit := svgLimit + 1; body.read > limit {
		t.Errorf("read %d bytes, want at most %d", body.read, limit)
	}
}

func TestAnSVGOfExactlyTheLimitIsAcceptedAndOneByteMoreIsNot(t *testing.T) {
	const head, tail = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><!--`, `--></svg>`
	exact := head + strings.Repeat("x", svgLimit-len(head)-len(tail)) + tail

	out, _, _, err := checkSVG(exact)
	if err != nil || len(out) != svgLimit {
		t.Errorf("a file of %d bytes answered %v, want success", len(exact), err)
	}
	if _, _, _, err := checkSVG(exact + " "); !errors.Is(err, imagesafe.ErrTooLarge) {
		t.Errorf("a file of %d bytes answered %v, want ErrTooLarge", len(exact)+1, err)
	}
}

func TestALongRunOfLocalReferencesIsAccepted(t *testing.T) {
	run := strings.Repeat("url(#a) ", 12000)
	doc := inSVG(`<linearGradient id="a"/><g style="` + run + `"/><style>` + run + `</style>`)

	_, _, _, err := checkSVG(doc)

	if err != nil {
		t.Errorf("answered %v, want success", err)
	}
}

func TestAReadThatFailsIsReturnedAsItIsAndNotAsATooLargeSVG(t *testing.T) {
	cause := errors.New("connection reset")

	_, _, _, err := imagesafe.CheckSVG(failingReader{cause}, svgLimit)

	if !errors.Is(err, cause) || errors.Is(err, imagesafe.ErrTooLarge) {
		t.Errorf("answered %v, want the read's own error", err)
	}
}

func TestACheckedSVGIsReturnedByteForByte(t *testing.T) {
	doc := fmt.Sprintf("%s\n<!-- %s -->\n", figmaExport, strings.Repeat("é", 50))

	out, _, _, err := checkSVG(doc)

	if err != nil || !bytes.Equal(out, []byte(doc)) {
		t.Errorf("answered %v and %d bytes, want the %d bytes it was given", err, len(out), len(doc))
	}
}
