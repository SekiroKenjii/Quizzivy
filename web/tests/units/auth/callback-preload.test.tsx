import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { http } from "msw";
import { createMemoryRouter, RouterProvider } from "react-router";
import GoogleCallbackPage from "@/features/auth/pages/GoogleCallbackPage";
import { rememberPending } from "@/features/auth/google/pkce";
import { preloadStudentHome } from "@/features/auth/home";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { useAuthStore } from "@/stores/auth";
import "@/lib/i18n";

vi.mock("@/features/auth/home", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/features/auth/home")>();
  return { ...actual, preloadStudentHome: vi.fn() };
});

const BASE = "http://localhost:8080";
const preload = vi.mocked(preloadStudentHome);

const STUDENT = {
  id: "018f0000-0000-7000-8000-0000000000e9",
  email: "an@example.com",
  fullName: "Nguyễn Văn An",
  role: "student",
  hasPassword: false,
  linkedProviders: ["google"],
  mustChangePassword: false,
  createdAt: "2026-01-01T00:00:00Z",
};

/** Back from Google with a code, the way the redirect lands. */
function arrive(pending: { joinCode?: string }) {
  const state = "s".repeat(43);
  rememberPending({ verifier: "v".repeat(43), state, mode: "signin", ...pending });
  const router = createMemoryRouter(
    [
      { path: "/auth/google/callback", element: <GoogleCallbackPage /> },
      { path: "/app", element: <p>student home</p> },
      { path: "/login", element: <p>login</p> },
    ],
    { initialEntries: [`/auth/google/callback?code=abc&state=${state}`] },
  );
  render(<RouterProvider router={router} />);
}

beforeEach(() => {
  sessionStorage.clear();
  preload.mockReset();
  server.use(
    http.post(`${BASE}/auth/google`, () =>
      contractJson("/auth/google", "post", 200, {
        accessToken: "token",
        expiresIn: 900,
        user: STUDENT,
      }),
    ),
  );
});
afterEach(() => {
  useAuthStore.getState().clearSession();
});

/**
 * Rendering /app needs two nested lazy chunks after the URL settles. A student
 * arriving from a QR code waits for both before the first screen they ever
 * see; overlapping that with the exchange is what makes the wait shorter.
 */
describe("the student's home during the code exchange", () => {
  it("has started downloading by the time the exchange reaches the server", async () => {
    let startedFirst: boolean | null = null;
    server.use(
      http.post(`${BASE}/auth/google`, () => {
        startedFirst = preload.mock.calls.length > 0;
        return contractJson("/auth/google", "post", 200, {
          accessToken: "token",
          expiresIn: 900,
          user: STUDENT,
        });
      }),
    );

    arrive({ joinCode: "ABCD-EFGH" });
    expect(await screen.findByText("student home")).toBeInTheDocument();
    expect(startedFirst).toBe(true);
    expect(preload).toHaveBeenCalledTimes(1);
  });

  // §2: an anonymous visitor downloads neither tree.
  it("is not touched for a plain sign-in, whose role is not yet known", async () => {
    arrive({});
    expect(await screen.findByText("student home")).toBeInTheDocument();
    expect(preload).not.toHaveBeenCalled();
  });
});
