import { describe, expect, it } from "vitest";

import { cn } from "@/lib/utils";

describe("cn knows the deck's scales", () => {
  it("merges a deck font size with a stock one and keeps the text colour", () => {
    expect(cn("text-primary-foreground text-sm", "text-ui")).toBe(
      "text-primary-foreground text-ui",
    );
    expect(cn("text-ui", "text-sm")).toBe("text-sm");
  });

  it("merges deck radii and shadows with stock ones", () => {
    expect(cn("rounded-md", "rounded-ctl")).toBe("rounded-ctl");
    expect(cn("shadow-xs", "shadow-card")).toBe("shadow-card");
  });

  it("keeps a deck-scoped class beside the unscoped one", () => {
    expect(cn("h-9", "in-data-[scale=deck]:h-9.5")).toBe(
      "h-9 in-data-[scale=deck]:h-9.5",
    );
  });
});
