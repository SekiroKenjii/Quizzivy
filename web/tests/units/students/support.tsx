import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { http } from "msw";
import { Toaster } from "@/components/ui/sonner";
import StudentsPage from "@/features/students/pages/teacher/StudentsPage";
import type { components } from "@/lib/api/schema";
import { useAuthStore } from "@/stores/auth";
import { contractJson } from "@tests/support/contractResponse";
import { contentWidth } from "@tests/support/contentWidth";
import { server } from "@tests/support/server";

/** BASE is the API origin the tests' handlers answer on. */
export const BASE = "http://localhost:8080";

/** StudentRow is one row of `GET /teacher/students`. */
export type StudentRow = components["schemas"]["StudentRow"];

/** CLASS_A and CLASS_B are two classes the signed-in teacher teaches. */
export const CLASS_A = {
  id: "018f0000-0000-7000-8000-0000000000a1",
  name: "IELTS 6.5 Tối",
};
export const CLASS_B = {
  id: "018f0000-0000-7000-8000-0000000000a2",
  name: "IELTS Nền tảng A",
};

/** student builds a StudentRow with the given name and overrides. */
export function student(
  id: string,
  fullName: string,
  overrides: Partial<StudentRow> = {},
): StudentRow {
  const base = {
    id,
    email: `${id.slice(-4)}@example.com`,
    fullName,
    hasPassword: true,
    linkedProviders: [],
    mustChangePassword: false,
    createdAt: "2026-01-01T00:00:00Z",
    disabledAt: null,
    lastActiveAt: null,
    classes: [],
    stats: {
      submittedCount: 0,
      flaggedCount: 0,
      activity: { live: false, lastAttemptAt: null },
    },
    ...overrides,
  };
  return { accountStatus: statusOfFixture(base), ...base };
}

function statusOfFixture(row: {
  disabledAt: string | null;
  mustChangePassword: boolean;
}): StudentRow["accountStatus"] {
  if (row.disabledAt !== null) return "disabled";
  if (row.mustChangePassword) return "must_change_password";
  return "active";
}

/** inClass is a membership of `klass` for a StudentRow's `classes`. */
export function inClass(klass: { id: string; name: string }) {
  return {
    id: klass.id,
    name: klass.name,
    joinedVia: "admin" as const,
    joinedAt: "2026-02-01T00:00:00Z",
  };
}

function classRow(klass: { id: string; name: string }) {
  return {
    id: klass.id,
    name: klass.name,
    studentCount: 1,
    openAssignmentCount: 0,
    selfJoinEnabled: false,
    archivedAt: null,
    averageScore: null,
    createdAt: "2026-01-01T00:00:00Z",
  };
}

/**
 * serveStudents answers the list with `rows`, filtered by `q` and `classId`
 * as the server would, the one-student read, and the teacher's two classes,
 * and records every list request's query.
 */
export function serveStudents(rows: readonly StudentRow[]) {
  const asked: URLSearchParams[] = [];
  server.use(
    http.get(`${BASE}/teacher/students`, ({ request }) => {
      const params = new URL(request.url).searchParams;
      asked.push(params);
      const q = params.get("q") ?? "";
      const items = rows.filter((row) => row.fullName.includes(q));
      return contractJson("/teacher/students", "get", 200, {
        items,
        page: 1,
        pageSize: 20,
        total: items.length,
        facets: { total: rows.length, activeLast7Days: 0 },
      });
    }),
    http.get(`${BASE}/teacher/students/:id`, ({ params }) =>
      contractJson(
        "/teacher/students/{id}",
        "get",
        200,
        rows.find((row) => row.id === params["id"]) ?? rows[0],
      ),
    ),
    http.get(`${BASE}/teacher/classes`, () =>
      contractJson("/teacher/classes", "get", 200, {
        items: [classRow(CLASS_A), classRow(CLASS_B)],
        page: 1,
        pageSize: 100,
        total: 2,
        facets: { all: 2, joinable: 0, archived: 0, students: 2 },
      }),
    ),
  );
  return asked;
}

/** renderStudents signs `user` in and renders the Students page at `path`. */
export function renderStudents(
  user: components["schemas"]["CurrentUser"],
  path = "/teacher/students",
) {
  useAuthStore.getState().setSession("token", user);
  contentWidth(1280);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const router = createMemoryRouter(
    [{ path: "/teacher/students", element: <StudentsPage /> }],
    { initialEntries: [path] },
  );
  const view = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
      <Toaster />
    </QueryClientProvider>,
  );
  return { user: userEvent.setup(), router, client, unmount: view.unmount };
}

/** table is the students table once it has rows. */
export async function table() {
  return screen.findByRole("table", { name: "Học viên" });
}

/** rowOf is the table row that names `name`. */
export async function rowOf(name: string) {
  return within(await table()).findByRole("row", { name: new RegExp(name) });
}

/** menuOf opens the row menu of `name` and returns the menu. */
export async function menuOf(user: ReturnType<typeof userEvent.setup>, name: string) {
  const row = await rowOf(name);
  await user.click(within(row).getByRole("button", { name: `Thao tác với ${name}` }));
  return screen.findByRole("menu");
}
