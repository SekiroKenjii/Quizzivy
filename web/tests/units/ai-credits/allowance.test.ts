import { describe, expect, it } from "vitest";
import { allowanceBody, baseAllowance } from "@/features/ai-credits/allowance";
import { bao, giau, linh } from "@tests/support/aiCredits";

const granted = { ...bao, allowance: 100000, granted: 20000 };

describe("a custom teacher's own figure", () => {
  it("leaves out the credits approved requests added on top", () => {
    expect(baseAllowance(granted)).toBe(80000);
    expect(baseAllowance(bao)).toBe(80000);
  });
});

describe("what the allowance dialog sends", () => {
  it("sends nothing when nothing changed", () => {
    expect(
      allowanceBody(giau, { mode: "default", credits: "50000", paused: false }),
    ).toBeNull();
    expect(
      allowanceBody(bao, { mode: "custom", credits: "80000", paused: false }),
    ).toBeNull();
    expect(
      allowanceBody(granted, { mode: "custom", credits: "80000", paused: false }),
    ).toBeNull();
  });

  it("sends a figure only when it moved, with the custom flag", () => {
    expect(
      allowanceBody(giau, { mode: "custom", credits: " 60000 ", paused: false }),
    ).toEqual({ custom: true, credits: 60000 });
    expect(
      allowanceBody(granted, { mode: "custom", credits: "90000", paused: false }),
    ).toEqual({ custom: true, credits: 90000 });
  });

  it("returns a custom teacher to the default with no figure", () => {
    expect(
      allowanceBody(bao, { mode: "default", credits: "80000", paused: false }),
    ).toEqual({ custom: false });
  });

  it("sends the pause alone, whichever way it moved, and leaves the figure out of it", () => {
    expect(
      allowanceBody(giau, { mode: "default", credits: "50000", paused: true }),
    ).toEqual({ paused: true });
    expect(
      allowanceBody(linh, { mode: "default", credits: "50000", paused: false }),
    ).toEqual({ paused: false });
    expect(
      allowanceBody(bao, { mode: "custom", credits: "80000", paused: true }),
    ).toEqual({ paused: true });
  });

  it("sends the figure and the pause together", () => {
    expect(
      allowanceBody(giau, { mode: "custom", credits: "1000", paused: true }),
    ).toEqual({ custom: true, credits: 1000, paused: true });
  });
});
