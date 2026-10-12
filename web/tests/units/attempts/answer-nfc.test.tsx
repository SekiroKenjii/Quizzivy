import { expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AttemptReviewPage from "@/features/attempts/pages/teacher/AttemptReviewPage";
import { GradeByQuestion } from "@/features/attempts/components/GradeByQuestion";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { ASSIGNMENT_ID, ATTEMPT_ID, BASE, ESSAY_ID, review } from "./fixtures";
import "@/lib/i18n";

const composed = "Quê hương tôi ở Huế, bên dòng sông Hương.";
const decomposed = composed.normalize("NFD");

function paperWithDecomposedAnswer() {
  const paper = review();
  return {
    ...paper,
    answers: {
      ...paper.answers,
      [ESSAY_ID]: {
        ...paper.answers[ESSAY_ID]!,
        answer: { type: "text" as const, value: decomposed },
      },
    },
  };
}

function renderAt(element: React.ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter([{ path: "/teacher/attempts/:id", element }], {
    initialEntries: [`/teacher/attempts/${ATTEMPT_ID}`],
  });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

it("draws a student's decomposed answer composed in the attempt review", async () => {
  const paper = paperWithDecomposedAnswer();
  server.use(
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}`, () =>
      contractJson("/teacher/attempts/{id}", "get", 200, paper),
    ),
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}/events`, () =>
      contractJson("/teacher/attempts/{id}/events", "get", 200, {
        startedAt: paper.attempt.startedAt,
        events: [],
        summary: paper.integrity,
      }),
    ),
  );
  renderAt(<AttemptReviewPage />);
  expect(await screen.findByText(composed)).toBeVisible();
  expect(document.body.textContent).not.toContain(decomposed);
});

it("draws a student's decomposed answer composed when grading by question", async () => {
  const paper = review();
  server.use(
    http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/answers`, () =>
      contractJson("/teacher/assignments/{id}/answers", "get", 200, {
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
            answer: { type: "text", value: decomposed },
            manualScore: null,
            graderComment: null,
          },
        ],
      }),
    ),
  );
  renderAt(
    <GradeByQuestion
      assignmentId={ASSIGNMENT_ID}
      initialQuestionId={ESSAY_ID}
      testTitle="Bài kiểm tra"
      onExit={() => undefined}
    />,
  );
  expect(await screen.findByText(composed)).toBeVisible();
  expect(document.body.textContent).not.toContain(decomposed);
});
