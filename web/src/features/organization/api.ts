import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from "@tanstack/react-query";
import { api, uploadFile } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import {
  PUBLIC_ORGANIZATION_KEY,
  refreshPublicOrganization,
  resolveOrganization,
} from "@/lib/organization";

/** PasswordSignIn is the centre's password sign-in switch as far as the app knows it. */
export type PasswordSignIn = "pending" | "on" | "off";

/** OrganizationSettings is the centre's settings document as `GET /admin/settings` stores it. */
export type OrganizationSettings = components["schemas"]["OrganizationSettings"];

/** OrganizationSettingsInput is the document `PUT /admin/settings` replaces it with. */
export type OrganizationSettingsInput =
  components["schemas"]["OrganizationSettingsInput"];

/** SignInPolicy is how people sign in: the two methods, the rules and the Google domains. */
export type SignInPolicy = components["schemas"]["SignInPolicy"];

/** RetentionPolicy is how long the centre keeps what it collects. */
export type RetentionPolicy = components["schemas"]["RetentionPolicy"];

/** SignInImpact is what a proposed sign-in policy would do to the active accounts. */
export type SignInImpact = components["schemas"]["SignInImpact"];

/** OrganizationLogo is a stored logo, named by the hash of its bytes. */
export type OrganizationLogo = components["schemas"]["OrganizationLogo"];

/** LogoVariant names the ground a logo is drawn for. */
export type LogoVariant = "light" | "dark";

/** ORGANIZATION_SETTINGS_KEY is the query key of `GET /admin/settings`. */
export const ORGANIZATION_SETTINGS_KEY = ["organization-settings"] as const;

/**
 * organizationSettingsQuery reads `GET /admin/settings`. The answer is never
 * kept by the browser, and the form built from it names the revision it read.
 */
export function organizationSettingsQuery() {
  return queryOptions({
    queryKey: ORGANIZATION_SETTINGS_KEY,
    queryFn: ({ signal }) => api("get", "/admin/settings", { signal }),
  });
}

/**
 * reloadOrganizationSettings reads the settings document again, whatever the
 * query holds, and makes it the query's answer.
 */
export async function reloadOrganizationSettings(
  client: QueryClient,
): Promise<OrganizationSettings> {
  const fresh = await api("get", "/admin/settings");
  client.setQueryData(ORGANIZATION_SETTINGS_KEY, fresh);
  return fresh;
}

/**
 * signInImpactQuery asks `POST /admin/settings/sign-in-impact` what `policy`
 * would do to the active accounts. It stores nothing on the server.
 */
export function signInImpactQuery(policy: SignInPolicy) {
  return queryOptions({
    queryKey: [...ORGANIZATION_SETTINGS_KEY, "sign-in-impact", policy] as const,
    queryFn: ({ signal }) =>
      api("post", "/admin/settings/sign-in-impact", {
        body: { signIn: policy },
        signal,
      }),
  });
}

/**
 * useSaveOrganizationSettings replaces the settings document. On success it
 * keeps the stored document as the query's answer and makes the front door
 * read the centre again, past the browser's five minutes.
 */
export function useSaveOrganizationSettings() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: OrganizationSettingsInput) =>
      api("put", "/admin/settings", { body: input }),
    onSuccess: (saved) => {
      client.setQueryData(ORGANIZATION_SETTINGS_KEY, saved);
      void refreshPublicOrganization(client);
    },
  });
}

/**
 * uploadOrganizationLogo stores a PNG as the logo of `variant` and answers its
 * hash. The logo applies only once a save of the settings names that hash.
 */
export function uploadOrganizationLogo(file: File, variant: LogoVariant) {
  return uploadFile<components["schemas"]["OrganizationLogoUpload"]>(
    "/admin/settings/logo",
    file,
    { fields: { variant } },
  );
}

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
