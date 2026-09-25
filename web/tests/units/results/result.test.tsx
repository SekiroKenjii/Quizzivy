import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, Outlet, RouterProvider } from "react-router";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import ResultPage from "@/features/results/pages/ResultPage";
import type { components } from "@/lib/api/schema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { viewport } from "@tests/support/viewport";
import "@/lib/i18n";
import { previewGroup } from "@tests/support/groupPreview";

const BASE = "http://localhost:8080";
const ATTEMPT_ID = "018f0000-0000-7000-8000-0000000000a7";
const OPTION_A = "018f0000-0000-7000-8000-00000000bb01";
const OPTION_B = "018f0000-0000-7000-8000-00000000bb02";
type Review = components["schemas"]["ReviewPolicy"];
type Question = components["schemas"]["ResultQuestion"];

function result(review: Review, transcript: boolean) {
  const choice: Question = {
    id: "018f0000-0000-7000-8000-00000000aa01",
    type: "single_choice",
    prompt: "The letter ____ yesterday.",
    points: 1,
    options: [
      { id: OPTION_A, text: "was wrote" },
      { id: OPTION_B, text: "was written" },
    ],
    answer: { type: "choice", optionIds: [OPTION_A] },
    pendingManual: false,
    ...(review.showScore ? { earned: 0 } : {}),
    ...(review.showCorrectAnswers ? { correctOptionIds: [OPTION_B] } : {}),
    ...(review.showExplanations
      ? { explanation: "Bị động thì quá khứ đơn dùng was/were + phân từ II." }
      : {}),
  };
  const listening: Question = {
    id: "018f0000-0000-7000-8000-00000000aa03",
    type: "single_choice",
    prompt: "Người phụ nữ đề nghị làm gì?",
    points: 1,
    media: {
      id: "018f0000-0000-7000-8000-00000000cc01",
      kind: "audio",
      url: "https://media.example/unit4.mp3",
      mimeType: "audio/mpeg",
      bytes: 159711,
      durationMs: 10004,
      originalFilename: "unit4.mp3",
      createdAt: "2026-08-20T00:00:00Z",
    },
    options: [{ id: "018f0000-0000-7000-8000-00000000bb03", text: "Đi bộ" }],
    answer: { type: "choice", optionIds: ["018f0000-0000-7000-8000-00000000bb03"] },
    pendingManual: false,
    audioPlaysUsed: 2,
    ...(review.showScore ? { earned: 1 } : {}),
    ...(review.showCorrectAnswers
      ? { correctOptionIds: ["018f0000-0000-7000-8000-00000000bb03"] }
      : {}),
    ...(transcript
      ? { transcript: "A: I'm struggling to keep up with the Tuesday class." }
      : {}),
  };
  return {
    attempt: {
      id: ATTEMPT_ID,
      assignmentId: "018f0000-0000-7000-8000-0000000000d1",
      studentId: "018f0000-0000-7000-8000-0000000000e1",
      testVersionId: "018f0000-0000-7000-8000-0000000000f1",
      attemptNo: 1,
      status: "submitted" as const,
      startedAt: "2026-08-26T12:40:00Z",
      deadlineAt: "2026-08-26T13:25:00Z",
      submittedAt: "2026-08-26T13:14:00Z",
      ...(review.showScore
        ? { score: { earned: 1, total: 2, pendingManual: 0 } }
        : { score: null }),
    },
    review,
    testTitle: "Unit 4 — Passive voice",
    maxAttempts: 2,
    questions: [choice, listening],
  };
}

