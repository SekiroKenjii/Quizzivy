import { contentStringLength } from "./unicode";
import { CONTENT_LIMITS } from "./model";

/** withinContentBudget bounds traversal before recursive validation; depth and visit limits reject cycles while permitting shared values. */
export function withinContentBudget(value: unknown): boolean {
  const queue = [{ value, depth: 0 }];
  let nodes = 0;
  let characters = 0;
  while (queue.length) {
    const next = queue.pop()!;
    if (next.depth > CONTENT_LIMITS.depth || ++nodes > CONTENT_LIMITS.values)
      return false;
    if (typeof next.value === "string") characters += contentStringLength(next.value);
    if (characters > CONTENT_LIMITS.strings) return false;
    if (!next.value || typeof next.value !== "object") continue;
    const values: unknown[] = Object.values(next.value);
    if (
      values.length > CONTENT_LIMITS.nodes ||
      nodes + queue.length + values.length > CONTENT_LIMITS.values
    )
      return false;
    for (const child of values) queue.push({ value: child, depth: next.depth + 1 });
  }
  return true;
}
