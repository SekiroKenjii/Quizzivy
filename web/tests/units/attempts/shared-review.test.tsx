import { expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import AttemptReviewPage from "@/features/attempts/pages/AttemptReviewPage";
import { GradeByQuestion } from "@/features/attempts/components/GradeByQuestion";
import { previewGroup } from "@tests/support/groupPreview";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import {
  ASSIGNMENT_ID,
  ATTEMPT_ID,
  BASE,
  CHOICE_ID,
  ESSAY_ID,
  review,
} from "./fixtures";
import "@/lib/i18n";

const group = {
  ...previewGroup,
  questionIds: [CHOICE_ID, ESSAY_ID],
  stimuli: [
    {
      ...previewGroup.stimuli[0]!,
      gaps: [{ kind: "question" as const, gapId: "material", questionId: CHOICE_ID }],
    },
  ],
};
const shared = {
  groups: [group],
  transcripts: { [group.recordings[0]!.id]: "Teacher-only shared transcript." },
};

function renderPage(byQuestion: boolean) {
  const router = createMemoryRouter(
    [
      {
        path: "/admin/attempts/:id",
        element: byQuestion ? (
          <GradeByQuestion
            assignmentId={ASSIGNMENT_ID}
            initialQuestionId={ESSAY_ID}
            testTitle="Shared test"
            onExit={() => undefined}
          />
        ) : (
          <AttemptReviewPage />
        ),
      },
    ],
    { initialEntries: [`/admin/attempts/${ATTEMPT_ID}`] },
  );
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

it("shows a teacher the group's material, transcript and attempt allowance while changing members", async () => {
  server.use(
    http.get(`${BASE}/admin/attempts/${ATTEMPT_ID}`, () =>
      contractJson("/admin/attempts/{id}", "get", 200, {
        ...review(),
        sharedContext: { ...shared, audioPlays: { [group.recordings[0]!.id]: 4 } },
      }),
    ),
    http.get(`${BASE}/admin/attempts/${ATTEMPT_ID}/events`, () =>
      contractJson("/admin/attempts/{id}/events", "get", 200, {
        startedAt: review().attempt.startedAt,
        events: [],
        summary: review().integrity,
      }),
    ),
  );
  renderPage(false);
  const user = userEvent.setup();
  await screen.findByText(group.title);
  expect(
    screen.getByText("Đã nghe 4 lượt · Quy định 2 lượt cho cả nhóm"),
  ).toBeVisible();
  await user.click(screen.getByText("Nội dung bài nghe"));
  expect(screen.getByText("Teacher-only shared transcript.")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Ô A — chuyển đến câu 1" }));
  expect(screen.getByText("Thủ đô của Việt Nam?")).toBeVisible();
  expect(screen.getByText(group.title)).toBeVisible();
});

it("keeps shared context in anonymous question grading without inventing a cross-attempt play total", async () => {
  server.use(
    http.get(`${BASE}/admin/assignments/${ASSIGNMENT_ID}/answers`, () =>
      contractJson("/admin/assignments/{id}/answers", "get", 200, {
        question: review().questions[1],
        questionNumber: 2,
        questionCount: 2,
        manualQuestionIds: [ESSAY_ID],
        items: [],
        sharedContext: shared,
      }),
    ),
  );
  renderPage(true);
  await screen.findByText(group.title);
  expect(screen.getByText("Ngữ liệu dùng chung · Câu 1–2")).toBeVisible();
  expect(screen.queryByText(/^Đã nghe/)).not.toBeInTheDocument();
  await userEvent.setup().click(screen.getByText("Nội dung bài nghe"));
  expect(screen.getByText("Teacher-only shared transcript.")).toBeVisible();
});
