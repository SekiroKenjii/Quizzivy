import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQuery } from "@tanstack/react-query";
import { Download } from "lucide-react";
import { Pager } from "@/components/shared/data/Pager";
import { Callout } from "@/components/shared/Callout";
import { QueryStates } from "@/components/shared/ListState";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { trimmed, useDebouncedInput } from "@/hooks/useDebouncedInput";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage } from "@/hooks/usePage";
import { PageHead } from "@/layouts/shell/PageHead";
import { useDisplayTimeZone } from "@/lib/i18n/datetime";
import {
  AUDIT_PAGE_SIZE,
  auditLogQuery,
  type AuditCategory,
  type AuditRange,
} from "../../api";
import { AuditFilterBar } from "../../components/AuditFilterBar";
import { AuditTable } from "../../components/AuditTable";
import { CustomRangeDialog } from "../../components/CustomRangeDialog";
import { ExportAuditDialog } from "../../components/ExportAuditDialog";
import {
  DEFAULT_RANGE,
  isFiltered,
  lastSevenDays,
  readAuditFilters,
} from "../../filters";

const SIZES = [AUDIT_PAGE_SIZE] as const;
const NO_ROWS: ReadonlySet<number> = new Set();
const NO_DAYS = { from: "", to: "" } as const;

function TableSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-label={t("common.loading")}
      className="bg-card shadow-card overflow-hidden rounded-xl border"
    >
      <div className="bg-muted h-10" />
      {Array.from({ length: 6 }, (_, index) => (
        <div key={index} className="flex h-12.5 items-center gap-3 border-t px-4">
          <Skeleton className="size-6.5 rounded-full" />
          <Skeleton className="h-3.5 w-40" />
          <Skeleton className="h-3.5 flex-1" />
        </div>
      ))}
    </div>
  );
}

/**
 * AuditLogPage is the Admin console's audit log: the append-only record of
 * sensitive changes, ten entries a page, newest first, filtered by category,
 * search and date range (all kept in the address, the search once typing
 * pauses for 300 ms) and exported as a file.
 * A row opens to show what it changed. A page past the last one the log has
 * is replaced in the address by the last. When a read that failed succeeds
 * and focus was lost with the Retry button, focus moves to the list.
 */
export default function AuditLogPage() {
  const { t } = useTranslation();
  const zone = useDisplayTimeZone();
  const { params, setParams, setFilter } = useListFilters();
  const filters = useMemo(() => readAuditFilters(params), [params]);
  const [query, setQuery] = useDebouncedInput(
    filters.q,
    (value) => setFilter("q", value),
    trimmed,
  );
  const search = filters.q.trim();
  const asked = useMemo(() => ({ ...filters, q: search }), [filters, search]);
  const [page, setPage] = usePage(JSON.stringify(asked));
  const list = useQuery(auditLogQuery(asked, page));
  const rangeButton = useRef<HTMLButtonElement>(null);
  const region = useRef<HTMLDivElement>(null);
  const failed = list.isError;
  const shown = list.data !== undefined && !failed;
  const recovering = useRef(false);
  useEffect(() => {
    if (failed) {
      recovering.current = true;
      return;
    }
    if (!shown || !recovering.current) return;
    recovering.current = false;
    if (document.activeElement === null || document.activeElement === document.body)
      region.current?.focus();
  }, [failed, shown]);
  const total = list.data?.total;
  const pageSize = list.data?.pageSize;
  const stale = list.isPlaceholderData;
  useEffect(() => {
    if (total === undefined || pageSize === undefined || stale) return;
    const last = Math.max(1, Math.ceil(total / pageSize));
    if (page > last) setPage(last);
  }, [total, pageSize, stale, page, setPage]);
  const [open, setOpen] = useState<ReadonlySet<number>>(NO_ROWS);
  const [choosing, setChoosing] = useState<{ from: string; to: string } | null>(null);
  const [exporting, setExporting] = useState(false);

  const toggle = useCallback((id: number) => {
    setOpen((current) => {
      const next = new Set(current);
      if (!next.delete(id)) next.add(id);
      return next;
    });
  }, []);
  const setRange = (range: AuditRange, from: string | null, to: string | null) =>
    setParams(
      (current) => {
        const next = new URLSearchParams(current);
        next.delete("page");
        next.delete("from");
        next.delete("to");
        if (range === DEFAULT_RANGE) next.delete("range");
        else next.set("range", range);
        if (range === "custom" && from !== null && to !== null) {
          next.set("from", from);
          next.set("to", to);
        }
        return next;
      },
      { replace: true },
    );
  const reset = () =>
    setParams(
      (current) => {
        const next = new URLSearchParams(current);
        for (const key of ["q", "range", "from", "to", "page"]) next.delete(key);
        return next;
      },
      { replace: true },
    );
  const noMatches = isFiltered(filters) || filters.category !== null;
  const results = (data: NonNullable<typeof list.data>) => (
    <AuditTable
      entries={data.items}
      open={open}
      onToggle={toggle}
      empty={noMatches ? t("audit.noMatches") : t("audit.empty")}
      footer={
        <Pager
          page={data.page}
          pageSize={data.pageSize}
          total={data.total}
          noun={(count) => t("audit.noun", { count })}
          sizes={SIZES}
        />
      }
    />
  );
  return (
    <div className="flex w-full min-w-0 flex-col gap-4">
      <PageHead
        title={t("audit.title")}
        description={t("audit.lead")}
        actions={
          <Button
            type="button"
            variant="outline"
            className="shadow-card h-9 gap-1.5 px-3.5"
            onClick={() => setExporting(true)}
          >
            <Download aria-hidden="true" />
            {t("audit.export")}
          </Button>
        }
      />
      <AuditFilterBar
        filters={filters}
        facets={list.data?.facets}
        rangeButton={rangeButton}
        onCategory={(category: AuditCategory | null) => setFilter("category", category)}
        query={query}
        onSearch={setQuery}
        onClear={() => {
          setQuery("");
          setFilter("q", null);
        }}
        onPreset={(range) => setRange(range, null, null)}
        onCustom={() =>
          setChoosing(
            filters.range === "custom" && filters.from !== null && filters.to !== null
              ? { from: filters.from, to: filters.to }
              : lastSevenDays(new Date(), zone),
          )
        }
        onReset={reset}
      />
      <div
        ref={region}
        tabIndex={-1}
        className="flex min-w-0 flex-col gap-4 outline-none"
      >
        {list.data === undefined ? (
          <QueryStates
            query={list}
            skeleton={<TableSkeleton />}
            failed={t("audit.loadFailed")}
          >
            {results}
          </QueryStates>
        ) : (
          <>
            {list.isError ? (
              <Callout
                tone="danger"
                size="md"
                announce
                actions={
                  <button
                    type="button"
                    className="text-ui font-semibold whitespace-nowrap underline underline-offset-2"
                    onClick={() => void list.refetch()}
                  >
                    {t("common.retry")}
                  </button>
                }
              >
                {t("audit.loadFailed")}
              </Callout>
            ) : null}
            {results(list.data)}
          </>
        )}
      </div>
      <CustomRangeDialog
        open={choosing !== null}
        initial={choosing ?? NO_DAYS}
        returnFocus={rangeButton}
        onOpenChange={(next) => {
          if (!next) setChoosing(null);
        }}
        onApply={({ from, to }) => setRange("custom", from, to)}
      />
      <ExportAuditDialog
        open={exporting}
        filters={filters}
        onOpenChange={setExporting}
      />
    </div>
  );
}
