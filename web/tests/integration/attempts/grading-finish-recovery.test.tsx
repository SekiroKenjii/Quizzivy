import { StrictMode, type ReactNode } from "react";
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
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import { getTeacherSummary } from "@/features/dashboard/api";
import GradingPage from "@/features/attempts/pages/teacher/GradingPage";
import type { GradingQueueItem, AttemptReview } from "@/features/attempts/api";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { previewGroup } from "@tests/support/groupPreview";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import {
  BASE,
  ASSIGNMENT_ID,
  ATTEMPT_ID,
  STUDENT_ID,
  ESSAY_ID,
  review,
} from "../../units/attempts/fixtures";

const item: GradingQueueItem = {
  attemptId: ATTEMPT_ID,
  questionId: ESSAY_ID,
  assignmentId: ASSIGNMENT_ID,
  assignmentTitle: "Unit 5",
  studentId: STUDENT_ID,
  studentName: "Nguyễn Đức Minh",
  questionNumber: 2,
  type: "short_answer",
  prompt: "Question to mark",
  answer: { type: "text", value: "Saved answer" },
  points: 1,
  score: null,
  comment: null,
  sampleAnswer: "Sample",
};
let grades: unknown[];
let finishes: number;
let marked: boolean;
let freshPending: number | undefined;
let finishFails: boolean;

function mount(entry = "/teacher/grading", sibling?: ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/grading", element: <GradingPage /> },
      { path: "/teacher/attempts/:id", element: <p>Full review</p> },
    ],
    { initialEntries: [entry] },
  );
  render(
    <StrictMode>
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
        {sibling}
      </QueryClientProvider>
    </StrictMode>,
  );
  return { user: userEvent.setup(), router, client };
}

beforeEach(() => {
  useAuthStore.getState().setSession("token", teacherUser);
  grades = [];
  finishes = 0;
  marked = false;
  freshPending = 0;
  finishFails = false;
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: marked ? [] : [item],
        groups: marked
          ? []
          : [
              {
                key: STUDENT_ID,
                kind: "student",
                label: item.studentName,
                sub: item.assignmentTitle,
                remaining: 1,
              },
            ],
        answersRemaining: marked ? 0 : 1,
        studentsWaiting: marked ? 0 : 1,
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
    http.get(`${BASE}/teacher/attempts/${ATTEMPT_ID}`, () => {
      const response: AttemptReview = review({ essayScore: marked ? 0 : null });
      if (freshPending === undefined) delete response.attempt.score;
      else response.attempt.score!.pendingManual = freshPending;
      return contractJson("/teacher/attempts/{id}", "get", 200, response);
    }),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, async ({ request }) => {
      grades.push(await request.json());
      marked = true;
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 0,
        total: 1,
        pendingManual: 0,
      });
    }),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/finish-grading`, () => {
      finishes += 1;
      if (finishFails)
        return HttpResponse.json(
          { error: { code: "UNKNOWN", message: "Finish failed" } },
          { status: 500 },
        );
      return contractJson("/teacher/attempts/{id}/finish-grading", "post", 200, {
        ...review().attempt,
        status: "graded",
      });
    }),
  );
});

it("score0 saves Grade only; failed explicit Finish survives empty queue and retry sends no Grade", async () => {
  finishFails = true;
  const { user } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0 điểm" }));
  await waitFor(() =>
    expect(grades).toEqual([
      { items: [{ questionId: ESSAY_ID, points: 0, comment: null }] },
    ]),
  );
  expect(finishes).toBe(0);
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Lưu & câu tiếp theo" })).toBeEnabled(),
  );
  await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  expect(finishes).toBe(0);
  await user.click(await screen.findByRole("button", { name: "Hoàn tất chấm" }));
  expect(
    await screen.findByText("Điểm đã lưu, nhưng chưa hoàn tất chấm bài."),
  ).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Chấm 0 điểm" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(grades).toHaveLength(1);
  expect(finishes).toBe(1);
  finishFails = false;
  await user.click(screen.getByRole("button", { name: "Thử hoàn tất lại" }));
  await waitFor(() => expect(finishes).toBe(2));
  await waitFor(() =>
    expect(
      screen.queryByRole("button", { name: "Thử hoàn tất lại" }),
    ).not.toBeInTheDocument(),
  );
  expect(grades).toHaveLength(1);
});

it("comments and appended snippets flush at the chosen score before Finish", async () => {
  const { user } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0.5 điểm" }));
  await waitFor(() => expect(screen.getByLabelText(/^Nhận xét/)).toBeEnabled());
  await user.type(screen.getByLabelText(/^Nhận xét/), "Original");
  await user.click(screen.getByRole("button", { name: "Kiểm tra chính tả" }));
  await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  expect(finishes).toBe(0);
  await user.click(await screen.findByRole("button", { name: "Hoàn tất chấm" }));
  await waitFor(() => expect(finishes).toBe(1));
  expect(grades).toEqual([
    { items: [{ questionId: ESSAY_ID, points: 0.5, comment: null }] },
    {
      items: [
        { questionId: ESSAY_ID, points: 0.5, comment: "Original Kiểm tra chính tả." },
      ],
    },
  ]);
});

it.each([undefined, 1])(
  "fresh optional pending count %s never silently finishes",
  async (pending) => {
    freshPending = pending;
    const { user } = mount();
    await user.click(await screen.findByRole("button", { name: "Chấm 1 điểm" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Lưu & câu tiếp theo" })).toBeEnabled(),
    );
    await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Lưu & câu tiếp theo" })).toBeEnabled(),
    );
    expect(finishes).toBe(0);
    expect(grades).toHaveLength(1);
  },
);

it("denied permission performs no private queue/recovery read", async () => {
  let reads = 0;
  useAuthStore.getState().setUser({ ...teacherUser, permissions: [] });
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () => {
      reads += 1;
      return HttpResponse.error();
    }),
    http.get(`${BASE}/teacher/attempts`, () => {
      reads += 1;
      return HttpResponse.error();
    }),
  );
  mount();
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Bạn không có quyền chấm bài tại đây.",
  );
  expect(reads).toBe(0);
});

it("late Grade after same-ID new admission reveals no old answer or Finish recovery", async () => {
  let release: (() => void) | undefined;
  let asked = false;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  server.use(
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, async () => {
      asked = true;
      await held;
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 1,
        total: 1,
        pendingManual: 0,
      });
    }),
  );
  const { user } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 1 điểm" }));
  await waitFor(() => expect(asked).toBe(true));
  await act(() =>
    useAuthStore
      .getState()
      .setSession("new-token", { ...teacherUser, permissions: [] }),
  );
  await act(async () => {
    release?.();
    await held;
  });
  expect(screen.queryByText("Saved answer")).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Thử hoàn tất lại" }),
  ).not.toBeInTheDocument();
  expect(finishes).toBe(0);
});

it("mode and filters preserve unrelated duplicate parameters and hash", async () => {
  const { user, router } = mount(`/teacher/grading?x=1&x=2#paper`);
  await user.click(await screen.findByRole("button", { name: "Theo câu hỏi" }));
  expect(router.state.location.hash).toBe("#paper");
  expect(new URLSearchParams(router.state.location.search).getAll("x")).toEqual([
    "1",
    "2",
  ]);
  expect(new URLSearchParams(router.state.location.search).get("mode")).toBe(
    "question",
  );
});

