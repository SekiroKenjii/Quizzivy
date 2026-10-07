import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cancelTransitionInterest,
  cookieOperation,
  finishTransition,
  reserveTransition,
  setTransitionPhase,
  transitionStatus,
  waitForTransition,
} from "@/lib/api/authTransition";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

afterEach(() => vi.useRealTimers());

describe("raw cookie ownership", () => {
  it("admits one waiting transition only after the complete refresh body settles", async () => {
    const body = deferred<void>();
    const refresh = cookieOperation("refresh", 1, () => body.promise);
    const ticket = reserveTransition("login", 2);
    expect(() => reserveTransition("google", 2)).toThrow();
    const send = vi.fn(() => Promise.resolve("session"));
    const login = cookieOperation("login", 2, send, ticket);
    await Promise.resolve();
    expect(send).not.toHaveBeenCalled();
    expect(transitionStatus().cookieKind).toBe("refresh");
    body.resolve();
    await refresh;
    await expect(login).resolves.toBe("session");
    expect(send).toHaveBeenCalledOnce();
    finishTransition(ticket);
    expect(transitionStatus().kind).toBe("idle");
  });

  it("settles the UI deadline without releasing raw ownership or replaying a request", async () => {
    vi.useFakeTimers();
    const body = deferred<string>();
    const ticket = reserveTransition("login", 3);
    const send = vi.fn(() => body.promise);
    const raw = cookieOperation("login", 3, send, ticket);
    const visible = waitForTransition(ticket, raw);
    const failure = expect(visible).rejects.toMatchObject({ phase: "account" });
    await vi.advanceTimersByTimeAsync(19_999);
    expect(ticket.interested).toBe(true);
    expect(transitionStatus().cookieKind).toBe("login");
    await vi.advanceTimersByTimeAsync(1);
    await failure;
    expect(ticket.interested).toBe(false);
    expect(transitionStatus().cookieKind).toBe("login");
    expect(() => reserveTransition("login", 4)).toThrow();
    expect(send).toHaveBeenCalledOnce();
    body.resolve("late");
    await raw;
    finishTransition(ticket);
    expect(transitionStatus().kind).toBe("idle");
  });

  it("keeps cleanup admission closed after cancellation and ignores an obsolete release", async () => {
    const ticket = reserveTransition("logout", 4);
    cancelTransitionInterest(ticket);
    expect(() => reserveTransition("login", 5)).toThrow();
    finishTransition(ticket);
    const next = reserveTransition("login", 5);
    finishTransition(ticket);
    expect(transitionStatus().kind).toBe("login");
    finishTransition(next);
  });
});

it("changes cleanup UI status at ten seconds while keeping its natural completion owned", async () => {
  vi.useFakeTimers();
  const body = deferred<void>();
  const ticket = reserveTransition("logout", 8);
  setTransitionPhase(ticket, "cleanup");
  let settled = false;
  const visible = waitForTransition(ticket, body.promise).catch((error: unknown) => {
    settled = true;
    return error;
  });
  try {
    await vi.advanceTimersByTimeAsync(9_999);
    expect(settled).toBe(false);
    expect(ticket.interested).toBe(true);
    await vi.advanceTimersByTimeAsync(1);
    expect(settled).toBe(true);
    expect(await visible).toMatchObject({ phase: "cleanup" });
    expect(() => reserveTransition("login", 9)).toThrow();
  } finally {
    body.resolve();
    await visible;
    finishTransition(ticket);
  }
});
