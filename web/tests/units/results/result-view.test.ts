import { describe, expect, it } from "vitest";
import type { ResultQuestion } from "@/features/results/api";
import {
  classNameOf,
  correctKey,
  given,
  justNow,
  resultView,
  ringTone,
  shownUnder,
  verdict,
} from "@/features/results/resultView";
import {
  ASSIGNMENT_ID,
  CLOSED,
  OPEN,
  PART_ONE,
  PART_TWO,
  SCORE_ONLY,
  assignmentCard,
  blank,
  choice,
  essay,
  grading,
  grammarCheck,
  justSubmitted,
  paper,
  scored,
  uuid,
} from "./fixtures";

const AB = ["was wrote", "was written"];

describe("verdict", () => {
  it("is correct at full points, partial below them and wrong at none", () => {
    expect(verdict(choice(1, AB, [0], { earned: 1 }), OPEN)).toBe("correct");
    expect(verdict(choice(1, AB, [0], { points: 2, earned: 2.5 }), OPEN)).toBe(
      "correct",
    );
    expect(verdict(choice(1, AB, [0], { points: 2, earned: 0.5 }), OPEN)).toBe(
      "partial",
    );
    expect(verdict(choice(1, AB, [0], { earned: 0 }), OPEN)).toBe("wrong");
  });

  it("is waiting while the teacher has not graded, whatever the policy shows", () => {
    const waiting = essay(1, "Viết một câu.", "Một câu.");
    expect(verdict(waiting, OPEN)).toBe("waiting");
    expect(verdict(waiting, CLOSED)).toBe("waiting");
  });

  it("is unknown without a mark, and when the policy hides a mark that was sent", () => {
    expect(verdict(choice(1, AB, [0]), OPEN)).toBe("unknown");
    expect(verdict(choice(1, AB, [0], { earned: null }), OPEN)).toBe("unknown");
    expect(verdict(choice(1, AB, [0], { earned: 0 }), CLOSED)).toBe("unknown");
    expect(verdict(choice(1, AB, [0], { earned: 1 }), CLOSED)).toBe("unknown");
  });
});

describe("ringTone", () => {
  it.each([
    [8, 10, "success"],
    [10, 10, "success"],
    [7.999, 10, "accent"],
    [6, 10, "accent"],
    [5.999, 10, "warning"],
    [0, 10, "warning"],
    [0, 0, "warning"],
    [24, 30, "success"],
    [2.4, 3, "success"],
    [1.8, 3, "accent"],
  ] as const)("%s of %s is %s", (earned, total, tone) => {
    expect(ringTone(earned, total)).toBe(tone);
  });
});

