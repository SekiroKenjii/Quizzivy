/** Query keys the monitor, the review and the timeline share, so an intervention on one refreshes the others. */
export const POLL_MS = 15_000;
export const monitorKey = (assignmentId: string) =>
  ["admin-monitor", assignmentId] as const;
export const reviewKey = (attemptId: string) => ["admin-attempt", attemptId] as const;
export const eventsKey = (attemptId: string) =>
  ["admin-attempt-events", attemptId] as const;
export const answersKey = (assignmentId: string, questionId: string) =>
  ["admin-answers-by-question", assignmentId, questionId] as const;

/** gradingKey isolates each actor and filter reading while retaining queue-prefix invalidation. */
export const gradingKey = (
  actor: number,
  mode: string,
  assignment: string,
  student: string,
) => ["teacher-grading", actor, mode, assignment, student] as const;
