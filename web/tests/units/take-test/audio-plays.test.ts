import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useTakeTestStore } from "@/features/take-test/store";
import { recordAudioPlay } from "@/features/take-test/api";
import { session } from "./support";

vi.mock("@/features/take-test/api", () => ({
  saveAnswers: vi.fn(),
  submitAttempt: vi.fn(),
  recordAudioPlay: vi.fn(),
  getAttempt: vi.fn(),
}));

const counted = vi.mocked(recordAudioPlay);

/** Drains the microtask queue, so a .then or .catch has actually run. */
const flush = () => new Promise((resolve) => setTimeout(resolve, 0));
const now = "2026-09-01T08:00:00.000Z";
const deadline = "2026-09-01T09:00:00.000Z";

function start(audioPlays: Record<string, number> = {}) {
  const payload = session({ serverTime: now, deadlineAt: deadline });
  useTakeTestStore.getState().hydrate({ ...payload, audioPlays });
}

beforeEach(() => {
  counted.mockReset().mockResolvedValue({ plays: 1, maxPlays: 2 });
  useTakeTestStore.getState().reset();
});
afterEach(() => useTakeTestStore.getState().reset());

/**
 * §11.4: the count is the server's, but the number on screen has to move at
 * once. A student watching "còn 2 lượt nghe" not change has no way to tell a
 * slow network from a play that did not register.
 */
describe("counting a play", () => {
  it("moves immediately, before the server has answered", () => {
    let settle: ((v: { plays: number; maxPlays: number | null }) => void) | undefined;
    counted.mockImplementation(() => new Promise((resolve) => (settle = resolve)));

    start({ q1: 0 });
    useTakeTestStore.getState().notePlay("q1");

    expect(useTakeTestStore.getState().audioPlays["q1"]).toBe(1);
    settle?.({ plays: 1, maxPlays: 2 });
  });

  it("reconciles to whatever the server says", async () => {
    // The server knows about a play this device never saw -- another tab, or a
    // reload that lost an optimistic increment.
    counted.mockResolvedValue({ plays: 5, maxPlays: 2 });

    start({ q1: 0 });
    await useTakeTestStore.getState().notePlay("q1");
    await vi.waitFor(() =>
      expect(useTakeTestStore.getState().audioPlays["q1"]).toBe(5),
    );
  });

  it("keeps the optimistic count when the post fails", async () => {
    counted.mockRejectedValue(new Error("offline"));

    start({ q1: 1 });
    useTakeTestStore.getState().notePlay("q1");
    // Let the rejection settle. Waiting only for the call would assert before
    // any rollback could have run, and pass whether one existed or not.
    await flush();

    // Not rolled back: the play really happened, and the next fetch settles it.
    // Rolling back would hand the student a listen they had already spent.
    expect(useTakeTestStore.getState().audioPlays["q1"]).toBe(2);
  });

  it("counts a question it has never seen a play for", () => {
    start();
    useTakeTestStore.getState().notePlay("q9");
    expect(useTakeTestStore.getState().audioPlays["q9"]).toBe(1);
  });

  it("does nothing before an attempt is loaded", () => {
    useTakeTestStore.getState().notePlay("q1");
    expect(counted).not.toHaveBeenCalled();
  });
});

/**
 * The reason the count lives on the server at all: a reload is exactly when a
 * client-side counter would forget, and exactly when a student wanting extra
 * listens would reach for one.
 */
describe("after a reload", () => {
  it("takes the server's count over anything remembered locally", async () => {
    counted.mockResolvedValue({ plays: 1, maxPlays: 2 });
    start({ q1: 0 });
    useTakeTestStore.getState().notePlay("q1");
    await vi.waitFor(() =>
      expect(useTakeTestStore.getState().audioPlays["q1"]).toBe(1),
    );

    const payload = session({ serverTime: now, deadlineAt: deadline });
    useTakeTestStore.getState().hydrate({ ...payload, audioPlays: { q1: 2 } });

    expect(useTakeTestStore.getState().audioPlays["q1"]).toBe(2);
  });
});