it("input, contenteditable, modifiers and composition do not trigger score shortcuts", async () => {
  mount();
  const comment = await screen.findByLabelText(/^Nhận xét/);
  fireEvent.keyDown(comment, { key: "1" });
  const card = screen.getByRole("region", { name: "Chấm bài" });
  fireEvent.keyDown(card, { key: "1", isComposing: true });
  fireEvent.keyDown(card, { key: "1", ctrlKey: true });
  fireEvent.keyDown(card, { key: "1", shiftKey: true });
  const editable = document.createElement("div");
  editable.contentEditable = "true";
  editable.setAttribute("contenteditable", "true");
  card.append(editable);
  fireEvent.keyDown(editable, { key: "1" });
  expect(grades).toHaveLength(0);
  fireEvent.keyDown(within(card).getByRole("button", { name: "Chấm 0 điểm" }), {
    key: "1",
  });
  await waitFor(() => expect(grades).toHaveLength(1));
});

it("failed Grade retains the answer and refuses advancement and Finish", async () => {
  let calls = 0;
  server.use(
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, () => {
      calls += 1;
      return HttpResponse.json(
        { error: { code: "UNKNOWN", message: "Grade failed" } },
        { status: 500 },
      );
    }),
  );
  const { user } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0 điểm" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Grade failed");
  await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  await waitFor(() => expect(calls).toBe(2));
  expect(screen.getByText("Saved answer")).toBeInTheDocument();
  expect(finishes).toBe(0);
});

it("a held Grade disables duplicate score and next actions", async () => {
  let release: (() => void) | undefined;
  let calls = 0;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  server.use(
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, async () => {
      calls += 1;
      await held;
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 0,
        total: 1,
        pendingManual: 1,
      });
    }),
  );
  const { user } = mount();
  const score = await screen.findByRole("button", { name: "Chấm 0 điểm" });
  await user.click(score);
  await waitFor(() => expect(calls).toBe(1));
  expect(score).toBeDisabled();
  expect(screen.getByRole("button", { name: "Lưu & câu tiếp theo" })).toBeDisabled();
  fireEvent.click(score);
  fireEvent.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  expect(calls).toBe(1);
  await act(async () => {
    release?.();
    await held;
  });
  await waitFor(() => expect(score).toBeEnabled());
  expect(finishes).toBe(0);
});

