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
 * useKnownPasswordSignInOff reports whether the centre has turned password
 * sign-in off, from an answer the app already holds. It never fetches, so it
 * is false while nothing has asked, and a screen that relies on it still
 * shows the server's refusal when the switch is off.
 */
export function useKnownPasswordSignInOff(): boolean {
  const { data } = useQuery({
    ...publicOrganizationQuery(),
    enabled: false,
    select: (organization) => !organization.signIn.password,
  });
  return data === true;
}

/**
 * useJoinCodesOff reports whether the centre has turned joining with a code
 * off. It is false until the answer arrives and when it fails, so a screen
 * never says the centre has switched a thing off that it has not.
 */
export function useJoinCodesOff(): boolean {
  const { data } = useQuery({
    ...publicOrganizationQuery(),
    select: (organization) => !organization.joinCodes,
  });
  return data === true;
}
