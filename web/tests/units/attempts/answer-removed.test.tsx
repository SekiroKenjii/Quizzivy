import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AttemptReviewPage from "@/features/attempts/pages/teacher/AttemptReviewPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import {
  ASSIGNMENT_ID,
  ATTEMPT_ID,
  BASE,
  CHOICE_ID,
  ESSAY_ID,
  review,
} from "./fixtures";

const NOTICE = "Câu trả lời đã được xoá theo chính sách lưu trữ của trung tâm.";
const ESSAY_TEXT = "I usually wake up at six o'clock.";
const REFUSAL = "Câu trả lời này đã được xoá nên không thể chấm.";
const OTHER_ATTEMPT = "018f0000-0000-7000-8000-0000000000a8";

type Paper = ReturnType<typeof review>;

function removed(paper: Paper, essay: { score: number | null }): Paper {
  return {
    ...paper,
    attempt: {
      ...paper.attempt,
      score: { earned: 5 + (essay.score ?? 0), total: 10, pendingManual: 0 },
    },
    answers: {
      [CHOICE_ID]: {
        answer: null,
        answerRemoved: true,
        autoScore: 5,
        requiresManual: false,
      },
      [ESSAY_ID]: {
        answer: null,
        answerRemoved: true,
        requiresManual: true,
        manualScore: essay.score,
        graderComment: essay.score === null ? null : "Ý tốt.",
      },
    },
  };
}

let current: Paper = review();
let refuse = false;
let grades = 0;

function refusal() {
  return contractJson("/teacher/attempts/{id}/grade", "post", 409, {
    error: {
      code: "ANSWER_REMOVED",
      message: REFUSAL,
      requestId: "018f0000-0000-7000-8000-00000000dd01",
      details: { questionIds: [ESSAY_ID] },
    },
  });
}

beforeEach(() => {
  current = review();
  refuse = false;
  grades = 0;
  server.use(
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}`, () =>
      contractJson("/teacher/attempts/{id}", "get", 200, current),
    ),
    http.post(`${BASE}/teacher/attempts/:id/grade`, () => {
      grades += 1;
      if (refuse) {
        current = removed(review(), { score: null });
        return refusal();
      }
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 8,
        total: 10,
        pendingManual: 0,
      });
    }),
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}/events`, () =>
      contractJson("/teacher/attempts/{id}/events", "get", 200, {
        startedAt: "2026-09-04T02:10:00Z",
        events: [],
        summary: review().integrity,
      }),
    ),
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

describe("an answer the retention policy removed, in the teacher's review", () => {
  it("names the removal in place of a choice, and keeps its mark", async () => {
    current = removed(review(), { score: 3 });
    renderReview();
    expect(await screen.findByText("Thủ đô của Việt Nam?")).toBeInTheDocument();
    expect(screen.getByText(NOTICE)).toBeInTheDocument();
    expect(screen.queryByText("Hà Nội")).toBeNull();
    expect(screen.queryByText("học viên chọn · đúng")).toBeNull();
    expect(screen.getByRole("button", { name: "Câu 1, Đúng" })).toBeInTheDocument();
  });

  it("draws no score editor for a removed essay, and shows the score and comment it kept", async () => {
    current = removed(review(), { score: 3 });
    const user = renderReview();
    await user.click(await screen.findByRole("button", { name: /^Câu 2,/ }));

    expect(
      screen.getByText("Viết 2–3 câu tả thói quen buổi sáng."),
    ).toBeInTheDocument();
    expect(screen.getByText(NOTICE)).toBeInTheDocument();
    expect(screen.queryByText(ESSAY_TEXT)).toBeNull();
    expect(screen.queryByLabelText("Điểm")).toBeNull();
    expect(screen.queryByRole("button", { name: "Lưu & câu tiếp theo" })).toBeNull();
    const kept = screen.getByText("Ý tốt.").closest<HTMLElement>("[data-slot='card']")!;
    expect(within(kept).getByText("3 / 5")).toBeInTheDocument();
    expect(within(kept).getByText("Điểm")).toBeInTheDocument();
  });

  it("does not count a removed essay with no score as waiting for a mark", async () => {
    current = removed(review(), { score: null });
    renderReview();
    expect(
      await screen.findByRole("button", { name: "Câu 2, Đã xoá" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("còn 1 câu chờ chấm")).toBeNull();
    expect(screen.getByRole("button", { name: "Hoàn tất chấm" })).toBeEnabled();
    expect(screen.queryByLabelText("Điểm")).toBeNull();
  });

  it("leaves an answer that was kept as it was", async () => {
    renderReview();
    expect(await screen.findByText(ESSAY_TEXT)).toBeInTheDocument();
    expect(screen.queryByText(NOTICE)).toBeNull();
    expect(screen.getByLabelText("Điểm")).toBeInTheDocument();
  });

  it("shows the server's refusal and reloads the paper when a save names a removed answer", async () => {
    refuse = true;
    const user = renderReview();
    const points = await screen.findByLabelText("Điểm");
    await user.type(points, "4");
    await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));

    expect(await screen.findByText(REFUSAL)).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText(NOTICE)).toBeInTheDocument());
    expect(screen.queryByLabelText("Điểm")).toBeNull();
    expect(screen.queryByText(ESSAY_TEXT)).toBeNull();
    expect(grades).toBe(1);
  });
});

