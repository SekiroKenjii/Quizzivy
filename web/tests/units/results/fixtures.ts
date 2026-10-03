import type { AttemptResult, ResultQuestion } from "@/features/results/api";
import type { components } from "@/lib/api/schema";

type Review = components["schemas"]["ReviewPolicy"];
type Card = components["schemas"]["StudentAssignmentCard"];

export const BASE = "http://localhost:8080";
export const ATTEMPT_ID = "018f0000-0000-7000-8000-0000000000a7";
export const ASSIGNMENT_ID = "018f0000-0000-7000-8000-0000000000d1";
export const PART_ONE = "018f0000-0000-7000-8000-0000000000c1";
export const PART_TWO = "018f0000-0000-7000-8000-0000000000c2";

export const OPEN: Review = {
  showScore: true,
  showCorrectAnswers: true,
  showExplanations: true,
};
export const SCORE_ONLY: Review = {
  showScore: true,
  showCorrectAnswers: false,
  showExplanations: false,
};
export const CLOSED: Review = {
  showScore: false,
  showCorrectAnswers: false,
  showExplanations: false,
};

export function uuid(kind: string, n: number): string {
  return `018f0000-0000-7000-8000-${kind}${String(n).padStart(12 - kind.length, "0")}`;
}

export function choice(
  n: number,
  texts: readonly string[],
  picked: readonly number[] | null,
  over: Partial<ResultQuestion> = {},
): ResultQuestion {
  const options = texts.map((text, index) => ({
    id: uuid("b", n * 10 + index),
    text,
  }));
  return {
    id: uuid("a", n),
    sectionId: PART_ONE,
    type: "single_choice",
    prompt: `Câu hỏi ${n}`,
    points: 1,
    options,
    answer:
      picked === null
        ? null
        : { type: "choice", optionIds: picked.map((index) => options[index]!.id) },
    pendingManual: false,
    ...over,
  };
}

export function blank(
  n: number,
  prompt: string,
  typed: string | null,
  over: Partial<ResultQuestion> = {},
): ResultQuestion {
  const blankId = uuid("e", n);
  return {
    id: uuid("a", n),
    sectionId: PART_ONE,
    type: "fill_blank",
    prompt,
    points: 1,
    blanks: [{ id: blankId, ordinal: 1, caseSensitive: false }],
    answer:
      typed === null ? null : { type: "fill_blank", values: { [blankId]: typed } },
    pendingManual: false,
    ...over,
  };
}

export function essay(
  n: number,
  prompt: string,
  written: string | null,
  over: Partial<ResultQuestion> = {},
): ResultQuestion {
  return {
    id: uuid("a", n),
    sectionId: PART_ONE,
    type: "short_answer",
    prompt,
    points: 1,
    answer: written === null ? null : { type: "text", value: written },
    pendingManual: written !== null,
    ...over,
  };
}

export function paper(over: Partial<AttemptResult> = {}): AttemptResult {
  return {
    attempt: {
      id: ATTEMPT_ID,
      assignmentId: ASSIGNMENT_ID,
      studentId: "018f0000-0000-7000-8000-0000000000e1",
      testVersionId: "018f0000-0000-7000-8000-0000000000f1",
      attemptNo: 1,
      status: "submitted",
      startedAt: "2026-08-26T12:40:00Z",
      deadlineAt: "2026-08-26T13:25:00Z",
      submittedAt: "2026-08-26T13:14:00Z",
      score: null,
    },
    review: OPEN,
    testTitle: "Unit 4 — Passive voice",
    maxAttempts: 1,
    sections: [{ id: PART_ONE, title: "Phần 1", instructions: null }],
    questions: [],
    ...over,
  };
}

export function scored(
  questions: readonly ResultQuestion[],
  over: Partial<AttemptResult> = {},
): AttemptResult {
  const base = paper(over);
  const marked = questions.filter((question) => question.pendingManual !== true);
  return {
    ...base,
    questions: [...questions],
    attempt: {
      ...base.attempt,
      score: {
        earned: marked.reduce((sum, question) => sum + (question.earned ?? 0), 0),
        total: questions.reduce((sum, question) => sum + question.points, 0),
        pendingManual: questions.length - marked.length,
      },
    },
  };
}

export function assignmentCard(over: Partial<Card> = {}): Card {
  return {
    id: ASSIGNMENT_ID,
    testTitle: "Unit 4 — Passive voice",
    className: "IELTS Foundation A",
    classId: "018f0000-0000-7000-8000-0000000000c9",
    status: "closed",
    opensAt: "2026-08-26T01:00:00Z",
    closesAt: "2026-08-26T14:00:00Z",
    durationMinutes: 45,
    questionCount: 2,
    totalPoints: 2,
    attemptsUsed: 1,
    maxAttempts: 1,
    hasLiveAttempt: false,
    lastAttemptId: ATTEMPT_ID,
    lastSubmittedAt: "2026-08-26T13:14:00Z",
    ...over,
  };
}

