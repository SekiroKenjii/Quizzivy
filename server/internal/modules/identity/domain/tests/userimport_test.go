package domain_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"quizzivy/internal/modules/identity/domain"
)

var (
	roleStudent   = domain.ImportRole{ID: "r-student", Name: "Học viên", BuiltinKey: "student", StudentTarget: true}
	roleTeacher   = domain.ImportRole{ID: "r-teacher", Name: "Giáo viên", BuiltinKey: "teacher"}
	roleAdmin     = domain.ImportRole{ID: "r-admin", Name: "Quản trị viên", BuiltinKey: "admin", Refusal: domain.RoleBeyondImporter}
	roleAssistant = domain.ImportRole{ID: "r-assistant", Name: "Trợ giảng", BuiltinKey: "assistant", Refusal: domain.RoleNeverGiven}
	roleReader    = domain.ImportRole{ID: "r-reader", Name: "Người đọc đề"}
	roleTakerOnly = domain.ImportRole{ID: "r-taker", Name: "Chỉ làm bài", StudentTarget: true}
)

func rules(place bool, classes ...domain.ImportClass) domain.ImportRules {
	return domain.ImportRules{
		Roles:         []domain.ImportRole{roleStudent, roleTeacher, roleAdmin, roleAssistant, roleReader, roleTakerOnly},
		DefaultRoleID: roleStudent.ID,
		Classes:       classes,
		PlaceInClass:  place,
	}
}

var (
	class10A  = domain.ImportClass{ID: "c-10a", Name: "Lớp 10A"}
	class10B  = domain.ImportClass{ID: "c-10b", Name: "Lớp 10B"}
	classTwin = domain.ImportClass{ID: "c-twin-1", Name: "Toán nâng cao"}
	classCopy = domain.ImportClass{ID: "c-twin-2", Name: "toán nâng cao"}
)

func row(line int, name, email, role, class string) domain.ImportRow {
	return domain.ImportRow{Line: line, FullName: name, Email: email, Role: role, Class: class}
}

func issuesOf(plan domain.ImportPlan) map[int][]string {
	out := map[int][]string{}
	for _, i := range plan.Issues {
		out[i.Line] = append(out[i.Line], string(i.Field)+":"+string(i.Code))
	}
	return out
}

func TestTheHeaderIsReadWhateverItsCaseAndOrder(t *testing.T) {
	cols, problem := domain.ReadImportColumns([]string{"  CLASS ", "Email", "extra", "FULL_NAME", "Role"})

	if !problem.Empty() {
		t.Fatalf("problem = %+v", problem)
	}
	if want := (domain.ImportColumns{FullName: 3, Email: 1, Role: 4, Class: 0}); cols != want {
		t.Errorf("columns = %+v, want %+v", cols, want)
	}
}

func TestRoleAndClassMayBeAbsentFromTheHeader(t *testing.T) {
	cols, problem := domain.ReadImportColumns([]string{"full_name", "email"})

	if !problem.Empty() || cols.Role != -1 || cols.Class != -1 {
		t.Errorf("columns = %+v, problem = %+v, want both optional columns absent and no problem", cols, problem)
	}
}

func TestAHeaderNamesWhatItLacksAndWhatItRepeats(t *testing.T) {
	for name, tc := range map[string]struct {
		header            []string
		missing, repeated []domain.ImportField
	}{
		"no name":         {[]string{"email"}, []domain.ImportField{domain.FieldFullName}, nil},
		"no email":        {[]string{"full_name", "role"}, []domain.ImportField{domain.FieldEmail}, nil},
		"neither":         {[]string{"role", "class"}, []domain.ImportField{domain.FieldFullName, domain.FieldEmail}, nil},
		"an empty header": {nil, []domain.ImportField{domain.FieldFullName, domain.FieldEmail}, nil},
		"email twice":     {[]string{"email", "full_name", "EMAIL"}, nil, []domain.ImportField{domain.FieldEmail}},
		"class thrice":    {[]string{"full_name", "email", "class", "class", "Class"}, nil, []domain.ImportField{domain.FieldClass}},
		"a header in another language is unknown": {[]string{"họ tên", "thư điện tử"}, []domain.ImportField{domain.FieldFullName, domain.FieldEmail}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			_, problem := domain.ReadImportColumns(tc.header)

			if !slices.Equal(problem.Missing, tc.missing) || !slices.Equal(problem.Repeated, tc.repeated) {
				t.Errorf("missing %v, repeated %v, want %v and %v", problem.Missing, problem.Repeated, tc.missing, tc.repeated)
			}
		})
	}
}

