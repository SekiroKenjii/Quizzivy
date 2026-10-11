import type { Assignment, AssignmentInput } from "./api";
import { windowInput, windowInstant, type AssignmentDraft } from "./draft";
import { toInput } from "./input";
import type { statusAt } from "./status";

/** SettingsGroup names one card of the assignment's Settings tab and the dialog that edits it. */
export type SettingsGroup = "timing" | "window" | "integrity" | "results" | "note";

/**
 * settingsBody is the PATCH body that saves `group` from `draft` and keeps
 * everything else as stored: the window is sent only by the Window group, and
 * an unchanged moment is sent as stored, to the second, so a save never moves
 * it by rounding to the minute. A draft stays a draft.
 */
export function settingsBody(
  group: SettingsGroup,
  a: Assignment,
  draft: AssignmentDraft,
  status: ReturnType<typeof statusAt>,
): AssignmentInput {
  const base: AssignmentInput = {
    ...toInput(a),
    studentNote: a.studentNote ?? null,
    draft: status === "draft",
  };
  switch (group) {
    case "timing":
      return {
        ...base,
        durationMinutes: draft.durationMinutes,
        maxAttempts: draft.maxAttempts,
        shuffleQuestions: draft.shuffleQuestions,
        shuffleOptions: draft.shuffleOptions,
      };
    case "window":
      return {
        ...base,
        window: {
          opensAt: moment(draft.opensAt, a.window.opensAt),
          closesAt: moment(draft.closesAt, a.window.closesAt),
        },
        targets: {
          classIds: draft.classes.map((c) => c.id),
          studentIds: base.targets.studentIds,
        },
      };
    case "integrity":
      return { ...base, integrity: draft.integrity };
    case "results":
      return { ...base, review: draft.review };
    case "note":
      return {
        ...base,
        studentNote: draft.studentNote.trim() === "" ? null : draft.studentNote,
      };
  }
}

function moment(local: string, stored: string): string {
  return local === windowInput(stored) ? stored : windowInstant(local).toISOString();
}
