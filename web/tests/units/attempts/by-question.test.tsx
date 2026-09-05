import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AttemptReviewPage from "@/features/attempts/pages/AttemptReviewPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import { ASSIGNMENT_ID, ATTEMPT_ID, BASE, ESSAY_ID, review } from "./fixtures";

const OTHER_ATTEMPT = "018f0000-0000-7000-8000-0000000000a8";
let grades: { attemptId: string; body: unknown }[] = [];

beforeEach(() => {
  grades = [];
  const paper = review();
  server.use(
    http.get(`${BASE}/admin/attempts/${ATTEMPT_ID}`, () =>
      contractJson("/admin/attempts/{id}", "get", 200, paper),
    ),
    http.get(`${BASE}/admin/assignments/${ASSIGNMENT_ID}/answers`, ({ request }) => {
      const url = new URL(request.url);
      expect(url.searchParams.get("questionId")).toBe(ESSAY_ID);
      return contractJson("/admin/assignments/{id}/answers", "get", 200, {
        question: paper.questions[1],
        questionNumber: 2,
        questionCount: 2,
        manualQuestionIds: [ESSAY_ID],
        items: [
          {
            attemptId: ATTEMPT_ID,
            studentId: paper.student.id,
            studentName: "Nguyễn Đức Minh",
            attemptNo: 1,
            answer: { type: "text", value: "I usually wake up at six." },
            manualScore: 4,
            graderComment: null,
          },
          {
            attemptId: OTHER_ATTEMPT,
            studentId: "018f0000-0000-7000-8000-0000000000e2",
            studentName: "Phạm Gia Hân",
            attemptNo: 1,
            answer: { type: "text", value: "I wake up 6 o'clock." },
            manualScore: null,
            graderComment: null,
          },
        ],
      });
    }),
    http.post(`${BASE}/admin/attempts/:id/grade`, async ({ params, request }) => {
      grades.push({ attemptId: String(params["id"]), body: await request.json() });
      return contractJson("/admin/attempts/{id}/grade", "post", 200, {
        earned: 8,
        total: 10,
        pendingManual: 0,
      });
    }),
  );
});

function renderReview() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [{ path: "/admin/attempts/:id", element: <AttemptReviewPage /> }],
    { initialEntries: [`/admin/attempts/${ATTEMPT_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

describe("G-04, grading one question across every paper", () => {
  it("hides names, pins the rubric, and grades a paper from its row", async () => {
    const user = renderReview();
    await user.click(await screen.findByRole("button", { name: "Chấm theo câu hỏi" }));

    expect(await screen.findByText("Học viên 01")).toBeInTheDocument();
    expect(screen.getByText("Học viên 02")).toBeInTheDocument();
    expect(screen.queryByText("Phạm Gia Hân")).toBeNull();
    expect(screen.getByText("Unit 5 · Câu 2 / 2")).toBeInTheDocument();
    expect(screen.getByText("Đã chấm 1/2")).toBeInTheDocument();
    expect(
      screen.getByText("I get up at half past six every morning."),
    ).toBeInTheDocument();

    const second = screen
      .getByText("Học viên 02")
      .closest<HTMLElement>("[data-slot='card']")!;
    const points = within(second).getByRole("spinbutton");
    await user.type(points, "3{Enter}");

    await waitFor(() => expect(grades).toHaveLength(1));
    expect(grades[0]).toEqual({
      attemptId: OTHER_ATTEMPT,
      body: { items: [{ questionId: ESSAY_ID, points: 3, comment: null }] },
    });
  });

  it("reveals names on request", async () => {
    const user = renderReview();
    await user.click(await screen.findByRole("button", { name: "Chấm theo câu hỏi" }));
    await screen.findByText("Học viên 01");

    await user.click(screen.getByRole("button", { name: "Hiện tên" }));

    expect(screen.getByText("Phạm Gia Hân")).toBeInTheDocument();
    expect(screen.queryByText("Học viên 02")).toBeNull();
  });
});