function renderResult() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      {
        element: <Outlet context={{ setTitle: () => {} }} />,
        children: [
          { path: "/app/attempts/:attemptId/result", element: <ResultPage /> },
        ],
      },
    ],
    { initialEntries: [`/app/attempts/${ATTEMPT_ID}/result`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

function serve(body: unknown) {
  server.use(
    http.get(`${BASE}/app/attempts/${ATTEMPT_ID}/result`, () =>
      contractJson("/app/attempts/{id}/result", "get", 200, body),
    ),
  );
}

beforeEach(() => viewport("phone"));
afterEach(() => vi.unstubAllGlobals());

it("keeps shared material under a result filter and follows gaps to a hidden question", async () => {
  const body = result(
    { showScore: true, showCorrectAnswers: false, showExplanations: false },
    false,
  );
  const group = {
    ...previewGroup,
    questionIds: body.questions.map((question) => question.id),
    stimuli: [
      {
        ...previewGroup.stimuli[0]!,
        gaps: [
          {
            kind: "question" as const,
            gapId: "material",
            questionId: body.questions[1]!.id,
          },
        ],
      },
    ],
  };
  serve({
    ...body,
    sharedContext: {
      groups: [group],
      transcripts: {},
      audioPlays: { [group.recordings[0]!.id]: 3 },
    },
  });
  renderResult();
  const user = userEvent.setup();
  await screen.findByText(group.title);
  expect(screen.queryByText("Nội dung bài nghe")).not.toBeInTheDocument();
  expect(
    screen.getByText("Đã nghe 3 lượt · Quy định 2 lượt cho cả nhóm"),
  ).toBeVisible();
  await user.click(screen.getByRole("button", { name: /^Sai/ }));
  expect(screen.getByText(group.title)).toBeVisible();
  expect(screen.queryByText(body.questions[1]!.prompt)).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Ô A — chuyển đến câu 2" }));
  expect(screen.getByText(body.questions[1]!.prompt)).toBeVisible();
  expect(document.activeElement).toHaveAttribute(
    "id",
    `result-question-${body.questions[1]!.id}`,
  );
});

it("shows a released group transcript while score, answer keys and explanations are hidden", async () => {
  const body = result(
    { showScore: false, showCorrectAnswers: false, showExplanations: false },
    false,
  );
  const group = {
    ...previewGroup,
    questionIds: body.questions.map((question) => question.id),
  };
  serve({
    ...body,
    sharedContext: {
      groups: [group],
      transcripts: { [group.recordings[0]!.id]: "A released group transcript." },
      audioPlays: {},
    },
  });
  renderResult();
  await screen.findByText(group.title);
  await userEvent.setup().click(screen.getByText("Nội dung bài nghe"));
  expect(screen.getByText("A released group transcript.")).toBeVisible();
  expect(screen.getByText("Giáo viên chưa công bố điểm")).toBeVisible();
});

describe("the result page", () => {
  const flags: [boolean, boolean, boolean][] = [
    [false, false, false],
    [true, false, false],
    [false, true, false],
    [false, false, true],
    [true, true, false],
    [true, false, true],
    [false, true, true],
    [true, true, true],
  ];

  it.each(flags)(
    "showScore=%s showCorrectAnswers=%s showExplanations=%s renders exactly its blocks",
    async (showScore, showCorrectAnswers, showExplanations) => {
      serve(result({ showScore, showCorrectAnswers, showExplanations }, true));
      renderResult();
      await screen.findByText("The letter ____ yesterday.");

      // score tile vs the locked card
      expect(screen.queryByText("Điểm của bạn") !== null).toBe(showScore);
      expect(screen.queryByText("Giáo viên chưa công bố điểm") !== null).toBe(
        !showScore,
      );
      expect(screen.getByText("Nộp lúc 20:14 · 26/08 · Lượt 1/2")).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /^Sai/ }) !== null).toBe(showScore);

      // the key
      expect(screen.queryAllByText("đáp án đúng").length > 0).toBe(showCorrectAnswers);
      expect(
        screen.queryByText("Giáo viên không hiển thị đáp án đúng cho bài này.") !==
          null,
      ).toBe(!showCorrectAnswers);

      // explanations
      expect(screen.queryByText(/Bị động thì quá khứ đơn/) !== null).toBe(
        showExplanations,
      );
      expect(
        screen.queryByText("Giáo viên không hiển thị giải thích cho bài này.") !== null,
      ).toBe(!showExplanations);

      // the student's own choice is always marked
      expect(screen.getAllByText("bạn chọn").length).toBeGreaterThan(0);
      expect(screen.getByText("Xem lời thoại")).toBeInTheDocument();
    },
  );

  it("says the transcript is withheld in the transcript's own slot", async () => {
    serve(
      result(
        { showScore: true, showCorrectAnswers: true, showExplanations: true },
        false,
      ),
    );
    renderResult();
    await screen.findByText("Người phụ nữ đề nghị làm gì?");
    expect(screen.getByText("Lời thoại không được hiển thị.")).toBeInTheDocument();
    expect(screen.queryByText("Xem lời thoại")).not.toBeInTheDocument();
  });

  it("shows the not-ready card on a 409 with one way home", async () => {
    server.use(
      http.get(`${BASE}/app/attempts/${ATTEMPT_ID}/result`, () =>
        HttpResponse.json(
          {
            error: {
              code: "ATTEMPT_IN_PROGRESS",
              message: "Bài chưa được nộp.",
              requestId: "018f0000-0000-7000-8000-00000000dd01",
            },
          },
          { status: 409 },
        ),
      ),
    );
    renderResult();
    expect(await screen.findByText("Kết quả chưa sẵn sàng.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Về trang chủ" })).toHaveAttribute(
      "href",
      "/app",
    );
  });
});

describe("from 1024px (S-16)", () => {
  beforeEach(() => viewport("desktop"));

  it("keeps the title, score and counts above the paper", async () => {
    serve(
      result(
        { showScore: true, showCorrectAnswers: false, showExplanations: true },
        true,
      ),
    );
    renderResult();
    await screen.findByText("The letter ____ yesterday.");

    const panel = screen;
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(panel.getByText("Điểm của bạn")).toBeInTheDocument();
    expect(
      panel.getByText("Đúng", { selector: "dt" }).nextElementSibling,
    ).toHaveTextContent("1");
    expect(
      panel.getByText("Sai", { selector: "dt" }).nextElementSibling,
    ).toHaveTextContent("1");
    expect(
      panel.getByText("Giáo viên không hiển thị đáp án đúng cho bài này."),
    ).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Unit 4");
    expect(screen.getByText("Nộp lúc 20:14 · 26/08 · Lượt 1/2")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Bài của tôi" })).toHaveAttribute(
      "href",
      "/app",
    );
  });
});

it("explains an empty result filter and lets the student return to all questions", async () => {
  const user = userEvent.setup();
  serve(
    result(
      { showScore: true, showCorrectAnswers: false, showExplanations: false },
      false,
    ),
  );
  renderResult();
  await user.click(await screen.findByRole("button", { name: /Chờ chấm/ }));
  expect(screen.getByText("Không còn câu nào đang chờ chấm.")).toBeInTheDocument();
  expect(screen.queryByText("The letter ____ yesterday.")).toBeNull();
  await user.click(screen.getByRole("button", { name: "Xem tất cả câu" }));
  expect(screen.getByText("The letter ____ yesterday.")).toBeInTheDocument();
});
