import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useTakeTestStore } from "@/features/take-test/store";
import { saveAnswers } from "@/features/take-test/api";
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
}));

const saved = vi.mocked(saveAnswers);
const serverNow = new Date("2026-09-01T08:00:00.000Z");
const deadline = new Date("2026-09-01T08:30:00.000Z");
const store = () => useTakeTestStore.getState();
const reply = {
  serverTime: serverNow.toISOString(),
  savedAt: serverNow.toISOString(),
  deadlineAt: deadline.toISOString(),
};

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(serverNow);
  saved.mockReset().mockResolvedValue(reply);
  sessionStorage.clear();
  clearSession("att-1");
  store().reset();
  localStorage.clear();
  store().hydrate(
    session({
      serverTime: serverNow.toISOString(),
      deadlineAt: deadline.toISOString(),
    }),
  );
  beginSession("att-1", "ses-1");
});
afterEach(() => {
  store().reset();
  clearSession("att-1");
  vi.useRealTimers();
});

it("saves the answer with the oldest 500 events and sends the other 100 in the next request", async () => {
  for (let at = 0; at < 600; at += 1) record("att-1", "window_blur");
  store().setAnswer("q1", { type: "choice", optionIds: ["a"] });

  await store().flush();
  expect(saved).toHaveBeenCalledTimes(1);
  const first = saved.mock.calls[0]![1];
  expect(first.answers).toEqual({ q1: { type: "choice", optionIds: ["a"] } });
  expect(first.events).toHaveLength(500);
  expect(pending()).toHaveLength(100);
  expect(store().dirty.size).toBe(0);

  await vi.advanceTimersByTimeAsync(10_000);

  expect(saved).toHaveBeenCalledTimes(2);
  expect(saved.mock.calls[1]![1].events).toHaveLength(100);
  expect(saved.mock.calls[1]![1].events?.[0]?.clientSeq).toBe(500);
  expect(pending()).toEqual([]);
});

it("keeps the events of a request that failed, so the next one carries them", async () => {
  saved.mockRejectedValueOnce(new Error("offline"));
  for (let at = 0; at < 600; at += 1) record("att-1", "window_blur");
  store().setAnswer("q1", { type: "choice", optionIds: ["a"] });

  await store().flush();

  expect(pending()).toHaveLength(600);
});
