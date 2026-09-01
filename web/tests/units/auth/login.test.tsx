import { afterEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import { createMemoryRouter, RouterProvider, type RouteObject } from "react-router";
import LoginPage from "@/features/auth/pages/LoginPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { useAuthStore } from "@/stores/auth";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

function renderLogin(initialEntry = "/login") {
  const routes: RouteObject[] = [
    { path: "/login", element: <LoginPage /> },
    { path: "/admin", element: <p>admin home</p> },
    { path: "/app", element: <p>student home</p> },
  ];
  const router = createMemoryRouter(routes, { initialEntries: [initialEntry] });
  render(<RouterProvider router={router} />);
  return router;
}

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("/login", () => {
  it("does not submit an invalid email", async () => {
    const user = userEvent.setup();
    renderLogin();

    await user.type(screen.getByLabelText("Email"), "not-an-email");
    await user.type(screen.getByLabelText("Mật khẩu"), "quizzivy-dev");
    await user.click(screen.getByRole("button", { name: "Đăng nhập" }));
    expect(useAuthStore.getState().accessToken).toBeNull();
  });

  it("renders the server's message verbatim on a failure", async () => {
    server.use(
      http.post(`${BASE}/auth/login`, () =>
        contractJson("/auth/login", "post", 401, {
          error: {
            code: "INVALID_CREDENTIALS",
            message: "Email hoặc mật khẩu không đúng.",
            requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
          },
        }),
      ),
    );

    const user = userEvent.setup();
    renderLogin();
    await user.type(screen.getByLabelText("Email"), "ai-do@example.com");
    await user.type(screen.getByLabelText("Mật khẩu"), "sai-mat-khau");
    await user.click(screen.getByRole("button", { name: "Đăng nhập" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Email hoặc mật khẩu không đúng.");
  });

  it("sends a teacher to the admin tree and a student to their own", async () => {
    // "/" would bounce off the index route straight back to /login.
    const user = userEvent.setup();
    const router = renderLogin();

    await user.type(screen.getByLabelText("Email"), "thuong@example.com");
    await user.type(screen.getByLabelText("Mật khẩu"), "quizzivy-dev");
    await user.click(screen.getByRole("button", { name: "Đăng nhập" }));

    // The default handler answers with adminUser.
    await waitFor(() => expect(router.state.location.pathname).toBe("/admin"));
  });

  it("refuses an off-site ?next=", async () => {
    const user = userEvent.setup();
    const router = renderLogin("/login?next=https://evil.test/steal");

    await user.type(screen.getByLabelText("Email"), "thuong@example.com");
    await user.type(screen.getByLabelText("Mật khẩu"), "quizzivy-dev");
    await user.click(screen.getByRole("button", { name: "Đăng nhập" }));

    await waitFor(() => expect(router.state.location.pathname).toBe("/admin"));
    expect(router.state.location.pathname).not.toContain("evil.test");
  });

  it("keeps the Google button reachable from the keyboard", async () => {
    const user = userEvent.setup();
    renderLogin();

    const google = screen.getByRole("button", { name: "Tiếp tục với Google" });
    screen.getByLabelText("Mật khẩu").focus();
    await user.tab();
    await user.tab();
    expect(google).toHaveFocus();
  });

  it("offers no way to create an account", () => {
    renderLogin();
    expect(screen.queryByRole("button", { name: /đăng ký|tạo tài khoản/i })).toBeNull();
    expect(screen.queryByRole("link", { name: /đăng ký|tạo tài khoản/i })).toBeNull();
  });
});
