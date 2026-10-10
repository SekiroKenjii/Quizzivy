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

// Each prefix lists the suffixes that name a utility other than a colour.
// Anything else after the prefix has to be a colour (or, for shadow, a shadow
// size) that index.css's @theme or Tailwind itself declares.
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
    `[xytrblse](?:-(?:\\d+|${LINE_STYLES.join("|")}))?`,
    `\\d+|${LINE_STYLES.join("|")}`,
    "collapse|separate|spacing(?:-[xy])?-.+",
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

// CSS property names and values that begin with a prefix but are not classes.
const CSS_WORDS = new Set([
  "text-align",
  "text-decoration",
  "text-decoration-line",
  "text-indent",
  "border-box",
]);

// Longest first, so that border-x- is read before border-, not after it.
const PREFIXES = Object.keys(NON_COLOUR).sort((a, b) => b.length - a.length);

function withoutComments(text: string): string {
  return text.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(^|[\s;{(,])\/\/.*$/gm, "$1");
}

// Collapses [..] and (..) groups, which hold arbitrary values and variant
// arguments, so that a colon or a slash inside one does not split a word.
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

/**
 * deadColourClasses reads every class-shaped word in a TypeScript source, with
 * variants, the opacity suffix and the important marker removed, and returns
 * the colour utilities that name no theme colour, no Tailwind default and no
 * non-colour utility of their prefix. Tailwind generates no CSS for such a
 * class, so it does nothing.
 *
 * The scan is a token regex over the file, not a parse: it does not know a
 * class from a string that happens to look like one, so it sees the same text
 * in a template literal, cn(...), cva(...) and clsx(...). A word with a
 * bracket or parenthesis after the prefix is an arbitrary value and is skipped.
 */
export function deadColourClasses(text: string): string[] {
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
