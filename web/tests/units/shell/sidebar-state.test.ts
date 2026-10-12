import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import {
  readSidebarState,
  useSidebarState,
  writeSidebarState,
} from "@/layouts/shell/sidebarState";

const KEY = "quizzivy.sidebar";

afterEach(() => {
  vi.restoreAllMocks();
  writeSidebarState("expanded");
  localStorage.clear();
});

describe("the sidebar's stored state", () => {
  it("is expanded when nothing is stored", () => {
    expect(localStorage.getItem(KEY)).toBeNull();
    expect(readSidebarState()).toBe("expanded");
  });

  it("is kept under quizzivy.sidebar, as one of two words", () => {
    writeSidebarState("collapsed");
    expect(localStorage.getItem(KEY)).toBe("collapsed");
    expect(readSidebarState()).toBe("collapsed");

    writeSidebarState("expanded");
    expect(localStorage.getItem(KEY)).toBe("expanded");
    expect(readSidebarState()).toBe("expanded");
  });

  it.each(["true", "COLLAPSED", "", "60"])("reads %j as expanded", (stored) => {
    localStorage.setItem(KEY, stored);
    expect(readSidebarState()).toBe("expanded");
  });

  it("is expanded when storage cannot be read", () => {
    localStorage.setItem(KEY, "collapsed");
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("denied");
    });
    expect(readSidebarState()).toBe("expanded");
  });

  it("holds a choice storage refused for as long as the page lives", () => {
    const refuse = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("quota");
    });
    expect(() => writeSidebarState("collapsed")).not.toThrow();
    expect(localStorage.getItem(KEY)).toBeNull();
    expect(readSidebarState()).toBe("collapsed");

    refuse.mockRestore();
    writeSidebarState("expanded");
    expect(localStorage.getItem(KEY)).toBe("expanded");
    expect(readSidebarState()).toBe("expanded");
  });

  it("tells a mounted reader when the choice changes", () => {
    const { result } = renderHook(() => useSidebarState());
    expect(result.current).toBe("expanded");
    act(() => writeSidebarState("collapsed"));
    expect(result.current).toBe("collapsed");
  });
});
