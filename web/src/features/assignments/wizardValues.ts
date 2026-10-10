import type { TFunction } from "i18next";
import {
  WINDOW_ZONE,
  windowInstant,
  type AssignmentDraft,
} from "@/features/assignments/draft";
import type { Locale } from "@/lib/i18n";
import { dayDate } from "@/lib/i18n/datetime";

const DAY_MS = 24 * 60 * 60 * 1000;

/** windowIsValid reports whether the draft's window closes after it opens. */
export function windowIsValid(draft: Pick<AssignmentDraft, "opensAt" | "closesAt">) {
  const opens = windowInstant(draft.opensAt).getTime();
  const closes = windowInstant(draft.closesAt).getTime();
  return Number.isFinite(opens) && Number.isFinite(closes) && closes > opens;
}

/** windowDays is how many days the window is open, rounded up, and 0 when it is invalid. */
export function windowDays(draft: Pick<AssignmentDraft, "opensAt" | "closesAt">) {
  if (!windowIsValid(draft)) return 0;
  const span =
    windowInstant(draft.closesAt).getTime() - windowInstant(draft.opensAt).getTime();
  return Math.ceil(span / DAY_MS);
}

/**
 * firstGap is the index of the first step that keeps the draft from being
 * assigned, with the locale key of what it lacks, or null when it is ready:
 * a test (step 0), a class or a student (step 1), a window that closes after
 * it opens (step 2).
 */
export function firstGap(draft: AssignmentDraft): { step: number; key: string } | null {
  if (draft.picked === null)
    return { step: 0, key: "assignments.wizard.needTestAssign" };
  if (draft.classes.length === 0 && draft.students.length === 0)
    return { step: 1, key: "assignments.wizard.needTargets" };
  if (!windowIsValid(draft))
    return { step: 2, key: "assignments.wizard.windowInvalid" };
  return null;
}

/**
 * stepValues is the line under each step's name in the stepper: the test,
 * who takes it, the window and time limit, and the rules that are on.
 */
export function stepValues(
  draft: AssignmentDraft,
  t: TFunction,
  locale: Locale,
): string[] {
  const targets = [
    ...draft.classes.map((klass) => klass.label),
    ...(draft.students.length > 0
      ? [t("assignments.wizard.value.individuals", { count: draft.students.length })]
      : []),
  ];
  const leaving = draft.integrity.maxFocusLoss;
  const rules = [
    draft.integrity.requireFullscreen && t("assignments.wizard.value.fullscreen"),
    draft.integrity.blockCopyPaste && t("assignments.wizard.value.noCopy"),
    leaving === -1 && t("assignments.wizard.value.noLeaving"),
    leaving > 0 && t("assignments.wizard.value.leavingAllowed", { count: leaving }),
    draft.review.showScore && t("assignments.wizard.value.scoreShown"),
  ].filter((part): part is string => typeof part === "string");
  const dated = windowIsValid(draft);
  return [
    draft.picked?.testTitle ?? t("assignments.wizard.value.noTest"),
    targets.length > 0 ? targets.join(", ") : t("assignments.wizard.value.noOne"),
    dated
      ? t("assignments.wizard.value.schedule", {
          opens: dayDate(windowInstant(draft.opensAt), locale, WINDOW_ZONE),
          closes: dayDate(windowInstant(draft.closesAt), locale, WINDOW_ZONE),
          minutes: draft.durationMinutes,
        })
      : t("assignments.wizard.minutes", { count: draft.durationMinutes }),
    rules.length > 0 ? rules.join(" · ") : t("assignments.wizard.value.defaultRules"),
  ];
}

/** NOTE_LIMIT is the most characters a note to students may hold, counted before trimming. */
export const NOTE_LIMIT = 500;

type Review = AssignmentDraft["review"];
type Integrity = AssignmentDraft["integrity"];

/** RulesPatch is the part of a draft the Rules step changes. */
export type RulesPatch = Partial<
  Pick<AssignmentDraft, "shuffleQuestions" | "shuffleOptions" | "studentNote">
> & {
  review?: Partial<Review>;
  integrity?: Partial<Integrity>;
};

/** Leaving is the deck's three choices for leaving the test. */
export type Leaving = "unlimited" | "none" | "limit";

/**
 * leavingMode reads `maxFocusLoss` as the deck's three choices: 0 is no
 * limit, -1 is not allowed, and a positive number is a limit.
 */
export function leavingMode(maxFocusLoss: number): Leaving {
  if (maxFocusLoss === 0) return "unlimited";
  return maxFocusLoss < 0 ? "none" : "limit";
}

/**
 * leavingValue is the `maxFocusLoss` a choice of `mode` stores: 0, -1, or
 * the current limit when there is one and 2 otherwise.
 */
export function leavingValue(mode: Leaving, current: number): number {
  if (mode === "unlimited") return 0;
  if (mode === "none") return -1;
  return current > 0 ? current : 2;
}
