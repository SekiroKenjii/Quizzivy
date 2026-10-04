import { afterEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { viewport } from "@tests/support/viewport";

const SIDEBAR = "(min-width: 768px)";
const WIDE = "(min-width: 1024px)";
const SEARCH = "(min-width: 1100px)";
const PHONE = "(max-width: 767px)";
const REDUCED_MOTION = "(prefers-reduced-motion: reduce)";

function matches(query: string) {
  return window.matchMedia(query).matches;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("viewport, given a number of pixels", () => {
  it.each([
    [767, false, false, false],
    [768, true, false, false],
    [800, true, false, false],
    [1023, true, false, false],
    [1024, true, true, false],
    [1050, true, true, false],
    [1099, true, true, false],
    [1100, true, true, true],
    [1440, true, true, true],
  ])(
    "at %i answers 768, 1024 and 1100 as %s, %s and %s",
    (width, sidebar, wide, search) => {
      viewport(width);
      expect(matches(SIDEBAR)).toBe(sidebar);
      expect(matches(WIDE)).toBe(wide);
      expect(matches(SEARCH)).toBe(search);
    },
  );

  it.each([
    [500, true],
    [767, true],
    [768, false],
    [800, false],
  ])("at %i answers (max-width: 767px) as %s", (width, expected) => {
    viewport(width);
    expect(matches(PHONE)).toBe(expected);
  });

  it.each([
    [767, false],
    [768, true],
    [1023, true],
    [1024, false],
  ])("at %i answers a query with both bounds as %s", (width, expected) => {
    viewport(width);
    expect(matches("(min-width: 768px) and (max-width: 1023px)")).toBe(expected);
  });

  it("answers false to a query that is not about the width", () => {
    viewport(1440);
    expect(matches(REDUCED_MOTION)).toBe(false);
    expect(matches("(min-height: 600px)")).toBe(false);
    expect(matches("(orientation: landscape)")).toBe(false);
  });

  it("tells a useMediaQuery consumer when it is resized", () => {
    const view = viewport(800);
    const sidebar = renderHook(() => useMediaQuery(SIDEBAR)).result;
    const wide = renderHook(() => useMediaQuery(WIDE)).result;
    const search = renderHook(() => useMediaQuery(SEARCH)).result;
    expect([sidebar.current, wide.current, search.current]).toEqual([
      true,
      false,
      false,
    ]);

    act(() => view.resize(1200));
    expect([sidebar.current, wide.current, search.current]).toEqual([true, true, true]);

    act(() => view.resize(500));
    expect([sidebar.current, wide.current, search.current]).toEqual([
      false,
      false,
      false,
    ]);
  });
});

describe("viewport, given a name", () => {
  it("answers true on desktop to every min-width query and false to any other", () => {
    viewport("desktop");
    expect(matches(SIDEBAR)).toBe(true);
    expect(matches("(min-width: 5000px)")).toBe(true);
    expect(matches(PHONE)).toBe(false);
    expect(matches(REDUCED_MOTION)).toBe(false);
  });

  it("answers false on a phone to every query", () => {
    viewport("phone");
    expect(matches(SIDEBAR)).toBe(false);
    expect(matches("(min-width: 1px)")).toBe(false);
    expect(matches(PHONE)).toBe(false);
    expect(matches(REDUCED_MOTION)).toBe(false);
  });

  it("tells a useMediaQuery consumer when it crosses to the other side", () => {
    const view = viewport("phone");
    const sidebar = renderHook(() => useMediaQuery(SIDEBAR)).result;
    expect(sidebar.current).toBe(false);
    act(() => view.resize("desktop"));
    expect(sidebar.current).toBe(true);
    act(() => view.resize("phone"));
    expect(sidebar.current).toBe(false);
  });

  it("moves between a name and a number", () => {
    const view = viewport("desktop");
    const search = renderHook(() => useMediaQuery(SEARCH)).result;
    expect(search.current).toBe(true);
    act(() => view.resize(1050));
    expect(search.current).toBe(false);
    act(() => view.resize("desktop"));
    expect(search.current).toBe(true);
  });
});
