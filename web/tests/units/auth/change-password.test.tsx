import { afterEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import ChangePasswordPage from "@/features/auth/pages/ChangePasswordPage";
import { PasswordSection } from "@/features/auth/components/SettingsSections";
import { saveJoinContext } from "@/features/join/context";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { useAuthStore } from "@/stores/auth";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

const USER = {
  id: "018f0000-0000-7000-8000-0000000000a2",
  email: "an@example.com",
  fullName: "Nguyễn Văn An",
  role: "student" as const,
  hasPassword: true,
  linkedProviders: [],
  mustChangePassword: true,
  createdAt: "2026-01-01T00:00:00Z",
};

/** Counts requests: a client-side rejection must never reach the server. */
function countPosts() {
  let posts = 0;
  server.use(
    http.post(`${BASE}/auth/change-password`, () => {
      posts += 1;
      return new Response(null, { status: 204 });
    }),
  );
  return () => posts;
}

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("the forced password change", () => {
  it("refuses a short password at the field, without a request", async () => {
    const posts = countPosts();
    const user = userEvent.setup();
    const router = createMemoryRouter(
      [{ path: "/change-password", element: <ChangePasswordPage /> }],
      { initialEntries: ["/change-password"] },
    );
    render(<RouterProvider router={router} />);

    const field = screen.getByLabelText("Mật khẩu mới");
    await user.type(field, "abc");
    await user.click(screen.getByRole("button", { name: "Đổi mật khẩu" }));

    expect(
      await screen.findByText("Mật khẩu mới cần ít nhất 8 ký tự."),
    ).toBeInTheDocument();
    expect(field).toHaveAttribute("aria-invalid", "true");
    expect(field).toHaveFocus();
    expect(posts()).toBe(0);
  });
});

describe("the forced password change during a join", () => {
  it("goes back to finish the join once the password is changed", async () => {
    sessionStorage.clear();
    saveJoinContext({
      code: "K7QM2PXA",
      className: "TOEIC 600 Weekend",
      teacherName: "Hoàng Thương",
    });
    countPosts();
    server.use(
      http.get(`${BASE}/auth/me`, () =>
        contractJson("/auth/me", "get", 200, { ...USER, mustChangePassword: false }),
      ),
    );
    const user = userEvent.setup();
    const router = createMemoryRouter(
      [
        { path: "/change-password", element: <ChangePasswordPage /> },
        { path: "/join/:code", element: <p>join page</p> },
      ],
      { initialEntries: ["/change-password"] },
    );
    render(<RouterProvider router={router} />);

    await user.type(screen.getByLabelText("Mật khẩu mới"), "một-mật-khẩu-mới-9");
    await user.click(screen.getByRole("button", { name: "Đổi mật khẩu" }));

    expect(await screen.findByText("join page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/join/K7QM2PXA");
  });
});

describe("the settings password form", () => {
  it("refuses a short password at the field, without a request", async () => {
    useAuthStore.getState().setSession("token", USER);
    const posts = countPosts();
    const user = userEvent.setup();
    render(<PasswordSection />);

    const field = screen.getByLabelText("Mật khẩu mới");
    await user.type(field, "abc");
    await user.click(screen.getByRole("button", { name: "Đổi mật khẩu" }));

    expect(
      await screen.findByText("Mật khẩu mới cần ít nhất 8 ký tự."),
    ).toBeInTheDocument();
    expect(field).toHaveAttribute("aria-invalid", "true");
    expect(posts()).toBe(0);
  });
});
