import type { StudentAssignmentCard } from "@/features/assignments/api";
import { answered } from "@/features/take-test/answered";
import type { AttemptResult, ResultQuestion } from "./api";

type Review = AttemptResult["review"];
type Attempt = AttemptResult["attempt"];
type Section = AttemptResult["sections"][number];
type Option = NonNullable<ResultQuestion["options"]>[number];

const MANUAL: ReadonlySet<ResultQuestion["type"]> = new Set(["short_answer"]);
const FRESH_MS = 60_000;

/**
 * Verdict is what the result says about one answer. `waiting` is an answer
 * the teacher has yet to grade; `unknown` is an answer whose mark the review
 * policy does not show. `partial` earned some of the points and not all.
 */
export type Verdict = "correct" | "partial" | "wrong" | "waiting" | "unknown";

/**
 * verdict reads one answer's mark. It needs the policy to show scores and the
 * question to carry `earned`: a mark the server sent against the policy is
 * not drawn, and nothing is worked out from the answer key.
 */
export function verdict(question: ResultQuestion, review: Review): Verdict {
  if (question.pendingManual === true) return "waiting";
  if (!review.showScore || question.earned == null) return "unknown";
  if (question.earned >= question.points) return "correct";
  return question.earned > 0 ? "partial" : "wrong";
}

/** RingTone is the colour of the score ring's arc and of a part tile's bar. */
export type RingTone = "success" | "accent" | "warning";

function share(earned: number, total: number): number {
  if (total <= 0) return 0;
  return Math.round(Math.min(1, Math.max(0, earned / total)) * 10_000) / 10_000;
}

/**
 * ringTone colours a final score as the deck does: success from 80% of the
 * points, the accent from 60%, and warning below that.
 */
export function ringTone(earned: number, total: number): RingTone {
  const part = share(earned, total);
  if (part >= 0.8) return "success";
  return part >= 0.6 ? "accent" : "warning";
}

/**
 * Ring is what the summary's ring shows: the final score, the points marked
 * so far while answers wait for the teacher, "grading" when nothing has been
 * marked yet, or nothing when the score is withheld. `share` is the arc, from
 * 0 to 1.
 */
export type Ring =
  | {
      readonly kind: "score";
      readonly earned: number;
      readonly total: number;
      readonly share: number;
      readonly tone: RingTone;
    }
  | {
      readonly kind: "soFar";
      readonly earned: number;
      readonly total: number;
      readonly share: number;
    }
  | { readonly kind: "grading" }
  | { readonly kind: "withheld" };

/**
 * Summary is the state the sentence under the title is written from. `graded`
 * is a paper with every answer marked; `partly` has answers marked
 * automatically and answers waiting; `pending` has nothing marked
 * automatically and answers waiting; `withheld` is a paper whose score the
 * policy hides.
 */
export type Summary =
  | {
      readonly kind: "graded";
      readonly correct: number;
      readonly total: number;
      readonly key: boolean;
    }
  | { readonly kind: "partly"; readonly marked: number; readonly waiting: number }
  | { readonly kind: "pending" }
  | { readonly kind: "withheld"; readonly answered: number; readonly total: number };

/**
 * Tile is one box under the summary. While answers wait: the points marked
 * automatically, how many answers wait, and the time used. Once graded: one
 * box per part of the paper. The time is in whole minutes, a started minute
 * counting as one, and its share is those minutes of the attempt's allowance.
 */
export type Tile =
  | {
      readonly kind: "auto";
      readonly earned: number;
      readonly total: number;
      readonly share: number;
    }
  | { readonly kind: "waiting"; readonly count: number }
  | { readonly kind: "time"; readonly minutes: number; readonly share: number }
  | {
      readonly kind: "part";
      readonly id: string;
      readonly title: string;
      readonly earned: number;
      readonly total: number;
      readonly share: number;
      readonly tone: "success" | "warning";
    };

/** Filter is one choice of the "Your answers" filter. */
export type Filter = "all" | "wrong" | "waiting";

/**
 * Lock names what the review policy hides, for the line above the answers:
 * the score, the correct answers, the explanations, or a combination. It is
 * null when the policy hides nothing.
 */
export type Lock =
  | "score"
  | "answers"
  | "explanations"
  | "scoreAnswers"
  | "scoreExplanations"
  | "answersExplanations"
  | "all"
  | null;

/** ResultView is everything the result page draws above the answers. */
export interface ResultView {
  readonly ring: Ring;
  readonly summary: Summary;
  readonly tiles: readonly Tile[];
  readonly filters: readonly Filter[];
  readonly lock: Lock;
}

