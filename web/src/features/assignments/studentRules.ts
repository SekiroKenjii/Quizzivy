import type { TFunction } from "i18next";
import type { IntegrityPolicy, ReviewPolicy } from "@/features/assignments/api";

export interface RulesDraft {
  durationMinutes: number;
  maxAttempts: number;
  review: ReviewPolicy;
  integrity: IntegrityPolicy;
  /** Set by the intro, which knows the paper; the teacher's form does not. */
  audio?: { maxPlays: number | null };
}

/**
 * One sentence the student reads before starting, with what it is about so
 * the intro can put the right icon beside it. The text is the contract; the
 * kind is presentation.
 */
export interface Rule {
  kind:
    | "clock"
    | "attempts"
    | "fullscreen"
    | "copy"
    | "focusLoss"
    | "audio"
    | "autosave"
    | "honest";
  text: string;
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
  return [
    ...duringRules(draft, t).map((r) => r.text),
    afterSubmit(draft.review, t, locale),
  ];
}

/** "Khi làm bài": everything that is true while the clock runs. */
export function duringRules(draft: RulesDraft, t: TFunction): Rule[] {
  const rules: Rule[] = [
    {
      kind: "clock",
      text: t("assignments.rules.clock", { minutes: draft.durationMinutes }),
    },
  ];

  if (draft.maxAttempts > 1) {
    rules.push({
      kind: "attempts",
      text: t("assignments.rules.attempts", { count: draft.maxAttempts }),
    });
  }
  if (draft.audio) {
    rules.push({
      kind: "audio",
      text:
        draft.audio.maxPlays === null
          ? t("assignments.rules.audioUnlimited")
          : t("assignments.rules.audioLimited", { count: draft.audio.maxPlays }),
    });
  }
  if (draft.integrity.requireFullscreen) {
    rules.push({ kind: "fullscreen", text: t("assignments.rules.fullscreen") });
  }
  if (draft.integrity.blockCopyPaste) {
    rules.push({ kind: "copy", text: t("assignments.rules.noCopyPaste") });
  }
  if (draft.integrity.maxFocusLoss > 0) {
    rules.push({
      kind: "focusLoss",
      text: t(`assignments.rules.focusLoss.${draft.integrity.onLimitExceeded}`, {
        count: draft.integrity.maxFocusLoss,
      }),
    });
    // §10.5's honest limits, stated where the rule is: the monitoring sees
    // this tab losing focus and nothing else, and the teacher decides.
    rules.push({ kind: "honest", text: t("integrity.honestLimits") });
  }
  rules.push({ kind: "autosave", text: t("assignments.rules.autosave") });
  return rules;
}

// Names what the student will NOT see as well as what they will, which is the
// half that stops them hunting for a missing answer key.
export function afterSubmit(
  review: ReviewPolicy,
  t: TFunction,
  locale: string,
): string {
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
