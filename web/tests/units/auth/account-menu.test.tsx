import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { AccountMenu } from "@/features/auth/AccountMenu";
import { DeckScale } from "@/components/ui/deck-scale";
import { writeThemePreference } from "@/lib/theme";
import { useAuthStore } from "@/stores/auth";
import { STUDENT } from "../student/support";
import "@/lib/i18n";

function renderMenu(deck: boolean) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <DeckScale>
          <AccountMenu settingsTo="/app/settings" deck={deck} />
        </DeckScale>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

const trigger = () =>
  screen.getByRole("button", { name: "Tài khoản của Nguyễn Văn An" });
const itemNames = () =>
  within(screen.getByRole("menu"))
    .getAllByRole("menuitem")
    .map((item) => item.textContent);

beforeEach(() => {
  useAuthStore.setState({ user: STUDENT });
  writeThemePreference("light");
});

afterEach(() => {
  writeThemePreference("light");
  localStorage.clear();
});

describe("the account menu on a console built to the deck", () => {
  it("shows who is signed in, then Settings, the theme and Sign out, in that order", async () => {
    const user = renderMenu(true);
    await user.click(trigger());

    const menu = screen.getByRole("menu");
    expect(within(menu).getByText("Nguyễn Văn An")).toBeInTheDocument();
    expect(within(menu).getByText("an@example.com")).toBeInTheDocument();
    expect(itemNames()).toEqual(["Cài đặt", "Chế độ tối", "Đăng xuất"]);
    expect(within(menu).getByRole("menuitem", { name: "Cài đặt" })).toHaveAttribute(
      "href",
      "/app/settings",
    );
    expect(menu.dataset["scale"]).toBe("deck");
  });

  it("opens from the keyboard, moves through its items and gives focus back on Escape", async () => {
    const user = renderMenu(true);
    trigger().focus();
    await user.keyboard("{Enter}");

    const items = within(screen.getByRole("menu")).getAllByRole("menuitem");
    expect(items[0]).toHaveFocus();
    await user.keyboard("{ArrowDown}");
    expect(items[1]).toHaveFocus();
    await user.keyboard("{ArrowDown}");
    expect(items[2]).toHaveFocus();
    await user.keyboard("{Home}");
    expect(items[0]).toHaveFocus();

    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).toBeNull();
    expect(trigger()).toHaveFocus();
  });

  it("switches to the dark theme and back, remembering the choice", async () => {
    const user = renderMenu(true);
    await user.click(trigger());
    await user.click(screen.getByRole("menuitem", { name: "Chế độ tối" }));
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(localStorage.getItem("quizzivy.theme")).toBe("dark");

    await user.click(trigger());
    expect(itemNames()).toEqual(["Cài đặt", "Chế độ sáng", "Đăng xuất"]);
    await user.click(screen.getByRole("menuitem", { name: "Chế độ sáng" }));
    expect(document.documentElement.classList.contains("dark")).toBe(false);
    expect(localStorage.getItem("quizzivy.theme")).toBe("light");
  });

  it("leaves the system setting for an explicit theme when the user picks one", async () => {
    writeThemePreference("system");
    const user = renderMenu(true);
    await user.click(trigger());
    await user.click(screen.getByRole("menuitem", { name: "Chế độ tối" }));
    expect(localStorage.getItem("quizzivy.theme")).toBe("dark");
  });
});

describe("the account menu on a console not yet rebuilt", () => {
  it("offers Settings and Sign out and no theme", async () => {
    const user = renderMenu(false);
    await user.click(trigger());
    expect(itemNames()).toEqual(["Cài đặt", "Đăng xuất"]);
  });
});
