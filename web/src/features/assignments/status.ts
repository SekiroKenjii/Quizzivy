import type { Assignment, AssignmentStatus } from "@/features/assignments/api";

export const STATUS_VARIANT: Record<
  AssignmentStatus,
  "success" | "secondary" | "outline"
> = {
  draft: "secondary",
  open: "success",
  scheduled: "secondary",
  closed: "outline",
};

/**
 * D-18's derived status, recomputed from the timestamps the row already
 * carries.
 */
export function statusAt(assignment: Assignment, now: Date): AssignmentStatus {
  if (!assignment.publishedAt) return "draft";

  const { opensAt, closesAt, closedAt } = assignment.window;
  if (closedAt && now >= new Date(closedAt)) return "closed";
  if (now < new Date(opensAt)) return "scheduled";
  if (now < new Date(closesAt)) return "open";
  return "closed";
}
