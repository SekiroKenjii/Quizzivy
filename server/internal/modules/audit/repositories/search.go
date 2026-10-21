package repositories

import (
	"fmt"
	"strconv"
	"strings"

	"quizzivy/internal/modules/audit/domain"
	"quizzivy/internal/platform/db"
)

type clauses struct {
	conditions []string
	args       []any
}

func (c *clauses) arg(v any) string {
	c.args = append(c.args, v)
	return "$" + strconv.Itoa(len(c.args))
}

func (c *clauses) add(condition string) { c.conditions = append(c.conditions, condition) }

func conditions(f domain.Filter, withCategory bool) (string, []any) {
	var c clauses
	if f.Window.From != nil {
		c.add("a.occurred_at >= " + c.arg(*f.Window.From))
	}
	if f.Window.To != nil {
		c.add("a.occurred_at < " + c.arg(*f.Window.To))
	}
	if f.Through > 0 {
		c.add("a.id <= " + c.arg(f.Through))
	}
	if withCategory && f.Category != "" {
		c.add("a.entity = ANY(" + c.arg(domain.EntitiesOf(f.Category)) + "::text[])")
	}
	if f.Query != "" {
		c.add(c.search(f))
	}
	if len(c.conditions) == 0 {
		return "true", nil
	}
	return strings.Join(c.conditions, " AND "), c.args
}

func (c *clauses) search(f domain.Filter) string {
	pattern := c.arg(db.EscapeLike(f.Query))
	matches := func(expr string) string {
		return fmt.Sprintf(`app.immutable_unaccent(lower(%s)) LIKE '%%' || app.immutable_unaccent(lower(%s)) || '%%' ESCAPE '\'`, expr, pattern)
	}
	alternatives := []string{
		`EXISTS (SELECT 1 FROM app.users sa WHERE sa.id = a.actor_user_id AND (` +
			matches("sa.full_name") + ` OR ` + matches("sa.display_name") + ` OR ` + matches("sa.email") + `))`,
	}
	if len(f.Actions) > 0 {
		alternatives = append(alternatives, "a.action = ANY("+c.arg(f.Actions)+"::text[])")
	}
	for _, source := range labelSources {
		alternatives = append(alternatives, fmt.Sprintf(
			`(a.entity = ANY(%s::text[]) AND EXISTS (SELECT 1 FROM (%s) l WHERE l.id = a.entity_id AND %s))`,
			c.arg(source.entities), source.rows, matches("l.label")))
	}
	return "(" + strings.Join(alternatives, " OR ") + ")"
}
