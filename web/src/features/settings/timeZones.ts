import type { TFunction } from "i18next";
import type { FormOption } from "@/components/shared/form/fields/types";

/**
 * TIME_ZONES is the curated list Settings offers, each IANA zone with the
 * key of its city's name under `settings.zones`. Ho Chi Minh City comes
 * first, the rest run east from it and then west.
 */
export const TIME_ZONES = [
  { zone: "Asia/Ho_Chi_Minh", key: "hoChiMinh" },
  { zone: "Asia/Bangkok", key: "bangkok" },
  { zone: "Asia/Jakarta", key: "jakarta" },
  { zone: "Asia/Kuala_Lumpur", key: "kualaLumpur" },
  { zone: "Asia/Singapore", key: "singapore" },
  { zone: "Asia/Manila", key: "manila" },
  { zone: "Asia/Shanghai", key: "shanghai" },
  { zone: "Asia/Hong_Kong", key: "hongKong" },
  { zone: "Asia/Taipei", key: "taipei" },
  { zone: "Asia/Seoul", key: "seoul" },
  { zone: "Asia/Tokyo", key: "tokyo" },
  { zone: "Australia/Sydney", key: "sydney" },
  { zone: "Europe/London", key: "london" },
  { zone: "Europe/Paris", key: "paris" },
  { zone: "Europe/Berlin", key: "berlin" },
  { zone: "America/New_York", key: "newYork" },
  { zone: "America/Los_Angeles", key: "losAngeles" },
  { zone: "UTC", key: "utc" },
] as const;

/**
 * zoneOffset is `zone`'s offset from UTC at `at` as the deck writes it,
 * "GMT+7", or null when the browser does not know the zone.
 */
export function zoneOffset(zone: string, at: Date): string | null {
  try {
    const parts = new Intl.DateTimeFormat("en", {
      timeZone: zone,
      timeZoneName: "shortOffset",
    }).formatToParts(at);
    return parts.find((part) => part.type === "timeZoneName")?.value ?? null;
  } catch {
    return null;
  }
}

/**
 * timeZoneOptions are the Time zone field's options, "(GMT+7) Ho Chi Minh
 * City", at `at`. A `current` zone the list does not hold is added at the
 * end under its own id, so the field always shows the account's zone.
 */
export function timeZoneOptions(t: TFunction, current: string, at: Date): FormOption[] {
  const label = (zone: string, name: string) => {
    const offset = zoneOffset(zone, at);
    return offset === null ? name : `(${offset}) ${name}`;
  };
  const options: FormOption[] = TIME_ZONES.map(({ zone, key }) => ({
    value: zone,
    label: label(zone, t(`settings.zones.${key}`)),
  }));
  if (current !== "" && !options.some((option) => option.value === current)) {
    options.push({ value: current, label: label(current, current) });
  }
  return options;
}