describe("resultView: a graded paper", () => {
  it("draws the final score, counts the fully right answers and names the key", () => {
    const view = resultView(grammarCheck());
    expect(view.ring).toEqual({
      kind: "score",
      earned: 8,
      total: 10,
      share: 0.8,
      tone: "success",
    });
    expect(view.summary).toEqual({ kind: "graded", correct: 8, total: 10, key: true });
    expect(view.filters).toEqual(["all", "wrong"]);
    expect(view.lock).toBeNull();
  });

  it("names the key only when an answer below draws one", () => {
    const body = scored([
      choice(1, AB, [1], { earned: 1, correctOptionIds: [uuid("b", 11)] }),
      essay(2, "Viết một câu.", "Một câu.", { pendingManual: false, earned: 0 }),
    ]);
    expect(resultView(body).summary).toEqual({
      kind: "graded",
      correct: 1,
      total: 2,
      key: false,
    });
  });

  it("sums a tile per part from the questions that name it", () => {
    expect(resultView(grammarCheck()).tiles).toEqual([
      {
        kind: "part",
        id: PART_ONE,
        title: "Present perfect",
        earned: 4,
        total: 5,
        share: 0.8,
        tone: "success",
      },
      {
        kind: "part",
        id: PART_TWO,
        title: "Past simple",
        earned: 4,
        total: 5,
        share: 0.8,
        tone: "success",
      },
    ]);
  });

  it("turns a part's bar amber below 70% of its points", () => {
    const body = scored(
      [
        choice(1, AB, [0], { points: 10, earned: 7 }),
        choice(2, AB, [0], { points: 10, earned: 6.99, sectionId: PART_TWO }),
      ],
      {
        sections: [
          { id: PART_ONE, title: "Nghe", instructions: null },
          { id: PART_TWO, title: "Đọc", instructions: null },
        ],
      },
    );
    expect(
      resultView(body).tiles.map((tile) => tile.kind === "part" && tile.tone),
    ).toEqual(["success", "warning"]);
  });

  it("has no tiles for a paper of one part, or of two where one holds no question", () => {
    const one = scored([choice(1, AB, [0], { earned: 1 })]);
    expect(resultView(one).tiles).toEqual([]);
    const hollow = scored([choice(1, AB, [0], { earned: 1 })], {
      sections: [
        { id: PART_ONE, title: "Nghe", instructions: null },
        { id: PART_TWO, title: "Đọc", instructions: null },
      ],
    });
    expect(resultView(hollow).tiles).toEqual([]);
  });

  it("does not call a partly right answer correct", () => {
    const body = scored([
      choice(1, AB, [0], { earned: 1 }),
      choice(2, AB, [0], { points: 2, earned: 1 }),
      choice(3, AB, [0], { earned: 0 }),
    ]);
    expect(resultView({ ...body, review: SCORE_ONLY }).summary).toEqual({
      kind: "graded",
      correct: 1,
      total: 3,
      key: false,
    });
  });
});

describe("resultView: answers waiting for the teacher", () => {
  it("shows what is decided so far and the three tiles", () => {
    const view = resultView(justSubmitted("2025-09-24T11:29:40Z"));
    expect(view.ring).toEqual({ kind: "soFar", earned: 3, total: 6, share: 0.5 });
    expect(view.summary).toEqual({ kind: "partly", marked: 6, waiting: 2 });
    expect(view.tiles).toEqual([
      { kind: "auto", earned: 3, total: 6, share: 0.5 },
      { kind: "waiting", count: 2 },
      { kind: "time", minutes: 7, share: 0.1556 },
    ]);
    expect(view.filters).toEqual(["all", "wrong", "waiting"]);
    expect(view.lock).toBe("answersExplanations");
  });

  it("keeps a hand-graded answer out of what was marked automatically", () => {
    const body = scored([
      choice(1, AB, [1], { earned: 1 }),
      choice(2, AB, [0], { earned: 0 }),
      essay(3, "Viết.", "Đã viết.", { points: 5, pendingManual: false, earned: 4 }),
      essay(4, "Viết nữa.", "Đang chờ.", { points: 5 }),
    ]);
    const view = resultView(body);
    expect(view.ring).toEqual({
      kind: "soFar",
      earned: 5,
      total: 7,
      share: 0.7143,
    });
    expect(view.tiles[0]).toEqual({ kind: "auto", earned: 1, total: 2, share: 0.5 });
    expect(view.summary).toEqual({ kind: "partly", marked: 2, waiting: 1 });
  });

  it("only says grading when nothing was marked automatically", () => {
    const view = resultView(grading());
    expect(view.ring).toEqual({ kind: "grading" });
    expect(view.summary).toEqual({ kind: "pending" });
    expect(view.tiles).toEqual([]);
    expect(view.filters).toEqual(["all", "wrong", "waiting"]);
  });

  it("counts a started minute, never zero, and caps the bar at the allowance", () => {
    const timed = (startedAt: string, submittedAt: string | null, deadlineAt: string) =>
      resultView(
        scored([choice(1, AB, [0], { earned: 1 }), essay(2, "Viết.", "Xong.")], {
          attempt: { ...paper().attempt, startedAt, submittedAt, deadlineAt },
        }),
      ).tiles.find((tile) => tile.kind === "time");
    expect(
      timed("2026-08-26T12:00:00Z", "2026-08-26T12:07:00Z", "2026-08-26T12:45:00Z"),
    ).toEqual({ kind: "time", minutes: 7, share: 0.1556 });
    expect(
      timed("2026-08-26T12:00:00Z", "2026-08-26T12:07:01Z", "2026-08-26T12:45:00Z"),
    ).toMatchObject({ minutes: 8, share: 0.1778 });
    expect(
      timed("2026-08-26T12:00:00Z", "2026-08-26T12:00:00Z", "2026-08-26T12:45:00Z"),
    ).toMatchObject({ minutes: 1, share: 0.0222 });
    expect(
      timed("2026-08-26T12:00:00Z", "2026-08-26T12:50:00Z", "2026-08-26T12:45:00Z"),
    ).toMatchObject({ minutes: 50, share: 1 });
    expect(
      timed("2026-08-26T12:00:00Z", "2026-08-26T12:10:00Z", "2026-08-26T12:00:00Z"),
    ).toMatchObject({ minutes: 10, share: 0 });
    expect(timed("2026-08-26T12:00:00Z", null, "2026-08-26T12:45:00Z")).toBeUndefined();
    expect(
      timed("2026-08-26T12:00:00Z", "2026-08-26T11:59:00Z", "2026-08-26T12:45:00Z"),
    ).toBeUndefined();
  });
});

