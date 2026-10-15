import { queryOptions, useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";

/** PublicOrganization is what `GET /public/organization` says about the centre to anyone. */
export type PublicOrganization = components["schemas"]["PublicOrganization"];

/** PUBLIC_ORGANIZATION_KEY is the query key of `GET /public/organization`. */
export const PUBLIC_ORGANIZATION_KEY = ["public-organization"] as const;

/**
 * publicOrganizationQuery reads `GET /public/organization`. The route lets a
 * browser keep the answer for five minutes, and so does the query.
 */
export function publicOrganizationQuery() {
  return queryOptions({
    queryKey: PUBLIC_ORGANIZATION_KEY,
    queryFn: ({ signal }) => api("get", "/public/organization", { signal }),
    staleTime: 300_000,
  });
}

/**
 * joinCodesQuery reads the same route for a teacher's screens, which must not
 * lag the centre's setting by minutes: it asks the browser to revalidate, and
 * is stale after thirty seconds. Its key sits under PUBLIC_ORGANIZATION_KEY,
 * so whatever invalidates the centre's answer invalidates it too.
 */
export function joinCodesQuery() {
  return queryOptions({
    queryKey: [...PUBLIC_ORGANIZATION_KEY, "join-codes"] as const,
    queryFn: ({ signal }) =>
      api("get", "/public/organization", { signal, cache: "no-cache" }),
    staleTime: 30_000,
    select: (organization) => !organization.joinCodes,
  });
}

/**
 * useJoinCodesOff reports whether the centre has turned joining with a code
 * off. It is false until the answer arrives and when it fails, so a screen
 * never says the centre has switched a thing off that it has not.
 */
export function useJoinCodesOff(): boolean {
  const { data } = useQuery(joinCodesQuery());
  return data === true;
}
