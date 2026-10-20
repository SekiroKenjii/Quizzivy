import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useTakeTestStore } from "@/features/take-test/store";
import { flushEvents, recordAudioPlay, saveAnswers } from "@/features/take-test/api";
import { ApiError } from "@/lib/api/errors";
import {
  beginSession,
  clearSession,
  pending,
  record,
} from "@/features/integrity/buffer";
import { session } from "./support";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  recordGroupAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
  flushEvents: vi.fn(),
}));

const saved = vi.mocked(saveAnswers);
const sent = vi.mocked(flushEvents);
const played = vi.mocked(recordAudioPlay);
const serverNow = new Date("2026-09-01T08:00:00.000Z");
const deadline = new Date("2026-09-01T08:30:00.000Z");
const store = () => useTakeTestStore.getState();
const taken = () =>
  new ApiError({ status: 409, code: "SESSION_SUPERSEDED", message: "elsewhere" });

function hydrate(over: { superseded?: boolean } = {}) {
  store().hydrate({
    ...session({
      serverTime: serverNow.toISOString(),
      deadlineAt: deadline.toISOString(),
    }),
    ...over,
  });
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(serverNow);
  saved.mockReset();
  played.mockReset();
  sent.mockReset().mockResolvedValue(undefined as never);
  sessionStorage.clear();
  clearSession("att-1");
  store().reset();
  localStorage.clear();
});
afterEach(() => {
  store().reset();
  clearSession("att-1");
  vi.useRealTimers();
});

it("sends the events a save's 409 left, once and under its own session, and stops recording", async () => {
  hydrate();
  beginSession("att-1", "ses-1");
  record("att-1", "network_offline");
  record("att-1", "copy");
  saved.mockRejectedValue(taken());
  store().setAnswer("q1", { type: "choice", optionIds: ["a"] });

  await store().flush();
  await vi.advanceTimersByTimeAsync(0);

  expect(store().lock).toBe("superseded");
  expect(sent).toHaveBeenCalledOnce();
  expect(sent.mock.calls[0]![0]).toBe("att-1");
  expect(sent.mock.calls[0]![1].sessionId).toBe("ses-1");
  expect(sent.mock.calls[0]![1].events.map((event) => event.kind)).toEqual([
    "network_offline",
    "copy",
  ]);
  expect(pending()).toEqual([]);

  record("att-1", "window_blur");
  expect(pending()).toEqual([]);

  store().lockNow("superseded");
  await vi.advanceTimersByTimeAsync(0);
  expect(sent).toHaveBeenCalledOnce();
});

it("drops the events when the send is refused", async () => {
  hydrate();
  beginSession("att-1", "ses-1");
  record("att-1", "copy");
  sent.mockRejectedValue(taken());
  saved.mockRejectedValue(taken());
  store().setAnswer("q1", { type: "choice", optionIds: ["a"] });

  await store().flush();
  await vi.advanceTimersByTimeAsync(0);

  expect(sent).toHaveBeenCalledOnce();
  expect(pending()).toEqual([]);
  expect(JSON.parse(sessionStorage.getItem("quizzivy.integrity.att-1")!)).toMatchObject(
    {
      events: [],
    },
  );
});

it("sends them when an audio play is answered with 409", async () => {
  hydrate();
  beginSession("att-1", "ses-1");
  record("att-1", "context_menu");
  played.mockRejectedValue(taken());

  store().notePlay("q1");
  await vi.advanceTimersByTimeAsync(0);

  expect(store().lock).toBe("superseded");
  expect(sent).toHaveBeenCalledOnce();
  expect(sent.mock.calls[0]![1].events.map((event) => event.kind)).toEqual([
    "context_menu",
  ]);
});

it("sends what a reload left when the paper opens already superseded", async () => {
  beginSession("att-1", "ses-1");
  record("att-1", "paste");

  hydrate({ superseded: true });
  await vi.advanceTimersByTimeAsync(0);

  expect(sent).toHaveBeenCalledOnce();
  expect(sent.mock.calls[0]![1].sessionId).toBe("ses-1");
  expect(pending()).toEqual([]);
});

it("sends nothing for a paper that is not superseded", async () => {
  hydrate();
  beginSession("att-1", "ses-1");
  record("att-1", "copy");

  await vi.advanceTimersByTimeAsync(5_000);

  expect(sent).not.toHaveBeenCalled();
  expect(pending()).toHaveLength(1);
});
