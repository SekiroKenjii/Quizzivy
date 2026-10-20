import { expect } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import UsersPage from "@/features/users/pages/admin/UsersPage";
import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import { adminUser } from "@tests/support/fixtures";
import { server } from "@tests/support/server";

export const BASE = "http://localhost:8080";

export type UserRow = components["schemas"]["AdminUserRow"];
export type UserRole = UserRow["role"];
type Tab = components["schemas"]["AdminUserTab"];
type Facets = components["schemas"]["AdminUserFacets"];

export const ADMIN_ROLE: UserRole = {
  id: "019535d8-0000-7000-8000-000000000001",
  name: "Quản trị viên",
  builtinKey: "admin",
  icon: "shield",
  color: "dark",
};
export const TEACHER_ROLE: UserRole = {
  id: "019535d8-0000-7000-8000-000000000002",
  name: "Giáo viên",
  builtinKey: "teacher",
  icon: "graduation-cap",
  color: "lime",
};
export const STUDENT_ROLE: UserRole = {
  id: "019535d8-0000-7000-8000-000000000004",
  name: "Học viên",
  builtinKey: "student",
  icon: "user",
  color: "gray",
};
export const HEAD_ROLE: UserRole = {
  id: "019535d8-0000-7000-8000-0000000000a9",
  name: "Tổ trưởng bộ môn",
  builtinKey: null,
  icon: "crown",
  color: "amber",
};

/** HOURS_AGO is an instant that many whole hours before now, in ISO form. */
export function hoursAgo(hours: number): string {
  return new Date(Date.now() - hours * 3_600_000).toISOString();
}

/** user builds an AdminUserRow with the given name and overrides. */
export function user(
  id: string,
  fullName: string,
  overrides: Partial<UserRow> = {},
): UserRow {
  return {
    id: `018f0000-0000-7000-8000-${id.padStart(12, "0")}`,
    fullName,
    email: `${id}@example.com`,
    avatarUrl: null,
    role: STUDENT_ROLE,
    signIn: { password: true, google: false },
    classes: { count: 0, firstName: null },
    lastActiveAt: hoursAgo(3),
    status: "active",
    createdAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function tabOfRow(row: UserRow): Exclude<Tab, "all" | "disabled"> {
  if (row.role.builtinKey === "admin") return "admins";
  return row.role.builtinKey === "student" ? "students" : "teachers";
}

function inTab(row: UserRow, tab: Tab): boolean {
  if (tab === "all") return true;
  if (tab === "disabled") return row.status === "disabled";
  return tabOfRow(row) === tab;
}

/** facetsOf counts the five tabs over `rows`, as the server does under the same search. */
export function facetsOf(rows: readonly UserRow[]): Facets {
  return {
    all: rows.length,
    teachers: rows.filter((row) => inTab(row, "teachers")).length,
    students: rows.filter((row) => inTab(row, "students")).length,
    admins: rows.filter((row) => inTab(row, "admins")).length,
    disabled: rows.filter((row) => inTab(row, "disabled")).length,
  };
}

/**
 * serveUsers answers the list with `rows` filtered by `tab`, `status` and `q`
 * as the server would, one page of `limit` rows, and returns every request's
 * query. `facets` and `total` stand in for a centre larger than `rows`.
 */
export function serveUsers(
  rows: readonly UserRow[],
  fixed: { facets?: Facets; total?: number } = {},
) {
  const asked: URLSearchParams[] = [];
  server.use(
    http.get(`${BASE}/admin/users`, ({ request }) => {
      const params = new URL(request.url).searchParams;
      asked.push(params);
      const tab = (params.get("tab") ?? "all") as Tab;
      const status = params.get("status");
      const q = (params.get("q") ?? "").toLowerCase();
      const limit = Number(params.get("limit") ?? 20);
      const page = Number(params.get("page") ?? 1);
      const matching = rows.filter(
        (row) =>
          (q === "" || `${row.fullName} ${row.email}`.toLowerCase().includes(q)) &&
          (status === null || row.status === status),
      );
      const items = matching.filter((row) => inTab(row, tab));
      return contractJson("/admin/users", "get", 200, {
        items: items.slice((page - 1) * limit, page * limit),
        page,
        pageSize: limit,
        total: fixed.total ?? items.length,
        facets: fixed.facets ?? facetsOf(matching),
      });
    }),
  );
  return asked;
}

/**
 * renderUsers signs `who` in and renders the Users page at `path` with the
 * console's content `width`, and returns the router, the query client and a
 * user-event instance that does not wait between keys.
 */
export function renderUsers(
  path = "/admin/users",
  width = 1280,
  who: components["schemas"]["CurrentUser"] = adminUser,
) {
  useAuthStore.getState().setSession("token", who);
  const widths = contentWidth(width);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const router = createMemoryRouter(
    [{ path: "/admin/users", element: <UsersPage /> }],
    { initialEntries: [path] },
  );
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup({ delay: null }), router, client, widths };
}

/** table is the users table once it has rows. */
export async function table() {
  return screen.findByRole("table", { name: "Người dùng" });
}

/** rowOf is the table row that names `name`. */
export async function rowOf(name: string) {
  return within(await table()).findByRole("row", { name: new RegExp(name) });
}

/** headersOf lists the column headers of the table. */
export async function headersOf() {
  return within(await table())
    .getAllByRole("columnheader")
    .map((cell) => cell.textContent);
}

/** lastQuery is the query of the most recent list request, once there is one. */
export async function lastQuery(asked: readonly URLSearchParams[]) {
  await waitFor(() => expect(asked.length).toBeGreaterThan(0));
  return asked.at(-1)!;
}

export type ClassRow = components["schemas"]["AdminClassRow"];

/** classRow is an active class `name` taught by `teacher`, with no join code. */
export function classRow(n: string, name: string, teacher: string): ClassRow {
  return {
    id: `018f0000-0000-7000-8000-c${n.padStart(11, "0")}`,
    name,
    scheduleLabel: null,
    room: null,
    teacher: {
      id: `018f0000-0000-7000-8000-d${n.padStart(11, "0")}`,
      name: teacher,
      avatarUrl: null,
      disabled: false,
    },
    studentCount: 0,
    joinCode: null,
    createdAt: "2026-09-01T08:00:00Z",
  };
}

/** serveClasses answers `GET /admin/classes` from `rows`, searched by class or teacher name. */
export function serveClasses(rows: readonly ClassRow[], seen: string[] = []) {
  server.use(
    http.get(`${BASE}/admin/classes`, ({ request }) => {
      const url = new URL(request.url);
      seen.push(url.search);
      const q = (url.searchParams.get("q") ?? "").toLowerCase();
      const items = rows.filter(
        (row) =>
          q === "" ||
          row.name.toLowerCase().includes(q) ||
          row.teacher.name.toLowerCase().includes(q),
      );
      return contractJson("/admin/classes", "get", 200, {
        items,
        total: items.length,
        page: 1,
        pageSize: 10,
        facets: { active: items.length, archived: 0 },
        disabledTeacherClasses: 0,
      });
    }),
  );
}
