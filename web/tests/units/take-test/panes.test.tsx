import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import {
  getAttempt,
  saveAnswers,
  type AttemptSession,
  type StudentGroup,
  type StudentQuestion,
} from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { writeLargerTestText } from "@/lib/testText";
import i18n from "@/lib/i18n";
import { contractJson } from "@tests/support/contractResponse";
import {
  DECK_ATTEMPT,
  DECK_PASSAGE,
  deckPassageSession,
  deckSaved,
  deckSession,
} from "./deckSession";
import { viewport } from "./support";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  recordGroupAudioPlay: vi.fn(),
}));

const NOW = new Date("2026-09-22T05:00:00.000Z");
const store = () => useTakeTestStore.getState();
const SWITCHER = "Xem ngữ liệu hoặc câu hỏi";

async function open(paper: AttemptSession) {
  vi.mocked(getAttempt).mockResolvedValue(paper);
  const router = createMemoryRouter(
    [{ path: "/app/attempts/:attemptId", element: <TakeTestPage /> }],
    { initialEntries: [`/app/attempts/${DECK_ATTEMPT}`] },
  );
  const view = render(<RouterProvider router={router} />);
  await screen.findByRole("main");
  return view;
}

const passage = () => screen.getByRole("article", { name: DECK_PASSAGE });
const sheet = () => document.querySelector<HTMLElement>('[id^="answer-question-"]')!;
const questionPane = () => sheet().closest("section")!;
const switcher = () => screen.queryByRole("group", { name: SWITCHER });
const next = () => screen.getByRole("button", { name: "Câu sau" });

function twoParts(paper: AttemptSession): AttemptSession {
  const second = "018f0000-0000-7000-8000-00000000a002";
  return {
    ...paper,
    sections: [
      { id: paper.sections[0]!.id, title: "Phần 1 · Đọc", instructions: null },
      { id: second, title: "Phần 2 · Viết", instructions: "Viết câu trả lời ngắn." },
    ],
    questions: paper.questions.map((question, index) =>
      index < 4 ? question : { ...question, sectionId: second },
    ),
  };
}

function listening(paper: AttemptSession): AttemptSession {
  const asset = "018f0000-0000-7000-8000-00000000d001";
  const group: StudentGroup = {
    id: "018f0000-0000-7000-8000-00000000d002",
    sectionId: paper.sections[0]!.id,
    title: "Hội thoại ở bưu điện",
    questionIds: paper.questions.slice(0, 2).map((question) => question.id),
    instructions: {
      format: "semantic_v1",
      blocks: [
        {
          type: "paragraph",
          content: [{ type: "text", text: "Nghe rồi trả lời hai câu.", marks: [] }],
        },
      ],
    },
    stimuli: [],
    recordings: [
      {
        id: "018f0000-0000-7000-8000-00000000d003",
        assetId: asset,
        policy: { maxPlays: 2, allowSeek: false, showTranscriptAfterSubmit: false },
      },
    ],
    assets: [
      {
        id: asset,
        kind: "audio",
        mimeType: "audio/mpeg",
        bytes: 2048,
        durationMs: 30_000,
        originalFilename: "post-office.mp3",
        createdAt: "2026-09-20T00:00:00Z",
        url: "https://assets.example/post-office.mp3",
      },
    ],
  };
  return { ...paper, groups: [group], groupAudioPlays: {} };
}

beforeEach(async () => {
  viewport("desktop");
  localStorage.clear();
  sessionStorage.clear();
  store().reset();
  vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => undefined);
  vi.mocked(saveAnswers)
    .mockReset()
    .mockImplementation(async () =>
      deckSaved(new Date(), new Date(store().deadlineAt).toISOString()),
    );
  await i18n.changeLanguage("vi");
});

afterEach(async () => {
  store().reset();
  act(() => writeLargerTestText(false));
  localStorage.clear();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  await i18n.changeLanguage("vi");
});

describe("the deck's fixtures", () => {
  it("are payloads the contract accepts", () => {
    expect(() =>
      contractJson("/app/attempts/{id}", "get", 200, deckPassageSession(NOW)),
    ).not.toThrow();
    expect(() =>
      contractJson("/app/attempts/{id}", "get", 200, listening(deckSession(NOW))),
    ).not.toThrow();
  });
});

