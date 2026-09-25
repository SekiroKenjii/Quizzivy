import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
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
  useTakeTestStore.getState().reset();
  localStorage.clear();
  sessionStorage.clear();
  play.mockClear();
  vi.spyOn(HTMLMediaElement.prototype, "play").mockImplementation(play);
  vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => undefined);
  const now = new Date().toISOString();
  vi.mocked(saveAnswers).mockResolvedValue({ serverTime: now, savedAt: now });
  vi.mocked(recordGroupAudioPlay).mockImplementation(async (_id, input) => ({
    playId: input.playId,
    plays: 1,
    maxPlays: 2,
  }));
  vi.mocked(getAttempt).mockResolvedValue({
    ...session({
      serverTime: now,
      deadlineAt: new Date(Date.now() + 3600000).toISOString(),
    }),
    sections: [previewSection],
    questions: previewQuestions.slice(1),
    groups: [previewGroup],
    groupAudioPlays: {},
  });
});
afterEach(() => {
  useTakeTestStore.getState().reset();
  vi.restoreAllMocks();
});

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

it("collapses phone material while keeping listening controls available and remembers the choice", async () => {
  viewport("phone");
  const user = userEvent.setup();
  const { container } = mount();
  await screen.findByText(previewQuestions[1]!.prompt);
  const audio = container.querySelector("audio")!;
  await user.click(screen.getByRole("button", { name: "Thu gọn ngữ liệu" }));
  expect(screen.getByText("Lịch hoạt động")).not.toBeVisible();
  expect(screen.getByRole("button", { name: "Phát" })).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Câu sau" }));
  expect(screen.getByRole("button", { name: "Mở ngữ liệu" })).toHaveAttribute(
    "aria-expanded",
    "false",
  );
  expect(container.querySelector("audio")).toBe(audio);
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
