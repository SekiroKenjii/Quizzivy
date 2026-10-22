import { beforeEach, expect, it } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { focusManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
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
let hold: Promise<void> | null;

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
      await hold;
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
  hold = null;
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
  // Each paper's scale is named after its row, so rows can be told apart.
  const scale = await screen.findByRole("group", { name: "Student 01 Band" });
  expect(screen.queryByText("This question is graded automatically.")).toBeNull();
  // The band keys' hint shows once for the list, not under every row.
  expect(
    screen.getAllByText("3–9 sets a band, Shift + digit the half above"),
  ).toHaveLength(1);
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

function mountReview() {
  // As the app's client: a read stays fresh for 30 s, so only "always" re-reads on focus.
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 30_000 } },
  });
  const router = createMemoryRouter(
    [
      { path: "/teacher/attempts/:id", element: <AttemptReviewPage /> },
      { path: "/teacher", element: <p>Overview</p> },
    ],
    { initialEntries: [`/teacher/attempts/${ATTEMPT_ID}`] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup({ delay: null }), router, client };
}

it("asks before leaving the paper review with an unsaved band", async () => {
  serve(essayPaper(false));
  const { user, router } = mountReview();
  const scale = await screen.findByRole("group", { name: "Band" });
  await act(async () => void router.navigate("/teacher"));
  expect(await screen.findByText("Overview")).toBeVisible();
  await act(async () => void router.navigate(`/teacher/attempts/${ATTEMPT_ID}`));
  await user.click(
    within(await screen.findByRole("group", { name: "Band" })).getByRole("button", {
      name: "Give band 6.0",
    }),
  );
  expect(scale).not.toBeInTheDocument();
  await act(async () => void router.navigate("/teacher"));
  const dialog = await screen.findByRole("dialog");
  expect(dialog).toHaveTextContent("Leave with unsaved marks?");
  await user.click(within(dialog).getByRole("button", { name: "Stay" }));
  expect(screen.queryByText("Overview")).toBeNull();
  await act(async () => void router.navigate("/teacher"));
  await user.click(
    within(await screen.findByRole("dialog")).getByRole("button", { name: "Leave" }),
  );
  expect(await screen.findByText("Overview")).toBeVisible();
  expect(graded).toEqual([]);
});

it("takes a grade saved elsewhere while untouched, and names it once marks are given", async () => {
  const paper = essayPaper(false);
  serve(paper);
  const { user, client } = mountReview();
  await screen.findByRole("group", { name: "Band" });
  // Stay on the essay once it is graded, rather than the next pending one.
  await user.click(screen.getByRole("button", { name: /^Question 2,/ }));
  // Another tab saves band 7.0 and this tab reads the paper again.
  paper.answers[ESSAY_ID] = { ...paper.answers[ESSAY_ID]!, manualScore: 7, band: 7 };
  await act(async () => {
    await client.refetchQueries();
  });
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Give band 7.0" })).toHaveAttribute(
      "aria-pressed",
      "true",
    ),
  );
  expect(screen.queryByText(/graded elsewhere/)).toBeNull();
  // With a band of its own given, a later grade elsewhere is named, not taken.
  await user.click(screen.getByRole("button", { name: "Give band 5.0" }));
  paper.answers[ESSAY_ID] = { ...paper.answers[ESSAY_ID]!, manualScore: 8, band: 8 };
  await act(async () => {
    await client.refetchQueries();
  });
  expect(await screen.findByText(/graded elsewhere/)).toBeVisible();
  expect(screen.getByRole("button", { name: "Give band 5.0" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await user.click(screen.getByRole("button", { name: "Show that grade" }));
  expect(screen.getByRole("button", { name: "Give band 8.0" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(screen.queryByText(/graded elsewhere/)).toBeNull();
});

it("puts focus back on the band when a refused save left it on the page", async () => {
  refuse = "GRADE_NEEDS_BAND";
  let answer!: () => void;
  hold = new Promise((resolve) => (answer = resolve));
  serve(essayPaper(false));
  const { user } = mountReview();
  await user.click(await screen.findByRole("button", { name: "Give band 5.0" }));
  await user.click(screen.getByRole("button", { name: "Save & next" }));
  // A browser drops focus from the button the save pauses.
  await waitFor(() => expect(graded).toHaveLength(1));
  // jsdom cannot blur the paused button, so park focus on the page by hand.
  const comment = screen.getByLabelText(/^Comment/);
  comment.focus();
  comment.blur();
  expect(document.activeElement).toBe(document.body);
  answer();
  await screen.findByRole("alert");
  await waitFor(() =>
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "Give band 5.0" }),
    ),
  );
});

it("asks before leaving G-04 with an unsaved band on a paper", async () => {
  serve(essayPaper(false));
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
  );
  const { user, router } = mountReview();
  await user.click(await screen.findByRole("button", { name: "Grade by question" }));
  const scale = await screen.findByRole("group", { name: "Student 01 Band" });
  await user.click(within(scale).getByRole("button", { name: "Give band 6.5" }));
  await act(async () => void router.navigate("/teacher"));
  expect(await screen.findByRole("dialog")).toHaveTextContent(
    "Leave with unsaved marks?",
  );
  expect(graded).toEqual([]);
});

