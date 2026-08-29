import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider, type RouteObject } from "react-router";
import "@/lib/i18n";

// §5.3's config is optional: a deployment without a Google client id is a
// supported configuration, not a broken one. LoginPage renders the whole Google
// block behind googleSignInAvailable(), and nothing asserted what happens when
// it says no.
//
// CI was exercising exactly this branch by accident for twelve runs -- it has no
// .env, so the button was never rendered and the sibling test failed. Once that
// is fixed by pinning the variable for tests, this branch goes back to having no
// coverage at all unless it is asked for explicitly.
//
// Mocked rather than driven by the environment variable: googleSignInAvailable
// reads import.meta.env once at module load, so no per-test env change can move
// it after the fact.
vi.mock("@/features/auth/google/useGoogleSignIn", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/features/auth/google/useGoogleSignIn")>()),
  googleSignInAvailable: () => false,
}));

function renderLogin() {
  const routes: RouteObject[] = [
    { path: "/login", element: <LoginPage /> },
    { path: "/admin", element: <p>admin home</p> },
    { path: "/app", element: <p>student home</p> },
  ];
  render(
    <RouterProvider
      router={createMemoryRouter(routes, { initialEntries: ["/login"] })}
    />,
  );
}

const { default: LoginPage } = await import("@/features/auth/pages/LoginPage");

describe("/login without Google configured", () => {
  it("still offers password sign-in", () => {
    renderLogin();

    // The point of the branch: the page is fully usable, not degraded.
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(screen.getByLabelText("Mật khẩu")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Đăng nhập" })).toBeEnabled();
  });

  it("does not render a Google button or a dangling divider", () => {
    renderLogin();

    expect(screen.queryByRole("button", { name: "Tiếp tục với Google" })).toBeNull();
    expect(screen.queryByText(/hoặc/i)).toBeNull();
  });
});
