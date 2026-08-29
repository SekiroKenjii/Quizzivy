import { afterEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  createMemoryRouter,
  Outlet,
  RouterProvider,
  type RouteObject,
} from "react-router";
import { RequireSession } from "@/app/guards/RequireSession";
import { AdminOnly, StudentArea } from "@/app/guards/RequireRole";
import { useAuthStore } from "@/stores/auth";
import { adminUser, studentUser } from "@tests/support/fixtures";
import "@/lib/i18n";

/**
 * §5.4's route rules. Each one exists because of a specific failure:
 *
 *  - waiting during bootstrap, so a reload does not flash /login and lose the
 *    deep link;
 *  - `?next=`, so signing in returns you to where you were going;
 *  - a 403 PAGE for a student on /admin, because a redirect makes a
 *    permissions mistake look like a navigation quirk;
 *  - a redirect for an admin on /app, because they have more access, not less,
 *    and there is nothing to surface.
 */

/** A router with every guarded shape, so a test only has to pick a URL. */
function renderAt(path: string) {
  const routes: RouteObject[] = [
    { path: "/login", element: <p>login page</p> },
    { path: "/change-password", element: <p>change password page</p> },
    {
      element: <RequireSession />,
      children: [
        { path: "/change-password-guarded", element: <p>change password page</p> },
        {
          path: "/admin",
          element: <AdminOnly />,
          children: [
            {
              element: <Outlet />,
              children: [{ index: true, element: <p>admin home</p> }],
            },
          ],
        },
        {
          path: "/app",
          element: <StudentArea />,
          children: [
            {
              element: <Outlet />,
              children: [{ index: true, element: <p>student home</p> }],
            },
          ],
        },
      ],
    },
  ];
  const router = createMemoryRouter(routes, { initialEntries: [path] });
  render(<RouterProvider router={router} />);
  return router;
}

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("RequireSession", () => {
  it("waits while the session is still being restored", async () => {
    useAuthStore.setState({ isBootstrapping: true, user: null, accessToken: null });
    renderAt("/app");
    expect(await screen.findByRole("status")).toBeInTheDocument();
    expect(screen.queryByText("login page")).not.toBeInTheDocument();
  });

  it("sends an anonymous visitor to /login with where they were going", async () => {
    useAuthStore.setState({ isBootstrapping: false, user: null, accessToken: null });
    const router = renderAt("/app");

    expect(await screen.findByText("login page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/login");
    expect(router.state.location.search).toBe(`?next=${encodeURIComponent("/app")}`);
  });

  it("forces the password change from any route", async () => {
    useAuthStore.setState({
      isBootstrapping: false,
      accessToken: "t",
      user: { ...studentUser, mustChangePassword: true },
    });
    const router = renderAt("/app");

    expect(await screen.findByText("change password page")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/change-password");
  });

  it("does not trap a Google-only account on the password change", () => {
    useAuthStore.setState({
      isBootstrapping: false,
      accessToken: "t",
      user: {
        ...studentUser,
        hasPassword: false,
        linkedProviders: ["google"],
        mustChangePassword: false,
      },
    });
    const router = renderAt("/app");
    expect(router.state.location.pathname).toBe("/app");
  });
});

describe("role guards", () => {
  it("renders 403 for a student on the admin tree, rather than navigating", async () => {
    useAuthStore.setState({
      isBootstrapping: false,
      accessToken: "t",
      user: studentUser,
    });
    const router = renderAt("/admin");
    expect(await screen.findByRole("heading", { level: 1 })).toBeInTheDocument();
    expect(screen.queryByText("admin home")).not.toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/admin");
  });

  it("lets a teacher into the admin tree", async () => {
    useAuthStore.setState({
      isBootstrapping: false,
      accessToken: "t",
      user: adminUser,
    });
    renderAt("/admin");
    expect(await screen.findByText("admin home")).toBeInTheDocument();
  });

  it("redirects a teacher off the student tree", async () => {
    useAuthStore.setState({
      isBootstrapping: false,
      accessToken: "t",
      user: adminUser,
    });
    const router = renderAt("/app");

    expect(await screen.findByText("admin home")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/admin");
  });

  it("lets a student into the student tree", async () => {
    useAuthStore.setState({
      isBootstrapping: false,
      accessToken: "t",
      user: studentUser,
    });
    renderAt("/app");
    expect(await screen.findByText("student home")).toBeInTheDocument();
  });
});

describe("signing out", () => {
  it("does not leave a ?next= pointing at the previous user's page", async () => {
    useAuthStore.setState({
      isBootstrapping: false,
      accessToken: "t",
      user: adminUser,
    });
    const router = renderAt("/admin");
    expect(await screen.findByText("admin home")).toBeInTheDocument();

    // The order useLogout uses: leave first, forget second.
    await router.navigate("/login", { replace: true });
    useAuthStore.getState().clearSession();

    expect(router.state.location.pathname).toBe("/login");
    expect(router.state.location.search).toBe("");
  });
});

describe("losing the session", () => {
  it("leaves a visitor on a public screen where they are", async () => {
    const routes: RouteObject[] = [
      { path: "/login", element: <p>login page</p> },
      { path: "/join/:code/confirm", element: <p>confirm page</p> },
      {
        element: <RequireSession />,
        children: [{ path: "/app", element: <p>student home</p> }],
      },
    ];
    const router = createMemoryRouter(routes, {
      initialEntries: ["/join/K7M3P9QR/confirm"],
    });
    render(<RouterProvider router={router} />);
    expect(await screen.findByText("confirm page")).toBeInTheDocument();

    // What the API client does when a session turns out not to exist.
    useAuthStore.getState().clearSession();

    expect(router.state.location.pathname).toBe("/join/K7M3P9QR/confirm");
    expect(screen.getByText("confirm page")).toBeInTheDocument();
  });
});
