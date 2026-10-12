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
  it("is one token, 1600px", () => {
    expect(read("index.css")).toMatch(/--container-page:\s*100rem;/);
  });

  it("lets no route choose its own width", () => {
    expect(read("app/router.tsx")).not.toMatch(/\bwidth:/);
    expect(read("layouts/shell/handle.ts")).not.toMatch(/\bwidth\??:/);
  });

  it("draws the splash's page block at the frame's width", () => {
    expect(read("layouts/TeacherSkeleton.tsx")).toContain("var(--container-page)");
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
