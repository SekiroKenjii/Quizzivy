import { queryOptions, useQuery, type QueryClient } from "@tanstack/react-query";
import { useMemo } from "react";

import { api, BASE_URL } from "@/lib/api/client";
import type { components } from "@/lib/api/schema";
import { config } from "@/lib/config";
import type { ResolvedTheme } from "@/lib/theme";

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
 * refreshPublicOrganization makes every reader of `GET /public/organization`
 * ask again after the centre's settings were saved. The route lets a browser
 * keep its answer for five minutes, so the first request revalidates the
 * browser's copy (`no-cache`) and stores the new answer in it; the queries
 * under PUBLIC_ORGANIZATION_KEY are then invalidated and read that copy. It
 * never rejects: a failed revalidation leaves the old answer to expire as it
 * would have.
 */
export async function refreshPublicOrganization(client: QueryClient): Promise<void> {
  await api("get", "/public/organization", { cache: "no-cache" }).catch(
    () => undefined,
  );
  await client.invalidateQueries({ queryKey: PUBLIC_ORGANIZATION_KEY });
}

/**
 * Organization is what the front door prints about the centre: each line the
 * centre's own setting, else the build value, else null, and the ways in the
 * sign-in and join pages offer. A line that is null stays hidden.
 */
export type Organization = {
  name: string | null;
  logo: { light: string | null; dark: string | null };
  frontDesk: { phone: string | null; hours: string | null };
  signIn: { google: boolean; password: boolean };
  joinCodes: boolean;
};

const PRODUCT_NAME = "Quizzivy";

function record(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null
    ? (value as Record<string, unknown>)
    : {};
}

function line(value: unknown): string | null {
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  return trimmed ? trimmed : null;
}

function flag(value: unknown): boolean {
  return typeof value === "boolean" ? value : true;
}

function logoUrl(path: unknown): string | null {
  if (typeof path !== "string" || !path.startsWith("/")) return null;
  const origin = BASE_URL.endsWith("/") ? BASE_URL.slice(0, -1) : BASE_URL;
  return `${origin}${path}`;
}

/**
 * storedLogoUrl is the address `GET /public/organization/logo` serves the
 * logo of `variant` at, named by the hash a save stored, as the front door
 * is given it.
 */
export function storedLogoUrl(variant: "light" | "dark", sha256: string): string {
  return logoUrl(`/public/organization/logo?variant=${variant}&v=${sha256}`) ?? "";
}

/**
 * resolveOrganization merges the API's answer with the build values. Without
 * an answer, every line is its build value and every way in is on, so the
 * front door never says a thing is off that it has not been told is off.
 * The answer is read member by member and never trusted to have the
 * contract's shape: a member that is missing or of another type is as good
 * as unanswered, so a proxy's page or a build ahead of the API cannot blank
 * the pages that render this. The settings row is born named after the
 * product, which the lockup above the line already prints, so that name
 * counts as unset.
 */
export function resolveOrganization(data: unknown): Organization {
  const answer = record(data);
  const logo = record(answer.logo);
  const signIn = record(answer.signIn);
  const own = line(answer.name);
  const named = own !== null && own.toLowerCase() !== PRODUCT_NAME.toLowerCase();
  return {
    name: named ? own : config.orgName,
    logo: { light: logoUrl(logo.lightUrl), dark: logoUrl(logo.darkUrl) },
    frontDesk: {
      phone: line(answer.frontDeskPhone) ?? config.frontDesk.phone,
      hours: line(answer.frontDeskHours) ?? config.frontDesk.hours,
    },
    signIn: { google: flag(signIn.google), password: flag(signIn.password) },
    joinCodes: flag(answer.joinCodes),
  };
}

/**
 * ThemedLogo is the logo to draw: its URL, and whether it is the other
 * theme's variant, drawn because the centre uploaded no logo for this theme.
 * A flipped logo is made for the opposite ground, so it needs one.
 */
export type ThemedLogo = { url: string; flipped: boolean };

/**
 * logoFor picks the logo to draw on the given theme: the centre's logo for
 * that theme, else the other one flipped, else none.
 */
export function logoFor(
  organization: Organization,
  theme: ResolvedTheme,
): ThemedLogo | null {
  const { light, dark } = organization.logo;
  const own = theme === "dark" ? dark : light;
  if (own !== null) return { url: own, flipped: false };
  const other = theme === "dark" ? light : dark;
  return other === null ? null : { url: other, flipped: true };
}

/**
 * useOrganization reads the centre for the front door. It never blocks a
 * render and never throws: while the request is out, after it fails, and
 * with the API down, the build values answer, and a cached answer is kept
 * past a failed refresh.
 */
export function useOrganization(): Organization {
  const { data } = useQuery(publicOrganizationQuery());
  return useMemo(() => resolveOrganization(data), [data]);
}
