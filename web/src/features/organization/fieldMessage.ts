import type { TFunction } from "i18next";

type Shown = { type?: unknown; message?: unknown; root?: unknown };

function first(error: unknown): Shown | undefined {
  if (error === null || typeof error !== "object") return undefined;
  const found = error as Shown;
  if (typeof found.message === "string" && found.message !== "") return found;
  if (found.root !== undefined) return first(found.root);
  for (const item of Object.values(error)) {
    const inner = first(item);
    if (inner !== undefined) return inner;
  }
  return undefined;
}

/**
 * fieldMessage is the sentence to show for a field's validation error: the
 * server's own words when the error came from a save it refused (`type`
 * "server"), else the translation of the message key the schema gave. It
 * looks inside the errors of an array's items and takes the first.
 */
export function fieldMessage(error: unknown, t: TFunction): string | undefined {
  const found = first(error);
  if (found === undefined || typeof found.message !== "string") return undefined;
  return found.type === "server" ? found.message : t(found.message);
}
