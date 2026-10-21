//go:build integration

package repositories_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgerrcode"

	"quizzivy/internal/modules/organization/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/limits"
)

func TestSaveStoresEveryFieldAndReturnsTheStoredDocument(t *testing.T) {
	s := lockedSession(t)
	actor := s.coverAdmin()
	before := s.stored()
	want := s.fullDocument()

	got, err := s.repo.Save(context.Background(), s.save(want, before.Revision, actor))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	sameDocument(t, "returned document", got, want)
	if got.Revision != before.Revision+1 {
		t.Errorf("revision = %d, want %d", got.Revision, before.Revision+1)
	}
	if got.UpdatedBy != actor {
		t.Errorf("updated by = %q, want %q", got.UpdatedBy, actor)
	}
	if !got.UpdatedAt.Equal(s.dbNow()) {
		t.Errorf("updated at = %v, want the time of the change %v", got.UpdatedAt, s.dbNow())
	}
	if reread := s.stored(); !sameSettings(reread, got) {
		t.Errorf("read after save = %+v, want the returned document %+v", reread, got)
	}
	wantRow := rawOf(want, int32(got.Revision), actor, got.UpdatedAt)
	if row := s.raw(); !row.same(wantRow) {
		t.Errorf("stored columns = %+v, want %+v", row, wantRow)
	}
}

func TestSaveStoresAnAbsentOptionalFieldAsNull(t *testing.T) {
	s := lockedSession(t)
	actor := s.coverAdmin()
	ctx := context.Background()
	first, err := s.repo.Save(ctx, s.save(s.fullDocument(), s.stored().Revision, actor))
	if err != nil {
		t.Fatalf("save of the full document: %v", err)
	}
	want := bareDocument()

	got, err := s.repo.Save(ctx, s.save(want, first.Revision, actor))
	if err != nil {
		t.Fatalf("save of the bare document: %v", err)
	}

	sameDocument(t, "returned document", got, want)
	wantRow := rawOf(want, int32(got.Revision), actor, got.UpdatedAt)
	if row := s.raw(); !row.same(wantRow) {
		t.Errorf("stored columns = %+v, want %+v", row, wantRow)
	}
	if got.Profile.LightLogo != nil || got.Profile.DarkLogo != nil || got.Retention.KeepAnswersMonths != nil {
		t.Errorf("returned document keeps what the save removed: %+v", got)
	}
}