it("fresh candidate review enables an explicit Finish without replaying any Grade", async () => {
  server.use(
    http.get(`${BASE}/teacher/attempts`, () =>
      contractJson("/teacher/attempts", "get", 200, {
        items: [
          {
            id: ATTEMPT_ID,
            assignmentId: ASSIGNMENT_ID,
            studentId: STUDENT_ID,
            studentName: "Recovery student",
            testTitle: "Recovery paper",
            status: "submitted",
            flagged: false,
          },
        ],
        total: 1,
        page: 1,
        pageSize: 100,
      }),
    ),
  );
  const { user } = mount();
  await user.click(
    await screen.findByRole("button", {
      name: "Xem bài của Recovery student: Recovery paper",
    }),
  );
  const finish = await screen.findByRole("button", { name: "Hoàn tất chấm" });
  expect(finish).toBeEnabled();
  expect(finishes).toBe(0);
  expect(grades).toHaveLength(0);
  await user.click(finish);
  await waitFor(() => expect(finishes).toBe(1));
  expect(grades).toHaveLength(0);
});

it("a candidate with missing current count cannot finish even when its list row says zero", async () => {
  freshPending = undefined;
  server.use(
    http.get(`${BASE}/teacher/attempts`, () =>
      contractJson("/teacher/attempts", "get", 200, {
        items: [
          {
            id: ATTEMPT_ID,
            assignmentId: ASSIGNMENT_ID,
            studentId: STUDENT_ID,
            studentName: "Recovery student",
            testTitle: "Recovery paper",
            status: "submitted",
            pendingManual: 0,
            flagged: false,
          },
        ],
        total: 1,
        page: 1,
        pageSize: 100,
      }),
    ),
  );
  const { user } = mount();
  await user.click(
    await screen.findByRole("button", {
      name: "Xem bài của Recovery student: Recovery paper",
    }),
  );
  expect(await screen.findByRole("button", { name: "Hoàn tất chấm" })).toBeDisabled();
  expect(finishes).toBe(0);
  expect(grades).toHaveLength(0);
});

it("permission loss aborts a held recovery scan without showing its late private candidates", async () => {
  let release: (() => void) | undefined;
  let called = false;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  server.use(
    http.get(`${BASE}/teacher/attempts`, async () => {
      called = true;
      await held;
      return contractJson("/teacher/attempts", "get", 200, {
        items: [
          {
            id: ATTEMPT_ID,
            assignmentId: ASSIGNMENT_ID,
            studentId: STUDENT_ID,
            studentName: "Late secret",
            testTitle: "Secret paper",
            status: "submitted",
            flagged: false,
          },
        ],
        total: 1,
        page: 1,
        pageSize: 100,
      });
    }),
  );
  mount();
  await waitFor(() => expect(called).toBe(true));
  await act(() => useAuthStore.getState().setUser({ ...teacherUser, permissions: [] }));
  await act(async () => {
    release?.();
    await held;
  });
  expect(screen.queryByText("Late secret")).not.toBeInTheDocument();
  expect(screen.queryByText("Saved answer")).not.toBeInTheDocument();
});

it("refill preserves server order, complete counts and saved progress across another student's selection", async () => {
  const second: GradingQueueItem = {
    ...item,
    attemptId: "018f0000-0000-7000-8000-000000000008",
    studentId: "018f0000-0000-7000-8000-000000000009",
    studentName: "Second student",
    answer: { type: "text", value: "Second answer" },
  };
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: marked ? [second] : [item],
        groups: [
          {
            key: marked ? second.studentId : item.studentId,
            kind: "student",
            label: marked ? second.studentName : item.studentName,
            sub: "Unit 5",
            remaining: marked ? 249 : 250,
          },
        ],
        answersRemaining: marked ? 249 : 250,
        studentsWaiting: marked ? 120 : 121,
      }),
    ),
  );
  const { user } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0 điểm" }));
  expect(
    await screen.findByText("Còn 249 câu trả lời từ 120 học viên."),
  ).toBeInTheDocument();
  expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "1");
  await user.click(
    screen.getByRole("button", { name: /Second student · Unit 5 · Còn 249/ }),
  );
  expect(await screen.findByText("Second answer")).toBeInTheDocument();
  expect(screen.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "1");
  expect(grades).toHaveLength(1);
  expect(finishes).toBe(0);
});

it("late mark from a departed filter cannot mutate the current answer or Finish it", async () => {
  const second: GradingQueueItem = {
    ...item,
    attemptId: "018f0000-0000-7000-8000-000000000008",
    studentId: "018f0000-0000-7000-8000-000000000009",
    studentName: "Second student",
    answer: { type: "text", value: "Current answer" },
  };
  let release: (() => void) | undefined;
  let asked = false;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, ({ request }) => {
      const current = new URL(request.url).searchParams.has("studentId")
        ? second
        : item;
      return contractJson("/teacher/grading/queue", "get", 200, {
        items: [current],
        groups: [
          {
            key: current.studentId,
            kind: "student",
            label: current.studentName,
            sub: "Unit 5",
            remaining: 1,
          },
        ],
        answersRemaining: 1,
        studentsWaiting: 1,
      });
    }),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, async () => {
      asked = true;
      await held;
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 1,
        total: 1,
        pendingManual: 0,
      });
    }),
  );
  const { user, router } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 1 điểm" }));
  await waitFor(() => expect(asked).toBe(true));
  await act(() => router.navigate(`/teacher/grading?student=${second.studentId}`));
  expect(await screen.findByText("Current answer")).toBeInTheDocument();
  await act(async () => {
    release?.();
    await held;
  });
  expect(screen.getByRole("button", { name: "Chấm 1 điểm" })).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  expect(finishes).toBe(0);
});

