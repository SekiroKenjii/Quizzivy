import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AttemptReviewPage from "@/features/attempts/pages/teacher/AttemptReviewPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import { ATTEMPT_ID, BASE, ESSAY_ID, review } from "./fixtures";

const SECOND_ESSAY = "018f0000-0000-7000-8000-00000000aa03";

let scores: Record<string, number | null> = {};
let flagged = false;
let finished = 0;
let held: Promise<void> = Promise.resolve();
let release: () => void = () => {};

function hold() {
  held = new Promise((resolve) => {
    release = resolve;
  });
}

function paper() {
  const base = review();
  const essay = base.questions.find((q) => q.id === ESSAY_ID)!;
  const pendingManual = Object.values(scores).filter((s) => s === null).length;
  return {
    ...base,
    attempt: {
      ...base.attempt,
      status: finished > 0 ? ("graded" as const) : base.attempt.status,
      score: { earned: 5, total: 15, pendingManual },
      integrity: { ...base.attempt.integrity, flagged },
    },
    questions: [
      ...base.questions,
      { ...essay, id: SECOND_ESSAY, prompt: "Tả căn phòng của bạn." },
    ],
    answers: {
      ...base.answers,
      [ESSAY_ID]: {
        answer: { type: "text" as const, value: "I wake up at six." },
        requiresManual: true,
        manualScore: scores[ESSAY_ID] ?? null,
      },
      [SECOND_ESSAY]: {
        answer: { type: "text" as const, value: "My room is small." },
        requiresManual: true,
        manualScore: scores[SECOND_ESSAY] ?? null,
      },
    },
  };
}

beforeEach(() => {
  scores = { [ESSAY_ID]: null, [SECOND_ESSAY]: null };
  flagged = false;
  finished = 0;
  held = Promise.resolve();
  server.use(
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}`, () =>
      contractJson("/teacher/attempts/{id}", "get", 200, paper()),
    ),
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}/events`, () =>
      contractJson("/teacher/attempts/{id}/events", "get", 200, {
        startedAt: "2026-09-04T02:10:00Z",
        events: [],
        summary: review().integrity,
      }),
    ),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, async ({ request }) => {
      const body = (await request.json()) as {
        items: { questionId: string; points: number }[];
      };
      for (const item of body.items) scores[item.questionId] = item.points;
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 5,
        total: 15,
        pendingManual: Object.values(scores).filter((s) => s === null).length,
      });
    }),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/flag`, async ({ request }) => {
      flagged = ((await request.json()) as { flagged: boolean }).flagged;
      await held;
      return contractJson("/teacher/attempts/{id}/flag", "post", 200, paper().attempt);
    }),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/finish-grading`, async () => {
      finished += 1;
      await held;
      return contractJson(
        "/teacher/attempts/{id}/finish-grading",
        "post",
        200,
        paper().attempt,
      );
    }),
  );
});

function renderReview() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/teacher/attempts/:id", element: <AttemptReviewPage /> }],
    { initialEntries: [`/teacher/attempts/${ATTEMPT_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("an attempt the teacher cannot open (VER-47 obs 2)", () => {
  it("says it was not found, with no Retry, when the server answers 404", async () => {
    server.use(
      http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}`, () =>
        contractJson("/teacher/attempts/{id}", "get", 404, {
          error: {
            code: "NOT_FOUND",
            message: "Not found.",
            requestId: "018f0000-0000-7000-8000-0000000009b1",
          },
        }),
      ),
    );
    renderReview();
    expect(await screen.findByText("Không tìm thấy bài làm này.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Thử lại" })).toBeNull();
    expect(screen.queryByText("Điểm và nhận xét đã lưu vẫn còn nguyên.")).toBeNull();
  });
});

describe("the review keeps keyboard focus through its own actions (QA-47-1)", () => {
  it("does not take focus when the page first opens", async () => {
    renderReview();
    await screen.findByLabelText("Điểm");
    expect(document.body).toHaveFocus();
  });

  it("puts focus in the next answer's points box after Enter saves", async () => {
    const user = renderReview();
    const points = await screen.findByLabelText("Điểm");
    await user.click(points);
    await user.type(points, "4{Enter}");
    await screen.findByText("Tả căn phòng của bạn.");
    await waitFor(() => expect(screen.getByLabelText("Điểm")).toHaveFocus());
    expect(screen.getByLabelText("Điểm")).not.toBe(points);
    expect(scores[ESSAY_ID]).toBe(4);
  });

  it("puts focus in the next answer's points box after Save & next is clicked", async () => {
    const user = renderReview();
    await user.type(await screen.findByLabelText("Điểm"), "3");
    await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
    await screen.findByText("Tả căn phòng của bạn.");
    await waitFor(() => expect(screen.getByLabelText("Điểm")).toHaveFocus());
  });

  it("keeps one focusable flag button while Flag becomes Remove flag and back", async () => {
    const user = renderReview();
    const button = await screen.findByRole("button", { name: "Đánh dấu" });
    hold();
    await user.click(button);
    await waitFor(() => expect(button).toHaveAttribute("aria-disabled", "true"));
    expect(button).toBeEnabled();
    expect(button).toHaveFocus();
    release();
    await waitFor(() => expect(button).toHaveAccessibleName("Bỏ đánh dấu"));
    expect(button).toHaveFocus();
    await user.keyboard("{Enter}");
    await waitFor(() => expect(button).toHaveAccessibleName("Đánh dấu"));
    expect(button).toHaveFocus();
  });

  it("keeps focus on Finish grading, which stays enabled, once the paper is graded", async () => {
    scores = { [ESSAY_ID]: 4, [SECOND_ESSAY]: 3 };
    const user = renderReview();
    const finish = await screen.findByRole("button", { name: "Hoàn tất chấm" });
    hold();
    await user.click(finish);
    await waitFor(() => expect(finish).toHaveAttribute("aria-disabled", "true"));
    expect(finish).toBeEnabled();
    release();
    await waitFor(() => expect(finished).toBe(1));
    expect(await screen.findByText("Đã chấm")).toBeInTheDocument();
    expect(finish).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Hoàn tất chấm" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Hoàn tất chấm" })).toHaveFocus();
  });
});
