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

const switcher = () =>
  within(screen.getByRole("group", { name: "Xem ngữ liệu hoặc câu hỏi" }));
const passage = () => screen.getByRole("article", { name: previewGroup.title });

function mount() {
  const router = createMemoryRouter(
    [{ path: "/app/attempts/:attemptId", element: <TakeTestPage /> }],
    { initialEntries: ["/app/attempts/att-1"] },
  );
  return render(<RouterProvider router={router} />);
}

it("keeps one player and its position while moving between shared questions", async () => {
  const user = userEvent.setup();
  const { container } = mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  const audio = container.querySelector("audio")!;
  fireEvent.click(screen.getByRole("button", { name: "Phát" }));
  expect(play).toHaveBeenCalledOnce();
  expect(recordGroupAudioPlay).toHaveBeenCalled();
  audio.currentTime = 0.5;
  fireEvent.timeUpdate(audio);
  await user.click(screen.getByRole("button", { name: "Câu sau" }));
  expect(screen.getByText(previewQuestions[2]!.prompt)).toBeVisible();
  expect(container.querySelector("audio")).toBe(audio);
  expect(audio.currentTime).toBe(0.5);
  expect(screen.getByText("Còn 1 lượt nghe")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Câu trước" }));
  expect(container.querySelector("audio")).toBe(audio);
});

it("keeps the listening controls with the question on a phone, where the passage is behind its tab", async () => {
  viewport("phone");
  const user = userEvent.setup();
  const { container } = mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  const audio = container.querySelector("audio")!;

  expect(switcher().getByRole("button", { name: "Câu 1" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(screen.getByText("Lịch hoạt động")).not.toBeVisible();
  expect(screen.getByRole("button", { name: "Phát" })).toBeVisible();

  await user.click(switcher().getByRole("button", { name: "Ngữ liệu" }));
  expect(screen.getByText("Lịch hoạt động")).toBeVisible();
  expect(screen.getByText(previewQuestions[1]!.prompt)).not.toBeVisible();
  expect(screen.queryByRole("contentinfo")).toBeNull();
  expect(container.querySelector("audio")).toBe(audio);

  await user.click(switcher().getByRole("button", { name: "Câu 1" }));
  expect(screen.getByText("Lịch hoạt động")).not.toBeVisible();
  expect(screen.getByRole("button", { name: "Phát" })).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Câu sau" }));
  expect(switcher().getByRole("button", { name: "Câu 2" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  expect(container.querySelector("audio")).toBe(audio);
});

it("returns to the question, and to the gap's target, from a gap in the passage on a phone", async () => {
  viewport("phone");
  const user = userEvent.setup();
  mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  await user.click(switcher().getByRole("button", { name: "Ngữ liệu" }));
  expect(screen.getByRole("button", { name: "Ô A — chuyển đến câu 2" })).toHaveClass(
    "content-gap",
    "items-end",
  );
  await user.click(screen.getByRole("button", { name: "Ô A — chuyển đến câu 2" }));

  expect(screen.getByText(previewQuestions[2]!.prompt)).toBeVisible();
  expect(screen.getByText("Lịch hoạt động")).not.toBeVisible();
  expect(document.activeElement).toHaveAttribute(
    "id",
    `answer-question-${previewQuestions[2]!.id}`,
  );
  expect(screen.getByRole("contentinfo")).toBeInTheDocument();
});

it("shows the question again when a gap in the passage points at the question already open", async () => {
  viewport("phone");
  const user = userEvent.setup();
  mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  await user.click(screen.getByRole("button", { name: "Câu sau" }));
  await user.click(switcher().getByRole("button", { name: "Ngữ liệu" }));
  expect(screen.getByText(previewQuestions[2]!.prompt)).not.toBeVisible();

  await user.click(screen.getByRole("button", { name: "Ô A — chuyển đến câu 2" }));
  expect(screen.getByText(previewQuestions[2]!.prompt)).toBeVisible();
  expect(document.activeElement).toHaveAttribute(
    "id",
    `answer-question-${previewQuestions[2]!.id}`,
  );
});

it("keeps the student's place in the passage while the question is showing on a phone", async () => {
  viewport("phone");
  const user = userEvent.setup();
  mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  await user.click(switcher().getByRole("button", { name: "Ngữ liệu" }));
  passage().scrollTop = 240;
  fireEvent.scroll(passage());

  await user.click(switcher().getByRole("button", { name: "Câu 1" }));
  screen.getByRole("article", { hidden: true }).scrollTop = 0;
  await user.click(switcher().getByRole("button", { name: "Ngữ liệu" }));
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
  await user.click(switcher().getByRole("button", { name: "Ngữ liệu" }));
  sheet.scrollTop = 0;
  await user.click(screen.getByRole("button", { name: "Ô A — chuyển đến câu 2" }));
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
  await user.click(screen.getByRole("button", { name: "Ô A — chuyển đến câu 2" }));
  expect(sheet.scrollTop).toBe(0);
});

it("jumps from a material gap to its question and preserves the prior answer", async () => {
  const user = userEvent.setup();
  mount();
  await user.click(
    await screen.findByRole("radio", { name: previewQuestions[1]!.options![0]!.text }),
  );
  await user.click(screen.getByRole("button", { name: "Ô A — chuyển đến câu 2" }));
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
    .click(await screen.findByRole("button", { name: "Ô A — chuyển đến câu 2" }));
  expect(screen.getByRole("textbox", { name: "Chỗ trống 1" })).toHaveFocus();
});