func TestARowShorterThanTheHeaderReadsEmptyCells(t *testing.T) {
	cols, _ := domain.ReadImportColumns([]string{"full_name", "email", "role", "class"})

	got := domain.NewImportRow(7, []string{"Mai", "mai@x.test"}, cols)

	if got != (domain.ImportRow{Line: 7, FullName: "Mai", Email: "mai@x.test"}) {
		t.Errorf("row = %+v", got)
	}
}

func TestAGoodRowTakesTheDefaultRoleWhenItNamesNone(t *testing.T) {
	plan := domain.PlanImport([]domain.ImportRow{row(2, "Nguyễn Thị Mai", "mai@x.test", "", "")}, rules(true), nil)

	if len(plan.Issues) != 0 || len(plan.Accepted) != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	got := plan.Accepted[0]
	if got.RoleID != roleStudent.ID || got.ClassID != "" || got.Ordinal != 0 || got.Line != 2 || got.Existing {
		t.Errorf("accepted = %+v", got)
	}
	if plan.NewAccounts != 1 || plan.ExistingCount != 0 || plan.InvalidRows != 0 {
		t.Errorf("counts = %d new, %d existing, %d invalid", plan.NewAccounts, plan.ExistingCount, plan.InvalidRows)
	}
}

func TestARoleIsNamedByKeyOrByNameInAnyCase(t *testing.T) {
	for cell, want := range map[string]string{
		"student":        roleStudent.ID,
		"STUDENT":        roleStudent.ID,
		"học viên":       roleStudent.ID,
		"  Học   Viên  ": roleStudent.ID,
		"Giáo viên":      roleTeacher.ID,
		"giáo viên":      roleTeacher.ID,
		"teacher":        roleTeacher.ID,
		"người đọc đề":   roleReader.ID,
		"Chỉ làm bài":    roleTakerOnly.ID,
	} {
		plan := domain.PlanImport([]domain.ImportRow{row(2, "A", "a@x.test", cell, "")}, rules(true), nil)

		if len(plan.Issues) != 0 || plan.Accepted[0].RoleID != want {
			t.Errorf("%q: issues %v, role %v, want %s", cell, plan.Issues, plan.Accepted, want)
		}
	}
}

func TestAnAccentIsPartOfARoleName(t *testing.T) {
	plan := domain.PlanImport([]domain.ImportRow{row(2, "A", "a@x.test", "hoc vien", "")}, rules(true), nil)

	if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"role:UNKNOWN_ROLE"}) {
		t.Errorf("issues = %v, want an unknown role", got)
	}
}

func TestEveryRefusalOfARoleHasItsOwnCode(t *testing.T) {
	for cell, want := range map[string]domain.ImportCode{
		"admin":          domain.CodeRoleNotAllowed,
		"Quản trị viên":  domain.CodeRoleNotAllowed,
		"assistant":      domain.CodeRoleNotGivable,
		"Trợ giảng":      domain.CodeRoleNotGivable,
		"giám đốc":       domain.CodeUnknownRole,
		"teacher; admin": domain.CodeUnknownRole,
	} {
		plan := domain.PlanImport([]domain.ImportRow{row(4, "A", "a@x.test", cell, "")}, rules(true), nil)

		got := plan.Issues
		if len(got) != 1 || got[0] != (domain.ImportIssue{Line: 4, Field: domain.FieldRole, Code: want}) {
			t.Errorf("%q: issues = %v, want %s on role", cell, got, want)
		}
		if len(plan.Accepted) != 0 {
			t.Errorf("%q: accepted %v, want nothing", cell, plan.Accepted)
		}
	}
}

