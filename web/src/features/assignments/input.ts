import type { Assignment, AssignmentInput } from "@/features/assignments/api";

/** The PATCH body that re-sends an assignment as it is, for publish and close. */
export function toInput(a: Assignment): Omit<AssignmentInput, "draft"> {
  return {
    testVersionId: a.testVersionId,
    targets: {
      classIds: a.targets.classes.map((c) => c.id),
      studentIds: a.targets.students.map((s) => s.id),
    },
    window: { opensAt: a.window.opensAt, closesAt: a.window.closesAt },
    durationMinutes: a.durationMinutes,
    maxAttempts: a.maxAttempts,
    shuffleQuestions: a.shuffleQuestions,
    shuffleOptions: a.shuffleOptions,
    review: a.review,
    integrity: a.integrity,
  };
}
