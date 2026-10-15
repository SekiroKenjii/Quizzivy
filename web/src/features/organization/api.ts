import { queryOptions, useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import {
  PUBLIC_ORGANIZATION_KEY,
  publicOrganizationQuery,
  resolveOrganization,
} from "@/lib/organization";

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
 * joinCodesQuery reads `GET /public/organization` for a teacher's screens,
 * which must not lag the centre's setting by minutes: it asks the browser to
 * revalidate, and is stale after thirty seconds. Its key sits under
 * PUBLIC_ORGANIZATION_KEY, so whatever invalidates the centre's answer
 * invalidates it too. It revalidates when the window is focused again, so a
 * tab left open follows the setting, and it answers true when joining by code
 * is off.
 */
export function joinCodesQuery() {
  return queryOptions({
    queryKey: [...PUBLIC_ORGANIZATION_KEY, "join-codes"] as const,
    queryFn: ({ signal }) =>
      api("get", "/public/organization", { signal, cache: "no-cache" }),
    staleTime: 30_000,
    refetchOnWindowFocus: true,
    select: (answer) => !resolveOrganization(answer).joinCodes,
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
