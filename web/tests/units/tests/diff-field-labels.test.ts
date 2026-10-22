import { describe, expect, it } from "vitest";
import en from "@/lib/i18n/locales/en.json";
import vi_ from "@/lib/i18n/locales/vi.json";
import { loadSpec } from "@tests/support/openapi";

const fields: string[] = loadSpec().components.schemas.DiffField.enum;
const labels = (locale: typeof vi_) =>
  locale.tests.detail.compare.field as Record<string, string>;

describe("version comparison field labels", () => {
  it("reads the contract's fields", () => {
    expect(fields).toContain("items");
  });

  it.each([
    ["vi", vi_],
    ["en", en],
  ])("names every DiffField in %s", (_, locale) => {
    const missing = fields.filter((field) => !labels(locale)[field]?.trim());
    expect(missing).toEqual([]);
  });
});
