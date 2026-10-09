import { StrictMode } from "react";
import { beforeEach, expect, it } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { focusManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import GradingPage from "@/features/attempts/pages/teacher/GradingPage";
import type { GradingQueueItem, AttemptReview } from "@/features/attempts/api";
import { gradingItemKey } from "@/features/attempts/pages/teacher/gradingRecovery";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import { BASE, ASSIGNMENT_ID, review } from "../../units/attempts/fixtures";

const NAM = "018f0000-0000-7000-8000-0000000001e1";
const LAN = "018f0000-0000-7000-8000-0000000001e2";
const NAM_PAPER = "018f0000-0000-7000-8000-0000000001a1";
const LAN_PAPER = "018f0000-0000-7000-8000-0000000001a2";
const Q1 = "018f0000-0000-7000-8000-0000000001b1";
const Q2 = "018f0000-0000-7000-8000-0000000001b2";

function answer(
  attemptId: string,
  studentId: string,
  studentName: string,
  questionId: string,
  questionNumber: number,
  points: number,
): GradingQueueItem {
  return {
    attemptId,
    questionId,
    assignmentId: ASSIGNMENT_ID,
    assignmentTitle: "Unit 5",
    studentId,
    studentName,
    questionNumber,
    type: "short_answer",
    prompt: `${studentName.split(" ").at(-1)} câu ${questionNumber}`,
    answer: { type: "text", value: `Bài của ${studentName}` },
    points,
    score: null,
    comment: null,
  };
}

let pending: GradingQueueItem[];
let grades: { attemptId: string; questionId: string; points: number }[];
let elsewhere: Record<string, number>;
let queueReads: number;

function byStudent() {
  return [
    answer(NAM_PAPER, NAM, "Trần Văn Nam", Q1, 1, 1),
    answer(NAM_PAPER, NAM, "Trần Văn Nam", Q2, 2, 1),
    answer(LAN_PAPER, LAN, "Lê Thị Lan", Q1, 1, 1),
  ];
}

function byQuestion() {
  return [
    answer(NAM_PAPER, NAM, "Trần Văn Nam", Q1, 1, 9),
    answer(LAN_PAPER, LAN, "Lê Thị Lan", Q1, 1, 9),
    answer(NAM_PAPER, NAM, "Trần Văn Nam", Q2, 2, 4),
  ];
}

function left(attemptId: string) {
  return pending.filter((item) => item.attemptId === attemptId).length;
}

function groupsOf(mode: string) {
  const groups = new Map<
    string,
    { key: string; kind: string; label: string; sub: string; remaining: number }
  >();
  for (const item of pending) {
    const key =
      mode === "question" ? `${item.assignmentId}:${item.questionId}` : item.studentId;
    const group = groups.get(key) ?? {
      key,
      kind: mode,
      label: mode === "question" ? String(item.questionNumber) : item.studentName,
      sub: item.assignmentTitle,
      remaining: 0,
    };
    group.remaining += 1;
    groups.set(key, group);
  }
  return [...groups.values()];
}

function mount(entry: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 30_000 } },
  });
  const router = createMemoryRouter(
    [{ path: "/teacher/grading", element: <GradingPage /> }],
    { initialEntries: [entry] },
  );
  render(
    <StrictMode>
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </StrictMode>,
  );
  return userEvent.setup();
}

function serve(items: GradingQueueItem[]) {
  pending = items;
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, ({ request }) => {
      queueReads += 1;
      const mode = new URL(request.url).searchParams.get("mode") ?? "student";
      return contractJson("/teacher/grading/queue", "get", 200, {
        items: pending,
        groups: groupsOf(mode),
        answersRemaining: pending.length,
        studentsWaiting: new Set(pending.map((item) => item.studentId)).size,
      });
    }),
    http.get(`${BASE}/teacher/attempts`, () =>
      contractJson("/teacher/attempts", "get", 200, {
        items: [],
        total: 0,
        page: 1,
        pageSize: 100,
      }),
    ),
    http.get(`${BASE}/teacher/attempts/:id`, ({ params }) => {
      const attemptId = String(params.id);
      const response: AttemptReview = review();
      response.attempt.id = attemptId;
      response.attempt.studentId = attemptId === NAM_PAPER ? NAM : LAN;
      response.attempt.score!.pendingManual = left(attemptId);
      for (const [key, points] of Object.entries(elsewhere)) {
        const [paper, questionId] = key.split(":");
        if (paper === attemptId)
          response.answers[questionId!] = {
            answer: { type: "text", value: "elsewhere" },
            requiresManual: true,
            manualScore: points,
            graderComment: "Chấm ở tab khác",
          };
      }
      return contractJson("/teacher/attempts/{id}", "get", 200, response);
    }),
    http.post(`${BASE}/teacher/attempts/:id/grade`, async ({ params, request }) => {
      const attemptId = String(params.id);
      const body = (await request.json()) as {
        items: { questionId: string; points: number }[];
      };
      for (const mark of body.items) {
        grades.push({ attemptId, questionId: mark.questionId, points: mark.points });
        pending = pending.filter(
          (item) =>
            gradingItemKey(item) !==
            gradingItemKey({ attemptId, questionId: mark.questionId }),
        );
      }
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 1,
        total: 2,
        pendingManual: left(attemptId),
      });
    }),
    http.post(`${BASE}/teacher/attempts/:id/finish-grading`, ({ params }) => {
      const finished: AttemptReview = review();
      finished.attempt.id = String(params.id);
      finished.attempt.studentId = params.id === NAM_PAPER ? NAM : LAN;
      return contractJson("/teacher/attempts/{id}/finish-grading", "post", 200, {
        ...finished.attempt,
        status: "graded",
      });
    }),
  );
}