func TestSaveNamesOnlyTheFieldsThatChanged(t *testing.T) {
	cases := []struct {
		name   string
		change func(d *domain.Settings)
		fields []string
	}{
		{"name", func(d *domain.Settings) { d.Profile.Name = "Tên khác" }, []string{"organization.name"}},
		{"short name cleared", func(d *domain.Settings) { d.Profile.ShortName = "" }, []string{"organization.shortName"}},
		{"website", func(d *domain.Settings) { d.Profile.Website = "https://khac.example" }, []string{"organization.website"}},
		{"contact email", func(d *domain.Settings) { d.Profile.ContactEmail = "khac@hoamai.example" }, []string{"organization.contactEmail"}},
		{"phone", func(d *domain.Settings) { d.Profile.FrontDeskPhone = "028 3822 0001" }, []string{"organization.frontDeskPhone"}},
		{"hours cleared", func(d *domain.Settings) { d.Profile.FrontDeskHours = "" }, []string{"organization.frontDeskHours"}},
		{"address", func(d *domain.Settings) { d.Profile.Address = "34 Hai Bà Trưng" }, []string{"organization.address"}},
		{"light logo removed", func(d *domain.Settings) { d.Profile.LightLogo = nil }, []string{"organization.logo.light"}},
		{"dark logo bytes", func(d *domain.Settings) {
			d.Profile.DarkLogo = &domain.Logo{SHA256: digest(90), Type: domain.LogoPNG}
		}, []string{"organization.logo.dark"}},
		{"google off", func(d *domain.Settings) { d.SignIn.Google = false }, []string{"signIn.google"}},
		{"password off", func(d *domain.Settings) { d.SignIn.Password = false }, []string{"signIn.password"}},
		{"force password change on", func(d *domain.Settings) { d.SignIn.ForcePasswordChange = true }, []string{"signIn.forcePasswordChange"}},
		{"join codes on", func(d *domain.Settings) { d.SignIn.JoinCodes = true }, []string{"signIn.joinCodes"}},
		{"a domain added", func(d *domain.Settings) {
			d.SignIn.AllowedGoogleDomains = append(slices.Clone(d.SignIn.AllowedGoogleDomains), "them.example")
		}, []string{"signIn.allowedGoogleDomains"}},
		{"domains reordered", func(d *domain.Settings) {
			d.SignIn.AllowedGoogleDomains = []string{d.SignIn.AllowedGoogleDomains[1], d.SignIn.AllowedGoogleDomains[0]}
		}, []string{"signIn.allowedGoogleDomains"}},
		{"domains cleared", func(d *domain.Settings) { d.SignIn.AllowedGoogleDomains = []string{} }, []string{"signIn.allowedGoogleDomains"}},
		{"answers kept for ever", func(d *domain.Settings) { d.Retention.KeepAnswersMonths = nil }, []string{"retention.keepAnswersMonths"}},
		{"answers kept 12 months", func(d *domain.Settings) { d.Retention.KeepAnswersMonths = intp(12) }, []string{"retention.keepAnswersMonths"}},
		{"focus events kept 24 months", func(d *domain.Settings) { d.Retention.KeepFocusEventsMonths = 24 }, []string{"retention.keepFocusEventsMonths"}},
		{"disabled accounts never anonymised", func(d *domain.Settings) { d.Retention.AnonymiseDisabledAfterMonths = nil }, []string{"retention.anonymiseDisabledAfterMonths"}},
		{"a limit raised", func(d *domain.Settings) {
			d.Limits = limits.Stored{limits.Seats: 500, limits.EmailsPerDay: 80}
		}, []string{"limits.seats"}},
		{"a limit back to its default", func(d *domain.Settings) {
			d.Limits = limits.Stored{limits.Seats: 400}
		}, []string{"limits.emailsPerDay"}},
		{"a limit set", func(d *domain.Settings) {
			d.Limits = limits.Stored{limits.Seats: 400, limits.EmailsPerDay: 80, limits.WarnAtPercent: 90}
		}, []string{"limits.warnAtPercent"}},
		{"several, in the document's order", func(d *domain.Settings) {
			d.Retention.KeepFocusEventsMonths = 12
			d.SignIn.JoinCodes = true
			d.Profile.Name = "Ba thay đổi"
		}, []string{"organization.name", "signIn.joinCodes", "retention.keepFocusEventsMonths"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := lockedSession(t)
			actor := s.coverAdmin()
			ctx := context.Background()
			first, err := s.repo.Save(ctx, s.save(s.fullDocument(), s.stored().Revision, actor))
			if err != nil {
				t.Fatalf("first save: %v", err)
			}
			next := first
			c.change(&next)

			second, err := s.repo.Save(ctx, s.save(next, first.Revision, actor))
			if err != nil {
				t.Fatalf("second save: %v", err)
			}

			audits := s.audits()
			if len(audits) != 2 {
				t.Fatalf("audit rows = %d, want 2", len(audits))
			}
			got := decodeDiff(t, audits[1].diff)
			if !slices.Equal(got.Fields, c.fields) {
				t.Errorf("fields = %v, want %v", got.Fields, c.fields)
			}
			if got.Revision != second.Revision || second.Revision != first.Revision+1 {
				t.Errorf("diff revision = %d, stored revision = %d, want %d", got.Revision, second.Revision, first.Revision+1)
			}
		})
	}
}

func TestSaveRecordsEachChangedLimitBeforeAndAfter(t *testing.T) {
	s := lockedSession(t)
	actor := s.coverAdmin()
	ctx := context.Background()
	first, err := s.repo.Save(ctx, s.save(s.fullDocument(), s.stored().Revision, actor))
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	next := first
	next.Limits = limits.Stored{limits.Seats: 450, limits.StorageBytes: 20 * limits.GiB}
	second, err := s.repo.Save(ctx, s.save(next, first.Revision, actor))
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if !second.Limits.Equal(next.Limits) || !s.stored().Limits.Equal(next.Limits) {
		t.Errorf("limits = %v, stored %v, want %v", second.Limits, s.stored().Limits, next.Limits)
	}
	got := decodeDiff(t, s.audits()[1].diff).Limits
	want := map[limits.Name][2]*int64{
		limits.Seats:        {i64(400), i64(450)},
		limits.StorageBytes: {nil, i64(20 * limits.GiB)},
		limits.EmailsPerDay: {i64(80), nil},
	}
	if len(got) != len(want) {
		t.Fatalf("limit changes = %+v, want %d", got, len(want))
	}
	for _, c := range got {
		w, ok := want[c.Name]
		if !ok || !sameInt(c.Before, w[0]) || !sameInt(c.After, w[1]) {
			t.Errorf("%s: %v → %v, want %v → %v", c.Name, c.Before, c.After, w[0], w[1])
		}
	}
}

