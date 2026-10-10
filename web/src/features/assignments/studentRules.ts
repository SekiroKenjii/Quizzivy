import type { TFunction } from "i18next";
import type { IntegrityPolicy, ReviewPolicy } from "@/features/assignments/api";
import type { Locale } from "@/lib/i18n";
import {
  APP_TIME_ZONE,
  dayDate,
  formatTime,
  fromDateTimeInput,
  getDisplayTimeZone,
  sameAppDay,
} from "@/lib/i18n/datetime";

/**
 * RulesInput is what the sentences are generated from: the stored policies
 * and, where the caller knows them, the window, whether an attempt is already
 * running, the paper's audio and whether this browser has fullscreen. `window`
 * is left out for a closed assignment, about which no date would be true.
 */
export interface RulesInput {
  readonly review: Pick<
    ReviewPolicy,
    "showScore" | "showCorrectAnswers" | "showExplanations"
  > &
    Partial<Pick<ReviewPolicy, "release">>;
  readonly integrity: IntegrityPolicy;
  readonly window?: {
    readonly opensAt: string | Date;
    readonly closesAt: string | Date;
    readonly upcoming: boolean;
  };
  readonly running?: boolean;
  readonly audio?: { readonly maxPlays: number | null; readonly shared: boolean };
  readonly fullscreenSupported?: boolean;
}

/** RuleKind says what a sentence is about, so a screen can put an icon beside it. */
export type RuleKind =
  "availability" | "timer" | "fullscreen" | "copy" | "leaving" | "audio" | "score";

/** Rule is one sentence a student reads before starting, with a stable id. */
export interface Rule {
  readonly id: string;
  readonly kind: RuleKind;
  readonly text: string;
}

function moment(at: string | Date, t: TFunction, locale: Locale, zone: string): string {
  return t("assignments.rules.moment", {
    time: formatTime(at, locale, zone),
    date: dayDate(at, locale, zone),
  });
}

function availability(
  window: NonNullable<RulesInput["window"]>,
  t: TFunction,
  locale: Locale,
  now: Date,
  zone: string,
): string {
  const { opensAt, closesAt } = window;
  if (!window.upcoming)
    return sameAppDay(closesAt, now, zone)
      ? t("assignments.rules.availableUntilToday", {
          time: formatTime(closesAt, locale, zone),
        })
      : t("assignments.rules.availableUntil", {
          when: moment(closesAt, t, locale, zone),
        });
  if (!sameAppDay(opensAt, closesAt, zone))
    return t("assignments.rules.opens", {
      from: moment(opensAt, t, locale, zone),
      to: moment(closesAt, t, locale, zone),
    });
  const hours = {
    from: formatTime(opensAt, locale, zone),
    to: formatTime(closesAt, locale, zone),
  };
  return sameAppDay(opensAt, now, zone)
    ? t("assignments.rules.opensToday", hours)
    : t("assignments.rules.opensSameDay", {
        ...hours,
        date: dayDate(opensAt, locale, zone),
      });
}

function leaving(integrity: IntegrityPolicy, t: TFunction): string {
  const { maxFocusLoss: limit, onLimitExceeded: action } = integrity;
  if (limit === 0) return t("assignments.rules.leaving.recorded");
  return limit < 0
    ? t(`assignments.rules.leaving.none.${action}`)
    : t(`assignments.rules.leaving.limit.${action}`, { count: limit });
}

function score(review: RulesInput["review"], t: TFunction): string {
  const group =
    review.release === "after_close"
      ? "assignments.rules.scoreAfterClose"
      : "assignments.rules.score";
  if (review.showCorrectAnswers && review.showExplanations) return t(`${group}.all`);
  if (review.showCorrectAnswers) return t(`${group}.answers`);
  return review.showExplanations ? t(`${group}.explanations`) : t(`${group}.only`);
}

/** studentRules generates policy sentences using an explicit display zone while preserving engine behavior. */
export function studentRules(
  input: RulesInput,
  t: TFunction,
  locale: Locale,
  now: Date,
  zone = getDisplayTimeZone(),
): Rule[] {
  const { review, integrity, audio } = input;
  const rules: Rule[] = [];
  const add = (id: string, kind: RuleKind, text: string) =>
    rules.push({ id, kind, text });

  if (input.window)
    add(
      "availability",
      "availability",
      availability(input.window, t, locale, now, zone),
    );
  add(
    "timer",
    "timer",
    t(input.running ? "assignments.rules.timerRunning" : "assignments.rules.timer"),
  );
  if (integrity.requireFullscreen)
    add(
      "fullscreen",
      "fullscreen",
      t(
        input.fullscreenSupported === false
          ? "assignments.rules.fullscreenUnsupported"
          : "assignments.rules.fullscreen",
      ),
    );
  if (integrity.blockCopyPaste) add("copy", "copy", t("assignments.rules.noCopyPaste"));
  add("leaving", "leaving", leaving(integrity, t));
  if (audio)
    add(
      "audio",
      "audio",
      audio.maxPlays === null
        ? t("assignments.rules.audioUnlimited")
        : t("assignments.rules.audioLimited", { count: audio.maxPlays }),
    );
  if (audio?.shared) add("audio-shared", "audio", t("assignments.rules.audioShared"));
  if (review.showScore) add("score", "score", score(review, t));
  return rules;
}

/**
 * PreviewInput is what the teacher's form holds: the policies and the window
 * as `datetime-local` values.
 */
export interface PreviewInput {
  readonly review: RulesInput["review"];
  readonly integrity: IntegrityPolicy;
  readonly opensAt: string;
  readonly closesAt: string;
}

/**
 * previewRules is what the teacher is told students will read for `input`:
 * the student's own sentences, with the dates in the app's zone. A window
 * that is not filled in, is reversed or has already closed leaves the
 * availability sentence out.
 */
export function previewRules(
  input: PreviewInput,
  t: TFunction,
  locale: Locale,
  now: Date,
): Rule[] {
  const opensAt = fromDateTimeInput(input.opensAt);
  const closesAt = fromDateTimeInput(input.closesAt);
  const dated =
    closesAt.getTime() > opensAt.getTime() && closesAt.getTime() > now.getTime();
  return studentRules(
    {
      review: input.review,
      integrity: input.integrity,
      ...(dated
        ? { window: { opensAt, closesAt, upcoming: opensAt.getTime() > now.getTime() } }
        : {}),
    },
    t,
    locale,
    now,
    APP_TIME_ZONE,
  );
}
