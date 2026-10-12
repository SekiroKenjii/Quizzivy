import { afterEach, describe, expect, it, vi } from "vitest";

afterEach(() => {
  vi.doUnmock("@/lib/i18n/locales/en.json");
  vi.resetModules();
  localStorage.clear();
  document.documentElement.lang = "vi";
});

async function startWithoutEnglish() {
  vi.resetModules();
  vi.doMock("@/lib/i18n/locales/en.json", () => {
    throw new Error("the English chunk is unreachable");
  });
  return import("@/lib/i18n");
}

describe("when the English strings cannot be fetched", () => {
  it("a start with English stored renders in Vietnamese and keeps the choice for the next load", async () => {
    localStorage.setItem("quizzivy.locale", "en");
    document.documentElement.lang = "en";
    const { default: i18n, localeReady, chosenLocale } = await startWithoutEnglish();

    await localeReady;

    expect(i18n.language).toBe("vi");
    expect(document.documentElement.lang).toBe("vi");
    expect(chosenLocale()).toBe("vi");
    expect(i18n.t("common.cancel")).toBe("Huỷ");
    expect(localStorage.getItem("quizzivy.locale")).toBe("en");
  });

  it("a pick of English shows as chosen while it loads, then gives the choice back", async () => {
    const { default: i18n, setLocale, chosenLocale } = await startWithoutEnglish();

    const switching = setLocale("en");
    expect(chosenLocale()).toBe("en");
    await switching;

    expect(i18n.language).toBe("vi");
    expect(chosenLocale()).toBe("vi");
  });
});
