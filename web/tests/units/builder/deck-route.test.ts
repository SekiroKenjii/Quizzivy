import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";

describe("the rebuilt builder route", () => {
  it("opts only its actual leaf into the teacher shell with translated crumbs and the deck width", () => {
    const source = readFileSync(
      resolve(import.meta.dirname, "../../../src/app/router.tsx"),
      "utf8",
    );
    const leaf = source.slice(
      source.indexOf('path: "tests/:id/edit"'),
      source.indexOf('path: "imports"'),
    );
    expect(leaf).toContain('key: "teacherShell.nav.tests", to: "/teacher/tests"');
    expect(leaf).toContain('key: "builder.titleLabel"');
    expect(leaf).toContain("width: 1320");
    expect(leaf).toContain('import("@/features/tests/pages/teacher/TestBuilderPage")');
    expect(i18n.t("teacherShell.nav.tests")).toBe("Đề thi");
    expect(i18n.t("builder.titleLabel")).toBe("Tên đề thi");
  });
});
