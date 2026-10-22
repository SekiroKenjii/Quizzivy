import { beforeEach, expect, it } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import { Toaster } from "@/components/ui/sonner";
import GradingPage from "@/features/attempts/pages/teacher/GradingPage";
import type { GradingQueueItem } from "@/features/attempts/api";
import type { AiScoreSuggestion } from "@/features/ai/api";
import { setLocale } from "@/lib/i18n";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/features/attempts/gradingMessages";
import { aiOn } from "../ai/support";
import { ASSIGNMENT_ID, ATTEMPT_ID, BASE, ESSAY_ID, STUDENT_ID } from "./fixtures";

const SCORE_PATH =
  "/teacher/ai/attempts/{attemptId}/answers/{questionId}/score-suggestion";
const TASK = "018f0000-0000-7000-8000-00000000cc01";
const COHERENCE = "018f0000-0000-7000-8000-00000000cc02";

const essay: GradingQueueItem = {
  attemptId: ATTEMPT_ID,
  questionId: ESSAY_ID,
  assignmentId: ASSIGNMENT_ID,
  assignmentTitle: "Writing Task 1 Practice",
  studentId: STUDENT_ID,
  studentName: "Võ Khánh Linh",
  questionNumber: 1,
  type: "essay",
  prompt: "Summarise the chart.",
  answer: { type: "text", value: "The bar chart illustrates the proportion." },
  points: 9,
  score: null,
  comment: null,
  rubric: [],
};

const rubric = [
  { id: TASK, ordinal: 0, label: "Task achievement" },
  { id: COHERENCE, ordinal: 1, label: "Coherence and cohesion" },
];

const withCriteria: AiScoreSuggestion = {
  score: 6.5,
  band: 6.5,
  criteria: [
    { criterionId: TASK, band: 6 },
    { criterionId: COHERENCE, band: 7 },
  ],
  confidence: "medium",
  reasons: [
    "Task achievement: clear overview, but few figures to support it.",
    "Coherence and cohesion: logical order and a good paraphrase of the task.",
  ],
  comment: "Strong overview. To reach band 7, add the key figures.",
  usage: { credits: 900, left: 18400 },
};

let queue: GradingQueueItem[];
let answer: AiScoreSuggestion;
let grades: unknown[];

beforeEach(async () => {
  await act(async () => setLocale("en"));
  aiOn();
  queue = [{ ...essay, rubric }];
  answer = withCriteria;
  grades = [];
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: queue,
        groups: [],
        answersRemaining: queue.length,
        studentsWaiting: queue.length,
      }),
    ),
    http.get(`${BASE}/teacher/attempts`, () =>
      contractJson("/teacher/attempts", "get", 200, {
        items: [],
        total: 0,
        page: 1,
        pageSize: 100,
      }),
    ),
    http.post(
      `${BASE}/teacher/ai/attempts/:attemptId/answers/:questionId/score-suggestion`,
      () => contractJson(SCORE_PATH, "post", 200, answer),
    ),
    http.post(`${BASE}/teacher/attempts/:id/grade`, async ({ request }) => {
      grades.push(await request.json());
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 6.5,
        total: 9,
        pendingManual: 0,
      });
    }),
  );
});

function mount() {
  const router = createMemoryRouter(
    [{ path: "/teacher/grading", element: <GradingPage /> }],
    { initialEntries: ["/teacher/grading"] },
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return userEvent.setup({ delay: null });
}

it("suggests a band for each criterion, and Use fills the scales and the comment without a write", async () => {
  const user = mount();
  const box = await screen.findByRole("region", { name: "AI suggestion" });
  expect(
    within(box).getByText(
      "Reads the essay and suggests a band for each criterion. You decide the bands.",
    ),
  ).toBeVisible();
  await user.click(within(box).getByRole("button", { name: "Suggest a score" }));
  expect(await within(box).findByText("Medium confidence")).toBeVisible();
  expect(within(box).getByText("6.5")).toBeVisible();
  expect(within(box).getByText("/ 9")).toBeVisible();
  expect(
    within(box).getByText(
      "Task achievement: clear overview, but few figures to support it.",
    ),
  ).toBeVisible();
  expect(
    within(box).getByText(
      "Coherence and cohesion: logical order and a good paraphrase of the task.",
    ),
  ).toBeVisible();

  await user.click(within(box).getByRole("button", { name: "Use score and comment" }));
  expect(
    await screen.findByText(
      "Band and comment filled in. Check them, then Save & next.",
    ),
  ).toBeVisible();
  const task = screen.getByRole("group", { name: "Task achievement" });
  const coherence = screen.getByRole("group", { name: "Coherence and cohesion" });
  expect(within(task).getByRole("button", { name: "Give band 6.0" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(
    within(coherence).getByRole("button", { name: "Give band 7.0" }),
  ).toHaveAttribute("aria-pressed", "true");
  expect(screen.getByLabelText(/^Comment/)).toHaveValue(
    "Strong overview. To reach band 7, add the key figures.",
  );
  expect(grades).toEqual([]);

  await user.click(screen.getByRole("button", { name: /Save & next/ }));
  await waitFor(() =>
    expect(grades).toEqual([
      {
        items: [
          {
            questionId: ESSAY_ID,
            criteria: [
              { criterionId: TASK, band: 6 },
              { criterionId: COHERENCE, band: 7 },
            ],
            comment: "Strong overview. To reach band 7, add the key figures.",
          },
        ],
      },
    ]),
  );
});

it("suggests one band for an essay without a rubric, and Save & next writes it", async () => {
  queue = [essay];
  answer = {
    score: 6,
    band: 6,
    confidence: "high",
    reasons: ["A clear overview."],
    comment: "Add figures.",
    usage: { credits: 700, left: 18000 },
  };
  const user = mount();
  const box = await screen.findByRole("region", { name: "AI suggestion" });
  expect(
    within(box).getByText("Reads the essay and suggests a band. You decide the band."),
  ).toBeVisible();
  await user.click(within(box).getByRole("button", { name: "Suggest a score" }));
  expect(await within(box).findByText("6.0")).toBeVisible();
  expect(within(box).getByText("/ 9")).toBeVisible();
  await user.click(within(box).getByRole("button", { name: "Use score and comment" }));
  expect(screen.getByRole("button", { name: "Give band 6.0" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(grades).toEqual([]);
  await user.click(screen.getByRole("button", { name: /Save & next/ }));
  await waitFor(() =>
    expect(grades).toEqual([
      { items: [{ questionId: ESSAY_ID, band: 6, comment: "Add figures." }] },
    ]),
  );
});

it("fills nothing from a suggestion that leaves a criterion without a band", async () => {
  answer = { ...withCriteria, criteria: [{ criterionId: TASK, band: 6 }] };
  const user = mount();
  const box = await screen.findByRole("region", { name: "AI suggestion" });
  await user.click(within(box).getByRole("button", { name: "Suggest a score" }));
  await user.click(
    await within(box).findByRole("button", { name: "Use score and comment" }),
  );
  const task = screen.getByRole("group", { name: "Task achievement" });
  expect(within(task).getByRole("button", { name: "Give band 6.0" })).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  expect(screen.getByLabelText(/^Comment/)).toHaveValue("");
  expect(grades).toEqual([]);
});
