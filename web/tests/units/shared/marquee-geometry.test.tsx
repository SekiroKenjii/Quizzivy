import { render } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MarqueeText } from "@/components/shared/MarqueeText";

const matchMedia = window.matchMedia;
let reduced = false;

beforeEach(() => {
  reduced = false;
  window.matchMedia = (query) =>
    ({
      matches: query.includes("prefers-reduced-motion") && reduced,
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
    }) as unknown as MediaQueryList;
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockReturnValue({
    width: 600,
  } as DOMRect);
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(200);
});

afterEach(() => {
  window.matchMedia = matchMedia;
  vi.restoreAllMocks();
});

it("preserves existing gap and stylesheet mask defaults when geometry is omitted", () => {
  const { container } = render(<MarqueeText text="A sufficiently long title" />);
  expect(container.firstElementChild).toHaveClass("qz-marquee-masked");
  expect((container.firstElementChild as HTMLElement).style.maskImage).toBe("");
  const duplicate = container.querySelector<HTMLElement>(
    '.qz-marquee-track [aria-hidden="true"]',
  )!;
  expect(duplicate).toHaveClass("pl-8");
  expect(duplicate.style.paddingLeft).toBe("");
});

it.each([
  [48, 16],
  [28, 10],
])(
  "exposes a %ipx duplicate gap and %ipx edge mask without duplicating accessible text",
  (gapPx, maskPx) => {
    const { container } = render(
      <MarqueeText text="A sufficiently long title" gapPx={gapPx} maskPx={maskPx} />,
    );
    const root = container.firstElementChild as HTMLElement;
    expect(root.style.maskImage).toBe(
      `linear-gradient(90deg, transparent, var(--foreground) ${maskPx}px, var(--foreground) calc(100% - ${maskPx}px), transparent)`,
    );
    const track = container.querySelector<HTMLElement>(".qz-marquee-track")!;
    expect(track.style.animationDuration).toBe("8s");
    expect(track.querySelectorAll('[aria-hidden="true"]')).toHaveLength(1);
    expect(
      track.querySelector<HTMLElement>('[aria-hidden="true"]')!.style.paddingLeft,
    ).toBe(`${gapPx}px`);
    expect(track.firstElementChild).not.toHaveAttribute("aria-hidden");
  },
);

it("omits custom moving geometry when the text fits", () => {
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(800);
  const { container } = render(
    <MarqueeText text="A fitting title" gapPx={48} maskPx={16} />,
  );
  expect(container.querySelector(".qz-marquee-track")).toBeNull();
  expect((container.firstElementChild as HTMLElement).style.maskImage).toBe("");
});

it("keeps reduced-motion overflow static without an edge mask", () => {
  reduced = true;
  const { container } = render(
    <MarqueeText text="A sufficiently long title" gapPx={48} maskPx={16} />,
  );
  expect(container.querySelector(".qz-marquee-track")).toBeNull();
  expect(container.firstElementChild).toHaveClass("text-ellipsis");
  expect((container.firstElementChild as HTMLElement).style.maskImage).toBe("");
});
