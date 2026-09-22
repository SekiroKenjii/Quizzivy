import { useTranslation } from "react-i18next";
import { Checkbox } from "@/components/ui/checkbox";
import type { useBulkSelection } from "@/hooks/useBulkSelection";

export type BulkSelection<T extends { id: string }> = ReturnType<
  typeof useBulkSelection<T>
>;

export function BulkSelectAll<T extends { id: string }>({
  items,
  selection,
}: Readonly<{ items: readonly T[]; selection: BulkSelection<T> }>) {
  const { t } = useTranslation();
  const count = items.filter((item) => selection.selected.has(item.id)).length;
  const partial = count > 0 && count < items.length;
  return (
    <Checkbox
      ref={(element) => {
        if (element) element.indeterminate = partial;
      }}
      aria-checked={partial ? "mixed" : items.length > 0 && count === items.length}
      className="indeterminate:bg-primary indeterminate:border-primary indeterminate:after:bg-primary-foreground indeterminate:after:h-0.5 indeterminate:after:w-2 indeterminate:after:content-['']"
      aria-label={t("common.selectPage")}
      checked={items.length > 0 && count === items.length}
      onChange={(event) => selection.selectPage(items, event.target.checked)}
    />
  );
}

export function BulkSelectRow<T extends { id: string }>({
  item,
  name,
  selection,
}: Readonly<{ item: T; name: string; selection: BulkSelection<T> }>) {
  const { t } = useTranslation();
  return (
    <Checkbox
      aria-label={t("common.selectNamed", { name })}
      checked={selection.selected.has(item.id)}
      onChange={() => selection.toggle(item)}
    />
  );
}