func TestADefaultRoleTheImporterMayNotGiveRefusesTheRowsThatTakeIt(t *testing.T) {
	r := rules(true)
	r.DefaultRoleID = roleAdmin.ID

	plan := domain.PlanImport([]domain.ImportRow{row(2, "A", "a@x.test", "", ""), row(3, "B", "b@x.test", "teacher", "")}, r, nil)

	if got := issuesOf(plan); !slices.Equal(got[2], []string{"role:ROLE_NOT_ALLOWED"}) || len(got[3]) != 0 {
		t.Errorf("issues = %v, want the first row refused and the second taken", got)
	}
}

func TestACustomRoleNamedLikeABuiltInKeyIsAmbiguous(t *testing.T) {
	r := rules(true)
	r.Roles = append(r.Roles, domain.ImportRole{ID: "r-custom", Name: "Teacher"})

	plan := domain.PlanImport([]domain.ImportRow{row(2, "A", "a@x.test", "teacher", ""), row(3, "B", "b@x.test", "giáo viên", "")}, r, nil)

	if got := issuesOf(plan); !slices.Equal(got[2], []string{"role:AMBIGUOUS_ROLE"}) || len(got[3]) != 0 {
		t.Errorf("issues = %v, want only the key ambiguous", got)
	}
}

func TestANameAndAnAddressAreRequiredAndBounded(t *testing.T) {
	long := strings.Repeat("a", domain.MaxFullNameLength+1)
	longEmail := strings.Repeat("a", domain.MaxEmailLength) + "@x.test"
	exact := strings.Repeat("a", domain.MaxFullNameLength)
	rows := []domain.ImportRow{
		row(2, "", "a@x.test", "", ""),
		row(3, "   ", "b@x.test", "", ""),
		row(4, long, "c@x.test", "", ""),
		row(5, "D", "", "", ""),
		row(6, "E", longEmail, "", ""),
		row(7, exact, "g@x.test", "", ""),
	}

	got := issuesOf(domain.PlanImport(rows, rules(true), nil))

	want := map[int][]string{
		2: {"full_name:REQUIRED"}, 3: {"full_name:REQUIRED"}, 4: {"full_name:TOO_LONG"},
		5: {"email:REQUIRED"}, 6: {"email:TOO_LONG"},
	}
	for line, w := range want {
		if !slices.Equal(got[line], w) {
			t.Errorf("row %d: %v, want %v", line, got[line], w)
		}
	}
	if len(got[7]) != 0 {
		t.Errorf("a name of exactly %d characters: %v, want it taken", domain.MaxFullNameLength, got[7])
	}
}

func TestANameIsComposedAndItsSpaceCollapsed(t *testing.T) {
	decomposed := "Nguyễn  Văn\tA"

	plan := domain.PlanImport([]domain.ImportRow{row(2, decomposed, "a@x.test", "", "")}, rules(true), nil)

	if len(plan.Accepted) != 1 || plan.Accepted[0].FullName != "Nguyễn Văn A" {
		t.Errorf("accepted = %+v, want the composed name with single spaces", plan.Accepted)
	}
}

func TestAControlOrFormatCharacterInANameIsRefused(t *testing.T) {
	for name, cell := range map[string]string{
		"a bell":            "Mai\u0007",
		"a bidi override":   "Mai\u202eevil",
		"a zero-width mark": "Ma\u200bi",
		"a null":            "Mai\x00",
	} {
		plan := domain.PlanImport([]domain.ImportRow{row(2, cell, "a@x.test", "", "")}, rules(true), nil)

		if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"full_name:INVALID"}) {
			t.Errorf("%s: issues = %v, want full_name:INVALID", name, got)
		}
	}
}