it("adds the band to the review header only for a banded paper", async () => {
  const points = review();
  points.attempt.score = { earned: 27, total: 30, pendingManual: 0 };
  serve(points);
  mountReview();
  const heading = await screen.findByRole("heading", { level: 1 });
  const head = heading.closest<HTMLElement>('[data-slot="page-head"]')!;
  await waitFor(() => expect(head).toHaveTextContent("27/30"));
  expect(head).not.toHaveTextContent(/band/i);
});

it("writes the band in the review header for a banded paper", async () => {
  const banded = essayPaper(false);
  banded.attempt.score = { earned: 6, total: 9, pendingManual: 0, band: 6 };
  serve(banded);
  mountReview();
  const heading = await screen.findByRole("heading", { level: 1 });
  const head = heading.closest<HTMLElement>('[data-slot="page-head"]')!;
  await waitFor(() => expect(head).toHaveTextContent("· Band 6.0"));
});

it("pauses Finish grading on the paper review while an answer waits, and sends nothing", async () => {
  const finished: string[] = [];
  serve(essayPaper(false));
  server.use(
    http.post(`${BASE}/teacher/attempts/:id/finish-grading`, ({ params }) => {
      finished.push(String(params.id));
      return new HttpResponse(null, { status: 204 });
    }),
  );
  const { user } = mountReview();
  const finish = await screen.findByRole("button", { name: "Finish grading" });
  expect(finish).toHaveAttribute("aria-disabled", "true");
  expect(finish).toBeEnabled();
  await user.click(finish);
  expect(finished).toEqual([]);
});

it("names a banded essay graded on a band, not partly correct", async () => {
  const banded = essayPaper(false);
  banded.answers[ESSAY_ID] = { ...banded.answers[ESSAY_ID]!, manualScore: 6, band: 6 };
  serve(banded);
  mountReview();
  expect(
    await screen.findByRole("button", { name: "Question 2, Graded on a band" }),
  ).toBeInTheDocument();
  expect(screen.queryByText("Partly correct")).toBeNull();
});

