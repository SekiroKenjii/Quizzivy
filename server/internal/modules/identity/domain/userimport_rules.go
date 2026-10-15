package domain

import (
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"quizzivy/internal/shared/content"
)

// ImportRoleRefusal is why the importer may not give a role.
type ImportRoleRefusal int

const (
	// RoleGivable means the importer may give the role.
	RoleGivable ImportRoleRefusal = iota
	// RoleBeyondImporter means the role holds a permission the importer lacks.
	RoleBeyondImporter
	// RoleNeverGiven means the role is the Assistant's, which no one gives.
	RoleNeverGiven
)

// ImportRole is a role a file may name, with what the importer may do with it.
type ImportRole struct {
	ID            string
	Name          string
	BuiltinKey    string
	StudentTarget bool
	Refusal       ImportRoleRefusal
}

// ImportClass is an active class a file may name.
type ImportClass struct {
	ID   string
	Name string
}

// ImportRules is what a file's rows are judged against.
type ImportRules struct {
	// Roles is every role. A row names one by its built-in key, its name or,
	// when empty, takes DefaultRoleID.
	Roles         []ImportRole
	DefaultRoleID string
	// Classes is every active class, and PlaceInClass whether the importer may
	// put students in them (scope.all).
	Classes      []ImportClass
	PlaceInClass bool
}

// ImportIssue is one refused cell.
type ImportIssue struct {
	Line  int
	Field ImportField
	Code  ImportCode
}

// ImportAccepted is a row that passed every rule. Existing says the address
// already belongs to an account, which an import never changes.
type ImportAccepted struct {
	Ordinal  int
	Line     int
	FullName string
	Email    string
	RoleID   string
	ClassID  string
	Existing bool
}

// ImportPlan is what a file comes to: the rows to store, the cells refused,
// and the counts a preview shows.
type ImportPlan struct {
	Accepted       []ImportAccepted
	Issues         []ImportIssue
	InvalidRows    int
	NewAccounts    int
	ExistingCount  int
	ExistingSample []string
	Classes        []ImportClass
}

// ImportEmails returns the distinct addresses of rows that read as addresses,
// lower-cased, for the application to look up before PlanImport.
func ImportEmails(rows []ImportRow) []string {
	seen := make(map[string]struct{}, len(rows))
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		email := cleanEmail(row.Email)
		if !validEmail(email) {
			continue
		}
		key := strings.ToLower(email)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			out = append(out, key)
		}
	}
	return out
}

// PlanImport judges every row against rules. existing holds the lower-cased
// addresses that already belong to an account. A row is refused cell by cell,
// so one pass shows everything to correct; an address named twice is refused
// from its second row on, and an existing address is accepted and skipped when
// the job runs.
func PlanImport(rows []ImportRow, rules ImportRules, existing map[string]struct{}) ImportPlan {
	judge := newJudge(rules, existing)
	var plan ImportPlan
	for _, row := range rows {
		accepted, issues := judge.row(row)
		if len(issues) > 0 {
			plan.InvalidRows++
			plan.Issues = append(plan.Issues, issues...)
			continue
		}
		accepted.Ordinal = len(plan.Accepted)
		plan.Accepted = append(plan.Accepted, accepted)
		plan.tally(accepted, judge)
	}
	return plan
}

func (p *ImportPlan) tally(row ImportAccepted, j *judge) {
	if row.Existing {
		if p.ExistingCount++; len(p.ExistingSample) < MaxExistingSample {
			p.ExistingSample = append(p.ExistingSample, row.Email)
		}
	} else {
		p.NewAccounts++
	}
	if row.ClassID != "" && !p.hasClass(row.ClassID) {
		p.Classes = append(p.Classes, j.classByID[row.ClassID])
	}
}

func (p *ImportPlan) hasClass(id string) bool {
	for _, c := range p.Classes {
		if c.ID == id {
			return true
		}
	}
	return false
}

type judge struct {
	roles        map[string]ImportRole
	rolesByToken map[string][]string
	defaultRole  string
	classes      map[string][]ImportClass
	classByID    map[string]ImportClass
	place        bool
	existing     map[string]struct{}
	seen         map[string]struct{}
}