func TestANameNoReaderCanSeeIsRefusedAsTheFormRefusesIt(t *testing.T) {
	for name, cell := range map[string]string{
		"a Hangul filler":          string(rune(0x3164)),
		"a Braille blank":          "Mai" + string(rune(0x2800)),
		"a soft hyphen":            "Ma" + string(rune(0xad)) + "i",
		"a byte order mark":        string(rune(0xfeff)) + "Mai",
		"a lone combining accent":  string(rune(0x301)),
		"an unassigned code point": "Mai" + string(rune(0x378)),
	} {
		plan := domain.PlanImport([]domain.ImportRow{row(2, cell, "a@x.test", "", "")}, rules(true), nil)

		if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"full_name:INVALID"}) {
			t.Errorf("%s: issues = %v, want full_name:INVALID", name, got)
		}
	}
	for name, cell := range map[string]string{"a no-break space": string(rune(0xa0)), "an ideographic space": string(rune(0x3000))} {
		plan := domain.PlanImport([]domain.ImportRow{row(2, cell, "a@x.test", "", "")}, rules(true), nil)

		if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"full_name:REQUIRED"}) {
			t.Errorf("%s: issues = %v, want full_name:REQUIRED", name, got)
		}
	}
}

func TestAQuotedAddressThePrinterCannotWriteBackIsNotAnEmail(t *testing.T) {
	for _, cell := range []string{`"qc tp"@x.test`, `"a..b"@x.test`, `"a\"b"@x.test`, `"a b"@x.test`} {
		plan := domain.PlanImport([]domain.ImportRow{row(2, "A", cell, "", "")}, rules(true), nil)

		if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"email:INVALID"}) {
			t.Errorf("%q: issues = %v, want email:INVALID", cell, got)
		}
	}
}

func TestAnAddressIsStoredBareWhateverFormTheCellWritesItIn(t *testing.T) {
	for cell, want := range map[string]string{
		"mai@x.test":                  "mai@x.test",
		"  mai@x.test  ":              "mai@x.test",
		"Mai.Nguyen+lop10@X.Test":     "Mai.Nguyen+lop10@X.Test",
		"Mai <mai@x.test>":            "mai@x.test",
		"Nguyễn Văn Mai <mai@x.test>": "mai@x.test",
		"<mai@x.test>":                "mai@x.test",
		"\"mai\"@x.test":              "mai@x.test",
		"mai@x.test (ghi chú)":        "mai@x.test",
	} {
		plan := domain.PlanImport([]domain.ImportRow{row(2, "A", cell, "", "")}, rules(true), nil)

		if len(plan.Accepted) != 1 || plan.Accepted[0].Email != want {
			t.Errorf("%q: accepted %+v, issues %v; want the address %q", cell, plan.Accepted, plan.Issues, want)
		}
	}
}

func TestACellThatIsNotOneAddressIsNotAnEmail(t *testing.T) {
	for _, cell := range []string{
		"mai", "mai@", "@x.test", "mai@x.test, an@x.test", "mai@x.test\nan@x.test", "Mai <mai@x.test", "mai@x.test>",
		"Mai <mai@x.test> <an@x.test>", "mai x@x.test",
	} {
		plan := domain.PlanImport([]domain.ImportRow{row(2, "A", cell, "", "")}, rules(true), nil)

		if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"email:INVALID"}) {
			t.Errorf("%q: issues = %v, want email:INVALID", cell, got)
		}
	}
}

func TestAnAddressWithACharacterNoReaderCanSeeIsInvalidWhereverItSits(t *testing.T) {
	for _, r := range []rune{
		0x00, 0x07, 0xa0, 0xad, 0x115f, 0x1160, 0x2003, 0x200b, 0x200c, 0x200d, 0x2028, 0x202e, 0x2066,
		0x2800, 0x3000, 0x3164, 0xe000, 0xfeff, 0xffa0, 0xfffe,
		0x034f, 0x180b, 0xfe00, 0xfe0f, 0xe0100, 0xe01ef,
	} {
		for _, cell := range []string{
			"ma" + string(r) + "i@x.test",
			"mai@x" + string(r) + ".test",
			"Mai <ma" + string(r) + "i@x.test>",
		} {
			plan := domain.PlanImport([]domain.ImportRow{row(2, "A", cell, "", "")}, rules(true), nil)

			if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"email:INVALID"}) {
				t.Errorf("%q: issues = %v, want email:INVALID", cell, got)
			}
		}
	}
}

