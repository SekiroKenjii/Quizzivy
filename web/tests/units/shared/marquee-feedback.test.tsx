import { render } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MarqueeText } from "@/components/shared/MarqueeText";

const TEXT_WIDTH = 263;

function inFlowWidth(box: HTMLElement): number {
  const track = box.querySelector<HTMLElement>(".qz-marquee-track");
  const trackInFlow = track !== null && !track.classList.contains("absolute");
  return trackInFlow ? TEXT_WIDTH * 2 + 48 : TEXT_WIDTH;
}

beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (
    this: HTMLElement,
  ) {
    return { width: TEXT_WIDTH } as DOMRect;
  });
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockImplementation(function (
    this: HTMLElement,
  ) {
    if (!this.classList.contains("qz-marquee")) return 0;
    return inFlowWidth(this) > TEXT_WIDTH ? 308 : 230;
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

it("settles in a host that grows with its content, as the builder's title bar does", () => {
  const { container } = render(
    <MarqueeText text="IELTS Reading Practice 3" gapPx={48} />,
  );

  const box = container.firstElementChild as HTMLElement;
  expect(box).toHaveClass("qz-marquee-masked");
  expect(box.querySelector(".qz-marquee-track")).toHaveClass("absolute");
  expect(box.querySelector('[data-slot="marquee-sizer"]')).toHaveTextContent(
    "IELTS Reading Practice 3",
  );
});
