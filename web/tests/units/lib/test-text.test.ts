import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  readLargerTestText,
  useLargerTestText,
  writeLargerTestText,
} from "@/lib/testText";

const KEY = "quizzivy.testText";

beforeEach(() => localStorage.clear());
afterEach(() => {
  vi.restoreAllMocks();
  writeLargerTestText(false);
  localStorage.clear();
});

describe("larger text in tests", () => {
  it("is off until the student turns it on", () => {
    expect(readLargerTestText()).toBe(false);
    expect(localStorage.getItem(KEY)).toBeNull();
  });

  it("is on only for the value the switch writes", () => {
    localStorage.setItem(KEY, "large");
    expect(readLargerTestText()).toBe(true);
    for (const other of ["default", "true", "1", "LARGE", ""]) {
      localStorage.setItem(KEY, other);
      expect(readLargerTestText(), other).toBe(false);
    }
  });

  it("stores the choice under the key the engine reads, on and off", () => {
    writeLargerTestText(true);
    expect(localStorage.getItem(KEY)).toBe("large");
    expect(readLargerTestText()).toBe(true);
    writeLargerTestText(false);
    expect(localStorage.getItem(KEY)).toBe("default");
    expect(readLargerTestText()).toBe(false);
  });

  it("tells a mounted reader at once", () => {
    const { result } = renderHook(() => useLargerTestText());
    expect(result.current).toBe(false);
    act(() => writeLargerTestText(true));
    expect(result.current).toBe(true);
    act(() => writeLargerTestText(false));
    expect(result.current).toBe(false);
  });

  it("follows a change made in another tab", () => {
    const { result } = renderHook(() => useLargerTestText());
    act(() => {
      localStorage.setItem(KEY, "large");
      window.dispatchEvent(new StorageEvent("storage", { key: KEY }));
    });
    expect(result.current).toBe(true);
    act(() => {
      localStorage.clear();
      window.dispatchEvent(new StorageEvent("storage", { key: null }));
    });
    expect(result.current).toBe(false);
  });

  it("ignores another key's change and stops listening when unmounted", () => {
    let renders = 0;
    const { unmount } = renderHook(() => {
      renders += 1;
      return useLargerTestText();
    });
    const before = renders;
    act(() => {
      localStorage.setItem(KEY, "large");
      window.dispatchEvent(new StorageEvent("storage", { key: "quizzivy.theme" }));
    });
    expect(renders).toBe(before);
    unmount();
    act(() => writeLargerTestText(true));
    expect(renders).toBe(before);
  });

  it("reads off when storage throws", () => {
    localStorage.setItem(KEY, "large");
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("denied");
    });
    expect(readLargerTestText()).toBe(false);
  });

  it("holds a choice the browser refuses to store, for as long as the page lives", () => {
    const get = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("storage is off");
    });
    const set = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("storage is off");
    });
    const { result } = renderHook(() => useLargerTestText());

    act(() => writeLargerTestText(true));
    expect(result.current).toBe(true);
    expect(readLargerTestText()).toBe(true);
    act(() => writeLargerTestText(false));
    expect(result.current).toBe(false);

    get.mockRestore();
    set.mockRestore();
    act(() => writeLargerTestText(true));
    expect(localStorage.getItem(KEY)).toBe("large");
    expect(readLargerTestText()).toBe(true);
    expect(result.current).toBe(true);
    localStorage.setItem(KEY, "default");
    expect(readLargerTestText()).toBe(false);
  });

  it("reads nothing more once its reader is gone", () => {
    const { unmount } = renderHook(() => useLargerTestText());
    unmount();
    const read = vi.spyOn(Storage.prototype, "getItem");
    act(() => {
      writeLargerTestText(true);
      window.dispatchEvent(new StorageEvent("storage", { key: KEY }));
    });
    expect(read).not.toHaveBeenCalled();
  });
});
