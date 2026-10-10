import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import {
  PASTE_EXAMPLE,
  normalizePaste,
  pasteLength,
  pastedTitle,
  scanPastedText,
} from "@/features/imports/paste";

interface CountCase {
  name: string;
  text: string;
  sections: number;
  questions: number;
  answered: number;
  missing: number[];
  duplicates?: number[];
}

const CASES = JSON.parse(
  readFileSync(
    resolve(import.meta.dirname, "../../../../api/testdata/pasted-text-counts.json"),
    "utf8",
  ),
) as CountCase[];

describe("the quick count of pasted text, against the server's fixture", () => {
  it("reads the whole shared corpus", () => {
    expect(CASES.length).toBeGreaterThanOrEqual(46);
  });

  it("shows as the page's example a case with an answer for every question", () => {
    const example = CASES.find((c) => c.name === "page-example");
    expect(example?.text).toBe(PASTE_EXAMPLE);
    expect(example?.missing).toEqual([]);
  });

  it.each(CASES.map((c) => [c.name, c] as const))("%s", (_name, c) => {
    const scan = scanPastedText(c.text);
    expect({
      sections: scan.sections,
      questions: scan.questions,
      answered: scan.answered,
      missing: scan.missing.map(Number),
      duplicates: scan.duplicates,
    }).toEqual({
      sections: c.sections,
      questions: c.questions,
      answered: c.answered,
      missing: c.missing,
      duplicates: c.duplicates ?? [],
    });
  });
});

describe("the pasted title", () => {
  const byName = (name: string) => CASES.find((c) => c.name === name)!.text;

  it("is the first line when it reads as a title", () => {
    expect(pastedTitle(byName("deck-sample"))).toBe("Unit 5 Quick Check");
  });

  it("skips blank lines before it", () => {
    expect(pastedTitle(`\n   \n${byName("deck-sample")}`)).toBe("Unit 5 Quick Check");
  });

  it.each(["deck-example", "bare-repeat", "keyword-Q1.", "nfd-section", "bai-tap"])(
    "is empty when the first line of %s is a part heading or a question",
    (name) => {
      expect(pastedTitle(byName(name))).toBe("");
    },
  );

  it("is empty for an option and for a line over 120 characters", () => {
    expect(pastedTitle("A. alpha\n1. First?")).toBe("");
    expect(pastedTitle(`${"x".repeat(121)}\n1. First?`)).toBe("");
    expect(pastedTitle(`${"x".repeat(120)}\n1. First?`)).toBe("x".repeat(120));
  });
});

const bom = String.fromCodePoint(0xfeff);
const decomposed = `Pha${String.fromCodePoint(0x302, 0x300)}n`;
const composed = `Ph${String.fromCodePoint(0x1ea7)}n`;

describe("the text the page sends", () => {
  it("is NFC with LF line ends and no leading BOM", () => {
    expect(decomposed).toHaveLength(6);
    expect(normalizePaste(`${bom}${decomposed} 1\r\n1. a\rb`)).toBe(
      `${composed} 1\n1. a\nb`,
    );
  });

  it("is counted in code points of its NFC form", () => {
    expect(pasteLength(decomposed)).toBe(4);
    expect(pasteLength(String.fromCodePoint(0x1f600))).toBe(1);
  });
});
