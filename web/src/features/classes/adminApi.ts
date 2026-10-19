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
 * classes with their join codes in full, and the two tabs' counts, with the
 * tab it was asked for, so a page shown while the next one loads keeps its
 * own tab's rules. The codes are bearer secrets, so the answer leaves memory
 * with the screen.
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
    queryFn: async ({ signal }) => {
      const answer = await api("get", "/admin/classes", {
        query: {
          ...(tab === "archived" ? { tab } : {}),
          ...(q === "" ? {} : { q }),
          ...(disabledTeacher ? { teacher: "disabled" as const } : {}),
          ...(page > 1 ? { page } : {}),
          limit,
        },
        signal,
      });
      return { ...answer, tab };
    },
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

const TEACHER_PAGE = 50;

async function everyUser(tab: "teachers" | "admins", signal: AbortSignal) {
  const users: components["schemas"]["AdminUserRow"][] = [];
  for (let page = 1; ; page++) {
    const answer = await api("get", "/admin/users", {
      query: { tab, limit: TEACHER_PAGE, ...(page > 1 ? { page } : {}) },
      signal,
    });
    users.push(...answer.items);
    if (answer.items.length < TEACHER_PAGE || users.length >= answer.total)
      return users;
  }
}

/**
 * classTeachersQuery lists who may teach a class: every active account of the
 * Users table's Teachers and Admins tabs, read page by page to the end, each
 * once however the pages shift between reads. The
 * server makes the last check (`INVALID_RECIPIENT`), so a role without the
 * teacher workspace that the Teachers tab holds is refused there.
 */
export function classTeachersQuery() {
  return queryOptions({
    queryKey: [...ADMIN_CLASSES_KEY, "teachers"] as const,
    queryFn: async ({ signal }): Promise<readonly ClassTeacherOption[]> => {
      const tabs = await Promise.all(
        (["teachers", "admins"] as const).map((tab) => everyUser(tab, signal)),
      );
      const byId = new Map<string, ClassTeacherOption>();
      for (const user of tabs.flat())
        if (user.status !== "disabled" && !byId.has(user.id))
          byId.set(user.id, { id: user.id, name: user.fullName, email: user.email });
      return [...byId.values()];
    },
  });
}

/**
 * teacherOptions are the choices of a teacher picker. A name two accounts
 * share carries the email after it, since the picker draws only the label.
 */
export function teacherOptions(teachers: readonly ClassTeacherOption[]) {
  const seen = new Map<string, number>();
  for (const one of teachers) seen.set(one.name, (seen.get(one.name) ?? 0) + 1);
  return teachers.map((one) => ({
    value: one.id,
    label: (seen.get(one.name) ?? 0) > 1 ? `${one.name} (${one.email})` : one.name,
  }));
}

/** invalidateAdminClasses marks every list of classes stale, the console's and the teacher's. */
export function invalidateAdminClasses(client: QueryClient) {
  return Promise.all([
    client.invalidateQueries({ queryKey: [...ADMIN_CLASSES_KEY, "list"] }),
    client.invalidateQueries({ queryKey: ["teacher-classes"] }),
  ]);
}
