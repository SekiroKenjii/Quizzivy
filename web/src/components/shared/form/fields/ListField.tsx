import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, Search } from "lucide-react";
import { cn } from "@/lib/utils";
import { matchesOption } from "./rules";
import type { FieldControlProps, FormOption } from "./types";
/** ListField filters and toggles named choices while keeping its search outside form values. */
export function ListField({
  id,
  label,
  value,
  onChange,
  invalid: _invalid,
  describedBy,
  disabled,
  options,
  search = true,
  searchPlaceholder,
}: FieldControlProps<readonly string[]> &
  Readonly<{
    options: readonly FormOption[];
    search?: boolean | undefined;
    searchPlaceholder?: string | undefined;
  }>) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const chosen = new Set(value);
  const hasSearch = search && options.length > 5;
  const shown = options.filter((option) =>
    matchesOption(option, hasSearch ? query : ""),
  );
  return (
    <>
      {hasSearch && (
        <label
          className={cn(
            "border-border bg-bg flex h-9 items-center gap-2 rounded-md border px-2.5",
          )}
        >
          <Search aria-hidden="true" className="text-muted-fg size-3.75" />
          <input
            aria-label={searchPlaceholder ?? t("formDialog.search")}
            placeholder={searchPlaceholder ?? t("formDialog.search")}
            size={1}
            value={query}
            disabled={disabled}
            onChange={(event) => setQuery(event.target.value)}
            className="text-ui min-w-0 flex-1 bg-transparent outline-none"
          />
          {value.length > 0 && (
            <span className="text-muted-fg text-xs whitespace-nowrap">
              {t("formDialog.selected", { count: value.length })}
            </span>
          )}
        </label>
      )}
      <div
        id={id}
        role="group"
        aria-label={label}
        aria-describedby={describedBy}
        className="border-border flex max-h-75 flex-col overflow-y-auto rounded-lg border"
      >
        {shown.length === 0 && (
          <p className="text-muted-fg px-3 py-4.5 text-center text-sm">
            {t("formDialog.noMatch", { query })}
          </p>
        )}
        {shown.map((option) => {
          const on = chosen.has(option.value);
          return (
            <button
              key={option.value}
              type="button"
              role="checkbox"
              aria-checked={on}
              aria-describedby={describedBy}
              disabled={disabled}
              className={cn(
                "hover:bg-muted flex items-center gap-2.5 border-t px-3 py-2.5 text-left first:border-t-0",
                on && "bg-muted",
              )}
              onClick={() =>
                onChange(
                  on
                    ? value.filter((v) => v !== option.value)
                    : [...value, option.value],
                )
              }
            >
              <span
                aria-hidden="true"
                className={cn(
                  "border-ring bg-card grid size-4 flex-none place-items-center rounded-sm border",
                  on && "border-primary bg-primary text-primary-fg",
                )}
              >
                {on && <Check className="size-2.75" />}
              </span>
              <span className="min-w-0 flex-1">
                <span className="text-ui block truncate">{option.label}</span>
                {option.meta && (
                  <span className="text-muted-fg block text-xs">{option.meta}</span>
                )}
              </span>
            </button>
          );
        })}
      </div>
    </>
  );
}