func i64(v int64) *int64 { return &v }

func sameInt(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func TestSaveOfADocumentEqualToTheStoredOneChangesNothing(t *testing.T) {
	s := lockedSession(t)
	actor := s.coverAdmin()
	ctx := context.Background()
	first, err := s.repo.Save(ctx, s.save(s.fullDocument(), s.stored().Revision, actor))
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	rowAfterFirst := s.raw()
	again := first
	again.SignIn.AllowedGoogleDomains = slices.Clone(first.SignIn.AllowedGoogleDomains)

	got, err := s.repo.Save(ctx, s.save(again, first.Revision, s.coverAdmin()))
	if err != nil {
		t.Fatalf("save of the same document: %v", err)
	}

	if !sameSettings(got, first) {
		t.Errorf("returned %+v, want the stored document %+v", got, first)
	}
	if row := s.raw(); !row.same(rowAfterFirst) {
		t.Errorf("the row changed: %+v, want %+v", row, rowAfterFirst)
	}
	if audits := s.audits(); len(audits) != 1 {
		t.Errorf("audit rows = %d, want only the first save's", len(audits))
	}
	if _, err := s.repo.Save(ctx, s.save(again, first.Revision-1, actor)); !errors.Is(err, domain.ErrStale) {
		t.Errorf("the same document over a stale revision = %v, want ErrStale", err)
	}
}

func TestSaveOfNoDomainsEqualsStoredNoDomains(t *testing.T) {
	s := lockedSession(t)
	actor := s.coverAdmin()
	ctx := context.Background()
	bare := bareDocument()
	first, err := s.repo.Save(ctx, s.save(bare, s.stored().Revision, actor))
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	bare.SignIn.AllowedGoogleDomains = nil

	got, err := s.repo.Save(ctx, s.save(bare, first.Revision, actor))

	if err != nil || got.Revision != first.Revision {
		t.Errorf("save with nil domains = revision %d, %v, want the stored revision %d", got.Revision, err, first.Revision)
	}
}

func TestTheFirstAuditRowNamesEveryFieldTheSaveSet(t *testing.T) {
	s := lockedSession(t)
	actor := s.coverAdmin()
	s.exec(`
		UPDATE app.organization_settings
		   SET name = 'Quizzivy', short_name = NULL, website = NULL, contact_email = NULL, front_desk_phone = NULL,
		       front_desk_hours = NULL, address = NULL, logo_sha256 = NULL, logo_type = NULL,
		       logo_dark_sha256 = NULL, logo_dark_type = NULL, google_sign_in = true, password_sign_in = true,
		       force_password_change = true, join_codes_enabled = true, allowed_google_domains = '{}',
		       keep_answers_months = NULL, keep_focus_events_months = 13, anonymise_disabled_after_months = NULL`)
	before := s.stored()

	if _, err := s.repo.Save(context.Background(), s.save(s.fullDocument(), before.Revision, actor)); err != nil {
		t.Fatalf("save: %v", err)
	}

	audits := s.audits()
	if len(audits) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(audits))
	}
	want := []string{
		"organization.name", "organization.shortName", "organization.website", "organization.contactEmail",
		"organization.frontDeskPhone", "organization.frontDeskHours", "organization.address",
		"organization.logo.light", "organization.logo.dark",
		"signIn.forcePasswordChange", "signIn.joinCodes", "signIn.allowedGoogleDomains",
		"retention.keepAnswersMonths", "retention.keepFocusEventsMonths", "retention.anonymiseDisabledAfterMonths",
		"limits.seats", "limits.emailsPerDay",
	}
	got := decodeDiff(t, audits[0].diff)
	if !slices.Equal(got.Fields, want) {
		t.Errorf("fields = %v, want %v", got.Fields, want)
	}
	if len(got.Limits) != 2 || got.Limits[0].Name != limits.Seats || got.Limits[0].Before != nil || *got.Limits[0].After != 400 ||
		got.Limits[1].Name != limits.EmailsPerDay || got.Limits[1].Before != nil || *got.Limits[1].After != 80 {
		t.Errorf("limits = %+v, want seats and emails per day from the default to 400 and 80", got.Limits)
	}
}

