import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LiveDot } from "@/components/shared/LiveDot";

const CSS = readFileSync(
  resolve(import.meta.dirname, "../../../src/index.css"),
  "utf8",
);
const REDUCED = CSS.split("@media (prefers-reduced-motion: reduce)")
  .slice(1)
  .map((block) => block.slice(0, block.indexOf("\n}")));

describe("LiveDot", () => {
  it("is decoration, hidden from assistive technology", () => {
    const { container } = render(<LiveDot />);
    expect(container.firstElementChild).toHaveAttribute("aria-hidden", "true");
    expect(container.firstElementChild).toHaveClass("qz-live-dot");
  });

  it("holds still under reduced motion", () => {
    expect(
      REDUCED.some((b) => b.includes(".qz-live-dot") && b.includes("animation: none")),
    ).toBe(true);
  });
});
