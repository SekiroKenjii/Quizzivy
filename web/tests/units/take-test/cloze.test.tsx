import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import {
  getAttempt,
  saveAnswers,
  type AttemptSession,
  type StudentQuestion,
} from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import i18n from "@/lib/i18n";
import { DECK_MOCK_PROMPT, deckMockTestSession } from "./deckSession";
import { viewport } from "./support";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  recordGroupAudioPlay: vi.fn(),
}));

const NOW = new Date();
const RUN_LINE = "Câu 23–27 trên 34 · Điền từ vào đoạn văn · 5 chỗ trống";
const HINT =
  "Các chỗ trống nằm trong đoạn văn. Chọn chỗ trống ở đây hoặc trong đoạn văn.";

let paper: AttemptSession;
const member = (no: number) => paper.questions[no - 1]!;
const openKey = () => `quizzivy.open-question.${paper.attempt.id}`;

function withMembers(
  change: (question: StudentQuestion, no: number) => StudentQuestion,
) {
  paper = {
    ...paper,
    questions: paper.questions.map((question, at) =>
      at >= 22 && at <= 26 ? change(question, at + 1) : question,
    ),
  };
}

function mount(openOn = 23) {
  sessionStorage.setItem(openKey(), member(openOn).id);
  vi.mocked(getAttempt).mockResolvedValue(paper);
  const router = createMemoryRouter(
    [{ path: "/app/attempts/:attemptId", element: <TakeTestPage /> }],
    { initialEntries: [`/app/attempts/${paper.attempt.id}`] },
  );
  return render(<RouterProvider router={router} />);
}

const card = () => screen.getByRole("group", { name: /^Chỗ trống \d+$/ });
const passage = () => screen.getByRole("article", { name: "Reading · Green spaces" });
const strip = (no: number, word = "trống") =>
  screen
    .getAllByRole("button", { name: `Chỗ trống ${no}, ${word}` })
    .find((chip) => !passage().contains(chip))!;
const option = (text: string) => within(card()).getByRole("radio", { name: text });

beforeEach(async () => {
  await i18n.changeLanguage("vi");
  viewport("desktop");
  useTakeTestStore.getState().reset();
  localStorage.clear();
  sessionStorage.clear();
  paper = deckMockTestSession(NOW);
  vi.mocked(saveAnswers).mockResolvedValue({
    serverTime: NOW.toISOString(),
    savedAt: NOW.toISOString(),
    deadlineAt: paper.attempt.deadlineAt,
  });
});
afterEach(async () => {
  useTakeTestStore.getState().reset();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  await i18n.changeLanguage("vi");
});

