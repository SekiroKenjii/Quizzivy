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

/** PagerProps supplies a list's figures, size choices and frame with optional hash-preserving navigation. */
export type PagerProps = Readonly<{
  page: number;
  pageSize: number;
  total: number;
  noun: (count: number) => string;
  sizes?: readonly number[] | undefined;
  frame?: "card" | "plain" | undefined;
  preserveHash?: boolean | undefined;
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

/** Pager renders the responsive range, size choice and page links with hash preservation only when requested. */
export function Pager({
  page,
  pageSize,
  total,
  noun,
  sizes = PAGE_SIZES,
  frame = "card",
  preserveHash = false,
}: PagerProps) {
  const { t } = useTranslation();
  const locale = useLocale();
  const { pathname, search, hash } = useLocation();
  const wide = useMediaQuery("(min-width: 768px)");
  const [size, setSize] = usePageSize(sizes, preserveHash);
  const labelId = useId();
  const figure = useMemo(() => new Intl.NumberFormat(locale), [locale]);
  const range = pageRange(page, pageSize, total);
  const first = range.page <= 1;
  const last = range.page >= range.pages;
  const href = (next: number) =>
    preserveHash
      ? `${pathname}${pageHref(search, next)}${hash}`
      : pageHref(search, next);

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
              to={href(1)}
              label={t("pager.first")}
              icon={ChevronsLeft}
              off={first}
            />
          ) : null}
          <Arrow
            to={href(Math.max(1, range.page - 1))}
            label={t("pager.previous")}
            icon={ChevronLeft}
            off={first}
          />
          <Arrow
            to={href(Math.min(range.pages, range.page + 1))}
            label={t("pager.next")}
            icon={ChevronRight}
            off={last}
          />
          {wide ? (
            <Arrow
              to={href(range.pages)}
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
