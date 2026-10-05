import { CirclePlus, X } from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemText,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

/** FacetOption names a selectable value in a facet menu. */
export type FacetOption = Readonly<{ value: string; label: string }>;

/** FacetFilterProps controls selections, including values absent from the current options. */
export type FacetFilterProps = Readonly<{
  label: string;
  title?: string;
  options: readonly FacetOption[];
  selected: readonly string[];
  onChange: (next: readonly string[]) => void;
  clearable?: boolean;
  disabled?: boolean;
  align?: "start" | "end";
  menuClassName?: string;
}>;

/** FacetFilter toggles controlled selections without closing its check menu and closes when cleared. */
export function FacetFilter({
  label,
  title,
  options,
  selected,
  onChange,
  clearable = true,
  disabled = false,
  align = "end",
  menuClassName,
}: FacetFilterProps) {
  const { t } = useTranslation();
  const chosen = new Set(selected);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild disabled={disabled}>
        <button
          type="button"
          disabled={disabled}
          aria-label={
            selected.length
              ? t("facet.active", { label, count: selected.length })
              : label
          }
          className="bg-card text-fg hover:bg-muted inline-flex h-9 items-center gap-1.5 rounded-md border border-dashed px-3 text-sm leading-4 font-normal disabled:pointer-events-none disabled:opacity-50 in-data-[scale=deck]:h-8.5 in-data-[scale=deck]:rounded-[0.5rem]"
        >
          <CirclePlus aria-hidden="true" className="text-muted-fg size-3.5 shrink-0" />
          <span>{label}</span>
          {selected.length > 0 && (
            <span
              aria-hidden="true"
              className="bg-primary text-primary-fg text-2xs inline-flex h-4.5 min-w-4.5 items-center justify-center rounded-full px-1.25 leading-none font-semibold tabular-nums"
            >
              {selected.length}
            </span>
          )}
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align={align}
        className={cn("data-[scale=deck]:w-60", menuClassName)}
      >
        {title && <DropdownMenuLabel>{title}</DropdownMenuLabel>}
        {options.length ? (
          options.map((option) => (
            <DropdownMenuCheckboxItem
              key={option.value}
              checked={chosen.has(option.value)}
              onCheckedChange={() =>
                onChange(
                  chosen.has(option.value)
                    ? selected.filter((value) => value !== option.value)
                    : [...selected, option.value],
                )
              }
            >
              <DropdownMenuItemText>{option.label}</DropdownMenuItemText>
            </DropdownMenuCheckboxItem>
          ))
        ) : (
          <DropdownMenuLabel>{t("facet.none")}</DropdownMenuLabel>
        )}
        {clearable && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              disabled={selected.length === 0}
              onSelect={() => onChange([])}
            >
              <X aria-hidden="true" />
              <DropdownMenuItemText>{t("facet.clear")}</DropdownMenuItemText>
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
