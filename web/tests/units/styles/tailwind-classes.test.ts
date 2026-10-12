import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(import.meta.dirname, "../../../src");
const CSS = readFileSync(join(SRC, "index.css"), "utf8");

function sources(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sources(path);
    return /\.tsx?$/.test(name) ? [path] : [];
  });
}

function themeKeys(namespace: string): Set<string> {
  const pattern = new RegExp(
    String.raw`--${namespace}-([a-z0-9]+(?:-[a-z0-9]+)*)\s*:`,
    "g",
  );
  return new Set([...CSS.matchAll(pattern)].map((match) => match[1]!));
}

const COLOURS = themeKeys("color");
const SHADOWS = themeKeys("shadow");
const FONT_SIZES = themeKeys("text");

const PALETTE_NAMES =
  "zinc|slate|gray|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose";
const TAILWIND_COLOUR = new RegExp(
  `^(?:transparent|current|inherit|white|black|(?:${PALETTE_NAMES})-(?:50|[1-9]00|950))$`,
);

const LINE_STYLES = ["solid", "dashed", "dotted", "double", "hidden", "none"];
const POSITIONS = ["center", "top", "bottom", "left", "right"];

function anyOf(...alternatives: string[]): RegExp {
  return new RegExp(`^(?:${alternatives.join("|")})$`);
}

const NON_COLOUR: Record<string, RegExp> = {
  bg: anyOf(
    "none|auto|cover|contain|fixed|local|scroll|no-repeat|repeat",
    "(?:clip|origin|blend|repeat|size|position)-.+",
    "(?:gradient-to|linear|radial|conic)-.+",
    `(?:${POSITIONS.join("|")})(?:-(?:top|bottom|left|right))?`,
  ),
  text: anyOf(
    "xs|sm|base|lg|xl|[2-9]xl",
    "left|center|right|justify|start|end",
    "wrap|nowrap|balance|pretty|ellipsis|clip|shadow-.+",
  ),
  border: anyOf(
    "[xytrblse]",
    `\\d+|${LINE_STYLES.join("|")}`,
    "collapse|separate|spacing(?:-[xy])?-.+",
  ),
  ...Object.fromEntries(
    [..."xytrblse"].map((side) => [`border-${side}`, anyOf("\\d+")]),
  ),
  ring: anyOf("\\d+", "inset"),
  "ring-offset": anyOf("\\d+"),
  fill: anyOf("none"),
  stroke: anyOf("none", "\\d+"),
  shadow: anyOf("none|xs|sm|md|lg|xl|2xl|inner"),
  outline: anyOf("\\d+|offset-\\d+", "none|hidden|solid|dashed|dotted|double"),
  decoration: anyOf(
    "\\d+|auto|from-font|clone|slice",
    "solid|double|dotted|dashed|wavy",
  ),
  divide: anyOf("[xy](?:-\\d+|-reverse)?", LINE_STYLES.join("|")),
  from: anyOf("\\d+%"),
  via: anyOf("\\d+%"),
  to: anyOf("\\d+%"),
  placeholder: anyOf(""),
  caret: anyOf(""),
  accent: anyOf("auto"),
};

const CSS_WORDS = new Set([
  "text-align",
  "text-decoration",
  "text-decoration-line",
  "text-indent",
  "border-box",
]);

const PREFIXES = Object.keys(NON_COLOUR).sort((a, b) => b.length - a.length);

function withoutComments(text: string): string {
  return text.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(^|[\s;{(,])\/\/.*$/gm, "$1");
}

function collapseGroups(text: string): string {
  let out = text;
  for (let before = ""; before !== out;) {
    before = out;
    out = out.replace(/\[[^[\]\s"'`]*\]/g, "[]");
  }
  return out.replace(/-\([^()\s"'`]*\)/g, "-()");
}

function utilityOf(word: string): string {
  const last = word.slice(word.lastIndexOf(":") + 1);
  return last.replace(/^!|!$/g, "").replace(/\/[\w.%[\]-]*$/, "");
}

function deadColourClasses(text: string): string[] {
  const words = collapseGroups(withoutComments(text)).split(
    /[^A-Za-z0-9_\-:/.!%@#[\]()]+/,
  );
  const dead = new Set<string>();
  for (const word of words) {
    const utility = utilityOf(word);
    const prefix = PREFIXES.find((candidate) => utility.startsWith(`${candidate}-`));
    if (prefix === undefined || CSS_WORDS.has(utility)) continue;
    const rest = utility.slice(prefix.length + 1);
    if (rest === "" || rest.includes("[") || rest.includes("(")) continue;
    if (NON_COLOUR[prefix]!.test(rest)) continue;
    if (TAILWIND_COLOUR.test(rest) || COLOURS.has(rest)) continue;
    if (prefix === "shadow" && SHADOWS.has(rest)) continue;
    if (prefix === "text" && FONT_SIZES.has(rest)) continue;
    dead.add(utility);
  }
  return [...dead];
}

describe("the scan reads classes the way Tailwind does", () => {
  it("finds the class that has no theme colour", () => {
    expect(deadColourClasses('<div className="bg-accent-soft p-2" />')).toEqual([
      "bg-accent-soft",
    ]);
  });

  it("accepts theme colours under variants, opacity and the important marker", () => {
    const text = `cn("bg-brand-soft hover:bg-brand/50 dark:text-muted-fg", on && "!border-danger", "group-has-[:checked]/row:ring-focus max-[767px]:bg-card")`;
    expect(deadColourClasses(text)).toEqual([]);
  });

  it("finds a dead class inside a template literal, cva and clsx", () => {
    const text = [
      "const a = `flex ${gap} text-nope`;",
      'const b = cva("base", { variants: { tone: { x: "border-gone hover:fill-lost" } } });',
      'const c = clsx({ "stroke-missing": on });',
    ].join("\n");
    expect(new Set(deadColourClasses(text))).toEqual(
      new Set(["border-gone", "fill-lost", "stroke-missing", "text-nope"]),
    );
  });

  it("leaves non-colour utilities, arbitrary values and the theme's sizes alone", () => {
    const text = `"text-sm text-meta text-[13px] text-(length:--x) border-2 border-t-0 border-dashed ring-2 ring-inset shadow-none shadow-card outline-none bg-clip-text bg-[url(/a.svg)] from-10% divide-y"`;
    expect(deadColourClasses(text)).toEqual([]);
  });

  it("reads the side of a border before its colour", () => {
    const accepted = '"border-t-brand border-x-danger/40 border-b-0 border-s-card"';
    expect(deadColourClasses(accepted)).toEqual([]);
    expect(deadColourClasses('"border-l-gone"')).toEqual(["border-l-gone"]);
  });

  it("ignores prose in comments", () => {
    expect(deadColourClasses("// text-based\n/* bg-nothing */")).toEqual([]);
  });
});

describe("every colour class resolves to a theme token (spec §12)", () => {
  it("declares the theme colours it reads", () => {
    expect(COLOURS.has("brand-soft")).toBe(true);
    expect(COLOURS.has("accent-soft")).toBe(false);
    expect(SHADOWS.has("card")).toBe(true);
    expect(FONT_SIZES.has("meta")).toBe(true);
  });

  it("has no colour class that generates no CSS", () => {
    const offenders = sources(SRC).flatMap((path) => {
      const file = relative(SRC, path).replaceAll("\\", "/");
      return deadColourClasses(readFileSync(path, "utf8")).map(
        (name) => `${file}: ${name}`,
      );
    });
    expect(offenders).toEqual([]);
  });
});
