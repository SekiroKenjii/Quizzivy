import { useEffect } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, render } from "@testing-library/react";
import { useAutosave } from "@/features/tests/useAutosave";

/**
 * `flush()` is what publish relies on, and a version is immutable — one that
 * froze without the last edit cannot be repaired.
 *
 * The window the debounce creates is exactly where the teacher's hand goes:
 * they stop typing, the save fires at 1.5s, and they reach for Publish while
 * the request is still on the wire.
 */
interface Handle {
  schedule: (value: string) => void;
  flush: () => Promise<void>;
}

function Harness({
  save,
  onReady,
}: {
  save: (value: string) => Promise<void>;
  onReady: (handle: Handle) => void;
}) {
  const { schedule, flush } = useAutosave<string>({ save });
  useEffect(() => {
    onReady({ schedule, flush });
  }, [onReady, schedule, flush]);
  return null;
}

let resolveSave: (() => void) | null = null;
let saved: string[] = [];

function save(value: string) {
  saved.push(value);
  return new Promise<void>((resolve) => {
    resolveSave = resolve;
  });
}

function mount() {
  let handle: Handle = {
    schedule: () => {
      throw new Error("not ready");
    },
    flush: () => Promise.reject(new Error("not ready")),
  };
  render(<Harness save={save} onReady={(h) => (handle = h)} />);
  return handle;
}

beforeEach(() => {
  saved = [];
  resolveSave = null;
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("flush and a save already on the wire", () => {
  it("does not resolve before the in-flight request does", async () => {
    const handle = mount();

    act(() => handle.schedule("bản cuối"));
    await act(async () => {
      vi.advanceTimersByTime(1500);
    });
    expect(saved, "the debounced save is now in flight").toEqual(["bản cuối"]);

    let flushed = false;
    const waiting = handle.flush().then(() => {
      flushed = true;
    });

    // Nothing is pending, but the PATCH has not come back. Publishing here is
    // what freezes a version without the last edit.
    await act(async () => {
      await Promise.resolve();
    });
    expect(flushed, "flush returned while the save was still in flight").toBe(false);

    await act(async () => {
      resolveSave?.();
      await waiting;
    });
    expect(flushed).toBe(true);
  });

  it("still sends a pending edit rather than only waiting", async () => {
    const handle = mount();

    act(() => handle.schedule("chưa gửi"));
    const waiting = handle.flush();

    await act(async () => {
      await Promise.resolve();
      resolveSave?.();
      await waiting;
    });

    expect(saved).toEqual(["chưa gửi"]);
  });

  it("resolves immediately when there is nothing to save", async () => {
    const handle = mount();

    await act(async () => {
      await handle.flush();
    });

    expect(saved).toEqual([]);
  });
});
