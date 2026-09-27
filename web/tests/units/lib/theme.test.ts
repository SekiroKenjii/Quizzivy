import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  readThemePreference,
  useForcedLightTheme,
  useResolvedTheme,
  writeThemePreference,
} from "@/lib/theme";

let deviceDark = false;
const changeListeners = new Set<() => void>();
const original = window.matchMedia;

function deviceTurns(dark: boolean) {
  deviceDark = dark;
  for (const listener of changeListeners) listener();
}

beforeEach(() => {
  deviceDark = false;
  changeListeners.clear();
  localStorage.clear();
  window.matchMedia = (query: string) =>
    ({
      get matches() {
        return query.includes("prefers-color-scheme: dark") ? deviceDark : false;
      },
      media: query,
      addEventListener: (_: string, listener: () => void) =>
        changeListeners.add(listener),
      removeEventListener: (_: string, listener: () => void) =>
        changeListeners.delete(listener),
    }) as unknown as MediaQueryList;
});

afterEach(() => {
  window.matchMedia = original;
  vi.restoreAllMocks();
  document.documentElement.classList.remove("dark");
});

const isDark = () => document.documentElement.classList.contains("dark");

describe("theme preference", () => {
  it("defaults to light", () => {
    expect(readThemePreference()).toBe("light");
  });

  it("treats an unknown stored value as light", () => {
    localStorage.setItem("quizzivy.theme", "sepia");
    expect(readThemePreference()).toBe("light");
  });

  it("falls back to light when storage throws", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("denied");
    });
    expect(readThemePreference()).toBe("light");
  });

  it("applies a chosen theme and reports it", () => {
    const { result } = renderHook(() => useResolvedTheme());
    act(() => writeThemePreference("dark"));
    expect(isDark()).toBe(true);
    expect(document.documentElement.style.colorScheme).toBe("dark");
    expect(result.current).toBe("dark");
    act(() => writeThemePreference("light"));
    expect(isDark()).toBe(false);
    expect(result.current).toBe("light");
  });

  it("follows the device live when set to system", () => {
    const { result } = renderHook(() => useResolvedTheme());
    act(() => writeThemePreference("system"));
    expect(result.current).toBe("light");
    act(() => deviceTurns(true));
    expect(isDark()).toBe(true);
    expect(result.current).toBe("dark");
    act(() => deviceTurns(false));
    expect(result.current).toBe("light");
  });

  it("keeps a screen light while it forces it, then restores the preference", () => {
    act(() => writeThemePreference("dark"));
    const reader = renderHook(() => useResolvedTheme());
    const forced = renderHook(() => useForcedLightTheme());
    expect(isDark()).toBe(false);
    expect(reader.result.current).toBe("light");
    forced.unmount();
    expect(isDark()).toBe(true);
    expect(reader.result.current).toBe("dark");
  });
});
