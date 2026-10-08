package repositories

const assignmentStatus = `CASE
	WHEN a.published_at IS NULL THEN 'draft'
	WHEN a.closed_at IS NOT NULL AND now() >= a.closed_at THEN 'closed'
	WHEN now() < a.opens_at THEN 'scheduled'
	WHEN now() < a.closes_at THEN 'open'
	ELSE 'closed'
END`

const testAssignments = `SELECT count(*)
	  FROM app.assignments a
	  JOIN app.test_versions av ON av.id = a.test_version_id
	 WHERE av.test_id = t.id AND ` + assignmentStatus

const testAssignmentColumns = `
	       (` + testAssignments + ` = 'open'),
	       (` + testAssignments + ` = 'scheduled'),
	       (` + testAssignments + ` = 'closed'),`

const versionAssignmentCount = `(SELECT count(*) FROM app.assignments a WHERE a.test_version_id = v.id)`
