package email_test

import (
	"regexp"
	"strings"
	"testing"

	"quizzivy/internal/platform/email"
)

const appURL = "https://app.quizzivy.com"

func letterFor(t *testing.T, tmpl email.Template, locale, name string) email.Letter {
	t.Helper()
	l := email.Letter{Locale: locale, Org: "Trung tâm Ánh Dương", AppURL: appURL}
	switch tmpl {
	case email.AccountInvite:
		l.Params, l.Secret = email.AccountInviteParams{FullName: name}, "tok_en-123"
	case email.PasswordResetLink:
		l.Params, l.Secret = email.PasswordResetLinkParams{FullName: name}, "tok_en-123"
	case email.SignInInvite:
		l.Params = email.SignInInviteParams{FullName: name}
	case email.DataExportReady:
		l.Params = email.DataExportReadyParams{FullName: name, ExportID: "0192a7c0-0000-7000-8000-000000000001"}
	default:
		t.Fatalf("no fixture for template %q", tmpl)
	}
	return l
}

func wantLink(tmpl email.Template) string {
	switch tmpl {
	case email.AccountInvite, email.PasswordResetLink:
		return appURL + "/set-password#token=tok_en-123"
	case email.SignInInvite:
		return appURL + "/login"
	default:
		return appURL + "/admin/exports/0192a7c0-0000-7000-8000-000000000001"
	}
}

func TestEveryTemplateRendersInBothLocalesWithItsLinkAndOrganization(t *testing.T) {
	r, err := email.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	for _, tmpl := range email.Templates() {
		for _, locale := range email.Locales {
			t.Run(string(tmpl)+"/"+locale, func(t *testing.T) {
				got, err := r.Render(letterFor(t, tmpl, locale, "Nguyễn Văn An"))
				if err != nil {
					t.Fatal(err)
				}
				if strings.TrimSpace(got.Subject) == "" || strings.TrimSpace(got.Text) == "" || strings.TrimSpace(got.HTML) == "" {
					t.Fatalf("a part is empty: %+v", got)
				}
				if strings.ContainsAny(got.Subject, "\r\n") {
					t.Errorf("the subject spans lines: %q", got.Subject)
				}
				for name, part := range map[string]string{"subject": got.Subject, "text": got.Text, "html": got.HTML} {
					if strings.Contains(part, "{{") || strings.Contains(part, "<no value>") {
						t.Errorf("%s keeps template syntax: %q", name, part)
					}
				}
				if !strings.Contains(got.Text, wantLink(tmpl)) {
					t.Errorf("the text part lacks %s:\n%s", wantLink(tmpl), got.Text)
				}
				if !strings.Contains(got.HTML, `href="`+wantLink(tmpl)+`"`) {
					t.Errorf("the HTML part lacks a link to %s:\n%s", wantLink(tmpl), got.HTML)
				}
				if !strings.HasPrefix(got.Text, "Trung tâm Ánh Dương\n") || !strings.Contains(got.Subject, "Trung tâm Ánh Dương") {
					t.Errorf("the organization does not head the message: %q / %q", got.Subject, got.Text)
				}
				if !strings.Contains(got.Text, "Nguyễn Văn An") {
					t.Errorf("the greeting lacks the name:\n%s", got.Text)
				}
			})
		}
	}
}

func TestATemplateIsWrittenInItsLocale(t *testing.T) {
	r, err := email.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	vi, _ := r.Render(letterFor(t, email.AccountInvite, "vi", ""))
	en, _ := r.Render(letterFor(t, email.AccountInvite, "en", ""))
	if !strings.Contains(vi.Text, "24 giờ") || !strings.Contains(vi.Text, "một lần") {
		t.Errorf("vi does not say the link works once and for 24 hours:\n%s", vi.Text)
	}
	if !strings.Contains(en.Text, "24 hours") || !strings.Contains(en.Text, "works once") {
		t.Errorf("en does not say the link works once and for 24 hours:\n%s", en.Text)
	}
	if !strings.Contains(vi.HTML, `lang="vi"`) || !strings.Contains(en.HTML, `lang="en"`) {
		t.Error("the HTML part does not declare its language")
	}
	reset, _ := r.Render(letterFor(t, email.PasswordResetLink, "en", ""))
	if !strings.Contains(reset.Text, "24 hours") || !strings.Contains(reset.Text, "works once") {
		t.Errorf("the reset link does not say it works once and for 24 hours:\n%s", reset.Text)
	}
	signIn, _ := r.Render(letterFor(t, email.SignInInvite, "en", ""))
	if !strings.Contains(signIn.Text, "Google") || strings.Contains(signIn.Text, "set-password") {
		t.Errorf("the sign-in invitation must send the user to Google:\n%s", signIn.Text)
	}
	ready, _ := r.Render(letterFor(t, email.DataExportReady, "en", ""))
	if !strings.Contains(ready.Text, "once") || strings.Contains(ready.Text, "amazonaws") {
		t.Errorf("the export notice must link to the app page and say the download works once:\n%s", ready.Text)
	}
}

