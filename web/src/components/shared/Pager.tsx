import { useTranslation } from "react-i18next";
import { useLocation } from "react-router";
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import { pageHref } from "@/hooks/usePage";
import { pageCountOf, pageWindow } from "@/lib/pagination";

/**
 * Numbered pages under every listing (O-20). Nothing is drawn for a single
 * page: a control that can only ever say "1" is noise under a short list.
 */
export function Pager({
  page,
  pageSize,
  total,
}: {
  page: number;
  pageSize: number;
  total: number;
}) {
  const { t } = useTranslation();
  const { search } = useLocation();
  const pageCount = pageCountOf(total, pageSize);
  if (pageCount <= 1) return null;

  const clamped = Math.min(Math.max(page, 1), pageCount);
  return (
    <Pagination>
      <PaginationContent>
        <PaginationItem>
          <PaginationPrevious
            to={pageHref(search, clamped - 1)}
            aria-disabled={clamped === 1 || undefined}
            className={clamped === 1 ? "pointer-events-none opacity-50" : undefined}
          />
        </PaginationItem>
        {pageWindow(clamped, pageCount).map((slot, i) =>
          slot === "gap" ? (
            <PaginationItem key={`gap-${i}`}>
              <PaginationEllipsis />
            </PaginationItem>
          ) : (
            <PaginationItem key={slot}>
              <PaginationLink
                to={pageHref(search, slot)}
                isActive={slot === clamped}
                aria-label={t("pagination.page", { n: slot })}
              >
                {slot}
              </PaginationLink>
            </PaginationItem>
          ),
        )}
        <PaginationItem>
          <PaginationNext
            to={pageHref(search, clamped + 1)}
            aria-disabled={clamped === pageCount || undefined}
            className={
              clamped === pageCount ? "pointer-events-none opacity-50" : undefined
            }
          />
        </PaginationItem>
      </PaginationContent>
    </Pagination>
  );
}
