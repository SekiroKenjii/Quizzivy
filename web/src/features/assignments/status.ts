import type { Assignment, AssignmentStatus } from "@/features/assignments/api";

/**
 * D-18's derived status, recomputed from the timestamps the row already
 * carries.
 *
 * The server sends `status` too, but it is a fact about the moment the response
 * was built, and a cached page outlives that moment: an assignment that opened
 * at 08:00 would keep reading "sắp mở" until something refetched. Deriving it
 * costs nothing and uses the same rule the server does, so the two cannot
 * disagree about what "open" means.
 */
export function statusAt(assignment: Assignment, now: Date): AssignmentStatus {
  const { opensAt, closesAt, closedAt } = assignment.window;
  if (closedAt && now >= new Date(closedAt)) return "closed";
  if (now < new Date(opensAt)) return "scheduled";
  if (now < new Date(closesAt)) return "open";
  return "closed";
}
