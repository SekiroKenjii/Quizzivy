import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { GroupListening } from "@/features/take-test/components/GroupContext";
import { recordGroupAudioPlay } from "@/features/take-test/api";
import { useTakeTestStore } from "@/features/take-test/store";
import { useGroupPlaybackStore } from "@/features/take-test/groupPlayback";
import { beginSession, clearSession, pending } from "@/features/integrity/buffer";
import { previewGroup } from "@tests/support/groupPreview";
import { session } from "./support";
import "@/lib/i18n";

vi.mock("@/features/take-test/api", () => ({
  getAttempt: vi.fn(),
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  recordGroupAudioPlay: vi.fn(),
}));

const counted = vi.mocked(recordGroupAudioPlay);
const originalNotePlay = useGroupPlaybackStore.getState().notePlay;
const recordingId = previewGroup.recordings[0]!.id;
const T0 = Date.parse("2026-09-01T08:00:00.000Z");
let play: ReturnType<typeof vi.fn<() => Promise<void>>>;
let paused = true;

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"], now: T0 });
  paused = true;
  play = vi.fn(() => {
    paused = false;
    return Promise.resolve();
  });
  vi.spyOn(HTMLMediaElement.prototype, "play").mockImplementation(play);
  vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {
    paused = true;
  });
  vi.spyOn(HTMLMediaElement.prototype, "paused", "get").mockImplementation(
    () => paused,
  );
  sessionStorage.clear();
  localStorage.clear();
  useTakeTestStore.getState().reset();
  counted.mockReset().mockImplementation(async (_attempt, body) => ({
    playId: body.playId,
    plays: 1,
    maxPlays: 2,
  }));
  useTakeTestStore.getState().hydrate({
    ...session({
      serverTime: new Date(T0).toISOString(),
      deadlineAt: new Date(T0 + 3600000).toISOString(),
    }),
    groups: [previewGroup],
    groupAudioPlays: {},
  });
  beginSession("att-1", "ses-1");
});

afterEach(() => {
  useGroupPlaybackStore.setState({ notePlay: originalNotePlay });
  clearSession("att-1");
  useTakeTestStore.getState().reset();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

const audio = () => document.querySelector("audio") as HTMLAudioElement;
const pressPlay = () => fireEvent.click(screen.getByRole("button", { name: "Phát" }));
const mount = () =>
  render(<GroupListening group={previewGroup} onRetryMedia={() => undefined} />);
const duration = () => pending().at(-1)?.meta;
const advance = (ms: number) => vi.setSystemTime(Date.now() + ms);
const flush = () => act(() => useGroupPlaybackStore.getState().flush());

it("records the end of a shared play, with the recording and how long it lasted", async () => {
  mount();
  pressPlay();
  advance(12000);
  fireEvent.ended(audio());
  expect(pending()).toEqual([
    {
      kind: "audio_ended",
      occurredAt: new Date(T0 + 12000).toISOString(),
      clientSeq: 0,
      meta: { scope: "group", recordingId, durationMs: 12000 },
    },
  ]);
  expect(counted).toHaveBeenCalledTimes(1);
  await flush();
});

it("records a shared play the browser blocked", async () => {
  play.mockImplementation(() =>
    Promise.reject(new DOMException("blocked", "NotAllowedError")),
  );
  mount();
  pressPlay();
  expect(await screen.findByRole("alert")).toBeInTheDocument();
  expect(
    pending().map(({ kind, questionId, meta }) => ({ kind, questionId, meta })),
  ).toEqual([
    {
      kind: "audio_blocked",
      questionId: undefined,
      meta: { scope: "group", recordingId },
    },
  ]);
  expect(counted).toHaveBeenCalledTimes(1);
  await flush();
});

it("records nothing for a pause before the start", async () => {
  play.mockImplementation(() =>
    Promise.reject(new DOMException("aborted", "AbortError")),
  );
  mount();
  pressPlay();
  await act(async () => {
    await Promise.resolve();
  });
  expect(pending()).toEqual([]);
  expect(screen.queryByRole("alert")).toBeNull();
  expect(counted).toHaveBeenCalledTimes(1);
  await flush();
});

it("measures from the last start after a pause and a resume", async () => {
  mount();
  pressPlay();
  advance(5000);
  paused = true;
  fireEvent.pause(audio());
  advance(30000);
  pressPlay();
  advance(7000);
  fireEvent.ended(audio());
  expect(duration()).toEqual({ scope: "group", recordingId, durationMs: 7000 });
  await flush();
  expect(counted).toHaveBeenCalledTimes(2);
});

it("records nothing when the attempt is not loaded", () => {
  useTakeTestStore.getState().reset();
  mount();
  pressPlay();
  fireEvent.ended(audio());
  expect(pending()).toEqual([]);
  expect(counted).not.toHaveBeenCalled();
});

it("records no blocked event when the attempt is not loaded", async () => {
  useTakeTestStore.getState().reset();
  play.mockImplementation(() =>
    Promise.reject(new DOMException("blocked", "NotAllowedError")),
  );
  mount();
  pressPlay();
  await screen.findByRole("alert");
  expect(pending()).toEqual([]);
  expect(counted).not.toHaveBeenCalled();
});

it("reports zero when the clock moves backwards during a play", async () => {
  mount();
  pressPlay();
  advance(-5000);
  fireEvent.ended(audio());
  expect(duration()).toEqual({ scope: "group", recordingId, durationMs: 0 });
  await flush();
});

it("omits a duration when no start was observed", () => {
  mount();
  fireEvent.ended(audio());
  expect(duration()).toEqual({ scope: "group", recordingId });
});

it("forgets the start after an end", async () => {
  mount();
  pressPlay();
  advance(12000);
  fireEvent.ended(audio());
  advance(5000);
  fireEvent.ended(audio());
  expect(pending().map((event) => event.meta)).toEqual([
    { scope: "group", recordingId, durationMs: 12000 },
    { scope: "group", recordingId },
  ]);
  await flush();
});

it("forgets the start after a block", async () => {
  play.mockImplementation(() =>
    Promise.reject(new DOMException("blocked", "NotAllowedError")),
  );
  mount();
  pressPlay();
  await screen.findByRole("alert");
  advance(5000);
  fireEvent.ended(audio());
  expect(duration()).toEqual({ scope: "group", recordingId });
  await flush();
});

it("captures the start before counting and plays in the click's synchronous tick", async () => {
  const note = useGroupPlaybackStore.getState().notePlay;
  useGroupPlaybackStore.setState({
    notePlay: (id) => {
      advance(5000);
      note(id);
    },
  });
  mount();
  pressPlay();
  expect(play).toHaveBeenCalledTimes(1);
  advance(7000);
  fireEvent.ended(audio());
  expect(duration()).toEqual({ scope: "group", recordingId, durationMs: 12000 });
  useGroupPlaybackStore.setState({ notePlay: note });
  await flush();
});
