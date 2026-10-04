import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import { getAttempt, saveAnswers } from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { deckSession } from "./deckSession";
import { session, viewport } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
}));

beforeEach(() => {
  useTakeTestStore.getState().reset();
  const now = new Date().toISOString();
  const deadline = new Date(Date.now() + 3_600_000).toISOString();
  vi.mocked(saveAnswers).mockResolvedValue({
    serverTime: now,
    savedAt: now,
    deadlineAt: deadline,
  });
  vi.mocked(getAttempt).mockResolvedValue({
    ...session({
      serverTime: now,
      deadlineAt: deadline,
    }),
    questions: [
      {
        id: "q1",
        sectionId: "s1",
        type: "single_choice",
        prompt: "Choose one",
        points: 1,
        options: [
          { id: "a", text: "First answer" },
          { id: "b", text: "Second answer" },
        ],
      },
      {
        id: "q2",
        sectionId: "s1",
        type: "short_answer",
        prompt: "Explain your answer",
        points: 1,
      },
    ],
  });
});
afterEach(() => useTakeTestStore.getState().reset());

it("keeps navigation and answer shortcuts working after clicking a radio", async () => {
  const router = createMemoryRouter(
    [{ path: "/app/attempts/:attemptId", element: <TakeTestPage /> }],
    {
      initialEntries: ["/app/attempts/att-1"],
    },
  );
  render(<RouterProvider router={router} />);
  const user = userEvent.setup();
  await user.click(await screen.findByRole("radio", { name: "First answer" }));
  await user.keyboard("b");
  expect(screen.getByRole("radio", { name: "Second answer" })).toBeChecked();
  await user.keyboard("f");
  expect(screen.getByRole("button", { name: "Đã đánh dấu" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await user.keyboard("{ArrowRight}");
  expect(await screen.findByText("Explain your answer")).toBeVisible();
  const input = screen.getByRole("textbox");
  await user.type(input, "first");
  await user.keyboard("{ArrowLeft}f");
  expect(input).toHaveValue("firsft");
  expect(useTakeTestStore.getState().flags.has("q2")).toBe(false);
});

function mount() {
  const router = createMemoryRouter(
    [
      { path: "/app/attempts/:attemptId", element: <TakeTestPage /> },
      { path: "/app", element: <p>home</p> },
    ],
    { initialEntries: ["/app/attempts/att-1"] },
  );
  render(<RouterProvider router={router} />);
  return userEvent.setup();
}

const store = () => useTakeTestStore.getState();
const onQuestion = (n: number) => screen.queryByRole("main", { name: `Câu ${n}` });
const flagged = () => screen.queryByRole("button", { name: "Đã đánh dấu" });

async function sixOptions() {
  const paper = await getAttempt("att-1");
  vi.mocked(getAttempt).mockResolvedValue({
    ...paper,
    questions: [
      {
        ...paper.questions[0]!,
        options: ["One", "Two", "Three", "Four", "Five", "Six"].map((text) => ({
          id: text.toLowerCase(),
          text,
        })),
      },
      paper.questions[1]!,
    ],
  });
}

describe("the keys", () => {
  beforeEach(() => {
    sessionStorage.clear();
    localStorage.clear();
  });
  afterEach(() => vi.unstubAllGlobals());

  it("choose with A to E, in either case, and leave a sixth option to Tab: F flags", async () => {
    await sixOptions();
    const user = mount();
    await screen.findByRole("radio", { name: "One" });

    await user.keyboard("e");
    expect(screen.getByRole("radio", { name: "Five" })).toBeChecked();
    await user.keyboard("D");
    expect(screen.getByRole("radio", { name: "Four" })).toBeChecked();
    await user.keyboard("a");
    expect(screen.getByRole("radio", { name: "One" })).toBeChecked();

    await user.keyboard("f");
    expect(flagged()).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Six" })).not.toBeChecked();
    expect(screen.getByRole("radio", { name: "One" })).toBeChecked();
    await user.keyboard("g");
    expect(store().answers["q1"]).toEqual({ type: "choice", optionIds: ["one"] });
  });

  it("toggle the fifth option of a question that takes several answers", async () => {
    const paper = deckSession(new Date());
    const fourth = paper.questions[3]!;
    vi.mocked(getAttempt).mockResolvedValue(paper);
    const user = mount();
    await screen.findByRole("main");
    await user.click(screen.getByRole("button", { name: "Câu 4" }));

    await user.keyboard("e");
    expect(screen.getByRole("checkbox", { name: "More tourists" })).toBeChecked();
    await user.keyboard("a");
    expect(store().answers[fourth.id]).toEqual({
      type: "choice",
      optionIds: [fourth.options![4]!.id, fourth.options![0]!.id],
    });
    await user.keyboard("e");
    expect(screen.getByRole("checkbox", { name: "More tourists" })).not.toBeChecked();
  });

  it("ignore a letter past the question's last option", async () => {
    const user = mount();
    await screen.findByRole("radio", { name: "First answer" });
    await user.keyboard("c");
    await user.keyboard("e");
    expect(store().answers["q1"]).toBeUndefined();
    expect(store().dirty.size).toBe(0);
  });

  it("move with the arrows and stop at either end, opening nothing", async () => {
    const user = mount();
    await screen.findByRole("radio", { name: "First answer" });

    await user.keyboard("{ArrowLeft}");
    expect(onQuestion(1)).toBeInTheDocument();
    await user.keyboard("{ArrowRight}");
    expect(onQuestion(2)).toBeInTheDocument();
    await user.keyboard("{ArrowRight}");
    await user.keyboard("{ArrowRight}");
    expect(onQuestion(2)).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).toBeNull();
    await user.keyboard("{ArrowLeft}");
    expect(onQuestion(1)).toBeInTheDocument();
  });

  it("flag once however long F is held", async () => {
    mount();
    await screen.findByRole("radio", { name: "First answer" });

    fireEvent.keyDown(window, { key: "f" });
    expect(store().flags.has("q1")).toBe(true);
    fireEvent.keyDown(window, { key: "f", repeat: true });
    expect(store().flags.has("q1")).toBe(true);
    fireEvent.keyDown(window, { key: "f", repeat: true });
    expect(store().flags.has("q1")).toBe(true);
    fireEvent.keyDown(window, { key: "F" });
    expect(store().flags.has("q1")).toBe(false);
  });

  it("rest while Ctrl, Alt or the command key is held", async () => {
    const user = mount();
    await screen.findByRole("radio", { name: "First answer" });

    await user.keyboard("{Control>}f{/Control}");
    await user.keyboard("{Meta>}b{/Meta}");
    await user.keyboard("{Alt>}{ArrowRight}{/Alt}");
    expect(store().flags.size).toBe(0);
    expect(store().answers["q1"]).toBeUndefined();
    expect(onQuestion(1)).toBeInTheDocument();
  });

  it("rest while the Leave dialog is open, and Esc closes it", async () => {
    const user = mount();
    await screen.findByRole("radio", { name: "First answer" });
    await user.click(screen.getByRole("button", { name: "Thoát khỏi bài làm" }));
    expect(await screen.findByRole("dialog")).toHaveAccessibleName(
      "Thoát khỏi bài làm?",
    );

    await user.keyboard("b");
    await user.keyboard("f");
    await user.keyboard("{ArrowRight}");
    expect(store().answers["q1"]).toBeUndefined();
    expect(store().flags.size).toBe(0);
    expect(store().questions[0]?.id).toBe("q1");
    expect(screen.queryByRole("main", { name: "Câu 2", hidden: true })).toBeNull();

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(onQuestion(1)).toBeInTheDocument();
    await user.keyboard("b");
    expect(screen.getByRole("radio", { name: "Second answer" })).toBeChecked();
  });

  it("rest while the question sheet is open, and Esc closes it", async () => {
    viewport("phone");
    const user = mount();
    await screen.findByRole("radio", { name: "First answer" });
    await user.click(screen.getByRole("button", { name: /^Danh sách câu: / }));
    await screen.findByRole("dialog", { name: "Danh sách câu" });

    await user.keyboard("b");
    await user.keyboard("f");
    await user.keyboard("{ArrowRight}");
    expect(store().answers["q1"]).toBeUndefined();
    expect(store().flags.size).toBe(0);
    expect(screen.queryByRole("main", { name: "Câu 2", hidden: true })).toBeNull();

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(onQuestion(1)).toBeInTheDocument();
    await user.keyboard("{ArrowRight}");
    expect(onQuestion(2)).toBeInTheDocument();
  });

  it("rest on a paper that is locked", async () => {
    const user = mount();
    await screen.findByRole("radio", { name: "First answer" });
    act(() => store().lockNow("deadline"));

    await user.keyboard("b");
    await user.keyboard("f");
    await user.keyboard("{ArrowRight}");
    expect(store().answers["q1"]).toBeUndefined();
    expect(store().flags.size).toBe(0);
    expect(onQuestion(1)).toBeInTheDocument();
  });
});
