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

const fillIn: StudentQuestion = {
  id: "q2",
  sectionId: "s1",
  type: "fill_blank",
  prompt: "She {{1}} home and {{2}} dinner.",
  points: 1,
  blanks: [
    { id: "b1", ordinal: 1, caseSensitive: false },
    { id: "b2", ordinal: 2, caseSensitive: false },
  ],
};

function withPassage(base: AttemptSession, questionIds: string[]): AttemptSession {
  return {
    ...base,
    groups: [
      {
        id: "g1",
        sectionId: "s1",
        title: "Bài đọc",
        questionIds,
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
  };
}

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
const gridButton = () => screen.getByRole("button", { name: /^Câu \d+ · đã làm / });
const rail = () => screen.getByRole("complementary", { name: "Các phần" });
const noRail = () => expect(screen.queryByRole("complementary")).toBeNull();
const noSheet = () => waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
const squares = (frame: HTMLElement) =>
  within(frame)
    .getAllByRole("button")
    .filter((button) => /^\d+$/.test(button.textContent ?? ""));
const hint = () => nav().querySelector("p");
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
  it("holds Previous as an icon, the grid button and Next, under the question", async () => {
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
      "pb-2.5",
      "px-3.5",
    );
    noRail();

    const controls = footer().getAllByRole("button");
    expect(controls).toHaveLength(3);
    const [previous, grid, next] = controls;
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
    expect(grid).toHaveAccessibleName("Câu 1 · đã làm 0 trên 3");
    expect(grid).toHaveTextContent(/^1 · 0\/3$/);
    expect(grid).toHaveAttribute("aria-haspopup", "dialog");
    expect(grid).toHaveAttribute("aria-expanded", "false");
    expect(grid).toHaveClass(
      "h-11",
      "min-w-0",
      "flex-1",
      "gap-2",
      "rounded-lg",
      "border",
      "bg-card",
      "text-base",
      "font-medium",
      "whitespace-nowrap",
    );
    expect(grid?.querySelector("svg")).toHaveClass("size-4", "flex-none");
    expect(grid?.querySelector("span")).toHaveClass("min-w-0", "truncate");
    expect(next).toHaveAccessibleName("Câu sau");
    expect(next).toHaveTextContent(/^Sau$/);
    expect(next).toHaveClass(
      "h-11",
      "flex-none",
      "rounded-lg",
      "bg-primary",
      "text-primary-fg",
      "px-4",
      "font-semibold",
    );
    expect(hint()).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("counts the question on screen and the answers as they change", async () => {
    const user = userEvent.setup();
    await open();
    expect(gridButton()).toHaveTextContent("1 · 0/3");

    await user.click(screen.getByRole("radio", { name: /Beta/ }));
    expect(gridButton()).toHaveTextContent("1 · 1/3");

    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    expect(onQuestion(2)).toBeInTheDocument();
    expect(gridButton()).toHaveTextContent("2 · 1/3");
    expect(gridButton()).toHaveAccessibleName("Câu 2 · đã làm 1 trên 3");
    expect(footer().getByRole("button", { name: "Câu trước" })).toBeEnabled();

    await user.click(footer().getByRole("button", { name: "Câu trước" }));
    expect(onQuestion(1)).toBeInTheDocument();
  });

  it("says the long form from 420 and the short one below it", async () => {
    const view = viewport(420);
    await open();
    expect(gridButton()).toHaveTextContent(/^Câu 1 · đã làm 0 trên 3$/);

    act(() => view.resize(419));
    expect(gridButton()).toHaveTextContent(/^1 · 0\/3$/);
    expect(gridButton()).toHaveAccessibleName("Câu 1 · đã làm 0 trên 3");
  });

  it("reads question 4 with three answered on the deck's paper", async () => {
    const user = userEvent.setup();
    await open(deckSession(new Date(now)));
    for (let moves = 0; moves < 3; moves++) {
      await user.click(footer().getByRole("button", { name: "Câu sau" }));
    }
    expect(gridButton()).toHaveAccessibleName("Câu 4 · đã làm 3 trên 8");
    expect(gridButton()).toHaveTextContent(/^4 · 3\/8$/);
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
    await open(withPassage(paper(), ["q1", "q2", "q3"]));
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
  it("opens from the grid button as a bottom sheet: no visible title or total, a block per section of six columns of 44px squares, and the legend", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(screen.getByRole("radio", { name: /Beta/ }));
    const opener = gridButton();
    await user.click(opener);

    const dialog = await screen.findByRole("dialog", { name: NAV });
    expect(opener).toHaveAttribute("aria-expanded", "true");
    expect(dialog).toHaveAccessibleDescription("Câu 1 · đã làm 1 trên 3");
    expect(dialog).toHaveClass(
      "top-auto",
      "right-0",
      "bottom-0",
      "left-0",
      "max-h-[80%]",
      "max-w-none",
      "sm:max-w-none",
      "translate-x-0",
      "translate-y-0",
      "flex",
      "flex-col",
      "gap-4",
      "overflow-y-auto",
      "rounded-t-3xl",
      "rounded-b-none",
      "border",
      "bg-card",
      "px-4",
      "pt-3",
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
      "sr-only",
    );
    expect(within(dialog).getByText("Câu 1 · đã làm 1 trên 3")).toHaveClass("sr-only");

    const all = squares(dialog);
    expect(all.map((square) => square.textContent)).toEqual(["1", "2", "3"]);
    expect(all[0]).toHaveAccessibleName("Câu 1, đang xem, đã trả lời");
    expect(all[0]).toHaveAttribute("aria-current", "true");
    expect(all[1]).toHaveAccessibleName("Câu 2");
    expect(all[1]).not.toHaveAttribute("aria-current");
    expect(all[2]).toHaveAccessibleName("Câu 3");
    expect(all[0]?.parentElement).toHaveClass("grid", "grid-cols-6", "gap-[5px]");
    for (const square of all) {
      expect(square).toHaveClass(
        "h-11",
        "lg:h-10.5",
        "min-w-0",
        "rounded-md",
        "border-[1.5px]",
        "text-meta",
        "font-semibold",
        "tabular-nums",
      );
    }

    const heading = within(dialog).getByRole("heading", { level: 3 });
    expect(heading).toHaveTextContent(/^Phần 1$/);
    expect(heading).toHaveClass("text-meta", "text-fg", "font-semibold");
    expect(heading.nextElementSibling).toHaveTextContent(/^1 \/ 3$/);
    expect(heading.nextElementSibling).toHaveClass(
      "text-caption",
      "text-muted-fg",
      "tabular-nums",
    );

    const answered = within(dialog).getByText("Đã trả lời");
    expect(answered.firstElementChild).toHaveClass(
      "bg-primary",
      "size-[11px]",
      "rounded-[3px]",
    );
    const partly = within(dialog).getByText("Trả lời một phần");
    expect(partly.firstElementChild).toHaveClass(
      "bg-brand-soft",
      "border-primary",
      "border",
      "size-[11px]",
    );
    const flagged = within(dialog).getByText("Đã đánh dấu");
    expect(flagged.firstElementChild).toHaveClass(
      "bg-warning",
      "size-[9px]",
      "rounded-full",
    );
    expect(answered.parentElement).toBe(flagged.parentElement);
    expect(answered.parentElement).toHaveClass(
      "text-muted-fg",
      "text-caption",
      "gap-x-3.5",
    );
    expect(answered.parentElement?.children).toHaveLength(3);
    expect(dialog.lastElementChild).toBe(answered.parentElement);
  });

  it("is a card 560px wide and 76px from the bottom from 768, beside shared content at 1024", async () => {
    viewport(1024);
    const user = userEvent.setup();
    await open(withPassage(paper(), ["q1", "q2", "q3"]));
    noRail();
    expect(gridButton()).toHaveTextContent(/^1 · 0\/3$/);
    await user.click(gridButton());

    const dialog = await screen.findByRole("dialog", { name: NAV });
    expect(dialog).toHaveClass(
      "left-[calc(50%-280px)]",
      "right-[calc(50%-280px)]",
      "bottom-[76px]",
      "top-auto",
      "w-auto",
      "rounded-2xl",
      "border",
      "max-h-[80%]",
      "px-4",
      "pt-3",
      "pb-5",
      "gap-4",
    );
    expect(dialog).not.toHaveClass("rounded-t-3xl", "bottom-0");
    expect(squares(dialog)).toHaveLength(3);
  });

  it("fills an answered square, rings the one on screen with the accent and dots a flagged one", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(screen.getByRole("radio", { name: /Beta/ }));
    await user.click(screen.getByRole("button", { name: "Đánh dấu xem lại" }));
    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    await user.click(gridButton());

    const [first, second, third] = squares(await screen.findByRole("dialog"));
    expect(first).toHaveAccessibleName("Câu 1, đã trả lời, đã đánh dấu");
    expect(first).toHaveClass("bg-primary", "text-primary-fg", "border-primary");
    expect(first).not.toHaveClass("border-brand", "ring-2");
    expect(flagDot(first)).toHaveClass(
      "bg-warning",
      "absolute",
      "-top-1",
      "-right-1",
      "size-[9px]",
      "rounded-full",
      "border-2",
      "border-card",
    );

    expect(second).toHaveAccessibleName("Câu 2, đang xem");
    expect(second).toHaveClass(
      "bg-card",
      "text-fg",
      "border-brand",
      "ring-2",
      "ring-brand-soft",
    );
    expect(second).not.toHaveClass("border-border");
    expect(flagDot(second)).toBeNull();

    expect(third).toHaveAccessibleName("Câu 3");
    expect(third).toHaveClass("bg-card", "text-fg", "border-border");
    expect(third).not.toHaveClass("bg-primary", "ring-2");
    expect(flagDot(third)).toBeNull();
  });

  it("keeps the accent border on the open question once it is answered", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(screen.getByRole("radio", { name: /Alpha/ }));
    await user.click(gridButton());

    const [first] = squares(await screen.findByRole("dialog"));
    expect(first).toHaveClass(
      "bg-primary",
      "text-primary-fg",
      "border-brand",
      "ring-2",
    );
    expect(first).not.toHaveClass("border-primary");
  });

  it("goes to a question from its square, closes, and lands on that question", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(gridButton());
    await user.click(within(await screen.findByRole("dialog")).getByText("3"));

    await noSheet();
    expect(onQuestion(3)).toBeInTheDocument();
    expect(panel("q3")).toHaveFocus();
    expect(gridButton()).toHaveTextContent("3 · 0/3");
    expect(gridButton()).toHaveAttribute("aria-expanded", "false");
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(panel("q3")).toHaveFocus();
  });

  it("closes from the open question's own square without moving", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    await user.click(gridButton());
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
    await user.click(gridButton());

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
    const opener = gridButton();
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
    const opener = gridButton();
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
    await user.click(gridButton());
    const dialog = await screen.findByRole("dialog");

    expect(within(dialog).getAllByRole("button")).toHaveLength(3);
    expect(
      within(dialog).queryByRole("button", { name: /nộp|đóng|hoàn tất/i }),
    ).toBeNull();
  });

  it("heads each part with its answered count and names the part in each square on a paper of several parts", async () => {
    const user = userEvent.setup();
    await open(twoParts);
    await user.click(screen.getByRole("radio", { name: /Alpha/ }));
    await user.click(gridButton());
    const dialog = await screen.findByRole("dialog");

    const headings = within(dialog).getAllByRole("heading", { level: 3 });
    expect(headings.map((heading) => heading.textContent)).toEqual([
      "Phần 1 · Ngữ pháp",
      "Phần 2 · Nghe",
    ]);
    expect(headings.map((heading) => heading.nextElementSibling?.textContent)).toEqual([
      "1 / 2",
      "0 / 1",
    ]);
    expect(headings[0]).toHaveClass("text-meta", "text-fg", "font-semibold");
    expect(headings[1]).toHaveClass("text-meta", "text-muted-fg", "font-medium");
    const grids = headings.map(
      (heading) => heading.parentElement?.nextElementSibling as HTMLElement,
    );
    expect(grids[0]).toHaveClass("grid-cols-6");
    expect(squares(grids[0]!).map((square) => square.textContent)).toEqual(["1", "2"]);
    expect(squares(grids[1]!).map((square) => square.textContent)).toEqual(["3"]);
    expect(squares(dialog).map((square) => square.getAttribute("aria-label"))).toEqual([
      "Câu 1, Phần 1 · Ngữ pháp, đang xem, đã trả lời",
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
    await user.click(gridButton());
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

  it("scrolls inside itself on a long paper", async () => {
    const user = userEvent.setup();
    await open(long(80));
    expect(gridButton()).toHaveTextContent("1 · 0/80");
    await user.click(gridButton());
    const dialog = await screen.findByRole("dialog");

    expect(squares(dialog)).toHaveLength(80);
    expect(dialog).toHaveAccessibleDescription("Câu 1 · đã làm 0 trên 80");
    expect(dialog).toHaveClass("max-h-[80%]", "overflow-y-auto");
  });

  it("takes the card's shape at 768 and goes away when the engine grows past 900, where the rail takes over", async () => {
    const view = viewport(767);
    const user = userEvent.setup();
    await open();
    await user.click(gridButton());
    expect(await screen.findByRole("dialog")).toHaveClass("rounded-t-3xl");

    act(() => view.resize(768));
    expect(screen.getByRole("dialog")).toHaveClass("rounded-2xl", "bottom-[76px]");

    act(() => view.resize(900));
    await noSheet();
    expect(squares(rail())).toHaveLength(3);
    expect(screen.queryByRole("button", { name: /^Câu \d+ · đã làm / })).toBeNull();
  });
});

describe("the rail", () => {
  beforeEach(() => viewport(900));

  it("stands at the paper's left edge from 900, with Previous, the key hint and Next in the footer", async () => {
    await open();

    expect(screen.getByRole("main").firstElementChild).toBe(rail());
    expect(rail().tagName).toBe("ASIDE");
    expect(rail()).toHaveClass(
      "w-59",
      "flex-none",
      "bg-sidebar",
      "border-r",
      "px-3.5",
      "py-4",
      "flex",
      "flex-col",
      "gap-4",
      "overflow-y-auto",
    );
    const heading = within(rail()).getByRole("heading", { level: 3 });
    expect(heading).toHaveTextContent(/^Phần 1$/);
    expect(heading).toHaveClass(
      "text-meta",
      "leading-[1.35]",
      "text-fg",
      "font-semibold",
    );
    expect(heading.nextElementSibling).toHaveTextContent(/^0 \/ 3$/);
    expect(squares(rail())[0]?.parentElement).toHaveClass(
      "grid",
      "grid-cols-5",
      "gap-[5px]",
    );
    for (const square of squares(rail())) {
      expect(square).toHaveClass(
        "h-8.5",
        "min-h-0",
        "min-w-0",
        "rounded-md",
        "border-[1.5px]",
        "text-meta",
        "font-semibold",
        "tabular-nums",
      );
    }
    const legend = within(rail()).getByText("Trả lời một phần").parentElement;
    expect(legend).toBe(rail().lastElementChild);
    expect(legend).toHaveClass("mt-auto", "pt-2", "text-caption");

    expect(nav().parentElement).toBe(panel("q1")?.closest("section"));
    expect(nav()).toHaveClass("px-6");
    const controls = footer().getAllByRole("button");
    expect(controls.map((control) => control.textContent)).toEqual(["Trước", "Sau"]);
    expect(controls[0]).toHaveAccessibleName("Câu trước");
    expect(controls[0]).toBeDisabled();
    expect(controls[1]).toHaveAccessibleName("Câu sau");
    expect(hint()).toHaveTextContent(/^←→để chuyển câu · A–D để chọn đáp án$/);
    expect(hint()).toHaveClass(
      "flex-1",
      "min-w-0",
      "justify-center",
      "text-meta",
      "text-muted-fg",
      "whitespace-nowrap",
      "overflow-hidden",
    );
    const keys = hint()!.querySelectorAll("kbd");
    expect([...keys].map((key) => key.textContent)).toEqual(["←", "→"]);
    expect(keys[0]).toHaveClass(
      "h-5.5",
      "min-w-5.5",
      "border",
      "border-b-2",
      "bg-card",
    );
    expect(hint()?.lastElementChild).toHaveClass("truncate");
    expect(screen.queryByRole("button", { name: /^Câu \d+ · đã làm / })).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("keeps the title, the save line and the timer in the header, and draws a rail at 900 and a hint in the footer", async () => {
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

    expect(screen.queryByRole("separator")).toBeNull();
    expect(document.querySelector("[data-side-column]")).toBeNull();
    expect(screen.queryByRole("button", { name: "Xem lại & nộp" })).toBeNull();
    expect(panel("q1")?.querySelectorAll("button")).toHaveLength(1);
    expect(screen.getByRole("complementary")).toBe(rail());
    expect(
      footer().getByText("để chuyển câu · A–D để chọn đáp án"),
    ).toBeInTheDocument();
  });

  it("is not drawn below 900", async () => {
    viewport(899);
    await open();
    noRail();
    expect(gridButton()).toHaveTextContent(/^Câu 1 · đã làm 0 trên 3$/);
    expect(hint()).toBeNull();
  });

  it("fills an answered square, rings the one on screen with the accent and dots a flagged one", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(screen.getByRole("radio", { name: /Beta/ }));
    await user.click(screen.getByRole("button", { name: "Đánh dấu xem lại" }));
    await user.click(footer().getByRole("button", { name: "Câu sau" }));

    const [first, second, third] = squares(rail());
    expect(first).toHaveAccessibleName("Câu 1, đã trả lời, đã đánh dấu");
    expect(first).not.toHaveAttribute("aria-current");
    expect(first).toHaveClass("bg-primary", "text-primary-fg", "border-primary");
    expect(flagDot(first)).toHaveClass(
      "bg-warning",
      "-top-1",
      "-right-1",
      "size-[9px]",
      "rounded-full",
      "border-2",
      "border-card",
    );

    expect(second).toHaveAccessibleName("Câu 2, đang xem");
    expect(second).toHaveAttribute("aria-current", "true");
    expect(second).toHaveClass("bg-card", "text-fg", "border-brand", "ring-2");
    expect(flagDot(second)).toBeNull();

    expect(third).toHaveAccessibleName("Câu 3");
    expect(third).toHaveClass("bg-card", "text-fg", "border-border");

    await user.click(footer().getByRole("button", { name: "Câu trước" }));
    await user.click(screen.getByRole("button", { name: "Đã đánh dấu" }));
    expect(flagDot(squares(rail())[0])).toBeNull();
    expect(squares(rail())[0]).toHaveAccessibleName("Câu 1, đang xem, đã trả lời");
    expect(squares(rail())[0]).toHaveClass("bg-primary", "border-brand");
    expect(within(rail()).getByText("1 / 3")).toBeInTheDocument();
  });

  it("marks a fill-in with some of its blanks typed as partly answered, and as answered once all are", async () => {
    const user = userEvent.setup();
    await open(paper({ questions: [questions[0]!, fillIn, questions[2]!] }));
    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    await user.type(screen.getByRole("textbox", { name: "Chỗ trống 1" }), "went");
    expect(squares(rail())[1]).toHaveAccessibleName(
      "Câu 2, đang xem, trả lời một phần",
    );
    expect(squares(rail())[1]).toHaveClass("bg-brand-soft", "text-fg", "border-brand");

    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    const partly = squares(rail())[1];
    expect(partly).toHaveAccessibleName("Câu 2, trả lời một phần");
    expect(partly).toHaveClass("bg-brand-soft", "text-fg", "border-primary");
    expect(within(rail()).getByText("0 / 3")).toBeInTheDocument();

    await user.click(partly!);
    await user.type(screen.getByRole("textbox", { name: "Chỗ trống 2" }), "cooked");
    expect(squares(rail())[1]).toHaveAccessibleName("Câu 2, đang xem, đã trả lời");
    expect(squares(rail())[1]).toHaveClass("bg-primary", "text-primary-fg");
    expect(within(rail()).getByText("1 / 3")).toBeInTheDocument();
  });

  it("goes to a question from its square and lands on it", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(within(rail()).getByRole("button", { name: "Câu 3" }));

    expect(onQuestion(3)).toBeInTheDocument();
    expect(panel("q3")).toHaveFocus();
    expect(
      within(rail()).getByRole("button", { name: "Câu 3, đang xem" }),
    ).toHaveAttribute("aria-current", "true");
    expect(within(rail()).getByRole("button", { name: "Câu 1" })).not.toHaveAttribute(
      "aria-current",
    );
  });

  it("leaves Next to turn into Finish on the last question, which opens the Submit dialog the header's Submit opens", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(within(rail()).getByRole("button", { name: "Câu 3" }));

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

  it("heads each part with its count, names the part in each square, and states a part's instructions on its first question", async () => {
    const user = userEvent.setup();
    await open(twoParts);

    const headings = within(rail()).getAllByRole("heading", { level: 3 });
    expect(headings.map((heading) => heading.textContent)).toEqual([
      "Phần 1 · Ngữ pháp",
      "Phần 2 · Nghe",
    ]);
    expect(headings.map((heading) => heading.nextElementSibling?.textContent)).toEqual([
      "0 / 2",
      "0 / 1",
    ]);
    expect(headings[0]).toHaveClass("text-fg", "font-semibold");
    expect(headings[1]).toHaveClass("text-muted-fg", "font-medium");

    const [first, second, third] = squares(rail());
    expect(first).toHaveAccessibleName("Câu 1, Phần 1 · Ngữ pháp, đang xem");
    expect(second).toHaveAccessibleName("Câu 2, Phần 1 · Ngữ pháp");
    expect(third).toHaveAccessibleName("Câu 3, Phần 2 · Nghe");

    const pane = () => within(screen.getByRole("main"));
    expect(pane().getByText("Câu 1 trên 3 · Chọn một đáp án")).toBeInTheDocument();
    expect(screen.queryByRole("note")).toBeNull();

    await user.click(third!);
    expect(headings[0]).toHaveClass("text-muted-fg", "font-medium");
    expect(
      within(rail()).getByRole("heading", { level: 3, name: "Phần 2 · Nghe" }),
    ).toHaveClass("text-fg", "font-semibold");
    expect(pane().getByText("Câu 3 trên 3 · Đúng hay sai")).toBeInTheDocument();
    expect(screen.getByRole("note")).toHaveTextContent(
      "Nghe đoạn hội thoại rồi trả lời.",
    );
  });

  it("names no part in a square on a paper of one part", async () => {
    await open();
    expect(squares(rail()).map((square) => square.getAttribute("aria-label"))).toEqual([
      "Câu 1, đang xem",
      "Câu 2",
      "Câu 3",
    ]);
  });

  it("scrolls by itself on a long paper and brings the open question into view", async () => {
    const seen: Element[] = [];
    const scrollIntoView = vi
      .spyOn(Element.prototype, "scrollIntoView")
      .mockImplementation(function (this: Element) {
        seen.push(this);
      });
    const user = userEvent.setup();
    await open(long(40));

    expect(squares(rail())).toHaveLength(40);
    expect(rail()).toHaveClass("overflow-y-auto");
    expect(seen).toContain(squares(rail())[0]);
    expect(scrollIntoView).toHaveBeenCalledWith({ block: "nearest" });

    await user.click(within(rail()).getByRole("button", { name: "Câu 37" }));
    expect(seen).toContain(squares(rail())[36]);
    expect(squares(rail())[36]).toHaveAttribute("aria-current", "true");
  });

  it("says only how to move on a question with no options", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    expect(hint()).toHaveTextContent(/^←→để chuyển câu$/);
  });

  it("gives way to the grid button when the window shrinks below 900", async () => {
    const view = viewport(900);
    await open();
    expect(squares(rail())).toHaveLength(3);

    act(() => view.resize(899));
    noRail();
    expect(gridButton()).toHaveTextContent("Câu 1 · đã làm 0 trên 3");
    expect(footer().getByRole("button", { name: "Câu trước" })).toHaveTextContent(
      "Trước",
    );

    act(() => view.resize(767));
    expect(gridButton()).toHaveTextContent("Câu 1 · đã làm 0 trên 3");
    expect(footer().getByRole("button", { name: "Câu trước" })).toHaveTextContent(/^$/);
  });
});

describe("the rail beside shared content", () => {
  it("waits for 1180, and the footer's hint is the short one", async () => {
    const view = viewport(1179);
    await open(withPassage(paper(), ["q1", "q2", "q3"]));
    noRail();
    expect(gridButton()).toHaveTextContent(/^1 · 0\/3$/);
    expect(gridButton()).toHaveAccessibleName("Câu 1 · đã làm 0 trên 3");

    act(() => view.resize(1180));
    expect(squares(rail())).toHaveLength(3);
    expect(screen.getByRole("main").firstElementChild).toBe(rail());
    expect(hint()).toHaveTextContent(/^←→để chuyển · A–D để chọn$/);
    expect(screen.queryByRole("button", { name: /^Câu \d+ · đã làm / })).toBeNull();
  });

  it("shows the grid button's short label at 768 and the long one with no shared content", async () => {
    viewport(768);
    const user = userEvent.setup();
    await open(withPassage(paper(), ["q2", "q3"]));
    expect(gridButton()).toHaveTextContent(/^Câu 1 · đã làm 0 trên 3$/);

    await user.click(footer().getByRole("button", { name: "Câu sau" }));
    expect(gridButton()).toHaveTextContent(/^2 · 0\/3$/);
  });

  it("comes and goes between questions from 900 to 1179", async () => {
    viewport(1024);
    const user = userEvent.setup();
    await open(withPassage(paper(), ["q2", "q3"]));
    expect(squares(rail())).toHaveLength(3);
    expect(hint()).toHaveTextContent("để chuyển câu · A–D để chọn đáp án");

    await user.click(within(rail()).getByRole("button", { name: "Câu 2" }));
    expect(onQuestion(2)).toBeInTheDocument();
    noRail();
    expect(gridButton()).toHaveTextContent(/^2 · 0\/3$/);

    await user.click(footer().getByRole("button", { name: "Câu trước" }));
    expect(onQuestion(1)).toBeInTheDocument();
    expect(squares(rail())).toHaveLength(3);
  });
});

describe.each(["superseded", "closed"] as const)(
  "on a paper whose lock is %s",
  (lock) => {
    it("offers no Finish on the last question, and nothing on the page opens the Submit dialog", async () => {
      viewport("desktop");
      const user = userEvent.setup();
      await open();
      await user.click(within(rail()).getByRole("button", { name: "Câu 3" }));
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
      ).toEqual(["Trước"]);
      expect(squares(rail())).toHaveLength(3);
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
      await user.click(gridButton());
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
    await user.click(within(rail()).getByRole("button", { name: "Câu 3" }));
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

describe("the navigator in English", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("is the deck's wording beside the rail", async () => {
    viewport(1280);
    const user = userEvent.setup();
    await open(deckSession(new Date(now)));
    const bar = within(screen.getByRole("navigation", { name: "Questions" }));
    const sections = within(screen.getByRole("complementary", { name: "Sections" }));

    expect(bar.getByRole("button", { name: "Previous question" })).toHaveTextContent(
      /^Previous$/,
    );
    expect(bar.getByRole("button", { name: "Next question" })).toHaveTextContent(
      /^Next$/,
    );
    expect(
      bar.getByText("to move between questions · A–D to choose an answer"),
    ).toBeInTheDocument();
    expect(
      sections.getByRole("heading", { name: "Reading passage 1" }),
    ).toBeInTheDocument();
    expect(sections.getByText("3 / 8")).toBeInTheDocument();
    expect(
      sections.getByRole("button", { name: "Question 1, current, answered" }),
    ).toBe(squares(screen.getByRole("complementary"))[0]);
    expect(sections.getByRole("button", { name: "Question 4" })).toBeInTheDocument();
    expect(sections.getByText("Answered")).toBeInTheDocument();
    expect(sections.getByText("Partly")).toBeInTheDocument();
    expect(sections.getByText("Flagged")).toBeInTheDocument();

    await user.click(sections.getByRole("button", { name: "Question 8" }));
    expect(bar.queryByRole("button", { name: "Next question" })).toBeNull();
    expect(bar.getByRole("button", { name: "Finish" })).toBeInTheDocument();
  });

  it("is the deck's wording in the grid button and the sheet, in both label forms", async () => {
    const view = viewport(600);
    const user = userEvent.setup();
    await open(deckSession(new Date(now)));
    for (let moves = 0; moves < 3; moves++) {
      await user.click(screen.getByRole("button", { name: "Next question" }));
    }
    const grid = () => screen.getByRole("button", { name: "Question 4 · 3 of 8 done" });
    expect(grid()).toHaveTextContent(/^Question 4 · 3 of 8 done$/);
    act(() => view.resize(360));
    expect(grid()).toHaveTextContent(/^4 · 3\/8$/);

    await user.click(grid());
    const dialog = await screen.findByRole("dialog", { name: "Questions" });
    expect(dialog).toHaveAccessibleDescription("Question 4 · 3 of 8 done");
    expect(within(dialog).getByText("Answered")).toBeInTheDocument();
    expect(within(dialog).getByText("Partly")).toBeInTheDocument();
    expect(within(dialog).getByText("Flagged")).toBeInTheDocument();
    await waitFor(() =>
      expect(
        within(dialog).getByRole("button", { name: "Question 4, current" }),
      ).toHaveFocus(),
    );
  });

  it("names a partly answered square", async () => {
    viewport(1280);
    const user = userEvent.setup();
    await open(paper({ questions: [fillIn, questions[2]!] }));
    await user.type(screen.getByRole("textbox", { name: "Blank 1" }), "went");
    expect(
      within(screen.getByRole("complementary")).getByRole("button", {
        name: "Question 1, current, partly answered",
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("navigation")).toHaveTextContent(/←→to move/);
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
