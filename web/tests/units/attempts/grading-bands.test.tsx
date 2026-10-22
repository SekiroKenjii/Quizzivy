import { beforeEach, expect, it } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import GradingPage from "@/features/attempts/pages/teacher/GradingPage";
import type { GradingQueueItem } from "@/features/attempts/api";
import { setLocale } from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { ASSIGNMENT_ID, ATTEMPT_ID, BASE, ESSAY_ID, STUDENT_ID } from "./fixtures";

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

let queue: GradingQueueItem[];
let grades: unknown[];
let refusal: { code: string; status: number } | null;

beforeEach(async () => {
  await act(async () => setLocale("en"));
  useAuthStore.getState().setSession("token", teacherUser);
  queue = [essay];
  grades = [];
  refusal = null;
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: queue,
        groups: queue.map((item) => ({
          key: item.studentId,
          kind: "student",
          label: item.studentName,
          sub: item.assignmentTitle,
          remaining: 1,
        })),
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
    http.post(`${BASE}/teacher/attempts/:id/grade`, async ({ request }) => {
      grades.push(await request.json());
      if (refusal)
        return HttpResponse.json(
          {
            error: {
              code: refusal.code,
              message: "Refused",
              requestId: "req-grade-1",
              details: { questionIds: [ESSAY_ID] },
            },
          },
          { status: refusal.status },
        );
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 6.5,
        total: 9,
        pendingManual: 1,
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
    </QueryClientProvider>,
  );
}

it("draws the thirteen bands, 3 to 9 in half steps, each with its key", async () => {
  mount();
  const scale = await screen.findByRole("group", { name: "Band" });
  const buttons = within(scale).getAllByRole("button");
  expect(buttons.map((button) => button.getAttribute("aria-label"))).toEqual(
    [3, 3.5, 4, 4.5, 5, 5.5, 6, 6.5, 7, 7.5, 8, 8.5, 9].map(
      (band) => `Give band ${band.toFixed(1)}`,
    ),
  );
  expect(buttons[6]).toHaveTextContent("6.06");
  expect(buttons[7]).toHaveTextContent("6.56");
  expect(screen.queryByRole("button", { name: /Give \d+ points?/ })).toBeNull();
  expect(
    screen.getByText("3–9 sets a band, Shift + digit the half above"),
  ).toBeVisible();
});

it("saves a band without a rubric at once, from a click or from Shift and the digit", async () => {
  const user = userEvent.setup({ delay: null });
  mount();
  await user.click(await screen.findByRole("button", { name: "Give band 7.0" }));
  await waitFor(() => expect(grades).toHaveLength(1));
  expect(grades[0]).toEqual({
    items: [{ questionId: ESSAY_ID, band: 7, comment: null }],
  });
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Give band 7.0" })).toHaveAttribute(
      "aria-pressed",
      "true",
    ),
  );
  (document.activeElement as HTMLElement | null)?.blur();
  await user.keyboard("{Shift>}6{/Shift}");
  await waitFor(() => expect(grades).toHaveLength(2));
  expect(grades[1]).toEqual({
    items: [{ questionId: ESSAY_ID, band: 6.5, comment: null }],
  });
});

it("leaves the position keys to points questions: a digit 1 or 2 sets no score on an essay", async () => {
  // Worth 2 points, a points question would take 1 as "0 points".
  queue = [{ ...essay, points: 2 }];
  const user = userEvent.setup({ delay: null });
  mount();
  await screen.findByRole("group", { name: "Band" });
  await user.keyboard("12");
  expect(grades).toEqual([]);
  expect(
    screen
      .getAllByRole("button", { pressed: true })
      .filter((button) => button.dataset["score"]),
  ).toEqual([]);
});

it("marks a rubric one criterion at a time, derives the band live, and saves only when complete", async () => {
  queue = [{ ...essay, rubric }];
  const user = userEvent.setup({ delay: null });
  mount();
  const task = await screen.findByRole("group", { name: "Task achievement" });
  const coherence = screen.getByRole("group", { name: "Coherence and cohesion" });
  const next = screen.getByRole("button", { name: /Save & next/ });
  expect(next).toBeDisabled();
  expect(screen.getByText("Overall band: give every criterion a band")).toBeVisible();

  await user.click(within(task).getByRole("button", { name: "Give band 6.0" }));
  expect(grades).toEqual([]);
  expect(next).toBeDisabled();
  // A key in a scale sets that scale and moves on to the next criterion.
  await user.keyboard("{Shift>}6{/Shift}");
  expect(within(task).getByRole("button", { name: "Give band 6.5" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(coherence).toContainElement(document.activeElement as HTMLElement);
  await user.keyboard("7");
  await waitFor(() => expect(grades).toHaveLength(1));
  expect(grades[0]).toEqual({
    items: [
      {
        questionId: ESSAY_ID,
        criteria: [
          { criterionId: TASK, band: 6.5 },
          { criterionId: COHERENCE, band: 7 },
        ],
        comment: null,
      },
    ],
  });
  expect(screen.getByText("Overall band: 7.0")).toBeVisible();
  await waitFor(() => expect(next).toBeEnabled());
});

it("sets the first criterion without a band when a key is pressed outside the scales", async () => {
  queue = [{ ...essay, rubric }];
  const user = userEvent.setup({ delay: null });
  mount();
  await screen.findByRole("group", { name: "Task achievement" });
  await user.keyboard("5");
  expect(
    within(screen.getByRole("group", { name: "Task achievement" })).getByRole(
      "button",
      {
        name: "Give band 5.0",
      },
    ),
  ).toHaveAttribute("aria-pressed", "true");
  expect(grades).toEqual([]);
});

it("shows a refused grade beside the scales, in the reader's language", async () => {
  refusal = { code: "GRADE_CRITERIA_INCOMPLETE", status: 422 };
  queue = [{ ...essay, rubric }];
  const user = userEvent.setup({ delay: null });
  mount();
  const task = await screen.findByRole("group", { name: "Task achievement" });
  await user.click(within(task).getByRole("button", { name: "Give band 6.0" }));
  await user.click(
    within(screen.getByRole("group", { name: "Coherence and cohesion" })).getByRole(
      "button",
      { name: "Give band 6.0" },
    ),
  );
  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent(
    "Give every criterion a band. If the rubric looks different, reload the page.",
  );
  expect(
    screen.getByRole("group", { name: "Task achievement" }).closest("fieldset"),
  ).toContainElement(alert);
});

it("names the bands in Vietnamese first", async () => {
  await act(async () => setLocale("vi"));
  mount();
  expect(await screen.findByRole("button", { name: "Cho band 6,5" })).toBeVisible();
  expect(
    screen.getByText("Phím 3–9 chọn band, Shift + phím cộng thêm nửa band"),
  ).toBeVisible();
});