it("two score activations in the same React batch reserve one immutable mark", async () => {
  let release: (() => void) | undefined;
  let calls = 0;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  server.use(
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, async () => {
      calls += 1;
      await held;
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 0,
        total: 1,
        pendingManual: 1,
      });
    }),
  );
  mount();
  const zero = await screen.findByRole("button", { name: "Chấm 0 điểm" });
  const one = screen.getByRole("button", { name: "Chấm 1 điểm" });
  act(() => {
    fireEvent.click(zero);
    fireEvent.click(one);
  });
  await waitFor(() => expect(calls).toBe(1));
  await act(async () => {
    release?.();
    await held;
  });
  await waitFor(() => expect(zero).toBeEnabled());
  expect(calls).toBe(1);
});

it("same frozen question in different assignments stays in two canonical question groups", async () => {
  const second = {
    ...item,
    assignmentId: "018f0000-0000-7000-8000-000000000002",
    attemptId: "018f0000-0000-7000-8000-000000000003",
    answer: { type: "text", value: "Different frozen paper" },
  } satisfies GradingQueueItem;
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: [item, second],
        groups: [item, second].map((row) => ({
          key: `${row.assignmentId}:${row.questionId}`,
          kind: "question",
          label: "2",
          sub: "Unit 5",
          remaining: 1,
        })),
        answersRemaining: 2,
        studentsWaiting: 1,
      }),
    ),
  );
  const { user } = mount("/teacher/grading?mode=question");
  const rows = await screen.findAllByRole("button", { name: "Câu 2 · Unit 5 · Còn 1" });
  expect(rows).toHaveLength(2);
  await user.click(rows[1]!);
  expect(await screen.findByText("Different frozen paper")).toBeInTheDocument();
  expect(rows[1]).toHaveAttribute("aria-current", "true");
});

it("repeated student names remain distinct while one student's assignments stay together", async () => {
  const sameStudent = {
    ...item,
    assignmentId: "018f0000-0000-7000-8000-000000000002",
    attemptId: "018f0000-0000-7000-8000-000000000003",
    answer: { type: "text", value: "Same student second assignment" },
  } satisfies GradingQueueItem;
  const otherStudent = {
    ...item,
    studentId: "018f0000-0000-7000-8000-000000000004",
    attemptId: "018f0000-0000-7000-8000-000000000005",
    answer: { type: "text", value: "Same name different student" },
  } satisfies GradingQueueItem;
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: [item, sameStudent, otherStudent],
        groups: [
          {
            key: item.studentId,
            kind: "student",
            label: item.studentName,
            sub: "Unit 5",
            remaining: 2,
          },
          {
            key: otherStudent.studentId,
            kind: "student",
            label: otherStudent.studentName,
            sub: "Unit 5",
            remaining: 1,
          },
        ],
        answersRemaining: 3,
        studentsWaiting: 2,
      }),
    ),
  );
  const { user } = mount();
  const secondGroup = await screen.findByRole("button", {
    name: "Nguyễn Đức Minh · Unit 5 · Còn 1",
  });
  expect(
    screen.getByRole("button", { name: "Nguyễn Đức Minh · Unit 5 · Còn 2" }),
  ).toBeInTheDocument();
  await user.click(secondGroup);
  expect(await screen.findByText("Same name different student")).toBeInTheDocument();
  expect(screen.getByText("Câu trả lời 1/1 của Minh")).toBeInTheDocument();
});

function Summary() {
  const summary = useQuery({
    queryKey: ["admin-dashboard", "summary"],
    queryFn: ({ signal }) => getTeacherSummary(signal),
  });
  return <p>Manual answers waiting: {summary.data?.answersToGrade}</p>;
}

