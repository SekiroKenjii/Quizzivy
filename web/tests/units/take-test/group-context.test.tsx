import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import {
  getAttempt,
  recordGroupAudioPlay,
  saveAnswers,
} from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import {
  previewGroup,
  previewQuestions,
  previewSection,
} from "@tests/support/groupPreview";
import { deckListeningSession } from "./deckSession";
import { session, viewport } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  recordGroupAudioPlay: vi.fn(),
}));
const play = vi.fn().mockResolvedValue(undefined);
beforeEach(() => {
  viewport("desktop");
  useTakeTestStore.getState().reset();
  localStorage.clear();
  sessionStorage.clear();
  play.mockClear();
  vi.spyOn(HTMLMediaElement.prototype, "play").mockImplementation(play);
  vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => undefined);
  const now = new Date().toISOString();
  const deadline = new Date(Date.now() + 3600000).toISOString();
  vi.mocked(saveAnswers).mockResolvedValue({
    serverTime: now,
    savedAt: now,
    deadlineAt: deadline,
  });
  vi.mocked(recordGroupAudioPlay).mockImplementation(async (_id, input) => ({
    playId: input.playId,
    plays: 1,
    maxPlays: 2,
  }));
  vi.mocked(getAttempt).mockResolvedValue({
    ...session({
      serverTime: now,
      deadlineAt: deadline,
    }),
    sections: [previewSection],
    questions: previewQuestions.slice(1),
    groups: [previewGroup],
    groupAudioPlays: {},
  });
});
afterEach(() => {
  useTakeTestStore.getState().reset();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const passage = () => screen.getByRole("article", { name: previewGroup.title });
const drawer = () => screen.getByRole("article", { hidden: true });
const openDrawer = () => screen.getByRole("button", { name: "Mở ngữ liệu" });

function mount() {
  const router = createMemoryRouter(
    [{ path: "/app/attempts/:attemptId", element: <TakeTestPage /> }],
    { initialEntries: ["/app/attempts/att-1"] },
  );
  return render(<RouterProvider router={router} />);
}

it("keeps one player and its position in the stimulus pane while moving between shared questions", async () => {
  const user = userEvent.setup();
  const { container } = mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  const audio = container.querySelector("audio")!;
  expect(passage()).toContainElement(audio);
  fireEvent.click(screen.getByRole("button", { name: "Phát" }));
  expect(play).toHaveBeenCalledOnce();
  expect(recordGroupAudioPlay).toHaveBeenCalled();
  audio.currentTime = 0.5;
  fireEvent.timeUpdate(audio);
  await user.click(screen.getByRole("button", { name: "Câu sau" }));
  expect(screen.getByText(previewQuestions[2]!.prompt)).toBeVisible();
  expect(container.querySelector("audio")).toBe(audio);
  expect(audio.currentTime).toBe(0.5);
  expect(within(passage()).getByText("Lượt nghe 1/2")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Câu trước" }));
  expect(container.querySelector("audio")).toBe(audio);
});

it("keeps the recording in the drawer on a phone, mounted while the drawer is closed", async () => {
  viewport("phone");
  const user = userEvent.setup();
  const { container } = mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  const audio = container.querySelector("audio")!;

  expect(drawer()).toHaveAttribute("inert");
  expect(drawer()).toContainElement(audio);
  expect(
    within(drawer()).getByRole("button", { name: "Phát", hidden: true }),
  ).toBeInTheDocument();

  await user.click(openDrawer());
  expect(passage()).not.toHaveAttribute("inert");
  expect(within(passage()).getByText("Lịch hoạt động")).toBeInTheDocument();
  expect(within(passage()).getByRole("button", { name: "Phát" })).toBeVisible();
  expect(screen.getByRole("navigation", { name: "Danh sách câu" })).toBeInTheDocument();
  expect(container.querySelector("audio")).toBe(audio);

  await user.click(screen.getByRole("button", { name: "Đóng" }));
  expect(drawer()).toHaveAttribute("inert");
  await user.click(screen.getByRole("button", { name: "Câu sau" }));
  expect(screen.getByRole("main", { name: "Câu 2" })).toBeInTheDocument();
  expect(drawer()).toHaveAttribute("inert");
  expect(container.querySelector("audio")).toBe(audio);
});

it("marks the drawer's handle with a dot while the group's recording plays", async () => {
  viewport("phone");
  const { container } = mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  const audio = container.querySelector("audio")!;
  const dot = () => openDrawer().querySelector(".bg-brand");
  expect(dot()).toBeNull();

  let paused = false;
  vi.spyOn(audio, "paused", "get").mockImplementation(() => paused);
  fireEvent.play(audio);
  expect(dot()).toHaveClass("size-[7px]", "rounded-full");

  paused = true;
  fireEvent.pause(audio);
  expect(dot()).toBeNull();
});

it("closes the drawer and returns to the question, and to the gap's target, from a gap on a phone", async () => {
  viewport("phone");
  const user = userEvent.setup();
  mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  await user.click(openDrawer());
  expect(screen.getByRole("button", { name: "Chỗ trống 2, trống" })).toHaveAttribute(
    "data-state",
    "empty",
  );
  await user.click(screen.getByRole("button", { name: "Chỗ trống 2, trống" }));

  expect(screen.getByText(previewQuestions[2]!.prompt)).toBeVisible();
  expect(drawer()).toHaveAttribute("inert");
  expect(document.activeElement).toHaveAttribute(
    "id",
    `answer-question-${previewQuestions[2]!.id}`,
  );
  expect(screen.getByRole("navigation", { name: "Danh sách câu" })).toBeInTheDocument();
});

it("closes the drawer when a gap in it points at the question already open", async () => {
  viewport("phone");
  const user = userEvent.setup();
  mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  await user.click(screen.getByRole("button", { name: "Câu sau" }));
  await user.click(openDrawer());
  expect(drawer()).not.toHaveAttribute("inert");

  await user.click(screen.getByRole("button", { name: "Chỗ trống 2, trống" }));
  expect(drawer()).toHaveAttribute("inert");
  expect(document.activeElement).toHaveAttribute(
    "id",
    `answer-question-${previewQuestions[2]!.id}`,
  );
});

it("keeps the student's place in the passage while the drawer is closed on a phone", async () => {
  viewport("phone");
  const user = userEvent.setup();
  mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  await user.click(openDrawer());
  passage().scrollTop = 240;
  fireEvent.scroll(passage());

  await user.click(screen.getByRole("button", { name: "Đóng" }));
  await user.click(openDrawer());
  expect(passage().scrollTop).toBe(240);
});

it("starts each question at the top of its pane and keeps the passage where it was", async () => {
  const user = userEvent.setup();
  mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  const sheet = screen
    .getByText(previewQuestions[1]!.prompt)
    .closest(".overflow-y-auto")!;
  sheet.scrollTop = 180;
  passage().scrollTop = 320;

  await user.click(screen.getByRole("button", { name: "Câu sau" }));
  expect(sheet.scrollTop).toBe(0);
  expect(passage().scrollTop).toBe(320);
  expect(document.activeElement).toHaveAttribute(
    "id",
    `answer-question-${previewQuestions[2]!.id}`,
  );
});

it("opens a gap's question at the top of its pane, not where the last question was left", async () => {
  viewport("phone");
  const user = userEvent.setup();
  mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  const sheet = screen
    .getByText(previewQuestions[1]!.prompt)
    .closest(".overflow-y-auto")!;
  sheet.scrollTop = 180;
  fireEvent.scroll(sheet);
  await user.click(openDrawer());
  await user.click(screen.getByRole("button", { name: "Chỗ trống 2, trống" }));
  expect(sheet.scrollTop).toBe(0);
});

it("opens a gap's question at the top of its pane beside the passage too", async () => {
  const user = userEvent.setup();
  mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  const sheet = screen
    .getByText(previewQuestions[1]!.prompt)
    .closest(".overflow-y-auto")!;
  sheet.scrollTop = 180;
  fireEvent.scroll(sheet);
  await user.click(screen.getByRole("button", { name: "Chỗ trống 2, trống" }));
  expect(sheet.scrollTop).toBe(0);
});

it("jumps from a material gap to its question and preserves the prior answer", async () => {
  const user = userEvent.setup();
  mount();
  await user.click(
    await screen.findByRole("radio", { name: previewQuestions[1]!.options![0]!.text }),
  );
  await user.click(screen.getByRole("button", { name: "Chỗ trống 2, trống" }));
  expect(screen.getByText(previewQuestions[2]!.prompt)).toBeVisible();
  expect(document.activeElement).toHaveAttribute(
    "id",
    `answer-question-${previewQuestions[2]!.id}`,
  );
  await user.click(screen.getByRole("button", { name: "Câu trước" }));
  expect(
    screen.getByRole("radio", { name: previewQuestions[1]!.options![0]!.text }),
  ).toBeChecked();
});

it("pauses and disables group playback after the attempt is locked", async () => {
  mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  act(() => useTakeTestStore.getState().lockNow("superseded"));
  expect(screen.getByRole("button", { name: "Phát" })).toBeDisabled();
  expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
});

it("focuses a material's stable blank target even when its printed number differs", async () => {
  const current = await vi.mocked(getAttempt)("att-1");
  const blankId = "01935000-0000-7000-8000-000000000077";
  const questionId = previewQuestions[2]!.id;
  vi.mocked(getAttempt).mockResolvedValue({
    ...current,
    questions: [
      previewQuestions[1]!,
      {
        id: questionId,
        sectionId: previewSection.id,
        type: "fill_blank",
        prompt: "[99]",
        points: 1,
        promptContent: {
          format: "semantic_v1",
          blocks: [
            {
              type: "paragraph",
              content: [{ type: "gap", id: "target", label: "99" }],
            },
          ],
        },
        blanks: [{ id: blankId, ordinal: 1, gapId: "target", caseSensitive: false }],
      },
    ],
    groups: [
      {
        ...previewGroup,
        stimuli: [
          {
            ...previewGroup.stimuli[0]!,
            gaps: [
              { kind: "blank", gapId: "material", questionId, blankGapId: "target" },
            ],
          },
        ],
      },
    ],
  });
  mount();
  await userEvent
    .setup()
    .click(
      await screen.findByRole("button", { name: "Chỗ trống A — chuyển đến câu 2" }),
    );
  expect(screen.getByRole("textbox", { name: "Chỗ trống 1" })).toHaveFocus();
});

it("gives a material that is only a recording no heading: the group's title is the kicker", async () => {
  const paper = deckListeningSession(new Date());
  vi.mocked(getAttempt).mockResolvedValue(paper);
  mount();
  const pane = await screen.findByRole("article", { name: "Part 1 · Asking the way" });

  expect(within(pane).getByText("Part 1 · Asking the way")).toHaveClass(
    "uppercase",
    "tracking-[0.04em]",
  );
  expect(within(pane).queryAllByRole("heading")).toHaveLength(0);
  expect(within(pane).queryByRole("region")).toBeNull();
  expect(screen.queryByText(paper.groups![0]!.assets[0]!.originalFilename)).toBeNull();
});

it("keeps the heading of a material with text and a recording, under the group's title as the kicker", async () => {
  mount();
  const pane = await screen.findByRole("article", { name: previewGroup.title });

  expect(within(pane).getByText(previewGroup.title)).toHaveClass("uppercase");
  const heading = within(pane).getByRole("heading", { level: 2 });
  expect(heading).toHaveTextContent(previewGroup.stimuli[0]!.title);
  expect(heading).toHaveClass("text-stat", "font-semibold");
  expect(within(pane).getAllByRole("heading", { level: 2 })).toHaveLength(1);
  expect(within(pane).queryAllByRole("heading", { level: 4 })).toHaveLength(0);
  expect(within(pane).getByText("Giữ nguyên thứ tự câu trong nhóm.")).toBeVisible();
  expect(
    heading.compareDocumentPosition(
      within(pane).getByText("Giữ nguyên thứ tự câu trong nhóm."),
    ) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
});

it("draws a listening group's card at its audio node, named by the node and with nothing printed", async () => {
  const paper = deckListeningSession(new Date());
  vi.mocked(getAttempt).mockResolvedValue(paper);
  const { container } = mount();
  const pane = await screen.findByRole("article", { name: "Part 1 · Asking the way" });

  const card = within(pane).getByRole("group", { name: "Asking the way" });
  expect(within(card).getByRole("button", { name: "Phát" })).toBeVisible();
  expect(within(card).getByText("Lượt nghe 1/2")).toBeVisible();
  expect(within(card).getByText("Bấm phát khi bạn đã sẵn sàng.")).toBeVisible();
  expect(within(pane).queryByText("Asking the way")).toBeNull();
  expect(container.querySelectorAll("audio")).toHaveLength(1);
  const sheet = document.querySelector<HTMLElement>('[id^="answer-question-"]')!;
  expect(within(sheet).queryByRole("button", { name: "Phát" })).toBeNull();
});

it("draws the card where the audio node stands in a material with text, after the table", async () => {
  mount();
  const pane = await screen.findByRole("article", { name: previewGroup.title });

  const card = within(pane).getByRole("group", { name: "Hội thoại mẫu" });
  const table = within(pane).getByRole("table");
  expect(
    table.compareDocumentPosition(card) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  expect(within(pane).queryByText("Hội thoại mẫu")).toBeNull();
  expect(within(pane).getByText(previewGroup.title)).toHaveClass("uppercase");
});

it("draws two recordings at their own nodes, each under its label, or its number for a blank label", async () => {
  const first = "01935000-0000-7000-8000-0000000000a1";
  const second = "01935000-0000-7000-8000-0000000000a2";
  const asset = previewGroup.assets[0]!;
  const recording = previewGroup.recordings[0]!;
  const paragraph = (text: string) => ({
    type: "paragraph" as const,
    content: [{ type: "text" as const, text, marks: [] }],
  });
  const current = await vi.mocked(getAttempt)("att-1");
  vi.mocked(getAttempt).mockResolvedValue({
    ...current,
    groups: [
      {
        ...previewGroup,
        stimuli: [
          {
            ...previewGroup.stimuli[0]!,
            content: {
              format: "semantic_v1",
              blocks: [
                paragraph("Đoạn mở đầu."),
                { type: "audio", assetId: first, label: "Đoạn hội thoại thứ nhất" },
                paragraph("Đoạn giữa."),
                { type: "audio", assetId: second, label: " " },
              ],
            },
            gaps: [],
          },
        ],
        recordings: [
          { ...recording, id: "01935000-0000-7000-8000-0000000000b1", assetId: first },
          { ...recording, id: "01935000-0000-7000-8000-0000000000b2", assetId: second },
        ],
        assets: [
          { ...asset, id: first, url: "https://assets.example/first.mp3" },
          { ...asset, id: second, url: "https://assets.example/second.mp3" },
        ],
      },
    ],
  });
  const { container } = mount();
  const pane = await screen.findByRole("article", { name: previewGroup.title });

  const one = within(pane).getByRole("group", { name: "Đoạn hội thoại thứ nhất" });
  const two = within(pane).getByRole("group", { name: "Bài nghe 2" });
  expect(within(pane).getByText("Đoạn hội thoại thứ nhất")).toBeVisible();
  expect(within(pane).getByText("Bài nghe 2")).toBeVisible();
  const middle = within(pane).getByText("Đoạn giữa.");
  expect(
    one.compareDocumentPosition(middle) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  expect(
    middle.compareDocumentPosition(two) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  expect(container.querySelectorAll("audio")).toHaveLength(2);
});

it("still draws a recording no audio node names, after the materials: a payload the server refuses", async () => {
  const current = await vi.mocked(getAttempt)("att-1");
  vi.mocked(getAttempt).mockResolvedValue({
    ...current,
    groups: [{ ...previewGroup, stimuli: [] }],
  });
  mount();
  const pane = await screen.findByRole("article", { name: previewGroup.title });

  expect(within(pane).queryAllByRole("heading")).toHaveLength(0);
  const card = within(pane).getByRole("group", { name: "Bài nghe 1" });
  expect(
    within(pane)
      .getByText("Giữ nguyên thứ tự câu trong nhóm.")
      .compareDocumentPosition(card) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).toBeTruthy();
  expect(within(card).getByRole("button", { name: "Phát" })).toBeVisible();
});

it("pauses a recording when the student leaves its group, and resumes it there for no play on coming back", async () => {
  const user = userEvent.setup();
  vi.mocked(recordGroupAudioPlay).mockClear();
  vi.mocked(getAttempt).mockResolvedValue(deckListeningSession(new Date()));
  const { container } = mount();
  await screen.findByRole("article", { name: "Part 1 · Asking the way" });
  const first = container.querySelector("audio")!;
  fireEvent.click(screen.getByRole("button", { name: "Phát" }));
  expect(recordGroupAudioPlay).toHaveBeenCalledTimes(1);
  first.currentTime = 42;
  fireEvent.timeUpdate(first);
  vi.mocked(HTMLMediaElement.prototype.pause).mockClear();

  for (let step = 0; step < 5; step += 1)
    await user.click(screen.getByRole("button", { name: "Câu sau" }));
  expect(
    await screen.findByRole("article", { name: "Part 2 · A guided walk" }),
  ).toBeInTheDocument();
  expect(HTMLMediaElement.prototype.pause).toHaveBeenCalled();
  expect(first).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "Câu trước" }));
  const pane = await screen.findByRole("article", { name: "Part 1 · Asking the way" });
  const again = container.querySelector("audio")!;
  expect(again).not.toBe(first);
  expect(within(pane).getByText("Đã tạm dừng")).toBeVisible();
  fireEvent.click(within(pane).getByRole("button", { name: "Nghe tiếp" }));
  expect(play).toHaveBeenCalledTimes(2);
  fireEvent.loadedMetadata(again);
  expect(again.currentTime).toBe(42);
  vi.spyOn(again, "paused", "get").mockReturnValue(false);
  fireEvent.timeUpdate(again);
  expect(recordGroupAudioPlay).toHaveBeenCalledTimes(1);
});

it("draws part 2's recording, which cannot be paused, with its sentence and an inert button while it plays", async () => {
  const user = userEvent.setup();
  vi.mocked(getAttempt).mockResolvedValue(deckListeningSession(new Date()));
  const { container } = mount();
  await screen.findByRole("article", { name: "Part 1 · Asking the way" });
  expect(screen.queryByText(/không tạm dừng được/)).not.toBeInTheDocument();

  for (let step = 0; step < 5; step += 1)
    await user.click(screen.getByRole("button", { name: "Câu sau" }));
  const pane = await screen.findByRole("article", { name: "Part 2 · A guided walk" });
  expect(
    within(pane).getByText(
      "Bấm phát khi bạn đã sẵn sàng. Bài nghe này không tạm dừng được sau khi bắt đầu.",
    ),
  ).toBeVisible();

  const audio = container.querySelector("audio")!;
  fireEvent.click(within(pane).getByRole("button", { name: "Phát" }));
  vi.spyOn(audio, "paused", "get").mockReturnValue(false);
  fireEvent.play(audio);
  const inert = within(pane).getByRole("button", { name: "Tạm dừng" });
  expect(inert).toHaveAttribute("aria-disabled", "true");
  vi.mocked(HTMLMediaElement.prototype.pause).mockClear();
  fireEvent.click(inert);
  expect(HTMLMediaElement.prototype.pause).not.toHaveBeenCalled();
  expect(
    within(pane).getByText("Đang phát · bài nghe này không tạm dừng được"),
  ).toBeVisible();
});
