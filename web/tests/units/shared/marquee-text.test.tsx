import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { MarqueeText } from "@/components/shared/MarqueeText";

const CSS = readFileSync(
  resolve(import.meta.dirname, "../../../src/index.css"),
  "utf8",
);
const REDUCED = CSS.split("@media (prefers-reduced-motion: reduce)")
  .slice(1)
  .map((block) => block.slice(0, block.indexOf("\n}")));
const original = window.matchMedia;
let reduced = false;

function setWidths(text: number, box: number) {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
    width: text,
  } as DOMRect);
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(box);
}

beforeEach(() => {
  reduced = false;
  window.matchMedia = (query: string) =>
    ({
      matches: query.includes("prefers-reduced-motion") ? reduced : false,
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
    }) as unknown as MediaQueryList;
});

afterEach(() => {
  window.matchMedia = original;
  vi.restoreAllMocks();
});

const title = "Mid-term Reading Mock · Urban green space and the history of maps";

describe("MarqueeText", () => {
  it("stays still and uncut when the text fits", () => {
    setWidths(120, 300);
    const { container } = render(<MarqueeText text={title} />);
    expect(container.querySelector(".qz-marquee-track")).toBeNull();
    expect(container.querySelector('[aria-hidden="true"]')).toBeNull();
    expect(container.firstElementChild).not.toHaveAttribute("title");
  });

  it("scrolls when the text overflows, reading it once", () => {
    setWidths(600, 200);
    const { container } = render(<MarqueeText text={title} />);
    const track = container.querySelector(".qz-marquee-track");
    expect(track).not.toBeNull();
    expect(container.querySelectorAll('[aria-hidden="true"]')).toHaveLength(1);
    expect(container.firstElementChild).toHaveAttribute("title", title);
    expect((track as HTMLElement).style.animationDuration).toBe("16s");
  });

  it("truncates with an ellipsis instead of moving under reduced motion", () => {
    reduced = true;
    setWidths(600, 200);
    const { container } = render(<MarqueeText text={title} />);
    expect(container.querySelector(".qz-marquee-track")).toBeNull();
    expect(container.firstElementChild).toHaveClass("text-ellipsis");
    expect(container.firstElementChild).toHaveAttribute("title", title);
  });

  it("pauses under the pointer and keyboard focus, and holds still under reduced motion", () => {
    expect(CSS).toMatch(/\.qz-marquee:hover \.qz-marquee-track/);
    expect(CSS).toMatch(/:focus-visible \.qz-marquee-track/);
    expect(CSS).toMatch(/animation-play-state: paused/);
    expect(
      REDUCED.some(
        (b) => b.includes(".qz-marquee-track") && b.includes("animation: none"),
      ),
    ).toBe(true);
  });
});
