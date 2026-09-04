import { afterEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LanguageSection } from "@/features/auth/components/SettingsSections";
import i18n, { setLocale } from "@/lib/i18n";

afterEach(() => {
  setLocale("vi");
  localStorage.clear();
});

/** The choice has to outlive the tab: stored, and announced on <html lang>. */
describe("the language control", () => {
  it("persists the choice and updates the document language", async () => {
    const user = userEvent.setup();
    render(<LanguageSection />);

    await user.click(screen.getByRole("tab", { name: "English" }));

    expect(i18n.language).toBe("en");
    expect(localStorage.getItem("quizzivy.locale")).toBe("en");
    expect(document.documentElement.lang).toBe("en");
  });
});
