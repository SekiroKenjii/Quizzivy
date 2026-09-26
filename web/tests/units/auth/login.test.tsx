import { afterEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import { createMemoryRouter, RouterProvider, type RouteObject } from "react-router";
import LoginPage from "@/features/auth/pages/LoginPage";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { saveJoinContext } from "@/features/join/context";
import { useAuthStore } from "@/stores/auth";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

function renderLogin(initialEntry = "/login") {
  const routes: RouteObject[] = [
    { path: "/login", element: <LoginPage /> },
    { path: "/admin", element: <p>admin home</p> },
    { path: "/app", element: <p>student home</p> },
    { path: "/join/:code", element: <p>join page</p> },
  ];
  const router = createMemoryRouter(routes, { initialEntries: [initialEntry] });
  render(<RouterProvider router={router} />);
  return router;
}

afterEach(() => {
  useAuthStore.getState().clearSession();
  sessionStorage.clear();
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

  it("says the credentials do not match in the product's words", async () => {
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
    expect(alert).toHaveTextContent(
      "Email và mật khẩu không khớp. Hãy kiểm tra lỗi gõ, hoặc nhờ giáo viên đặt lại mật khẩu.",
    );
  });

  it("shows any other server message as written, and clears it on the next edit", async () => {
    server.use(
      http.post(`${BASE}/auth/login`, () =>
        contractJson("/auth/login", "post", 429, {
          error: {
            code: "RATE_LIMITED",
            message: "Bạn thử quá nhiều lần. Hãy chờ một phút.",
            requestId: "019535d9-3df7-79fb-b466-fa907fa17f9e",
          },
        }),
      ),
    );

    const user = userEvent.setup();
    renderLogin();
    await user.type(screen.getByLabelText("Email"), "ai-do@example.com");
    await user.type(screen.getByLabelText("Mật khẩu"), "mat-khau");
    await user.click(screen.getByRole("button", { name: "Đăng nhập" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bạn thử quá nhiều lần. Hãy chờ một phút.",
    );
    await user.type(screen.getByLabelText("Mật khẩu"), "x");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("asks for both fields before sending anything", async () => {
    const user = userEvent.setup();
    renderLogin();
    await user.click(screen.getByRole("button", { name: "Đăng nhập" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Hãy nhập email và mật khẩu.",
    );
    expect(useAuthStore.getState().accessToken).toBeNull();
  });

  it("marks the button busy while signing in rather than disabling it", async () => {
    let release = () => {};
    server.use(
      http.post(
        `${BASE}/auth/login`,
        () =>
          new Promise<Response>((resolve) => {
            release = () => resolve(new Response(null, { status: 503 }));
          }),
      ),
    );
    const user = userEvent.setup();
    renderLogin();
    await user.type(screen.getByLabelText("Email"), "thuong@example.com");
    await user.type(screen.getByLabelText("Mật khẩu"), "quizzivy-dev");
    await user.click(screen.getByRole("button", { name: "Đăng nhập" }));

    const busy = await screen.findByRole("button", { name: "Đang đăng nhập…" });
    expect(busy).toHaveAttribute("aria-busy", "true");
    expect(busy).toBeEnabled();
    release();
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

  it("walks from Google through the form in the order the deck draws it", async () => {
    const user = userEvent.setup();
    renderLogin();

    screen.getByRole("button", { name: "Tiếp tục với Google" }).focus();
    await user.tab();
    expect(screen.getByLabelText("Email")).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("link", { name: "Quên mật khẩu?" })).toHaveFocus();
    await user.tab();
    expect(screen.getByLabelText("Mật khẩu")).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Hiện mật khẩu" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "Đăng nhập" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("link", { name: "Tham gia lớp" })).toHaveFocus();
  });

  it("uses the autocomplete tokens password managers need", () => {
    renderLogin();
    expect(screen.getByLabelText("Email")).toHaveAttribute("autocomplete", "username");
    expect(screen.getByLabelText("Mật khẩu")).toHaveAttribute(
      "autocomplete",
      "current-password",
    );
  });

  it("never renders the deck's prototype chrome", () => {
    renderLogin();
    expect(
      screen.queryByText(/demo accounts|tài khoản dùng thử|T6NB-4WLQ/i),
    ).toBeNull();
  });

  it("offers no way to create an account", () => {
    renderLogin();
    expect(screen.queryByRole("button", { name: /đăng ký|tạo tài khoản/i })).toBeNull();
    expect(screen.queryByRole("link", { name: /đăng ký|tạo tài khoản/i })).toBeNull();
  });

  it("names the class a visitor came to join, and goes back to finish it", async () => {
    saveJoinContext({
      code: "K7QM2PXA",
      className: "TOEIC 600 Weekend",
      teacherName: "Hoàng Thương",
    });
    const user = userEvent.setup();
    const router = renderLogin();
    expect(screen.getByText("Đăng nhập để tham gia TOEIC 600 Weekend.")).toBeVisible();

    await user.type(screen.getByLabelText("Email"), "thuong@example.com");
    await user.type(screen.getByLabelText("Mật khẩu"), "quizzivy-dev");
    await user.click(screen.getByRole("button", { name: "Đăng nhập" }));

    await waitFor(() => expect(router.state.location.pathname).toBe("/join/K7QM2PXA"));
  });

  it("welcomes back a visitor who is not joining anything", () => {
    renderLogin();
    expect(screen.getByText("Chào mừng bạn trở lại.")).toBeVisible();
  });
});