func TestSaveRecordsTheActorAddressAndUserAgentInTheSameTransaction(t *testing.T) {
	s := lockedSession(t)
	actor := s.coverAdmin()
	ctx := context.Background()
	doc := s.fullDocument()

	in := s.save(doc, s.stored().Revision, actor)
	in.IP = "203.0.113.7"
	in.UserAgent = "Mozilla/5.0 (X11; Linux x86_64) Chrome/130 " + s.nonce
	first, err := s.repo.Save(ctx, in)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	doc.Profile.Name = "Đổi tên lần hai"
	in = s.save(doc, first.Revision, actor)
	in.IP = "2001:db8::7"
	in.UserAgent = ""
	second, err := s.repo.Save(ctx, in)
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	doc.Profile.Name = "Đổi tên lần ba"
	in = s.save(doc, second.Revision, actor)
	in.IP = ""
	in.UserAgent = "Quizzivy-Test/1.0"
	if _, err := s.repo.Save(ctx, in); err != nil {
		t.Fatalf("third save: %v", err)
	}

	audits := s.audits()
	if len(audits) != 3 {
		t.Fatalf("audit rows = %d, want 3", len(audits))
	}
	for i, a := range audits {
		if a.actor == nil || *a.actor != actor {
			t.Errorf("row %d actor = %v, want %s", i, a.actor, actor)
		}
		if a.action != "organization.updated" || a.entity != "organization" {
			t.Errorf("row %d = %q on %q, want organization.updated on organization", i, a.action, a.entity)
		}
		if a.entityID != nil {
			t.Errorf("row %d entity id = %v, want none", i, *a.entityID)
		}
		if !a.occurredAt.Equal(s.now) {
			t.Errorf("row %d occurred at %v, want the save's time %v", i, a.occurredAt, s.now)
		}
	}
	if ip := audits[0].ip; ip == nil || *ip != "203.0.113.7" {
		t.Errorf("first row address = %v, want 203.0.113.7", ip)
	}
	if ua := audits[0].userAgent; ua == nil || *ua != "Mozilla/5.0 (X11; Linux x86_64) Chrome/130 "+s.nonce {
		t.Errorf("first row user agent = %v", ua)
	}
	if ip := audits[1].ip; ip == nil || *ip != "2001:db8::7" {
		t.Errorf("second row address = %v, want 2001:db8::7", ip)
	}
	if audits[1].userAgent != nil {
		t.Errorf("second row user agent = %q, want none", *audits[1].userAgent)
	}
	if audits[2].ip != nil {
		t.Errorf("third row address = %q, want none", *audits[2].ip)
	}
	if ua := audits[2].userAgent; ua == nil || *ua != "Quizzivy-Test/1.0" {
		t.Errorf("third row user agent = %v", ua)
	}
}

func TestSaveWithNoActorLeavesTheAuthorAndTheAuditActorEmpty(t *testing.T) {
	s := lockedSession(t)
	s.coverAdmin()

	got, err := s.repo.Save(context.Background(), s.save(s.fullDocument(), s.stored().Revision, ""))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	if got.UpdatedBy != "" || s.raw().updatedBy != nil {
		t.Errorf("updated by = %q, column %v, want none", got.UpdatedBy, s.raw().updatedBy)
	}
	audits := s.audits()
	if len(audits) != 1 || audits[0].actor != nil {
		t.Errorf("audit rows = %+v, want one with no actor", audits)
	}
}