func TestAnAddressAtTheEndOfACellLosesOnlyTheSpaceAroundIt(t *testing.T) {
	for _, r := range []rune{0xa0, 0x2003, 0x3000} {
		cell := string(r) + "mai@x.test" + string(r)
		plan := domain.PlanImport([]domain.ImportRow{row(2, "A", cell, "", "")}, rules(true), nil)

		if len(plan.Accepted) != 1 || plan.Accepted[0].Email != "mai@x.test" {
			t.Errorf("%q: accepted %+v, issues %v", cell, plan.Accepted, plan.Issues)
		}
	}
}

func TestADisplayNameDoesNotHideAnAddressThatIsTooLong(t *testing.T) {
	long := strings.Repeat("a", domain.MaxEmailLength) + "@x.test"
	for _, cell := range []string{long, "Mai <" + long + ">"} {
		plan := domain.PlanImport([]domain.ImportRow{row(2, "A", cell, "", "")}, rules(true), nil)

		if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"email:TOO_LONG"}) {
			t.Errorf("an address of %d characters: issues = %v, want email:TOO_LONG", domain.MaxEmailLength+7, got)
		}
	}
}

func TestASecondRowWithTheSameAddressInAnyCaseIsADuplicate(t *testing.T) {
	rows := []domain.ImportRow{
		row(2, "A", "a@x.test", "", ""),
		row(3, "B", "B@x.test", "", ""),
		row(4, "A again", "A@X.TEST", "", ""),
		row(5, "B again", " b@x.test ", "", ""),
	}

	plan := domain.PlanImport(rows, rules(true), nil)

	if got := issuesOf(plan); !slices.Equal(got[4], []string{"email:DUPLICATE_IN_FILE"}) || !slices.Equal(got[5], []string{"email:DUPLICATE_IN_FILE"}) || len(got[2])+len(got[3]) != 0 {
		t.Errorf("issues = %v", got)
	}
	if len(plan.Accepted) != 2 || plan.Accepted[0].Line != 2 || plan.Accepted[1].Line != 3 {
		t.Errorf("accepted = %+v, want the first row of each address", plan.Accepted)
	}
}

func TestAnAddressAnInvalidRowNamedStillClaimsItForTheRowsAfter(t *testing.T) {
	rows := []domain.ImportRow{row(2, "", "a@x.test", "", ""), row(3, "A", "a@x.test", "", "")}

	got := issuesOf(domain.PlanImport(rows, rules(true), nil))

	if !slices.Equal(got[2], []string{"full_name:REQUIRED"}) || !slices.Equal(got[3], []string{"email:DUPLICATE_IN_FILE"}) {
		t.Errorf("issues = %v", got)
	}
}

func TestAnExistingAddressIsAcceptedAndCountedNotRefused(t *testing.T) {
	rows := []domain.ImportRow{row(2, "A", "A@x.test", "", ""), row(3, "B", "b@x.test", "", "")}

	plan := domain.PlanImport(rows, rules(true), map[string]struct{}{"a@x.test": {}})

	if len(plan.Issues) != 0 || len(plan.Accepted) != 2 {
		t.Fatalf("plan = %+v", plan)
	}
	if !plan.Accepted[0].Existing || plan.Accepted[1].Existing {
		t.Errorf("existing = %v %v, want the first only", plan.Accepted[0].Existing, plan.Accepted[1].Existing)
	}
	if plan.NewAccounts != 1 || plan.ExistingCount != 1 || !slices.Equal(plan.ExistingSample, []string{"A@x.test"}) {
		t.Errorf("new %d, existing %d, sample %v", plan.NewAccounts, plan.ExistingCount, plan.ExistingSample)
	}
}

func TestTheSampleOfExistingAddressesIsBounded(t *testing.T) {
	var rows []domain.ImportRow
	existing := map[string]struct{}{}
	for i := range 12 {
		email := string(rune('a'+i)) + "@x.test"
		rows = append(rows, row(i+2, "A", email, "", ""))
		existing[email] = struct{}{}
	}

	plan := domain.PlanImport(rows, rules(true), existing)

	if plan.ExistingCount != 12 || len(plan.ExistingSample) != domain.MaxExistingSample {
		t.Errorf("count %d, sample %v, want 12 and %d named", plan.ExistingCount, plan.ExistingSample, domain.MaxExistingSample)
	}
}