async function saveAndNext(user: ReturnType<typeof userEvent.setup>) {
  const next = screen.getByRole("button", { name: "Lưu & câu tiếp theo" });
  await waitFor(() => expect(next).toBeEnabled());
  await user.click(next);
}

beforeEach(() => {
  useAuthStore.getState().setSession("token", teacherUser);
  grades = [];
  elsewhere = {};
  queueReads = 0;
});

function refocus() {
  act(() => {
    focusManager.setFocused(false);
    focusManager.setFocused(true);
  });
}

it("under StrictMode, a score click sends the Grade and Save & next goes to the student's own next answer", async () => {
  serve(byStudent());
  const user = mount("/teacher/grading");
  expect(await screen.findByText("Nam câu 1")).toBeVisible();
  expect(screen.getByText("Câu trả lời 1/2 của Nam")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Chấm 1 điểm" }));
  await waitFor(() =>
    expect(grades).toEqual([{ attemptId: NAM_PAPER, questionId: Q1, points: 1 }]),
  );
  await saveAndNext(user);
  expect(await screen.findByText("Nam câu 2")).toBeVisible();
  expect(screen.getByText("Câu trả lời 2/2 của Nam")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Chấm 0 điểm" }));
  await waitFor(() => expect(grades).toHaveLength(2));
  expect(screen.getByText("Câu trả lời 2/2 của Nam")).toBeVisible();
});

it("by question, a graded answer keeps its place, so Save & next reaches the next student on the same question", async () => {
  serve(byQuestion());
  const user = mount("/teacher/grading?mode=question");
  expect(await screen.findByText("Bài của Trần Văn Nam")).toBeVisible();
  expect(screen.getByText("Câu trả lời 1/2 của Nam")).toBeVisible();
  const field = screen.getByRole("spinbutton");
  await user.type(field, "7");
  await user.tab();
  await waitFor(() =>
    expect(grades).toEqual([{ attemptId: NAM_PAPER, questionId: Q1, points: 7 }]),
  );
  await saveAndNext(user);
  expect(await screen.findByText("Bài của Lê Thị Lan")).toBeVisible();
  expect(screen.getByText("Câu trả lời 2/2 của Lan")).toBeVisible();
});

it("grades from the keyboard alone after each save: 2, then j, then 1", async () => {
  serve(byStudent());
  const user = mount("/teacher/grading");
  const first = await screen.findByRole("button", { name: "Chấm 0 điểm" });
  first.focus();
  await user.keyboard("2");
  await waitFor(() =>
    expect(grades).toEqual([{ attemptId: NAM_PAPER, questionId: Q1, points: 0.5 }]),
  );
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Chấm 0.5 điểm" })).toBeEnabled(),
  );
  await user.keyboard("j");
  expect(await screen.findByText("Nam câu 2")).toBeVisible();
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Chấm 0 điểm" })).toBeEnabled(),
  );
  await user.keyboard("1");
  await waitFor(() =>
    expect(grades).toEqual([
      { attemptId: NAM_PAPER, questionId: Q1, points: 0.5 },
      { attemptId: NAM_PAPER, questionId: Q2, points: 0 },
    ]),
  );
});

it("takes a key with focus on the page and then puts focus on the pressed score", async () => {
  serve(byStudent());
  const user = mount("/teacher/grading");
  expect(await screen.findByRole("button", { name: "Chấm 0 điểm" })).toBeEnabled();
  expect(document.body).toHaveFocus();
  await user.keyboard("2");
  await waitFor(() =>
    expect(grades).toEqual([{ attemptId: NAM_PAPER, questionId: Q1, points: 0.5 }]),
  );
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Chấm 0.5 điểm" })).toHaveFocus(),
  );
});

