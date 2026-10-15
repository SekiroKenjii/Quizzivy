package command

import (
	"context"

	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/application/model"
	"quizzivy/internal/modules/identity/domain"
)

// PreviewUserImport asks what a file would come to if it were imported now.
type PreviewUserImport struct {
	Request     domain.WriteRequest
	File        []byte
	DefaultRole domain.ImportDefaultRole
	Delivery    domain.ImportDelivery
}

// UserImportPreview is what a file comes to, before anything is stored. Rows is
// the data rows the file holds; Issues lists at most
// domain.MaxImportIssuesShown refused cells, and InvalidRows counts every
// refused row. Budget is nil for a delivery that sends no email.
type UserImportPreview struct {
	Rows           int
	NewAccounts    int
	ExistingCount  int
	ExistingSample []string
	Issues         []domain.ImportIssue
	InvalidRows    int
	Classes        []domain.ImportClass
	Budget         *model.EmailAllowance
}

type PreviewUserImportHandler struct {
	*support.Imports
}

// Handle reads and judges the file and writes nothing. It fails as
// support.Imports.Judge does.
func (h PreviewUserImportHandler) Handle(ctx context.Context, cmd PreviewUserImport) (UserImportPreview, error) {
	judged, err := h.Judge(ctx, cmd.Request, support.Upload{File: cmd.File, DefaultRole: cmd.DefaultRole, Delivery: cmd.Delivery})
	if err != nil {
		return UserImportPreview{}, err
	}
	plan := judged.Plan
	out := UserImportPreview{
		Rows:           judged.Rows,
		NewAccounts:    plan.NewAccounts,
		ExistingCount:  plan.ExistingCount,
		ExistingSample: plan.ExistingSample,
		Issues:         plan.Issues[:min(len(plan.Issues), domain.MaxImportIssuesShown)],
		InvalidRows:    plan.InvalidRows,
		Classes:        plan.Classes,
	}
	allowance, ok, err := h.Allowance(ctx, cmd.Delivery)
	if err != nil {
		return UserImportPreview{}, err
	}
	if ok {
		out.Budget = &allowance
	}
	return out, nil
}