func TestARefusedRowIsStillRefusedWhenItsAddressExists(t *testing.T) {
	plan := domain.PlanImport([]domain.ImportRow{row(2, "A", "a@x.test", "admin", "")}, rules(true), map[string]struct{}{"a@x.test": {}})

	if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"role:ROLE_NOT_ALLOWED"}) || plan.ExistingCount != 0 {
		t.Errorf("issues = %v, existing %d, want the role refused and nothing counted existing", got, plan.ExistingCount)
	}
}

func TestEveryRefusedCellOfARowIsReportedAndTheRowCountsOnce(t *testing.T) {
	rows := []domain.ImportRow{row(9, "", "no-at-sign", "giám đốc", "Lớp lạ")}

	plan := domain.PlanImport(rows, rules(true, class10A), nil)

	want := []string{"full_name:REQUIRED", "email:INVALID", "role:UNKNOWN_ROLE", "class:UNKNOWN_CLASS"}
	if got := issuesOf(plan)[9]; !slices.Equal(got, want) {
		t.Errorf("issues = %v, want %v", got, want)
	}
	if plan.InvalidRows != 1 {
		t.Errorf("invalid rows = %d, want 1", plan.InvalidRows)
	}
}

func TestAClassIsNamedExactlyApartFromCaseAndSpace(t *testing.T) {
	for cell, want := range map[string]string{
		"Lớp 10A":    class10A.ID,
		"lớp 10a":    class10A.ID,
		"  Lớp  10B": class10B.ID,
	} {
		plan := domain.PlanImport([]domain.ImportRow{row(2, "A", "a@x.test", "", cell)}, rules(true, class10A, class10B), nil)

		if len(plan.Accepted) != 1 || plan.Accepted[0].ClassID != want {
			t.Errorf("%q: %+v %v, want class %s", cell, plan.Accepted, plan.Issues, want)
		}
	}
	for name, cell := range map[string]string{"without its accent": "Lop 10A", "a prefix": "Lớp 10", "a different class": "Lớp 11A"} {
		plan := domain.PlanImport([]domain.ImportRow{row(2, "A", "a@x.test", "", cell)}, rules(true, class10A, class10B), nil)

		if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"class:UNKNOWN_CLASS"}) {
			t.Errorf("%s: issues = %v, want an unknown class", name, got)
		}
	}
}

func TestTwoClassesOfOneNameAreAmbiguous(t *testing.T) {
	plan := domain.PlanImport([]domain.ImportRow{row(2, "A", "a@x.test", "", "Toán nâng cao")}, rules(true, classTwin, classCopy), nil)

	if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"class:AMBIGUOUS_CLASS"}) {
		t.Errorf("issues = %v, want an ambiguous class", got)
	}
}