var addresses = regexp.MustCompile(`https?://[^\s"'<>)]+`)

func TestTheExportNoticeLinksOnlyToTheAppPage(t *testing.T) {
	r, err := email.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	want := appURL + "/admin/exports/0192a7c0-0000-7000-8000-000000000001"
	for _, locale := range email.Locales {
		got, err := r.Render(letterFor(t, email.DataExportReady, locale, "Nguyễn Văn An"))
		if err != nil {
			t.Fatal(err)
		}
		for part, body := range map[string]string{"text": got.Text, "html": got.HTML} {
			found := addresses.FindAllString(strings.ReplaceAll(body, "&amp;", "&"), -1)
			if len(found) == 0 {
				t.Errorf("%s/%s: the notice has no link", locale, part)
			}
			for _, address := range found {
				if address != want {
					t.Errorf("%s/%s: the notice links to %s, want only %s", locale, part, address, want)
				}
			}
		}
		for _, secret := range []string{"token", "signature", "X-Amz", "amazonaws", "?", "#"} {
			if strings.Contains(got.Text, secret) {
				t.Errorf("%s: the text part carries %q, which has no place in a notice that names no secret", locale, secret)
			}
		}
	}
}

func TestANameWithMarkupIsEscapedInHTMLAndUntouchedInText(t *testing.T) {
	r, _ := email.NewRenderer()
	const name = `<script>alert("x")</script>`
	for _, tmpl := range email.Templates() {
		got, err := r.Render(letterFor(t, tmpl, "en", name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got.HTML, "<script>") || !strings.Contains(got.HTML, "&lt;script&gt;") {
			t.Errorf("%s: the HTML part does not escape the name:\n%s", tmpl, got.HTML)
		}
		if !strings.Contains(got.Text, name) {
			t.Errorf("%s: the text part altered the name", tmpl)
		}
	}
}

func TestNoTemplateLoadsAnythingRemoteOrTracks(t *testing.T) {
	r, _ := email.NewRenderer()
	for _, tmpl := range email.Templates() {
		got, err := r.Render(letterFor(t, tmpl, "vi", "An"))
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(got.HTML)
		for _, forbidden := range []string{"<img", "<link", "<script", "@import", "url(", "<style", "<iframe"} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("%s: the HTML part contains %q", tmpl, forbidden)
			}
		}
	}
}

func TestRenderRefusesWhatItCannotRenderSafely(t *testing.T) {
	r, _ := email.NewRenderer()
	l := letterFor(t, email.AccountInvite, "vi", "An")
	l.Secret = ""
	if _, err := r.Render(l); err == nil {
		t.Error("a set-password message rendered without its secret")
	}
	l = letterFor(t, email.AccountInvite, "fr", "An")
	if _, err := r.Render(l); err == nil {
		t.Error("an unknown locale rendered")
	}
	if _, err := r.Render(email.Letter{Locale: "vi"}); err == nil {
		t.Error("a letter without params rendered")
	}
	l = letterFor(t, email.AccountInvite, "vi", "An")
	l.AppURL = appURL + "/"
	got, err := r.Render(l)
	if err != nil || !strings.Contains(got.Text, appURL+"/set-password#token=") {
		t.Errorf("a trailing slash on the app URL doubled in the link: %v\n%s", err, got.Text)
	}
}

func TestStoredParamsAreReadByTheirTypeAndIgnoreAFieldANewerReleaseAdded(t *testing.T) {
	got, err := email.DecodeParams(email.DataExportReady, []byte(`{"fullName":"An","exportId":"x"}`))
	if err != nil || got.Template() != email.DataExportReady {
		t.Fatalf("decode: %v %v", got, err)
	}
	got, err = email.DecodeParams(email.DataExportReady, []byte(`{"fullName":"An","exportId":"x","extra":1}`))
	if err != nil || got.(email.DataExportReadyParams).ExportID != "x" {
		t.Errorf("a field a newer release added: %v %v", got, err)
	}
	if _, err := email.DecodeParams(email.DataExportReady, []byte(`{"fullName":7}`)); err == nil {
		t.Error("a field of the wrong type was accepted")
	}
	if _, err := email.DecodeParams("nope", []byte(`{}`)); err == nil {
		t.Error("an unknown template was accepted")
	}
}
