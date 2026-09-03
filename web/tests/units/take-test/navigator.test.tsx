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
import { session } from "./support";
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
    type: "single_choice",
    prompt: "Pick one",
    points: 1,
    options: [
      { id: "o1", text: "Alpha" },
      { id: "o2", text: "Beta" },
    ],
  },
  { id: "q2", type: "short_answer", prompt: "Write", points: 1 },
  { id: "q3", type: "true_false", prompt: "True?", points: 1 },
];

function paper(over: Partial<AttemptSession> = {}): AttemptSession {
  return { ...session({ serverTime: now, deadlineAt: deadline }), questions, ...over };
}

function renderPage() {
  const router = createMemoryRouter(
    [
      { path: "/app/attempts/:attemptId", element: <TakeTestPage /> },
      { path: "/app", element: <p>home</p> },
    ],
    { initialEntries: ["/app/attempts/att-1"] },
  );
  render(<RouterProvider router={router} />);
  return router;
}

const counter = () => screen.getAllByText(/^Câu \d\/3$/)[0]!;
// The rail (S-08) is display:none below 1024px, which jsdom cannot see, so
// the phone's footer is addressed by its landmark.
const footer = () => within(screen.getByRole("contentinfo"));

beforeEach(() => {
  sessionStorage.clear();
  vi.mocked(getAttempt).mockReset().mockResolvedValue(paper());
  vi.mocked(saveAnswers)
    .mockReset()
    .mockResolvedValue({ serverTime: now, savedAt: now });
  vi.mocked(submitAttempt)
    .mockReset()
    .mockResolvedValue(
      session({ serverTime: now, deadlineAt: deadline, status: "submitted" }).attempt,
    );
  useTakeTestStore.getState().reset();
});
afterEach(() => useTakeTestStore.getState().reset());

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
    expect(counter()).toHaveTextContent("Câu 3/3");
  });

  it("marks a flagged question, in the grid and on the button", async () => {
    const user = userEvent.setup();
    renderPage();
    const flag = await screen.findByRole("button", { name: "Đánh dấu câu này" });
    await user.click(flag);
    expect(screen.getByRole("button", { name: "Bỏ đánh dấu câu này" })).toHaveAttribute(
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
    expect(
      screen.getByRole("button", { name: "Bỏ đánh dấu câu này" }),
    ).toBeInTheDocument();
    await user.keyboard("{ArrowRight}");
    expect(counter()).toHaveTextContent("Câu 2/3");
    await user.keyboard("{ArrowLeft}");
    expect(counter()).toHaveTextContent("Câu 1/3");
  });

  it("stay out of the way while the student is typing", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("radio", { name: /Alpha/ });
    await user.click(screen.getByRole("button", { name: "Câu sau" }));
    await user.type(screen.getByRole("textbox"), "f");
    expect(screen.getByRole("textbox")).toHaveValue("f");
    expect(counter()).toHaveTextContent("Câu 2/3");
    expect(
      screen.getByRole("button", { name: "Đánh dấu câu này" }),
    ).toBeInTheDocument();
  });
});

describe("review and submit", () => {
  it("counts what is empty, jumps back to it, and says the number before submitting", async () => {
    const user = userEvent.setup();
    const router = renderPage();
    await user.click(await screen.findByRole("radio", { name: /Beta/ }));
    await user.click(screen.getByRole("button", { name: "Danh sách câu" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Xem lại & nộp",
      }),
    );

    expect(
      await screen.findByRole("heading", { name: "Xem lại trước khi nộp" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Bạn đã trả lời 1 / 3 câu.")).toBeInTheDocument();
    expect(screen.getByText("2 câu chưa trả lời")).toBeInTheDocument();

    // A dot is a way back to the question it names.
    await user.click(screen.getByRole("button", { name: "Câu 3" }));
    expect(counter()).toHaveTextContent("Câu 3/3");
    await user.click(footer().getByRole("button", { name: "Xem lại & nộp" }));

    await user.click(await screen.findByRole("button", { name: "Nộp bài" }));
    const confirm = await screen.findByRole("dialog");
    expect(confirm).toHaveTextContent(
      "Còn 2 câu bạn chưa trả lời. Sau khi nộp, bạn không sửa được nữa.",
    );

    await user.click(within(confirm).getByRole("button", { name: "Nộp bài" }));
    await waitFor(() =>
      expect(submitAttempt).toHaveBeenCalledWith("att-1", { reason: "manual" }),
    );
    await waitFor(() => expect(router.state.location.pathname).toBe("/app"));
  });

  it("lets the student back out at both steps", async () => {
    const user = userEvent.setup();
    renderPage();
    await screen.findByRole("radio", { name: /Alpha/ });
    await user.click(screen.getByRole("button", { name: "Danh sách câu" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Xem lại & nộp",
      }),
    );

    await user.click(await screen.findByRole("button", { name: "Nộp bài" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Quay lại",
      }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());

    await user.click(screen.getByRole("button", { name: "Quay lại làm tiếp" }));
    expect(await screen.findByRole("radio", { name: /Alpha/ })).toBeInTheDocument();
    expect(submitAttempt).not.toHaveBeenCalled();
  });

  it("says when the paper is fully answered", async () => {
    const user = userEvent.setup();
    vi.mocked(getAttempt).mockResolvedValue(
      paper({
        answers: {
          q1: { type: "choice", optionIds: ["o1"] },
          q2: { type: "text", value: "done" },
          q3: { type: "true_false", value: true },
        },
      }),
    );
    renderPage();
    await screen.findByRole("radio", { name: /Alpha/ });
    await user.click(screen.getByRole("button", { name: "Danh sách câu" }));
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Xem lại & nộp",
      }),
    );
    expect(
      await screen.findByText("Bạn đã trả lời tất cả các câu."),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Nộp bài" }));
    expect(await screen.findByRole("dialog")).toHaveTextContent(
      "Sau khi nộp, bạn không sửa được nữa.",
    );
    expect(screen.getByRole("dialog")).not.toHaveTextContent("chưa trả lời");
  });
});
