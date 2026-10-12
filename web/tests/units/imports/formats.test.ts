import { afterAll, describe, expect, it } from "vitest";
import i18n from "@/lib/i18n";
import { listFormats, orderFormats } from "@/features/imports/formats";

const t = i18n.t.bind(i18n);

afterAll(async () => {
  await i18n.changeLanguage("vi");
});

describe("the formats the server accepts (QA-51-13)", () => {
  it("keeps the deck's order whatever order the server sends", () => {
    expect(orderFormats(["docx", "pdf", "doc"])).toEqual(["docx", "doc", "pdf"]);
    expect(orderFormats(["pdf", "docx"])).toEqual(["docx", "pdf"]);
    expect(orderFormats(["odt", "pdf", "rtf", "docx"])).toEqual([
      "docx",
      "pdf",
      "odt",
      "rtf",
    ]);
  });

  it("writes the deck's English, with no comma before the last word", async () => {
    await i18n.changeLanguage("en");
    expect(listFormats(["docx", "pdf", "doc"], "name", "or", t)).toBe(
      "DOCX, DOC or PDF",
    );
    expect(listFormats(["docx", "pdf", "doc"], "extension", "and", t)).toBe(
      ".docx, .doc and .pdf",
    );
    expect(listFormats(["docx", "pdf"], "name", "or", t)).toBe("DOCX or PDF");
    expect(listFormats(["docx"], "extension", "or", t)).toBe(".docx");
  });

  it("writes Vietnamese with hoặc and và", async () => {
    await i18n.changeLanguage("vi");
    expect(listFormats(["docx", "pdf", "doc"], "name", "or", t)).toBe(
      "DOCX, DOC hoặc PDF",
    );
    expect(listFormats(["docx", "pdf", "doc"], "extension", "and", t)).toBe(
      ".docx, .doc và .pdf",
    );
  });
});