describe("the panes from 768", () => {
  it("splits a question that has a passage into the passage pane and the question pane", async () => {
    await open(deckPassageSession(NOW));

    expect(passage()).toHaveClass(
      "flex-[1_1_0]",
      "overflow-y-auto",
      "select-none",
      "border-r",
      "px-8",
      "py-7",
    );
    expect(passage()).toBeVisible();
    expect(passage()).toHaveAttribute("tabindex", "0");
    expect(passage()).toHaveClass("-outline-offset-2!");
    expect(questionPane()).toHaveClass("bg-sidebar", "flex-[1_1_0]");
    expect(questionPane()).toBeVisible();
    expect(sheet().parentElement).toHaveClass("overflow-y-auto", "px-8", "py-7");
    expect(sheet()).toHaveClass("mx-auto", "max-w-150", "gap-4.5");
    expect(
      passage().compareDocumentPosition(questionPane()) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.getByRole("main")).toContainElement(passage());
    expect(switcher()).toBeNull();
  });

  it("draws the passage as the part eyebrow, the title and the paragraphs at 16px", async () => {
    await open(deckPassageSession(NOW));
    const pane = within(passage());

    const eyebrow = pane.getByText("Reading passage 1");
    expect(eyebrow).toHaveClass("text-meta", "font-semibold", "uppercase");
    const title = pane.getByRole("heading", { level: 2, name: DECK_PASSAGE });
    expect(title).toHaveClass("text-stat", "leading-[1.3]", "font-semibold");
    expect(title.parentElement).toHaveClass("mx-auto", "max-w-160", "gap-3.5");
    expect(pane.getAllByRole("heading")).toHaveLength(1);

    const first = pane.getByText(/Planners once treated parks as a luxury/);
    expect(first.closest("p")?.textContent).toMatch(/^A\u2002Planners once/);
    expect(pane.getByText("A").tagName).toBe("STRONG");
    expect(passage().querySelectorAll("p")).toHaveLength(5);
    expect(first.closest(".text-title")).toHaveClass(
      "leading-[1.75]",
      "text-pretty",
      "[&_.semantic-content>*+*]:mt-3.5!",
      "[&>div]:gap-3.5",
    );
    expect(pane.queryByRole("alert")).toBeNull();
  });

  it("uses the question pane alone, centred, when the question has no passage", async () => {
    await open(deckSession(NOW));

    expect(screen.queryByRole("article")).toBeNull();
    expect(screen.getByRole("main").children).toHaveLength(1);
    expect(questionPane()).toHaveClass("bg-sidebar", "flex-[1_1_0]");
    expect(sheet()).toHaveClass("mx-auto", "max-w-150");
    expect(switcher()).toBeNull();
  });

  it("names the question's position and type, one label for each type", async () => {
    const user = userEvent.setup();
    const paper = deckSession(NOW);
    await open({
      ...paper,
      questions: [
        ...paper.questions.slice(0, 5),
        {
          ...paper.questions[5]!,
          type: "true_false",
          options: paper.questions[5]!.options!.slice(0, 2),
        },
        {
          ...paper.questions[6]!,
          type: "fill_blank",
          prompt: "Locals call them {{1}} streets.",
          blanks: [
            {
              id: "018f0000-0000-7000-8000-00000000e001",
              ordinal: 1,
              caseSensitive: false,
            },
          ],
        },
        paper.questions[7]!,
      ],
    });
    const line = () => within(sheet()).getByText(/^Câu \d trên 8/);

    expect(line()).toHaveTextContent("Câu 1 trên 8 · Chọn một đáp án");
    expect(line()).toHaveClass("text-muted-fg", "text-sm", "font-medium");
    for (const label of [
      "Câu 2 trên 8 · Chọn một đáp án",
      "Câu 3 trên 8 · Chọn một đáp án",
      "Câu 4 trên 8 · Chọn nhiều đáp án",
      "Câu 5 trên 8 · Trả lời ngắn",
      "Câu 6 trên 8 · Đúng hay sai",
      "Câu 7 trên 8 · Điền vào chỗ trống",
    ]) {
      await user.click(next());
      expect(line()).toHaveTextContent(label);
    }
  });

  it("is the deck's line in English", async () => {
    await i18n.changeLanguage("en");
    const user = userEvent.setup();
    await open(deckSession(NOW));
    for (let moves = 0; moves < 3; moves++) {
      await user.click(screen.getByRole("button", { name: "Next" }));
    }
    expect(screen.getByText("Question 4 of 8 · Choose one or more")).toBeVisible();
    expect(screen.getByRole("button", { name: "Flag for review" })).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Flag for review" }));
    expect(screen.getByRole("button", { name: "Flagged" })).toBeVisible();
  });

  it("flags from the toggle beside the line, which takes the warning tones", async () => {
    const user = userEvent.setup();
    await open(deckSession(NOW));
    const off = screen.getByRole("button", { name: "Đánh dấu xem lại" });
    expect(off).toHaveAttribute("aria-pressed", "false");
    expect(off).toHaveClass("h-8", "min-h-0", "border-border", "bg-card", "text-fg");
    expect(off.parentElement).toContainElement(
      screen.getByText("Câu 1 trên 8 · Chọn một đáp án"),
    );

    await user.click(off);
    const on = screen.getByRole("button", { name: "Đã đánh dấu" });
    expect(on).toHaveAttribute("aria-pressed", "true");
    expect(on).toHaveClass("border-warning", "bg-warning-soft", "text-warning-ink");
    expect(store().flags.has(deckSession(NOW).questions[0]!.id)).toBe(true);

    await user.click(next());
    expect(screen.getByRole("button", { name: "Đánh dấu xem lại" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    await user.click(screen.getByRole("button", { name: "Câu trước" }));
    await user.click(screen.getByRole("button", { name: "Đã đánh dấu" }));
    expect(store().flags.size).toBe(0);
  });

  it("cannot flag on a paper that is locked", async () => {
    await open(deckSession(NOW));
    act(() => store().lockNow("superseded"));
    expect(screen.getByRole("button", { name: "Đánh dấu xem lại" })).toBeDisabled();
  });

  it("names the part above a question with no passage, when the paper has more than one", async () => {
    const user = userEvent.setup();
    await open(twoParts(deckSession(NOW)));

    const part = within(sheet()).getByText("Phần 1 · Đọc");
    expect(part).toHaveClass("text-meta", "font-semibold", "uppercase");
    expect(screen.queryByRole("note")).toBeNull();
    for (let moves = 0; moves < 4; moves++) await user.click(next());
    expect(within(sheet()).getByText("Phần 2 · Viết")).toBeVisible();
    expect(screen.getByRole("note")).toHaveTextContent("Viết câu trả lời ngắn.");
    await user.click(next());
    expect(screen.queryByRole("note")).toBeNull();
  });

  it("leaves a one-part paper's question pane without a part line", async () => {
    await open(deckSession(NOW));
    expect(within(sheet()).queryByText("Reading passage 1")).toBeNull();
  });

  it("names the part in the passage, not twice, when the question has one", async () => {
    await open(twoParts(deckPassageSession(NOW)));
    expect(within(passage()).getByText("Phần 1 · Đọc")).toBeVisible();
    expect(within(sheet()).queryByText("Phần 1 · Đọc")).toBeNull();
  });

  it("leads a listening part with the note that names it", async () => {
    const paper = deckSession(NOW);
    await open({
      ...paper,
      sections: [
        {
          id: paper.sections[0]!.id,
          title: "Phần 1 · Nghe",
          instructions: "Mỗi đoạn nghe được hai lần.",
        },
      ],
      questions: [
        {
          ...paper.questions[0]!,
          media: {
            id: "018f0000-0000-7000-8000-00000000f001",
            kind: "audio",
            url: "https://assets.example/q1.mp3",
            mimeType: "audio/mpeg",
            bytes: 2048,
            durationMs: 20_000,
            originalFilename: "q1.mp3",
            createdAt: "2026-09-20T00:00:00Z",
          },
          audio: { maxPlays: 2, allowSeek: false, showTranscriptAfterSubmit: false },
        },
        ...paper.questions.slice(1),
      ],
    });

    const note = screen.getByRole("note");
    expect(note).toHaveClass("bg-muted", "rounded-xl");
    expect(within(note).getByText("Phần 1 · Nghe")).toHaveClass("font-medium");
    expect(note).toHaveTextContent("Mỗi đoạn nghe được hai lần.");
    const play = within(sheet()).getByRole("button", { name: "Phát" });
    expect(play).toBeVisible();
    expect(play).toHaveClass("in-data-[scale=deck]:size-10");
    expect(play.parentElement).toHaveClass(
      "in-data-[scale=deck]:bg-card",
      "in-data-[scale=deck]:rounded-[11px]",
      "in-data-[scale=deck]:border-[1.5px]",
      "in-data-[scale=deck]:px-3.5",
      "in-data-[scale=deck]:py-2.5",
    );
    expect(within(sheet()).getByText("Còn 2 lượt nghe")).toHaveClass(
      "in-data-[scale=deck]:text-meta",
    );
  });

  it("draws a group that has only recordings inside the question pane", async () => {
    const user = userEvent.setup();
    const { container } = await open(listening(deckSession(NOW)));

    expect(screen.queryByRole("article")).toBeNull();
    const pane = within(sheet());
    expect(
      pane.getByRole("heading", { level: 2, name: "Hội thoại ở bưu điện" }),
    ).toBeVisible();
    expect(pane.getByText("Nghe rồi trả lời hai câu.")).toBeVisible();
    expect(pane.getByRole("group", { name: "Bài nghe 1" })).toBeVisible();
    expect(pane.getByText("Còn 2 lượt nghe")).toBeVisible();
    const audio = container.querySelector("audio");

    await user.click(next());
    expect(container.querySelector("audio")).toBe(audio);
    await user.click(next());
    expect(container.querySelector("audio")).toBeNull();
    expect(within(sheet()).queryByRole("heading")).toBeNull();
  });

  it("draws a question's own image in the question pane, on the paper surface", async () => {
    const paper = deckSession(NOW);
    await open({
      ...paper,
      questions: [
        {
          ...paper.questions[0]!,
          media: {
            id: "018f0000-0000-7000-8000-00000000f002",
            kind: "image",
            url: "https://assets.example/map.png",
            mimeType: "image/png",
            bytes: 4096,
            originalFilename: "map.png",
            createdAt: "2026-09-20T00:00:00Z",
          },
        },
        ...paper.questions.slice(1),
      ],
    });

    const image = within(sheet()).getByAltText("map.png");
    expect(image).toHaveAttribute("src", "https://assets.example/map.png");
    expect(onPaper(image)).toBe(true);
    expect(within(sheet()).queryByRole("button", { name: "Phát" })).toBeNull();
  });

  it("keeps the paper surface for images and rich tables in the passage and the question", async () => {
    await open(deckPassageSession(NOW));
    const recipe = [
      "[&_img]:bg-paper",
      "[&_.content-table-scroll]:bg-paper",
      "[&_.content-table-scroll]:text-paper-fg",
    ];
    expect(
      within(passage()).getByRole("heading", { name: DECK_PASSAGE }).parentElement,
    ).toHaveClass(...recipe);
    expect(
      screen.getByText("Câu 1 trên 8 · Chọn một đáp án").parentElement?.parentElement,
    ).toHaveClass(...recipe);
  });
});

describe("the panes below 768", () => {
  beforeEach(() => {
    viewport("phone");
  });

  it("puts the Passage | Question n switcher under the header, with the question first", async () => {
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));

    const tabs = switcher()!;
    expect(tabs).toHaveClass("flex-none", "gap-1.5", "border-b", "px-3.5", "py-2");
    expect(
      screen.getByRole("banner").compareDocumentPosition(tabs) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(
      tabs.compareDocumentPosition(screen.getByRole("main")) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    const [read, answer] = within(tabs).getAllByRole("button");
    expect(read).toHaveTextContent("Ngữ liệu");
    expect(answer).toHaveTextContent("Câu 1");
    expect(answer).toHaveAttribute("aria-pressed", "true");
    expect(answer).toHaveClass("h-8.5", "min-h-0", "flex-1", "bg-primary");
    expect(read).toHaveAttribute("aria-pressed", "false");
    expect(read).toHaveClass("bg-muted", "text-fg");

    expect(questionPane()).toBeVisible();
    expect(screen.getByRole("article", { hidden: true })).not.toBeVisible();
    expect(sheet().parentElement).toHaveClass("px-4", "py-4.5");

    await user.click(read!);
    expect(passage()).toBeVisible();
    expect(passage()).toHaveClass("px-4", "py-4.5");
    expect(passage()).not.toHaveClass("border-r");
    expect(questionPane()).not.toBeVisible();
    expect(read).toHaveAttribute("aria-pressed", "true");
    expect(screen.queryByRole("contentinfo")).toBeNull();

    await user.click(answer!);
    expect(questionPane()).toBeVisible();
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();
  });

  it("names the question in the switcher as the student moves", async () => {
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    await user.click(next());
    expect(within(switcher()!).getByRole("button", { name: "Câu 2" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("takes no answer and no flag from the keys while the passage hides the question", async () => {
    const user = userEvent.setup();
    const paper = deckPassageSession(NOW);
    const fourth = paper.questions[3]!;
    await open(paper);
    for (let moves = 0; moves < 3; moves++) await user.click(next());
    await user.click(within(switcher()!).getByRole("button", { name: "Ngữ liệu" }));

    await user.keyboard("a");
    await user.keyboard("f");
    expect(store().answers[fourth.id]).toBeUndefined();
    expect(store().flags.has(fourth.id)).toBe(false);

    await user.click(within(switcher()!).getByRole("button", { name: "Câu 4" }));
    await user.keyboard("a");
    await user.keyboard("f");
    expect(store().answers[fourth.id]).toEqual({
      type: "choice",
      optionIds: [fourth.options![0]!.id],
    });
    expect(store().flags.has(fourth.id)).toBe(true);
  });

  it("still moves to the next question from the arrow keys while the passage shows, and shows it", async () => {
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    await user.click(within(switcher()!).getByRole("button", { name: "Ngữ liệu" }));
    expect(questionPane()).not.toBeVisible();

    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("main", { name: "Câu 2" })).toBeInTheDocument();
    expect(questionPane()).toBeVisible();
    expect(screen.getByRole("article", { hidden: true })).not.toBeVisible();
  });

  it("shows the question again on coming back to one whose passage was open", async () => {
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    await user.click(within(switcher()!).getByRole("button", { name: "Ngữ liệu" }));

    await user.keyboard("{ArrowRight}");
    await user.keyboard("{ArrowLeft}");
    expect(screen.getByRole("main", { name: "Câu 1" })).toBeInTheDocument();
    expect(questionPane()).toBeVisible();
    expect(screen.getByRole("contentinfo")).toBeInTheDocument();
  });

  it("shows the question again on coming back by touch after the window was wide", async () => {
    const view = viewport("phone");
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    await user.click(within(switcher()!).getByRole("button", { name: "Ngữ liệu" }));

    act(() => view.resize("desktop"));
    await user.click(next());
    act(() => view.resize("phone"));
    await user.click(screen.getByRole("button", { name: "Câu trước" }));
    expect(screen.getByRole("main", { name: "Câu 1" })).toBeInTheDocument();
    expect(questionPane()).toBeVisible();
  });

  it("keeps the student's place in the question while the passage is showing", async () => {
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    const pane = sheet().parentElement!;
    pane.scrollTop = 160;
    fireEvent.scroll(pane);

    await user.click(within(switcher()!).getByRole("button", { name: "Ngữ liệu" }));
    pane.scrollTop = 0;
    await user.click(within(switcher()!).getByRole("button", { name: "Câu 1" }));
    expect(pane.scrollTop).toBe(160);
  });

  it("has no switcher when the question has no passage", async () => {
    await open(deckSession(NOW));
    expect(switcher()).toBeNull();
    expect(questionPane()).toBeVisible();
  });

  it("has no switcher for a group that has only recordings", async () => {
    await open(listening(deckSession(NOW)));
    expect(switcher()).toBeNull();
    expect(within(sheet()).getByRole("button", { name: "Phát" })).toBeVisible();
  });

  it("shows both panes again when a phone showing the passage grows past 768", async () => {
    const view = viewport("phone");
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    await user.click(within(switcher()!).getByRole("button", { name: "Ngữ liệu" }));
    expect(questionPane()).not.toBeVisible();

    act(() => view.resize("desktop"));
    expect(passage()).toBeVisible();
    expect(questionPane()).toBeVisible();

    act(() => view.resize("phone"));
    expect(passage()).toBeVisible();
    expect(questionPane()).not.toBeVisible();
  });

  it("keeps an answer being typed when the window crosses 768 on a paper with a passage", async () => {
    const view = viewport("desktop");
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    for (let moves = 0; moves < 4; moves++) await user.click(next());
    await user.type(screen.getByRole("textbox"), "green space");

    act(() => view.resize("phone"));
    expect(switcher()).not.toBeNull();
    expect(screen.getByRole("textbox")).toHaveFocus();
    expect(screen.getByRole("textbox")).toHaveValue("green space");

    act(() => view.resize("desktop"));
    expect(switcher()).toBeNull();
    expect(passage()).toBeVisible();
    expect(screen.getByRole("textbox")).toHaveFocus();
  });
});

describe("larger text in tests", () => {
  it("is 16, 17 and 15px by default", async () => {
    await open(deckPassageSession(NOW));
    expect(
      within(passage())
        .getByText(/Planners once/)
        .closest(".text-pretty"),
    ).toHaveClass("text-title");
    expect(
      screen.getByText("What is the main purpose of the passage?").parentElement,
    ).toHaveClass("text-lg", "font-medium");
    expect(
      screen.getByRole("radio", { name: "To describe the history of city parks" })
        .parentElement,
    ).toHaveClass("text-md");
  });

  it("sets the passage, the prompt and the options to 18, 19 and 17px", async () => {
    localStorage.setItem("quizzivy.testText", "large");
    await open(deckPassageSession(NOW));

    const text = within(passage())
      .getByText(/Planners once/)
      .closest(".text-pretty");
    expect(text).toHaveClass("text-stat-sm");
    expect(text).not.toHaveClass("text-title");
    const prompt = screen.getByText("What is the main purpose of the passage?");
    expect(prompt.parentElement).toHaveClass("text-[1.1875rem]");
    expect(prompt.parentElement).not.toHaveClass("text-lg");
    const option = screen.getByRole("radio", {
      name: "To describe the history of city parks",
    }).parentElement;
    expect(option).toHaveClass("text-lg");
    expect(option).not.toHaveClass("text-md");
  });

  it("follows the setting while the paper is open, and sizes the answer field with it", async () => {
    const user = userEvent.setup();
    await open(deckSession(NOW));
    for (let moves = 0; moves < 4; moves++) await user.click(next());
    expect(screen.getByRole("textbox")).toHaveClass("lg:text-md", "py-[13px]");

    act(() => writeLargerTestText(true));
    expect(screen.getByRole("textbox")).toHaveClass("text-lg", "py-3");
    expect(screen.getByRole("textbox")).not.toHaveClass("lg:text-md");
  });
});

describe("an unknown question type", () => {
  const fabricated = (paper: AttemptSession): AttemptSession => ({
    ...paper,
    questions: [
      { ...paper.questions[0]!, type: "matching" as StudentQuestion["type"] },
      ...paper.questions.slice(1),
    ],
    answers: {},
  });

  it("draws the reload block, with no options and no type in the line", async () => {
    await open(fabricated(deckSession(NOW)));

    expect(screen.getByText("Câu 1 trên 8")).toBeVisible();
    expect(within(sheet()).getByRole("status")).toHaveTextContent(
      "Câu 1 thuộc dạng câu hỏi mới mà trang này chưa hiển thị được.",
    );
    expect(screen.getByRole("button", { name: "Tải lại trang" })).toBeVisible();
    expect(screen.queryByRole("radio")).toBeNull();
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(
      screen.queryByText("To compare parks in European and Asian cities"),
    ).toBeNull();
    expect(screen.queryByText(/1 điểm/)).toBeNull();
    expect(screen.getByText(/Phím tắt/)).not.toHaveTextContent("chọn đáp án");
  });

  it("writes no answer from the keys either", async () => {
    const user = userEvent.setup();
    const paper = fabricated(deckSession(NOW));
    await open(paper);

    await user.keyboard("a");
    await user.keyboard("b");
    expect(store().answers[paper.questions[0]!.id]).toBeUndefined();
    expect(store().dirty.size).toBe(0);
    expect(saveAnswers).not.toHaveBeenCalled();

    await user.click(next());
    await user.keyboard("a");
    expect(store().answers[paper.questions[1]!.id]).toEqual({
      type: "choice",
      optionIds: [paper.questions[1]!.options![0]!.id],
    });
  });
});

describe("pasting into an answer", () => {
  async function field(blockCopyPaste: boolean) {
    const user = userEvent.setup();
    const paper = deckSession(NOW);
    await open({ ...paper, integrity: { ...paper.integrity, blockCopyPaste } });
    for (let moves = 0; moves < 4; moves++) await user.click(next());
    return screen.getByRole("textbox", { name: "Bài làm của bạn" });
  }

  it("is refused when the policy blocks copy and paste", async () => {
    const answer = await field(true);
    expect(fireEvent.paste(answer)).toBe(false);
    expect(fireEvent.copy(passageless())).toBe(false);
  });

  it("is allowed when the policy does not", async () => {
    const answer = await field(false);
    expect(fireEvent.paste(answer)).toBe(true);
  });
});

function onPaper(element: HTMLElement): boolean {
  for (let at = element.parentElement; at !== null; at = at.parentElement) {
    if (at.classList.contains("[&_img]:bg-paper")) return true;
  }
  return false;
}

function passageless() {
  return screen.getByText(
    "What should cities prioritise when they plan new districts?",
  );
}
