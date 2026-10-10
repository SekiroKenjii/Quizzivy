import { afterEach, beforeEach, describe, expect, it } from "vitest";
import i18n, { setLocale } from "@/lib/i18n";
import en from "@/lib/i18n/locales/en.json";

beforeEach(async () => {
  await i18n.changeLanguage("vi");
  i18n.removeResourceBundle("en", "translation");
});

afterEach(async () => {
  i18n.addResourceBundle("en", "translation", en, true, true);
  await setLocale("vi");
  localStorage.removeItem("quizzivy.locale");
});

describe("the English strings", () => {
  it("are not bundled: Vietnamese is the only language loaded at start", () => {
    expect(i18n.hasResourceBundle("vi", "translation")).toBe(true);
    expect(i18n.hasResourceBundle("en", "translation")).toBe(false);
  });

  it("load on the switch to English, which then translates", async () => {
    const switching = setLocale("en");
    expect(i18n.language).toBe("vi");
    expect(i18n.t("common.cancel")).toBe("Huỷ");

    await switching;

    expect(i18n.hasResourceBundle("en", "translation")).toBe(true);
    expect(i18n.language).toBe("en");
    expect(document.documentElement.lang).toBe("en");
    expect(i18n.t("common.cancel")).toBe("Cancel");
    expect(localStorage.getItem("quizzivy.locale")).toBe("en");
  });

  it("give way to a later choice made while they load", async () => {
    const english = setLocale("en");
    const vietnamese = setLocale("vi");
    await Promise.all([english, vietnamese]);

    expect(i18n.language).toBe("vi");
    expect(document.documentElement.lang).toBe("vi");
    expect(localStorage.getItem("quizzivy.locale")).toBe("vi");
  });

  it("switch at once once loaded", async () => {
    await setLocale("en");
    await setLocale("vi");

    void setLocale("en");

    expect(i18n.language).toBe("en");
  });
});
