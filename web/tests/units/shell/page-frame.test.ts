import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = resolve(import.meta.dirname, "../../../src");
const read = (path: string) => readFileSync(join(SRC, path), "utf8");

function teacherPages(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return teacherPages(path);
    return /\.tsx?$/.test(name) && /[/\\]pages[/\\]teacher[/\\]/.test(path)
      ? [path]
      : [];
  });
}

const STRING = /(["'`])((?:(?!\1)[^\\]|\\.)*)\1/g;
const NARROW = /(?:^|\s)max-w-(?!full(?:\s|$))\S+/;

function centredNarrowBoxes(source: string): string[] {
  const found: string[] = [];
  for (const [, , text = ""] of source.matchAll(STRING)) {
    if (/(?:^|\s)mx-auto(?:\s|$)/.test(text) && NARROW.test(text)) found.push(text);
  }
  return found;
}

describe("the teacher workspace's page frame (DG-205)", () => {
  it("is the design's tokens: 1600px, gutters 14, 28 and 40, fields 480px, prose 72ch", () => {
    const css = read("index.css");
    expect(css).toMatch(/--page-max:\s*1600px;/);
    expect(css).toMatch(/--page-gutter:\s*14px;/);
    expect(css).toMatch(
      /@media \(min-width: 768px\) \{\s*:root \{\s*--page-gutter:\s*28px;/,
    );
    expect(css).toMatch(
      /@media \(min-width: 1920px\) \{\s*:root \{\s*--page-gutter:\s*40px;/,
    );
    expect(css).toMatch(/--field-max:\s*480px;/);
    expect(css).toMatch(/--prose-max:\s*72ch;/);
  });

  it("lets one route, the import review, lift the maximum, and no route choose a width", () => {
    const router = read("app/router.tsx");
    const teacher = router.slice(
      router.indexOf("const teacherTree"),
      router.indexOf("const adminTree"),
    );
    expect(teacher.match(/\bwidth:[^,}]*/g)).toEqual(['width: "full"']);
    const review = router.slice(router.indexOf('path: ":id/review"'));
    expect(review.slice(0, review.indexOf("lazy:"))).toContain('width: "full"');
  });

  it("gives the Admin console's routes the deck's two widths and no other", () => {
    const router = read("app/router.tsx");
    const admin = router.slice(router.indexOf("const adminTree"));
    const widths = new Set(admin.match(/\bwidth:[^,}]*/g));
    expect([...widths].sort((a, b) => a.localeCompare(b))).toEqual([
      "width: 1080",
      "width: 1320",
    ]);
  });

  it("draws the splash's page block at the frame's width and gutters", () => {
    const skeleton = read("layouts/TeacherSkeleton.tsx");
    expect(skeleton).toContain("var(--page-max)");
    expect(skeleton).toContain("px-(--page-gutter)");
  });

  it("finds the teacher pages it scans", () => {
    expect(teacherPages(join(SRC, "features")).length).toBeGreaterThan(20);
  });

  it("never centres a teacher page, or one of its states, in a narrower box", () => {
    const offenders = teacherPages(join(SRC, "features")).flatMap((path) =>
      centredNarrowBoxes(readFileSync(path, "utf8")).map(
        (text) => `${relative(SRC, path)}: ${text}`,
      ),
    );
    expect(offenders).toEqual([]);
  });

  it("would catch a page that does", () => {
    expect(
      centredNarrowBoxes(
        'const ROOT = "mx-auto flex w-full max-w-[1080px] flex-col";\n<div className="mx-auto max-w-full" />',
      ),
    ).toEqual(["mx-auto flex w-full max-w-[1080px] flex-col"]);
  });
});
