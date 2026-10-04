import { afterEach, describe, expect, it } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryRouter,
  Outlet,
  RouterProvider,
  type RouteObject,
} from "react-router";
import { RequireSession } from "@/app/guards/RequireSession";
import { HomeRedirect } from "@/app/guards/HomeRedirect";
import { StudentArea, TeacherWorkspace } from "@/app/guards/RequireWorkspace";
import { useAuthStore } from "@/stores/auth";
import {
  adminUser,
  adminWhoTakesTests,
  assistantUser,
  studentUser,
  teacherUser,
} from "@tests/support/fixtures";
import "@/lib/i18n";

/** §5.4's route rules. Each one exists because of a specific failure: */

/** A router with every guarded shape, so a test only has to pick a URL. */
function renderAt(path: string) {
  const routes: RouteObject[] = [
    { path: "/", element: <HomeRedirect /> },
    { path: "/login", element: <p>login page</p> },
    { path: "/change-password", element: <p>change password page</p> },
    {
      element: <RequireSession />,
      children: [
        { path: "/change-password-guarded", element: <p>change password page</p> },
        {
          path: "/admin",
          element: <TeacherWorkspace />,
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
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("RequireSession", () => {
  it("waits while the session is still being restored", async () => {
    useAuthStore.setState({ isBootstrapping: true, user: null, accessToken: null });
    const router = renderAt("/app");
    await waitFor(() => expect(router.state.initialized).toBe(true));
    await act(() => new Promise((resolve) => setTimeout(resolve, 20)));

    expect(router.state.location.pathname).toBe("/app");
    expect(screen.queryByText("login page")).not.toBeInTheDocument();
    expect(screen.queryByText("student home")).not.toBeInTheDocument();
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

const nobody: typeof studentUser = { ...studentUser, permissions: [], workspaces: [] };

describe.each([
  {
    who: "the Admin",
    user: adminUser,
    home: "/admin",
    admin: "admin home",
    app: "/admin",
  },
  {
    who: "a Teacher",
    user: teacherUser,
    home: "/admin",
    admin: "admin home",
    app: "/admin",
  },
  {
    who: "an Assistant",
    user: assistantUser,
    home: "/admin",
    admin: "admin home",
    app: "/admin",
  },
  { who: "a Student", user: studentUser, home: "/app", admin: "403", app: "/app" },
  {
    who: "an Admin who takes tests",
    user: adminWhoTakesTests,
    home: "/admin",
    admin: "admin home",
    app: "/app",
  },
  {
    who: "a role with no workspace",
    user: nobody,
    home: "/app",
    admin: "403",
    app: "403",
  },
])("workspace guards for $who", ({ user, home, admin, app }) => {
  function signIn() {
    useAuthStore.setState({ isBootstrapping: false, accessToken: "t", user });
  }

  function expectForbidden(router: ReturnType<typeof renderAt>, path: string) {
    return waitFor(() => {
      expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
      expect(screen.queryByText("admin home")).not.toBeInTheDocument();
      expect(screen.queryByText("student home")).not.toBeInTheDocument();
      expect(router.state.location.pathname).toBe(path);
    });
  }

  it(`lands on ${home} from /`, async () => {
    signIn();
    const router = renderAt("/");
    await waitFor(() => expect(router.state.location.pathname).toBe(home));
  });

  it(`answers /admin with ${admin}`, async () => {
    signIn();
    const router = renderAt("/admin");
    if (admin === "403") {
      await expectForbidden(router, "/admin");
      return;
    }
    expect(await screen.findByText(admin)).toBeInTheDocument();
  });

  it(`answers /app with ${app}`, async () => {
    signIn();
    const router = renderAt("/app");
    if (app === "403") {
      await expectForbidden(router, "/app");
      return;
    }
    await waitFor(() => expect(router.state.location.pathname).toBe(app));
    expect(
      await screen.findByText(app === "/app" ? "student home" : "admin home"),
    ).toBeInTheDocument();
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

    try {
      await router.navigate("/login", { replace: true });
      useAuthStore.getState().signOut();
      await waitFor(() => {
        expect(router.state.navigation.state).toBe("idle");
        expect(router.state.location.pathname).toBe("/login");
      });
      await act(() => new Promise((resolve) => setTimeout(resolve, 0)));
      await act(() => new Promise((resolve) => setTimeout(resolve, 0)));

      expect(router.state.location.pathname).toBe("/login");
      expect(router.state.location.search).toBe("");
    } finally {
      useAuthStore.setState({ signedOut: false });
    }
  });
});

describe("losing the session", () => {
  it("leaves a visitor on a public screen where they are", async () => {
    const routes: RouteObject[] = [
      { path: "/login", element: <p>login page</p> },
      { path: "/join/:code", element: <p>join page</p> },
      {
        element: <RequireSession />,
        children: [{ path: "/app", element: <p>student home</p> }],
      },
    ];
    const router = createMemoryRouter(routes, {
      initialEntries: ["/join/K7M3P9QR"],
    });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
    expect(await screen.findByText("join page")).toBeInTheDocument();

    // What the API client does when a session turns out not to exist.
    useAuthStore.getState().clearSession();

    expect(router.state.location.pathname).toBe("/join/K7M3P9QR");
    expect(screen.getByText("join page")).toBeInTheDocument();
  });
});
