import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import { Toaster } from "@/components/ui/sonner";
import GradingPage from "@/features/attempts/pages/teacher/GradingPage";
import type { GradingQueue, GradingQueueItem } from "@/features/attempts/api";
import type { AiScoreSuggestion } from "@/features/ai/api";
import { api } from "@/lib/api/client";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";
import "@/features/attempts/gradingMessages";
import { aiError, aiOn, month } from "../ai/support";
import { ASSIGNMENT_ID, ATTEMPT_ID, BASE, ESSAY_ID, STUDENT_ID } from "./fixtures";

vi.mock("@/lib/api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api/client")>();
  return { ...actual, api: vi.fn(actual.api) };
});

const STUDENT = "Nguyễn Đức Minh";
const SCORE_PATH =
  "/teacher/ai/attempts/{attemptId}/answers/{questionId}/score-suggestion";
const SOURCE = resolve(
  import.meta.dirname,
  "../../../src/features/attempts/components/AiScoreBox.tsx",
);

const first: GradingQueueItem = {
  attemptId: ATTEMPT_ID,
  questionId: ESSAY_ID,
  assignmentId: ASSIGNMENT_ID,
  assignmentTitle: `Unit 5 · ${STUDENT}`,
  studentId: STUDENT_ID,
  studentName: STUDENT,
  questionNumber: 2,
  type: "short_answer",
  prompt: "Where do the children play?",
  answer: { type: "text", value: "play street" },
  points: 1,
  score: null,
  comment: null,
  blanks: [],
  sampleAnswer: "play streets",
};

const second: GradingQueueItem = {
  ...first,
  attemptId: "018f0000-0000-7000-8000-0000000000a8",
  studentId: "018f0000-0000-7000-8000-0000000000e2",
  studentName: "Trần Thu Hà",
  assignmentTitle: "Unit 5",
};

const suggestion: AiScoreSuggestion = {
  score: 0.5,
  confidence: "medium",
  reasons: ["The plural is missing.", "The idea matches the passage."],
  comment: "Close: the passage says play streets.",
  usage: { credits: 400, left: 18400 },
};

let suggested: { url: string; body: string }[] = [];
let graded: unknown[] = [];

function serve(answer: () => Response | Promise<Response>) {
  server.use(
    http.get(`${BASE}/teacher/grading/queue`, () =>
      contractJson("/teacher/grading/queue", "get", 200, {
        items: [first, second],
        groups: [],
        answersRemaining: 2,
        studentsWaiting: 2,
      } satisfies GradingQueue),
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
      async ({ request }) => {
        suggested.push({ url: request.url, body: await request.text() });
        return answer();
      },
    ),
    http.post(`${BASE}/teacher/attempts/:id/grade`, async ({ request }) => {
      graded.push(await request.json());
      return contractJson("/teacher/attempts/{id}/grade", "post", 200, {
        earned: 0.5,
        total: 1,
        pendingManual: 1,
      });
    }),
  );
}

function answered() {
  return contractJson(SCORE_PATH, "post", 200, suggestion);
}

