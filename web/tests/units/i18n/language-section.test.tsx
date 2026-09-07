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

    await user.click(screen.getByRole("button", { name: "English" }));

    expect(i18n.language).toBe("en");
    expect(localStorage.getItem("quizzivy.locale")).toBe("en");
    expect(document.documentElement.lang).toBe("en");
  });

  /**
   * It is a group of buttons, not a tab strip: Radix Tabs with no panel to
   * control emitted a dangling aria-controls and left every trigger at
   * tabindex="-1", which put the switch out of reach of the keyboard.
   */
  it("is reachable by keyboard and says which language is on", async () => {
    const user = userEvent.setup();
    render(<LanguageSection />);

    const vi = screen.getByRole("button", { name: "Tiếng Việt" });
    const en = screen.getByRole("button", { name: "English" });
    expect(vi).toHaveAttribute("aria-pressed", "true");
    expect(en).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryByRole("tab")).toBeNull();

    await user.tab();
    expect(vi).toHaveFocus();
    await user.tab();
    expect(en).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(i18n.language).toBe("en");
  });
});