func TestAClassNeedsAStudentTargetRoleAndTheRightToPlaceStudents(t *testing.T) {
	for name, tc := range map[string]struct {
		place bool
		role  string
		want  []string
	}{
		"a student, allowed":             {true, "student", nil},
		"a role holding only take_tests": {true, "Chỉ làm bài", nil},
		"a teacher":                      {true, "teacher", []string{"class:CLASS_NOT_ALLOWED"}},
		"a custom role beyond student":   {true, "Người đọc đề", []string{"class:CLASS_NOT_ALLOWED"}},
		"a student, without scope.all":   {false, "student", []string{"class:CLASS_NOT_ALLOWED"}},
		"no role named, without scope":   {false, "", []string{"class:CLASS_NOT_ALLOWED"}},
	} {
		t.Run(name, func(t *testing.T) {
			plan := domain.PlanImport([]domain.ImportRow{row(2, "A", "a@x.test", tc.role, "Lớp 10A")}, rules(tc.place, class10A), nil)

			if got := issuesOf(plan)[2]; !slices.Equal(got, tc.want) {
				t.Errorf("issues = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAClassOfARowWhoseRoleIsRefusedIsStillChecked(t *testing.T) {
	plan := domain.PlanImport([]domain.ImportRow{row(2, "A", "a@x.test", "admin", "Lớp lạ")}, rules(true, class10A), nil)

	if got := issuesOf(plan)[2]; !slices.Equal(got, []string{"role:ROLE_NOT_ALLOWED", "class:UNKNOWN_CLASS"}) {
		t.Errorf("issues = %v", got)
	}
}

func TestThePlanNamesEachClassItPlacesStudentsInOnce(t *testing.T) {
	rows := []domain.ImportRow{
		row(2, "A", "a@x.test", "", "Lớp 10B"),
		row(3, "B", "b@x.test", "", "lớp 10a"),
		row(4, "C", "c@x.test", "", "Lớp 10B"),
		row(5, "D", "d@x.test", "", ""),
	}

	plan := domain.PlanImport(rows, rules(true, class10A, class10B), nil)

	if !slices.Equal(plan.Classes, []domain.ImportClass{class10B, class10A}) {
		t.Errorf("classes = %v, want each once in the order first used", plan.Classes)
	}
}

func TestOrdinalsFollowTheAcceptedRowsAndLinesTheFile(t *testing.T) {
	rows := []domain.ImportRow{row(2, "A", "a@x.test", "", ""), row(3, "", "b@x.test", "", ""), row(4, "C", "c@x.test", "", "")}

	plan := domain.PlanImport(rows, rules(true), nil)

	if len(plan.Accepted) != 2 || plan.Accepted[0].Ordinal != 0 || plan.Accepted[1].Ordinal != 1 || plan.Accepted[1].Line != 4 {
		t.Errorf("accepted = %+v, want ordinals 0 and 1 for lines 2 and 4", plan.Accepted)
	}
	if plan.Issues[0].Line != 3 {
		t.Errorf("issue = %+v, want the file's own line", plan.Issues[0])
	}
}

func TestImportEmailsAreTheDistinctLowerCasedAddressesThatReadAsAddresses(t *testing.T) {
	rows := []domain.ImportRow{
		row(2, "A", "A@x.test", "", ""), row(3, "B", "a@X.TEST", "", ""), row(4, "C", " c@x.test ", "", ""),
		row(5, "D", "not an address", "", ""), row(6, "E", "", "", ""),
	}

	got := domain.ImportEmails(rows)

	if !slices.Equal(got, []string{"a@x.test", "c@x.test"}) {
		t.Errorf("emails = %v", got)
	}
}

func TestTheSheetIsAvailableOnlyOnceDoneUnopenedAndUnexpired(t *testing.T) {
	now := time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	later, earlier := now.Add(time.Hour), now.Add(-time.Hour)
	job := domain.ImportJob{Delivery: domain.ImportSheet, Status: domain.ImportDone, Created: 3, SheetExpiresAt: &later}
	for name, tc := range map[string]struct {
		change func(*domain.ImportJob)
		want   bool
	}{
		"done and fresh":  {func(*domain.ImportJob) {}, true},
		"still running":   {func(j *domain.ImportJob) { j.Status = domain.ImportRunning }, false},
		"failed":          {func(j *domain.ImportJob) { j.Status = domain.ImportFailed }, false},
		"downloaded":      {func(j *domain.ImportJob) { j.SheetDownloadedAt = &earlier }, false},
		"expired":         {func(j *domain.ImportJob) { j.SheetExpiresAt = &earlier }, false},
		"nothing created": {func(j *domain.ImportJob) { j.Created = 0 }, false},
		"no expiry set":   {func(j *domain.ImportJob) { j.SheetExpiresAt = nil }, false},
	} {
		t.Run(name, func(t *testing.T) {
			j := job
			tc.change(&j)

			state, has := j.Sheet(now)

			if !has || state.Available != tc.want {
				t.Errorf("has %v, available %v, want a sheet and available %v", has, state.Available, tc.want)
			}
		})
	}
	if _, has := (domain.ImportJob{Delivery: domain.ImportEmailLink, Status: domain.ImportDone, Created: 3}).Sheet(now); has {
		t.Error("a job delivered by email link has a sheet")
	}
}

func TestProcessedIsWhatWasCreatedOrSkipped(t *testing.T) {
	if got := (domain.ImportJob{Created: 7, Skipped: 3, Invalid: 40}).Processed(); got != 10 {
		t.Errorf("processed = %d, want 10", got)
	}
}
