import type { TFunction } from "i18next";
import type { IntegrityPolicy, ReviewPolicy } from "@/features/assignments/api";

export interface RulesDraft {
  durationMinutes: number;
  maxAttempts: number;
  review: ReviewPolicy;
  integrity: IntegrityPolicy;
}

/**
 * The sentences §10.2 shows the student before they start, built from the
 * switches the teacher is looking at.
 *
 * Pure and shared so the intro page renders the same list from the same rule.
 * The point of G-01's panel is that these are the actual sentences; a preview
 * that drifts from the real thing is worse than no preview at all.
 */
export function studentRules(
  draft: RulesDraft,
  t: TFunction,
  locale: string,
): string[] {
  const rules = [t("assignments.rules.clock", { minutes: draft.durationMinutes })];

  if (draft.maxAttempts > 1) {
    rules.push(t("assignments.rules.attempts", { count: draft.maxAttempts }));
  }
  if (draft.integrity.requireFullscreen) {
    rules.push(t("assignments.rules.fullscreen"));
  }
  if (draft.integrity.blockCopyPaste) {
    rules.push(t("assignments.rules.noCopyPaste"));
  }
  if (draft.integrity.maxFocusLoss > 0) {
    rules.push(
      t(`assignments.rules.focusLoss.${draft.integrity.onLimitExceeded}`, {
        count: draft.integrity.maxFocusLoss,
      }),
    );
  }
  rules.push(afterSubmit(draft.review, t, locale));
  return rules;
}

// Names what the student will NOT see as well as what they will, which is the
// half that stops them hunting for a missing answer key.
function afterSubmit(review: ReviewPolicy, t: TFunction, locale: string): string {
  const nouns = [
    { on: review.showScore, noun: t("assignments.rules.score") },
    { on: review.showCorrectAnswers, noun: t("assignments.rules.correctAnswers") },
    { on: review.showExplanations, noun: t("assignments.rules.explanations") },
  ];

  const shown = nouns.filter((n) => n.on).map((n) => n.noun);
  const hidden = nouns.filter((n) => !n.on).map((n) => n.noun);
  if (shown.length === 0) return t("assignments.rules.afterSubmitNothing");

  const list = new Intl.ListFormat(locale, { style: "long", type: "conjunction" });
  return t("assignments.rules.afterSubmit", {
    shown: list.format(shown),
    hidden:
      hidden.length === 0
        ? ""
        : t("assignments.rules.butNot", { hidden: list.format(hidden) }),
  });
}