describe("resultView: a score the policy hides", () => {
  it("draws no number and no tiles, and counts answers by the engine's rule", () => {
    const twoBlanks = blank(5, "{{1}} và {{2}}", "một", {
      blanks: [
        { id: uuid("e", 5), ordinal: 1, caseSensitive: false },
        { id: uuid("e", 6), ordinal: 2, caseSensitive: false },
      ],
    });
    const body = paper({
      review: CLOSED,
      questions: [
        choice(1, AB, [0]),
        choice(2, AB, []),
        essay(3, "Viết.", "   ", { pendingManual: true }),
        blank(4, "{{1}}", "has lived"),
        twoBlanks,
        choice(6, AB, null),
      ],
    });
    const view = resultView(body);
    expect(view.ring).toEqual({ kind: "withheld" });
    expect(view.summary).toEqual({ kind: "withheld", answered: 2, total: 6 });
    expect(view.tiles).toEqual([]);
    expect(view.filters).toEqual(["all", "waiting"]);
    expect(view.lock).toBe("all");
  });

  it("ignores a score the server sent against the policy", () => {
    const body = scored([choice(1, AB, [0], { earned: 1 })], { review: CLOSED });
    expect(resultView(body).ring).toEqual({ kind: "withheld" });
    expect(resultView(body).filters).toEqual(["all"]);
  });

  it("has no part tiles on a paper in parts", () => {
    const body = paper({
      review: CLOSED,
      sections: [
        { id: PART_ONE, title: "Nghe", instructions: null },
        { id: PART_TWO, title: "Đọc", instructions: null },
      ],
      questions: [choice(1, AB, [0]), choice(2, AB, [0], { sectionId: PART_TWO })],
    });
    expect(resultView(body).tiles).toEqual([]);
  });
});

describe("resultView: the lock line", () => {
  it.each([
    [true, true, true, null],
    [false, true, true, "score"],
    [true, false, true, "answers"],
    [true, true, false, "explanations"],
    [false, false, true, "scoreAnswers"],
    [false, true, false, "scoreExplanations"],
    [true, false, false, "answersExplanations"],
    [false, false, false, "all"],
  ] as const)(
    "showScore=%s showCorrectAnswers=%s showExplanations=%s names %s",
    (showScore, showCorrectAnswers, showExplanations, lock) => {
      const body = paper({
        review: {
          showScore,
          showCorrectAnswers,
          showExplanations,
          release: "on_submit",
          showClassAverage: false,
        },
        questions: [choice(1, AB, [0])],
      });
      expect(resultView(body).lock).toBe(lock);
    },
  );
});

