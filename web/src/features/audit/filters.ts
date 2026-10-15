import { formatInTimeZone } from "date-fns-tz";
import type { AuditCategory, AuditFilters, AuditRange } from "./api";

/** AUDIT_CATEGORIES lists the categories in the order the control draws them. */
export const AUDIT_CATEGORIES = [
  "accounts",
  "classes",
  "attempts",
  "tests",
  "media",
  "imports",
] as const satisfies readonly AuditCategory[];

/** AUDIT_PRESETS are the ranges the menu offers by name; `custom` takes a dialog. */
export const AUDIT_PRESETS = ["today", "7d", "30d", "all"] as const;

/** DEFAULT_RANGE is the range the list opens with, and the absence of `?range=`. */
export const DEFAULT_RANGE: AuditRange = "7d";

const DAY = /^(\d{4})-(\d{2})-(\d{2})$/;

/** isDay reports whether `value` is a real calendar day written `YYYY-MM-DD`. */
export function isDay(value: string | null): value is string {
  const parts = value === null ? null : DAY.exec(value);
  if (parts === null) return false;
  const [year, month, day] = [Number(parts[1]), Number(parts[2]), Number(parts[3])];
  const date = new Date(Date.UTC(year, month - 1, day));
  return (
    date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day
  );
}

function isCategory(value: string | null): value is AuditCategory {
  return (AUDIT_CATEGORIES as readonly (string | null)[]).includes(value);
}

function isPreset(value: string | null): value is (typeof AUDIT_PRESETS)[number] {
  return (AUDIT_PRESETS as readonly (string | null)[]).includes(value);
}

/**
 * readAuditFilters reads the filters an address holds. A value the screen
 * does not offer is dropped, and a custom range needs both of its days, the
 * second not before the first, or it is the default range.
 */
export function readAuditFilters(params: URLSearchParams): AuditFilters {
  const requested = params.get("category");
  const category = isCategory(requested) ? requested : null;
  const q = params.get("q") ?? "";
  const range = params.get("range");
  const from = params.get("from");
  const to = params.get("to");
  if (range === "custom" && isDay(from) && isDay(to) && from <= to) {
    return { category, q, range, from, to };
  }
  return {
    category,
    q,
    range: isPreset(range) ? range : DEFAULT_RANGE,
    from: null,
    to: null,
  };
}

/** isFiltered reports whether a search or a range other than the default is set: what "Reset" clears and what changes the empty sentence. */
export function isFiltered(filters: AuditFilters): boolean {
  return filters.q.trim() !== "" || filters.range !== DEFAULT_RANGE;
}

/** hasNarrowing reports whether a category or a search is set: what an export can narrow by. */
export function hasNarrowing(filters: AuditFilters): boolean {
  return filters.category !== null || filters.q.trim() !== "";
}

/**
 * lastSevenDays is the two days a custom range opens on: the six before
 * today and today, today being `now` in `zone`, the viewer's own.
 */
export function lastSevenDays(now: Date, zone: string): { from: string; to: string } {
  const to = formatInTimeZone(now, zone, "yyyy-MM-dd");
  const parts = DAY.exec(to);
  if (parts === null) return { from: to, to };
  const start = new Date(
    Date.UTC(Number(parts[1]), Number(parts[2]) - 1, Number(parts[3]) - 6),
  );
  return { from: start.toISOString().slice(0, 10), to };
}