it("keys typed into the comment and the number field neither score nor move", async () => {
  serve(byQuestion());
  const user = mount("/teacher/grading?mode=question");
  expect(await screen.findByText("Bài của Trần Văn Nam")).toBeVisible();
  await user.click(screen.getByLabelText(/^Nhận xét/));
  await user.keyboard("jk12");
  expect(screen.getByLabelText(/^Nhận xét/)).toHaveValue("jk12");
  await user.click(screen.getByRole("spinbutton"));
  await user.keyboard("jk");
  expect(grades).toEqual([]);
  expect(screen.getByText("Bài của Trần Văn Nam")).toBeVisible();
  expect(screen.getByText("Câu trả lời 1/2 của Nam")).toBeVisible();
});

it("offers nine keyed buttons for a four-point answer and a number field above four", async () => {
  serve([answer(NAM_PAPER, NAM, "Trần Văn Nam", Q2, 2, 4)]);
  mount("/teacher/grading?mode=question");
  expect(await screen.findByText("Bài của Trần Văn Nam")).toBeVisible();
  const buttons = screen.getAllByRole("button", { name: /^Chấm [\d.]+ điểm$/ });
  expect(buttons).toHaveLength(9);
  expect(buttons.map((button) => button.querySelector("kbd")?.textContent)).toEqual([
    "1",
    "2",
    "3",
    "4",
    "5",
    "6",
    "7",
    "8",
    "9",
  ]);
  expect(screen.queryByRole("spinbutton")).not.toBeInTheDocument();
});

it("by question, Finish on a student's last answer goes on to the next ungraded answer, not a graded one", async () => {
  serve([...byQuestion(), answer(LAN_PAPER, LAN, "Lê Thị Lan", Q2, 2, 4)]);
  const user = mount("/teacher/grading?mode=question");
  expect(await screen.findByText("Bài của Trần Văn Nam")).toBeVisible();
  await user.type(screen.getByRole("spinbutton"), "9");
  await user.tab();
  await waitFor(() => expect(grades).toHaveLength(1));
  await saveAndNext(user);
  expect(await screen.findByText("Câu trả lời 2/2 của Lan")).toBeVisible();
  await user.type(screen.getByRole("spinbutton"), "9");
  await user.tab();
  await waitFor(() => expect(grades).toHaveLength(2));
  await saveAndNext(user);
  expect(await screen.findByText("Nam câu 2")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Chấm 4 điểm" }));
  await waitFor(() => expect(grades).toHaveLength(3));
  await saveAndNext(user);
  const finish = await screen.findByRole("button", { name: /^Hoàn tất chấm/ });
  await waitFor(() => expect(finish).toBeEnabled());
  await user.click(finish);
  expect(await screen.findByText("Lan câu 2")).toBeVisible();
  expect(screen.getByRole("button", { name: "Chấm 0 điểm" })).toHaveAttribute(
    "aria-pressed",
    "false",
  );
});

it("puts focus on the heading after a filter chip is removed", async () => {
  serve(byStudent());
  const user = mount(`/teacher/grading?student=${NAM}`);
  const remove = await screen.findByRole("button", { name: "Bỏ lọc: Trần Văn Nam" });
  await waitFor(() => expect(remove).toBeEnabled());
  await user.click(remove);
  await waitFor(() =>
    expect(screen.getByRole("heading", { level: 1, name: "Chấm bài" })).toHaveFocus(),
  );
  expect(screen.queryByRole("button", { name: /^Bỏ lọc/ })).not.toBeInTheDocument();
});

it("refetches the queue every time the window regains focus, even inside the stale time", async () => {
  serve(byStudent());
  mount("/teacher/grading");
  expect(await screen.findByText("Nam câu 1")).toBeVisible();
  const before = queueReads;
  refocus();
  await waitFor(() => expect(queueReads).toBe(before + 1));
  refocus();
  await waitFor(() => expect(queueReads).toBe(before + 2));
});

it("shows another tab's grade on the open answer after a refetch, and keeps it open", async () => {
  serve(byStudent());
  mount("/teacher/grading");
  expect(await screen.findByText("Nam câu 1")).toBeVisible();
  expect(screen.getByRole("button", { name: "Chấm 1 điểm" })).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  pending = pending.filter(
    (item) =>
      gradingItemKey(item) !== gradingItemKey({ attemptId: NAM_PAPER, questionId: Q1 }),
  );
  elsewhere[`${NAM_PAPER}:${Q1}`] = 1;
  refocus();
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Chấm 1 điểm" })).toHaveAttribute(
      "aria-pressed",
      "true",
    ),
  );
  expect(screen.getByText("Nam câu 1")).toBeVisible();
  expect(screen.getByLabelText(/^Nhận xét/)).toHaveValue("Chấm ở tab khác");
});
