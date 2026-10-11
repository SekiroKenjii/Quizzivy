import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http } from "msw";
import { createMemoryRouter, Link, Outlet, RouterProvider } from "react-router";
import { Toaster } from "@/components/ui/sonner";
import TeacherSettingsPage from "@/features/settings/pages/teacher/TeacherSettingsPage";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import { teacherUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";
import { BASE, THIS_DEVICE, listsSessions, signIn } from "./support";
import "@/lib/i18n";

const SAVED = { ...teacherUser, fullName: "Hoàng Thương" };
const TITLE = "Rời Cài đặt mà không lưu?";

beforeEach(() => {
  contentWidth(1080);
  signIn(SAVED);
  listsSessions([THIS_DEVICE]);
});

afterEach(() => {
  useAuthStore.getState().clearSession();
});

function renderWithElsewhere() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(
    [
      {
        element: (
          <>
            <Link to="/teacher/classes">Lớp học</Link>
            <Outlet />
          </>
        ),
        children: [
          { path: "/teacher/settings/:section?", element: <TeacherSettingsPage /> },
          { path: "/teacher/classes", element: <h1>Trang lớp học</h1> },
          { path: "/login", element: <h1>Trang đăng nhập</h1> },
        ],
      },
    ],
    { initialEntries: ["/teacher/settings"] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return router;
}

const nameField = () => screen.getByRole("textbox", { name: "Họ và tên" });
const elsewhere = () => screen.getByRole("link", { name: "Lớp học" });

describe("leaving Settings with unsaved changes", () => {
  it("leaves at once when nothing is unsaved", async () => {
    const router = renderWithElsewhere();
    const user = userEvent.setup();

    await user.click(elsewhere());

    expect(await screen.findByRole("heading", { name: "Trang lớp học" })).toBeVisible();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(router.state.location.pathname).toBe("/teacher/classes");
  });

  it("asks first; Stay keeps the edit and returns focus to the link", async () => {
    const router = renderWithElsewhere();
    const user = userEvent.setup();

    await user.type(nameField(), " Hoàng");
    elsewhere().focus();
    await user.keyboard("{Enter}");

    const dialog = await screen.findByRole("dialog", { name: TITLE });
    expect(within(dialog).getByText("Bạn có thay đổi chưa lưu.")).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: "Ở lại" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(router.state.location.pathname).toBe("/teacher/settings");
    expect(nameField()).toHaveValue("Hoàng Thương Hoàng");
    expect(elsewhere()).toHaveFocus();
  });

  it("goes on when Leave is chosen", async () => {
    const router = renderWithElsewhere();
    const user = userEvent.setup();

    await user.type(nameField(), " Hoàng");
    await user.click(elsewhere());
    const dialog = await screen.findByRole("dialog", { name: TITLE });
    await user.click(within(dialog).getByRole("button", { name: "Rời đi" }));

    expect(await screen.findByRole("heading", { name: "Trang lớp học" })).toBeVisible();
    expect(router.state.location.pathname).toBe("/teacher/classes");
  });

  it("does not ask right after a successful save", async () => {
    server.use(
      http.patch(`${BASE}/auth/me`, async ({ request }) => {
        const body = (await request.json()) as Record<string, unknown>;
        return contractJson("/auth/me", "patch", 200, { ...SAVED, ...body });
      }),
    );
    const router = renderWithElsewhere();
    const user = userEvent.setup();

    await user.type(nameField(), " Hoàng");
    await user.click(screen.getByRole("button", { name: "Lưu thay đổi" }));
    expect(await screen.findByText("Đã lưu cài đặt")).toBeInTheDocument();
    await user.click(elsewhere());

    expect(await screen.findByRole("heading", { name: "Trang lớp học" })).toBeVisible();
    expect(screen.queryByRole("dialog", { name: TITLE })).toBeNull();
    expect(router.state.location.pathname).toBe("/teacher/classes");
  });

  it("does not ask when moving between sections, and keeps the edit", async () => {
    const router = renderWithElsewhere();
    const user = userEvent.setup();

    await user.type(nameField(), " Hoàng");
    await user.click(screen.getByRole("link", { name: "Giao diện" }));

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/teacher/settings/appearance"),
    );
    expect(screen.queryByRole("dialog", { name: TITLE })).toBeNull();
    await user.click(screen.getByRole("link", { name: "Hồ sơ" }));
    expect(nameField()).toHaveValue("Hoàng Thương Hoàng");
  });

  it("lets a move to the sign-in page through", async () => {
    const router = renderWithElsewhere();
    const user = userEvent.setup();

    await user.type(nameField(), " Hoàng");
    await router.navigate("/login");

    expect(
      await screen.findByRole("heading", { name: "Trang đăng nhập" }),
    ).toBeVisible();
    expect(screen.queryByRole("dialog", { name: TITLE })).toBeNull();
  });

  it("asks the browser to confirm a reload only while something is unsaved", async () => {
    renderWithElsewhere();
    const user = userEvent.setup();
    const unload = () => {
      const event = new Event("beforeunload", { cancelable: true });
      window.dispatchEvent(event);
      return event.defaultPrevented;
    };

    expect(unload()).toBe(false);
    await user.type(nameField(), " Hoàng");
    await waitFor(() => expect(unload()).toBe(true));
    await user.click(screen.getByRole("button", { name: "Bỏ thay đổi" }));
    await waitFor(() => expect(unload()).toBe(false));
  });
});
