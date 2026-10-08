package repositories

import "quizzivy/internal/shared/schedule"

const testAssignments = `SELECT count(*)
	  FROM app.assignments a
	  JOIN app.test_versions av ON av.id = a.test_version_id
	 WHERE av.test_id = t.id AND ` + schedule.DerivedStatus

const testAssignmentColumns = `
	       (` + testAssignments + ` = 'open'),
	       (` + testAssignments + ` = 'scheduled'),
	       (` + testAssignments + ` = 'closed'),`

const versionAssignmentCount = `(SELECT count(*) FROM app.assignments a WHERE a.test_version_id = v.id)`
