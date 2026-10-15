import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import { router as appRouter } from "@/app/router";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import { adminUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import {
  BASE,
  SAVED,
  failure,
  servesImpact,
  servesSettings,
  signInAsAdmin,
} from "./support";
import { renderSettings } from "./render";
import "@/lib/i18n";

beforeEach(() => {
  contentWidth(1080);
  signInAsAdmin();
  servesImpact();
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

const nav = () =>
  within(screen.getByRole("navigation", { name: "Mục cài đặt hệ thống" }));
const name = () => screen.findByRole("textbox", { name: "Tên tổ chức" });

describe("System settings", () => {
  it("heads the page as the plan words it and lists the two sections that have shipped", async () => {
    servesSettings();
    renderSettings();

    expect(
      screen.getByRole("heading", { level: 1, name: "Cài đặt hệ thống" }),
    ).toBeVisible();
    expect(
      await screen.findByText("Áp dụng cho mọi người ở Quizzivy English Centre."),
    ).toBeVisible();
    expect(
      nav()
        .getAllByRole("link")
        .map((link) => link.textContent),
    ).toEqual(["Tổ chức", "Đăng nhập"]);
    expect(nav().getByRole("link", { name: "Tổ chức" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(await name()).toHaveValue("Quizzivy English Centre");
  });

  it("holds a skeleton in the section while the document is read", async () => {
    server.use(
      http.get(`${BASE}/admin/settings`, async () => {
        await new Promise((resolve) => setTimeout(resolve, 150));
        return contractJson("/admin/settings", "get", 200, SAVED);
      }),
    );
    renderSettings();

    expect(screen.getByRole("status", { name: "Đang tải…" })).toBeVisible();
    expect(nav().getByRole("link", { name: "Đăng nhập" })).toBeVisible();
    expect(await name()).toBeVisible();
    expect(screen.queryByRole("status", { name: "Đang tải…" })).toBeNull();
  });

  it("says the document could not be read, and tries again on Retry", async () => {
    server.use(
      http.get(`${BASE}/admin/settings`, () =>
        contractJson(
          "/admin/settings",
          "get",
          500,
          failure("INTERNAL", "Lỗi máy chủ."),
        ),
      ),
    );
    renderSettings();
    const user = userEvent.setup();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Không tải được cài đặt hệ thống.",
    );
    servesSettings();
    await user.click(screen.getByRole("button", { name: "Thử lại" }));

    expect(await name()).toHaveValue("Quizzivy English Centre");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows the Organization card with the contact lines DG-41 asks for", async () => {
    servesSettings();
    renderSettings();

    expect(await name()).toBeVisible();
    for (const [label, value] of [
      ["Tên viết tắt", "Quizzivy"],
      ["Website", "quizzivy.com"],
      ["Email liên hệ", "hello@quizzivy.com"],
      ["Điện thoại quầy lễ tân", "028 7300 1234"],
      ["Giờ làm việc của quầy lễ tân", "8:00 – 20:00 hằng ngày"],
      ["Địa chỉ", "12 Nguyễn Huệ, Quận 1, TP.HCM"],
    ] as const)
      expect(screen.getByRole("textbox", { name: label })).toHaveValue(value);
    expect(screen.getByText("Dùng trong email và thẻ trình duyệt")).toBeVisible();
    expect(screen.getByRole("button", { name: "Thay logo" })).toBeVisible();
  });

  it("draws the Quizzivy mark until the centre has a logo, then the logo", async () => {
    servesSettings();
    const { unmount } = renderPage();
    expect(await name()).toBeVisible();
    expect(screen.queryByRole("img", { name: "Logo của trung tâm" })).toBeNull();
    unmount();

    servesSettings({
      ...SAVED,
      organization: {
        ...SAVED.organization,
        logo: { light: { sha256: "ab".repeat(32), type: "image/png" }, dark: null },
      },
    });
    renderPage();
    expect(
      await screen.findByRole("img", { name: "Logo của trung tâm" }),
    ).toHaveAttribute(
      "src",
      `${BASE}/public/organization/logo?variant=light&v=${"ab".repeat(32)}`,
    );
  });

  it("opens each section at its own address and keeps the other mounted", async () => {
    servesSettings();
    renderSettings("/admin/settings/sign-in");

    expect(
      await screen.findByRole("switch", { name: "Đăng nhập bằng Google" }),
    ).toBeVisible();
    expect(nav().getByRole("link", { name: "Đăng nhập" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(
      screen.getByRole("textbox", { name: "Tên tổ chức", hidden: true }),
    ).not.toBeVisible();
  });

  it.each([
    ["an unknown section", "/admin/settings/nope"],
    ["Data & privacy, which has not shipped", "/admin/settings/data"],
    ["Plan & usage, which has not shipped", "/admin/settings/plan"],
  ])("replaces %s with the bare address", async (_, path) => {
    servesSettings();
    const { router } = renderSettings(path);

    await waitFor(() => expect(router.state.location.pathname).toBe("/admin/settings"));
    expect(router.state.historyAction).toBe("REPLACE");
    expect(await name()).toBeVisible();
  });
});

describe("the Sign-in section reached from the join-codes callout (D-028)", () => {
  it("opens at /admin/settings/sign-in for a holder of system.settings.write", async () => {
    servesSettings();
    const router = createMemoryRouter(appRouter.routes, {
      initialEntries: ["/admin/settings/sign-in"],
    });
    render(
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );

    for (const label of [
      "Đăng nhập bằng Google",
      "Đăng nhập bằng email và mật khẩu",
      "Bắt buộc đổi mật khẩu ở lần đăng nhập đầu",
      "Học viên có thể vào lớp bằng mã",
    ])
      expect(await screen.findByRole("switch", { name: label })).toBeVisible();
    expect(router.state.location.pathname).toBe("/admin/settings/sign-in");
    expect(adminUser.permissions).toContain("system.settings.write");
  });
});

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      {
        path: "/admin/settings/:section?",
        lazy: async () => ({
          Component: (
            await import("@/features/organization/pages/admin/SystemSettingsPage")
          ).default,
        }),
      },
    ],
    { initialEntries: ["/admin/settings"] },
  );
  return render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}