describe("an answer the retention policy removed, graded by question", () => {
  let refuseRow = false;

  beforeEach(() => {
    refuseRow = false;
    current = review();
    server.use(
      http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/answers`, () =>
        contractJson("/teacher/assignments/{id}/answers", "get", 200, {
          question: current.questions[1],
          questionNumber: 2,
          questionCount: 2,
          manualQuestionIds: [ESSAY_ID],
          items: [
            {
              attemptId: ATTEMPT_ID,
              studentId: current.student.id,
              studentName: "Nguyễn Đức Minh",
              attemptNo: 1,
              answer: null,
              answerRemoved: true,
              manualScore: 2,
              graderComment: null,
            },
            {
              attemptId: OTHER_ATTEMPT,
              studentId: "018f0000-0000-7000-8000-0000000000e2",
              studentName: "Phạm Gia Hân",
              attemptNo: 1,
              answer: refuseRow
                ? null
                : { type: "text", value: "I wake up 6 o'clock." },
              ...(refuseRow ? { answerRemoved: true } : {}),
              manualScore: null,
              graderComment: null,
            },
          ],
        }),
      ),
      http.post(`${BASE}/teacher/attempts/:id/grade`, () => {
        grades += 1;
        refuseRow = true;
        return refusal();
      }),
    );
  });

  function row(label: string) {
    return screen.getByText(label).closest<HTMLElement>("[data-slot='card']")!;
  }

  it("names the removal in its row with no points box, and leaves the other rows alone", async () => {
    const user = renderReview();
    await user.click(await screen.findByRole("button", { name: "Chấm theo câu hỏi" }));
    await screen.findByText("Học viên 01");

    const gone = row("Học viên 01");
    expect(within(gone).getByText(NOTICE)).toBeInTheDocument();
    expect(within(gone).queryByRole("spinbutton")).toBeNull();
    expect(within(gone).queryByRole("button", { name: "Lưu" })).toBeNull();
    expect(within(gone).getByText("2/5")).toBeInTheDocument();
    expect(within(gone).queryByText("Không trả lời")).toBeNull();

    const kept = row("Học viên 02");
    expect(within(kept).getByText("I wake up 6 o'clock.")).toBeInTheDocument();
    expect(within(kept).getByRole("spinbutton")).toBeInTheDocument();
    expect(within(kept).queryByText(NOTICE)).toBeNull();
    expect(screen.getByText("Đã chấm 0/1")).toBeInTheDocument();
  });

  it("shows the server's refusal and reloads the rows when a save names a removed answer", async () => {
    const user = renderReview();
    await user.click(await screen.findByRole("button", { name: "Chấm theo câu hỏi" }));
    await screen.findByText("Học viên 02");
    await user.type(within(row("Học viên 02")).getByRole("spinbutton"), "3{Enter}");

    expect(await screen.findByRole("alert")).toHaveTextContent(REFUSAL);
    await waitFor(() =>
      expect(within(row("Học viên 02")).getByText(NOTICE)).toBeInTheDocument(),
    );
    expect(within(row("Học viên 02")).queryByRole("spinbutton")).toBeNull();
    expect(grades).toBe(1);
  });
});