it("acknowledged marks update the real mounted teacher-summary reading before Finish", async () => {
  server.use(
    http.get(`${BASE}/teacher/summary`, () =>
      contractJson("/teacher/summary", "get", 200, {
        liveAssignments: 1,
        answersToGrade: marked ? 0 : 1,
        unreadNotifications: 0,
      }),
    ),
  );
  const { user } = mount("/teacher/grading", <Summary />);
  expect(await screen.findByText("Manual answers waiting: 1")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Chấm 0 điểm" }));
  expect(await screen.findByText("Manual answers waiting: 0")).toBeInTheDocument();
  expect(finishes).toBe(0);
});

it("numeric fallback preserves half-point marks and refuses values above the real ceiling", async () => {
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: [{ ...item, points: 5 }],
        groups: [
          {
            key: STUDENT_ID,
            kind: "student",
            label: item.studentName,
            sub: item.assignmentTitle,
            remaining: 1,
          },
        ],
        answersRemaining: 1,
        studentsWaiting: 1,
      }),
    ),
  );
  const { user } = mount();
  const input = await screen.findByRole("spinbutton", { name: "Điểm (0–5)" });
  expect(screen.queryByRole("button", { name: "Chấm 0 điểm" })).not.toBeInTheDocument();
  await user.type(input, "6");
  await user.tab();
  expect(input).toHaveAttribute("aria-invalid", "true");
  expect(grades).toHaveLength(0);
  await user.clear(input);
  await user.type(input, "3.5");
  await user.tab();
  await waitFor(() =>
    expect(grades).toEqual([
      { items: [{ questionId: ESSAY_ID, points: 3.5, comment: null }] },
    ]),
  );
  expect(finishes).toBe(0);
});

it("selecting the next pending group row flushes the current comment under its original question identity", async () => {
  const second = {
    ...item,
    questionId: "018f0000-0000-7000-8000-000000000003",
    questionNumber: 3,
    answer: { type: "text", value: "Next question answer" },
  } satisfies GradingQueueItem;
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: marked ? [second] : [item, second],
        groups: [
          {
            key: STUDENT_ID,
            kind: "student",
            label: item.studentName,
            sub: item.assignmentTitle,
            remaining: marked ? 1 : 2,
          },
        ],
        answersRemaining: marked ? 1 : 2,
        studentsWaiting: 1,
      }),
    ),
  );
  const { user } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0 điểm" }));
  await waitFor(() => expect(screen.getByLabelText(/^Nhận xét/)).toBeEnabled());
  await user.type(screen.getByLabelText(/^Nhận xét/), "Current comment");
  await user.click(
    screen.getByRole("button", { name: "Nguyễn Đức Minh · Unit 5 · Còn 1" }),
  );
  expect(await screen.findByText("Next question answer")).toBeInTheDocument();
  expect(grades).toEqual([
    { items: [{ questionId: ESSAY_ID, points: 0, comment: null }] },
    { items: [{ questionId: ESSAY_ID, points: 0, comment: "Current comment" }] },
  ]);
  expect(finishes).toBe(0);
});

it("mode changes flush edited acknowledged comments before replacing the queue owner", async () => {
  const { user, router } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0 điểm" }));
  await waitFor(() => expect(screen.getByLabelText(/^Nhận xét/)).toBeEnabled());
  await user.type(screen.getByLabelText(/^Nhận xét/), "Keep across mode");
  await user.click(screen.getByRole("button", { name: "Theo câu hỏi" }));
  await waitFor(() =>
    expect(new URLSearchParams(router.state.location.search).get("mode")).toBe(
      "question",
    ),
  );
  expect(grades).toEqual([
    { items: [{ questionId: ESSAY_ID, points: 0, comment: null }] },
    { items: [{ questionId: ESSAY_ID, points: 0, comment: "Keep across mode" }] },
  ]);
  expect(finishes).toBe(0);
});

it("full-review navigation retains the latest comment and refuses departure when its save fails", async () => {
  const { user, router } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0 điểm" }));
  await waitFor(() => expect(screen.getByLabelText(/^Nhận xét/)).toBeEnabled());
  await user.type(screen.getByLabelText(/^Nhận xét/), "Keep before review");
  server.use(
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, () =>
      HttpResponse.json(
        { error: { code: "UNKNOWN", message: "Comment save failed" } },
        { status: 500 },
      ),
    ),
  );
  await user.click(screen.getByRole("link", { name: "Xem toàn bộ bài" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Comment save failed");
  expect(router.state.location.pathname).toBe("/teacher/grading");
  expect(screen.getByLabelText(/^Nhận xét/)).toHaveValue("Keep before review");
  expect(finishes).toBe(0);
});

it("same-ID readmission with permission keeps its new answer unmarked when the old Grade completes", async () => {
  let release: (() => void) | undefined;
  let called = false;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, ({ request }) => {
      const fresh = request.headers.get("authorization") === "Bearer new-token";
      return contractJson("/teacher/grading/queue", "get", 200, {
        items: [
          {
            ...item,
            answer: {
              type: "text",
              value: fresh ? "Current admission answer" : "Old admission answer",
            },
          },
        ],
        groups: [
          {
            key: STUDENT_ID,
            kind: "student",
            label: item.studentName,
            sub: item.assignmentTitle,
            remaining: 1,
          },
        ],
        answersRemaining: 1,
        studentsWaiting: 1,
      });
    }),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, async () => {
      called = true;
      await held;
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 0,
        total: 1,
        pendingManual: 0,
      });
    }),
  );
  const { user } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0 điểm" }));
  await waitFor(() => expect(called).toBe(true));
  await act(() => useAuthStore.getState().setSession("new-token", teacherUser));
  expect(await screen.findByText("Current admission answer")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Chấm 0 điểm" })).toBeEnabled();
  await act(async () => {
    release?.();
    await held;
  });
  expect(screen.getByRole("button", { name: "Chấm 0 điểm" })).toHaveAttribute(
    "aria-pressed",
    "false",
  );
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(finishes).toBe(0);
});

