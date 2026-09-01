import { beforeEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, Outlet, RouterProvider } from "react-router";
import { http } from "msw";
import { RequireSession } from "@/app/guards/RequireSession";
import { useBootstrapSession } from "@/features/auth/useSession";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { adminUser } from "@tests/support/fixtures";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

/**
 * Restoring the session on a reload (§5.4).
 *
 * The guard waits on `isBootstrapping` so a reload does not flash /login and
 * lose the deep link. That only holds if bootstrap finishes for the right
 * reason: an ABORTED request says the component went away, not that the session
 * is gone, and treating the two the same bounced a signed-in teacher to /login
 * on every reload while their refresh cookie was still good.
 */
function Harness() {
  useBootstrapSession();
  return (
    <RouterProvider
      router={createMemoryRouter(
        [
          { path: "/login", element: <p>login page</p> },
          {
            element: <RequireSession />,
            children: [
              {
                path: "/admin",
                element: <Outlet />,
                children: [{ index: true, element: <p>admin home</p> }],
              },
            ],
          },
        ],
        { initialEntries: ["/admin"] },
      )}
    />
  );
}

function renderApp() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Harness />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useAuthStore.setState({ accessToken: null, user: null, isBootstrapping: true });
});

describe("restoring a session on load", () => {
  it("lands on the deep link when the session is still good", async () => {
    server.use(
      http.get(`${BASE}/auth/me`, () =>
        contractJson("/auth/me", "get", 200, adminUser),
      ),
    );

    renderApp();

    expect(await screen.findByText("admin home")).toBeInTheDocument();
    expect(screen.queryByText("login page")).toBeNull();
  });

  it("does not flash /login while the answer is still in flight", async () => {
    server.use(
      http.get(`${BASE}/auth/me`, () =>
        contractJson("/auth/me", "get", 200, adminUser),
      ),
    );

    renderApp();

    // Before the request settles the guard must be waiting, not redirecting.
    expect(screen.queryByText("login page")).toBeNull();
    await screen.findByText("admin home");
  });

  it("goes to /login when the session really is gone", async () => {
    server.use(
      http.get(`${BASE}/auth/me`, () =>
        Response.json(
          { error: { code: "UNAUTHORIZED", message: "" } },
          { status: 401 },
        ),
      ),
      http.post(`${BASE}/auth/refresh`, () =>
        Response.json(
          { error: { code: "UNAUTHORIZED", message: "" } },
          { status: 401 },
        ),
      ),
    );

    renderApp();

    expect(await screen.findByText("login page")).toBeInTheDocument();
  });

  it("an unmount mid-flight does not report the session as gone", async () => {
    let started: () => void = () => undefined;
    const inFlight = new Promise<void>((r) => {
      started = r;
    });
    let release: () => void = () => undefined;
    const held = new Promise<void>((r) => {
      release = r;
    });

    server.use(
      http.get(`${BASE}/auth/me`, async () => {
        started();
        await held;
        return contractJson("/auth/me", "get", 200, adminUser);
      }),
    );

    const view = renderApp();
    // Unmount only once the request is genuinely out, or the abort never
    // happens and the test proves nothing.
    await inFlight;
    view.unmount();
    release();
    await new Promise((r) => setTimeout(r, 20));

    // clearSession() would have set isBootstrapping false with a null user,
    // which is the state that bounces the next render to /login.
    expect(useAuthStore.getState().isBootstrapping).toBe(true);
    expect(useAuthStore.getState().user).toBeNull();
  });
});
