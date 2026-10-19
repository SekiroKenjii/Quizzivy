import { queryOptions } from "@tanstack/react-query";
import { api, downloadFile, uploadFile } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { components } from "@/lib/api/schema";
import { formatInTimeZone } from "date-fns-tz";
import { getDisplayTimeZone } from "@/lib/i18n/datetime";
import { saveDownloadedFile } from "@/lib/saveFile";

export type AdminUser = components["schemas"]["AdminUserRow"];
export type AdminUserRole = components["schemas"]["AdminUserRole"];
export type AdminUserTab = components["schemas"]["AdminUserTab"];
export type AdminUserFacets = components["schemas"]["AdminUserFacets"];
export type AccountStatus = components["schemas"]["AccountStatus"];

/** UserStateFilter is a state the list narrows to beside its tab, which the Overview links to. */
export type UserStateFilter = "must_change_password" | "never_signed_in";

export const USERS_KEY = ["admin", "users"] as const;

/** USER_TABS are the Users tabs in the order the deck draws them. */
export const USER_TABS: readonly AdminUserTab[] = [
  "all",
  "teachers",
  "students",
  "admins",
  "disabled",
];

/** USER_STATES are the states `?status=` can carry. */
export const USER_STATES: readonly UserStateFilter[] = [
  "must_change_password",
  "never_signed_in",
];

/** USER_PAGE_SIZES are the rows per page the list offers; the first is the default. */
export const USER_PAGE_SIZES = [10, 20, 50] as const;

export type UserPageSize = (typeof USER_PAGE_SIZES)[number];

/**
 * SEARCH_MIN is the shortest search the list sends. A shorter one has no
 * trigram to look up and reads the whole table (QC-N-15).
 */
export const SEARCH_MIN = 3;

/** SEARCH_MAX is the longest search the contract takes; the list sends no more of it. */
export const SEARCH_MAX = 200;

/** PAGE_MAX is the highest page number the contract takes. */
export const PAGE_MAX = 1_000_000;

/** UsersParams is what one page of the list asks for; `q` is empty when no search applies. */
export interface UsersParams {
  readonly tab: AdminUserTab;
  readonly status: UserStateFilter | null;
  readonly q: string;
  readonly page: number;
  readonly limit: UserPageSize;
}

/**
 * usersQuery reads `GET /admin/users`: one page of accounts of any role with
 * the five tabs' counts, which follow the search and the state but not the tab.
 */
export function usersQuery(params: UsersParams) {
  const { tab, status, q, page, limit } = params;
  return queryOptions({
    queryKey: [...USERS_KEY, "list", tab, status, q, page, limit] as const,
    queryFn: ({ signal }) =>
      api("get", "/admin/users", {
        query: {
          ...(tab === "all" ? {} : { tab }),
          ...(status === null ? {} : { status }),
          ...(q === "" ? {} : { q }),
          ...(page > 1 ? { page } : {}),
          limit,
        },
        signal,
      }),
  });
}

export type UserImportPreview = components["schemas"]["UserImportPreview"];
export type UserImportJob = components["schemas"]["UserImportJob"];
export type UserImportIssue = components["schemas"]["UserImportIssue"];
export type UserImportDelivery = components["schemas"]["UserImportDelivery"];
export type UserImportDefaultRole = components["schemas"]["UserImportDefaultRole"];

/** USER_IMPORTS_KEY is the query key of the import jobs, apart from USERS_KEY so a refresh of the list leaves a job alone. */
export const USER_IMPORTS_KEY = ["admin", "user-imports"] as const;

/** UserImportChoice is what a preview or an import is asked of: the roles and the hand-out the Admin chose. */
export interface UserImportChoice {
  readonly defaultRole: UserImportDefaultRole;
  readonly delivery: UserImportDelivery;
}

/**
 * previewUserImport reads `POST /admin/user-imports/preview`: what `file`
 * would come to under the Admin's choices. Nothing is stored.
 */
export function previewUserImport(
  file: File,
  choice: UserImportChoice,
  signal?: AbortSignal,
) {
  return uploadFile<UserImportPreview>("/admin/user-imports/preview", file, {
    fields: { defaultRole: choice.defaultRole, delivery: choice.delivery },
    ...(signal === undefined ? {} : { signal }),
  });
}

/**
 * createUserImport sends `POST /admin/user-imports`. The same `requestId`
 * returns the job the first call stored, so a retry cannot import twice.
 */
export function createUserImport(
  file: File,
  choice: UserImportChoice,
  requestId: string,
) {
  return uploadFile<UserImportJob>("/admin/user-imports", file, {
    fields: { defaultRole: choice.defaultRole, delivery: choice.delivery, requestId },
  });
}

/** userImportQuery reads `GET /admin/user-imports/{id}`, the progress of one job. */
export function userImportQuery(id: string) {
  return queryOptions({
    queryKey: [...USER_IMPORTS_KEY, id] as const,
    queryFn: ({ signal }) =>
      api("get", "/admin/user-imports/{id}", { path: { id }, signal }),
  });
}

/**
 * sheetFileName is the name a hand-out sheet is saved under when the server
 * names none: `new-accounts-YYYYMMDD.csv`, as the server writes it.
 */
