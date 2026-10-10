import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { ProfileSection } from "@/features/settings/sections/Profile";
import i18n, { setLocale } from "@/lib/i18n";
import en from "@/lib/i18n/locales/en.json";
import { useAuthStore } from "@/stores/auth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";

const BASE = "http://localhost:8080";

beforeEach(() => {
  useAuthStore.getState().setSession("token", { ...teacherUser, locale: "vi" });
});

afterEach(async () => {
  useAuthStore.getState().clearSession();
  i18n.addResourceBundle("en", "translation", en, true, true);
  await setLocale("vi");
  localStorage.clear();
});

function savesAs(locale: "vi" | "en") {
  const bodies: unknown[] = [];
  server.use(
    http.patch(`${BASE}/auth/me`, async ({ request }) => {
      bodies.push(await request.json());
      return HttpResponse.json({ ...teacherUser, locale });
    }),
  );
  return bodies;
}

/**
 * The language is a field of the teacher's Profile (DG-157): it applies once
 * saved, and the choice outlives the tab, stored where `boot.js` reads it
 * before the first paint and announced on <html lang>.
 */
describe("the Profile's language", () => {
  it("lists Tiếng Việt first and stays as it was until Save changes", async () => {
    const bodies = savesAs("en");
    const user = userEvent.setup();
    render(<ProfileSection />);

    const field = screen.getByRole("combobox", { name: "Ngôn ngữ" });
    expect(field).toHaveTextContent("Tiếng Việt");
    await user.click(field);
    const options = within(screen.getByRole("listbox")).getAllByRole("option");
    expect(options.map((option) => option.textContent)).toEqual([
      "Tiếng Việt",
      "English",
    ]);
    await user.click(screen.getByRole("option", { name: "English" }));

    expect(i18n.language).toBe("vi");
    expect(localStorage.getItem("quizzivy.locale")).not.toBe("en");
    expect(bodies).toEqual([]);
  });

  it("stores the saved language for the next boot and updates the document language", async () => {
    const bodies = savesAs("en");
    const user = userEvent.setup();
    render(<ProfileSection />);

    await user.click(screen.getByRole("combobox", { name: "Ngôn ngữ" }));
    await user.click(screen.getByRole("option", { name: "English" }));
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    await waitFor(() => expect(i18n.language).toBe("en"));
    expect(bodies).toEqual([{ locale: "en" }]);
    expect(localStorage.getItem("quizzivy.locale")).toBe("en");
    expect(document.documentElement.lang).toBe("en");
  });

  it("is reachable by keyboard", async () => {
    savesAs("en");
    const user = userEvent.setup();
    render(<ProfileSection />);

    const field = screen.getByRole("combobox", { name: "Ngôn ngữ" });
    field.focus();
    await user.keyboard("{Enter}");
    await user.keyboard("{ArrowDown}{Enter}");
    expect(field).toHaveTextContent("English");
  });

  it("switches once the English strings arrive when they were not loaded", async () => {
    i18n.removeResourceBundle("en", "translation");
    savesAs("en");
    const user = userEvent.setup();
    render(<ProfileSection />);

    await user.click(screen.getByRole("combobox", { name: "Ngôn ngữ" }));
    await user.click(screen.getByRole("option", { name: "English" }));
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));

    await waitFor(() => expect(i18n.language).toBe("en"));
    expect(i18n.hasResourceBundle("en", "translation")).toBe(true);
    expect(localStorage.getItem("quizzivy.locale")).toBe("en");
  });
});
