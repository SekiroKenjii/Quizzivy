import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import TakeTestPage from "@/features/take-test/pages/TakeTestPage";
import {
  getAttempt,
  recordAudioPlay,
  recordGroupAudioPlay,
  saveAnswers,
  type AttemptSession,
  type StudentQuestion,
} from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { forgetAllPlaces } from "@/features/media/playback";
import i18n from "@/lib/i18n";
import {
  DECK_ATTEMPT,
  deckListeningSession,
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

/**
 * What a remembered place does across the engine: two audio questions in a
 * row, a recording shared by a group's questions, and a later attempt in the
 * same tab. jsdom plays nothing: `currentTime` and the media events stand in
 * for playback, and `play`, `pause` and `paused` are stubbed per element.
 */
const NOW = new Date("2026-09-22T05:00:00.000Z");
const NEXT_ATTEMPT = "018f0000-0000-7000-8000-0000000000e2";
const AUDIO_1 = "018f0000-0000-7000-8000-00000000d001";
const AUDIO_2 = "018f0000-0000-7000-8000-00000000d002";
const CARD_NAMES = new Set(["Phát", "Tạm dừng", "Nghe tiếp", "Phát lại"]);
const sounding = new Set<HTMLMediaElement>();
const store = () => useTakeTestStore.getState();

function audioQuestion(id: string, n: number, base: StudentQuestion): StudentQuestion {
  return {
    ...base,
    id,
    prompt: `Listen ${n}`,
    media: {
      id: `m${n}`,
      kind: "audio",
      url: `https://assets.example/q${n}.mp3`,
      mimeType: "audio/mpeg",
      bytes: 2048,
      durationMs: 60_000,
      originalFilename: `q${n}.mp3`,
      createdAt: "2026-09-01T00:00:00.000Z",
    },
    audio: {
      maxPlays: 2,
      allowSeek: false,
      showTranscriptAfterSubmit: false,
      allowPause: true,
    },
  };
}

function twoAudioQuestions(attemptId = DECK_ATTEMPT): AttemptSession {
  const base = deckSession(NOW);
  const [first, second, third] = base.questions;
  return {
    ...base,
    attempt: { ...base.attempt, id: attemptId },
    questions: [
      third!,
      audioQuestion(AUDIO_1, 1, first!),
      audioQuestion(AUDIO_2, 2, second!),
    ],
    audioPlays: {},
  };
}

async function open(paper: AttemptSession) {
  vi.mocked(getAttempt).mockResolvedValue(paper);
  const router = createMemoryRouter(
    [{ path: "/app/attempts/:attemptId", element: <TakeTestPage /> }],
    { initialEntries: [`/app/attempts/${paper.attempt.id}`] },
  );
  const view = render(<RouterProvider router={router} />);
  await screen.findByRole("main");
  return view;
}

const audio = () => document.querySelector("audio") as HTMLAudioElement;
const next = () => fireEvent.click(screen.getByRole("button", { name: "Câu sau" }));
const previous = () =>
  fireEvent.click(screen.getByRole("button", { name: "Câu trước" }));
const cardButton = () =>
  screen
    .getAllByRole("button")
    .find((item) => CARD_NAMES.has(item.getAttribute("aria-label") ?? ""))!;
const questionPlays = () =>
  vi
    .mocked(recordAudioPlay)
    .mock.calls.map(([attemptId, questionId]) => [attemptId, questionId]);

function startAndPlayTo(seconds: number) {
  fireEvent.click(cardButton());
  fireEvent.play(audio());
  audio().currentTime = seconds;
  fireEvent.timeUpdate(audio());
}

beforeEach(async () => {
  viewport("desktop");
  localStorage.clear();
  sessionStorage.clear();
  store().reset();
  sounding.clear();
  Object.defineProperty(HTMLMediaElement.prototype, "play", {
    configurable: true,
    value: vi.fn(function (this: HTMLMediaElement) {
      sounding.add(this);
      return Promise.resolve();
    }),
  });
  Object.defineProperty(HTMLMediaElement.prototype, "pause", {
    configurable: true,
    value: vi.fn(function (this: HTMLMediaElement) {
      sounding.delete(this);
    }),
  });
  Object.defineProperty(HTMLMediaElement.prototype, "paused", {
    configurable: true,
    get(this: HTMLMediaElement) {
      return !sounding.has(this);
    },
  });
  vi.mocked(saveAnswers)
    .mockReset()
    .mockImplementation(async () =>
      deckSaved(new Date(), new Date(store().deadlineAt).toISOString()),
    );
  vi.mocked(recordAudioPlay).mockReset().mockResolvedValue({ plays: 1, maxPlays: 2 });
  vi.mocked(recordGroupAudioPlay)
    .mockReset()
    .mockResolvedValue({ plays: 1 } as never);
  await i18n.changeLanguage("vi");
});

afterEach(() => {
  forgetAllPlaces();
  store().reset();
  vi.restoreAllMocks();
});

describe("two audio questions in a row", () => {
  it("puts the first question's remembered place on neither the second's player nor its count", async () => {
    await open(twoAudioQuestions());
    next();
    startAndPlayTo(20);
    sounding.clear();
    fireEvent.pause(audio());
    previous();
    expect(audio()).toBeNull();

    next();
    fireEvent.loadedMetadata(audio());
    expect(audio().currentTime).toBe(20);
    expect(cardButton()).toHaveAccessibleName("Nghe tiếp");

    next();
    fireEvent.loadedMetadata(audio());
    expect(audio().currentTime).toBe(0);
    expect(cardButton()).toHaveAccessibleName("Phát");
    fireEvent.click(cardButton());

    expect(questionPlays()).toEqual([
      [DECK_ATTEMPT, AUDIO_1],
      [DECK_ATTEMPT, AUDIO_2],
    ]);
  });

  it("gives the second question an idle card while the first is playing", async () => {
    await open(twoAudioQuestions());
    next();
    startAndPlayTo(20);
    const first = audio();

    next();

    expect(sounding.has(first)).toBe(false);
    expect(audio()).not.toBe(first);
    const card = screen.getByRole("group", { name: "Đoạn ghi âm của câu hỏi" });
    expect(card).toHaveAttribute("data-phase", "ready");
    expect(cardButton()).toHaveAccessibleName("Phát");
  });

  it("resumes the first question where it stopped on coming back from the second", async () => {
    await open(twoAudioQuestions());
    next();
    startAndPlayTo(20);
    next();
    previous();

    fireEvent.loadedMetadata(audio());
    expect(audio().currentTime).toBe(20);
    expect(cardButton()).toHaveAccessibleName("Nghe tiếp");
    fireEvent.click(cardButton());
    expect(questionPlays()).toEqual([[DECK_ATTEMPT, AUDIO_1]]);
  });
});

describe("leaving the engine", () => {
  it("forgets a play that was in progress, so a later attempt in the tab starts from the top", async () => {
    const view = await open(twoAudioQuestions());
    next();
    startAndPlayTo(20);
    view.unmount();
    store().reset();

    await open(twoAudioQuestions(NEXT_ATTEMPT));
    next();
    fireEvent.loadedMetadata(audio());

    expect(audio().currentTime).toBe(0);
    expect(cardButton()).toHaveAccessibleName("Phát");
    fireEvent.click(cardButton());
    expect(questionPlays()).toEqual([
      [DECK_ATTEMPT, AUDIO_1],
      [NEXT_ATTEMPT, AUDIO_1],
    ]);
  });

  it("forgets it for the same attempt too, whose next start spends a play", async () => {
    const view = await open(twoAudioQuestions());
    next();
    startAndPlayTo(20);
    view.unmount();
    store().reset({ keepDraft: true });

    await open(twoAudioQuestions());
    expect(screen.getByText("Listen 1")).toBeInTheDocument();
    fireEvent.loadedMetadata(audio());

    expect(audio().currentTime).toBe(0);
    expect(cardButton()).toHaveAccessibleName("Phát");
    fireEvent.click(cardButton());
    expect(questionPlays()).toEqual([
      [DECK_ATTEMPT, AUDIO_1],
      [DECK_ATTEMPT, AUDIO_1],
    ]);
  });
});

describe("a recording its group's questions share", () => {
  it.each([
    ["from 768", "desktop"],
    ["below 768, in the drawer", "phone"],
  ] as const)(
    "keeps playing %s while the student moves between the group's questions",
    async (_, width) => {
      viewport(width);
      await open(deckListeningSession(NOW));
      if (width === "phone")
        fireEvent.click(screen.getByRole("button", { name: "Mở bài nghe" }));
      const card = screen.getByRole("group", { name: "Asking the way" });
      fireEvent.click(within(card).getByRole("button", { name: "Phát" }));
      fireEvent.play(audio());
      audio().currentTime = 12;
      fireEvent.timeUpdate(audio());
      const element = audio();

      next();
      next();

      expect(audio()).toBe(element);
      expect(sounding.has(element)).toBe(true);
      expect(
        within(
          screen.getByRole("group", { name: "Asking the way", hidden: true }),
        ).getByRole("button", { name: "Tạm dừng", hidden: true }),
      ).toBeInTheDocument();
      expect(vi.mocked(recordGroupAudioPlay)).toHaveBeenCalledTimes(1);
    },
  );
});
