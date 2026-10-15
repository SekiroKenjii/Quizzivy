package command

import (
	"context"

	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

// CreateUserImport stores a file's accepted rows as a job for the worker. The
// file is read and judged again here; the preview is not trusted. RequestID is
// the caller's, held for as long as the dialog is open.
type CreateUserImport struct {
	Request     domain.WriteRequest
	RequestID   string
	Filename    string
	File        []byte
	DefaultRole domain.ImportDefaultRole
	Delivery    domain.ImportDelivery
}

// CreateUserImportResult is the job stored, with its sheet's state when it is
// delivered by sheet. Replayed is true when the same caller's earlier request
// with the same RequestID stored it.
type CreateUserImportResult struct {
	Job      domain.ImportJob
	Sheet    *domain.ImportSheetState
	Replayed bool
}

type CreateUserImportHandler struct {
	*support.Imports
}

// Handle fails as support.Imports.Judge does, and with
// domain.ErrImportNothingToCreate when no row of the file would create an
// account. A request id already used by the caller returns its job without
// reading the file.
func (h CreateUserImportHandler) Handle(ctx context.Context, cmd CreateUserImport) (CreateUserImportResult, error) {
	req := cmd.Request
	job, found, err := h.Repo.FindByRequest(ctx, req.ActorID, cmd.RequestID)
	if err != nil {
		return CreateUserImportResult{}, err
	}
	if found {
		return h.result(job, true), nil
	}
	judged, err := h.Judge(ctx, req, support.Upload{File: cmd.File, DefaultRole: cmd.DefaultRole, Delivery: cmd.Delivery})
	if err != nil {
		return CreateUserImportResult{}, err
	}
	if judged.Plan.NewAccounts == 0 {
		return CreateUserImportResult{}, domain.ErrImportNothingToCreate
	}
	job, replayed, err := h.Repo.Create(ctx, req, domain.NewImportJob{
		RequestID:     cmd.RequestID,
		Filename:      domain.ImportFilename(cmd.Filename),
		Format:        judged.Format,
		DefaultRoleID: judged.DefaultRoleID,
		Delivery:      cmd.Delivery,
		Rows:          judged.Plan.Accepted,
		Invalid:       judged.Plan.InvalidRows,
		Now:           h.Service.Now(),
	})
	if err != nil {
		return CreateUserImportResult{}, err
	}
	if !replayed && h.Wake != nil {
		h.Wake.Wake()
	}
	return h.result(job, replayed), nil
}

func (h CreateUserImportHandler) result(job domain.ImportJob, replayed bool) CreateUserImportResult {
	out := CreateUserImportResult{Job: job, Replayed: replayed}
	if state, ok := job.Sheet(h.Service.Now()); ok {
		out.Sheet = &state
	}
	return out
}
