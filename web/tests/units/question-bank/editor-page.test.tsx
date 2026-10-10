import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http, HttpResponse } from "msw";
import QuestionEditorPage from "@/features/question-bank/pages/teacher/QuestionEditorPage";
import {
  blockingIssue,
  emptyQuestion,
  type QuestionValues,
} from "@/features/question-bank/questionSchema";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import "@/lib/i18n";

const BASE = "http://localhost:8080";
const ID = "018f0000-0000-7000-8000-0000000000b1";
const COPY = "018f0000-0000-7000-8000-0000000000c1";
const TESTS = [
  "Unit 5 · Nghe",
  "Unit 6 · Đọc",
  "Kiểm tra giữa kỳ",
  "Ôn tập cuối kỳ",
].map((title, index) => ({ id: `018f0000-0000-7000-8000-00000000d00${index}`, title }));

let usedIn = TESTS;
let patches: unknown[] = [];
let deletes = 0;
let duplicates = 0;
let missing = false;

function question(overrides: Record<string, unknown> = {}) {
  return {
    id: ID,
    type: "short_answer" as const,
    level: "b1" as const,
    skill: "listening" as const,
    prompt: "Người phụ nữ đề nghị làm gì?",
    points: 2,
    tags: ["unit-5"],
    usedInTests: usedIn.length,
    usedIn,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  usedIn = TESTS;
  patches = [];
  deletes = 0;
  duplicates = 0;
  missing = false;
  server.use(
    http.get(`${BASE}/teacher/questions/:id`, ({ params }) =>
      missing
        ? contractJson("/teacher/questions/{id}", "get", 404, {
            error: {
              code: "NOT_FOUND",
              message: "Không tìm thấy.",
              requestId: "018f0000-0000-7000-8000-0000000000f4",
            },
          })
        : contractJson(
            "/teacher/questions/{id}",
            "get",
            200,
            question({ id: params["id"] as string }),
          ),
    ),
    http.patch(`${BASE}/teacher/questions/:id`, async ({ request }) => {
      const body = (await request.json()) as Record<string, unknown>;
      patches.push(body);
      return contractJson("/teacher/questions/{id}", "patch", 200, question(body));
    }),
    http.delete(`${BASE}/teacher/questions/:id`, () => {
      deletes += 1;
      return new HttpResponse(null, { status: 204 });
    }),
    http.post(`${BASE}/teacher/questions/:id/duplicate`, () => {
      duplicates += 1;
      return contractJson(
        "/teacher/questions/{id}/duplicate",
        "post",
        201,
        question({ id: COPY, usedInTests: 0, usedIn: [] }),
      );
    }),
  );
});

function renderPage(path = `/teacher/question-bank/${ID}`) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      { path: "/teacher/question-bank", element: <p>Danh sách ngân hàng</p> },
      { path: "/teacher/question-bank/new", element: <QuestionEditorPage /> },
      { path: "/teacher/question-bank/:id", element: <QuestionEditorPage /> },
      { path: "/teacher/tests/:id/edit", element: <p>Trình soạn đề</p> },
    ],
    { initialEntries: [path] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), router };
}

async function loaded() {
  return screen.findByRole("heading", { level: 1, name: "Sửa câu hỏi" });
}