it("current zero presents explicit Finish intent before the Finish request", async () => {
  const { user } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0 điểm" }));
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Lưu & câu tiếp theo" })).toBeEnabled(),
  );
  await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  expect(await screen.findByRole("button", { name: "Hoàn tất chấm" })).toBeEnabled();
  expect(finishes).toBe(0);
  expect(grades).toHaveLength(1);
  await user.click(screen.getByRole("button", { name: "Hoàn tất chấm" }));
  await waitFor(() => expect(finishes).toBe(1));
  expect(grades).toHaveLength(1);
});

it("material Retry renews the current queue URL without losing an ungraded comment or issuing Grade", async () => {
  let reads = 0;
  let renewed = false;
  const asset = previewGroup.assets[0]!;
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () => {
      reads += 1;
      return contractJson("/teacher/grading/queue", "get", 200, {
        items: [
          {
            ...item,
            sharedContext: {
              transcripts: {},
              groups: [
                {
                  ...previewGroup,
                  questionIds: [ESSAY_ID],
                  recordings: [],
                  stimuli: [
                    {
                      ...previewGroup.stimuli[0]!,
                      gaps: [],
                      content: {
                        format: "semantic_v1",
                        blocks: [
                          { type: "image", assetId: asset.id, alt: "Frozen material" },
                        ],
                      },
                    },
                  ],
                  assets: [
                    {
                      ...asset,
                      kind: "image",
                      mimeType: "image/png",
                      url: renewed
                        ? "https://assets.example/renewed.png"
                        : "https://assets.example/expired.png",
                    },
                  ],
                },
              ],
            },
          },
        ],
        groups: [
          {
            key: STUDENT_ID,
            kind: "student",
            label: item.studentName,
            sub: item.assignmentTitle,
            remaining: 1,
          },
        ],
        answersRemaining: 1,
        studentsWaiting: 1,
      });
    }),
  );
  const { user } = mount();
  const image = await screen.findByRole("img", { name: "Frozen material" });
  expect(image).toHaveAttribute("src", "https://assets.example/expired.png");
  await user.type(screen.getByLabelText(/^Nhận xét/), "Keep ungraded comment");
  fireEvent.error(image);
  const before = reads;
  renewed = true;
  await user.click(screen.getByRole("button", { name: "Thử lại" }));
  await waitFor(() => expect(reads).toBe(before + 1));
  await waitFor(() =>
    expect(screen.getByRole("img", { name: "Frozen material" })).toHaveAttribute(
      "src",
      "https://assets.example/renewed.png",
    ),
  );
  expect(screen.getByLabelText(/^Nhận xét/)).toHaveValue("Keep ungraded comment");
  expect(grades).toHaveLength(0);
  expect(finishes).toBe(0);
});

it("explicit Finish rereads current count and refuses a zero that changed after confirmation readiness", async () => {
  const { user } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0 điểm" }));
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Lưu & câu tiếp theo" })).toBeEnabled(),
  );
  await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  const finish = await screen.findByRole("button", { name: "Hoàn tất chấm" });
  freshPending = 1;
  await user.click(finish);
  await waitFor(() =>
    expect(
      screen.queryByRole("button", { name: "Hoàn tất chấm" }),
    ).not.toBeInTheDocument(),
  );
  expect(screen.getByRole("alert")).toHaveTextContent(
    "Chưa xác nhận được số câu còn chờ chấm. Xem lại bài trước khi hoàn tất.",
  );
  expect(finishes).toBe(0);
  expect(grades).toHaveLength(1);
});

it("editing a comment clears Finish readiness and flushes the latest comment before another explicit Finish", async () => {
  const { user } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0 điểm" }));
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Lưu & câu tiếp theo" })).toBeEnabled(),
  );
  await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  await screen.findByRole("button", { name: "Hoàn tất chấm" });
  await user.type(screen.getByLabelText(/^Nhận xét/), "Latest before Finish");
  expect(
    screen.queryByRole("button", { name: "Hoàn tất chấm" }),
  ).not.toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  expect(await screen.findByRole("button", { name: "Hoàn tất chấm" })).toBeEnabled();
  expect(grades).toEqual([
    { items: [{ questionId: ESSAY_ID, points: 0, comment: null }] },
    { items: [{ questionId: ESSAY_ID, points: 0, comment: "Latest before Finish" }] },
  ]);
  expect(finishes).toBe(0);
  await user.click(screen.getByRole("button", { name: "Hoàn tất chấm" }));
  await waitFor(() => expect(finishes).toBe(1));
  expect(grades).toHaveLength(2);
});

