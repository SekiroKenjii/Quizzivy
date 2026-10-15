package support

import (
	"context"
	"errors"

	accessmodel "quizzivy/internal/modules/access/application/model"
	accessquery "quizzivy/internal/modules/access/application/query"
	accessdomain "quizzivy/internal/modules/access/domain"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/application/ports"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/access"
)

// Imports carries what the user-import handlers share. One file is read at a
// time: a workbook is unpacked into memory, and a slot keeps two uploads from
// holding their parsed tables at once.
type Imports struct {
	Repo    domain.UserImports
	Roles   ports.RoleChoices
	Tables  ports.TableReader
	Budget  ports.EmailBudget
	Wake    ports.Wake
	Service *Service
	Sealer  ports.Sealer

	slot chan struct{}
}

// NewImports returns the helper over its ports. Budget and wake may be nil, and
// so may sealer, which a server without SECRETS_KEY cannot have: neither a link
// in an email nor a hand-out sheet's passwords can be delivered without it.
func NewImports(repo domain.UserImports, roles ports.RoleChoices, tables ports.TableReader, budget ports.EmailBudget, wake ports.Wake, service *Service, sealer ports.Sealer) *Imports {
	return &Imports{Repo: repo, Roles: roles, Tables: tables, Budget: budget, Wake: wake, Service: service, Sealer: sealer, slot: make(chan struct{}, 1)}
}

// Upload is a file offered for import with what the dialog chose.
type Upload struct {
	File        []byte
	DefaultRole domain.ImportDefaultRole
	Delivery    domain.ImportDelivery
}

// Judged is what a file comes to under the caller's rights.
type Judged struct {
	Plan          domain.ImportPlan
	Format        domain.ImportFormat
	Rows          int
	DefaultRoleID string
}

// Judge reads the file and judges its rows, writing nothing. It fails with the
// table reader's errors, *domain.ImportHeaderError, the access module's
// ErrTargetRoleNotAllowed for a default role the caller may not give,
// domain.ErrPasswordSignInDisabled for a sheet while password sign-in is off,
// domain.ErrSetPasswordUnavailable for email links on a server that cannot
// send one, and domain.ErrImportSheetsOff for a sheet it cannot keep.
func (i *Imports) Judge(ctx context.Context, req domain.WriteRequest, up Upload) (Judged, error) {
	if err := i.checkDelivery(ctx, up.Delivery); err != nil {
		return Judged{}, err
	}
	choices, err := i.Roles.Handle(ctx, accessquery.RoleChoices{By: Caller(req)})
	if err != nil {
		return Judged{}, err
	}
	rules, err := i.rules(ctx, req, choices, up.DefaultRole)
	if err != nil {
		return Judged{}, err
	}
	format := domain.SniffImportFormat(up.File)
	rows, err := i.read(ctx, up.File, format)
	if err != nil {
		return Judged{}, err
	}
	existing, err := i.Repo.ExistingEmails(ctx, domain.ImportEmails(rows))
	if err != nil {
		return Judged{}, err
	}
	return Judged{
		Plan:          domain.PlanImport(rows, rules, existing),
		Format:        format,
		Rows:          len(rows),
		DefaultRoleID: rules.DefaultRoleID,
	}, nil
}

func (i *Imports) checkDelivery(ctx context.Context, delivery domain.ImportDelivery) error {
	switch delivery {
	case domain.ImportEmailLink:
		if i.Service.Links == nil || i.Sealer == nil {
			return domain.ErrSetPasswordUnavailable
		}
	case domain.ImportSheet:
		if i.Sealer == nil {
			return domain.ErrImportSheetsOff
		}
		policy, err := i.Service.SignInPolicy(ctx)
		if err != nil {
			return err
		}
		if !policy.Password {
			return domain.ErrPasswordSignInDisabled
		}
	default:
		return errors.New("user import: unknown delivery")
	}
	return nil
}

func (i *Imports) rules(ctx context.Context, req domain.WriteRequest, choices []accessmodel.RoleChoice, defaultRole domain.ImportDefaultRole) (domain.ImportRules, error) {
	rules := domain.ImportRules{Roles: make([]domain.ImportRole, 0, len(choices)), PlaceInClass: req.All}
	key, ok := defaultKey(defaultRole)
	if !ok {
		return domain.ImportRules{}, errors.New("user import: unknown default role")
	}
	for _, c := range choices {
		rules.Roles = append(rules.Roles, domain.ImportRole{
			ID: c.ID, Name: c.Name, BuiltinKey: string(c.Builtin), StudentTarget: c.StudentTarget, Refusal: refusal(c.Verdict),
		})
		if c.Builtin != key {
			continue
		}
		if c.Verdict != accessmodel.RoleGivable {
			return domain.ImportRules{}, accessdomain.ErrTargetRoleNotAllowed
		}
		rules.DefaultRoleID = c.ID
	}
	if rules.DefaultRoleID == "" {
		return domain.ImportRules{}, accessdomain.ErrRoleNotFound
	}
	if req.All {
		classes, err := i.Repo.ActiveClasses(ctx)
		if err != nil {
			return domain.ImportRules{}, err
		}
		rules.Classes = classes
	}
	return rules, nil
}

func defaultKey(role domain.ImportDefaultRole) (access.Builtin, bool) {
	switch role {
	case domain.DefaultStudent:
		return access.BuiltinStudent, true
	case domain.DefaultTeacher:
		return access.BuiltinTeacher, true
	}
	return "", false
}

func refusal(v accessmodel.RoleVerdict) domain.ImportRoleRefusal {
	switch v {
	case accessmodel.RoleBeyondCaller:
		return domain.RoleBeyondImporter
	case accessmodel.RoleNeverGiven:
		return domain.RoleNeverGiven
	}
	return domain.RoleGivable
}

func (i *Imports) read(ctx context.Context, file []byte, format domain.ImportFormat) ([]domain.ImportRow, error) {
	select {
	case i.slot <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	source, err := i.Tables.Read(file, format)
	<-i.slot
	if err != nil {
		return nil, err
	}
	return source.ImportRows()
}

// Allowance returns how many messages the centre can still send today, and
// false for a delivery that sends none or a server that does not count.
func (i *Imports) Allowance(ctx context.Context, delivery domain.ImportDelivery) (model.EmailAllowance, bool, error) {
	if delivery != domain.ImportEmailLink || i.Budget == nil {
		return model.EmailAllowance{}, false, nil
	}
	allowance, err := i.Budget.Remaining(ctx, i.Service.Now())
	if err != nil {
		return model.EmailAllowance{}, false, err
	}
	return allowance, true, nil
}