func newJudge(rules ImportRules, existing map[string]struct{}) *judge {
	j := &judge{
		roles:        make(map[string]ImportRole, len(rules.Roles)),
		rolesByToken: map[string][]string{},
		defaultRole:  rules.DefaultRoleID,
		classes:      map[string][]ImportClass{},
		classByID:    make(map[string]ImportClass, len(rules.Classes)),
		place:        rules.PlaceInClass,
		existing:     existing,
		seen:         map[string]struct{}{},
	}
	for _, role := range rules.Roles {
		j.roles[role.ID] = role
		j.nameRole(foldText(role.Name), role.ID)
		j.nameRole(foldText(role.BuiltinKey), role.ID)
	}
	for _, class := range rules.Classes {
		key := foldText(class.Name)
		j.classes[key] = append(j.classes[key], class)
		j.classByID[class.ID] = class
	}
	return j
}

func (j *judge) nameRole(token, id string) {
	if token == "" {
		return
	}
	for _, have := range j.rolesByToken[token] {
		if have == id {
			return
		}
	}
	j.rolesByToken[token] = append(j.rolesByToken[token], id)
}

func (j *judge) row(in ImportRow) (ImportAccepted, []ImportIssue) {
	out := ImportAccepted{Line: in.Line}
	var issues []ImportIssue
	refuse := func(field ImportField, code ImportCode) {
		issues = append(issues, ImportIssue{Line: in.Line, Field: field, Code: code})
	}

	name, code := cleanName(in.FullName)
	if code != "" {
		refuse(FieldFullName, code)
	}
	out.FullName = name

	email, code := j.address(in.Email)
	if code != "" {
		refuse(FieldEmail, code)
	}
	out.Email = email
	_, out.Existing = j.existing[strings.ToLower(email)]

	role, code := j.role(in.Role)
	if code != "" {
		refuse(FieldRole, code)
	}
	out.RoleID = role.ID

	classID, code := j.class(in.Class, role)
	if code != "" {
		refuse(FieldClass, code)
	}
	out.ClassID = classID
	return out, issues
}

func (j *judge) address(cell string) (string, ImportCode) {
	email := cleanEmail(cell)
	switch {
	case email == "":
		return email, CodeRequired
	case utf8.RuneCountInString(email) > MaxEmailLength:
		return email, CodeTooLong
	case !validEmail(email):
		return email, CodeInvalid
	}
	key := strings.ToLower(email)
	if _, again := j.seen[key]; again {
		return email, CodeDuplicateInFile
	}
	j.seen[key] = struct{}{}
	return email, ""
}

func (j *judge) role(cell string) (ImportRole, ImportCode) {
	token := foldText(cell)
	if token == "" {
		return j.pick([]string{j.defaultRole})
	}
	ids := j.rolesByToken[token]
	if len(ids) == 0 {
		return ImportRole{}, CodeUnknownRole
	}
	if len(ids) > 1 {
		return ImportRole{}, CodeAmbiguousRole
	}
	return j.pick(ids)
}

func (j *judge) pick(ids []string) (ImportRole, ImportCode) {
	role, ok := j.roles[ids[0]]
	switch {
	case !ok:
		return ImportRole{}, CodeUnknownRole
	case role.Refusal == RoleNeverGiven:
		return ImportRole{}, CodeRoleNotGivable
	case role.Refusal == RoleBeyondImporter:
		return ImportRole{}, CodeRoleNotAllowed
	}
	return role, ""
}

func (j *judge) class(cell string, role ImportRole) (string, ImportCode) {
	token := foldText(cell)
	if token == "" {
		return "", ""
	}
	if !j.place || (role.ID != "" && !role.StudentTarget) {
		return "", CodeClassNotAllowed
	}
	found := j.classes[token]
	switch len(found) {
	case 0:
		return "", CodeUnknownClass
	case 1:
		return found[0].ID, ""
	}
	return "", CodeAmbiguousClass
}

func cleanName(cell string) (string, ImportCode) {
	name := content.NFC(strings.Join(strings.Fields(cell), " "))
	switch {
	case name == "":
		return name, CodeRequired
	case utf8.RuneCountInString(name) > MaxFullNameLength:
		return name, CodeTooLong
	case strings.ContainsFunc(name, func(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }):
		return name, CodeInvalid
	}
	return name, ""
}

func cleanEmail(cell string) string { return strings.TrimSpace(content.NFC(cell)) }

func validEmail(email string) bool {
	if email == "" {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email
}

func foldText(s string) string {
	return strings.ToLower(content.NFC(strings.Join(strings.Fields(s), " ")))
}
