import { queryOptions, type QueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import type { components, operations } from "@/lib/api/schema";

export type AdminClass = components["schemas"]["AdminClassRow"];
export type AdminClassFacets = components["schemas"]["AdminClassFacets"];
export type AdminClassTab = "active" | "archived";

/** ADMIN_CLASSES_KEY prefixes every query of the Admin console's Classes screen. */
export const ADMIN_CLASSES_KEY = ["admin", "classes"] as const;

/** ADMIN_CLASS_PAGE_SIZES are the rows per page the list offers; the first is the default. */
export const ADMIN_CLASS_PAGE_SIZES = [10, 20, 50] as const;

export type AdminClassPageSize = (typeof ADMIN_CLASS_PAGE_SIZES)[number];

/** ADMIN_CLASS_SEARCH_MAX is the longest search the contract takes; the list sends no more of it. */
export const ADMIN_CLASS_SEARCH_MAX = 200;

/** AdminClassesParams is what one page of the list asks for; `q` is empty when no search applies. */
export interface AdminClassesParams {
  readonly tab: AdminClassTab;
  readonly q: string;
  readonly disabledTeacher: boolean;
  readonly page: number;
  readonly limit: AdminClassPageSize;
}

/**
 * adminClassesQuery reads `GET /admin/classes`: one page of every teacher's
 * classes with their join codes in full, and the two tabs' counts. The codes
 * are bearer secrets, so the answer leaves memory with the screen.
 */
export function adminClassesQuery(params: AdminClassesParams) {
  const { tab, q, disabledTeacher, page, limit } = params;
  return queryOptions({
    queryKey: [
      ...ADMIN_CLASSES_KEY,
      "list",
      tab,
      q,
      disabledTeacher,
      page,
      limit,
    ] as const,
    queryFn: ({ signal }) =>
      api("get", "/admin/classes", {
        query: {
          ...(tab === "archived" ? { tab } : {}),
          ...(q === "" ? {} : { q }),
          ...(disabledTeacher ? { teacher: "disabled" as const } : {}),
          ...(page > 1 ? { page } : {}),
          limit,
        },
        signal,
      }),
    gcTime: 0,
  });
}

/** AdminClassCreate is the body `createClassForTeacher` sends. */
export type AdminClassCreate =
  operations["createClassForTeacher"]["requestBody"]["content"]["application/json"];

/** createClassForTeacher sends `POST /admin/classes`. */
export function createClassForTeacher(body: AdminClassCreate) {
  return api("post", "/admin/classes", { body });
}

/** changeClassTeacher sends `PUT /admin/classes/{id}/teacher`. */
export function changeClassTeacher(id: string, teacherId: string, notify: boolean) {
  return api("put", "/admin/classes/{id}/teacher", {
    path: { id },
    body: { teacherId, notify },
  });
}

/** ClassTeacherOption is an account a class can be given to. */
export type ClassTeacherOption = Readonly<{ id: string; name: string; email: string }>;

/**
 * classTeachersQuery lists who may teach a class: the active accounts of the
 * Users table's Teachers and Admins tabs. The server makes the last check
 * (`INVALID_RECIPIENT`), so a role without the teacher workspace that the
 * Teachers tab holds is refused there.
 */
export function classTeachersQuery() {
  return queryOptions({
    queryKey: [...ADMIN_CLASSES_KEY, "teachers"] as const,
    queryFn: async ({ signal }): Promise<readonly ClassTeacherOption[]> => {
      const pages = await Promise.all(
        (["teachers", "admins"] as const).map((tab) =>
          api("get", "/admin/users", { query: { tab, limit: 50 }, signal }),
        ),
      );
      return pages
        .flatMap((page) => page.items)
        .filter((user) => user.status !== "disabled")
        .map((user) => ({ id: user.id, name: user.fullName, email: user.email }));
    },
  });
}

/** invalidateAdminClasses marks every list of classes stale, the console's and the teacher's. */
export function invalidateAdminClasses(client: QueryClient) {
  return Promise.all([
    client.invalidateQueries({ queryKey: [...ADMIN_CLASSES_KEY, "list"] }),
    client.invalidateQueries({ queryKey: ["teacher-classes"] }),
  ]);
}
