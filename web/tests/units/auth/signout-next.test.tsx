import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import {
  createMemoryRouter,
  RouterProvider,
  type DataRouter,
  type RouteObject,
} from "react-router";
import { RequireSession } from "@/app/guards/RequireSession";
import { useLogout } from "@/features/auth/useSession";
import { useAuthStore } from "@/stores/auth";
import { server } from "@tests/support/server";
import { studentUser } from "@tests/support/fixtures";

vi.mock("@/lib/drafts/store", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/drafts/store")>()),
  clearAuthoringDrafts: async () => undefined,
}));

const BASE = "http://localhost:8080";

const loginPage: RouteObject = { path: "/login", element: <p>login page</p> };

const lazyLoginPage: RouteObject = {
  path: "/login",
  lazy: async () => {
    await new Promise((resolve) => setTimeout(resolve, 0));
    return { element: <p>login page</p> };
  },
};

function SignOut() {
  const logout = useLogout();
  return (
    <button type="button" onClick={() => void logout()}>
      sign out
    </button>
  );
}

function openSignedIn(entries: string[], login: RouteObject = loginPage) {
  useAuthStore.getState().setSession("t", studentUser);
  server.use(
    http.post(`${BASE}/auth/logout`, () => new HttpResponse(null, { status: 204 })),
  );
  const router = createMemoryRouter(
    [
      {
        element: <RequireSession />,
        children: [
          { path: "/app", element: <SignOut /> },
          { path: "/app/one", element: <SignOut /> },
          { path: "/app/two", element: <SignOut /> },
        ],
      },
      login,
    ],
    { initialEntries: entries },
  );
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

async function signOut() {
  const user = userEvent.setup();
  await user.click(await screen.findByRole("button", { name: "sign out" }));
}

async function settled(router: DataRouter) {
  await waitFor(() => {
    expect(router.state.navigation.state).toBe("idle");
    expect(router.state.location.pathname).toBe("/login");
  });
  await act(() => new Promise((resolve) => setTimeout(resolve, 0)));
  await act(() => new Promise((resolve) => setTimeout(resolve, 0)));
}

function address(router: DataRouter) {
  return router.state.location.pathname + router.state.location.search;
}

afterEach(() => {
  cleanup();
  useAuthStore.setState({ signedOut: false });
  useAuthStore.getState().clearSession();
});

describe("signing out", () => {
  it("ends on the sign-in page with no next", async () => {
    const router = openSignedIn(["/app"]);

    await signOut();
    await settled(router);

    expect(router.state.location.pathname).toBe("/login");
    expect(router.state.location.search).toBe("");
  });

  it("does so when the sign-in page loads lazily", async () => {
    const router = openSignedIn(["/app"], lazyLoginPage);

    await signOut();
    await settled(router);

    expect(await screen.findByText("login page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/login");
    expect(router.state.location.search).toBe("");
  });

  it("attaches none when the user goes Back after signing out", async () => {
    const router = openSignedIn(["/app/one", "/app/two"]);

    await signOut();
    await settled(router);
    await act(() => router.navigate(-1));
    await settled(router);

    expect(router.state.location.pathname).toBe("/login");
    expect(router.state.location.search).toBe("");
  });

  it("still attaches next when a session expired", async () => {
    const router = openSignedIn(["/app"]);
    expect(await screen.findByRole("button", { name: "sign out" })).toBeInTheDocument();

    act(() => useAuthStore.getState().clearSession());

    await waitFor(() => expect(address(router)).toBe("/login?next=%2Fapp"));
  });

  it("attaches next again once somebody has signed in after a sign-out", async () => {
    const router = openSignedIn(["/app"]);
    await signOut();
    await settled(router);

    act(() => useAuthStore.getState().setSession("t", studentUser));
    await act(() => router.navigate("/app"));
    expect(await screen.findByRole("button", { name: "sign out" })).toBeInTheDocument();
    act(() => useAuthStore.getState().clearSession());

    await waitFor(() => expect(address(router)).toBe("/login?next=%2Fapp"));
  });

  it("marks only the user's own sign-out", () => {
    const store = useAuthStore.getState();

    store.setSession("t", studentUser);
    store.signOut();
    expect(useAuthStore.getState()).toMatchObject({
      user: null,
      accessToken: null,
      expired: false,
      signedOut: true,
    });

    store.setSession("t", studentUser);
    expect(useAuthStore.getState().signedOut).toBe(false);

    store.clearSession();
    expect(useAuthStore.getState().signedOut).toBe(false);
  });
});
