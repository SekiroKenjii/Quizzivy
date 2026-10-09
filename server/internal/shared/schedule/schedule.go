// Package schedule holds the one rule that derives an assignment's status
// from its window, as SQL, so every list, filter and count of assignments by
// status agrees whichever module asks.
package schedule

// DerivedStatus is the status of an assignment as a SQL expression over a row
// of app.assignments aliased a, one of draft, scheduled, open or closed. It is
// never stored (D-18): an assignment is a draft until it is published, closed
// once its early close has passed or its window has ended, scheduled before
// it opens, and open in between.
const DerivedStatus = `
CASE
  WHEN a.published_at IS NULL THEN 'draft'
  WHEN a.closed_at IS NOT NULL AND now() >= a.closed_at THEN 'closed'
  WHEN now() < a.opens_at THEN 'scheduled'
  WHEN now() < a.closes_at THEN 'open'
  ELSE 'closed'
END`
