import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
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
const switcher = () =>
  screen.queryByRole("group", { name: "Xem ngữ liệu hoặc câu hỏi" });
const next = () => screen.getByRole("button", { name: "Câu sau" });
const footer = () => screen.queryByRole("navigation", { name: "Danh sách câu" });

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
        policy: {
          maxPlays: 2,
          allowSeek: false,
          showTranscriptAfterSubmit: false,
          allowPause: true,
        },
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
    expect(sheet()).toHaveClass("mx-auto", "max-w-150", "gap-4.5", "outline-none!");
    expect(
      passage().compareDocumentPosition(questionPane()) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.getByRole("main")).toContainElement(passage());
    expect(switcher()).toBeNull();
    expect(within(sheet()).queryByRole("heading")).toBeNull();
    expect(sheet().querySelectorAll("div:empty")).toHaveLength(0);
  });

  it("draws the passage as the kicker, the heading and the paragraphs at 16px", async () => {
    await open(deckPassageSession(NOW));
    const pane = within(passage());

    const kicker = pane.getByText("Reading passage 1");
    expect(kicker).toHaveClass(
      "text-meta",
      "font-semibold",
      "text-muted-fg",
      "uppercase",
      "tracking-[0.04em]",
    );
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
    expect(screen.getByRole("main").children).toHaveLength(2);
    expect(screen.getByRole("main").firstElementChild).toBe(
      screen.getByRole("complementary", { name: "Các phần" }),
    );
    expect(questionPane()).toHaveClass("bg-bg", "flex-[1_1_0]");
    expect(questionPane()).not.toHaveClass("bg-sidebar");
    expect(sheet()).toHaveClass("mx-auto", "max-w-180");
    expect(sheet()).not.toHaveClass("max-w-150");
    expect(switcher()).toBeNull();
  });

  it("puts the question pane on the sidebar surface, in a 600px column, beside shared content only", async () => {
    await open(deckPassageSession(NOW));
    expect(questionPane()).toHaveClass("bg-sidebar");
    expect(questionPane()).not.toHaveClass("bg-bg");
    expect(sheet()).toHaveClass("max-w-150");
    expect(sheet()).not.toHaveClass("max-w-180");
  });

  it("lays short options in columns alone, and in one column beside the passage", async () => {
    const user = userEvent.setup();
    const { unmount } = await open(deckSession(NOW));
    await user.click(next());
    await user.click(next());
    expect(within(sheet()).getByRole("radiogroup")).toHaveClass("grid-cols-3");
    unmount();
    store().reset();
    sessionStorage.clear();

    await open(deckPassageSession(NOW));
    await user.click(next());
    await user.click(next());
    expect(within(sheet()).getByRole("radiogroup")).toHaveClass("grid-cols-1");
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

    expect(line()).toHaveTextContent("Câu 1 trên 8 · Một đáp án");
    expect(line()).toHaveClass("text-muted-fg", "text-sm", "font-medium");
    for (const label of [
      "Câu 2 trên 8 · Một đáp án",
      "Câu 3 trên 8 · Một đáp án",
      "Câu 4 trên 8 · Nhiều đáp án",
      "Câu 5 trên 8 · Tự luận",
      "Câu 6 trên 8 · Đúng/Sai",
      "Câu 7 trên 8 · Điền từ",
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
      await user.click(screen.getByRole("button", { name: "Next question" }));
    }
    expect(screen.getByText("Question 4 of 8 · Multiple choice")).toBeVisible();
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
      screen.getByText("Câu 1 trên 8 · Một đáp án"),
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

  it("heads every question with its section's title and instructions on a paper of several", async () => {
    const user = userEvent.setup();
    await open(twoParts(deckSession(NOW)));

    const head = () => sheet().firstElementChild as HTMLElement;
    expect(head()).toHaveClass("flex", "flex-col", "gap-0.5", "border-b", "pb-3");
    const title = within(head()).getByText("Phần 1 · Đọc");
    expect(title).toHaveClass(
      "text-xs",
      "font-semibold",
      "text-muted-fg",
      "uppercase",
      "tracking-[0.04em]",
    );
    expect(head().children).toHaveLength(1);
    expect(screen.queryByRole("note")).toBeNull();
    expect(
      head().compareDocumentPosition(within(sheet()).getByText(/^Câu 1 trên 8/)) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();

    for (let moves = 0; moves < 4; moves++) await user.click(next());
    expect(within(head()).getByText("Phần 2 · Viết")).toBeVisible();
    const instructions = within(head()).getByText("Viết câu trả lời ngắn.");
    expect(instructions).toHaveClass("text-ui", "text-muted-fg");
    await user.click(next());
    expect(within(head()).getByText("Viết câu trả lời ngắn.")).toBeVisible();
    expect(within(sheet()).getAllByText("Viết câu trả lời ngắn.")).toHaveLength(1);
  });

  it("leaves a one-part paper's question pane without a head when the part has no instructions", async () => {
    await open(deckSession(NOW));
    expect(within(sheet()).queryByText("Reading passage 1")).toBeNull();
    expect(sheet().querySelector(".border-b")).toBeNull();
  });

  it("heads a one-part paper's questions when the part has instructions", async () => {
    const user = userEvent.setup();
    const paper = deckSession(NOW);
    await open({
      ...paper,
      sections: [{ ...paper.sections[0]!, instructions: "Read, then answer 1–8." }],
    });
    expect(within(sheet()).getByText("Reading passage 1")).toBeVisible();
    expect(within(sheet()).getByText("Read, then answer 1–8.")).toBeVisible();
    await user.click(next());
    expect(within(sheet()).getByText("Read, then answer 1–8.")).toBeVisible();
  });

  it("names the section in the passage's kicker when the heading is the group's title, and in the head", async () => {
    await open(twoParts(deckPassageSession(NOW)));
    expect(within(passage()).getByText("Phần 1 · Đọc")).toBeVisible();
    expect(
      within(passage()).getByRole("heading", { level: 2, name: DECK_PASSAGE }),
    ).toBeVisible();
    expect(within(sheet()).getByText("Phần 1 · Đọc")).toBeVisible();
  });

  it("states a listening part's instructions in the head, with no note", async () => {
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
          audio: {
            maxPlays: 2,
            allowSeek: false,
            showTranscriptAfterSubmit: false,
            allowPause: true,
          },
        },
        ...paper.questions.slice(1),
      ],
    });

    expect(screen.queryByRole("note")).toBeNull();
    const head = sheet().firstElementChild as HTMLElement;
    expect(within(head).getByText("Phần 1 · Nghe")).toHaveClass("uppercase");
    expect(head).toHaveTextContent("Mỗi đoạn nghe được hai lần.");
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
    const recording = pane.getByRole("group", { name: "Bài nghe 1" });
    expect(recording).toBeVisible();
    expect(
      recording.compareDocumentPosition(pane.getByRole("radiogroup")) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
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

    const image = within(sheet()).getByAltText("Hình ảnh của câu hỏi");
    expect(screen.queryByAltText("map.png")).toBeNull();
    expect(image).toHaveAttribute("src", "https://assets.example/map.png");
    expect(onPaper(image)).toBe(true);
    expect(within(sheet()).queryByRole("button", { name: "Phát" })).toBeNull();

    const loads = vi.mocked(getAttempt).mock.calls.length;
    fireEvent.error(image);
    await userEvent.click(within(sheet()).getByRole("button", { name: "Thử lại" }));
    await waitFor(() => expect(getAttempt).toHaveBeenCalledTimes(loads + 1));
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
      screen.getByText("Câu 1 trên 8 · Một đáp án").parentElement?.parentElement,
    ).toHaveClass(...recipe);
  });

  it("starts another group's passage at its top", async () => {
    const user = userEvent.setup();
    const paper = deckPassageSession(NOW);
    const first = paper.groups![0]!;
    await open({
      ...paper,
      groups: [
        { ...first, questionIds: first.questionIds.slice(0, 1) },
        {
          ...first,
          id: "018f0000-0000-7000-8000-000000009003",
          title: "Play streets",
          questionIds: first.questionIds.slice(1),
          stimuli: [
            {
              ...first.stimuli[0]!,
              id: "018f0000-0000-7000-8000-000000009004",
              title: "Play streets",
            },
          ],
        },
      ],
    });
    passage().scrollTop = 320;
    fireEvent.scroll(passage());

    await user.click(next());
    expect(screen.getByRole("article", { name: "Play streets" }).scrollTop).toBe(0);
  });
});

