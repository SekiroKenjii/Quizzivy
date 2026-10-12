import { describe, expect, it } from "vitest";
import type { RouteObject } from "react-router";
import i18n from "@/lib/i18n";
import { teacherHandleOf } from "@/layouts/shell/handle";
import type { WordImport } from "@/features/imports/api";
import { importName } from "@/features/imports/importName";

async function trails(): Promise<Map<string, string[]>> {
  const { router } = await import("@/app/router");
  const found = new Map<string, string[]>();
  const walk = (route: RouteObject, base: string) => {
    const path = route.index
      ? base
      : `/${[base, route.path].filter(Boolean).join("/")}`.replace(/\/+/g, "/");
    const handle = teacherHandleOf(route.handle);
    if (handle !== null && path.startsWith("/teacher/imports"))
      found.set(
        path,
        handle.crumb.map((crumb) => crumb.key),
      );
    for (const child of route.children ?? []) walk(child, path);
  };
  for (const route of router.routes) walk(route, "");
  return found;
}

function wordImport(sources: WordImport["sources"]): WordImport {
  return {
    id: "018f0000-0000-7000-8000-0000000000e1",
    title: "Unit 5 Quick Check",
    status: "needs_review",
    revision: 1,
    sourceRevision: 1,
    sources,
    pendingUploads: 0,
    createdBy: "018f0000-0000-7000-8000-0000000000e2",
    createdAt: "2026-10-10T01:00:00Z",
    updatedAt: "2026-10-10T01:00:00Z",
  } as WordImport;
}

function source(format: string, filename: string): WordImport["sources"][number] {
  return {
    id: "018f0000-0000-7000-8000-0000000000e3",
    role: "exam",
    filename,
    format,
    characters: 10,
    bytes: 10,
    sha256: "a".repeat(64),
    uploadedBy: "018f0000-0000-7000-8000-0000000000e2",
    createdAt: "2026-10-10T01:00:00Z",
  } as WordImport["sources"][number];
}

describe("the imports' breadcrumb trails", () => {
  it("follow the deck's: Tests › Imports, then the import or the new one", async () => {
    const tests = "teacherShell.nav.tests";
    const imports = "imports.historyTitle";
    expect(Object.fromEntries(await trails())).toEqual({
      "/teacher/imports": [tests, imports],
      "/teacher/imports/new": [tests, imports, "imports.newCrumb"],
      "/teacher/imports/:id": [tests, imports, "imports.detail.pageTitle"],
      "/teacher/imports/:id/review": [tests, imports, "imports.review.crumb"],
      "/teacher/imports/:id/confirm": [tests, imports, "imports.confirm.crumb"],
    });
  });

  it("name an import by its exam's file, a pasted one as pasted text", () => {
    const t = i18n.getFixedT("en");
    expect(
      importName(wordImport([source("docx", "Grade 9 Mock Test 3.docx")]), t),
    ).toBe("Grade 9 Mock Test 3.docx");
    expect(
      importName(wordImport([source("docx", "Đề 1.docx".normalize("NFD"))]), t),
    ).toBe("Đề 1.docx".normalize("NFC"));
    expect(importName(wordImport([source("text", "pasted-text.txt")]), t)).toBe(
      "Pasted text",
    );
    expect(importName(wordImport([]), t)).toBe("Unit 5 Quick Check");
  });
});
