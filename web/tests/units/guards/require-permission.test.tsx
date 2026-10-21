import { afterEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider, type RouteObject } from "react-router";
import { RequirePermission } from "@/app/guards/RequirePermission";
import { ADMIN_PATHS } from "@/app/adminPaths";
import type { AdminHandle } from "@/app/guards/adminHandle";
import { useAuthStore } from "@/stores/auth";
import { adminUser, studentUser, teacherUser } from "@tests/support/fixtures";
import "@/lib/i18n";

const FORBIDDEN = "Bạn không có quyền mở trang này";

const auditor: typeof adminUser = {
  ...studentUser,
  permissions: ["system.audit.read"],
  workspaces: ["admin"],
};
const userManager: typeof adminUser = {
  ...auditor,
  permissions: ["people.users.manage"],
};

function page(name: string, permission: AdminHandle["permission"]): RouteObject {
  const handle: AdminHandle = {
    crumb: [{ key: "adminShell.crumb" }],
    permission,
  };
  return { path: name, handle, element: <p>{`trang ${name}`}</p> };
}

const routes: RouteObject[] = [
  {
    path: "admin",
    element: <RequirePermission />,
    children: [
      page("users", "people.users.manage"),
      page("audit", "system.audit.read"),
      page("overview", "workspace.admin"),
      page("either", ["system.audit.read", "people.roles.manage"]),
      page("teaching", "workspace.teacher"),
      {
        path: "unnamed",
        handle: { crumb: [{ key: "adminShell.crumb" }] },
        element: <p>trang unnamed</p>,
      },
      { path: "bare", element: <p>trang bare</p> },
    ],
  },
];

function open(at: string, user: typeof adminUser | null) {
  useAuthStore.setState({
    isBootstrapping: false,
    accessToken: user ? "t" : null,
    user,
  });
  const router = createMemoryRouter(routes, { initialEntries: [at] });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

const forbidden = () => screen.queryByRole("heading", { level: 1, name: FORBIDDEN });

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("RequirePermission", () => {
  it("answers 403 where a custom role holding only system.audit.read opens the users page", async () => {
    const router = open(ADMIN_PATHS.users, auditor);

    expect(
      await screen.findByRole("heading", { level: 1, name: FORBIDDEN }),
    ).toBeInTheDocument();
    expect(screen.queryByText("trang users")).not.toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/admin/users");
  });

  it("opens the audit log for the same role", async () => {
    open("/admin/audit", auditor);

    expect(await screen.findByText("trang audit")).toBeInTheDocument();
    expect(forbidden()).not.toBeInTheDocument();
  });

  it("opens the users page for a role holding only people.users.manage", async () => {
    open("/admin/users", userManager);
    expect(await screen.findByText("trang users")).toBeInTheDocument();
  });

  it("refuses the audit log to a role holding only people.users.manage", async () => {
    open("/admin/audit", userManager);
    expect(
      await screen.findByRole("heading", { level: 1, name: FORBIDDEN }),
    ).toBeInTheDocument();
  });

  it.each(["users", "audit", "overview", "either", "teaching"])(
    "opens /admin/%s for an Admin",
    async (name) => {
      open(`/admin/${name}`, adminUser);

      expect(await screen.findByText(`trang ${name}`)).toBeInTheDocument();
      expect(forbidden()).not.toBeInTheDocument();
    },
  );

  it("opens a route whose permission is any of several for a holder of one", async () => {
    open("/admin/either", auditor);
    expect(await screen.findByText("trang either")).toBeInTheDocument();
  });

  it("refuses a route whose permission is any of several to a holder of none", async () => {
    open("/admin/either", userManager);
    expect(
      await screen.findByRole("heading", { level: 1, name: FORBIDDEN }),
    ).toBeInTheDocument();
  });

  it("reads the workspace pseudo-keys from the workspaces, not the permissions", async () => {
    open("/admin/overview", { ...adminUser, workspaces: ["teacher"] });
    expect(
      await screen.findByRole("heading", { level: 1, name: FORBIDDEN }),
    ).toBeInTheDocument();
  });

  it("refuses the teacher view's route to a user without the teacher workspace", async () => {
    open("/admin/teaching", { ...adminUser, workspaces: ["admin"] });
    expect(
      await screen.findByRole("heading", { level: 1, name: FORBIDDEN }),
    ).toBeInTheDocument();
  });

  it.each(["unnamed", "bare"])(
    "refuses /admin/%s, which names no permission, even to an Admin",
    async (name) => {
      open(`/admin/${name}`, adminUser);

      expect(
        await screen.findByRole("heading", { level: 1, name: FORBIDDEN }),
      ).toBeInTheDocument();
      expect(screen.queryByText(`trang ${name}`)).not.toBeInTheDocument();
    },
  );

  it("refuses a teacher every console route", async () => {
    open("/admin/overview", teacherUser);
    expect(
      await screen.findByRole("heading", { level: 1, name: FORBIDDEN }),
    ).toBeInTheDocument();
  });

  it("refuses when nobody is signed in, so a stale tab opens nothing", async () => {
    open("/admin/users", null);
    expect(
      await screen.findByRole("heading", { level: 1, name: FORBIDDEN }),
    ).toBeInTheDocument();
  });

  it("follows the user when a role changes under an open page", async () => {
    open("/admin/users", userManager);
    expect(await screen.findByText("trang users")).toBeInTheDocument();

    useAuthStore.setState({
      user: { ...userManager, permissions: ["system.audit.read"] },
    });

    expect(
      await screen.findByRole("heading", { level: 1, name: FORBIDDEN }),
    ).toBeInTheDocument();
  });
});
