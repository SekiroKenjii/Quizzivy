import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router";
import { DeckScale } from "@/components/ui/deck-scale";
import { AccountMenu } from "@/features/auth/AccountMenu";
import { setLocale } from "@/lib/i18n";
import type { components } from "@/lib/api/schema";
import { writeThemePreference } from "@/lib/theme";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { adminUser, studentUser, teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import "@/lib/i18n";

type User = components["schemas"]["CurrentUser"];
type Console = "teacher" | "admin";

const BASE = "http://localhost:8080";

const auditor: User = {
  ...studentUser,
  permissions: ["system.audit.read"],
  workspaces: ["admin"],
};
const operator: User = {
  ...auditor,
  permissions: ["system.settings.write", "system.api_reference", "people.users.manage"],
};

function renderMenu(user: User, workspace: Console) {
  useAuthStore.setState({ user });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <DeckScale>
          <AccountMenu workspace={workspace} />
        </DeckScale>
      </MemoryRouter>
    </QueryClientProvider>,
  );
  return userEvent.setup();
}

const trigger = (user: User) =>
  screen.getByRole("button", { name: `Tài khoản của ${user.fullName}` });
const itemNames = () =>
  within(screen.getByRole("menu"))
    .getAllByRole("menuitem")
    .map((item) => item.textContent);
const item = (name: string) => screen.getByRole("menuitem", { name });

let patches: unknown[] = [];

function recordPreferences() {
  patches = [];
  server.use(
    http.patch(`${BASE}/me/preferences`, async ({ request }) => {
      const patch = (await request.json()) as Record<string, unknown>;
      patches.push(patch);
      return HttpResponse.json(patch);
    }),
  );
}

const workspaces = () =>
  patches.filter((patch) => patch && typeof patch === "object" && "workspace" in patch);

beforeEach(() => {
  recordPreferences();
  writeThemePreference("light");
});

afterEach(async () => {
  writeThemePreference("light");
  localStorage.clear();
  useAuthStore.getState().clearSession();
  await setLocale("vi");
});

describe("the account menu in the teacher console", () => {
  it("is Settings, the theme and Sign out for a teacher with no other console", async () => {
    const user = renderMenu(teacherUser, "teacher");
    await user.click(trigger(teacherUser));

    expect(itemNames()).toEqual(["Cài đặt", "Chế độ tối", "Đăng xuất"]);
  });

  it("adds the Admin console, as a link out, for a user who holds it", async () => {
    const user = renderMenu(adminUser, "teacher");
    await user.click(trigger(adminUser));

    expect(itemNames()).toEqual([
      "Cài đặt",
      "Bảng quản trị",
      "Chế độ tối",
      "Đăng xuất",
    ]);
    expect(item("Cài đặt")).toHaveAttribute("href", "/teacher/settings");
    expect(item("Bảng quản trị")).toHaveAttribute("href", "/admin");
    expect(
      item("Bảng quản trị").querySelector("svg.lucide-arrow-up-right"),
    ).not.toBeNull();

    await user.click(item("Bảng quản trị"));
    await waitFor(() => expect(workspaces()).toEqual([{ workspace: "admin" }]));
  });
});

describe("the account menu in the Admin console", () => {
  it("lists the pages an Admin may open, then the teacher workspace, the theme and Sign out", async () => {
    const user = renderMenu(adminUser, "admin");
    await user.click(trigger(adminUser));

    expect(itemNames()).toEqual([
      "Cài đặt hệ thống",
      "Tài liệu API",
      "Không gian giáo viên",
      "Chế độ tối",
      "Đăng xuất",
    ]);
    expect(item("Cài đặt hệ thống")).toHaveAttribute("href", "/admin/settings");
    expect(item("Tài liệu API")).toHaveAttribute("href", "/admin/api");
    expect(item("Không gian giáo viên")).toHaveAttribute("href", "/teacher");
    expect(
      item("Không gian giáo viên").querySelector("svg.lucide-arrow-up-right"),
    ).not.toBeNull();

    await user.click(item("Không gian giáo viên"));
    await waitFor(() => expect(workspaces()).toEqual([{ workspace: "teacher" }]));
  });

  it("gives a user with no teacher workspace Change password and Language, after the pages they may open", async () => {
    const user = renderMenu(operator, "admin");
    await user.click(trigger(operator));

    expect(itemNames()).toEqual([
      "Cài đặt hệ thống",
      "Tài liệu API",
      "Đổi mật khẩu",
      "Ngôn ngữ" + "Tiếng Việt",
      "Chế độ tối",
      "Đăng xuất",
    ]);
    expect(item("Đổi mật khẩu")).toHaveAttribute("href", "/change-password");
  });

  it("leaves out the pages a role may not open", async () => {
    const user = renderMenu(auditor, "admin");
    await user.click(trigger(auditor));

    expect(itemNames()).toEqual([
      "Đổi mật khẩu",
      "Ngôn ngữ" + "Tiếng Việt",
      "Chế độ tối",
      "Đăng xuất",
    ]);
  });

  it("leaves out Change password where the account has no password to change", async () => {
    const googleOnly: User = { ...auditor, hasPassword: false };
    const user = renderMenu(googleOnly, "admin");
    await user.click(trigger(googleOnly));

    expect(itemNames()).toEqual(["Ngôn ngữ" + "Tiếng Việt", "Chế độ tối", "Đăng xuất"]);
  });

  it("writes the language through the profile and switches the words at once", async () => {
    const bodies: unknown[] = [];
    server.use(
      http.patch(`${BASE}/auth/me`, async ({ request }) => {
        const body = (await request.json()) as { locale: string };
        bodies.push(body);
        return contractJson("/auth/me", "patch", 200, {
          ...auditor,
          locale: body.locale,
        });
      }),
    );
    const user = renderMenu(auditor, "admin");
    await user.click(trigger(auditor));
    await user.click(screen.getByRole("menuitem", { name: /^Ngôn ngữ/ }));

    await waitFor(() => expect(bodies.at(-1)).toEqual({ locale: "en" }));
    await waitFor(() => expect(document.documentElement.lang).toBe("en"));
    await user.click(screen.getByRole("button", { name: /Nguyễn Văn An/ }));
    expect(itemNames()).toEqual([
      "Change password",
      "LanguageEnglish",
      "Dark mode",
      "Sign out",
    ]);
  });

  it("keeps Language and Change password out of the menu of a user who holds the teacher workspace", async () => {
    const user = renderMenu(adminUser, "admin");
    await user.click(trigger(adminUser));

    expect(screen.queryByRole("menuitem", { name: "Đổi mật khẩu" })).toBeNull();
    expect(screen.queryByRole("menuitem", { name: /^Ngôn ngữ/ })).toBeNull();
  });
});
