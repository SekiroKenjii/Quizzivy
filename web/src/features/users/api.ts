import { queryOptions } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

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
