// Package bands holds the one rule for an attempt's band, as SQL, so every
// AttemptScore that carries one agrees (T-R6.7).
package bands

import "fmt"

// OfPaper is the SQL expression for the band of the attempt aliased attempt,
// a row of app.attempts: the points-weighted mean of its essays' bands,
// rounded half up to 0.5, when every scored question of its version is an
// essay with a band, and NULL otherwise. A mixed paper, or an essay not yet
// graded or left blank, has no band.
func OfPaper(attempt string) string {
	return fmt.Sprintf(`(SELECT CASE WHEN bool_and(bq.type = 'essay' AND ba.band IS NOT NULL)
	                     THEN floor(sum(ba.band * bq.points) / sum(bq.points) * 2 + 0.5) / 2 END
	   FROM app.test_version_questions bq
	   JOIN app.test_version_sections bs ON bs.id = bq.test_version_section_id
	   LEFT JOIN app.attempt_answers ba ON ba.attempt_id = %[1]s.id AND ba.question_id = bq.id
	  WHERE bs.test_version_id = %[1]s.test_version_id AND bq.points > 0)`, attempt)
}
