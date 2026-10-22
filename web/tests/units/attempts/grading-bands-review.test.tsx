import { beforeEach, expect, it } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, MemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import AttemptReviewPage from "@/features/attempts/pages/teacher/AttemptReviewPage";
import { GradeByQuestion } from "@/features/attempts/components/GradeByQuestion";
import type { AttemptReview } from "@/features/attempts/api";
import { setLocale } from "@/lib/i18n";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  ASSIGNMENT_ID,
  ATTEMPT_ID,
  BASE,
  ESSAY_ID,
  STUDENT_ID,
  review,
} from "./fixtures";

const TASK = "018f0000-0000-7000-8000-00000000cc01";
const COHERENCE = "018f0000-0000-7000-8000-00000000cc02";
const rubric = [
  { id: TASK, ordinal: 0, label: "Task achievement" },
  { id: COHERENCE, ordinal: 1, label: "Coherence and cohesion" },
];

let graded: unknown[];
let refuse: string | null;

function essayPaper(withRubric: boolean): AttemptReview {
  const paper = review();
  const question = paper.questions[1]!;
  question.type = "essay";
  question.points = 9;
  question.rubric = withRubric ? rubric : [];
  return paper;
}

function serve(paper: AttemptReview) {
  server.use(
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}`, () =>
      contractJson("/teacher/attempts/{id}", "get", 200, paper),
    ),
    http.post(`${BASE}/teacher/attempts/:id/grade`, async ({ request }) => {
      graded.push(await request.json());
      if (refuse)
        return HttpResponse.json(
          {
            error: {
              code: refuse,
              message: "Refused",
              requestId: "req-1",
              details: { questionIds: [ESSAY_ID] },
            },
          },
          { status: 422 },
        );
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 11,
        total: 14,
        pendingManual: 0,
      });
    }),
  );
}

function renderReview() {
  const router = createMemoryRouter(
    [{ path: "/teacher/attempts/:id", element: <AttemptReviewPage /> }],
    { initialEntries: [`/teacher/attempts/${ATTEMPT_ID}`] },
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return userEvent.setup({ delay: null });
}

beforeEach(async () => {
  await act(async () => setLocale("en"));
  graded = [];
  refuse = null;
});

it("grades an essay on the paper review with its band, not a points box", async () => {
  serve(essayPaper(false));
  const user = renderReview();
  const scale = await screen.findByRole("group", { name: "Band" });
  expect(screen.queryByLabelText("Points")).toBeNull();
  const save = screen.getByRole("button", { name: "Save & next" });
  expect(save).toHaveAttribute("aria-disabled", "true");
  await user.click(save);
  expect(screen.getByRole("alert")).toHaveTextContent("Pick a band first (keys 3–9).");
  expect(graded).toEqual([]);
  await user.click(within(scale).getByRole("button", { name: "Give band 6.5" }));
  await user.click(save);
  await waitFor(() =>
    expect(graded).toEqual([
      { items: [{ questionId: ESSAY_ID, band: 6.5, comment: null }] },
    ]),
  );
});

it("keeps Save & next paused on the paper review until every criterion has a band", async () => {
  serve(essayPaper(true));
  const user = renderReview();
  const task = await screen.findByRole("group", { name: "Task achievement" });
  const save = screen.getByRole("button", { name: "Save & next" });
  await user.click(within(task).getByRole("button", { name: "Give band 7.0" }));
  expect(save).toHaveAttribute("aria-disabled", "true");
  expect(save).toBeEnabled();
  await user.click(
    within(screen.getByRole("group", { name: "Coherence and cohesion" })).getByRole(
      "button",
      { name: "Give band 6.0" },
    ),
  );
  expect(screen.getByText("Overall band: 6.5")).toBeVisible();
  await user.click(save);
  await waitFor(() =>
    expect(graded).toEqual([
      {
        items: [
          {
            questionId: ESSAY_ID,
            criteria: [
              { criterionId: TASK, band: 7 },
              { criterionId: COHERENCE, band: 6 },
            ],
            comment: null,
          },
        ],
      },
    ]),
  );
});

it("shows a 422 grade refusal on the paper review as a field message", async () => {
  refuse = "GRADE_NEEDS_BAND";
  serve(essayPaper(false));
  const user = renderReview();
  await user.click(await screen.findByRole("button", { name: "Give band 5.0" }));
  await user.click(screen.getByRole("button", { name: "Save & next" }));
  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("Choose a band for this essay.");
  expect(screen.queryByText("Refused")).toBeNull();
});

it("grades essays question by question, one band scale per paper", async () => {
  const user = userEvent.setup({ delay: null });
  server.use(
    http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/answers`, () =>
      contractJson("/teacher/assignments/{id}/answers", "get", 200, {
        question: { ...essayPaper(false).questions[1]! },
        questionNumber: 2,
        questionCount: 2,
        manualQuestionIds: [ESSAY_ID],
        items: [
          {
            attemptId: ATTEMPT_ID,
            studentId: STUDENT_ID,
            studentName: "Nguyễn Đức Minh",
            attemptNo: 1,
            answer: { type: "text", value: "The chart shows three countries." },
            manualScore: null,
            graderComment: null,
          },
        ],
      }),
    ),
    http.post(`${BASE}/teacher/attempts/:id/grade`, async ({ request }) => {
      graded.push(await request.json());
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 11,
        total: 14,
        pendingManual: 0,
      });
    }),
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <MemoryRouter>
        <GradeByQuestion
          assignmentId={ASSIGNMENT_ID}
          testTitle="Writing Task 1"
          initialQuestionId={ESSAY_ID}
          onExit={() => {}}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  const scale = await screen.findByRole("group", { name: "Band" });
  expect(screen.queryByText("This question is graded automatically.")).toBeNull();
  const save = screen.getByRole("button", { name: "Save" });
  expect(save).toHaveAttribute("aria-disabled", "true");
  await user.click(save);
  expect(screen.getByRole("alert")).toHaveTextContent("Pick a band first (keys 3–9).");
  expect(graded).toEqual([]);
  await user.click(within(scale).getByRole("button", { name: "Give band 8.0" }));
  expect(screen.queryByRole("alert")).toBeNull();
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() =>
    expect(graded).toEqual([
      { items: [{ questionId: ESSAY_ID, band: 8, comment: null }] },
    ]),
  );
});
