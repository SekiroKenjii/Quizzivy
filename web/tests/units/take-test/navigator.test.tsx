import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import {
  getAttempt,
  saveAnswers,
  submitAttempt,
  type AttemptSession,
  type StudentQuestion,
} from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { session, viewport } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
}));

const now = "2026-09-01T08:00:00.000Z";
const deadline = "2026-09-01T09:00:00.000Z";

const questions: StudentQuestion[] = [
  {
    id: "q1",
    sectionId: "s1",
    type: "single_choice",
    prompt: "Pick one",
    points: 1,
    options: [
      { id: "o1", text: "Alpha" },
      { id: "o2", text: "Beta" },
    ],
  },
  { id: "q2", sectionId: "s1", type: "short_answer", prompt: "Write", points: 1 },
  { id: "q3", sectionId: "s1", type: "true_false", prompt: "True?", points: 1 },
];

function paper(over: Partial<AttemptSession> = {}): AttemptSession {
  return { ...session({ serverTime: now, deadlineAt: deadline }), questions, ...over };
}

function renderPage() {
  const router = createMemoryRouter(
    [
      { path: "/app/attempts/:attemptId", element: <TakeTestPage /> },
      { path: "/app", element: <p>home</p> },
      { path: "/app/attempts/:attemptId/result", element: <p>result page</p> },
    ],
    { initialEntries: ["/app/attempts/att-1"] },
  );
  render(<RouterProvider router={router} />);
  return router;
}

const onQuestion = (n: number) => screen.queryByRole("main", { name: `Câu ${n}` });
const footer = () => within(screen.getByRole("contentinfo"));

beforeEach(() => {
  viewport("phone");
  sessionStorage.clear();
  vi.mocked(getAttempt).mockReset().mockResolvedValue(paper());
  vi.mocked(saveAnswers)
    .mockReset()
    .mockResolvedValue({ serverTime: now, savedAt: now, deadlineAt: deadline });
  vi.mocked(submitAttempt)
    .mockReset()
    .mockResolvedValue(
      session({ serverTime: now, deadlineAt: deadline, status: "submitted" }).attempt,
    );
  useTakeTestStore.getState().reset();
});
afterEach(() => {
  vi.unstubAllGlobals();
  useTakeTestStore.getState().reset();
});

describe("the navigator", () => {
  it("opens from the footer, shows each question's state, and jumps", async () => {
    const user = userEvent.setup();
    renderPage();
    await user.click(await screen.findByRole("radio", { name: /Beta/ }));

    await user.click(screen.getByRole("button", { name: "Danh sách câu" }));
    const sheet = await screen.findByRole("dialog");
    expect(
      within(sheet).getByRole("button", { name: "Câu 1, đang xem, đã trả lời" }),
    ).toHaveAttribute("aria-current", "true");
    expect(within(sheet).getByRole("button", { name: "Câu 2" })).toBeInTheDocument();

    await user.click(within(sheet).getByRole("button", { name: "Câu 3" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(onQuestion(3)).toBeInTheDocument();
  });

  it("marks a flagged question, in the grid and on the button", async () => {
    const user = userEvent.setup();
    renderPage();
    const flag = await screen.findByRole("button", { name: "Đánh dấu xem lại" });
    await user.click(flag);
    expect(screen.getByRole("button", { name: "Đã đánh dấu" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );

    await user.click(screen.getByRole("button", { name: "Danh sách câu" }));
    expect(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Câu 1, đang xem, đã đánh dấu",
      }),
    ).toBeInTheDocument();
  });

  it("turns the last question's next button into review", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("radio", { name: /Alpha/ });
    expect(footer().queryByRole("button", { name: "Xem lại & nộp" })).toBeNull();
    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    expect(footer().getByRole("button", { name: "Xem lại & nộp" })).toBeInTheDocument();
    expect(footer().queryByRole("button", { name: "Câu sau" })).toBeNull();
  });
});

describe("shortcuts", () => {
  it("move, flag and pick when nothing is being typed", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("radio", { name: /Alpha/ });

    await user.keyboard("b");
    expect(screen.getByRole("radio", { name: /Beta/ })).toBeChecked();
    await user.keyboard("f");
    expect(screen.getByRole("button", { name: "Đã đánh dấu" })).toBeInTheDocument();
    await user.keyboard("{ArrowRight}");
    expect(onQuestion(2)).toBeInTheDocument();
    await user.keyboard("{ArrowLeft}");
    expect(onQuestion(1)).toBeInTheDocument();
  });

  it("stay out of the way while the student is typing", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("radio", { name: /Alpha/ });
    await user.click(screen.getByRole("button", { name: "Câu sau" }));
    await user.type(screen.getByRole("textbox"), "f");
    expect(screen.getByRole("textbox")).toHaveValue("f");
    expect(onQuestion(2)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Đánh dấu xem lại" }),
    ).toBeInTheDocument();
  });
});

describe("from 768px", () => {
  const listening = paper({
    sections: [
      { id: "s1", title: "Phần 1 · Ngữ pháp", instructions: null },
      {
        id: "s2",
        title: "Phần 2 · Nghe",
        instructions: "Nghe đoạn hội thoại rồi trả lời.",
      },
    ],
    questions: [...questions.slice(0, 2), { ...questions[2]!, sectionId: "s2" }],
  });

  beforeEach(() => viewport("desktop"));

  it("puts the title, the save state and the clock in one row, and drops the footer", async () => {
    renderPage();
    await screen.findByRole("radio", { name: /Alpha/ });

    const header = within(screen.getByRole("banner"));
    expect(
      header.getByText("Unit 5 — Present perfect & listening"),
    ).toBeInTheDocument();
    expect(header.getByText("Đã lưu tất cả câu trả lời")).toBeInTheDocument();
    expect(
      header.getByRole("timer", { name: "Thời gian còn lại" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("contentinfo")).toBeNull();
    expect(screen.queryByRole("button", { name: "Danh sách câu" })).toBeNull();

    expect(screen.getByText("Câu 1 trên 3 · Chọn một đáp án")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Câu trước" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Câu sau" })).toHaveTextContent(
      "Câu sau",
    );
    expect(screen.getByText(/Phím tắt/)).toBeInTheDocument();
  });

  it("groups the rail by part and states a part's instructions on its first question", async () => {
    vi.mocked(getAttempt).mockResolvedValue(listening);
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("radio", { name: /Alpha/ });

    const rail = within(screen.getByRole("complementary", { name: "Danh sách câu" }));
    expect(rail.getByText("Phần 1 · Ngữ pháp")).toBeInTheDocument();
    expect(rail.getByText("Phần 2 · Nghe")).toBeInTheDocument();
    const paper = () => within(screen.getByRole("main"));
    expect(paper().getByText("Phần 1 · Ngữ pháp")).toBeInTheDocument();
    expect(paper().getByText("Câu 1 trên 3 · Chọn một đáp án")).toBeInTheDocument();
    expect(screen.queryByRole("note")).toBeNull();

    await user.click(rail.getByRole("button", { name: "Câu 3" }));
    expect(paper().getByText("Phần 2 · Nghe")).toBeInTheDocument();
    expect(paper().getByText("Câu 3 trên 3 · Đúng hay sai")).toBeInTheDocument();
    expect(screen.getByRole("note")).toHaveTextContent(
      "Nghe đoạn hội thoại rồi trả lời.",
    );
  });
});
