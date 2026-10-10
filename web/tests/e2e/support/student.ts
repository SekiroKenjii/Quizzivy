import { expect, type Page } from "@playwright/test";
import { sessionAs, studentUser, stubApi } from "./api";
import type { AttemptResult } from "../../../src/features/results/api";
import type { components } from "../../../src/lib/api/schema";

type Assignment = components["schemas"]["StudentAssignmentCard"];

export const classId = "018f0000-0000-7000-8000-0000000000c1";

export const classes = [
  {
    id: classId,
    name: "Lớp luyện tập buổi tối",
    description: "Thứ ba và thứ năm",
    teacherName: "Cô Thương",
    joinedAt: "2026-01-01T00:00:00Z",
  },
];

/** assignment is one open paper of the evening class, with or without an attempt in progress. */
export function assignment(index: number, live: boolean): Assignment {
  return {
    id: `assignment-${index}`,
    testTitle: `Bài luyện tập ${index}`,
    classId,
    className: classes[0]!.name,
    status: "open",
    opensAt: new Date(Date.now() - 3600000).toISOString(),
    closesAt: new Date(Date.now() + 86400000 * 15).toISOString(),
    durationMinutes: 45,
    questionCount: 24,
    totalPoints: 30,
    attemptsUsed: live ? 1 : 0,
    maxAttempts: 2,
    hasLiveAttempt: live,
    liveDeadlineAt: live ? new Date(Date.now() + index * 600000).toISOString() : null,
  };
}

/** student signs the student in with one class, two papers in progress and one still to start. */
export async function student(
  page: Page,
  user: Parameters<typeof sessionAs>[0] &
    Pick<components["schemas"]["CurrentUser"], "locale" | "preferences"> = studentUser,
) {
  await stubApi(page, {
    ...sessionAs(user),
    "GET /app/classes": { body: { items: classes } },
    "GET /app/assignments": {
      body: {
        dueNow: [assignment(1, true), assignment(2, true), assignment(3, false)],
        upcoming: [],
        completed: [],
      },
    },
  });
}

/** fits waits until neither the document, the deck surface nor `<main>` scrolls sideways. */
export async function fits(page: Page) {
  await expect
    .poll(() =>
      page.evaluate(() =>
        [
          document.documentElement,
          document.querySelector("[data-scale='deck']"),
          document.querySelector("main"),
        ].every((node) => node === null || node.scrollWidth <= node.clientWidth + 1),
      ),
    )
    .toBe(true);
}

/** gradedResult is a marked one-question paper whose answer is right, so its Wrong filter is empty. */
export const gradedResult: AttemptResult = {
  testTitle: "Bài kiểm tra có tiêu đề dài cần hiển thị đầy đủ trên điện thoại",
  maxAttempts: 2,
  review: {
    showScore: true,
    showCorrectAnswers: false,
    showExplanations: false,
    release: "on_submit",
    showClassAverage: false,
  },
  attempt: {
    id: "result",
    assignmentId: "assignment",
    studentId: studentUser.id,
    testVersionId: "version",
    attemptNo: 1,
    status: "submitted",
    startedAt: "2026-09-22T00:00:00Z",
    deadlineAt: "2026-09-22T01:00:00Z",
    submittedAt: "2026-09-22T00:30:00Z",
    score: { earned: 1, total: 1, pendingManual: 0 },
  },
  sections: [{ id: "section", title: "Phần 1", instructions: null }],
  questions: [
    {
      id: "question",
      sectionId: "section",
      type: "short_answer",
      prompt: "Câu trả lời đã chấm",
      points: 1,
      answer: { type: "text", value: "Đã trả lời" },
      earned: 1,
      pendingManual: false,
    },
  ],
};
