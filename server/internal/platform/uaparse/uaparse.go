// Package uaparse classifies a User-Agent header into the system and browser
// the Signed-in devices list names. It answers only from closed lists: no part
// of the header is ever copied into a result, so a header written to look like
// a label, or to carry markup, cannot reach a screen.
package uaparse

import "strings"

// System is an operating system the list names.
type System string

// The systems Parse recognises.
const (
	SystemUnknown System = ""
	Mac           System = "Mac"
	Windows       System = "Windows"
	IPhone        System = "iPhone"
	IPad          System = "iPad"
	Android       System = "Android"
	Linux         System = "Linux"
	ChromeOS      System = "ChromeOS"
)

// Browser is a browser the list names.
type Browser string

// The browsers Parse recognises.
const (
	BrowserUnknown  Browser = ""
	Chrome          Browser = "Chrome"
	Safari          Browser = "Safari"
	Firefox         Browser = "Firefox"
	Edge            Browser = "Edge"
	Opera           Browser = "Opera"
	SamsungInternet Browser = "Samsung Internet"
)

// Kind is the sort of device, which picks an icon.
type Kind string

// The kinds Parse returns.
const (
	KindUnknown  Kind = "unknown"
	KindComputer Kind = "computer"
	KindPhone    Kind = "phone"
	KindTablet   Kind = "tablet"
)

const (
	separator = " · "
	maxBytes  = 512
)

// Systems lists every System Parse can return besides SystemUnknown.
func Systems() []System {
	return []System{Mac, Windows, IPhone, IPad, Android, Linux, ChromeOS}
}

// Browsers lists every Browser Parse can return besides BrowserUnknown.
func Browsers() []Browser {
	return []Browser{Chrome, Safari, Firefox, Edge, Opera, SamsungInternet}
}

// Device is what a user agent says about the machine that sent it. A field
// the header does not settle is the zero value, or KindUnknown.
type Device struct {
	System  System
	Browser Browser
	Kind    Kind
}

// Parse classifies userAgent. Only its first 512 bytes are read. It never
// fails: a header it does not understand yields a Device with nothing known.
func Parse(userAgent string) Device {
	if len(userAgent) > maxBytes {
		userAgent = userAgent[:maxBytes]
	}
	system := systemOf(userAgent)
	return Device{System: system, Browser: browserOf(userAgent), Kind: kindOf(system, userAgent)}
}

// Label is the name shown for the device: "Mac · Chrome", "Mac" when the
// browser is unknown, "Chrome" when the system is, and "" when neither is.
func (d Device) Label() string {
	switch {
	case d.System != SystemUnknown && d.Browser != BrowserUnknown:
		return string(d.System) + separator + string(d.Browser)
	case d.System != SystemUnknown:
		return string(d.System)
	default:
		return string(d.Browser)
	}
}

func systemOf(ua string) System {
	switch {
	case strings.Contains(ua, "iPhone"):
		return IPhone
	case strings.Contains(ua, "iPad"):
		return IPad
	case strings.Contains(ua, "Android"):
		return Android
	case strings.Contains(ua, "CrOS"):
		return ChromeOS
	case strings.Contains(ua, "Windows"):
		return Windows
	case strings.Contains(ua, "Macintosh"), strings.Contains(ua, "Mac OS X"):
		return Mac
	case strings.Contains(ua, "Linux"), strings.Contains(ua, "X11"):
		return Linux
	default:
		return SystemUnknown
	}
}

func browserOf(ua string) Browser {
	switch {
	case containsAny(ua, "Edg/", "EdgA/", "EdgiOS/", "Edge/"):
		return Edge
	case containsAny(ua, "OPR/", "OPT/", "OPiOS/", "Opera"):
		return Opera
	case strings.Contains(ua, "SamsungBrowser/"):
		return SamsungInternet
	case containsAny(ua, "Firefox/", "FxiOS/"):
		return Firefox
	case containsAny(ua, "Chrome/", "CriOS/"):
		return Chrome
	case strings.Contains(ua, "Safari/"):
		return Safari
	default:
		return BrowserUnknown
	}
}

func kindOf(system System, ua string) Kind {
	switch system {
	case IPhone:
		return KindPhone
	case IPad:
		return KindTablet
	case Android:
		if strings.Contains(ua, "Mobile") {
			return KindPhone
		}
		return KindTablet
	case Mac, Windows, Linux, ChromeOS:
		return KindComputer
	default:
		return KindUnknown
	}
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}
