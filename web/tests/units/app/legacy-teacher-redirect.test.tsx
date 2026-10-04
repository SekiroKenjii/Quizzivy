import { afterEach, describe, expect, it } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryRouter,
  Outlet,
  RouterProvider,
  useLocation,
  type RouteObject,
} from "react-router";
import { LegacyTeacherRedirect } from "@/app/LegacyTeacherRedirect";
import { router as appRouter } from "@/app/router";
import { destinationAfterSignIn } from "@/features/auth/home";
import { useAuthStore } from "@/stores/auth";
import { adminUser, studentUser, teacherUser } from "@tests/support/fixtures";
import "@/lib/i18n";

function Here() {
  const { pathname, search, hash } = useLocation();
  return <p>{`trang ${pathname}${search}${hash}`}</p>;
}

function withoutPages(routes: RouteObject[]): RouteObject[] {
  return routes.map((route) => {
    if (route.index) return route.lazy ? { index: true, element: <Here /> } : route;
    const { lazy, children, ...rest } = route;
    return {
      ...rest,
      ...(lazy ? { element: children ? <Outlet /> : <Here /> } : {}),
      ...(children ? { children: withoutPages(children) } : {}),
    };
  });
}

function show(routes: RouteObject[], entries: string[]) {
  const router = createMemoryRouter(routes, {
    initialEntries: entries,
    initialIndex: entries.length - 1,
  });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

const alone = (path: string): RouteObject[] => [
  { path, element: <LegacyTeacherRedirect /> },
  { path: "/teacher/*", element: <Here /> },
  { path: "*", element: <p>trang trước đó</p> },
];

const address = (router: ReturnType<typeof show>) => {
  const { pathname, search, hash } = router.state.location;
  return pathname + search + hash;
};

function signIn(user: typeof studentUser | null) {
  useAuthStore.setState({
    isBootstrapping: false,
    accessToken: user ? "t" : null,
    user,
  });
}

afterEach(() => {
  useAuthStore.getState().clearSession();
});

describe("LegacyTeacherRedirect", () => {
  it("ends at the same path under /teacher, with the query and the hash", async () => {
    const router = show(alone("/admin/*"), ["/admin/assignments?classId=1#top"]);

    expect(
      await screen.findByText("trang /teacher/assignments?classId=1#top"),
    ).toBeInTheDocument();
    expect(router.state.location).toMatchObject({
      pathname: "/teacher/assignments",
      search: "?classId=1",
      hash: "#top",
    });
  });

  it("replaces the old address, so going back leaves it behind", async () => {
    const router = show(alone("/admin/*"), [
      "/before",
      "/admin/assignments?classId=1#top",
    ]);

    await screen.findByText("trang /teacher/assignments?classId=1#top");
    expect(router.state.historyAction).toBe("REPLACE");

    await router.navigate(-1);
    expect(await screen.findByText("trang trước đó")).toBeInTheDocument();
    expect(address(router)).toBe("/before");
  });

  it.each([
    ["/admin", "/teacher"],
    ["/admin/", "/teacher/"],
    ["/admin?tab=draft", "/teacher?tab=draft"],
    ["/admin/tests/abc/edit#q3", "/teacher/tests/abc/edit#q3"],
    ["/Admin/settings/security", "/teacher/settings/security"],
  ])("sends %s to %s", async (from, to) => {
    const router = show(alone("/admin/*"), [from]);

    await waitFor(() => expect(address(router)).toBe(to));
  });

  it("sends an address it cannot map to the teacher's home, keeping the query", async () => {
    const router = show(alone("/old/*"), ["/old/tests?tab=draft"]);

    await waitFor(() => expect(address(router)).toBe("/teacher?tab=draft"));
  });
});

describe("the old /admin addresses in the app's own route table", () => {
  const routes = withoutPages(appRouter.routes);

  it("keeps next= as it was written at sign-in", () => {
    expect(destinationAfterSignIn("/admin/tests", teacherUser)).toBe("/admin/tests");
    expect(destinationAfterSignIn("/admin/tests?tab=draft", adminUser)).toBe(
      "/admin/tests?tab=draft",
    );
  });

  it("takes a teacher from that next= to the page at /teacher", async () => {
    signIn(teacherUser);
    const router = show(routes, [destinationAfterSignIn("/admin/tests", teacherUser)]);

    expect(await screen.findByText("trang /teacher/tests")).toBeInTheDocument();
    expect(address(router)).toBe("/teacher/tests");
  });

  it.each([
    ["/admin", "/teacher"],
    ["/admin/imports/abc/review", "/teacher/imports/abc/review"],
    ["/admin/assignments/new?classId=c1", "/teacher/assignments/new?classId=c1"],
    ["/admin/settings/security", "/teacher/settings/security"],
    ["/admin/tests/abc#versions", "/teacher/tests/abc#versions"],
    ["/Admin/Tests/ABC", "/teacher/Tests/ABC"],
  ])("opens %s as %s for a teacher", async (from, to) => {
    signIn(teacherUser);
    const router = show(routes, [from]);

    expect(await screen.findByText(`trang ${to}`)).toBeInTheDocument();
    expect(address(router)).toBe(to);
  });

  it("redirects before it asks who is there, so a visitor signs in with the new address", async () => {
    signIn(null);
    const router = show(routes, ["/admin/tests?tab=draft"]);

    await waitFor(() => expect(router.state.location.pathname).toBe("/login"));
    expect(router.state.location.search).toBe(
      `?next=${encodeURIComponent("/teacher/tests?tab=draft")}`,
    );
  });

  it("gives a student the 403 at the new address, as the teacher's tree does", async () => {
    signIn(studentUser);
    const router = show(routes, ["/admin/tests"]);

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Bạn không có quyền mở trang này",
      }),
    ).toBeInTheDocument();
    expect(address(router)).toBe("/teacher/tests");
    expect(screen.queryByText("trang /teacher/tests")).not.toBeInTheDocument();
  });

  it("leaves an address that only looks like the old prefix to the 404", async () => {
    signIn(teacherUser);
    const router = show(routes, ["/administrator"]);

    expect(await screen.findByText("/administrator")).toBeInTheDocument();
    expect(address(router)).toBe("/administrator");
  });
});
