import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Guards spec §12's colour rules mechanically.
 *
 * §12 is unusually specific: "Primary action: dark charcoal (zinc-900) buttons,
 * white text. Not blue, not purple, not indigo." That is easy to honour on day
 * one and easy to lose later, when someone reaches for a familiar accent.
 *
 * The subtlety is that zinc is not hue-neutral — Tailwind's zinc sits around
 * hue 286, squarely in the blue/purple band — so a naive hue check would reject
 * the very palette the spec mandates. What separates zinc from indigo is
 * CHROMA: zinc's is <= 0.017, an actual indigo is ~0.2. So the rule is
 * "saturated AND in the blue band", not "in the blue band".
 */

const CSS = readFileSync(
  resolve(import.meta.dirname, "../../../src/index.css"),
  "utf8",
);

type Token = { name: string; l: number; c: number; h: number; raw: string };

function parseTokens(css: string): Token[] {
  const out: Token[] = [];
  const re = /(--[a-z0-9-]+)\s*:\s*oklch\(\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*\)/gi;
  for (const m of css.matchAll(re)) {
    out.push({
      name: m[1]!,
      l: Number(m[2]),
      c: Number(m[3]),
      h: Number(m[4]),
      raw: m[0],
    });
  }
  return out;
}

/** Saturated enough to read as a colour rather than a grey. */
const CHROMA_IS_A_COLOUR = 0.05;
/** Blue / indigo / violet / purple, in oklch hue degrees. */
const FORBIDDEN_HUE = { min: 220, max: 340 };

const tokens = parseTokens(CSS);

describe("§12 design tokens", () => {
  it("defines tokens at all (guards against a silent parse failure)", () => {
    expect(tokens.length).toBeGreaterThan(15);
  });

  it("has no blue, indigo or purple token", () => {
    const offenders = tokens.filter(
      (t) =>
        t.c > CHROMA_IS_A_COLOUR &&
        t.h >= FORBIDDEN_HUE.min &&
        t.h <= FORBIDDEN_HUE.max,
    );
    expect(
      offenders.map((t) => `${t.name} (${t.raw})`),
      "§12: primary is charcoal — not blue, not purple, not indigo",
    ).toEqual([]);
  });

  it("keeps the primary action a near-neutral charcoal", () => {
    const primary = tokens.find((t) => t.name === "--primary");
    expect(primary, "--primary must be defined").toBeDefined();
    // zinc-900 territory: very dark, essentially unsaturated.
    expect(primary!.l).toBeLessThan(0.35);
    expect(primary!.c).toBeLessThan(0.02);
  });

  it("keeps semantic colours to green, red and amber only", () => {
    const semantic = ["--success", "--destructive", "--warning"] as const;
    const bands: Record<(typeof semantic)[number], [number, number]> = {
      "--success": [120, 180], // green
      "--destructive": [0, 60], // red
      "--warning": [40, 110], // amber
    };
    for (const name of semantic) {
      const t = tokens.find((x) => x.name === name);
      expect(t, `${name} must be defined`).toBeDefined();
      const [lo, hi] = bands[name];
      expect(t!.h, `${name} sits outside its meaning band`).toBeGreaterThanOrEqual(lo);
      expect(t!.h, `${name} sits outside its meaning band`).toBeLessThanOrEqual(hi);
      expect(
        t!.c,
        `${name} should be saturated enough to read as a signal`,
      ).toBeGreaterThan(CHROMA_IS_A_COLOUR);
    }
  });

  it("routes every colour through a variable so dark mode stays addable", () => {
    const themeBlock = CSS.slice(CSS.indexOf("@theme inline"));
    const literals = [...themeBlock.matchAll(/--color-[a-z-]+:\s*(oklch|#|rgb)/gi)];
    expect(
      literals.map((m) => m[0]),
      "@theme must reference var(--token), never a literal colour",
    ).toEqual([]);
  });
});
