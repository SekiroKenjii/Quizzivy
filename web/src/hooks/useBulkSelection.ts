import { useState } from "react";

/** useBulkSelection retains selected records across pages and removes only successful actions. */
export function useBulkSelection<T extends { id: string }>() {
  const [selected, setSelected] = useState<ReadonlyMap<string, T>>(new Map());
  const toggle = (item: T) =>
    setSelected((current) => {
      const next = new Map(current);
      if (!next.delete(item.id)) next.set(item.id, item);
      return next;
    });
  const selectPage = (items: readonly T[], checked: boolean) =>
    setSelected((current) => {
      const next = new Map(current);
      for (const item of items) {
        if (checked) next.set(item.id, item);
        else next.delete(item.id);
      }
      return next;
    });
  const remove = (ids: readonly string[]) =>
    setSelected((current) => {
      const next = new Map(current);
      for (const id of ids) next.delete(id);
      return next;
    });
  return { selected, toggle, selectPage, remove, clear: () => setSelected(new Map()) };
}