function pointsRowHandlers(row: { manualScore: number | null }) {
  const question = review().questions[1]!;
  server.use(
    http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/answers`, () =>
      contractJson("/teacher/assignments/{id}/answers", "get", 200, {
        question,
        questionNumber: 2,
        questionCount: 2,
        manualQuestionIds: [ESSAY_ID],
        items: [
          {
            attemptId: ATTEMPT_ID,
            studentId: STUDENT_ID,
            studentName: "Nguyễn Đức Minh",
            attemptNo: 1,
            answer: { type: "text", value: "I usually wake up at six o'clock." },
            manualScore: row.manualScore,
            graderComment: null,
          },
        ],
      }),
    ),
    http.post(`${BASE}/teacher/attempts/:id/grade`, async ({ request }) => {
      const body = (await request.json()) as { items: { points: number }[] };
      graded.push(body);
      await hold;
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
      row.manualScore = body.items[0]!.points;
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 6,
        total: 10,
        pendingManual: 0,
      });
    }),
  );
}

function servePointsRows(row: { manualScore: number | null }) {
  pointsRowHandlers(row);
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <GradeByQuestion
          assignmentId={ASSIGNMENT_ID}
          testTitle="Daily routines"
          initialQuestionId={ESSAY_ID}
          onExit={() => {}}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return { user: userEvent.setup({ delay: null }), client };
}

it("reads a saved 0.50 on a G-04 points row as the 0.5 the server keeps", async () => {
  const row = { manualScore: null as number | null };
  const { user, client } = servePointsRows(row);
  const box = await screen.findByLabelText("Points for Student 01");
  // user-event drops the trailing zero a teacher can type, so set it whole.
  fireEvent.change(box, { target: { value: "0.50" } });
  expect(box).toHaveValue(0.5);
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(row.manualScore).toBe(0.5));
  await act(async () => {
    await client.refetchQueries();
  });
  await screen.findByText("Graded");
  expect(screen.queryByText(/graded elsewhere/)).toBeNull();
});

it("keeps a G-04 points edit that came back to its start when a grade lands elsewhere", async () => {
  const row = { manualScore: 0.5 as number | null };
  const { user, client } = servePointsRows(row);
  // Every paper is graded, so the board shows names.
  const box = await screen.findByLabelText("Points for Nguyễn Đức Minh");
  expect(box).toHaveValue(0.5);
  await user.clear(box);
  await user.type(box, "0.5");
  // Another tab saves 1.5 while this box reads its starting value again.
  row.manualScore = 1.5;
  await act(async () => {
    await client.refetchQueries();
  });
  expect(await screen.findByText(/graded elsewhere/)).toBeVisible();
  expect(box).toHaveValue(0.5);
  await user.click(screen.getByRole("button", { name: "Show that grade" }));
  expect(box).toHaveValue(1.5);
  expect(screen.queryByText(/graded elsewhere/)).toBeNull();
});

it("asks before the rail or Skip drops an unsaved band on the paper review", async () => {
  serve(essayPaper(false));
  const { user } = mountReview();
  await user.click(await screen.findByRole("button", { name: "Give band 6.0" }));
  await user.click(screen.getByRole("button", { name: "Skip" }));
  const dialog = await screen.findByRole("dialog");
  expect(dialog).toHaveTextContent("Leave with unsaved marks?");
  // The safe answer has focus.
  expect(within(dialog).getByRole("button", { name: "Stay" })).toHaveFocus();
  await user.click(within(dialog).getByRole("button", { name: "Stay" }));
  expect(screen.getByRole("button", { name: "Give band 6.0" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await user.click(screen.getByRole("button", { name: /^Question 1,/ }));
  await user.click(
    within(await screen.findByRole("dialog")).getByRole("button", { name: "Leave" }),
  );
  await waitFor(() =>
    expect(screen.queryByRole("button", { name: "Give band 6.0" })).toBeNull(),
  );
  expect(graded).toEqual([]);
});

it("moves along the rail at once when nothing is unsaved", async () => {
  serve(essayPaper(false));
  const { user } = mountReview();
  await screen.findByRole("button", { name: "Give band 6.0" });
  await user.click(screen.getByRole("button", { name: /^Question 1,/ }));
  expect(screen.queryByRole("dialog")).toBeNull();
  expect(screen.queryByRole("button", { name: "Give band 6.0" })).toBeNull();
});

it("asks before Grade per paper drops an unsaved points mark in G-04", async () => {
  serve(review());
  pointsRowHandlers({ manualScore: null });
  const { user, router } = mountReview();
  await user.click(await screen.findByRole("button", { name: "Grade by question" }));
  await user.type(await screen.findByLabelText("Points for Student 01"), "3");
  // A points row reports its unsaved mark to the page's guard.
  await act(async () => void router.navigate("/teacher"));
  await user.click(
    within(await screen.findByRole("dialog")).getByRole("button", { name: "Stay" }),
  );
  await user.click(screen.getByRole("button", { name: "Grade per paper" }));
  await user.click(
    within(await screen.findByRole("dialog")).getByRole("button", { name: "Stay" }),
  );
  expect(screen.getByLabelText("Points for Student 01")).toHaveValue(3);
  await user.click(screen.getByRole("button", { name: "Grade per paper" }));
  await user.click(
    within(await screen.findByRole("dialog")).getByRole("button", { name: "Leave" }),
  );
  expect(
    await screen.findByRole("button", { name: "Grade by question" }),
  ).toBeVisible();
  expect(graded).toEqual([]);
});

it("names a refused save on a G-04 points row and puts focus back on its box", async () => {
  refuse = "GRADE_NEEDS_POINTS";
  let answer!: () => void;
  hold = new Promise((resolve) => (answer = resolve));
  const { user } = servePointsRows({ manualScore: null });
  const box = await screen.findByLabelText("Points for Student 01");
  await user.type(box, "3");
  await user.click(screen.getByRole("button", { name: "Save" }));
  await waitFor(() => expect(graded).toHaveLength(1));
  // jsdom cannot blur the paused button, so park focus on the page by hand.
  box.focus();
  box.blur();
  expect(document.activeElement).toBe(document.body);

  answer();
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Choose a score for this answer.",
  );
  await waitFor(() => expect(box).toHaveFocus());
});

it("puts focus back on the points box when a refused save left it on the page", async () => {
  refuse = "GRADE_NEEDS_POINTS";
  let answer!: () => void;
  hold = new Promise((resolve) => (answer = resolve));
  serve(review());
  const { user } = mountReview();
  const box = await screen.findByLabelText("Points");
  await user.type(box, "3");
  await user.click(screen.getByRole("button", { name: "Save & next" }));
  await waitFor(() => expect(graded).toHaveLength(1));
  // jsdom cannot blur the paused button, so park focus on the page by hand.
  box.focus();
  box.blur();
  expect(document.activeElement).toBe(document.body);
  answer();
  await screen.findByRole("alert");
  await waitFor(() => expect(box).toHaveFocus());
});

it("reads the paper again when the window regains focus", async () => {
  let reads = 0;
  const paper = essayPaper(false);
  server.use(
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}`, () => {
      reads += 1;
      return contractJson("/teacher/attempts/{id}", "get", 200, paper);
    }),
  );
  mountReview();
  await screen.findByRole("group", { name: "Band" });
  expect(reads).toBe(1);
  act(() => focusManager.setFocused(false));
  act(() => focusManager.setFocused(true));
  await waitFor(() => expect(reads).toBe(2));
  act(() => focusManager.setFocused());
});

