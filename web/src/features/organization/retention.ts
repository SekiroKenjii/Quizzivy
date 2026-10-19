import type { RetentionPolicy } from "@/features/organization/api";

/** FOREVER is the select value of a retention that never ends. */
export const FOREVER = "forever";

/** retentionValue is the select value of a retention in months, `null` being never. */
export function retentionValue(months: number | null): string {
  return months === null ? FOREVER : String(months);
}

/** retentionMonths is the months a select value names, `null` being never. */
export function retentionMonths(value: string): number | null {
  return value === FOREVER ? null : Number(value);
}

function span(months: number | null): number {
  return months === null ? Number.POSITIVE_INFINITY : months;
}

/**
 * retentionShortened reports whether `next` keeps anything for less time than
 * `saved` does, which is what the nightly sweep can delete the first night.
 */
export function retentionShortened(saved: RetentionPolicy, next: RetentionPolicy) {
  return (
    span(next.keepAnswersMonths) < span(saved.keepAnswersMonths) ||
    span(next.keepFocusEventsMonths) < span(saved.keepFocusEventsMonths) ||
    span(next.anonymiseDisabledAfterMonths) < span(saved.anonymiseDisabledAfterMonths)
  );
}
