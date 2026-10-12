package repositories

import "quizzivy/internal/shared/schedule"

const testUsage = `
	  CROSS JOIN LATERAL (
	    SELECT count(*) FILTER (WHERE status = 'open') AS live,
	           count(*) FILTER (WHERE status = 'scheduled') AS scheduled,
	           count(*) FILTER (WHERE status = 'closed') AS closed
	      FROM (SELECT ` + schedule.DerivedStatus + ` AS status
	              FROM app.assignments a
	              JOIN app.test_versions av ON av.id = a.test_version_id
	             WHERE av.test_id = t.id) counted
	  ) uses`

const versionAssignmentCount = `(SELECT count(*) FROM app.assignments a WHERE a.test_version_id = v.id)`