it("keeps Finish recovery text and real retry/review controls together in Alert's content column", async () => {
  finishFails = true;
  const { user } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0 điểm" }));
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Lưu & câu tiếp theo" })).toBeEnabled(),
  );
  await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  await user.click(await screen.findByRole("button", { name: "Hoàn tất chấm" }));
  const notice = await screen.findByText("Điểm đã lưu, nhưng chưa hoàn tất chấm bài.");
  const alert = screen.getByRole("alert");
  const description = alert.querySelector(":scope > [data-slot=alert-description]");
  expect(description).toContainElement(notice);
  expect(description).toContainElement(screen.getByText("Finish failed"));
  expect(description).toContainElement(
    screen.getByRole("button", { name: "Thử hoàn tất lại" }),
  );
  expect(description).toContainElement(
    within(alert).getByRole("link", { name: "Xem toàn bộ bài" }),
  );
  expect(description).toHaveClass("col-start-2");
  expect(alert.children).toHaveLength(1);
  expect(grades).toHaveLength(1);
  expect(finishes).toBe(1);
  finishFails = false;
  await user.click(screen.getByRole("button", { name: "Thử hoàn tất lại" }));
  await waitFor(() => expect(finishes).toBe(2));
  expect(grades).toHaveLength(1);
});

it("places a current-count refusal in Alert's content column without sending Finish", async () => {
  const { user } = mount();
  await user.click(await screen.findByRole("button", { name: "Chấm 0 điểm" }));
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Lưu & câu tiếp theo" })).toBeEnabled(),
  );
  await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  const finish = await screen.findByRole("button", { name: "Hoàn tất chấm" });
  freshPending = 1;
  await user.click(finish);
  const alert = await screen.findByRole("alert");
  const description = alert.querySelector(":scope > [data-slot=alert-description]");
  expect(description).toHaveTextContent(
    "Chưa xác nhận được số câu còn chờ chấm. Xem lại bài trước khi hoàn tất.",
  );
  expect(description).toHaveClass("col-start-2");
  expect(alert.children).toHaveLength(1);
  expect(finishes).toBe(0);
  expect(grades).toHaveLength(1);
});

it("places denied grading text in Alert's content column without revealing private work", () => {
  useAuthStore.getState().setUser({ ...teacherUser, permissions: [] });
  mount();
  const alert = screen.getByRole("alert");
  const description = alert.querySelector(":scope > [data-slot=alert-description]");
  expect(description).toHaveTextContent("Bạn không có quyền chấm bài tại đây.");
  expect(description).toHaveClass("col-start-2");
  expect(alert.children).toHaveLength(1);
  expect(screen.queryByText("Saved answer")).not.toBeInTheDocument();
  expect(grades).toHaveLength(0);
  expect(finishes).toBe(0);
});

it("one numeric blur and Save-next click waits for its sole Grade before offering explicit Finish", async () => {
  let release: (() => void) | undefined;
  let asked = false;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  let blurs = 0;
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: marked ? [] : [{ ...item, points: 5 }],
        groups: marked
          ? []
          : [
              {
                key: STUDENT_ID,
                kind: "student",
                label: item.studentName,
                sub: item.assignmentTitle,
                remaining: 1,
              },
            ],
        answersRemaining: marked ? 0 : 1,
        studentsWaiting: marked ? 0 : 1,
      }),
    ),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, async ({ request }) => {
      grades.push(await request.json());
      asked = true;
      await held;
      marked = true;
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 2.5,
        total: 5,
        pendingManual: 0,
      });
    }),
  );
  const { user } = mount();
  const input = await screen.findByRole("spinbutton", { name: "Điểm (0–5)" });
  input.addEventListener("blur", () => {
    blurs += 1;
  });
  await user.type(input, "2.5");
  await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  await waitFor(() => expect(asked).toBe(true));
  expect(blurs).toBe(1);
  expect(grades).toEqual([
    { items: [{ questionId: ESSAY_ID, points: 2.5, comment: null }] },
  ]);
  expect(finishes).toBe(0);
  expect(
    screen.queryByRole("button", { name: "Hoàn tất chấm" }),
  ).not.toBeInTheDocument();
  await act(async () => {
    release?.();
    await held;
  });
  expect(await screen.findByRole("button", { name: "Hoàn tất chấm" })).toBeEnabled();
  expect(grades).toHaveLength(1);
  expect(finishes).toBe(0);
  await user.click(screen.getByRole("button", { name: "Hoàn tất chấm" }));
  await waitFor(() => expect(finishes).toBe(1));
  expect(grades).toHaveLength(1);
});

