import type { Ref } from "react";
import { useTranslation } from "react-i18next";
import { Calendar, CalendarRange, ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { auditDay } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import type { AuditFilters, AuditRange } from "../api";
import { AUDIT_PRESETS } from "../filters";

/**
 * DateRangeMenu is the "Date range" button and its menu: the four ranges the
 * log offers by name, the one in force ticked, and "Custom range…", which
 * asks the screen to open its dialog. A custom range reads as its two days.
 * `ref` is the button.
 */
export function DateRangeMenu({
  ref,
  filters,
  onPreset,
  onCustom,
}: Readonly<{
  ref?: Ref<HTMLButtonElement>;
  filters: AuditFilters;
  onPreset: (range: AuditRange) => void;
  onCustom: () => void;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const label =
    filters.range === "custom" && filters.from !== null && filters.to !== null
      ? t("audit.range.span", {
          from: auditDay(filters.from, locale),
          to: auditDay(filters.to, locale),
        })
      : t(`audit.range.${filters.range}`);
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          ref={ref}
          type="button"
          variant="outline"
          aria-label={`${t("audit.range.label")}: ${label}`}
          className="shadow-card h-9 flex-none gap-2 pr-2.5 pl-3 font-medium in-data-[scale=deck]:gap-2 in-data-[scale=deck]:pr-2.5 in-data-[scale=deck]:pl-3 in-data-[scale=deck]:text-sm"
        >
          <Calendar aria-hidden="true" className="text-muted-fg" />
          {label}
          <ChevronDown aria-hidden="true" className="text-muted-fg size-3.5" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="data-[scale=deck]:w-55">
        <DropdownMenuRadioGroup
          value={filters.range}
          onValueChange={(value) => onPreset(value as AuditRange)}
        >
          {AUDIT_PRESETS.map((preset) => (
            <DropdownMenuRadioItem key={preset} value={preset}>
              {t(`audit.range.${preset}`)}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={onCustom}>
          <CalendarRange aria-hidden="true" />
          {t("audit.range.custom")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
