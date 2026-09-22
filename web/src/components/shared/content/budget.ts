import { CONTENT_LIMITS } from "./model";

/** withinContentBudget bounds traversal before recursive validation; depth and visit limits reject cycles while permitting shared values. */
export function withinContentBudget(value: unknown): boolean {
  const queue = [{ value, depth: 0 }];
  let nodes = 0;
  let characters = 0;
  while (queue.length) {
    const next = queue.pop()!;
    if (next.depth > CONTENT_LIMITS.depth || ++nodes > CONTENT_LIMITS.nodes * 12)
      return false;
    if (typeof next.value === "string") characters += next.value.length;
    if (characters > CONTENT_LIMITS.text * 2) return false;
    if (!next.value || typeof next.value !== "object") continue;
    const values: unknown[] = Object.values(next.value);
    if (
      values.length > CONTENT_LIMITS.nodes ||
      nodes + queue.length + values.length > CONTENT_LIMITS.nodes * 12
    )
      return false;
    for (const child of values) queue.push({ value: child, depth: next.depth + 1 });
  }
  return true;
}
