// Package answered holds the one rule for when a saved answer says
// something, as SQL, so every count of answered questions agrees with the
// student's navigator.
package answered

import "fmt"

const ecmaScriptWhitespace = `E'\u0009\u000A\u000B\u000C\u000D\u0020\u00A0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200A\u2028\u2029\u202F\u205F\u3000\uFEFF'`

// SaysSomething is the rule as a boolean SQL expression over a row of
// app.attempt_answers aliased alias: a choice with an option picked, a
// true/false with a value, a text that is not blank, a fill-in with every
// blank of the frozen question filled. Blank is what JavaScript's trim()
// leaves empty; an answer of any other type says nothing.
func SaysSomething(alias string) string {
	return fmt.Sprintf(`
	CASE %[1]s.payload->>'type'
	  WHEN 'choice' THEN
	       CASE WHEN jsonb_typeof(%[1]s.payload->'optionIds') = 'array'
	            THEN jsonb_array_length(%[1]s.payload->'optionIds') > 0
	            ELSE false END
	  WHEN 'true_false' THEN coalesce(jsonb_typeof(%[1]s.payload->'value') = 'boolean', false)
	  WHEN 'text' THEN btrim(coalesce(%[1]s.payload->>'value', ''), `+ecmaScriptWhitespace+`) <> ''
	  WHEN 'fill_blank' THEN
	       EXISTS (SELECT 1 FROM app.test_version_blanks b
	                WHERE b.test_version_question_id = %[1]s.question_id)
	   AND NOT EXISTS (SELECT 1 FROM app.test_version_blanks b
	                    WHERE b.test_version_question_id = %[1]s.question_id
	                      AND btrim(coalesce(%[1]s.payload->'values'->>b.id::text, ''), `+ecmaScriptWhitespace+`) = '')
	  ELSE false
	END`, alias)
}