func TestSaveOfARevisionSomeoneSavedOverIsStaleAndChangesNothing(t *testing.T) {
	s := lockedSession(t)
	actor := s.coverAdmin()
	ctx := context.Background()
	readAt := s.stored().Revision
	winner := s.fullDocument()
	won, err := s.repo.Save(ctx, s.save(winner, readAt, actor))
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	rowAfterWinner := s.raw()
	loser := bareDocument()

	for _, expected := range []int64{readAt, won.Revision + 1, 0} {
		_, err := s.repo.Save(ctx, s.save(loser, expected, actor))
		if !errors.Is(err, domain.ErrStale) {
			t.Fatalf("save over revision %d (stored %d) = %v, want ErrStale", expected, won.Revision, err)
		}
		if errors.Is(err, domain.ErrLockout) {
			t.Errorf("a stale save also reads as a lockout: %v", err)
		}
	}

	if row := s.raw(); !row.same(rowAfterWinner) {
		t.Errorf("stale saves changed the row: %+v, want %+v", row, rowAfterWinner)
	}
	if audits := s.audits(); len(audits) != 1 {
		t.Errorf("audit rows = %d, want only the winner's", len(audits))
	}
}

func TestAFailedSaveLeavesTheRowAndTheAuditLogUntouched(t *testing.T) {
	cases := []struct {
		name  string
		build func(_ *session, in domain.Save) domain.Save
		check func(t *testing.T, err error)
	}{
		{
			"a stale revision",
			func(_ *session, in domain.Save) domain.Save { in.ExpectedRevision++; return in },
			func(t *testing.T, err error) {
				if !errors.Is(err, domain.ErrStale) {
					t.Errorf("error = %v, want ErrStale", err)
				}
			},
		},
		{
			"a sign-in policy that locks the actor out",
			func(s *session, in domain.Save) domain.Save {
				in.Settings.SignIn = domain.SignIn{Google: true, Password: false, AllowedGoogleDomains: []string{"nobody-" + s.nonce + ".example"}}
				return in
			},
			func(t *testing.T, err error) {
				if !errors.Is(err, domain.ErrLockout) {
					t.Errorf("error = %v, want ErrLockout", err)
				}
			},
		},
		{
			"a document the database refuses",
			func(_ *session, in domain.Save) domain.Save {
				in.Settings.Profile.Name = strings.Repeat("a", 121)
				return in
			},
			func(t *testing.T, err error) {
				if !db.IsCheckViolation(err, "organization_settings_name_check") {
					t.Errorf("error = %v, want the name check", err)
				}
			},
		},
		{
			"an author who is not a user",
			func(_ *session, in domain.Save) domain.Save {
				in.ActorID = "00000000-0000-7000-8000-0000000000aa"
				return in
			},
			func(t *testing.T, err error) {
				if name, ok := db.ForeignKeyViolation(err); !ok || name != "organization_settings_updated_by_fkey" {
					t.Errorf("error = %v, want the updated_by foreign key", err)
				}
			},
		},
		{
			"an audit row the database refuses after the update was made",
			func(_ *session, in domain.Save) domain.Save { in.IP = "not-an-address"; return in },
			func(t *testing.T, err error) {
				if pg := pgError(t, err); pg.Code != pgerrcode.InvalidTextRepresentation {
					t.Errorf("error code = %s, want %s", pg.Code, pgerrcode.InvalidTextRepresentation)
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := lockedSession(t)
			actor := s.coverAdmin()
			ctx := context.Background()
			before := s.stored()
			rowBefore := s.raw()
			in := s.save(s.fullDocument(), before.Revision, actor)

			_, err := s.repo.Save(ctx, c.build(s, in))

			if err == nil {
				t.Fatal("save succeeded, want it refused")
			}
			c.check(t, err)
			if row := s.raw(); !row.same(rowBefore) {
				t.Errorf("row after the refused save = %+v, want %+v", row, rowBefore)
			}
			if after := s.stored(); !sameSettings(after, before) {
				t.Errorf("document after the refused save = %+v, want %+v", after, before)
			}
			if audits := s.audits(); len(audits) != 0 {
				t.Errorf("audit rows after the refused save = %d, want none", len(audits))
			}
			next, err := s.repo.Save(ctx, s.save(s.fullDocument(), before.Revision, actor))
			if err != nil {
				t.Fatalf("save after the refused one: %v", err)
			}
			if next.Revision != before.Revision+1 {
				t.Errorf("revision after the refused save and a good one = %d, want %d", next.Revision, before.Revision+1)
			}
			if audits := s.audits(); len(audits) != 1 {
				t.Errorf("audit rows after the good save = %d, want 1", len(audits))
			}
		})
	}
}
