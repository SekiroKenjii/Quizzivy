/** Edit is one field of the form and the value it holds. */
export type Edit = Readonly<{ path: string; value: unknown }>;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * editsSince lists the fields of `now` that differ from `sent`, the form as it
 * stood when a save was submitted. A field is a leaf: a string, a switch, a
 * month count, a list of domains or a null logo; a logo that was set is
 * compared by its own fields. The list lets an edit made while the save was
 * in flight be put back over the document the save returned.
 */
export function editsSince(sent: unknown, now: unknown, path = ""): Edit[] {
  if (isRecord(sent) && isRecord(now)) {
    const keys = new Set([...Object.keys(sent), ...Object.keys(now)]);
    return [...keys].flatMap((key) =>
      editsSince(sent[key], now[key], path === "" ? key : `${path}.${key}`),
    );
  }
  return JSON.stringify(sent) === JSON.stringify(now) ? [] : [{ path, value: now }];
}