it("numeric Save-next blur keeps a failed mark visible without advancing or finishing", async () => {
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: [{ ...item, points: 5 }],
        groups: [
          {
            key: STUDENT_ID,
            kind: "student",
            label: item.studentName,
            sub: item.assignmentTitle,
            remaining: 1,
          },
        ],
        answersRemaining: 1,
        studentsWaiting: 1,
      }),
    ),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, async ({ request }) => {
      grades.push(await request.json());
      return HttpResponse.json(
        { error: { code: "UNKNOWN", message: "Numeric save refused" } },
        { status: 500 },
      );
    }),
  );
  const { user } = mount();
  const input = await screen.findByRole("spinbutton", { name: "Điểm (0–5)" });
  await user.type(input, "2.5");
  await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Numeric save refused");
  expect(input).toHaveValue(2.5);
  expect(grades).toHaveLength(1);
  expect(finishes).toBe(0);
  expect(
    screen.queryByRole("button", { name: "Hoàn tất chấm" }),
  ).not.toBeInTheDocument();
});

it("grading footer exposes the existing translated Previous control", async () => {
  mount();
  expect(await screen.findByRole("button", { name: "Trước" })).toBeDisabled();
  expect(screen.queryByText("common.previous")).not.toBeInTheDocument();
});

it("pointer cancellation leaves numeric blur as Grade only and does not queue advancement", async () => {
  let release: (() => void) | undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: [{ ...item, points: 5 }],
        groups: [
          {
            key: STUDENT_ID,
            kind: "student",
            label: item.studentName,
            sub: item.assignmentTitle,
            remaining: 1,
          },
        ],
        answersRemaining: 1,
        studentsWaiting: 1,
      }),
    ),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, async ({ request }) => {
      grades.push(await request.json());
      await held;
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 2.5,
        total: 5,
        pendingManual: 0,
      });
    }),
  );
  const { user } = mount();
  const input = await screen.findByRole("spinbutton", { name: "Điểm (0–5)" });
  await user.type(input, "2.5");
  const next = screen.getByRole("button", { name: "Lưu & câu tiếp theo" });
  await user.pointer({ keys: "[MouseLeft>]", target: next });
  await waitFor(() => expect(grades).toHaveLength(1));
  fireEvent.pointerCancel(next);
  expect(next).toBeDisabled();
  await user.pointer({ keys: "[/MouseLeft]", target: next });
  await act(async () => {
    release?.();
    await held;
  });
  await waitFor(() => expect(next).toBeEnabled());
  expect(grades).toHaveLength(1);
  expect(finishes).toBe(0);
  expect(
    screen.queryByRole("button", { name: "Hoàn tất chấm" }),
  ).not.toBeInTheDocument();
});

it("actor departure cancels a numeric Save-next intent waiting for the old Grade", async () => {
  let release: (() => void) | undefined;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: [{ ...item, points: 5 }],
        groups: [
          {
            key: STUDENT_ID,
            kind: "student",
            label: item.studentName,
            sub: item.assignmentTitle,
            remaining: 1,
          },
        ],
        answersRemaining: 1,
        studentsWaiting: 1,
      }),
    ),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, async ({ request }) => {
      grades.push(await request.json());
      await held;
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 2.5,
        total: 5,
        pendingManual: 0,
      });
    }),
  );
  const { user } = mount();
  await user.type(await screen.findByRole("spinbutton", { name: "Điểm (0–5)" }), "2.5");
  await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
  await waitFor(() => expect(grades).toHaveLength(1));
  await act(() =>
    useAuthStore
      .getState()
      .setSession("new-token", { ...teacherUser, permissions: [] }),
  );
  await act(async () => {
    release?.();
    await held;
  });
  expect(screen.queryByText("Saved answer")).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Hoàn tất chấm" }),
  ).not.toBeInTheDocument();
  expect(grades).toHaveLength(1);
  expect(finishes).toBe(0);
});

it("numeric refusal before pointer-up is not replayed by the same Save-next activation", async () => {
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: [{ ...item, points: 5 }],
        groups: [
          {
            key: STUDENT_ID,
            kind: "student",
            label: item.studentName,
            sub: item.assignmentTitle,
            remaining: 1,
          },
        ],
        answersRemaining: 1,
        studentsWaiting: 1,
      }),
    ),
    http.post(`${BASE}/teacher/attempts/${ATTEMPT_ID}/grade`, async ({ request }) => {
      grades.push(await request.json());
      return HttpResponse.json(
        { error: { code: "UNKNOWN", message: "Already refused" } },
        { status: 500 },
      );
    }),
  );
  const { user } = mount();
  await user.type(await screen.findByRole("spinbutton", { name: "Điểm (0–5)" }), "2.5");
  const next = screen.getByRole("button", { name: "Lưu & câu tiếp theo" });
  await user.pointer({ keys: "[MouseLeft>]", target: next });
  expect(await screen.findByRole("alert")).toHaveTextContent("Already refused");
  await user.pointer({ keys: "[/MouseLeft]", target: next });
  await waitFor(() => expect(next).toBeEnabled());
  expect(grades).toHaveLength(1);
  expect(finishes).toBe(0);
  expect(
    screen.queryByRole("button", { name: "Hoàn tất chấm" }),
  ).not.toBeInTheDocument();
});
