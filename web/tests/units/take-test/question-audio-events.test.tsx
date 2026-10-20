import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, screen } from "@testing-library/react";
import { QuestionAudio } from "@/features/take-test/components/QuestionAudio";
import { recordAudioPlay, type StudentQuestion } from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { forgetAllPlaces } from "@/features/media/playback";
import { beginSession, clearSession, pending } from "@/features/integrity/buffer";
import { renderWithPaper, session } from "./support";
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
  audio: {
    maxPlays: 2,
    allowSeek: false,
    showTranscriptAfterSubmit: false,
    allowPause: true,
  },
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
  forgetAllPlaces();
  clearSession("att-1");
  useTakeTestStore.getState().reset();
  vi.restoreAllMocks();
});

const audio = () => document.querySelector("audio") as HTMLAudioElement;
async function click(element: HTMLElement) {
  await act(async () => {
    fireEvent.click(element);
  });
}

const pressPlay = () => click(screen.getByRole("button", { name: "Phát" }));
const recorded = () => pending().map((event) => [event.kind, event.questionId]);

function mount() {
  return renderWithPaper(
    <QuestionAudio question={question} onExpired={() => undefined} />,
  );
}

it("records the play, then its end, for the question", async () => {
  mount();

  await pressPlay();
  fireEvent.ended(audio());

  expect(recorded()).toEqual([
    ["audio_play", "q1"],
    ["audio_ended", "q1"],
  ]);
  expect(counted).toHaveBeenCalledTimes(1);
});

it("records a blocked play, and spends no play on it", async () => {
  play.mockImplementation(() =>
    Promise.reject(new DOMException("blocked", "NotAllowedError")),
  );
  mount();

  await pressPlay();

  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(recorded()).toEqual([["audio_blocked", "q1"]]);
  expect(counted).not.toHaveBeenCalled();
});

it("records nothing new when the attempt is not loaded", async () => {
  useTakeTestStore.getState().reset();
  mount();

  await pressPlay();
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
  renderWithPaper(
    <QuestionAudio
      question={{
        ...question,
        audio: {
          maxPlays: 2,
          allowSeek: true,
          showTranscriptAfterSubmit: false,
          allowPause: true,
        },
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

it("records nothing for a pause before the start, and spends no play on it", async () => {
  play.mockImplementation(() =>
    Promise.reject(new DOMException("aborted", "AbortError")),
  );
  mount();

  await pressPlay();

  expect(recorded()).toEqual([]);
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(counted).not.toHaveBeenCalled();
});

it("records no second play for a resume", async () => {
  mount();

  await pressPlay();
  fireEvent.play(audio());
  audio().currentTime = 4;
  fireEvent.timeUpdate(audio());
  paused = true;
  fireEvent.pause(audio());
  await click(screen.getByRole("button", { name: "Nghe tiếp" }));
  paused = true;
  fireEvent.ended(audio());

  expect(recorded()).toEqual([
    ["audio_play", "q1"],
    ["audio_ended", "q1"],
  ]);
  expect(counted).toHaveBeenCalledTimes(1);
});

it("resumes a play left on another question where it stopped, recording nothing new", async () => {
  const view = mount();
  await pressPlay();
  audio().currentTime = 6;
  fireEvent.timeUpdate(audio());
  view.unmount();

  mount();
  await click(screen.getByRole("button", { name: "Nghe tiếp" }));
  fireEvent.loadedMetadata(audio());

  expect(audio().currentTime).toBe(6);
  expect(recorded()).toEqual([["audio_play", "q1"]]);
  expect(counted).toHaveBeenCalledTimes(1);
});

it("keeps a question's recording that cannot be paused playing, and a stop from elsewhere resumes for no play", async () => {
  renderWithPaper(
    <QuestionAudio
      question={{ ...question, audio: { ...question.audio!, allowPause: false } }}
      onExpired={() => undefined}
    />,
  );

  await pressPlay();
  fireEvent.play(audio());
  audio().currentTime = 3;
  fireEvent.timeUpdate(audio());
  const inert = screen.getByRole("button", { name: "Tạm dừng" });
  expect(inert).toHaveAttribute("aria-disabled", "true");
  fireEvent.click(inert);
  expect(HTMLMediaElement.prototype.pause).not.toHaveBeenCalled();

  paused = true;
  fireEvent.pause(audio());
  await click(screen.getByRole("button", { name: "Nghe tiếp" }));

  expect(play).toHaveBeenCalledTimes(2);
  expect(recorded()).toEqual([["audio_play", "q1"]]);
  expect(counted).toHaveBeenCalledTimes(1);
});
