import { appDaysUntil, appHour } from "@/lib/i18n/datetime";
import type { StudentAssignmentCard } from "./api";

const DAY_MS = 24 * 60 * 60 * 1000;

/** RECENT_RESULTS is how many results Home shows once Grades lists them all. */
export const RECENT_RESULTS = 3;

/** GreetingPeriod is the part of the day Home greets the student in. */
export type GreetingPeriod = "morning" | "afternoon" | "evening";

/**
 * greetingPeriod reads the app's clock, never the device's: morning from
 * 05:00, afternoon from 12:00, evening from 18:00 until 05:00.
 */
export function greetingPeriod(now: Date): GreetingPeriod {
  const hour = appHour(now);
  if (hour >= 5 && hour < 12) return "morning";
  if (hour >= 12 && hour < 18) return "afternoon";
  return "evening";
}

/**
 * minutesLeft is the whole minutes until `deadline`, and never less than one:
 * a live attempt is not told it has no time while the server still calls it
 * live.
 */
export function minutesLeft(deadline: string, now: Date): number {
  return Math.max(1, Math.floor((Date.parse(deadline) - now.getTime()) / 60_000));
}

/**
 * ComingUpPill is what a Coming up row says about its paper. A paper being
 * taken is in progress. An open paper is due today or tomorrow when it closes
 * within 24 hours, by the calendar day it closes on, and open otherwise. A
 * paper not yet open says when it opens.
 */
export type ComingUpPill = "inProgress" | "dueToday" | "dueTomorrow" | "open" | "opens";

/**
 * ComingUpRow is one paper still to do. `moment` is the date its tile shows:
 * the close of an open paper, the opening of one not yet open, the deadline
 * of one being taken.
 */
export interface ComingUpRow {
  readonly card: StudentAssignmentCard;
  readonly pill: ComingUpPill;
  readonly moment: string;
}

/** ResultOutcome is what a Recent results row shows in place of a score. */
export type ResultOutcome = "score" | "grading" | "submitted";

/** ResultRow is one paper whose latest attempt is over. */
export interface ResultRow {
  readonly card: StudentAssignmentCard;
  readonly outcome: ResultOutcome;
}

/**
 * HomeSub is the sentence under the greeting: about the live attempt, else
 * the papers closing today, open or still to open, else the next paper, else
 * that nothing is due. `none` is a Home with nothing to draw, which gets the
 * empty state instead.
 */
export type HomeSub =
  | { readonly kind: "live"; readonly closes: string }
  | { readonly kind: "dueToday"; readonly count: number }
  | { readonly kind: "next"; readonly title: string; readonly moment: string }
  | { readonly kind: "nothing" }
  | { readonly kind: "none" };

/** HomeView is everything Home draws, derived from the three lists. */
export interface HomeView {
  readonly resume: StudentAssignmentCard | null;
  readonly rows: readonly ComingUpRow[];
  readonly results: readonly ResultRow[];
  readonly sub: HomeSub;
}

type Lists = Readonly<{
  dueNow: readonly StudentAssignmentCard[];
  upcoming: readonly StudentAssignmentCard[];
  completed: readonly StudentAssignmentCard[];
}>;

function liveCloses(card: StudentAssignmentCard): string {
  return card.liveDeadlineAt ?? card.closesAt;
}

function byMoment<T>(moment: (item: T) => string, id: (item: T) => string) {
  return (a: T, b: T) =>
    Date.parse(moment(a)) - Date.parse(moment(b)) || id(a).localeCompare(id(b));
}

function row(card: StudentAssignmentCard, now: Date): ComingUpRow | null {
  if (card.hasLiveAttempt === true)
    return { card, pill: "inProgress", moment: liveCloses(card) };
  const left = Date.parse(card.closesAt) - now.getTime();
  if (left <= 0) return null;
  if (card.status !== "open" && Date.parse(card.opensAt) > now.getTime())
    return { card, pill: "opens", moment: card.opensAt };
  if (left > DAY_MS) return { card, pill: "open", moment: card.closesAt };
  const pill = appDaysUntil(card.closesAt, now) === 0 ? "dueToday" : "dueTomorrow";
  return { card, pill, moment: card.closesAt };
}

function submitted(card: StudentAssignmentCard): number {
  return card.lastSubmittedAt == null ? Infinity : Date.parse(card.lastSubmittedAt);
}

function outcome(card: StudentAssignmentCard): ResultOutcome {
  if (card.score == null) return "submitted";
  return card.score.pendingManual > 0 ? "grading" : "score";
}

function closesToday(row: ComingUpRow, now: Date): boolean {
  if (row.pill === "dueToday") return true;
  return row.pill === "opens" && appDaysUntil(row.card.closesAt, now) === 0;
}

function sub(
  resume: StudentAssignmentCard | null,
  rows: readonly ComingUpRow[],
  empty: boolean,
  now: Date,
): HomeSub {
  if (resume !== null) return { kind: "live", closes: liveCloses(resume) };
  const today = rows.filter((r) => closesToday(r, now)).length;
  if (today > 0) return { kind: "dueToday", count: today };
  const next = rows[0];
  if (next !== undefined)
    return { kind: "next", title: next.card.testTitle, moment: next.moment };
  return empty ? { kind: "none" } : { kind: "nothing" };
}

/**
 * homeView turns the student's three lists into what Home draws at `now`.
 * The resume card is the live attempt whose deadline comes first. Coming up
 * holds every other paper still to do, in the order of the dates on their
 * tiles: a paper with an attempt left stays there after a first attempt, and a
 * paper whose window has closed on this clock is left out. The results are
 * every paper with a finished attempt, the one submitted last first, wherever
 * the server lists them, so a result shows while a retake remains; the page
 * decides how many to draw. An attempt that ran out of time and that the
 * server has not closed yet comes first: opening its result is what closes it.
 */
export function homeView(lists: Lists, now: Date): HomeView {
  const live = lists.dueNow
    .filter((card) => card.hasLiveAttempt === true)
    .sort(byMoment(liveCloses, (card) => card.id));
  const resume = live[0] ?? null;
  const rows = [...lists.dueNow, ...lists.upcoming]
    .filter((card) => card !== resume)
    .flatMap((card) => row(card, now) ?? [])
    .sort(
      byMoment(
        (r) => r.moment,
        (r) => r.card.id,
      ),
    );
  const results = [...lists.completed, ...lists.dueNow, ...lists.upcoming]
    .filter((card) => card.hasLiveAttempt !== true && card.lastAttemptId != null)
    .sort((a, b) => submitted(b) - submitted(a) || a.id.localeCompare(b.id))
    .map((card) => ({ card, outcome: outcome(card) }));
  const empty = resume === null && rows.length === 0 && results.length === 0;
  return { resume, rows, results, sub: sub(resume, rows, empty, now) };
}

/**
 * justSubmitted says whether a result is less than a minute old. A device
 * whose clock runs behind the server's sees a submission in its future, and
 * that counts too.
 */
export function justSubmitted(card: StudentAssignmentCard, now: Date): boolean {
  if (card.lastSubmittedAt == null) return false;
  return now.getTime() - Date.parse(card.lastSubmittedAt) < 60_000;
}