describe("shownUnder", () => {
  const questions = [
    choice(1, AB, [1], { earned: 1 }),
    choice(2, AB, [0], { earned: 0 }),
    choice(3, AB, [0], { points: 2, earned: 1 }),
    essay(4, "Viết.", "Xong."),
    choice(5, AB, [0]),
  ];
  const ids = (list: readonly ResultQuestion[]) => list.map((question) => question.id);

  it("keeps the paper's order under All", () => {
    expect(ids(shownUnder("all", questions, OPEN))).toEqual(ids(questions));
  });

  it("holds every marked answer that lost points under Wrong", () => {
    expect(ids(shownUnder("wrong", questions, OPEN))).toEqual([
      questions[1]!.id,
      questions[2]!.id,
    ]);
  });

  it("holds the answers the teacher has yet to grade under Waiting", () => {
    expect(ids(shownUnder("waiting", questions, OPEN))).toEqual([questions[3]!.id]);
  });

  it("finds nothing wrong when scores are hidden", () => {
    expect(shownUnder("wrong", questions, CLOSED)).toEqual([]);
  });
});

describe("given", () => {
  it("lists the options picked, in the paper's order", () => {
    const question = choice(1, ["một", "hai", "ba"], [2, 0], {
      type: "multiple_choice",
    });
    expect(given(question)).toEqual({
      kind: "options",
      options: [question.options![0], question.options![2]],
    });
  });

  it("reads a cleared answer as no answer", () => {
    expect(given(choice(1, AB, null))).toEqual({ kind: "none" });
    expect(given(choice(1, AB, []))).toEqual({ kind: "none" });
    expect(given(essay(1, "Viết.", " \n "))).toEqual({ kind: "none" });
    expect(given(blank(1, "{{1}}", "  "))).toEqual({ kind: "none" });
    expect(given(blank(1, "{{1}}", null))).toEqual({ kind: "none" });
  });

  it("gives a written answer without the space around it", () => {
    expect(given(essay(1, "Viết.", "  Hai dòng\nchữ.  "))).toEqual({
      kind: "text",
      text: "Hai dòng\nchữ.",
    });
  });

  it("gives a true/false value as it was saved", () => {
    const question = choice(1, ["Đúng", "Sai"], null, {
      type: "true_false",
      answer: { type: "true_false", value: false },
    });
    expect(given(question)).toEqual({ kind: "boolean", value: false });
  });

  it("lists a fill-in's blanks by ordinal, with null for one left empty", () => {
    const first = uuid("e", 1);
    const second = uuid("e", 2);
    const question = blank(1, "{{1}} rồi {{2}}", null, {
      blanks: [
        { id: second, ordinal: 2, caseSensitive: false },
        { id: first, ordinal: 1, caseSensitive: false },
      ],
      answer: { type: "fill_blank", values: { [second]: " went ", [first]: "" } },
    });
    expect(given(question)).toEqual({ kind: "blanks", values: [null, "went"] });
  });

  it("shows nothing for options the paper does not have", () => {
    const question = choice(1, AB, null, {
      answer: { type: "choice", optionIds: [uuid("b", 999)] },
    });
    expect(given(question)).toEqual({ kind: "none" });
  });
});

