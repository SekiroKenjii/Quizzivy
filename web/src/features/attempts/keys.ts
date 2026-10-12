/** Query keys the monitor, the review and the timeline share, so an intervention on one refreshes the others. */
export const POLL_MS = 15_000;
export const monitorKey = (assignmentId: string) =>
  ["teacher-monitor", assignmentId] as const;
export const reviewKey = (attemptId: string) => ["teacher-attempt", attemptId] as const;
export const eventsKey = (attemptId: string) =>
  ["teacher-attempt-events", attemptId] as const;
export const answersKey = (assignmentId: string, questionId: string) =>
  ["teacher-answers-by-question", assignmentId, questionId] as const;

/** gradingKey isolates each actor and filter reading while retaining queue-prefix invalidation. */
export const gradingKey = (
  actor: number,
  mode: string,
  assignment: string,
  student: string,
) => ["teacher-grading", actor, mode, assignment, student] as const;
