import type { PickedVersion } from "@/features/assignments/components/TestVersionPicker";
import type { Token } from "@/features/assignments/components/TokenField";
import type { Assignment, AssignmentInput } from "@/features/assignments/api";
import type { TestVersion } from "@/features/tests/api";
import { fromDateTimeInput, toDateTimeInput } from "@/lib/i18n/datetime";

/**
 * AssignmentDraft is every field of an assignment as the form and the wizard
 * hold it: the picked version, the class and student targets as tokens, the
 * window as `datetime-local` values in the display zone, and the timing,
 * order, review and integrity choices. Saving a draft sends all of it (DG-65).
 */
export interface AssignmentDraft {
  picked: PickedVersion | null;
  classes: Token[];
  students: Token[];
  opensAt: string;
  closesAt: string;
  durationMinutes: number;
  maxAttempts: number;
  shuffleQuestions: boolean;
  shuffleOptions: boolean;
  review: AssignmentInput["review"];
  integrity: {
    requireFullscreen: boolean;
    blockCopyPaste: boolean;
    maxFocusLoss: number;
    onLimitExceeded: "warn" | "flag" | "auto_submit";
    minAwayMs: number;
  };
}

/**
 * emptyDraft is §10.3's conservative defaults: a window from the current hour
 * for three days, 45 minutes, one attempt, nothing shuffled, the score shown,
 * copy and paste blocked, no limit on leaving, flag, and 3 s of grace.
 */
export function emptyDraft(now = new Date()): AssignmentDraft {
  const opens = new Date(now);
  opens.setMinutes(0, 0, 0);
  const closes = new Date(opens.getTime() + 3 * 24 * 60 * 60 * 1000);
  return {
    picked: null,
    classes: [],
    students: [],
    opensAt: toDateTimeInput(opens),
    closesAt: toDateTimeInput(closes),
    durationMinutes: 45,
    maxAttempts: 1,
    shuffleQuestions: false,
    shuffleOptions: false,
    review: { showScore: true, showCorrectAnswers: false, showExplanations: false },
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: true,
      maxFocusLoss: 0,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
  };
}

/** draftBody is the request body for `draft`, which must have a picked version. */
export function draftBody(draft: AssignmentDraft): Omit<AssignmentInput, "draft"> {
  const picked = draft.picked;
  if (!picked) throw new Error("no version picked");
  return {
    testVersionId: picked.version.id,
    targets: {
      classIds: draft.classes.map((c) => c.id),
      studentIds: draft.students.map((s) => s.id),
    },
    window: {
      opensAt: fromDateTimeInput(draft.opensAt).toISOString(),
      closesAt: fromDateTimeInput(draft.closesAt).toISOString(),
    },
    durationMinutes: draft.durationMinutes,
    maxAttempts: draft.maxAttempts,
    shuffleQuestions: draft.shuffleQuestions,
    shuffleOptions: draft.shuffleOptions,
    review: draft.review,
    integrity: draft.integrity,
  };
}

/** draftOf reads a stored assignment back into a draft, with its version from `versions`. */
export function draftOf(a: Assignment, versions: TestVersion[]): AssignmentDraft {
  const version = versions.find((v) => v.id === a.testVersionId);
  return {
    picked: version ? { testId: a.testId, testTitle: a.testTitle, version } : null,
    classes: a.targets.classes.map((c) => ({
      id: c.id,
      label: c.name,
      hint: String(c.studentCount),
    })),
    students: a.targets.students.map((s) => ({ id: s.id, label: s.name })),
    opensAt: toDateTimeInput(a.window.opensAt),
    closesAt: toDateTimeInput(a.window.closesAt),
    durationMinutes: a.durationMinutes,
    maxAttempts: a.maxAttempts,
    shuffleQuestions: a.shuffleQuestions,
    shuffleOptions: a.shuffleOptions,
    review: a.review,
    integrity: { ...a.integrity },
  };
}
