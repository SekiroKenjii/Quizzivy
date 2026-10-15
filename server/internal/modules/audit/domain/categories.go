// Package domain is the audit log's read model: what an entry is, which tab of the
// Admin console's log it belongs to, how its action is worded, which date range
// a reader asked for, and how a stored diff is shown without a secret in it.
// The log itself is appended to by every other module and never changed.
package domain

// Category is one of the six tabs the audit log is filtered by.
type Category string

// The categories, in the order the console draws them.
const (
	Accounts Category = "accounts"
	Classes  Category = "classes"
	Attempts Category = "attempts"
	Tests    Category = "tests"
	Media    Category = "media"
	Imports  Category = "imports"
)

// Categories lists every category in display order.
func Categories() []Category {
	return []Category{Accounts, Classes, Attempts, Tests, Media, Imports}
}

// Valid reports whether c is one of the six.
func (c Category) Valid() bool {
	for _, known := range Categories() {
		if c == known {
			return true
		}
	}
	return false
}

var categoryOfEntity = map[string]Category{
	"user":                 Accounts,
	"users":                Accounts,
	"student":              Accounts,
	"user_identity":        Accounts,
	"user_import":          Accounts,
	"refresh_token_family": Accounts,
	"role":                 Accounts,

	"class":           Classes,
	"class_member":    Classes,
	"class_join_code": Classes,

	"attempt":    Attempts,
	"assignment": Attempts,

	"test":           Tests,
	"test_version":   Tests,
	"question":       Tests,
	"question_group": Tests,

	"media_asset": Media,

	"word_import": Imports,
}

var allOnlyEntities = []string{
	"ai_credit_periods",
	"ai_credit_requests",
	"ai_settings",
	"ai_teacher_credits",
	"attempt_events",
	"audit_log",
	"data_export",
	"maintenance_window",
	"organization",
}

// CategoryOf is the category an entry about entity belongs to. An entity in
// none, whether listed as All-only or not known at all, has ok false and
// appears only in the unfiltered list.
func CategoryOf(entity string) (category Category, ok bool) {
	category, ok = categoryOfEntity[entity]
	return category, ok
}

// EntitiesOf lists the entities whose entries are in c.
func EntitiesOf(c Category) []string {
	var out []string
	for entity, category := range categoryOfEntity {
		if category == c {
			out = append(out, entity)
		}
	}
	return out
}

// Entities lists every entity the log knows a place for: those in a category
// and those only the unfiltered list holds.
func Entities() []string {
	out := append([]string(nil), allOnlyEntities...)
	for entity := range categoryOfEntity {
		out = append(out, entity)
	}
	return out
}

// AllOnly reports whether entity is one the log knows to belong to no category.
func AllOnly(entity string) bool {
	for _, known := range allOnlyEntities {
		if known == entity {
			return true
		}
	}
	return false
}

// Facets is how many entries each category holds for one search and range,
// and how many entries there are in all.
type Facets struct {
	All      int
	Accounts int
	Classes  int
	Attempts int
	Tests    int
	Media    int
	Imports  int
}

// FoldFacets turns the count of entries per entity into Facets: All is every
// entry, the others the entries of the entities in that category.
func FoldFacets(perEntity map[string]int) Facets {
	var f Facets
	for entity, n := range perEntity {
		f.All += n
		category, ok := CategoryOf(entity)
		if !ok {
			continue
		}
		switch category {
		case Accounts:
			f.Accounts += n
		case Classes:
			f.Classes += n
		case Attempts:
			f.Attempts += n
		case Tests:
			f.Tests += n
		case Media:
			f.Media += n
		case Imports:
			f.Imports += n
		}
	}
	return f
}

// Of is the count under c, or All for the empty category.
func (f Facets) Of(c Category) int {
	switch c {
	case Accounts:
		return f.Accounts
	case Classes:
		return f.Classes
	case Attempts:
		return f.Attempts
	case Tests:
		return f.Tests
	case Media:
		return f.Media
	case Imports:
		return f.Imports
	}
	return f.All
}