export function sheetFileName(now: Date, zone = getDisplayTimeZone()) {
  return `new-accounts-${formatInTimeZone(now, zone, "yyyyMMdd")}.csv`;
}

/**
 * downloadUserImportSheet takes a job's hand-out sheet, which the server gives
 * once, and saves it. A `410 SHEET_UNAVAILABLE` says it was already taken or
 * is more than 24 hours old.
 */
export async function downloadUserImportSheet(id: string) {
  const file = await downloadFile("/admin/user-imports/{id}/sheet", undefined, {
    method: "POST",
    path: { id },
  });
  saveDownloadedFile(file, sheetFileName(new Date()));
}

export type AdminUserDetail = components["schemas"]["AdminUserDetail"];
export type AdminUserCan = components["schemas"]["AdminUserCan"];
export type AdminUserActivity = components["schemas"]["AdminUserActivity"];
export type AssignableRole = components["schemas"]["AssignableRole"];
export type UserDelivery = components["schemas"]["UserResetDelivery"];
export type UserCreate = components["schemas"]["UserCreate"];
export type UserContentTransferResult =
  components["schemas"]["UserContentTransferResult"];

/**
 * userQuery reads `GET /admin/users/{id}`: one account with what the sheet
 * draws beside it. A 4xx answer is final and not retried.
 */
export function userQuery(id: string) {
  return queryOptions({
    queryKey: [...USERS_KEY, "detail", id] as const,
    queryFn: ({ signal }) => api("get", "/admin/users/{id}", { path: { id }, signal }),
    retry: (count, error) =>
      !(error instanceof ApiError && error.status >= 400 && error.status < 500) &&
      count < 3,
  });
}

/** ASSIGNABLE_ROLES_KEY is the query key of the roles the caller may give. */
export const ASSIGNABLE_ROLES_KEY = ["admin", "roles", "assignable"] as const;

/** assignableRolesQuery reads `GET /admin/roles/assignable`, the roles the caller may give to a user. */
export function assignableRolesQuery() {
  return queryOptions({
    queryKey: ASSIGNABLE_ROLES_KEY,
    queryFn: ({ signal }) => api("get", "/admin/roles/assignable", { signal }),
  });
}

/**
 * emailTakenQuery asks `GET /admin/users?email=` whether an account already
 * uses `email`, compared as the server compares it, case-insensitively.
 */
export function emailTakenQuery(email: string) {
  return queryOptions({
    queryKey: [...USERS_KEY, "email", email.toLowerCase()] as const,
    queryFn: async ({ signal }) => {
      const page = await api("get", "/admin/users", {
        query: { email, limit: 10 },
        signal,
      });
      return page.total > 0;
    },
  });
}

/**
 * recipientsQuery lists who may receive a disabled account's content: the
 * accounts of the Teachers and the Admins tabs matching `q`, without the
 * disabled ones. The server makes the last check (`INVALID_RECIPIENT`).
 */
export function recipientsQuery(q: string) {
  return queryOptions({
    queryKey: [...USERS_KEY, "recipients", q] as const,
    queryFn: async ({ signal }) => {
      const pages = await Promise.all(
        (["teachers", "admins"] as const).map((tab) =>
          api("get", "/admin/users", {
            query: { tab, limit: 20, ...(q === "" ? {} : { q }) },
            signal,
          }),
        ),
      );
      return pages
        .flatMap((page) => page.items)
        .filter((user) => user.status !== "disabled");
    },
  });
}

/** createUser sends `POST /admin/users`; a temporary password in the answer is shown once. */
export function createUser(body: UserCreate) {
  return api("post", "/admin/users", { body });
}

/** changeUserRole sends `PUT /admin/users/{id}/role`. */
export function changeUserRole(id: string, roleId: string) {
  return api("put", "/admin/users/{id}/role", { path: { id }, body: { roleId } });
}

/** resetUserPassword sends `POST /admin/users/{id}/reset-password`. */
export function resetUserPassword(id: string, delivery: UserDelivery) {
  return api("post", "/admin/users/{id}/reset-password", {
    path: { id },
    body: { delivery },
  });
}

/** signOutUser sends `POST /admin/users/{id}/sign-out`. */
export function signOutUser(id: string) {
  return api("post", "/admin/users/{id}/sign-out", { path: { id } });
}

/** disableUser sends `POST /admin/users/{id}/disable`. */
export function disableUser(id: string) {
  return api("post", "/admin/users/{id}/disable", { path: { id } });
}

/** enableUser sends `POST /admin/users/{id}/enable`. */
export function enableUser(id: string) {
  return api("post", "/admin/users/{id}/enable", { path: { id } });
}

/** transferUserContent sends `POST /admin/users/{id}/transfer-content`. */
export function transferUserContent(id: string, toUserId: string, notify: boolean) {
  return api("post", "/admin/users/{id}/transfer-content", {
    path: { id },
    body: { toUserId, notify },
  });
}

/** UserPasswordReset is one account a reset reached, with its temporary password when one was asked for. */
export type UserPasswordReset = components["schemas"]["UserPasswordReset"];
