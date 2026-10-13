import { readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { createRef } from "react";
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { StickyAside } from "@/components/shared/StickyAside";

const SRC = resolve(import.meta.dirname, "../../../src");
const read = (path: string) => readFileSync(join(SRC, path), "utf8");
const ALLOWANCE = ["-mx-0.5", "-mt-px", "-mb-0.5", "px-0.5", "pt-px", "pb-0.5"];

describe("StickyAside (V-02)", () => {
  it("is an aside that keeps the deck's focus-ring allowance and is static undocked", () => {
    render(<StickyAside aria-label="Tóm tắt">nội dung</StickyAside>);
    const column = screen.getByRole("complementary", { name: "Tóm tắt" });

    expect(column).toHaveClass(...ALLOWANCE);
    expect(column).not.toHaveClass("sticky-column");
    expect(column).toHaveTextContent("nội dung");
  });

  it("sticks while docked", () => {
    render(
      <StickyAside aria-label="Tóm tắt" docked>
        nội dung
      </StickyAside>,
    );

    expect(screen.getByRole("complementary", { name: "Tóm tắt" })).toHaveClass(
      "sticky-column",
      ...ALLOWANCE,
    );
  });

  it("leaves docking to a host's container query, and renders the element asked for", () => {
    const ref = createRef<HTMLElement>();
    render(
      <StickyAside
        as="nav"
        ref={ref}
        aria-label="Cài đặt"
        className="@min-[768px]/settings:sticky-column flex"
      >
        liên kết
      </StickyAside>,
    );
    const column = screen.getByRole("navigation", { name: "Cài đặt" });

    expect(ref.current).toBe(column);
    expect(column).toHaveClass("flex", "@min-[768px]/settings:sticky-column");
    expect(column).not.toHaveClass("sticky-column");
  });

  it("docks the deck's way: 16px from the top, capped at the window less 88px, scrolling", () => {
    const css = read("index.css");
    const utility = /@utility sticky-column \{([^}]*)\}/.exec(css)?.[1] ?? "";

    expect(utility).toMatch(/position:\s*sticky;/);
    expect(utility).toMatch(/top:\s*16px;/);
    expect(utility).toMatch(/max-height:\s*calc\(100vh - 88px\);/);
    expect(utility).toMatch(/overflow-y:\s*auto;/);
  });
});

describe("the side columns' thresholds, by their own container's width", () => {
  it.each([
    [
      "the wizard's summary from 794px of the wizard",
      "features/assignments/components/wizard/WizardAside.tsx",
      "@min-[794px]/wizard:sticky-column",
    ],
    [
      "the question editor's settings from 900px of the editor",
      "features/question-bank/pages/teacher/QuestionEditorPage.tsx",
      "@min-[900px]/qe:sticky-column",
    ],
    [
      "the import's summary from 1000px of Preview and create",
      "features/imports/pages/teacher/ImportConfirmPage.tsx",
      "@min-[1000px]/confirm:sticky-column",
    ],
    [
      "the settings navigation from 768px of the settings layout",
      "components/shared/SettingsLayout.tsx",
      "@min-[768px]/settings:sticky-column",
    ],
    [
      "Class detail's column whenever it is beside the members (1000px of content)",
      "features/classes/pages/teacher/ClassDetailPage.tsx",
      "docked={two}",
    ],
    [
      "the builder's outline whenever the builder splits",
      "features/tests/pages/teacher/TestBuilderPage.tsx",
      "docked={split}",
    ],
  ])("docks %s", (_, path, docking) => {
    const source = read(path);

    expect(source).toContain("<StickyAside");
    expect(source).toContain(docking);
  });

  it("keeps the version history's own sticky card and puts the bank's filters 16px down", () => {
    expect(read("features/tests/pages/teacher/TestDetailPage.tsx")).toMatch(
      /id="versions"[^>]*className="[^"]*\bsticky-column\b/s,
    );
    expect(read("features/question-bank/pages/teacher/QuestionBankPage.tsx")).toMatch(
      /aria-label=\{t\("bank\.filters"\)\} className="sticky top-4 /,
    );
  });
});
