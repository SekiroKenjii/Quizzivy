import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const CSS = readFileSync(
  resolve(import.meta.dirname, "../../../src/index.css"),
  "utf8",
);

type Theme = "light" | "dark";
type Rgb = { r: number; g: number; b: number };

function block(selector: RegExp): Map<string, string> {
  const match = selector.exec(CSS);
  expect(match, `${selector} block`).not.toBeNull();
  let body = CSS.slice(
    match!.index + match![0].length,
    CSS.indexOf("\n}", match!.index),
  );
  for (let open = body.indexOf("/*"); open >= 0; open = body.indexOf("/*")) {
    body = body.slice(0, open) + body.slice(body.indexOf("*/", open) + 2);
  }
  const out = new Map<string, string>();
  for (const declaration of body.split(";")) {
    const colon = declaration.indexOf(":");
    const name = declaration.slice(0, colon).trim();
    if (colon > 0 && name.startsWith("--"))
      out.set(name, declaration.slice(colon + 1).trim());
  }
  return out;
}

const LIGHT = block(/^:root \{/m);
const DARK = block(/^:root\.dark \{/m);

function raw(theme: Theme, name: string): string {
  const value = (theme === "dark" ? DARK.get(name) : undefined) ?? LIGHT.get(name);
  expect(value, `${name} is defined`).toBeDefined();
  const alias = /^var\((--[a-z0-9-]+)\)$/.exec(value!);
  return alias ? raw(theme, alias[1]!) : value!;
}

const linear = (c: number) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
const gamma = (c: number) =>
  c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055;
const clamp = (c: number) => Math.min(1, Math.max(0, c));

function oklchToSrgb(l: number, c: number, h: number): Rgb {
  const a = c * Math.cos((h * Math.PI) / 180);
  const b = c * Math.sin((h * Math.PI) / 180);
  const l_ = (l + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const m_ = (l - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s_ = (l - 0.0894841775 * a - 1.291485548 * b) ** 3;
  return {
    r: clamp(gamma(4.0767416621 * l_ - 3.3077115913 * m_ + 0.2309699292 * s_)),
    g: clamp(gamma(-1.2684380046 * l_ + 2.6097574011 * m_ - 0.3413193965 * s_)),
    b: clamp(gamma(-0.0041960863 * l_ - 0.7034186147 * m_ + 1.707614701 * s_)),
  };
}

function srgbToOklch({ r, g, b }: Rgb): { l: number; c: number; h: number } {
  const [lr, lg, lb] = [linear(r), linear(g), linear(b)];
  const l = Math.cbrt(0.4122214708 * lr + 0.5363325363 * lg + 0.0514459929 * lb);
  const m = Math.cbrt(0.2119034982 * lr + 0.6806995451 * lg + 0.1073969566 * lb);
  const s = Math.cbrt(0.0883024619 * lr + 0.2817188376 * lg + 0.6299787005 * lb);
  const L = 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s;
  const A = 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s;
  const B = 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s;
  const h = (Math.atan2(B, A) * 180) / Math.PI;
  return { l: L, c: Math.hypot(A, B), h: h < 0 ? h + 360 : h };
}

function colour(theme: Theme, name: string): Rgb {
  const value = raw(theme, name);
  const hex = /^#([0-9a-f]{6})$/i.exec(value);
  if (hex) {
    const n = Number.parseInt(hex[1]!, 16);
    return { r: (n >> 16) / 255, g: ((n >> 8) & 255) / 255, b: (n & 255) / 255 };
  }
  const oklch = /^oklch\(([\d.]+) ([\d.]+) ([\d.]+)\)$/.exec(value);
  if (oklch) return oklchToSrgb(Number(oklch[1]), Number(oklch[2]), Number(oklch[3]));
  const rgba = /^rgba\((\d+), (\d+), (\d+), ([\d.]+)\)$/.exec(value);
  if (rgba) {
    const alpha = Number(rgba[4]);
    const under = colour(theme, "--bg");
    const mix = (c: string, u: number) => (Number(c) / 255) * alpha + u * (1 - alpha);
    return {
      r: mix(rgba[1]!, under.r),
      g: mix(rgba[2]!, under.g),
      b: mix(rgba[3]!, under.b),
    };
  }
  throw new Error(`${name} (${theme}): cannot parse ${value}`);
}

function contrast(theme: Theme, fg: string, bg: string): number {
  const lum = ({ r, g, b }: Rgb) =>
    0.2126 * linear(r) + 0.7152 * linear(g) + 0.0722 * linear(b);
  const [a, b] = [lum(colour(theme, fg)), lum(colour(theme, bg))].sort((x, y) => y - x);
  return (a! + 0.05) / (b! + 0.05);
}

const THEMES: Theme[] = ["light", "dark"];
const NEUTRALS = [
  "--bg",
  "--sidebar",
  "--card",
  "--muted",
  "--hover",
  "--fg",
  "--muted-fg",
  "--border",
  "--primary",
  "--primary-fg",
  "--ring",
];
const THEMED = [
  ...NEUTRALS,
  "--qz-shadow",
  "--qz-shadow-lg",
  "--accent-soft",
  "--accent-ink",
  "--overlay",
  "--input",
];
const SHARED_SOLIDS = ["--accent-c", "--success", "--warning", "--danger", "--info"];
const FAMILIES = ["success", "warning", "danger", "info"] as const;
const HUE: Record<string, [number, number]> = {
  "--accent-c": [110, 135],
  success: [140, 170],
  warning: [55, 90],
  danger: [15, 35],
  info: [220, 240],
};

describe("design tokens (spec §12)", () => {
  it("gives every themed token a dark value", () => {
    for (const name of [
      ...THEMED,
      ...FAMILIES.flatMap((f) => [`--${f}-soft`, `--${f}-ink`]),
    ]) {
      expect(DARK.has(name), `${name} has a dark value`).toBe(true);
    }
    for (const name of SHARED_SOLIDS) {
      expect(DARK.has(name), `${name} is shared by both themes`).toBe(false);
    }
  });

  it("keeps the neutrals neutral and the primary charcoal", () => {
    for (const theme of THEMES) {
      for (const name of NEUTRALS) {
        expect(
          srgbToOklch(colour(theme, name)).c,
          `${name} (${theme}) chroma`,
        ).toBeLessThan(0.03);
      }
    }
    expect(srgbToOklch(colour("light", "--primary")).l).toBeLessThan(0.35);
    expect(srgbToOklch(colour("dark", "--primary")).l).toBeGreaterThan(0.85);
  });

  it("keeps each colour family in its hue band, and blue for information only", () => {
    const inBand = (name: string, [lo, hi]: [number, number]) => {
      const { h } = srgbToOklch(colour("light", name));
      expect(h, `${name} hue`).toBeGreaterThanOrEqual(lo);
      expect(h, `${name} hue`).toBeLessThanOrEqual(hi);
    };
    inBand("--accent-c", HUE["--accent-c"]!);
    for (const family of FAMILIES) {
      for (const suffix of ["", "-soft", "-ink"])
        inBand(`--${family}${suffix}`, HUE[family]!);
    }
    for (const theme of THEMES) {
      const blue = [...LIGHT.keys()].filter((name) => {
        if (/shadow|overlay/.test(name)) return false;
        let rgb: Rgb;
        try {
          rgb = colour(theme, name);
        } catch {
          return false;
        }
        const { c, h } = srgbToOklch(rgb);
        return c > 0.05 && h >= 220 && h <= 340;
      });
      expect(
        blue.every((name) => name.startsWith("--info")),
        `${theme}: ${blue.join(", ")}`,
      ).toBe(true);
    }
  });

  it("keeps text and controls legible in both themes", () => {
    const text: [string, string][] = [
      ...["--bg", "--card", "--sidebar", "--muted"].flatMap(
        (bg): [string, string][] => [
          ["--fg", bg],
          ["--muted-fg", bg],
        ],
      ),
      ["--accent-ink", "--accent-soft"],
      ...FAMILIES.map((f): [string, string] => [`--${f}-ink`, `--${f}-soft`]),
      ["--primary-fg", "--primary"],
      ["--accent-fg", "--accent-c"],
      ["--danger-solid-fg", "--danger-solid"],
    ];
    const indicators: [string, string][] = [
      ...["--bg", "--card", "--sidebar", "--muted"].map((bg): [string, string] => [
        "--focus",
        bg,
      ]),
      ["--input", "--bg"],
      ["--input", "--card"],
    ];
    for (const theme of THEMES) {
      for (const [fg, bg] of text) {
        expect(
          contrast(theme, fg, bg),
          `${fg} on ${bg} (${theme})`,
        ).toBeGreaterThanOrEqual(4.5);
      }
      for (const [fg, bg] of indicators) {
        expect(
          contrast(theme, fg, bg),
          `${fg} on ${bg} (${theme})`,
        ).toBeGreaterThanOrEqual(3);
      }
    }
  });

  it("routes every utility colour through a variable", () => {
    const themeBlock = CSS.slice(CSS.indexOf("@theme inline"));
    const literals = [...themeBlock.matchAll(/--color-[a-z-]+:\s*(oklch|#|rgb)/gi)];
    expect(
      literals.map((m) => m[0]),
      "@theme must reference var(--token), never a literal colour",
    ).toEqual([]);
  });
});