describe("the panes below 768", () => {
  beforeEach(() => {
    viewport("phone");
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
      x: 0,
      y: 0,
      top: 0,
      left: 0,
      right: 360,
      bottom: 700,
      width: 360,
      height: 700,
      toJSON: () => ({}),
    });
  });

  const W = Math.round(360 * 0.88);
  const handle = () => screen.queryByRole("button", { name: "Mở ngữ liệu" });
  const drawer = () => screen.getByRole("article", { hidden: true });
  const backdrop = () => document.querySelector<HTMLElement>("[data-drawer-backdrop]");
  const closer = () => screen.getByRole("button", { name: "Đóng" });
  const panes = () => screen.getByRole("main");
  const isClosed = () => {
    expect(drawer()).toHaveAttribute("inert");
    expect(drawer()).toHaveClass("-translate-x-full");
    expect(drawer()).not.toHaveAttribute("data-drawer-open");
    expect(backdrop()).toBeNull();
    expect(handle()).toBeInTheDocument();
  };
  const swipe = (from: [number, number], to: [number, number]) => {
    fireEvent.pointerDown(panes(), {
      clientX: from[0],
      clientY: from[1],
      pointerType: "touch",
    });
    fireEvent.pointerMove(panes(), {
      clientX: (from[0] + to[0]) / 2,
      clientY: (from[1] + to[1]) / 2,
      pointerType: "touch",
    });
    fireEvent.pointerMove(panes(), {
      clientX: to[0],
      clientY: to[1],
      pointerType: "touch",
    });
    fireEvent.pointerUp(panes(), {
      clientX: to[0],
      clientY: to[1],
      pointerType: "touch",
    });
  };

  it("draws the passage as a closed drawer from the left behind a handle, with the question first", async () => {
    await open(deckPassageSession(NOW));

    expect(switcher()).toBeNull();
    expect(questionPane()).toBeVisible();
    expect(panes()).toHaveClass("relative");
    expect(panes()).toHaveStyle({ touchAction: "pan-y" });
    expect(drawer()).toHaveClass(
      "absolute",
      "inset-y-0",
      "left-0",
      "z-60",
      "w-[min(88%,420px)]",
      "bg-bg",
      "px-4",
      "py-4.5",
      "transition-transform",
      "duration-[320ms]",
      "ease-[cubic-bezier(.22,.8,.24,1)]",
      "motion-reduce:transition-none",
    );
    expect(drawer()).not.toHaveClass("border-r", "flex-[1_1_0]", "shadow-float");
    isClosed();
    expect(handle()).toHaveClass(
      "absolute",
      "top-[42%]",
      "left-0",
      "h-21",
      "w-7.5",
      "min-h-0",
      "min-w-0",
      "rounded-r-[12px]",
      "border",
      "border-l-0",
      "bg-card",
      "shadow-float",
    );
    expect(handle()!.querySelectorAll("svg")).toHaveLength(2);
    expect(sheet().parentElement).toHaveClass("px-4", "py-4.5");
    expect(footer()).toBeInTheDocument();
  });

  it("opens from the handle over the question, focuses its close button, and closes back to the handle", async () => {
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));

    await user.click(handle()!);
    expect(drawer()).not.toHaveAttribute("inert");
    expect(drawer()).toHaveAttribute("data-drawer-open");
    expect(drawer()).toHaveClass("translate-x-0", "shadow-float");
    expect(backdrop()).toHaveClass("bg-overlay", "absolute", "inset-0");
    expect(backdrop()).toHaveStyle({ opacity: "1" });
    expect(handle()).toBeNull();
    expect(
      within(passage()).getByText("Vuốt sang trái để quay lại câu hỏi"),
    ).toBeVisible();
    expect(closer()).toHaveClass("size-8.5", "min-h-0", "min-w-0");
    await waitFor(() => expect(closer()).toHaveFocus());
    expect(questionPane()).toBeVisible();
    expect(footer()).toBeInTheDocument();

    await user.click(closer());
    isClosed();
    await waitFor(() => expect(handle()).toHaveFocus());
  });

  it("closes on Esc and on the backdrop", async () => {
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    await user.click(handle()!);
    await waitFor(() => expect(closer()).toHaveFocus());
    await user.keyboard("{Escape}");
    isClosed();
    await waitFor(() => expect(handle()).toHaveFocus());

    await user.click(handle()!);
    await user.click(backdrop()!);
    isClosed();
  });

  it("is the deck's handle, tip and bar in English", async () => {
    await i18n.changeLanguage("en");
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    expect(
      screen.getByText("The passage is here. Swipe right or tap to read it."),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Open the passage" }));
    expect(screen.getByText("Swipe left to go back to the question")).toBeVisible();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });

  it("nudges and tips until the group's drawer has been opened once, and remembers it for the attempt", async () => {
    const user = userEvent.setup();
    const { unmount } = await open(deckPassageSession(NOW));
    const tip = () =>
      screen.queryByText("Ngữ liệu ở đây. Vuốt sang phải hoặc chạm để đọc.");
    expect(tip()).toHaveAttribute("role", "status");
    expect(tip()).toHaveClass(
      "bg-primary",
      "text-primary-fg",
      "text-meta",
      "max-w-55",
      "animate-[qz-tip_0.3s_ease-out_both]",
      "motion-reduce:animate-none",
    );
    expect(handle()).toHaveClass(
      "animate-[qz-nudge_1.4s_ease-in-out_0.4s_3]",
      "motion-reduce:animate-none",
    );

    await user.click(handle()!);
    await user.click(closer());
    expect(tip()).toBeNull();
    expect(handle()).not.toHaveClass("animate-[qz-nudge_1.4s_ease-in-out_0.4s_3]");
    expect(
      JSON.parse(sessionStorage.getItem(`quizzivy.stimulus-seen.${DECK_ATTEMPT}`)!),
    ).toEqual([deckPassageSession(NOW).groups![0]!.id]);

    unmount();
    store().reset();
    await open(deckPassageSession(NOW));
    expect(tip()).toBeNull();
  });

  it("costs nothing in a browser that refuses storage", async () => {
    const user = userEvent.setup();
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("denied", "SecurityError");
    });
    await open(deckPassageSession(NOW));
    await user.click(handle()!);
    await user.click(closer());
    expect(
      screen.queryByText("Ngữ liệu ở đây. Vuốt sang phải hoặc chạm để đọc."),
    ).toBeNull();
  });

  it("opens past a quarter of its width on a swipe right, and stays shut short of it", async () => {
    await open(deckPassageSession(NOW));
    swipe([4, 300], [4 + Math.round(W * 0.2), 302]);
    isClosed();

    swipe([4, 300], [4 + Math.round(W * 0.3), 302]);
    expect(drawer()).toHaveAttribute("data-drawer-open");
  });

  it("follows the finger while it drags, with the backdrop darkening as it comes out", async () => {
    await open(deckPassageSession(NOW));
    fireEvent.pointerDown(panes(), { clientX: 4, clientY: 300, pointerType: "touch" });
    fireEvent.pointerMove(panes(), { clientX: 24, clientY: 302, pointerType: "touch" });
    fireEvent.pointerMove(panes(), {
      clientX: 4 + W / 2,
      clientY: 302,
      pointerType: "touch",
    });
    expect(drawer()).toHaveStyle({ transform: `translateX(${-W / 2}px)` });
    expect(drawer()).not.toHaveClass("transition-transform");
    expect(backdrop()).toHaveStyle({ opacity: "0.5" });
    fireEvent.pointerUp(panes(), {
      clientX: 4 + W / 2,
      clientY: 302,
      pointerType: "touch",
    });
    expect(drawer()).toHaveAttribute("data-drawer-open");
    expect(drawer()).toHaveClass("transition-transform");
  });

  it("closes when dragged back past 30% of its width, and stays open short of it", async () => {
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    await user.click(handle()!);

    swipe([300, 300], [300 - Math.round(W * 0.25), 302]);
    expect(drawer()).toHaveAttribute("data-drawer-open");

    swipe([300, 300], [300 - Math.round(W * 0.35), 302]);
    isClosed();
  });

  it("leaves a vertical scroll to the page and never swallows the tap after it", async () => {
    const user = userEvent.setup();
    const paper = deckPassageSession(NOW);
    await open(paper);
    swipe([100, 400], [112, 300]);
    isClosed();
    const option = paper.questions[0]!.options![2]!;
    await user.click(screen.getByRole("radio", { name: option.text }));
    expect(store().answers[paper.questions[0]!.id]).toEqual({
      type: "choice",
      optionIds: [option.id],
    });
  });

  it("swallows a click that lands within 60ms of a swipe, and not one after", async () => {
    const paper = deckPassageSession(NOW);
    const first = paper.questions[0]!;
    const before = paper.answers[first.id];
    await open(paper);
    let clock = 1_000_000;
    vi.spyOn(Date, "now").mockImplementation(() => clock);
    const option = () => screen.getByRole("radio", { name: first.options![2]!.text });
    swipe([4, 300], [4 + Math.round(W * 0.3), 302]);
    clock += 59;
    fireEvent.click(option());
    expect(store().answers[first.id]).toEqual(before);
    clock += 1;
    fireEvent.click(option());
    expect(store().answers[first.id]).toEqual({
      type: "choice",
      optionIds: [first.options![2]!.id],
    });
  });

  it("takes no answer and no flag from the keys while the drawer is open", async () => {
    const user = userEvent.setup();
    const paper = deckPassageSession(NOW);
    const fourth = paper.questions[3]!;
    await open(paper);
    for (let moves = 0; moves < 3; moves++) await user.click(next());
    await user.click(handle()!);

    await user.keyboard("a");
    await user.keyboard("f");
    expect(store().answers[fourth.id]).toBeUndefined();
    expect(store().flags.has(fourth.id)).toBe(false);

    await user.keyboard("{Escape}");
    await user.keyboard("a");
    await user.keyboard("f");
    expect(store().answers[fourth.id]).toEqual({
      type: "choice",
      optionIds: [fourth.options![0]!.id],
    });
    expect(store().flags.has(fourth.id)).toBe(true);
  });

  it("closes when the arrow keys move to the next question, which they still do", async () => {
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    await user.click(handle()!);

    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("main", { name: "Câu 2" })).toBeInTheDocument();
    isClosed();
  });

  it("stays closed on coming back to a question whose drawer was open", async () => {
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    await user.click(handle()!);

    await user.keyboard("{ArrowRight}");
    await user.keyboard("{ArrowLeft}");
    expect(screen.getByRole("main", { name: "Câu 1" })).toBeInTheDocument();
    isClosed();
    expect(footer()).toBeInTheDocument();
  });

  it("stays closed on coming back by touch after the window was wide", async () => {
    const view = viewport("phone");
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    await user.click(handle()!);

    act(() => view.resize("desktop"));
    await user.click(next());
    act(() => view.resize("phone"));
    await user.click(screen.getByRole("button", { name: "Câu trước" }));
    expect(screen.getByRole("main", { name: "Câu 1" })).toBeInTheDocument();
    isClosed();
  });

  it("closes when the Submit dialog goes to a question, and lands on it", async () => {
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    for (let moves = 0; moves < 3; moves++) await user.click(next());
    await user.click(handle()!);

    await user.click(screen.getByRole("button", { name: "Nộp bài" }));
    await user.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "Câu 4" }),
    );
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    isClosed();
    expect(screen.getByRole("main", { name: "Câu 4" })).toBeInTheDocument();
    expect(sheet()).toHaveFocus();
  });

  it("keeps the student's place in the passage and in the question while it opens and closes", async () => {
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    const pane = sheet().parentElement!;
    pane.scrollTop = 160;
    fireEvent.scroll(pane);
    drawer().scrollTop = 240;
    fireEvent.scroll(drawer());

    await user.click(handle()!);
    expect(drawer().scrollTop).toBe(240);
    await user.click(closer());
    expect(pane.scrollTop).toBe(160);
    expect(drawer().scrollTop).toBe(240);
  });

  it("has no handle when the question has no passage", async () => {
    await open(deckSession(NOW));
    expect(handle()).toBeNull();
    expect(backdrop()).toBeNull();
    expect(questionPane()).toBeVisible();
  });

  it("has no handle for a group that has only recordings", async () => {
    await open(listening(deckSession(NOW)));
    expect(handle()).toBeNull();
    expect(within(sheet()).getByRole("button", { name: "Phát" })).toBeVisible();
  });

  it("shows both panes when an open drawer grows past 768, with the same pane element", async () => {
    const view = viewport("phone");
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    await user.click(handle()!);
    const element = drawer();

    act(() => view.resize("desktop"));
    expect(passage()).toBe(element);
    expect(passage()).toBeVisible();
    expect(passage()).not.toHaveAttribute("inert");
    expect(passage()).toHaveClass("border-r", "flex-[1_1_0]");
    expect(passage()).not.toHaveClass("absolute");
    expect(questionPane()).toBeVisible();
    expect(backdrop()).toBeNull();
    expect(handle()).toBeNull();

    act(() => view.resize("phone"));
    expect(drawer()).toBe(element);
  });

  it("keeps an answer being typed when the window crosses 768 on a paper with a passage", async () => {
    const view = viewport("desktop");
    const user = userEvent.setup();
    await open(deckPassageSession(NOW));
    for (let moves = 0; moves < 4; moves++) await user.click(next());
    await user.type(screen.getByRole("textbox"), "green space");

    act(() => view.resize("phone"));
    expect(handle()).not.toBeNull();
    expect(screen.getByRole("textbox")).toHaveFocus();
    expect(screen.getByRole("textbox")).toHaveValue("green space");

    act(() => view.resize("desktop"));
    expect(handle()).toBeNull();
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
