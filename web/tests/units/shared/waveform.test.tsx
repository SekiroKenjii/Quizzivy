import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { Waveform } from "@/components/shared/Waveform";
import { waveformBars, waveformSeed } from "@/components/shared/waveformBars";

describe("decorative waveform", () => {
  it("pins the deck's entire first row and second card start", () => {
    expect(waveformBars(0)).toEqual([
      20, 57, 24, 61, 28, 65, 32, 69, 36, 73, 40, 77, 44, 81, 48, 85, 52, 89, 56, 23,
      60, 27, 64, 31, 68, 35, 72, 39,
    ]);
    expect(waveformBars(1).slice(0, 2)).toEqual([31, 68]);
    for (let seed = 0; seed < 20; seed++)
      for (const height of waveformBars(seed)) {
        expect(height).toBeGreaterThanOrEqual(20);
        expect(height).toBeLessThanOrEqual(89);
      }
  });
  it("uses stable FNV-1a ids without randomness and preserves numeric indexes", () => {
    expect(waveformSeed("hello")).toBe(1335831723);
    expect(waveformSeed(2)).toBe(2);
    expect(waveformSeed("asset-a")).toBe(waveformSeed("asset-a"));
    expect(waveformBars(waveformSeed("asset-a"))).not.toEqual(
      waveformBars(waveformSeed("asset-b")),
    );
    const { container, rerender } = render(<Waveform seed="asset-a" />);
    const first = Array.from(container.firstElementChild!.children).map((bar) =>
      bar.getAttribute("style"),
    );
    rerender(<Waveform seed="asset-a" />);
    expect(
      Array.from(container.firstElementChild!.children).map((bar) =>
        bar.getAttribute("style"),
      ),
    ).toEqual(first);
  });
  it.each([
    [0, 0],
    [11 / 28, 11],
    [1, 28],
    [-1, 0],
    [2, 28],
    [0.02, 1],
  ])("colours progress %s on %s bars", (progress, expected) => {
    const { container } = render(<Waveform seed={0} progress={progress} />);
    const bars = container.firstElementChild!.children;
    expect(bars).toHaveLength(28);
    expect(container.querySelectorAll(".bg-fg")).toHaveLength(expected);
    expect(container.querySelectorAll(".bg-ring")).toHaveLength(28 - expected);
    expect(Array.from(bars).map((bar) => bar.getAttribute("style"))).toEqual(
      waveformBars(0).map((height) => `height: ${height}%;`),
    );
  });
  it("is hidden decoration with no role, label or focusable child and supports caller height", () => {
    const { container } = render(<Waveform seed={2} className="h-20" />);
    const root = container.firstElementChild!;
    expect(root).toHaveAttribute("aria-hidden", "true");
    expect(root).not.toHaveAttribute("role");
    expect(root).not.toHaveAttribute("aria-label");
    expect(root).not.toHaveAttribute("tabindex");
    expect(container.querySelector("button,a,input,[tabindex]")).toBeNull();
    expect(root).toHaveClass("h-20");
    expect(root).not.toHaveClass("h-11");
    expect(container.querySelectorAll(".bg-fg")).toHaveLength(0);
  });
});
