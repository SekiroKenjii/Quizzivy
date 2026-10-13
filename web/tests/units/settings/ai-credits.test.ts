import { describe, expect, it } from "vitest";
import { credits, resetsOn } from "@/features/ai/credits";

describe("the usage card's words", () => {
  it("names the first day of the next month as the deck writes it", () => {
    expect(resetsOn("2026-09-01", "en")).toBe("1 October");
    expect(resetsOn("2026-10-01", "vi")).toBe("1 tháng 11");
    expect(resetsOn("2026-12-01", "en")).toBe("1 January");
  });

  it("groups credits as the locale does", () => {
    expect(credits(20000, "en")).toBe("20,000");
    expect(credits(20000, "vi")).toBe("20.000");
  });
});
