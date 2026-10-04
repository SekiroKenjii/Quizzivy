import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
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
import i18n from "@/lib/i18n";
import { deckSession } from "./deckSession";
import { session, viewport } from "./support";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
}));

const now = "2026-09-01T08:00:00.000Z";
const deadline = "2026-09-01T09:00:00.000Z";
const NAV = "Danh sách câu";
const RAIL_WIDTH = "quizzivy.column.studentNavigator";

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

const twoParts = paper({
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

function long(count: number): AttemptSession {
  return paper({
    questions: Array.from({ length: count }, (_, index) => ({
      ...questions[1]!,
      id: `long-${index + 1}`,
    })),
  });
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

async function open(session: AttemptSession = paper()) {
  vi.mocked(getAttempt).mockResolvedValue(session);
  renderPage();
  await screen.findByRole("main");
}

const store = () => useTakeTestStore.getState();
const onQuestion = (n: number) => screen.queryByRole("main", { name: `Câu ${n}` });
const nav = () => screen.getByRole("navigation", { name: NAV });
const footer = () => within(nav());
const countButton = () => screen.getByRole("button", { name: /^Danh sách câu: / });
const noSheet = () => waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
const squares = (frame: HTMLElement) =>
  within(frame)
    .getAllByRole("button")
    .filter((button) => /^\d+$/.test(button.textContent ?? ""));
const flagDot = (square: HTMLElement | undefined) =>
  square?.querySelector("span[aria-hidden='true']") ?? null;
const panel = (id: string) => document.getElementById(`answer-question-${id}`);

beforeEach(async () => {
  viewport("phone");
  sessionStorage.clear();
  localStorage.clear();
  vi.mocked(getAttempt).mockReset().mockResolvedValue(paper());
  vi.mocked(saveAnswers)
    .mockReset()
    .mockResolvedValue({ serverTime: now, savedAt: now, deadlineAt: deadline });
  vi.mocked(submitAttempt)
    .mockReset()
    .mockResolvedValue(
      session({ serverTime: now, deadlineAt: deadline, status: "submitted" }).attempt,
    );
  store().reset();
  await i18n.changeLanguage("vi");
});
afterEach(async () => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  store().reset();
  await i18n.changeLanguage("vi");
});

describe("the footer below 768", () => {
  it("holds Previous as an icon, the count button and Next, under the question", async () => {
    await open();

    expect(nav().parentElement).toBe(panel("q1")?.closest("section"));
    expect(screen.getByRole("main")).toContainElement(nav());
    expect(nav().previousElementSibling).toContainElement(panel("q1"));
    expect(nav()).toHaveClass(
      "bg-bg",
      "flex-none",
      "items-center",
      "gap-2.5",
      "border-t",
      "pt-2.5",
      "px-3.5",
    );

    const controls = footer().getAllByRole("button");
    expect(controls).toHaveLength(3);
    const [previous, count, next] = controls;
    expect(previous).toHaveAccessibleName("Câu trước");
    expect(previous).toHaveTextContent(/^$/);
    expect(previous).toBeDisabled();
    expect(previous).toHaveClass(
      "h-11",
      "flex-none",
      "rounded-lg",
      "border",
      "bg-card",
      "px-3.5",
      "font-medium",
      "disabled:opacity-40",
    );
    expect(count).toHaveAccessibleName("Danh sách câu: 1 / 3 · đã trả lời 0");
    expect(count).toHaveAttribute("aria-haspopup", "dialog");
    expect(count).toHaveAttribute("aria-expanded", "false");
    expect(count).toHaveClass(
      "h-11",
      "min-w-0",
      "flex-1",
      "gap-2",
      "overflow-hidden",
      "rounded-lg",
      "border",
      "bg-card",
      "font-medium",
    );
    expect(next).toHaveAccessibleName("Câu sau");
    expect(next).toHaveClass(
      "h-11",
      "flex-none",
      "rounded-lg",
      "bg-primary",
      "text-primary-fg",
      "px-4",
      "font-semibold",
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("counts the question on screen and the answers as they change", async () => {
    const user = userEvent.setup();
    await open();
    expect(countButton()).toHaveTextContent("1 / 3 · đã trả lời 0");

    await user.click(screen.getByRole("radio", { name: /Beta/ }));
    expect(countButton()).toHaveTextContent("1 / 3 · đã trả lời 1");

    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    expect(onQuestion(2)).toBeInTheDocument();
    expect(countButton()).toHaveTextContent("2 / 3 · đã trả lời 1");
    expect(footer().getByRole("button", { name: "Câu trước" })).toBeEnabled();

    await user.click(footer().getByRole("button", { name: "Câu trước" }));
    expect(onQuestion(1)).toBeInTheDocument();
  });

  it("reads 4 / 8 with three answered on the deck's paper", async () => {
    const user = userEvent.setup();
    await open(deckSession(new Date(now)));
    for (let moves = 0; moves < 3; moves++) {
      await user.click(footer().getByRole("button", { name: "Câu sau" }));
    }
    expect(countButton()).toHaveAccessibleName("Danh sách câu: 4 / 8 · đã trả lời 3");
  });

  it("turns Next into Finish on the last question, which opens the Submit dialog", async () => {
    const user = userEvent.setup();
    await open();
    expect(footer().queryByRole("button", { name: "Hoàn tất" })).toBeNull();
    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    await user.click(footer().getByRole("button", { name: "Câu sau" }));

    expect(onQuestion(3)).toBeInTheDocument();
    expect(footer().queryByRole("button", { name: "Câu sau" })).toBeNull();
    const finish = footer().getByRole("button", { name: "Hoàn tất" });
    expect(finish).toHaveClass("h-11", "bg-primary", "text-primary-fg");

    await user.click(finish);
    expect(await screen.findByRole("dialog")).toHaveAccessibleName(
      "Nộp bài khi còn 3 câu chưa trả lời?",
    );
    expect(submitAttempt).not.toHaveBeenCalled();
  });

  it("hides with the question while the passage shows", async () => {
    const user = userEvent.setup();
    const base = paper();
    await open({
      ...base,
      groups: [
        {
          id: "g1",
          sectionId: "s1",
          title: "Bài đọc",
          questionIds: ["q1", "q2", "q3"],
          stimuli: [
            {
              id: "st1",
              title: "Bài đọc",
              content: {
                format: "semantic_v1",
                blocks: [
                  {
                    type: "paragraph",
                    content: [{ type: "text", text: "Một đoạn văn.", marks: [] }],
                  },
                ],
              },
              gaps: [],
            },
          ],
          recordings: [],
          assets: [],
        },
      ],
      groupAudioPlays: {},
    });
    expect(nav()).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Ngữ liệu" }));
    expect(screen.queryByRole("navigation", { name: NAV })).toBeNull();
    expect(
      screen.getByRole("navigation", { name: NAV, hidden: true }),
    ).not.toBeVisible();
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

describe("the question sheet", () => {
  it("opens from the count button with the title, the answered count, six columns of 44px squares and the legend", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(screen.getByRole("radio", { name: /Beta/ }));
    const opener = countButton();
    await user.click(opener);

    const dialog = await screen.findByRole("dialog", { name: NAV });
    expect(opener).toHaveAttribute("aria-expanded", "true");
    expect(dialog).toHaveAccessibleDescription("Đã trả lời 1 trên 3");
    expect(dialog).toHaveClass(
      "top-auto",
      "right-0",
      "bottom-0",
      "left-0",
      "max-h-[85svh]",
      "max-w-none",
      "sm:max-w-none",
      "translate-x-0",
      "translate-y-0",
      "flex",
      "flex-col",
      "gap-3.5",
      "rounded-t-3xl",
      "rounded-b-none",
      "border-0",
      "bg-card",
      "px-4",
      "pt-2.5",
      "pb-5",
      "shadow-float",
    );
    expect(dialog.firstElementChild).toHaveClass(
      "bg-border",
      "h-1",
      "w-10",
      "self-center",
      "rounded-full",
    );
    expect(dialog.firstElementChild).toHaveAttribute("aria-hidden", "true");
    expect(within(dialog).getByRole("heading", { level: 2, name: NAV })).toHaveClass(
      "text-md",
      "font-semibold",
    );
    expect(within(dialog).getByText("Đã trả lời 1 trên 3")).toHaveClass(
      "text-muted-fg",
      "text-meta",
    );

    const all = within(dialog).getAllByRole("button");
    expect(all.map((square) => square.textContent)).toEqual(["1", "2", "3"]);
    expect(all[0]).toHaveAccessibleName("Câu 1, đang xem, đã trả lời");
    expect(all[0]).toHaveAttribute("aria-current", "true");
    expect(all[1]).toHaveAccessibleName("Câu 2");
    expect(all[1]).not.toHaveAttribute("aria-current");
    expect(all[2]).toHaveAccessibleName("Câu 3");
    expect(all[0]?.parentElement).toHaveClass("grid", "grid-cols-6", "gap-2");
    for (const square of all) {
      expect(square).toHaveClass(
        "h-11",
        "min-w-0",
        "rounded-ctl",
        "border-[1.5px]",
        "text-base",
        "font-semibold",
      );
    }
    expect(all[0]?.parentElement?.parentElement?.parentElement).toHaveClass(
      "min-h-0",
      "overflow-y-auto",
      "gap-3.5",
    );
    expect(within(dialog).queryByRole("heading", { level: 3 })).toBeNull();

    const answered = within(dialog).getByText("Đã trả lời");
    expect(answered.firstElementChild).toHaveClass(
      "bg-primary",
      "size-3",
      "rounded-[3px]",
    );
    const flagged = within(dialog).getByText("Đã đánh dấu");
    expect(flagged.firstElementChild).toHaveClass(
      "bg-warning",
      "size-2.5",
      "rounded-full",
    );
    expect(answered.parentElement).toBe(flagged.parentElement);
    expect(answered.parentElement).toHaveClass("text-muted-fg", "text-xs", "gap-x-3.5");
    expect(answered.parentElement?.children).toHaveLength(2);
  });

  it("fills an answered square, rings the one on screen with the accent and dots a flagged one", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(screen.getByRole("radio", { name: /Beta/ }));
    await user.click(screen.getByRole("button", { name: "Đánh dấu xem lại" }));
    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    await user.click(countButton());

    const [first, second, third] = squares(await screen.findByRole("dialog"));
    expect(first).toHaveAccessibleName("Câu 1, đã trả lời, đã đánh dấu");
    expect(first).toHaveClass("bg-primary", "text-primary-fg", "border-primary");
    expect(first).not.toHaveClass("border-brand");
    expect(flagDot(first)).toHaveClass(
      "bg-warning",
      "absolute",
      "-top-1",
      "-right-1",
      "size-2.5",
      "rounded-full",
      "border-2",
      "border-card",
    );

    expect(second).toHaveAccessibleName("Câu 2, đang xem");
    expect(second).toHaveClass("bg-card", "text-fg", "border-brand");
    expect(second).not.toHaveClass("border-border");
    expect(flagDot(second)).toBeNull();

    expect(third).toHaveAccessibleName("Câu 3");
    expect(third).toHaveClass("bg-card", "text-fg", "border-border");
    expect(third).not.toHaveClass("bg-primary");
    expect(flagDot(third)).toBeNull();
  });

  it("keeps the accent border on the open question once it is answered", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(screen.getByRole("radio", { name: /Alpha/ }));
    await user.click(countButton());

    const [first] = squares(await screen.findByRole("dialog"));
    expect(first).toHaveClass("bg-primary", "text-primary-fg", "border-brand");
    expect(first).not.toHaveClass("border-primary");
  });

  it("goes to a question from its square, closes, and lands on that question", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(countButton());
    await user.click(within(await screen.findByRole("dialog")).getByText("3"));

    await noSheet();
    expect(onQuestion(3)).toBeInTheDocument();
    expect(panel("q3")).toHaveFocus();
    expect(countButton()).toHaveTextContent("3 / 3 · đã trả lời 0");
    expect(countButton()).toHaveAttribute("aria-expanded", "false");
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(panel("q3")).toHaveFocus();
  });

  it("closes from the open question's own square without moving", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    await user.click(countButton());
    await user.click(
      within(await screen.findByRole("dialog")).getByRole("button", {
        name: "Câu 2, đang xem",
      }),
    );

    await noSheet();
    expect(onQuestion(2)).toBeInTheDocument();
    expect(panel("q2")).toHaveFocus();
  });

  it("starts on the open question's square and keeps the focus inside", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    await user.click(countButton());

    const dialog = await screen.findByRole("dialog");
    const [first, second, third] = squares(dialog);
    await waitFor(() => expect(second).toHaveFocus());
    await user.tab();
    expect(third).toHaveFocus();
    await user.tab();
    expect(first).toHaveFocus();
    await user.tab({ shift: true });
    expect(third).toHaveFocus();
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
  });

  it("closes on Esc and gives the focus back to the button that opened it", async () => {
    const user = userEvent.setup();
    await open();
    const opener = countButton();
    await user.click(opener);
    await screen.findByRole("dialog");

    await user.keyboard("{Escape}");
    await noSheet();
    await waitFor(() => expect(opener).toHaveFocus());
    expect(onQuestion(1)).toBeInTheDocument();
    expect(opener).toHaveAttribute("aria-expanded", "false");
  });

  it("closes on the backdrop and gives the focus back", async () => {
    const user = userEvent.setup();
    await open();
    const opener = countButton();
    await user.click(opener);
    await screen.findByRole("dialog");
    const backdrop = document.querySelector<HTMLElement>(
      "[data-slot='dialog-overlay']",
    );
    expect(backdrop).toHaveClass("bg-overlay", "fixed", "inset-0");

    await user.click(backdrop!);
    await noSheet();
    await waitFor(() => expect(opener).toHaveFocus());
    expect(onQuestion(1)).toBeInTheDocument();
  });

  it("has only the squares: no close button and no way to hand the paper in", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(countButton());
    const dialog = await screen.findByRole("dialog");

    expect(within(dialog).getAllByRole("button")).toHaveLength(3);
    expect(
      within(dialog).queryByRole("button", { name: /nộp|đóng|hoàn tất/i }),
    ).toBeNull();
  });

  it("heads each part and names the part in each square on a paper of several parts", async () => {
    const user = userEvent.setup();
    await open(twoParts);
    await user.click(countButton());
    const dialog = await screen.findByRole("dialog");

    const headings = within(dialog).getAllByRole("heading", { level: 3 });
    expect(headings.map((heading) => heading.textContent)).toEqual([
      "Phần 1 · Ngữ pháp",
      "Phần 2 · Nghe",
    ]);
    expect(headings[0]).toHaveClass(
      "text-muted-fg",
      "text-meta",
      "font-semibold",
      "uppercase",
    );
    const grids = headings.map((heading) => heading.nextElementSibling as HTMLElement);
    expect(grids[0]).toHaveClass("grid-cols-6");
    expect(squares(grids[0]!).map((square) => square.textContent)).toEqual(["1", "2"]);
    expect(squares(grids[1]!).map((square) => square.textContent)).toEqual(["3"]);
    expect(squares(dialog).map((square) => square.getAttribute("aria-label"))).toEqual([
      "Câu 1, Phần 1 · Ngữ pháp, đang xem",
      "Câu 2, Phần 1 · Ngữ pháp",
      "Câu 3, Phần 2 · Nghe",
    ]);

    await user.click(squares(grids[1]!)[0]!);
    await noSheet();
    expect(onQuestion(3)).toBeInTheDocument();
  });

  it("gives a part with no title its squares and no heading", async () => {
    const user = userEvent.setup();
    await open(
      paper({
        sections: [{ id: "s1", title: "Phần 1", instructions: null }],
        questions: [...questions.slice(0, 2), { ...questions[2]!, sectionId: "lost" }],
      }),
    );
    await user.click(countButton());
    const dialog = await screen.findByRole("dialog");

    expect(
      within(dialog)
        .getAllByRole("heading", { level: 3 })
        .map((heading) => heading.textContent),
    ).toEqual(["Phần 1"]);
    expect(squares(dialog).map((square) => square.getAttribute("aria-label"))).toEqual([
      "Câu 1, Phần 1, đang xem",
      "Câu 2, Phần 1",
      "Câu 3",
    ]);
  });

  it("scrolls the squares inside itself on a long paper", async () => {
    const user = userEvent.setup();
    await open(long(80));
    expect(countButton()).toHaveTextContent("1 / 80 · đã trả lời 0");
    await user.click(countButton());
    const dialog = await screen.findByRole("dialog");

    expect(squares(dialog)).toHaveLength(80);
    expect(dialog).toHaveAccessibleDescription("Đã trả lời 0 trên 80");
    expect(dialog).toHaveClass("max-h-[85svh]");
    expect(squares(dialog)[0]?.closest(".overflow-y-auto")).toHaveClass(
      "-m-1",
      "p-1",
      "min-h-0",
    );
  });

  it("goes away when the window grows past 768, where the strip takes over", async () => {
    const view = viewport("phone");
    const user = userEvent.setup();
    await open();
    await user.click(countButton());
    await screen.findByRole("dialog");

    act(() => view.resize("desktop"));
    await noSheet();
    expect(squares(nav())).toHaveLength(3);
    expect(screen.queryByRole("button", { name: /^Danh sách câu: / })).toBeNull();
  });
});

describe("the footer from 768", () => {
  beforeEach(() => viewport("desktop"));

  it("holds Previous with its label, the strip of 34px squares and Next, under the question", async () => {
    await open();

    expect(nav().parentElement).toBe(panel("q1")?.closest("section"));
    expect(nav()).toHaveClass("bg-bg", "flex", "gap-2.5", "border-t", "pt-2.5", "px-6");
    expect(screen.getByRole("main")).toHaveClass(
      "flex",
      "min-h-0",
      "min-w-0",
      "flex-1",
    );
    expect(nav()).not.toHaveClass("px-3.5");

    const controls = footer().getAllByRole("button");
    expect(controls.map((control) => control.textContent)).toEqual([
      "Câu trước",
      "1",
      "2",
      "3",
      "Câu sau",
    ]);
    expect(controls[0]).toHaveAccessibleName("Câu trước");
    expect(controls[0]).toBeDisabled();
    expect(controls[0]).toHaveClass(
      "h-11",
      "rounded-lg",
      "border",
      "bg-card",
      "px-3.5",
    );
    expect(controls[4]).toHaveClass("h-11", "rounded-lg", "bg-primary", "px-4");

    const strip = controls[1]?.parentElement;
    expect(strip).toHaveClass(
      "-m-1",
      "p-1",
      "min-w-0",
      "flex-1",
      "flex-wrap",
      "justify-center",
      "gap-[5px]",
    );
    for (const square of squares(nav())) {
      expect(square).toHaveClass(
        "size-8.5",
        "min-h-0",
        "min-w-0",
        "flex-none",
        "rounded-md",
        "border-[1.5px]",
        "text-sm",
        "font-semibold",
        "tabular-nums",
      );
      expect(square).not.toHaveClass("ml-3");
    }
    expect(screen.queryByRole("button", { name: /^Danh sách câu: / })).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("keeps the title, the save line and the timer in the header, and draws no rail and no shortcut hint", async () => {
    await open();

    const header = within(screen.getByRole("banner"));
    expect(
      header.getByText("Unit 5 — Present perfect & listening"),
    ).toBeInTheDocument();
    expect(header.getByText("Đã lưu tất cả câu trả lời")).toBeInTheDocument();
    expect(
      header.getByRole("timer", { name: "Thời gian còn lại" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Câu 1 trên 3 · Chọn một đáp án")).toBeInTheDocument();

    expect(screen.queryByRole("complementary")).toBeNull();
    expect(screen.queryByRole("separator")).toBeNull();
    expect(document.querySelector("[data-side-column]")).toBeNull();
    expect(screen.queryByText(/Phím tắt/)).toBeNull();
    expect(screen.queryByRole("button", { name: "Xem lại & nộp" })).toBeNull();
    expect(panel("q1")?.querySelectorAll("button")).toHaveLength(1);
  });

  it("fills an answered square, rings the one on screen with the accent and dots a flagged one", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(screen.getByRole("radio", { name: /Beta/ }));
    await user.click(screen.getByRole("button", { name: "Đánh dấu xem lại" }));
    await user.click(footer().getByRole("button", { name: "Câu sau" }));

    const [first, second, third] = squares(nav());
    expect(first).toHaveAccessibleName("Câu 1, đã trả lời, đã đánh dấu");
    expect(first).not.toHaveAttribute("aria-current");
    expect(first).toHaveClass("bg-primary", "text-primary-fg", "border-primary");
    expect(flagDot(first)).toHaveClass(
      "bg-warning",
      "-top-1",
      "-right-1",
      "size-2.5",
      "rounded-full",
      "border-2",
      "border-bg",
    );

    expect(second).toHaveAccessibleName("Câu 2, đang xem");
    expect(second).toHaveAttribute("aria-current", "true");
    expect(second).toHaveClass("bg-card", "text-fg", "border-brand");
    expect(flagDot(second)).toBeNull();

    expect(third).toHaveAccessibleName("Câu 3");
    expect(third).toHaveClass("bg-card", "text-fg", "border-border");

    await user.click(footer().getByRole("button", { name: "Câu trước" }));
    await user.click(screen.getByRole("button", { name: "Đã đánh dấu" }));
    expect(flagDot(squares(nav())[0])).toBeNull();
    expect(squares(nav())[0]).toHaveAccessibleName("Câu 1, đang xem, đã trả lời");
    expect(squares(nav())[0]).toHaveClass("bg-primary", "border-brand");
  });

  it("goes to a question from its square and lands on it", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(footer().getByRole("button", { name: "Câu 3" }));

    expect(onQuestion(3)).toBeInTheDocument();
    expect(panel("q3")).toHaveFocus();
    expect(footer().getByRole("button", { name: "Câu 3, đang xem" })).toHaveAttribute(
      "aria-current",
      "true",
    );
    expect(footer().getByRole("button", { name: "Câu 1" })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("turns Next into Finish on the last question, which opens the Submit dialog the header's Submit opens", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(footer().getByRole("button", { name: "Câu 3" }));

    expect(footer().queryByRole("button", { name: "Câu sau" })).toBeNull();
    await user.click(footer().getByRole("button", { name: "Hoàn tất" }));
    const fromFinish = await screen.findByRole("dialog");
    expect(fromFinish).toHaveAccessibleName("Nộp bài khi còn 3 câu chưa trả lời?");
    expect(
      screen.getByRole("navigation", { name: NAV, hidden: true }),
    ).toBeInTheDocument();

    await user.click(
      within(fromFinish).getByRole("button", { name: "Quay lại làm tiếp" }),
    );
    await noSheet();
    await user.click(
      within(screen.getByRole("banner")).getByRole("button", { name: "Nộp bài" }),
    );
    expect(await screen.findByRole("dialog")).toHaveAccessibleName(
      "Nộp bài khi còn 3 câu chưa trả lời?",
    );
  });

  it("leaves a gap between parts, names the part in each square, and states a part's instructions on its first question", async () => {
    const user = userEvent.setup();
    await open(twoParts);

    const [first, second, third] = squares(nav());
    expect(first).toHaveAccessibleName("Câu 1, Phần 1 · Ngữ pháp, đang xem");
    expect(second).toHaveAccessibleName("Câu 2, Phần 1 · Ngữ pháp");
    expect(third).toHaveAccessibleName("Câu 3, Phần 2 · Nghe");
    expect(first).not.toHaveClass("ml-3");
    expect(second).not.toHaveClass("ml-3");
    expect(third).toHaveClass("ml-3", "size-8.5");
    expect(footer().queryByRole("heading")).toBeNull();

    const pane = () => within(screen.getByRole("main"));
    expect(pane().getByText("Phần 1 · Ngữ pháp")).toBeInTheDocument();
    expect(pane().getByText("Câu 1 trên 3 · Chọn một đáp án")).toBeInTheDocument();
    expect(screen.queryByRole("note")).toBeNull();

    await user.click(third!);
    expect(pane().getByText("Phần 2 · Nghe")).toBeInTheDocument();
    expect(pane().getByText("Câu 3 trên 3 · Đúng hay sai")).toBeInTheDocument();
    expect(screen.getByRole("note")).toHaveTextContent(
      "Nghe đoạn hội thoại rồi trả lời.",
    );
  });

  it("puts the gap before the first square of a later part only", async () => {
    await open(
      paper({
        sections: [
          { id: "s1", title: "Phần 1", instructions: null },
          { id: "s2", title: "Phần 2", instructions: null },
        ],
        questions: [
          questions[0]!,
          { ...questions[1]!, sectionId: "s2" },
          { ...questions[2]!, sectionId: "s2" },
        ],
      }),
    );

    const [first, second, third] = squares(nav());
    expect(first).not.toHaveClass("ml-3");
    expect(second).toHaveClass("ml-3");
    expect(second).toHaveAccessibleName("Câu 2, Phần 2");
    expect(third).not.toHaveClass("ml-3");
    expect(third).toHaveAccessibleName("Câu 3, Phần 2");
  });

  it("names no part in a square on a paper of one part", async () => {
    await open();
    expect(squares(nav()).map((square) => square.getAttribute("aria-label"))).toEqual([
      "Câu 1, đang xem",
      "Câu 2",
      "Câu 3",
    ]);
  });

  it("keeps a long paper's strip within four rows and brings the open question into view", async () => {
    const seen: Element[] = [];
    const scrollIntoView = vi
      .spyOn(Element.prototype, "scrollIntoView")
      .mockImplementation(function (this: Element) {
        seen.push(this);
      });
    const user = userEvent.setup();
    await open(long(40));

    const strip = squares(nav())[0]?.parentElement;
    expect(squares(nav())).toHaveLength(40);
    expect(strip).toHaveClass("max-h-40", "overflow-y-auto", "flex-wrap");
    expect(seen.at(-1)).toBe(squares(nav())[0]);
    expect(scrollIntoView).toHaveBeenLastCalledWith({ block: "nearest" });

    await user.click(footer().getByRole("button", { name: "Câu 37" }));
    expect(seen.at(-1)).toBe(squares(nav())[36]);
    expect(seen.at(-1)).toHaveAttribute("aria-current", "true");
  });

  it("swaps the strip for the count button when the window shrinks below 768", async () => {
    const view = viewport("desktop");
    await open();
    expect(squares(nav())).toHaveLength(3);

    act(() => view.resize("phone"));
    expect(squares(nav())).toHaveLength(0);
    expect(countButton()).toHaveTextContent("1 / 3 · đã trả lời 0");
    expect(footer().getByRole("button", { name: "Câu trước" })).toHaveTextContent(/^$/);
  });
});

describe.each(["superseded", "closed"] as const)(
  "on a paper whose lock is %s",
  (lock) => {
    it("offers no Finish on the last question, and nothing on the page opens the Submit dialog", async () => {
      viewport("desktop");
      const user = userEvent.setup();
      await open();
      await user.click(footer().getByRole("button", { name: "Câu 3" }));
      expect(footer().getByRole("button", { name: "Hoàn tất" })).toBeInTheDocument();

      act(() => store().lockNow(lock));
      expect(store().lock).toBe(lock);
      expect(onQuestion(3)).toBeInTheDocument();
      expect(footer().queryByRole("button", { name: "Hoàn tất" })).toBeNull();
      expect(footer().queryByRole("button", { name: "Câu sau" })).toBeNull();
      expect(
        footer()
          .getAllByRole("button")
          .map((control) => control.textContent),
      ).toEqual(["Câu trước", "1", "2", "3"]);
      expect(
        within(screen.getByRole("banner")).queryByRole("button", { name: "Nộp bài" }),
      ).toBeNull();
      expect(screen.queryByRole("button", { name: /nộp|hoàn tất/i })).toBeNull();

      await user.keyboard("{ArrowRight}");
      await user.keyboard("{Enter}");
      expect(screen.queryByRole("dialog")).toBeNull();

      await user.click(footer().getByRole("button", { name: "Câu trước" }));
      expect(onQuestion(2)).toBeInTheDocument();
      expect(footer().getByRole("button", { name: "Câu sau" })).toBeInTheDocument();
      expect(submitAttempt).not.toHaveBeenCalled();
    });

    it("offers none on a phone either, where the sheet still moves between questions", async () => {
      const user = userEvent.setup();
      await open();
      act(() => store().lockNow(lock));
      await user.click(countButton());
      await user.click(within(await screen.findByRole("dialog")).getByText("3"));
      await noSheet();

      expect(onQuestion(3)).toBeInTheDocument();
      expect(footer().getAllByRole("button")).toHaveLength(2);
      expect(screen.queryByRole("button", { name: /nộp|hoàn tất/i })).toBeNull();
    });
  },
);

describe("on a paper whose time is up", () => {
  it("keeps Finish, as the header keeps Submit", async () => {
    viewport("desktop");
    const user = userEvent.setup();
    await open();
    await user.click(footer().getByRole("button", { name: "Câu 3" }));
    act(() => store().lockNow("deadline"));

    expect(
      within(screen.getByRole("banner")).getByRole("button", { name: "Nộp bài" }),
    ).toBeInTheDocument();
    await user.click(footer().getByRole("button", { name: "Hoàn tất" }));
    expect(await screen.findByRole("dialog")).toHaveAccessibleName(
      "Nộp bài khi còn 3 câu chưa trả lời?",
    );
  });
});

describe("the footer in English", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("is the deck's wording from 768", async () => {
    viewport("desktop");
    const user = userEvent.setup();
    await open(deckSession(new Date(now)));
    const bar = within(screen.getByRole("navigation", { name: "Questions" }));

    expect(bar.getByRole("button", { name: "Previous question" })).toHaveTextContent(
      /^Previous$/,
    );
    expect(bar.getByRole("button", { name: "Next" })).toBeInTheDocument();
    expect(bar.getByRole("button", { name: "Question 1, current, answered" })).toBe(
      squares(screen.getByRole("navigation"))[0],
    );
    expect(bar.getByRole("button", { name: "Question 4" })).toBeInTheDocument();

    await user.click(bar.getByRole("button", { name: "Question 8" }));
    expect(bar.queryByRole("button", { name: "Next" })).toBeNull();
    expect(bar.getByRole("button", { name: "Finish" })).toBeInTheDocument();
  });

  it("is the deck's wording below 768, in the button and in the sheet", async () => {
    const user = userEvent.setup();
    await open(deckSession(new Date(now)));
    for (let moves = 0; moves < 3; moves++) {
      await user.click(screen.getByRole("button", { name: "Next" }));
    }
    const count = screen.getByRole("button", { name: /^Questions: / });
    expect(count).toHaveTextContent("4 / 8 · 3 answered");
    expect(count).toHaveAccessibleName("Questions: 4 / 8 · 3 answered");

    await user.click(count);
    const dialog = await screen.findByRole("dialog", { name: "Questions" });
    expect(dialog).toHaveAccessibleDescription("3 of 8 answered");
    expect(within(dialog).getByText("Answered")).toBeInTheDocument();
    expect(within(dialog).getByText("Flagged")).toBeInTheDocument();
    await waitFor(() =>
      expect(
        within(dialog).getByRole("button", { name: "Question 4, current" }),
      ).toHaveFocus(),
    );
  });
});

describe("the retired rail's remembered width", () => {
  it("is deleted from this browser when the engine opens", async () => {
    localStorage.setItem(RAIL_WIDTH, "272");
    localStorage.setItem("quizzivy.column.panel", "512");
    await open();

    expect(localStorage.getItem(RAIL_WIDTH)).toBeNull();
    expect(localStorage.getItem("quizzivy.column.panel")).toBe("512");
  });

  it("costs nothing in a browser that refuses storage", async () => {
    const remove = Storage.prototype.removeItem;
    const removeItem = vi
      .spyOn(Storage.prototype, "removeItem")
      .mockImplementation(function (this: Storage, key: string) {
        if (key === RAIL_WIDTH) throw new DOMException("denied", "SecurityError");
        remove.call(this, key);
      });
    await open();

    expect(removeItem).toHaveBeenCalledWith(RAIL_WIDTH);
    expect(removeItem.mock.results.some((result) => result.type === "throw")).toBe(
      true,
    );
    expect(nav()).toBeInTheDocument();
    expect(onQuestion(1)).toBeInTheDocument();
  });
});