function renderGrading() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/grading", element: <GradingPage /> },
      { path: "/teacher/settings/:section", element: <p>settings page</p> },
    ],
    { initialEntries: ["/teacher/grading"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return { router, user: userEvent.setup() };
}

function calls() {
  return vi.mocked(api).mock.calls.map(([method, path]) => `${method} ${path}`);
}

function card() {
  return screen.findByRole("region", { name: "Câu trả lời đang chấm" });
}

beforeEach(() => {
  suggested = [];
  graded = [];
  vi.mocked(api).mockClear();
  aiOn();
  serve(answered);
});

afterEach(() => {
  useAuthStore.setState({ accessToken: null, user: null });
});

describe("the grading card's Suggest a score", () => {
  it("is absent while the server has no assistant", async () => {
    aiOn({ ai: false });
    renderGrading();
    await card();
    await waitFor(() => expect(calls()).toContain("get /public/status"));
    expect(screen.queryByRole("region", { name: "Gợi ý của AI" })).toBeNull();
    expect(screen.queryByText(/Gợi ý của AI đang tắt/)).toBeNull();
  });

  it("is absent for a teacher without ai.use, who never reads the status", async () => {
    useAuthStore.setState({
      user: {
        ...teacherUser,
        permissions: teacherUser.permissions.filter((key) => key !== "ai.use"),
      },
    });
    renderGrading();
    await card();
    expect(screen.queryByRole("region", { name: "Gợi ý của AI" })).toBeNull();
    expect(calls()).not.toContain("get /public/status");
  });

  it("says in one line, with a link to Settings › AI assistant, that suggestions are off", async () => {
    useAuthStore.setState({
      user: {
        ...teacherUser,
        preferences: { ...teacherUser.preferences, ai: { readStudentAnswers: false } },
      },
    });
    const { router, user } = renderGrading();
    const link = await screen.findByRole("link", { name: "Cài đặt › Trợ lý AI" });
    expect(link.closest("p")).toHaveTextContent(
      "Gợi ý của AI đang tắt trong Cài đặt › Trợ lý AI.",
    );
    expect(screen.queryByRole("button", { name: "Gợi ý điểm" })).toBeNull();
    await user.click(link);
    expect(router.state.location.pathname).toBe("/teacher/settings/ai");
    expect(suggested).toEqual([]);
  });

  it("fills the score and the comment without a write, and Save & next saves them", async () => {
    const { user } = renderGrading();
    const box = await screen.findByRole("region", { name: "Gợi ý của AI" });
    expect(
      within(box).getByText(
        "So sánh câu trả lời với các đáp án được chấp nhận. Bạn quyết định điểm.",
      ),
    ).toBeVisible();
    await user.click(within(box).getByRole("button", { name: "Gợi ý điểm" }));
    expect(await within(box).findByText("Độ tin cậy trung bình")).toBeVisible();
    expect(within(box).getByText("0,5")).toBeVisible();
    expect(within(box).getByText("/ 1")).toBeVisible();
    expect(within(box).getByText("The plural is missing.")).toBeVisible();
    expect(
      within(box).getByText("Close: the passage says play streets."),
    ).toBeVisible();
    expect(within(box).getByText("Đã dùng 400 tín dụng · còn 18.400")).toBeVisible();

    await user.click(
      within(box).getByRole("button", { name: "Dùng điểm và nhận xét" }),
    );
    expect(
      await screen.findByText(
        "Đã điền điểm và nhận xét. Hãy kiểm tra lại, rồi bấm Lưu & câu tiếp theo.",
      ),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Chấm 0,5 điểm" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByLabelText(/^Nhận xét/)).toHaveValue(
      "Close: the passage says play streets.",
    );
    expect(graded).toEqual([]);
    expect(calls()).not.toContain("post /teacher/attempts/{id}/grade");

    await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
    await waitFor(() =>
      expect(graded).toEqual([
        {
          items: [
            {
              questionId: ESSAY_ID,
              points: 0.5,
              comment: "Close: the passage says play streets.",
            },
          ],
        },
      ]),
    );
    expect(
      await screen.findByText("Câu trả lời 1/1 của Hà", { exact: false }),
    ).toBeVisible();
  });

  it("asks before a route change drops a filled score, and Save and leave writes it", async () => {
    serve(() => contractJson(SCORE_PATH, "post", 200, { ...suggestion, comment: "" }));
    const { router, user } = renderGrading();
    const box = await screen.findByRole("region", { name: "Gợi ý của AI" });
    await user.click(within(box).getByRole("button", { name: "Gợi ý điểm" }));
    await user.click(
      await within(box).findByRole("button", { name: "Dùng điểm và nhận xét" }),
    );
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Chấm 0,5 điểm" })).toHaveAttribute(
        "aria-pressed",
        "true",
      ),
    );
    expect(screen.getByLabelText(/^Nhận xét/)).toHaveValue("");

    const unload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(unload);
    expect(unload.defaultPrevented).toBe(true);

    await act(() => router.navigate("/teacher/settings/ai"));
    const dialog = await screen.findByRole("dialog", {
      name: "Rời trang khi điểm chưa lưu?",
    });
    expect(
      within(dialog).getByText("Điểm bạn đã điền cho câu trả lời này chưa được lưu."),
    ).toBeVisible();
    expect(screen.queryByText("settings page")).toBeNull();
    expect(graded).toEqual([]);

    await user.click(within(dialog).getByRole("button", { name: "Lưu và rời đi" }));
    expect(await screen.findByText("settings page")).toBeVisible();
    expect(graded).toEqual([
      { items: [{ questionId: ESSAY_ID, points: 0.5, comment: null }] },
    ]);
  });

  it("lets a route change go without asking once Save & next has saved the fill", async () => {
    const { router, user } = renderGrading();
    const box = await screen.findByRole("region", { name: "Gợi ý của AI" });
    await user.click(within(box).getByRole("button", { name: "Gợi ý điểm" }));
    await user.click(
      await within(box).findByRole("button", { name: "Dùng điểm và nhận xét" }),
    );
    await user.click(screen.getByRole("button", { name: "Lưu & câu tiếp theo" }));
    await waitFor(() => expect(graded).toHaveLength(1));
    expect(
      await screen.findByText("Câu trả lời 1/1 của Hà", { exact: false }),
    ).toBeVisible();

    const unload = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(unload);
    expect(unload.defaultPrevented).toBe(false);

    await act(() => router.navigate("/teacher/settings/ai"));
    expect(await screen.findByText("settings page")).toBeVisible();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(graded).toHaveLength(1);
  });

  it("sends no student's name, only the attempt and question it names", async () => {
    const { user } = renderGrading();
    const box = await screen.findByRole("region", { name: "Gợi ý của AI" });
    await user.click(within(box).getByRole("button", { name: "Gợi ý điểm" }));
    await within(box).findByText("Độ tin cậy trung bình");
    expect(suggested).toHaveLength(1);
    const [request] = suggested;
    expect(new URL(request!.url).pathname).toBe(
      `/teacher/ai/attempts/${ATTEMPT_ID}/answers/${ESSAY_ID}/score-suggestion`,
    );
    expect(request!.body).toBe("");
    const sent = JSON.stringify(
      vi.mocked(api).mock.calls.filter(([, path]) => path === SCORE_PATH),
    );
    for (const text of [request!.url, request!.body, sent]) {
      expect(text).not.toContain(STUDENT);
      expect(text).not.toContain(encodeURIComponent(STUDENT));
      expect(text).not.toContain(STUDENT_ID);
    }
  });

  it("says the AI didn't respond on a 503, and tries again", async () => {
    let fail = true;
    serve(() => (fail ? aiError(503, "AI_UNAVAILABLE") : answered()));
    const { user } = renderGrading();
    const box = await screen.findByRole("region", { name: "Gợi ý của AI" });
    await user.click(within(box).getByRole("button", { name: "Gợi ý điểm" }));
    expect(
      await within(box).findByText("AI chưa phản hồi. Hãy thử lại sau một phút."),
    ).toBeVisible();
    fail = false;
    await user.click(within(box).getByRole("button", { name: "Thử lại" }));
    expect(await within(box).findByText("Độ tin cậy trung bình")).toBeVisible();
  });

  it("gives the reason and does not run when the allowance is used up", async () => {
    serve(() => aiError(402, "AI_CREDITS_EXHAUSTED", { scope: "teacher" }));
    const { user } = renderGrading();
    const box = await screen.findByRole("region", { name: "Gợi ý của AI" });
    await user.click(within(box).getByRole("button", { name: "Gợi ý điểm" }));
    expect(
      await within(box).findByText(
        "Bạn đã dùng hết hạn mức. Hãy xin thêm trong Cài đặt.",
      ),
    ).toBeVisible();
    expect(within(box).getByRole("button", { name: "Gợi ý điểm" })).toBeDisabled();
  });

  it("is disabled with the reason while the teacher is paused", async () => {
    aiOn({ usage: month({ paused: true }) });
    renderGrading();
    const box = await screen.findByRole("region", { name: "Gợi ý của AI" });
    expect(
      await within(box).findByText("AI đang tạm dừng với tài khoản của bạn."),
    ).toBeVisible();
    expect(within(box).getByRole("button", { name: "Gợi ý điểm" })).toBeDisabled();
  });

  it("turns into the off line when the server says the preference is off", async () => {
    serve(() => aiError(403, "AI_READ_OFF"));
    const { user } = renderGrading();
    const box = await screen.findByRole("region", { name: "Gợi ý của AI" });
    await user.click(within(box).getByRole("button", { name: "Gợi ý điểm" }));
    expect(
      await screen.findByRole("link", { name: "Cài đặt › Trợ lý AI" }),
    ).toBeVisible();
    expect(screen.queryByRole("region", { name: "Gợi ý của AI" })).toBeNull();
  });

  it("keeps j and k moving through the queue from the box's buttons", async () => {
    const { user } = renderGrading();
    const box = await screen.findByRole("region", { name: "Gợi ý của AI" });
    within(box).getByRole("button", { name: "Gợi ý điểm" }).focus();
    await user.keyboard("j");
    expect(
      await screen.findByText("Câu trả lời 1/1 của Hà", { exact: false }),
    ).toBeVisible();
    await user.keyboard("k");
    expect(
      await screen.findByText("Câu trả lời 1/1 của Minh", { exact: false }),
    ).toBeVisible();
    expect(graded).toEqual([]);
    expect(suggested).toEqual([]);
  });

  it("is drawn only for a short answer", async () => {
    server.use(
      http.get(`${BASE}/teacher/grading/queue`, () =>
        HttpResponse.json({
          items: [{ ...first, type: "fill_blank" }],
          groups: [],
          answersRemaining: 1,
          studentsWaiting: 1,
        }),
      ),
    );
    renderGrading();
    await card();
    await waitFor(() => expect(calls()).toContain("get /teacher/grading/queue"));
    expect(screen.queryByRole("region", { name: "Gợi ý của AI" })).toBeNull();
  });

  it("imports no grading write: the box reaches the API through features/ai alone", () => {
    const source = readFileSync(SOURCE, "utf8");
    expect(source).not.toMatch(/from "@\/lib\/api\/client"/);
    expect(source).not.toMatch(/^import (?!type )[^;]*from "[^"]*attempts\/api"/m);
    expect(source).not.toMatch(/gradeAttempt|finishGrading/);
  });
});
