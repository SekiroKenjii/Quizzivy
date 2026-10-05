import { useId, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation } from "react-router";
import {
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  type LucideIcon,
} from "lucide-react";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { PAGE_SIZES, pageHref, usePageSize } from "@/hooks/usePage";
import { useLocale } from "@/lib/i18n/useLocale";
import { pageRange } from "@/lib/pagination";
import { cn } from "@/lib/utils";

/**
 * PagerProps is what a list screen passes to Pager. `page`, `pageSize` and
 * `total` are what the API answered. `noun` returns the screen's noun for the
 * rows in the plural that fits `count`; it is called with the total, and with
 * 0 for an empty list. `sizes` is the rows-per-page choice, `PAGE_SIZES` when
 * absent, and is the list the screen gives `usePageSize`. `frame` is `"card"`
 * for the bar inside a card and `"plain"` for the bar under a list that is not
 * one card.
 */
export type PagerProps = Readonly<{
  page: number;
  pageSize: number;
  total: number;
  noun: (count: number) => string;
  sizes?: readonly number[] | undefined;
  frame?: "card" | "plain" | undefined;
}>;

const BAR = "text-muted-fg flex flex-wrap items-center justify-between gap-3 text-sm";

const FRAMES = {
  card: "sticky left-0 border-t px-4 py-2.5",
  plain: "px-0.5 py-1",
} as const;

const SIZE_TRIGGER =
  "border-border bg-card text-fg rounded-seg dark:bg-card dark:hover:bg-card in-data-[scale=deck]:bg-card dark:in-data-[scale=deck]:bg-card dark:in-data-[scale=deck]:hover:bg-card in-data-[scale=deck]:text-sm gap-1.75 px-2.5 py-0 pr-2 font-normal shadow-none [&_svg]:size-3.25";

const ARROW =
  "bg-card text-fg hover:bg-muted rounded-seg grid size-8 flex-none place-items-center border";

const ARROW_OFF = "pointer-events-none opacity-45";

function Arrow({
  to,
  label,
  icon: Icon,
  off,
}: Readonly<{ to: string; label: string; icon: LucideIcon; off: boolean }>) {
  return (
    <Link
      to={to}
      aria-label={label}
      aria-disabled={off || undefined}
      className={cn(ARROW, off && ARROW_OFF)}
    >
      <Icon aria-hidden="true" className="size-3.75" />
    </Link>
  );
}

/**
 * Pager is the bar under a list on a deck surface: the range of rows shown
 * ("1–10 of 312 assignments"), the rows per page, "Page 1 of 32" and four
 * arrows to the first, previous, next and last page. It is drawn for one page
 * and for an empty list too, where it reads "No assignments" and "Page 1 of
 * 1". The figures are grouped for the current locale. The arrows are links
 * that keep every other search parameter; one that cannot move points at the
 * current page, is `aria-disabled` and takes no pointer. The rows-per-page
 * select shows the size in `?size=` and writes it through `usePageSize`,
 * which goes back to page 1. Below 768px the select and the first and last
 * arrows are not drawn. `frame` is the screen's choice at every width: the
 * card bar has a top border and stays at the left edge of a table that
 * scrolls sideways, the plain bar has neither. It renders inside a router.
 */
export function Pager({
  page,
  pageSize,
  total,
  noun,
  sizes = PAGE_SIZES,
  frame = "card",
}: PagerProps) {
  const { t } = useTranslation();
  const locale = useLocale();
  const { search } = useLocation();
  const wide = useMediaQuery("(min-width: 768px)");
  const [size, setSize] = usePageSize(sizes);
  const labelId = useId();
  const figure = useMemo(() => new Intl.NumberFormat(locale), [locale]);
  const range = pageRange(page, pageSize, total);
  const first = range.page <= 1;
  const last = range.page >= range.pages;

  return (
    <nav
      aria-label={t("pager.label")}
      data-slot="pager"
      className={cn(BAR, FRAMES[frame])}
    >
      <span className="whitespace-nowrap tabular-nums">
        {range.to === 0
          ? t("pager.none", { noun: noun(0) })
          : t("pager.range", {
              from: figure.format(range.from),
              to: figure.format(range.to),
              total: figure.format(total),
              noun: noun(total),
            })}
      </span>
      <div className="flex flex-wrap items-center gap-4">
        {wide ? (
          <div className="flex items-center gap-2 whitespace-nowrap">
            <span id={labelId} className="text-fg font-medium">
              {t("pager.rowsPerPage")}
            </span>
            <Select
              value={String(size)}
              onValueChange={(value) => setSize(Number(value))}
            >
              <SelectTrigger
                size="sm"
                aria-labelledby={labelId}
                className={SIZE_TRIGGER}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent align="end">
                <SelectGroup>
                  {sizes.map((option) => (
                    <SelectItem key={option} value={String(option)}>
                      {option}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
        ) : null}
        <span className="text-fg font-medium whitespace-nowrap tabular-nums">
          {t("pager.pageOf", {
            page: figure.format(range.page),
            pages: figure.format(range.pages),
          })}
        </span>
        <div className="flex gap-1.5">
          {wide ? (
            <Arrow
              to={pageHref(search, 1)}
              label={t("pager.first")}
              icon={ChevronsLeft}
              off={first}
            />
          ) : null}
          <Arrow
            to={pageHref(search, Math.max(1, range.page - 1))}
            label={t("pager.previous")}
            icon={ChevronLeft}
            off={first}
          />
          <Arrow
            to={pageHref(search, Math.min(range.pages, range.page + 1))}
            label={t("pager.next")}
            icon={ChevronRight}
            off={last}
          />
          {wide ? (
            <Arrow
              to={pageHref(search, range.pages)}
              label={t("pager.last")}
              icon={ChevronsRight}
              off={last}
            />
          ) : null}
        </div>
      </div>
    </nav>
  );
}