function sum(values: readonly number[]): number {
  return values.reduce((total, value) => total + value, 0);
}

function timeUsed(attempt: Attempt): Tile | null {
  if (attempt.submittedAt == null) return null;
  const started = Date.parse(attempt.startedAt);
  const used = Date.parse(attempt.submittedAt) - started;
  if (Number.isNaN(used) || used < 0) return null;
  const minutes = Math.max(1, Math.ceil(used / 60_000));
  return {
    kind: "time",
    minutes,
    share: share(minutes * 60_000, Date.parse(attempt.deadlineAt) - started),
  };
}

function partTiles(
  sections: readonly Section[],
  questions: readonly ResultQuestion[],
): Tile[] {
  const parts = sections.flatMap((section): Tile[] => {
    const own = questions.filter((question) => question.sectionId === section.id);
    if (own.length === 0) return [];
    const earned = sum(own.map((question) => question.earned ?? 0));
    const total = sum(own.map((question) => question.points));
    return [
      {
        kind: "part",
        id: section.id,
        title: section.title,
        earned,
        total,
        share: share(earned, total),
        tone: share(earned, total) >= 0.7 ? "success" : "warning",
      },
    ];
  });
  return parts.length >= 2 ? parts : [];
}

function lock(review: Review): Lock {
  const score = !review.showScore;
  const answers = !review.showCorrectAnswers;
  const explanations = !review.showExplanations;
  if (score && answers && explanations) return "all";
  if (score && answers) return "scoreAnswers";
  if (score && explanations) return "scoreExplanations";
  if (answers && explanations) return "answersExplanations";
  if (score) return "score";
  if (answers) return "answers";
  return explanations ? "explanations" : null;
}

/**
 * resultView turns a result into what the page draws above the answers. A
 * score the policy hides gives no number and no tiles. While answers wait for
 * the teacher, the ring holds the points decided so far and the tiles say
 * what was marked automatically, how many answers wait and how long the
 * attempt took; with nothing marked automatically the ring only says
 * "grading". Once every answer is marked, the ring holds the final score and a
 * paper of two or more parts gets a tile per part, summed from its questions.
 * The filter offers Wrong only when scores are shown and Waiting only when
 * something waits.
 */
export function resultView(data: AttemptResult): ResultView {
  const { attempt, review, questions, sections } = data;
  const score = review.showScore ? (attempt.score ?? null) : null;
  const waiting = questions.filter((question) => question.pendingManual === true);
  const filters: Filter[] = [
    "all",
    ...(review.showScore ? (["wrong"] as const) : []),
    ...(waiting.length > 0 ? (["waiting"] as const) : []),
  ];
  const base = { filters, lock: lock(review) };

  if (score === null) {
    return {
      ...base,
      ring: { kind: "withheld" },
      summary: {
        kind: "withheld",
        answered: questions.filter((question) =>
          answered(question, question.answer ?? undefined),
        ).length,
        total: questions.length,
      },
      tiles: [],
    };
  }

  if (waiting.length === 0) {
    return {
      ...base,
      ring: {
        kind: "score",
        earned: score.earned,
        total: score.total,
        share: share(score.earned, score.total),
        tone: ringTone(score.earned, score.total),
      },
      summary: {
        kind: "graded",
        correct: questions.filter((question) => verdict(question, review) === "correct")
          .length,
        total: questions.length,
        key: review.showCorrectAnswers,
      },
      tiles: partTiles(sections, questions),
    };
  }

  const auto = questions.filter(
    (question) => question.pendingManual !== true && !MANUAL.has(question.type),
  );
  if (auto.length === 0) {
    return {
      ...base,
      ring: { kind: "grading" },
      summary: { kind: "pending" },
      tiles: [],
    };
  }

  const decided = sum(
    questions
      .filter((question) => question.pendingManual !== true)
      .map((question) => question.points),
  );
  const autoEarned = sum(auto.map((question) => question.earned ?? 0));
  const autoTotal = sum(auto.map((question) => question.points));
  const time = timeUsed(attempt);
  return {
    ...base,
    ring: {
      kind: "soFar",
      earned: score.earned,
      total: decided,
      share: share(score.earned, decided),
    },
    summary: { kind: "partly", marked: auto.length, waiting: waiting.length },
    tiles: [
      {
        kind: "auto",
        earned: autoEarned,
        total: autoTotal,
        share: share(autoEarned, autoTotal),
      },
      { kind: "waiting", count: waiting.length },
      ...(time === null ? [] : [time]),
    ],
  };
}