const GRAMMAR: readonly [string, string, string, string | null, string][] = [
  ["She {{1}} in Hanoi since 2019.", "has lived", "has lived", null, PART_ONE],
  [
    "They {{1}} the report yet.",
    "haven't finished",
    "haven't finished",
    null,
    PART_ONE,
  ],
  [
    "How long {{1}} you known each other?",
    "did",
    "have",
    "How long + present perfect asks about time up to now.",
    PART_ONE,
  ],
  ["He {{1}} his keys, so he cannot get in.", "has lost", "has lost", null, PART_ONE],
  ["My sister {{1}} here since March.", "has worked", "has worked", null, PART_ONE],
  [
    "I {{1}} him yesterday at the station.",
    "have seen",
    "saw",
    "Yesterday is a finished time, so use the past simple.",
    PART_TWO,
  ],
  ["We {{1}} to Da Nang last summer.", "went", "went", null, PART_TWO],
  ["She {{1}} the email two hours ago.", "sent", "sent", null, PART_TWO],
  ["When {{1}} you arrive last night?", "did", "did", null, PART_TWO],
  ["They {{1}} football last Sunday.", "played", "played", null, PART_TWO],
];

export function grammarCheck(): AttemptResult {
  const questions = GRAMMAR.map(
    ([prompt, typed, accepted, explanation, sectionId], i) =>
      blank(i + 1, prompt, typed, {
        sectionId,
        earned: typed === accepted ? 1 : 0,
        correctAnswers: [{ blankId: uuid("e", i + 1), answer: accepted }],
        explanation,
      }),
  );
  return scored(questions, {
    testTitle: "Unit 3 · Grammar check",
    sections: [
      { id: PART_ONE, title: "Present perfect", instructions: null },
      { id: PART_TWO, title: "Past simple", instructions: null },
    ],
    attempt: {
      ...paper().attempt,
      startedAt: "2025-09-19T03:00:00Z",
      deadlineAt: "2025-09-19T03:40:00Z",
      submittedAt: "2025-09-19T03:20:00Z",
    },
  });
}

export function justSubmitted(submittedAt: string): AttemptResult {
  const started = new Date(Date.parse(submittedAt) - (6 * 60 + 48) * 1000);
  const tfng = ["True", "False", "Not given"];
  const questions = [
    choice(
      1,
      [
        "To compare parks in European and Asian cities",
        "To argue that green space is worth planning for",
        "To describe the history of city parks",
        "To criticise councils that close streets",
      ],
      [1],
      { prompt: "What is the main purpose of the passage?", earned: 1 },
    ),
    choice(
      2,
      [
        "move house less often",
        "sleep better and see the doctor less",
        "spend more time outside at night",
        "pay less for their homes",
      ],
      [1],
      {
        prompt: "According to paragraph A, people who live near a park…",
        earned: 1,
      },
    ),
    choice(3, tfng, [1], {
      prompt: "Homes near parks always sell for 15 per cent more.",
      earned: 1,
    }),
    choice(
      4,
      [
        "Lower street temperatures",
        "Less traffic noise",
        "Higher property values",
        "Cleaner rivers",
        "More tourists",
      ],
      null,
      {
        type: "multiple_choice",
        prompt: "Which TWO benefits of trees and parks does paragraph B mention?",
        earned: 0,
      },
    ),
    essay(
      5,
      "What should cities prioritise when they plan new districts?",
      "public green space",
    ),
    choice(
      6,
      [
        "Parks must be larger than five hectares",
        "Every home must be a five-minute walk from a park",
        "New districts must have no cars",
        "Trees must line every street",
      ],
      null,
      { prompt: "What rule does Copenhagen follow?", earned: 0 },
    ),
    essay(
      7,
      "What do locals call the streets closed to traffic on Sundays?",
      "play streets",
    ),
    choice(8, tfng, null, {
      prompt: "Shop owners on play streets were against the idea at first.",
      earned: 0,
    }),
  ];
  return scored(questions, {
    review: SCORE_ONLY,
    testTitle: "Reading · Why cities need green space",
    sections: [{ id: PART_ONE, title: "Reading", instructions: null }],
    attempt: {
      ...paper().attempt,
      startedAt: started.toISOString(),
      deadlineAt: new Date(started.getTime() + 45 * 60_000).toISOString(),
      submittedAt,
    },
  });
}

export function grading(): AttemptResult {
  return scored(
    [
      essay(
        1,
        "Why does the writer say parks pay for themselves?",
        "Homes near a park sell for more, and the extra tax covers the upkeep.",
        { points: 5 },
      ),
      essay(
        2,
        "What does Copenhagen ask of every new neighbourhood?",
        "A park within a five-minute walk of each home.",
        { points: 5 },
      ),
    ],
    {
      review: SCORE_ONLY,
      testTitle: "Reading · TFNG Practice A",
      sections: [{ id: PART_ONE, title: "Reading", instructions: null }],
      attempt: {
        ...paper().attempt,
        startedAt: "2025-09-17T11:00:00Z",
        deadlineAt: "2025-09-17T11:45:00Z",
        submittedAt: "2025-09-17T11:31:00Z",
      },
    },
  );
}
