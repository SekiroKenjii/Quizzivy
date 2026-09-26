import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, Outlet, RouterProvider } from "react-router";
import { http } from "msw";
import { ErrorBoundary } from "@/app/ErrorBoundary";
import { RequireSession } from "@/app/guards/RequireSession";
import { useBootstrapSession } from "@/features/auth/useSession";
import { useAppState } from "@/stores/appState";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { contractJson } from "@tests/support/contractResponse";
import { adminUser } from "@tests/support/fixtures";
import "@/lib/i18n";

const BASE = "http://localhost:8080";

/** Restoring the session on a reload (§5.4). */
function Harness() {
  useBootstrapSession();
  return (
    <RouterProvider
      router={createMemoryRouter(
        [
          { path: "/login", element: <p>login page</p> },
          {
            ErrorBoundary,
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
  useAppState.setState({
    bootPhase: "booting",
    bootError: null,
    bootAttempt: 0,
    overlay: { kind: "none" },
  });
});

const REQUEST_ID = "019535d9-3df7-79fb-b466-fa907fa17f9e";

function meFailsBeforeAnyRefresh(response: () => Response | Promise<Response>) {
  server.use(
    http.get(`${BASE}/auth/me`, () =>
      Response.json({ error: { code: "UNAUTHORIZED", message: "" } }, { status: 401 }),
    ),
    http.post(`${BASE}/auth/refresh`, response),
  );
}

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
    await inFlight;
    view.unmount();
    release();
    await new Promise((r) => setTimeout(r, 20));

    expect(useAuthStore.getState().isBootstrapping).toBe(true);
    expect(useAuthStore.getState().user).toBeNull();
  });

  it("stays put, session untouched, when the API cannot be reached", async () => {
    meFailsBeforeAnyRefresh(() => Response.error());

    renderApp();

    await waitFor(() => expect(useAppState.getState().bootPhase).toBe("offline"));
    expect(useAuthStore.getState().isBootstrapping).toBe(true);
    expect(screen.queryByText("login page")).toBeNull();
  });

  it("tries again when asked, and lands where it was going", async () => {
    let answered = false;
    server.use(
      http.get(`${BASE}/auth/me`, () =>
        answered ? contractJson("/auth/me", "get", 200, adminUser) : Response.error(),
      ),
    );

    renderApp();
    await waitFor(() => expect(useAppState.getState().bootPhase).toBe("offline"));

    answered = true;
    act(() => useAppState.getState().retryBoot());

    expect(await screen.findByText("admin home")).toBeInTheDocument();
    expect(useAppState.getState().bootPhase).toBe("ready");
  });

  it("raises the maintenance overlay instead of signing out", async () => {
    const window = { startsAt: "2026-10-01T15:00:00Z", endsAt: "2026-10-01T16:00:00Z" };
    meFailsBeforeAnyRefresh(() =>
      Response.json(
        {
          error: {
            code: "MAINTENANCE",
            message: "Quizzivy đang bảo trì.",
            requestId: REQUEST_ID,
            details: window,
          },
        },
        { status: 503 },
      ),
    );

    renderApp();

    await waitFor(() =>
      expect(useAppState.getState().overlay).toEqual({ kind: "maintenance", window }),
    );
    expect(screen.queryByText("login page")).toBeNull();
    expect(useAuthStore.getState().isBootstrapping).toBe(true);
  });

  it("shows the unexpected-error page with the server's request id for anything else", async () => {
    server.use(
      http.get(`${BASE}/auth/me`, () =>
        Response.json(
          { error: { code: "INTERNAL", message: "boom", requestId: REQUEST_ID } },
          { status: 500 },
        ),
      ),
    );
    const quiet = vi.spyOn(console, "error").mockImplementation(() => undefined);

    renderApp();

    expect(await screen.findByText(REQUEST_ID)).toBeInTheDocument();
    expect(screen.queryByText("login page")).toBeNull();
    quiet.mockRestore();
  });
});
