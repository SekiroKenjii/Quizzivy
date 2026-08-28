import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { Button } from "@/components/ui/button";
import { GoogleMark } from "@/features/auth/components/GoogleMark";

// GoogleMark is the only inline <svg> in src/, and AGENTS.md blesses it as the
// sanctioned exception to §12's palette rules -- so it is the reference the next
// icon gets copied from. These pin the sizing contract it is meant to model.

function markIn(container: HTMLElement): SVGSVGElement {
  const svg = container.querySelector("svg");
  if (!svg) throw new Error("no svg rendered");
  return svg as SVGSVGElement;
}

describe("GoogleMark", () => {
  it("carries no size class of its own, so Button's variant rule applies", () => {
    const { container } = render(
      <Button>
        <GoogleMark />
        Tiếp tục với Google
      </Button>,
    );
    // The button's rule is `[&_svg:not([class*='size-'])]:size-4`. Any size-*
    // class here defeats it -- which is the bug this pins.
    expect(markIn(container).getAttribute("class") ?? "").not.toMatch(/size-/);
  });

  it("keeps a caller's className instead of dropping it", () => {
    const { container } = render(<GoogleMark className="size-3 mr-2" />);
    const cls = markIn(container).getAttribute("class") ?? "";
    expect(cls).toContain("size-3");
    expect(cls).toContain("mr-2");
  });

  it("stays out of the accessibility tree", () => {
    render(
      <Button>
        <GoogleMark />
        Tiếp tục với Google
      </Button>,
    );
    // One accessible name, from the text -- not "Google" announced twice.
    expect(screen.getByRole("button", { name: "Tiếp tục với Google" })).toBeInTheDocument();
  });
});
