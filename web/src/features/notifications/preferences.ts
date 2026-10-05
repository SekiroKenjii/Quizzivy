import type { NotificationEvent, NotificationPreference } from "./api";

/** withPreference patches one switch while retaining every other channel, row and position. */
export function withPreference(
  all: readonly NotificationPreference[],
  event: NotificationEvent,
  patch: Readonly<Partial<Pick<NotificationPreference, "inApp" | "email">>>,
): NotificationPreference[] {
  return all.map((row) => (row.event === event ? { ...row, ...patch } : row));
}

/** sameRows compares ordered preference rows by their event and both channels. */
export function sameRows(
  a: readonly NotificationPreference[],
  b: readonly NotificationPreference[],
): boolean {
  return (
    a.length === b.length &&
    a.every(
      (row, index) =>
        row.event === b[index]?.event &&
        row.inApp === b[index]?.inApp &&
        row.email === b[index]?.email,
    )
  );
}