it("reads G-04's answers again when the window regains focus", async () => {
  let reads = 0;
  serve(review());
  pointsRowHandlers({ manualScore: null });
  const question = review().questions[1]!;
  server.use(
    http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/answers`, () => {
      reads += 1;
      return contractJson("/teacher/assignments/{id}/answers", "get", 200, {
        question,
        questionNumber: 2,
        questionCount: 2,
        manualQuestionIds: [ESSAY_ID],
        items: [],
      });
    }),
  );
  const { user } = mountReview();
  await user.click(await screen.findByRole("button", { name: "Grade by question" }));
  await waitFor(() => expect(reads).toBe(1));
  act(() => focusManager.setFocused(false));
  act(() => focusManager.setFocused(true));
  await waitFor(() => expect(reads).toBe(2));
  act(() => focusManager.setFocused());
});

it("writes a fractional question's points in the reader's form", async () => {
  await act(async () => setLocale("vi"));
  const paper = review();
  paper.questions[1]!.points = 1.5;
  serve(paper);
  mountReview();
  expect(await screen.findByText("Câu 2 · Trả lời ngắn · 1,5 điểm")).toBeVisible();
});

const OTHER_ATTEMPT = "018f0000-0000-7000-8000-0000000000f2";

function twoRowHandlers(points = 5) {
  const question = { ...review().questions[1]!, points };
  const row = (attemptId: string, studentName: string) => ({
    attemptId,
    studentId: STUDENT_ID,
    studentName,
    attemptNo: 1,
    answer: { type: "text" as const, value: "I usually wake up at six o'clock." },
    manualScore: null,
    graderComment: null,
  });
  server.use(
    http.get(`${BASE}/teacher/assignments/${ASSIGNMENT_ID}/answers`, () =>
      contractJson("/teacher/assignments/{id}/answers", "get", 200, {
        question,
        questionNumber: 1,
        questionCount: 2,
        manualQuestionIds: [ESSAY_ID, TASK],
        items: [row(ATTEMPT_ID, "Nguyễn Đức Minh"), row(OTHER_ATTEMPT, "Trần Thu Hà")],
      }),
    ),
    http.post(`${BASE}/teacher/attempts/:id/grade`, async ({ request }) => {
      graded.push(await request.json());
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 6,
        total: 10,
        pendingManual: 0,
      });
    }),
  );
}

it("asks before the Integrity tab or a group link drops an unsaved band", async () => {
  serve(essayPaper(false));
  const { user } = mountReview();
  await user.click(await screen.findByRole("button", { name: "Give band 6.0" }));
  await user.click(screen.getByRole("tab", { name: "Integrity" }));
  await user.click(
    within(await screen.findByRole("dialog")).getByRole("button", { name: "Stay" }),
  );
  expect(screen.getByRole("tab", { name: "Paper" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  expect(screen.getByRole("button", { name: "Give band 6.0" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  // The question already shown is no move, so its rail dot does not ask.
  await user.click(screen.getByRole("button", { name: /^Question 2,/ }));
  expect(screen.queryByRole("dialog")).toBeNull();
});

it("asks before Grade by question drops an unsaved band on the paper review", async () => {
  serve(essayPaper(false));
  twoRowHandlers();
  const { user } = mountReview();
  await user.click(await screen.findByRole("button", { name: "Give band 6.0" }));
  await user.click(screen.getByRole("button", { name: "Grade by question" }));
  await user.click(
    within(await screen.findByRole("dialog")).getByRole("button", { name: "Stay" }),
  );
  expect(screen.queryByLabelText("Points for Student 01")).toBeNull();
});

it("asks before Next question, or a save and next, drops another row's mark in G-04", async () => {
  serve(review());
  twoRowHandlers();
  const { user } = mountReview();
  await user.click(await screen.findByRole("button", { name: "Grade by question" }));
  const first = await screen.findByLabelText("Points for Student 01");
  await user.type(screen.getByLabelText("Points for Student 02"), "2");
  await user.click(screen.getByRole("button", { name: /^Next question/ }));
  await user.click(
    within(await screen.findByRole("dialog")).getByRole("button", { name: "Stay" }),
  );
  // Saving row 1 and moving on would drop row 2's mark, so it asks too.
  await user.type(first, "4");
  await user.keyboard("{Control>}{Enter}{/Control}");
  await waitFor(() => expect(graded).toHaveLength(1));
  const dialog = await screen.findByRole("dialog");
  await user.click(within(dialog).getByRole("button", { name: "Stay" }));
  expect(screen.getByLabelText("Points for Student 02")).toHaveValue(2);
});

it("moves on after a save and next when no other row holds marks", async () => {
  serve(review());
  twoRowHandlers();
  let questions = 0;
  server.events.on("request:start", ({ request }) => {
    if (request.url.includes("/answers")) questions += 1;
  });
  const { user } = mountReview();
  await user.click(await screen.findByRole("button", { name: "Grade by question" }));
  await user.type(await screen.findByLabelText("Points for Student 01"), "4");
  await user.keyboard("{Control>}{Enter}{/Control}");
  await waitFor(() => expect(graded).toHaveLength(1));
  await waitFor(() => expect(questions).toBeGreaterThanOrEqual(3));
  expect(screen.queryByRole("dialog")).toBeNull();
  server.events.removeAllListeners();
});

it("marks a refused G-04 points box invalid and ties the message to it", async () => {
  refuse = "GRADE_NEEDS_POINTS";
  const { user } = servePointsRows({ manualScore: null });
  const box = await screen.findByLabelText("Points for Student 01");
  await user.type(box, "3");
  await user.click(screen.getByRole("button", { name: "Save" }));
  const alert = await screen.findByRole("alert");
  expect(box).toHaveAttribute("aria-invalid", "true");
  expect(box).toHaveAccessibleDescription(alert.textContent!);
});

it("writes G-04's question points in the reader's form, singular for one", async () => {
  serve(review());
  twoRowHandlers(1.5);
  await act(async () => setLocale("vi"));
  const { user } = mountReview();
  await user.click(await screen.findByRole("button", { name: "Chấm theo câu hỏi" }));
  expect(await screen.findByText("Câu 1 · Trả lời ngắn · 1,5 điểm")).toBeVisible();
});

it("writes one point in the singular in G-04", async () => {
  twoRowHandlers(1);
  servePointsRowsShell();
  expect(await screen.findByText("Question 1 · Short answer · 1 point")).toBeVisible();
});

it("pauses Next question on the last question instead of dimming it", async () => {
  pointsRowHandlers({ manualScore: null });
  servePointsRowsShell();
  const next = await screen.findByRole("button", { name: /^Next question/ });
  expect(next).toHaveAttribute("aria-disabled", "true");
  expect(next).toBeEnabled();
});

function servePointsRowsShell() {
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <MemoryRouter>
        <GradeByQuestion
          assignmentId={ASSIGNMENT_ID}
          testTitle="Daily routines"
          initialQuestionId={ESSAY_ID}
          onExit={() => {}}
        />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

it("lets a move asked during a save go on once the save lands", async () => {
  let answer!: () => void;
  hold = new Promise((resolve) => (answer = resolve));
  const paper = essayPaper(false);
  serve(paper);
  const { user } = mountReview();
  await user.click(await screen.findByRole("button", { name: "Give band 6.0" }));
  await user.click(screen.getByRole("button", { name: "Save & next" }));
  await waitFor(() => expect(graded).toHaveLength(1));
  await user.click(screen.getByRole("button", { name: /^Question 1,/ }));
  expect(await screen.findByRole("dialog")).toHaveTextContent(
    "Leave with unsaved marks?",
  );
  paper.answers[ESSAY_ID] = { ...paper.answers[ESSAY_ID]!, manualScore: 6, band: 6 };
  answer();
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  expect(screen.getByRole("button", { name: /^Question 1,/ })).toHaveAttribute(
    "aria-current",
    "true",
  );
});

it("lets a navigation asked during a save go on once the save lands", async () => {
  let answer!: () => void;
  hold = new Promise((resolve) => (answer = resolve));
  const paper = essayPaper(false);
  serve(paper);
  const { user, router } = mountReview();
  await user.click(await screen.findByRole("button", { name: "Give band 6.0" }));
  await user.click(screen.getByRole("button", { name: "Save & next" }));
  await waitFor(() => expect(graded).toHaveLength(1));
  await act(async () => void router.navigate("/teacher"));
  expect(await screen.findByRole("dialog")).toBeVisible();
  paper.answers[ESSAY_ID] = { ...paper.answers[ESSAY_ID]!, manualScore: 6, band: 6 };
  answer();
  expect(await screen.findByText("Overview")).toBeVisible();
});
