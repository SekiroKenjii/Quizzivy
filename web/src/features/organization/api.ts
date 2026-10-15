import { queryOptions, useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api/client";
import { PUBLIC_ORGANIZATION_KEY, resolveOrganization } from "@/lib/organization";

/** PasswordSignIn is the centre's password sign-in switch as far as the app knows it. */
export type PasswordSignIn = "pending" | "on" | "off";

/**
 * passwordSignInQuery reads `GET /public/organization` for the screens that
 * offer a person a password: it asks the browser to revalidate, is stale after
 * thirty seconds and does not retry, so a switch an Admin just changed reaches
 * a person holding a set-password link in a moment, and a failing answer is
 * "on" at once. Its key sits under PUBLIC_ORGANIZATION_KEY, so whatever
 * invalidates the centre's answer invalidates it too.
 */
export function passwordSignInQuery() {
  return queryOptions({
    queryKey: [...PUBLIC_ORGANIZATION_KEY, "password-sign-in"] as const,
    queryFn: ({ signal }) =>
      api("get", "/public/organization", { signal, cache: "no-cache" }),
    staleTime: 30_000,
    retry: false,
    select: (answer): "on" | "off" =>
      resolveOrganization(answer).signIn.password ? "on" : "off",
  });
}

/**
 * usePasswordSignIn reads whether the centre accepts passwords. It is
 * "pending" until the first answer arrives and "on" when none does, so a
 * screen that relies on it never says the centre has switched passwords off
 * when it has not been told so; the server's refusal stays the backstop.
 */
export function usePasswordSignIn(): PasswordSignIn {
  const { data, isPending } = useQuery(passwordSignInQuery());
  if (isPending) return "pending";
  return data ?? "on";
}

/**
 * usePasswordSignInOff reports whether the centre has turned password sign-in
 * off. It is false until the answer arrives and when it fails.
 */
export function usePasswordSignInOff(): boolean {
  return usePasswordSignIn() === "off";
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