describe("correctKey", () => {
  const wrong = choice(1, AB, [0], {
    earned: 0,
    correctOptionIds: [uuid("b", 11)],
  });

  it("gives the right options of an answer that was not fully right", () => {
    expect(correctKey(wrong, OPEN)).toEqual({
      kind: "options",
      options: [wrong.options![1]],
    });
  });

  it("is left out for an answer marked correct", () => {
    const right = choice(1, AB, [1], { earned: 1, correctOptionIds: [uuid("b", 11)] });
    expect(correctKey(right, OPEN)).toBeNull();
  });

  it("is given when the score is hidden, without saying whether the answer was right", () => {
    const review = {
      showScore: false,
      showCorrectAnswers: true,
      showExplanations: false,
      release: "on_submit" as const,
      showClassAverage: false,
    };
    const right = choice(1, AB, [1], { correctOptionIds: [uuid("b", 11)] });
    expect(correctKey(right, review)).toEqual({
      kind: "options",
      options: [right.options![1]],
    });
  });

  it("is never read when the policy hides correct answers", () => {
    expect(correctKey(wrong, SCORE_ONLY)).toBeNull();
    const fill = blank(2, "{{1}}", "did", {
      earned: 0,
      correctAnswers: [{ blankId: uuid("e", 2), answer: "have" }],
    });
    expect(correctKey(fill, SCORE_ONLY)).toBeNull();
  });

  it("gives one accepted answer per blank, by ordinal", () => {
    const first = uuid("e", 1);
    const second = uuid("e", 2);
    const fill = blank(1, "{{1}} rồi {{2}}", null, {
      earned: 0,
      blanks: [
        { id: second, ordinal: 2, caseSensitive: false },
        { id: first, ordinal: 1, caseSensitive: false },
      ],
      correctAnswers: [
        { blankId: second, answer: "saw" },
        { blankId: first, answer: "have" },
      ],
    });
    expect(correctKey(fill, OPEN)).toEqual({ kind: "blanks", values: ["have", "saw"] });
  });

  it("is null when the question carries no key", () => {
    expect(correctKey(choice(1, AB, [0], { earned: 0 }), OPEN)).toBeNull();
    expect(correctKey(blank(1, "{{1}}", "x", { earned: 0 }), OPEN)).toBeNull();
    expect(correctKey(essay(1, "Viết.", "Xong."), OPEN)).toBeNull();
  });
});

describe("justNow", () => {
  const now = new Date("2026-08-26T13:15:00.000Z");

  it("lasts one minute from the submission", () => {
    expect(justNow("2026-08-26T13:14:00.001Z", now)).toBe(true);
    expect(justNow("2026-08-26T13:14:00.000Z", now)).toBe(false);
  });

  it("holds for a submission the device sees in its future", () => {
    expect(justNow("2026-08-26T13:15:30.000Z", now)).toBe(true);
  });

  it("is false for an attempt with no submission time", () => {
    expect(justNow(null, now)).toBe(false);
    expect(justNow(undefined, now)).toBe(false);
  });
});

describe("classNameOf", () => {
  const none = { dueNow: [], upcoming: [], completed: [] };

  it("finds the paper's class in any of the three lists", () => {
    const card = assignmentCard();
    expect(classNameOf({ ...none, completed: [card] }, ASSIGNMENT_ID)).toBe(
      "IELTS Foundation A",
    );
    expect(classNameOf({ ...none, dueNow: [card] }, ASSIGNMENT_ID)).toBe(
      "IELTS Foundation A",
    );
    expect(classNameOf({ ...none, upcoming: [card] }, ASSIGNMENT_ID)).toBe(
      "IELTS Foundation A",
    );
  });

  it("is null without the lists, without the paper and without a single class", () => {
    expect(classNameOf(undefined, ASSIGNMENT_ID)).toBeNull();
    expect(classNameOf(none, ASSIGNMENT_ID)).toBeNull();
    expect(
      classNameOf(
        { ...none, completed: [assignmentCard({ id: uuid("d", 2) })] },
        ASSIGNMENT_ID,
      ),
    ).toBeNull();
    expect(
      classNameOf(
        { ...none, completed: [assignmentCard({ className: null, classId: null })] },
        ASSIGNMENT_ID,
      ),
    ).toBeNull();
    expect(
      classNameOf(
        { ...none, completed: [assignmentCard({ className: "  " })] },
        ASSIGNMENT_ID,
      ),
    ).toBeNull();
  });
});