/**
 * shownUnder is the questions a filter keeps, in the paper's order. Wrong
 * holds every marked answer that lost points, a partly right one included;
 * Waiting holds the answers the teacher has yet to grade.
 */
export function shownUnder(
  filter: Filter,
  questions: readonly ResultQuestion[],
  review: Review,
): ResultQuestion[] {
  return questions.filter((question) => {
    const mark = verdict(question, review);
    if (filter === "wrong") return mark === "wrong" || mark === "partial";
    if (filter === "waiting") return mark === "waiting";
    return true;
  });
}

/**
 * Given is what a review item writes after "You answered". A fill-in lists
 * its blanks in order, with null for a blank left empty. `none` is a question
 * with nothing to show.
 */
export type Given =
  | { readonly kind: "none" }
  | { readonly kind: "options"; readonly options: readonly Option[] }
  | { readonly kind: "text"; readonly text: string }
  | { readonly kind: "boolean"; readonly value: boolean }
  | { readonly kind: "blanks"; readonly values: readonly (string | null)[] };

function blanksOf(question: ResultQuestion) {
  return [...(question.blanks ?? [])].sort((a, b) => a.ordinal - b.ordinal);
}

/**
 * given reads what the student answered. Whether a question counts as
 * answered is the engine's rule (`take-test/answered`), so a saved answer
 * that was cleared reads "no answer". A fill-in with some blanks filled is
 * not answered by that rule, and still lists what was typed: those blanks may
 * have earned points.
 */
export function given(question: ResultQuestion): Given {
  const answer = question.answer;
  if (answer == null) return { kind: "none" };
  if (answer.type === "fill_blank") {
    const values = blanksOf(question).map((blank) => {
      const typed = (answer.values[blank.id] ?? "").trim();
      return typed === "" ? null : typed;
    });
    return values.some((value) => value !== null)
      ? { kind: "blanks", values }
      : { kind: "none" };
  }
  if (!answered(question, answer)) return { kind: "none" };
  if (answer.type === "text") return { kind: "text", text: answer.value.trim() };
  if (answer.type === "true_false") return { kind: "boolean", value: answer.value };
  const chosen = new Set(answer.optionIds);
  const options = (question.options ?? []).filter((option) => chosen.has(option.id));
  return options.length > 0 ? { kind: "options", options } : { kind: "none" };
}

/**
 * Key is the correct answer a review item writes: the right options of a
 * choice, or one accepted answer per blank of a fill-in, in the paper's order.
 */
export type Key =
  | { readonly kind: "options"; readonly options: readonly Option[] }
  | { readonly kind: "blanks"; readonly values: readonly string[] };

/**
 * correctKey is the correct answer to show for a question, or null. It needs
 * the policy to show correct answers and the question to carry them, and it
 * is left out for an answer already marked correct, which it would repeat.
 */
export function correctKey(question: ResultQuestion, review: Review): Key | null {
  if (!review.showCorrectAnswers || verdict(question, review) === "correct")
    return null;
  if (question.type === "fill_blank") {
    const accepted = new Map(
      (question.correctAnswers ?? []).map((entry) => [entry.blankId, entry.answer]),
    );
    const values = blanksOf(question).flatMap((blank) => accepted.get(blank.id) ?? []);
    return values.length > 0 ? { kind: "blanks", values } : null;
  }
  const correct = new Set(question.correctOptionIds ?? []);
  const options = (question.options ?? []).filter((option) => correct.has(option.id));
  return options.length > 0 ? { kind: "options", options } : null;
}

/**
 * justNow says whether an attempt was submitted less than a minute before
 * `now`. A device whose clock runs behind the server's sees the submission in
 * its future, and that counts too, as on Home.
 */
export function justNow(submittedAt: string | null | undefined, now: Date): boolean {
  if (submittedAt == null) return false;
  return now.getTime() - Date.parse(submittedAt) < FRESH_MS;
}

/**
 * classNameOf finds the class a result's paper was assigned through, in the
 * assignment lists the student pages share. It is null while the lists are
 * not loaded, and for a paper the server attributes to no single class.
 */
export function classNameOf(
  lists:
    | Readonly<{
        dueNow: readonly StudentAssignmentCard[];
        upcoming: readonly StudentAssignmentCard[];
        completed: readonly StudentAssignmentCard[];
      }>
    | undefined,
  assignmentId: string,
): string | null {
  if (lists === undefined) return null;
  const card = [...lists.completed, ...lists.dueNow, ...lists.upcoming].find(
    (entry) => entry.id === assignmentId,
  );
  const name = card?.className ?? null;
  return name === null || name.trim() === "" ? null : name;
}
