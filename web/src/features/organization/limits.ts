import { z } from "zod";
import type { components } from "@/lib/api/schema";

/** UsageLimits is every usage limit an Admin set, `null` where the default applies. */
export type UsageLimits = components["schemas"]["UsageLimits"];

/** UsageLimitDefaults is what each limit is when nothing is stored, and where that comes from. */
export type UsageLimitDefaults = components["schemas"]["UsageLimitDefaults"];

/** LimitName is the name of one usage limit. */
export type LimitName = keyof UsageLimits;

/** ByteUnit is the unit a size limit is typed in. */
export type ByteUnit = "MB" | "GB";

/** ByteLimitValue is a size limit as its field holds it: the amount as typed, and its unit. */
export type ByteLimitValue = { amount: string; unit: ByteUnit };

const MIB = 1024 * 1024;

/** UNIT_BYTES is the bytes in one of each unit, binary as the rest of the app writes sizes. */
export const UNIT_BYTES: Readonly<Record<ByteUnit, number>> = {
  MB: MIB,
  GB: 1024 * MIB,
};

/** BYTE_LIMITS are the limits that bound a size, typed with a unit. */
export const BYTE_LIMITS = [
  "storageBytes",
  "mediaPerTeacherBytes",
  "importBytes",
  "importPerTeacherBytes",
  "databaseBytes",
] as const;

/** COUNT_LIMITS are the limits that bound a count, typed as a whole number. */
export const COUNT_LIMITS = [
  "seats",
  "teachers",
  "emailsPerDay",
  "emailsPerMonth",
  "warnAtPercent",
] as const;

type ByteLimit = (typeof BYTE_LIMITS)[number];
type CountLimit = (typeof COUNT_LIMITS)[number];

const COUNT_RANGE: Readonly<Record<CountLimit, readonly [number, number, string]>> = {
  seats: [1, 1_000_000, "systemSettings.errors.limitPeople"],
  teachers: [1, 1_000_000, "systemSettings.errors.limitPeople"],
  emailsPerDay: [1, 100_000, "systemSettings.errors.limitEmailsPerDay"],
  emailsPerMonth: [1, 10_000_000, "systemSettings.errors.limitEmailsPerMonth"],
  warnAtPercent: [50, 100, "systemSettings.errors.limitWarnAt"],
};

const MAX_BYTES = 1024 * 1024 * UNIT_BYTES.GB;

/** amountOf reads a typed amount, taking a comma as the decimal mark; NaN when it is not a number. */
export function amountOf(typed: string): number {
  const text = typed.trim().replace(",", ".");
  return /^\d+(\.\d+)?$/.test(text) ? Number(text) : Number.NaN;
}

/** bytesOf is a typed size in bytes, rounded to whole megabytes; NaN when it is not a size. */
export function bytesOf({ amount, unit }: ByteLimitValue): number {
  return Math.round((amountOf(amount) * UNIT_BYTES[unit]) / MIB) * MIB;
}

/** unitFor is the unit a size reads best in: gigabytes when it is a whole number of them, else megabytes. */
export function unitFor(bytes: number | null): ByteUnit {
  return bytes !== null && bytes % UNIT_BYTES.GB === 0 ? "GB" : "MB";
}

/** inUnit writes `bytes` as an amount of `unit`, without trailing zeros. */
export function inUnit(bytes: number, unit: ByteUnit): string {
  return String(Number((bytes / UNIT_BYTES[unit]).toFixed(3)));
}

const countLimit = (name: CountLimit) => {
  const [min, max, message] = COUNT_RANGE[name];
  return z
    .string()
    .transform((value) => value.trim())
    .refine((value) => {
      if (value === "") return true;
      if (!/^\d+$/.test(value)) return false;
      const n = Number(value);
      return n >= min && n <= max;
    }, message)
    .transform((value) => (value === "" ? null : Number(value)));
};

const byteLimit = z
  .object({ amount: z.string(), unit: z.enum(["MB", "GB"]) })
  .refine(
    (value) => {
      if (value.amount.trim() === "") return true;
      const bytes = bytesOf(value);
      return bytes >= MIB && bytes <= MAX_BYTES;
    },
    { message: "systemSettings.errors.limitBytes", path: ["amount"] },
  )
  .transform((value) => (value.amount.trim() === "" ? null : bytesOf(value)));

/** limitsSchema is the Plan & usage fields: a count or a size per limit, empty for the default. */
export const limitsSchema = z.object({
  seats: countLimit("seats"),
  teachers: countLimit("teachers"),
  storageBytes: byteLimit,
  mediaPerTeacherBytes: byteLimit,
  importBytes: byteLimit,
  importPerTeacherBytes: byteLimit,
  emailsPerDay: countLimit("emailsPerDay"),
  emailsPerMonth: countLimit("emailsPerMonth"),
  databaseBytes: byteLimit,
  warnAtPercent: countLimit("warnAtPercent"),
});

/** LimitsFormValues is what the Plan & usage fields hold. */
export type LimitsFormValues = z.input<typeof limitsSchema>;

function byteField(stored: number | null, fallback: number | null): ByteLimitValue {
  const unit = unitFor(stored ?? fallback);
  return { amount: stored === null ? "" : inUnit(stored, unit), unit };
}

/**
 * limitsValues is the Plan & usage fields for the stored limits: a stored
 * value as typed, and an empty field where the default applies, its unit the
 * one the default reads best in.
 */
export function limitsValues(
  limits: UsageLimits,
  defaults: UsageLimitDefaults,
): LimitsFormValues {
  const counts = Object.fromEntries(
    COUNT_LIMITS.map((name) => [
      name,
      limits[name] === null ? "" : String(limits[name]),
    ]),
  ) as Record<CountLimit, string>;
  const sizes = Object.fromEntries(
    BYTE_LIMITS.map((name) => [name, byteField(limits[name], defaults[name].value)]),
  ) as Record<ByteLimit, ByteLimitValue>;
  return { ...counts, ...sizes };
}

/** isByteLimit reports whether `name` bounds a size. */
export function isByteLimit(name: LimitName): name is ByteLimit {
  return (BYTE_LIMITS as readonly string[]).includes(name);
}

/** typedLimit is the value a limit's field holds now, `null` when it is empty or not yet valid. */
export function typedLimit(value: string | ByteLimitValue | undefined): number | null {
  if (value === undefined) return null;
  if (typeof value === "string") {
    const n = amountOf(value);
    return Number.isInteger(n) && n > 0 ? n : null;
  }
  if (value.amount.trim() === "") return null;
  const bytes = bytesOf(value);
  return Number.isFinite(bytes) && bytes > 0 ? bytes : null;
}
