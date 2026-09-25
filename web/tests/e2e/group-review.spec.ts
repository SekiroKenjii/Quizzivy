import { fileURLToPath } from "node:url";
import { expect, test, type Page } from "@playwright/test";
import { adminUser, sessionAs, studentUser, stubApi } from "./support/api";
import { previewGroup, previewQuestions } from "../support/groupPreview";
import type { AttemptReview } from "../../src/features/attempts/api";
import type { AttemptResult } from "../../src/features/results/api";

const attemptId = "01935000-0000-7000-8000-000000000088";
const recordingId = previewGroup.recordings[0]!.id;
const audio = fileURLToPath(new URL("./fixtures/unit5-listening.mp3", import.meta.url));
const questions = previewQuestions.slice(1);
const transcript = "Nội dung hội thoại được giáo viên cho phép xem lại.";
const attempt = {
  id: attemptId,
  assignmentId: attemptId,
  studentId: studentUser.id,
  testVersionId: attemptId,
  attemptNo: 1,
  status: "submitted" as const,
  startedAt: "2026-09-24T01:00:00Z",
  deadlineAt: "2026-09-24T02:00:00Z",
  submittedAt: "2026-09-24T01:30:00Z",
  score: { earned: 1, total: 2, pendingManual: 0 },
};
const sharedContext = {
  groups: [previewGroup],
  transcripts: { [recordingId]: transcript },
  audioPlays: { [recordingId]: 3 },
};

function result(): AttemptResult {
  return {
    attempt,
    testTitle: "Đọc hiểu và nghe theo nhóm",
    maxAttempts: 1,
    review: { showScore: true, showCorrectAnswers: false, showExplanations: false },
    sharedContext,
    questions: questions.map((question, index) => ({
      ...question,
      answer: { type: "choice", optionIds: [question.options![0]!.id] },
      pendingManual: false,
      earned: index,
    })),
  };
}

function review(): AttemptReview {
  return {
    attempt: { ...attempt, score: { earned: 1, total: 2, pendingManual: 1 } },
    student: { ...studentUser, linkedProviders: [] },
    testTitle: "Đọc hiểu và nghe theo nhóm",
    maxAttempts: 1,
    teacherNote: null,
    audioPlays: {},
    sharedContext,
    integrity: {
      totalAwayMs: 0,
      awayEpisodes: 0,
      pasteCount: 0,
      resumeCount: 0,
      audioReplays: 0,
      offlineEpisodes: 0,
    },
    answers: {
      [questions[0]!.id]: { answer: null, autoScore: 0 },
      [questions[1]!.id]: {
        answer: { type: "text", value: "Câu trả lời cần giáo viên chấm." },
        requiresManual: true,
      },
    },
    questions: questions.map((question, index) => ({
      ...question,
      type: index === 1 ? "short_answer" : "single_choice",
      tags: [],
      blanks: [],
      options:
        index === 1
          ? []
          : question.options!.map((option) => ({
              ...option,
              ordinal: 0,
              isCorrect: true,
            })),
      createdAt: attempt.startedAt,
      updatedAt: attempt.startedAt,
    })),
  };
}

async function setup(page: Page, teacher: boolean) {
  let newPlays = 0;
  page.on("request", (request) => {
    if (request.url().endsWith("/group-audio-play")) newPlays += 1;
  });
  const paper = review();
  await stubApi(page, {
    ...sessionAs(teacher ? adminUser : studentUser),
    [`GET /app/attempts/${attemptId}/result`]: { body: result() },
    [`GET /admin/attempts/${attemptId}`]: { body: paper },
    [`GET /admin/attempts/${attemptId}/events`]: {
      body: { startedAt: attempt.startedAt, events: [], summary: paper.integrity },
    },
    [`GET /admin/assignments/${attemptId}/answers`]: {
      body: {
        question: paper.questions[1],
        questionNumber: 2,
        questionCount: 2,
        manualQuestionIds: [questions[1]!.id],
        items: [],
        sharedContext: {
          groups: [previewGroup],
          transcripts: sharedContext.transcripts,
        },
      },
    },
  });
  await page.route("https://assets.example/synthetic.mp3", (route) =>
    route.fulfill({ path: audio, contentType: "audio/mpeg" }),
  );
  await page.goto(
    teacher ? `/admin/attempts/${attemptId}` : `/app/attempts/${attemptId}/result`,
  );
  return () => newPlays;
}

async function checkLayout(page: Page, name: string) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth + 1,
    ),
  ).toBe(false);
  await page.screenshot({ path: test.info().outputPath(`${name}.png`) });
}

for (const width of [320, 1440]) {
  test(`result keeps shared context through filtering at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 950 });
    const plays = await setup(page, false);
    await expect(page.getByRole("heading", { name: previewGroup.title })).toBeVisible();
    await page.getByRole("button", { name: /^Sai/ }).click();
    await expect(page.getByText(questions[1]!.prompt, { exact: true })).toHaveCount(0);
    const gap = page.getByRole("button", { name: "Ô A — chuyển đến câu 2" });
    await gap.focus();
    await page.keyboard.press("Enter");
    await expect(page.locator(":focus")).toHaveAttribute(
      "id",
      `result-question-${questions[1]!.id}`,
    );
    await page.getByText("Nội dung bài nghe", { exact: true }).click();
    await expect(page.getByText(transcript, { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Phát", exact: true }).click();
    await expect(
      page.getByRole("button", { name: "Tạm dừng", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByText("Đã nghe 3 lượt · Quy định 2 lượt cho cả nhóm"),
    ).toBeVisible();
    expect(plays()).toBe(0);
    await checkLayout(page, `group-result-${width}`);
  });
}

for (const width of [768, 1440]) {
  test(`teacher sees shared context in both grading modes at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 950 });
    const plays = await setup(page, true);
    await expect(page.getByRole("heading", { name: previewGroup.title })).toBeVisible();
    await page.getByText("Nội dung bài nghe", { exact: true }).click();
    await expect(page.getByText(transcript, { exact: true })).toBeVisible();
    await page.getByRole("button", { name: "Phát", exact: true }).click();
    await expect(
      page.getByRole("button", { name: "Tạm dừng", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Ô A — chuyển đến câu 2" }).click();
    await expect(page.locator(":focus")).toHaveAttribute("id", "review-answer");
    await checkLayout(page, `group-review-${width}`);
    await page.getByRole("button", { name: "Chấm theo câu hỏi", exact: true }).click();
    await expect(page.getByRole("heading", { name: previewGroup.title })).toBeVisible();
    await expect(page.getByText("Ngữ liệu dùng chung · Câu 1–2")).toBeVisible();
    await expect(page.getByText(/^Đã nghe/)).toHaveCount(0);
    await page.getByText("Nội dung bài nghe", { exact: true }).click();
    await expect(page.getByText(transcript, { exact: true })).toBeVisible();
    expect(plays()).toBe(0);
    await checkLayout(page, `group-question-grading-${width}`);
  });
}
