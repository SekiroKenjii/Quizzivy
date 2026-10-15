package command

import (
	"context"

	"quizzivy/internal/modules/identity/application/internal/support"
	"quizzivy/internal/modules/identity/domain"
)

// DownloadUserImportSheet takes the temporary passwords a sheet job handed
// out, once.
type DownloadUserImportSheet struct {
	Request domain.WriteRequest
	ID      string
}

type DownloadUserImportSheetHandler struct {
	*support.Imports
}

// Handle returns a line for every account the job created, in file order, and
// erases the passwords it opened. It fails with domain.ErrUserImportNotFound
// for a job that does not exist or is another user's, and with
// domain.ErrSheetUnavailable for one with no sheet to give, and with
// domain.ErrImportSheetsOff on a server that cannot open a sealed password. A
// failure writes nothing and consumes nothing.
func (h DownloadUserImportSheetHandler) Handle(ctx context.Context, cmd DownloadUserImportSheet) ([]domain.SheetRow, error) {
	if h.Sealer == nil {
		return nil, domain.ErrImportSheetsOff
	}
	open := func(ordinal int, sealed []byte) (string, error) {
		password, err := h.Sealer.Open(sealed, domain.SheetAAD(cmd.ID, ordinal))
		return string(password), err
	}
	return h.Repo.TakeSheet(ctx, cmd.Request, cmd.ID, h.Service.Now(), open)
}
