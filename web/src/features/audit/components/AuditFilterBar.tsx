import { useRef, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Segmented } from "@/components/ui/segmented";
import type { AuditCategory, AuditFacets, AuditFilters, AuditRange } from "../api";
import { AUDIT_CATEGORIES, AUDIT_SEARCH_MAX, isFiltered } from "../filters";
import { DateRangeMenu } from "./DateRangeMenu";

const ALL = "all";

/**
 * AuditFilterBar is the row above the table: the categories as a segmented
 * control with the count each holds, which `facets` gives and which follow
 * the search and the range and not the category, then the search, the date
 * range and, while either is set, Reset. The two groups wrap onto lines of
 * their own when they do not fit. `facets` is undefined until the first read.
 * `query` is the text in the search field, which `onSearch` receives as it is
 * typed and `onClear` empties at once.
 * Clearing the search or pressing Reset puts focus in the search field, which
 * the control just pressed was about to leave; `rangeButton` is the ref of
 * the range button, which the screen's custom-range dialog returns focus to.
 */
export function AuditFilterBar({
  filters,
  query,
  facets,
  rangeButton,
  onCategory,
  onSearch,
  onClear,
  onPreset,
  onCustom,
  onReset,
}: Readonly<{
  filters: AuditFilters;
  query: string;
  facets: AuditFacets | undefined;
  rangeButton: RefObject<HTMLButtonElement | null>;
  onCategory: (category: AuditCategory | null) => void;
  onSearch: (value: string) => void;
  onClear: () => void;
  onPreset: (range: AuditRange) => void;
  onCustom: () => void;
  onReset: () => void;
}>) {
  const { t } = useTranslation();
  const search = useRef<HTMLInputElement>(null);
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
        <label className="bg-card focus-within:border-ring has-[input:focus-visible]:outline-focus flex h-9 max-w-80 min-w-37.5 flex-[1_1_16.25rem] items-center gap-2 rounded-md border pr-1.5 pl-2.5 has-[input:focus-visible]:outline-2 has-[input:focus-visible]:outline-offset-2">
          <Search aria-hidden="true" className="text-muted-fg size-3.75 flex-none" />
          <input
            ref={search}
            type="text"
            size={1}
            maxLength={AUDIT_SEARCH_MAX}
            value={query}
            placeholder={t("audit.search.placeholder")}
            aria-label={t("audit.search.label")}
            onChange={(event) => onSearch(event.target.value)}
            className="text-meta min-w-0 flex-1 border-0 bg-transparent outline-none"
          />
          {query === "" ? null : (
            <button
              type="button"
              aria-label={t("audit.search.clear")}
              onClick={() => {
                onClear();
                search.current?.focus();
              }}
              className="text-muted-fg hover:bg-hover hover:text-fg grid size-6 flex-none place-items-center rounded-md"
            >
              <X aria-hidden="true" className="size-3.25" />
            </button>
          )}
        </label>
        <DateRangeMenu
          ref={rangeButton}
          filters={filters}
          onPreset={onPreset}
          onCustom={onCustom}
        />
        {isFiltered(filters) ? (
          <Button
            type="button"
            variant="ghost"
            className="text-muted-fg hover:bg-muted hover:text-fg h-9 flex-none px-2.5 in-data-[scale=deck]:px-2.5 in-data-[scale=deck]:text-sm"
            onClick={() => {
              onReset();
              search.current?.focus();
            }}
          >
            {t("audit.range.reset")}
          </Button>
        ) : null}
      </div>
    </div>
  );
}
