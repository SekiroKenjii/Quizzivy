import type { PickedVersion } from "@/features/assignments/components/TestVersionPicker";
import type { Token } from "@/features/assignments/components/TokenField";
import type { Assignment, AssignmentInput } from "@/features/assignments/api";
import type { components } from "@/lib/api/schema";
import type { TestVersion } from "@/features/tests/api";
import { formatInTimeZone, fromZonedTime } from "date-fns-tz";
import { APP_TIME_ZONE } from "@/lib/i18n/datetime";

/**
 * AssignmentDraft is every field of an assignment as the form and the wizard
 * hold it: the picked version, the class and student targets as tokens, the
 * window as `datetime-local` values in WINDOW_ZONE, the timing, order,
 * review and integrity choices, and the note to students, empty for none.
 * Saving a draft sends all of it (DG-65).
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
  studentNote: string;
}

/**
 * WINDOW_ZONE is the zone an assignment's window is read and written in:
 * the app's, as every teacher wall-clock input is until T-R4.43 (spec v0.57)
 * moves them all to the account's zone together.
 */
export const WINDOW_ZONE = APP_TIME_ZONE;

/** windowInput is `at` as a `datetime-local` value in WINDOW_ZONE. */
export function windowInput(at: string | Date): string {
  return formatInTimeZone(at, WINDOW_ZONE, "yyyy-MM-dd'T'HH:mm");
}

/** windowInstant is the instant a `datetime-local` value names in WINDOW_ZONE. */
export function windowInstant(value: string): Date {
  return fromZonedTime(value, WINDOW_ZONE);
}

/** AssignmentDefaults is the teacher's stored starting point for a new assignment. */
export type AssignmentDefaults = NonNullable<
  components["schemas"]["UserPreferences"]["assignmentDefaults"]
>;

/**
 * emptyDraft is a new assignment: a window from the current hour for three
 * days, one attempt, options in their order, no correct answers or
 * explanations, results on submitting without the class average, no limit
 * on leaving, flag, 3 s of grace and no note. The time limit, question
 * shuffling, the score, copy and paste and fullscreen come from `defaults`
 * where the teacher stored them, and otherwise are §10.3's: 45 minutes, not
 * shuffled, shown, blocked and off.
 */
export function emptyDraft(
  now = new Date(),
  defaults: AssignmentDefaults = {},
): AssignmentDraft {
  const opens = new Date(now);
  opens.setMinutes(0, 0, 0);
  const closes = new Date(opens.getTime() + 3 * 24 * 60 * 60 * 1000);
  return {
    picked: null,
    classes: [],
    students: [],
    opensAt: windowInput(opens),
    closesAt: windowInput(closes),
    durationMinutes: defaults.durationMinutes ?? 45,
    maxAttempts: 1,
    shuffleQuestions: defaults.shuffleQuestions ?? false,
    shuffleOptions: false,
    review: {
      showScore: defaults.showScore ?? true,
      showCorrectAnswers: false,
      showExplanations: false,
      release: "on_submit",
      showClassAverage: false,
    },
    integrity: {
      requireFullscreen: defaults.requireFullscreen ?? false,
      blockCopyPaste: defaults.blockCopyPaste ?? true,
      maxFocusLoss: 0,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
    studentNote: "",
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
      opensAt: windowInstant(draft.opensAt).toISOString(),
      closesAt: windowInstant(draft.closesAt).toISOString(),
    },
    durationMinutes: draft.durationMinutes,
    maxAttempts: draft.maxAttempts,
    shuffleQuestions: draft.shuffleQuestions,
    shuffleOptions: draft.shuffleOptions,
    review: draft.review,
    integrity: draft.integrity,
    studentNote: draft.studentNote === "" ? null : draft.studentNote,
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
    opensAt: windowInput(a.window.opensAt),
    closesAt: windowInput(a.window.closesAt),
    durationMinutes: a.durationMinutes,
    maxAttempts: a.maxAttempts,
    shuffleQuestions: a.shuffleQuestions,
    shuffleOptions: a.shuffleOptions,
    review: a.review,
    integrity: { ...a.integrity },
    studentNote: a.studentNote ?? "",
  };
}
