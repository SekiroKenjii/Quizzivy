import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { afterEach, describe, expect, it, vi } from "vitest";

import "@/lib/i18n";

afterEach(() => {
  vi.unstubAllEnvs();
  vi.resetModules();
});

async function renderForgot() {
  const { default: ForgotPasswordPage } =
    await import("@/features/auth/pages/ForgotPasswordPage");
  const router = createMemoryRouter(
    [
      { path: "/forgot-password", element: <ForgotPasswordPage /> },
      { path: "/login", element: <p>sign in</p> },
    ],
    { initialEntries: ["/forgot-password"] },
  );
  render(<RouterProvider router={router} />);
}

describe("/forgot-password", () => {
  it("explains who resets a password and leads back to sign in", async () => {
    await renderForgot();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(
      "Quên mật khẩu?",
    );
    expect(
      screen.getByText(/Giáo viên hoặc trung tâm có thể đặt lại/),
    ).toBeInTheDocument();
    expect(screen.getByText("Bạn đăng ký bằng Google?")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Quay lại đăng nhập" })).toHaveAttribute(
      "href",
      "/login",
    );
  });

  it("hides the front desk until the centre's details are configured", async () => {
    await renderForgot();
    expect(screen.queryByText("Quầy lễ tân")).toBeNull();
  });

  it("shows the front desk as the centre wrote it", async () => {
    vi.stubEnv("VITE_ORG_FRONT_DESK_PHONE", "028 3812 4567");
    vi.stubEnv("VITE_ORG_FRONT_DESK_HOURS", "T2–T7, 8:00–20:00");
    await renderForgot();
    expect(screen.getByText("Quầy lễ tân")).toBeInTheDocument();
    expect(screen.getByText(/028 3812 4567 · T2–T7, 8:00–20:00/)).toBeInTheDocument();
  });
});
