function value(raw: string | undefined): string | null {
  const trimmed = raw?.trim();
  return trimmed ? trimmed : null;
}

/**
 * config holds the organisation's public details, set at build time until the
 * Admin console's Organization settings hold them (R5). A detail left unset
 * hides the line that shows it; front-desk details are the centre's own words
 * and are shown as written in either language.
 */
export const config = {
  orgName: value(import.meta.env["VITE_ORG_NAME"]),
  frontDesk: {
    phone: value(import.meta.env["VITE_ORG_FRONT_DESK_PHONE"]),
    hours: value(import.meta.env["VITE_ORG_FRONT_DESK_HOURS"]),
  },
} as const;
