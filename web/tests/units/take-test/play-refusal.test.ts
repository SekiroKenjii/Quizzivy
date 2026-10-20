import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useTakeTestStore } from "@/features/take-test/store";
import { recordAudioPlay } from "@/features/take-test/api";
import { ApiError } from "@/lib/api/errors";
import { session } from "./support";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  recordGroupAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
}));

const played = vi.mocked(recordAudioPlay);
const serverNow = new Date("2026-09-01T08:00:00.000Z");
const deadline = new Date("2026-09-01T08:30:00.000Z");
const store = () => useTakeTestStore.getState();
const refusal = (code: "SESSION_SUPERSEDED" | "ATTEMPT_CLOSED" | "DEADLINE_PASSED") =>
  new ApiError({ status: 409, code, message: code });

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(serverNow);
  played.mockReset();
  store().reset();
  localStorage.clear();
  store().hydrate({
    ...session({
      serverTime: serverNow.toISOString(),
      deadlineAt: deadline.toISOString(),
    }),
    audioPlays: { q1: 1 },
  });
});
afterEach(() => {
  store().reset();
  vi.useRealTimers();
});

it("names the session the tab holds in the request that counts a play", async () => {
  played.mockResolvedValue({ plays: 2, maxPlays: 3 });

  store().notePlay("q1");
  expect(store().audioPlays["q1"]).toBe(2);
  await vi.advanceTimersByTimeAsync(0);

  expect(played).toHaveBeenCalledExactlyOnceWith("att-1", "q1", "ses-1");
  expect(store().audioPlays["q1"]).toBe(2);
});

it.each([
  ["SESSION_SUPERSEDED", "superseded"],
  ["ATTEMPT_CLOSED", "closed"],
  ["DEADLINE_PASSED", "deadline"],
] as const)(
  "takes the count back and locks the paper as %s when the server refuses the play, and does not ask again",
  async (code, lock) => {
    played.mockRejectedValue(refusal(code));

    store().notePlay("q1");
    expect(store().audioPlays["q1"]).toBe(2);
    await vi.advanceTimersByTimeAsync(60_000);

    expect(store().audioPlays["q1"]).toBe(1);
    expect(store().lock).toBe(lock);
    expect(played).toHaveBeenCalledTimes(1);
  },
);

it("keeps the count a failure that is no refusal leaves, for the next fetch to correct", async () => {
  played.mockRejectedValue(
    new ApiError({ status: 500, code: "INTERNAL", message: "x" }),
  );

  store().notePlay("q1");
  await vi.advanceTimersByTimeAsync(60_000);

  expect(store().audioPlays["q1"]).toBe(2);
  expect(store().lock).toBeNull();
});
