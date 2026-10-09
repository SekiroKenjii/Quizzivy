import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PROBE_TIMEOUT_MS, readDuration } from "@/features/media/probe";

/**
 * A file whose metadata never arrives must not hold the upload at "checking"
 * for ever: the read gives up, as unknown, and lets go of its object URL.
 */
let revoked: string[] = [];

beforeEach(() => {
  vi.useFakeTimers();
  revoked = [];
  URL.createObjectURL = vi.fn(() => "blob:probe");
  URL.revokeObjectURL = vi.fn((url: string) => {
    revoked.push(url);
  });
  Object.defineProperty(HTMLMediaElement.prototype, "src", {
    configurable: true,
    set() {},
  });
});

afterEach(() => vi.useRealTimers());

describe("the duration read", () => {
  it("gives up after ten seconds and revokes its object URL", async () => {
    let settled: number | null | undefined;
    void readDuration(new File([new Uint8Array(4)], "treo.mp3")).then((value) => {
      settled = value;
    });

    await vi.advanceTimersByTimeAsync(PROBE_TIMEOUT_MS - 1);
    expect(settled).toBeUndefined();
    expect(revoked).toEqual([]);

    await vi.advanceTimersByTimeAsync(1);
    expect(settled).toBeNull();
    expect(revoked).toEqual(["blob:probe"]);
    expect(PROBE_TIMEOUT_MS).toBe(10_000);
  });
});
