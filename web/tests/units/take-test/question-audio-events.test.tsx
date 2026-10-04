import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { QuestionAudio } from "@/features/take-test/components/QuestionAudio";
import { recordAudioPlay, type StudentQuestion } from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { beginSession, clearSession, pending } from "@/features/integrity/buffer";
import { session } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  recordGroupAudioPlay: vi.fn(),
}));

const counted = vi.mocked(recordAudioPlay);

const question: StudentQuestion = {
  id: "q1",
  sectionId: "s1",
  type: "single_choice",
  prompt: "Listen and choose.",
  points: 1,
  options: [
    { id: "o1", text: "At the post office" },
    { id: "o2", text: "At the station" },
  ],
  media: {
    id: "m1",
    kind: "audio",
    url: "https://assets.example/q1.mp3",
    mimeType: "audio/mpeg",
    bytes: 2048,
    durationMs: 10_004,
    originalFilename: "q1.mp3",
    createdAt: "2026-09-01T00:00:00.000Z",
  },
  audio: { maxPlays: 2, allowSeek: false, showTranscriptAfterSubmit: false },
};

let play: ReturnType<typeof vi.fn>;
let paused = true;

beforeEach(() => {
  paused = true;
  play = vi.fn(() => {
    paused = false;
    return Promise.resolve();
  });
  Object.defineProperty(HTMLMediaElement.prototype, "play", {
    configurable: true,
    value: play,
  });
  Object.defineProperty(HTMLMediaElement.prototype, "pause", {
    configurable: true,
    value: vi.fn(() => {
      paused = true;
    }),
  });
  Object.defineProperty(HTMLMediaElement.prototype, "paused", {
    configurable: true,
    get: () => paused,
  });
  sessionStorage.clear();
  counted.mockReset().mockResolvedValue({ plays: 1, maxPlays: 2 });
  useTakeTestStore.getState().reset();
  useTakeTestStore.getState().hydrate({
    ...session({
      serverTime: "2026-09-01T08:00:00.000Z",
      deadlineAt: "2026-09-01T09:00:00.000Z",
    }),
    questions: [question],
  });
  beginSession("att-1", "ses-1");
});

afterEach(() => {
  clearSession("att-1");
  useTakeTestStore.getState().reset();
  vi.restoreAllMocks();
});

const audio = () => document.querySelector("audio") as HTMLAudioElement;
const pressPlay = () => fireEvent.click(screen.getByRole("button", { name: "Phát" }));
const recorded = () => pending().map((event) => [event.kind, event.questionId]);

function mount() {
  return render(<QuestionAudio question={question} onExpired={() => undefined} />);
}

it("records the play, then its end, for the question", () => {
  mount();

  pressPlay();
  fireEvent.ended(audio());

  expect(recorded()).toEqual([
    ["audio_play", "q1"],
    ["audio_ended", "q1"],
  ]);
  expect(counted).toHaveBeenCalledTimes(1);
});

it("records a blocked play, and counts the play as it did before", async () => {
  play.mockImplementation(() =>
    Promise.reject(new DOMException("blocked", "NotAllowedError")),
  );
  mount();

  pressPlay();

  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(recorded()).toEqual([
    ["audio_play", "q1"],
    ["audio_blocked", "q1"],
  ]);
  expect(counted).toHaveBeenCalledTimes(1);
});

it("records nothing new when the attempt is not loaded", () => {
  useTakeTestStore.getState().reset();
  mount();

  pressPlay();
  fireEvent.ended(audio());

  expect(pending()).toEqual([]);
  expect(counted).not.toHaveBeenCalled();
});

it("records a jump the player put back", () => {
  mount();

  audio().currentTime = 42;
  fireEvent.seeking(audio());

  expect(recorded()).toEqual([["audio_seek", "q1"]]);
  expect(audio().currentTime).toBe(0);
});

it("records no seek when the teacher allowed seeking", () => {
  render(
    <QuestionAudio
      question={{
        ...question,
        audio: { maxPlays: 2, allowSeek: true, showTranscriptAfterSubmit: false },
      }}
      onExpired={() => undefined}
    />,
  );

  audio().currentTime = 42;
  fireEvent.seeking(audio());

  expect(pending()).toEqual([]);
});

it("records no seek when the attempt is not loaded", () => {
  useTakeTestStore.getState().reset();
  mount();

  audio().currentTime = 42;
  fireEvent.seeking(audio());

  expect(pending()).toEqual([]);
});

it("records nothing for a pause before the start, and still counts the play", async () => {
  play.mockImplementation(() =>
    Promise.reject(new DOMException("aborted", "AbortError")),
  );
  mount();

  pressPlay();
  await act(async () => {
    await Promise.resolve();
  });

  expect(recorded()).toEqual([["audio_play", "q1"]]);
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(counted).toHaveBeenCalledTimes(1);
});
