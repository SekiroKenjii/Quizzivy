import type { components } from "@/lib/api/schema";

export const BASE = "http://localhost:8080";
export const ASSIGNMENT_ID = "018f0000-0000-7000-8000-0000000000d1";
export const ATTEMPT_ID = "018f0000-0000-7000-8000-0000000000a7";
export const STUDENT_ID = "018f0000-0000-7000-8000-0000000000e1";
export const CHOICE_ID = "018f0000-0000-7000-8000-00000000aa01";
export const ESSAY_ID = "018f0000-0000-7000-8000-00000000aa02";
export const OPTION_A = "018f0000-0000-7000-8000-00000000bb01";
export const OPTION_B = "018f0000-0000-7000-8000-00000000bb02";

export function assignment(
  over: Partial<components["schemas"]["Assignment"]> = {},
): components["schemas"]["Assignment"] {
  return {
    id: ASSIGNMENT_ID,
    testId: "018f0000-0000-7000-8000-0000000000a1",
    testVersionId: "018f0000-0000-7000-8000-0000000000f1",
    testVersion: 3,
    testTitle: "Unit 5 — Present perfect & listening",
    targets: {
      classes: [
        {
          id: "018f0000-0000-7000-8000-0000000000c1",
          name: "IELTS Foundation",
          studentCount: 18,
        },
      ],
      students: [],
    },
    publishedAt: "2026-08-27T00:00:00Z",
    updatedAt: "2026-08-27T02:12:00Z",
    window: {
      opensAt: "2020-09-07T01:00:00Z",
      closesAt: "2099-09-09T14:00:00Z",
      closedAt: null,
    },
    durationMinutes: 45,
    maxAttempts: 2,
    shuffleQuestions: false,
    shuffleOptions: false,
    review: { showScore: true, showCorrectAnswers: false, showExplanations: true },
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: true,
      maxFocusLoss: 2,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
    status: "open",
    submittedCount: 1,
    targetCount: 3,
    flaggedCount: 1,
    pendingGradingCount: 1,
    ...over,
  };
}

export function monitor(
  rows: components["schemas"]["MonitorRow"][] = defaultRows(),
): components["schemas"]["MonitorRow"][] extends never
  ? never
  : {
      serverTime: string;
      questionCount: number;
      rows: components["schemas"]["MonitorRow"][];
    } {
  return { serverTime: new Date().toISOString(), questionCount: 24, rows };
}

export function defaultRows(): components["schemas"]["MonitorRow"][] {
  return [
    {
      studentId: STUDENT_ID,
      fullName: "Phạm Gia Hân",
      state: "in_progress",
      attemptId: ATTEMPT_ID,
      attemptNo: 1,
      startedAt: new Date(Date.now() - 20 * 60_000).toISOString(),
      deadlineAt: new Date(Date.now() + 18 * 60_000).toISOString(),
      submittedAt: null,
      answeredCount: 8,
      score: null,
      focusLossCount: 3,
      flagged: true,
      audioOverLimit: false,
    },
    {
      studentId: "018f0000-0000-7000-8000-0000000000e2",
      fullName: "Nguyễn Đức Minh",
      state: "submitted",
      attemptId: "018f0000-0000-7000-8000-0000000000a8",
      attemptNo: 1,
      startedAt: new Date(Date.now() - 60 * 60_000).toISOString(),
      deadlineAt: new Date(Date.now() - 15 * 60_000).toISOString(),
      submittedAt: new Date(Date.now() - 20 * 60_000).toISOString(),
      answeredCount: 24,
      score: { earned: 26, total: 30, pendingManual: 2 },
      focusLossCount: 1,
      flagged: false,
      audioOverLimit: true,
    },
    {
      studentId: "018f0000-0000-7000-8000-0000000000e3",
      fullName: "Hoàng Tiến Dũng",
      state: "not_started",
      flagged: false,
      audioOverLimit: false,
    },
  ];
}

type Review = {
  attempt: components["schemas"]["Attempt"];
  student: components["schemas"]["User"];
  testTitle: string;
  maxAttempts: number;
  questions: components["schemas"]["AdminQuestion"][];
  answers: Record<
    string,
    {
      answer: components["schemas"]["Answer"] | null;
      autoScore?: number | null;
      manualScore?: number | null;
      requiresManual?: boolean;
      graderComment?: string | null;
    }
  >;
  audioPlays: Record<string, number>;
  integrity: components["schemas"]["IntegritySummary"];
  teacherNote: string | null;
};

export function review(over: { essayScore?: number | null } = {}): Review {
  const essayScore = over.essayScore ?? null;
  return {
    attempt: {
      id: ATTEMPT_ID,
      assignmentId: ASSIGNMENT_ID,
      studentId: STUDENT_ID,
      testVersionId: "018f0000-0000-7000-8000-0000000000f1",
      attemptNo: 1,
      status: essayScore === null ? "submitted" : "submitted",
      startedAt: "2026-09-04T02:10:00Z",
      deadlineAt: "2026-09-04T02:55:00Z",
      submittedAt: "2026-09-04T02:47:00Z",
      gradedAt: null,
      score: {
        earned: 5 + (essayScore ?? 0),
        total: 10,
        pendingManual: essayScore === null ? 1 : 0,
      },
      integrity: { focusLossCount: 1, flagged: false },
    },
    teacherNote: null,
    student: {
      id: STUDENT_ID,
      email: "minh@example.com",
      fullName: "Nguyễn Đức Minh",
      role: "student",
      hasPassword: true,
      linkedProviders: [],
      mustChangePassword: false,
      createdAt: "2026-01-01T00:00:00Z",
    },
    testTitle: "Unit 5",
    maxAttempts: 2,
    questions: [
      {
        id: CHOICE_ID,
        type: "single_choice",
        prompt: "Thủ đô của Việt Nam?",
        points: 5,
        tags: [],
        options: [
          { id: OPTION_A, ordinal: 0, text: "Hà Nội", isCorrect: true },
          { id: OPTION_B, ordinal: 1, text: "Huế", isCorrect: false },
        ],
        blanks: [],
        createdAt: "2026-08-20T00:00:00Z",
        updatedAt: "2026-08-20T00:00:00Z",
      },
      {
        id: ESSAY_ID,
        type: "short_answer",
        prompt: "Viết 2–3 câu tả thói quen buổi sáng.",
        points: 5,
        tags: [],
        sampleAnswer: "I get up at half past six every morning.",
        options: [],
        blanks: [],
        createdAt: "2026-08-20T00:00:00Z",
        updatedAt: "2026-08-20T00:00:00Z",
      },
    ],
    answers: {
      [CHOICE_ID]: {
        answer: { type: "choice", optionIds: [OPTION_A] },
        autoScore: 5,
        requiresManual: false,
      },
      [ESSAY_ID]: {
        answer: { type: "text", value: "I usually wake up at six o'clock." },
        requiresManual: true,
        manualScore: essayScore,
        graderComment: essayScore === null ? null : "Ý tốt.",
      },
    },
    audioPlays: {},
    integrity: {
      totalAwayMs: 6000,
      awayEpisodes: 1,
      pasteCount: 0,
      resumeCount: 0,
      audioReplays: 0,
      offlineEpisodes: 0,
    },
  };
}
