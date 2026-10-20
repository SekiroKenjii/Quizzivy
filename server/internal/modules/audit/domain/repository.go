package domain

import "context"

// Repository reads the audit log. It never changes an entry: the log is
// appended to by every other module, and the one write here is the audit of an
// export.
type Repository interface {
	// List returns up to limit records matching f, newest first, from offset.
	List(ctx context.Context, f Filter, offset, limit int) ([]Record, error)
	// Facets counts the entries matching f's search and window, ignoring its
	// category, and folds the count of each entity into the categories.
	Facets(ctx context.Context, f Filter) (Facets, error)
	// Labels returns the name of each row refs names that has one. A row that
	// is gone or has no name is absent.
	Labels(ctx context.Context, refs []EntityRef) (map[EntityRef]string, error)
	// CountExport counts the entries matching f, stopping once there are more
	// than MaxExportRows, under StatementTimeout.
	CountExport(ctx context.Context, f Filter) (ExportCount, error)
	// ExportBatch returns up to limit records matching f, newest first, after
	// the cursor or from the newest when it is nil, under StatementTimeout.
	ExportBatch(ctx context.Context, f Filter, after *Cursor, limit int) ([]Record, error)
	// RecordExport appends the entry that says who exported the log.
	RecordExport(ctx context.Context, e Export) error
}
