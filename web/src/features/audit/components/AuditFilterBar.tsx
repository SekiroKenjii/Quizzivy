import { useTranslation } from "react-i18next";
import { Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Segmented } from "@/components/ui/segmented";
import type { AuditCategory, AuditFacets, AuditFilters, AuditRange } from "../api";
import { AUDIT_CATEGORIES, isFiltered } from "../filters";
import { DateRangeMenu } from "./DateRangeMenu";

const ALL = "all";

/**
 * AuditFilterBar is the row above the table: the categories as a segmented
 * control with the count each holds, which `facets` gives and which follow
 * the search and the range and not the category, then the search, the date
 * range and, while either is set, Reset. The two groups wrap onto lines of
 * their own when they do not fit. `facets` is undefined until the first read.
 */
export function AuditFilterBar({
  filters,
  facets,
  onCategory,
  onSearch,
  onPreset,
  onCustom,
  onReset,
}: Readonly<{
  filters: AuditFilters;
  facets: AuditFacets | undefined;
  onCategory: (category: AuditCategory | null) => void;
  onSearch: (value: string) => void;
  onPreset: (range: AuditRange) => void;
  onCustom: () => void;
  onReset: () => void;
}>) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-wrap items-center justify-between gap-2.5">
      <Segmented
        label={t("audit.categories.label")}
        scroll
        value={filters.category ?? ALL}
        options={[
          { value: ALL, label: t("audit.categories.all"), count: facets?.all },
          ...AUDIT_CATEGORIES.map((category) => ({
            value: category,
            label: t(`audit.categories.${category}`),
            count: facets?.[category],
          })),
        ]}
        onChange={(value) =>
          onCategory(value === ALL ? null : (value as AuditCategory))
        }
      />
      <div className="flex min-w-0 flex-[1_1_23.75rem] items-center justify-end gap-2">
        <label className="bg-card focus-within:border-ring has-[input:focus-visible]:outline-focus flex h-9 max-w-80 min-w-37.5 flex-[1_1_16.25rem] items-center gap-2 rounded-lg border pr-1.5 pl-2.5 has-[input:focus-visible]:outline-2 has-[input:focus-visible]:outline-offset-2">
          <Search aria-hidden="true" className="text-muted-fg size-3.75 flex-none" />
          <input
            type="text"
            size={1}
            value={filters.q}
            placeholder={t("audit.search.placeholder")}
            aria-label={t("audit.search.label")}
            onChange={(event) => onSearch(event.target.value)}
            className="text-meta min-w-0 flex-1 border-0 bg-transparent outline-none"
          />
          {filters.q === "" ? null : (
            <button
              type="button"
              aria-label={t("audit.search.clear")}
              onClick={() => onSearch("")}
              className="text-muted-fg hover:bg-hover hover:text-fg grid size-6 flex-none place-items-center rounded-md"
            >
              <X aria-hidden="true" className="size-3.25" />
            </button>
          )}
        </label>
        <DateRangeMenu filters={filters} onPreset={onPreset} onCustom={onCustom} />
        {isFiltered(filters) ? (
          <Button
            type="button"
            variant="ghost"
            className="text-muted-fg hover:bg-muted hover:text-fg h-9 flex-none px-2.5 in-data-[scale=deck]:px-2.5 in-data-[scale=deck]:text-sm"
            onClick={onReset}
          >
            {t("audit.range.reset")}
          </Button>
        ) : null}
      </div>
    </div>
  );
}
