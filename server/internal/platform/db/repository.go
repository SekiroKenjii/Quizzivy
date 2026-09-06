package db

import "strings"

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// Repository is the base every module's Postgres repository embeds: the
// database context, and through it the transaction and query helpers.
type Repository struct {
	Context
}

func NewRepository(dbx Context) Repository { return Repository{Context: dbx} }