describe("a run of a cloze", () => {
  it("is one screen: the line, the shared prompt once, the hint, the strip and the open gap's card", async () => {
    mount();
    expect(await screen.findByText(RUN_LINE)).toBeVisible();
    expect(screen.getAllByText(DECK_MOCK_PROMPT)).toHaveLength(1);
    expect(within(card()).queryByText(DECK_MOCK_PROMPT)).toBeNull();
    expect(screen.getByText(HINT)).toBeVisible();
    for (const no of [23, 24, 25, 26, 27]) expect(strip(no)).toHaveTextContent("—");
    expect(strip(23)).toHaveAttribute("aria-current", "true");
    expect(card()).toHaveAccessibleName("Chỗ trống 23");
    expect(within(card()).getByText("Đã điền 0/5 chỗ trống")).toBeVisible();
    expect(within(card()).getAllByRole("radio")).toHaveLength(4);
    expect(within(card()).getByText("1 điểm")).toBeVisible();
    expect(screen.getByRole("button", { name: "Đánh dấu câu 23" })).toBeVisible();
  });

  it("reads the deck's line in English", async () => {
    await i18n.changeLanguage("en");
    mount();
    expect(
      await screen.findByText("Questions 23–27 of 34 · Cloze · 5 gaps"),
    ).toBeVisible();
    expect(
      screen.getByText(
        "The gaps are in the passage. Pick a gap here or in the passage.",
      ),
    ).toBeVisible();
    expect(screen.getByRole("group", { name: "Gap 23" })).toHaveTextContent(
      "0 of 5 gaps filled",
    );
    expect(screen.getByRole("button", { name: "Flag question 23" })).toBeVisible();
  });

  it("writes one choice answer under the open member's id and opens the next gap with no answer", async () => {
    const user = userEvent.setup();
    mount();
    await screen.findByText(RUN_LINE);
    await user.click(option("relaxing"));

    const { answers } = useTakeTestStore.getState();
    expect(Object.keys(answers)).toEqual([member(23).id]);
    expect(answers[member(23).id]).toEqual({
      type: "choice",
      optionIds: [member(23).options![1]!.id],
    });
    expect(card()).toHaveAccessibleName("Chỗ trống 24");
    expect(strip(23, "relaxing")).toHaveTextContent("relaxing");
    expect(within(card()).getByText("Đã điền 1/5 chỗ trống")).toBeVisible();
    expect(sessionStorage.getItem(openKey())).toBe(member(24).id);
  });

  it("goes round to the first empty gap, and stays on the last gap once every gap is filled", async () => {
    const user = userEvent.setup();
    mount(26);
    await screen.findByText(RUN_LINE);
    await user.click(option("some"));
    expect(card()).toHaveAccessibleName("Chỗ trống 27");
    await user.click(option("make"));
    expect(card()).toHaveAccessibleName("Chỗ trống 23");
    await user.click(option("relax"));
    await user.click(option("make"));
    await user.click(option("cover"));
    expect(card()).toHaveAccessibleName("Chỗ trống 25");
    await user.click(option("covered"));
    expect(card()).toHaveAccessibleName("Chỗ trống 25");
    expect(option("covered")).toHaveAttribute("aria-checked", "true");
    expect(within(card()).getByText("Đã điền 5/5 chỗ trống")).toBeVisible();
    expect(Object.keys(useTakeTestStore.getState().answers)).toHaveLength(5);
  });

  it("opens a gap from the strip, and from the passage, where the open gap's chip is marked", async () => {
    const user = userEvent.setup();
    mount();
    await screen.findByText(RUN_LINE);
    await user.click(strip(26));
    expect(card()).toHaveAccessibleName("Chỗ trống 26");
    const inPassage = within(passage()).getByRole("button", {
      name: "Chỗ trống 26, trống",
    });
    expect(inPassage).toHaveAttribute("data-state", "open");
    expect(passage().querySelector(".content-cloze")).toContainElement(inPassage);

    await user.click(
      within(passage()).getByRole("button", { name: "Chỗ trống 24, trống" }),
    );
    expect(card()).toHaveAccessibleName("Chỗ trống 24");
    await user.click(option("make"));
    expect(
      within(passage()).getByRole("button", { name: "Chỗ trống 24, make" }),
    ).toHaveAttribute("data-state", "filled");
  });

  it("chooses with A to E and flags with F for the open gap", async () => {
    mount();
    await screen.findByText(RUN_LINE);
    fireEvent.keyDown(window, { key: "c" });
    expect(useTakeTestStore.getState().answers[member(23).id]).toEqual({
      type: "choice",
      optionIds: [member(23).options![2]!.id],
    });
    expect(card()).toHaveAccessibleName("Chỗ trống 24");

    fireEvent.keyDown(window, { key: "f" });
    expect(useTakeTestStore.getState().flags.has(member(24).id)).toBe(true);
    expect(useTakeTestStore.getState().flags.has(member(23).id)).toBe(false);
    expect(screen.getByRole("button", { name: "Đã đánh dấu câu 24" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("is one step for the arrows and for Previous and Next", async () => {
    const user = userEvent.setup();
    mount(22);
    await screen.findByText("Câu 22 trên 34 · Một đáp án");
    await user.click(screen.getByRole("button", { name: "Câu sau" }));
    expect(screen.getByText(RUN_LINE)).toBeVisible();
    expect(card()).toHaveAccessibleName("Chỗ trống 23");
    await user.click(strip(25));
    await user.click(screen.getByRole("button", { name: "Câu sau" }));
    expect(screen.getByText("Câu 28 trên 34 · Một đáp án")).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Câu trước" }));
    expect(card()).toHaveAccessibleName("Chỗ trống 23");

    fireEvent.keyDown(window, { key: "ArrowRight" });
    expect(screen.getByText("Câu 28 trên 34 · Một đáp án")).toBeVisible();
    fireEvent.keyDown(window, { key: "ArrowLeft" });
    fireEvent.keyDown(window, { key: "ArrowLeft" });
    expect(screen.getByText("Câu 22 trên 34 · Một đáp án")).toBeVisible();
  });

  it("reopens on the stored question's gap after a reload", async () => {
    const first = mount(25);
    await screen.findByText(RUN_LINE);
    expect(card()).toHaveAccessibleName("Chỗ trống 25");
    expect(strip(25)).toHaveAttribute("aria-current", "true");
    first.unmount();
    useTakeTestStore.getState().reset({ keepDraft: true });

    const router = createMemoryRouter(
      [{ path: "/app/attempts/:attemptId", element: <TakeTestPage /> }],
      { initialEntries: [`/app/attempts/${paper.attempt.id}`] },
    );
    render(<RouterProvider router={router} />);
    await screen.findByText(RUN_LINE);
    expect(card()).toHaveAccessibleName("Chỗ trống 25");
  });

  it("shows each member's own prompt and worth in the card when the prompts differ", async () => {
    withMembers((question, no) => ({
      ...question,
      prompt: `Prompt of gap ${no}`,
      points: no === 25 ? 2 : 1,
      ...(no === 24
        ? {
            prompt: "Which verb goes with the air cleaner?",
            promptContent: {
              format: "semantic_v1",
              blocks: [
                {
                  type: "paragraph",
                  content: [
                    { type: "text", text: "Which verb goes with ", marks: [] },
                    { type: "text", text: "the air cleaner", marks: ["bold"] },
                    { type: "text", text: "?", marks: [] },
                  ],
                },
              ],
            },
          }
        : {}),
    }));
    const user = userEvent.setup();
    mount();
    await screen.findByText(RUN_LINE);
    expect(within(card()).getByText("Prompt of gap 23")).toBeVisible();
    expect(screen.queryByText("Prompt of gap 24")).toBeNull();

    await user.click(strip(24));
    expect(within(card()).getByText("the air cleaner").tagName).toBe("STRONG");
    await user.click(strip(25));
    expect(within(card()).getByText("Prompt of gap 25")).toBeVisible();
    expect(within(card()).getByText("2 điểm")).toBeVisible();
    await user.click(strip(26));
    expect(within(card()).getByText("1 điểm")).toBeVisible();
  });

  it("refuses edits on a locked paper and keeps the answers legible", async () => {
    mount();
    await screen.findByText(RUN_LINE);
    act(() => useTakeTestStore.getState().lockNow("superseded"));
    for (const radio of within(card()).getAllByRole("radio"))
      expect(radio).toBeDisabled();
    expect(strip(24)).toBeEnabled();
  });
});

describe("a gap with media of its own", () => {
  beforeEach(() => {
    withMembers((question, no) =>
      no === 26
        ? {
            ...question,
            media: {
              id: "018f0000-0000-7000-8000-00000000f126",
              kind: "image",
              url: "https://assets.example/q26.png",
              mimeType: "image/png",
            } as NonNullable<StudentQuestion["media"]>,
          }
        : question,
    );
  });

  it("cuts the run short, and the run's hint names the gaps with screens of their own", async () => {
    mount();
    expect(
      await screen.findByText(
        "Câu 23–25 trên 34 · Điền từ vào đoạn văn · 3 trong 5 chỗ trống",
      ),
    ).toBeVisible();
    expect(screen.getByText("Các chỗ trống 26 và 27 có màn hình riêng.").tagName).toBe(
      "STRONG",
    );
    expect(within(card()).getByText("Đã điền 0/3 chỗ trống")).toBeVisible();
    expect(
      within(passage()).getAllByRole("button", { name: /^Chỗ trống \d+, / }),
    ).toHaveLength(5);
  });

  it("is a question of its own that says it is a gap of the cloze, as is a gap left alone", async () => {
    const user = userEvent.setup();
    mount(26);
    expect(
      await screen.findByText(
        "Câu 26 trên 34 · Điền từ vào đoạn văn · 1 trong 5 chỗ trống",
      ),
    ).toBeVisible();
    expect(screen.queryByRole("group", { name: /^Chỗ trống/ })).toBeNull();
    expect(screen.getByRole("img")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Đánh dấu câu 26" })).toBeVisible();
    expect(
      within(passage()).getByRole("button", { name: "Chỗ trống 26, trống" }),
    ).toHaveAttribute("data-state", "open");

    await user.click(screen.getByRole("button", { name: "Câu sau" }));
    expect(
      screen.getByText("Câu 27 trên 34 · Điền từ vào đoạn văn · 1 trong 5 chỗ trống"),
    ).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Câu trước" }));
    await user.click(screen.getByRole("button", { name: "Câu trước" }));
    expect(card()).toHaveAccessibleName("Chỗ trống 23");
  });
});

describe("the squares of a run", () => {
  const rail = () => screen.getByRole("complementary", { name: "Các phần" });
  const square = (no: number) =>
    within(rail()).getByRole("button", { name: new RegExp(`^Câu ${no},`) });
  const tied = (no: number) => square(no).querySelector(".bg-ring") !== null;

  it("are tied within a row, named for the run, and ringed while the run is open", async () => {
    const user = userEvent.setup();
    mount(24);
    await screen.findByText(RUN_LINE);
    expect(square(24)).toHaveAccessibleName(
      "Câu 24, Section 3 · Reading, đang xem, bài điền từ câu 23 đến 27",
    );
    expect(square(24)).toHaveAttribute("aria-current", "true");
    expect(square(24)).toHaveClass("border-primary", "ring-brand");
    for (const no of [23, 25, 26, 27]) {
      expect(square(no)).toHaveClass("ring-brand-soft");
      expect(square(no)).not.toHaveAttribute("aria-current");
    }
    expect([22, 23, 24, 25, 26, 27, 28].map(tied)).toEqual([
      false,
      true,
      true,
      false,
      true,
      false,
      false,
    ]);
    expect(within(rail()).getByText("Cùng màn hình")).toBeVisible();

    await user.click(square(27));
    expect(card()).toHaveAccessibleName("Chỗ trống 27");
    await user.click(screen.getByRole("button", { name: "Câu sau" }));
    expect(square(24)).not.toHaveClass("ring-brand-soft");
    expect(tied(24)).toBe(true);
  });

  it("each keep their own state, and none reads partly answered", async () => {
    const user = userEvent.setup();
    mount();
    await screen.findByText(RUN_LINE);
    await user.click(option("relax"));
    expect(square(23)).toHaveAccessibleName(
      "Câu 23, Section 3 · Reading, đã trả lời, bài điền từ câu 23 đến 27",
    );
    expect(square(24)).toHaveAccessibleName(
      "Câu 24, Section 3 · Reading, đang xem, bài điền từ câu 23 đến 27",
    );
    expect(rail()).not.toHaveTextContent(/trả lời một phần,/);
  });

  it("are tied in the sheet as well, where a row holds six", async () => {
    viewport("phone");
    const user = userEvent.setup();
    mount();
    await screen.findByText(RUN_LINE);
    await user.click(screen.getByRole("button", { name: /^Câu 23 · đã làm/ }));
    const sheet = screen.getByRole("dialog");
    const inSheet = (no: number) =>
      within(sheet)
        .getByRole("button", { name: new RegExp(`^Câu ${no},`) })
        .querySelector(".bg-ring") !== null;
    expect([23, 24, 25, 26, 27].map(inSheet)).toEqual([true, true, true, false, false]);
    expect(within(sheet).getByText("Cùng màn hình")).toBeVisible();
  });
});
