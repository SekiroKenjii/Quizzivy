import { useQuery } from "@tanstack/react-query";
import { publicOrganizationQuery, useOrganization } from "@/lib/organization";

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
  return !useOrganization().joinCodes;
}