describe("the Question editor's frame", () => {
  it("opens a new question with its crumb, a blocked Save and no usage or menu", async () => {
    renderPage("/teacher/question-bank/new");

    expect(
      screen.getByRole("heading", { level: 1, name: "Câu hỏi mới" }),
    ).toBeVisible();
    expect(screen.getByRole("link", { name: "Ngân hàng câu hỏi" })).toHaveAttribute(
      "href",
      "/teacher/question-bank",
    );
    expect(screen.getByText("Hãy viết nội dung câu hỏi trước.")).toBeVisible();
    expect(screen.getByRole("button", { name: "Lưu" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Thao tác khác" })).toBeNull();
    expect(screen.queryByText(/Đang dùng trong|Chưa đề nào dùng/)).toBeNull();
    expect(screen.getByRole("group", { name: "Loại câu hỏi" })).toBeVisible();
    expect(
      screen.getByRole("complementary", { name: "Cài đặt câu hỏi" }),
    ).toBeVisible();
  });

  it("names a saved question's tests, three at most, and links to their builder", async () => {
    renderPage();
    await loaded();

    expect(
      screen.getByText(
        "Đang dùng trong 4 đề. Lưu thay đổi chỉ ảnh hưởng các đề về sau.",
      ),
    ).toBeVisible();
    const chips = TESTS.slice(0, 3).map((test) =>
      screen.getByRole("link", { name: test.title }),
    );
    expect(chips.map((chip) => chip.getAttribute("href"))).toEqual(
      TESTS.slice(0, 3).map((test) => `/teacher/tests/${test.id}/edit`),
    );
    expect(screen.queryByRole("link", { name: TESTS[3]!.title })).toBeNull();
  });

  it("says when no test uses the question", async () => {
    usedIn = [];
    renderPage();
    await loaded();
    expect(screen.getByText("Chưa đề nào dùng câu hỏi này.")).toBeVisible();
  });

  it("shows the settings and saves only after a change, with the level and skill kept", async () => {
    const { user } = renderPage();
    await loaded();
    const settings = screen.getByRole("complementary", { name: "Cài đặt câu hỏi" });
    expect(
      within(settings).getByRole("combobox", { name: "Trình độ" }),
    ).toHaveTextContent("B1");
    expect(
      within(settings).getByRole("combobox", { name: "Kỹ năng" }),
    ).toHaveTextContent("Nghe");
    const save = screen.getByRole("button", { name: "Lưu" });
    expect(save).toBeDisabled();

    const points = within(settings).getByRole("spinbutton", { name: "Điểm" });
    await user.clear(points);
    expect(screen.getAllByText("Điểm phải lớn hơn 0.").length).toBeGreaterThan(0);
    expect(save).toBeDisabled();
    await user.type(points, "3");
    expect(save).toBeEnabled();
    await user.click(save);

    await waitFor(() => expect(patches).toHaveLength(1));
    expect(patches[0]).toMatchObject({ points: 3, level: "b1", skill: "listening" });
  });

  it("says it cannot find a question that is gone", async () => {
    missing = true;
    renderPage();
    expect(
      await screen.findByText("Không tìm thấy câu hỏi này. Có thể nó đã bị xoá."),
    ).toBeVisible();
    expect(screen.getByRole("link", { name: "Về ngân hàng câu hỏi" })).toHaveAttribute(
      "href",
      "/teacher/question-bank",
    );
  });
});

describe("the Question editor's … menu", () => {
  it("deletes a question no test uses after asking, and returns to the bank", async () => {
    usedIn = [];
    const { user, router } = renderPage();
    await loaded();

    await user.click(screen.getByRole("button", { name: "Thao tác khác" }));
    await user.click(await screen.findByRole("menuitem", { name: "Xoá câu hỏi" }));
    const dialog = await screen.findByRole("dialog", { name: "Xoá câu hỏi này?" });
    expect(
      within(dialog).getByText(
        "Chưa đề nào dùng câu hỏi này. Các đề đã phát hành vẫn giữ bản sao riêng.",
      ),
    ).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: "Xoá" }));

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/question-bank"),
    );
    expect(deletes).toBe(1);
  });

  it("names the draft tests instead of deleting a question they still use", async () => {
    const { user } = renderPage();
    await loaded();

    await user.click(screen.getByRole("button", { name: "Thao tác khác" }));
    await user.click(await screen.findByRole("menuitem", { name: "Xoá câu hỏi" }));
    const dialog = await screen.findByRole("dialog", {
      name: "Chưa xoá được câu hỏi này",
    });
    for (const test of TESTS)
      expect(within(dialog).getByRole("link", { name: test.title })).toBeVisible();
    expect(deletes).toBe(0);
  });

  it("duplicates the question and opens the copy", async () => {
    const { user, router } = renderPage();
    await loaded();

    await user.click(screen.getByRole("button", { name: "Thao tác khác" }));
    await user.click(await screen.findByRole("menuitem", { name: "Nhân bản" }));

    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/teacher/question-bank/${COPY}`),
    );
    expect(duplicates).toBe(1);
  });

  it("asks before duplicating over an unsaved change, and sends nothing on Stay", async () => {
    const { user, router } = renderPage();
    await loaded();
    await user.type(screen.getByRole("spinbutton", { name: "Điểm" }), "5");

    await user.click(screen.getByRole("button", { name: "Thao tác khác" }));
    await user.click(await screen.findByRole("menuitem", { name: "Nhân bản" }));
    let dialog = await screen.findByRole("dialog", { name: "Rời đi mà không lưu?" });
    expect(duplicates).toBe(0);
    await user.click(within(dialog).getByRole("button", { name: "Ở lại" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(duplicates).toBe(0);
    expect(router.state.location.pathname).toBe(`/teacher/question-bank/${ID}`);

    await user.click(screen.getByRole("button", { name: "Thao tác khác" }));
    await user.click(await screen.findByRole("menuitem", { name: "Nhân bản" }));
    dialog = await screen.findByRole("dialog", { name: "Rời đi mà không lưu?" });
    await user.click(within(dialog).getByRole("button", { name: "Rời đi" }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe(`/teacher/question-bank/${COPY}`),
    );
    expect(duplicates).toBe(1);
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("the Question editor's unsaved-change guard", () => {
  it("asks before leaving with an unsaved change, and stays or leaves as told", async () => {
    const { user, router } = renderPage();
    await loaded();
    const points = screen.getByRole("spinbutton", { name: "Điểm" });
    await user.type(points, "5");

    await user.click(screen.getByRole("link", { name: "Ngân hàng câu hỏi" }));
    let dialog = await screen.findByRole("dialog", { name: "Rời đi mà không lưu?" });
    await user.click(within(dialog).getByRole("button", { name: "Ở lại" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(router.state.location.pathname).toBe(`/teacher/question-bank/${ID}`);
    expect(points).toHaveValue(25);

    await user.click(screen.getByRole("link", { name: "Ngân hàng câu hỏi" }));
    dialog = await screen.findByRole("dialog", { name: "Rời đi mà không lưu?" });
    await user.click(within(dialog).getByRole("button", { name: "Rời đi" }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/question-bank"),
    );
  });

  it("leaves without asking when nothing changed", async () => {
    const { user, router } = renderPage();
    await loaded();
    await user.click(screen.getByRole("link", { name: "Ngân hàng câu hỏi" }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/question-bank"),
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});

describe("blockingIssue", () => {
  const choice = (overrides: Partial<QuestionValues>): QuestionValues => ({
    ...emptyQuestion(),
    ...overrides,
  });

  it("names what blocks a save in the deck's order", () => {
    const steps: [QuestionValues, string | null][] = [
      [choice({ points: 0 }), "questionEditor.errors.promptRequired"],
      [choice({ prompt: "Chọn", points: 0 }), "questionEditor.pointsError"],
      [
        choice({ prompt: "Chọn", options: [{ id: null, text: "", isCorrect: false }] }),
        "questionEditor.errors.twoOptions",
      ],
      [choice({ prompt: "Chọn" }), "questionEditor.errors.optionRequired"],
      [
        choice({
          prompt: "Chọn",
          options: [
            { id: null, text: "A", isCorrect: false },
            { id: null, text: "B", isCorrect: false },
          ],
        }),
        "questionEditor.errors.correctRequired",
      ],
      [
        choice({ type: "fill_blank", prompt: "She lives here.", options: [] }),
        "questionEditor.errors.blankRequired",
      ],
      [
        choice({
          type: "fill_blank",
          prompt: "She {{1}} here.",
          options: [],
          blanks: [{ id: null, ordinal: 1, acceptedAnswers: [], caseSensitive: false }],
        }),
        "questionEditor.errors.answerRequired",
      ],
      [
        choice({
          prompt: "Chọn",
          options: [
            { id: null, text: "A", isCorrect: true },
            { id: null, text: "B", isCorrect: false },
          ],
        }),
        null,
      ],
    ];
    expect(
      steps.map(([values]) => blockingIssue(values, "questionEditor.saveFailed")),
    ).toEqual(steps.map(([, key]) => key));
  });
});
