import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { ClipboardPaste, FileText, FileUp } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { EmptyState, ListSkeleton, QueryStates } from "@/components/shared/ListState";
import { PageHead } from "@/layouts/shell/PageHead";
import { Segmented } from "@/components/ui/segmented";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";
import { useAuthStore } from "@/stores/auth";
import { ImportDownloadMenu } from "../../components/ImportDownloadMenu";
import { Pager } from "@/components/shared/Pager";
import { SearchInput } from "@/components/shared/SearchInput";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage, usePageSize } from "@/hooks/usePage";
import {
  formatDateTime,
  formatRelative,
  useDisplayTimeZone,
} from "@/lib/i18n/datetime";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";
import { useDebounced } from "@/lib/useDebounced";
import { listWordImports, type WordImport } from "../../api";
import { useImportAvailability } from "../../availability";
import { ImportStatusBadge } from "../../components/ImportStatusBadge";
import { ProcessingOffNotice } from "../../components/ProcessingOffNotice";
import { StaleNotice } from "../../components/StaleNotice";
import {
  importActionHref,
  importHref,
  isActiveStatus,
  isImportStatus,
} from "../../status";

type HistoryItem = Awaited<ReturnType<typeof listWordImports>>["items"][number];

const PAGE_SIZES = [20, 30, 50] as const;
const HISTORY_FILTERS = [
  "processing",
  "needs_review",
  "failed",
  "committed",
  "cancelled",
] as const;
const FACET_KEYS = {
  processing: "processing",
  needs_review: "needsReview",
  failed: "failed",
  committed: "committed",
  cancelled: "cancelled",
} as const;
const PROCESSING_STATUSES = [
  "awaiting_sources",
  "queued",
  "processing",
  "committing",
] as const;
const GRID =
  "@[960px]/import-history:grid-cols-[minmax(200px,2.4fr)_minmax(160px,1fr)_minmax(150px,1.4fr)_minmax(110px,1fr)_minmax(150px,1fr)_32px]";
const ALL = "all";
const HISTORY_POLL_MS = 5000;

export default function ImportsListPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const locale = useLocale();
  const { params, setParams, setFilter } = useListFilters();
  const availability = useImportAvailability();
  const canStart = availability === "on";
  const poll = useIdlePolling(HISTORY_POLL_MS);
  const [size, setSize] = usePageSize(PAGE_SIZES);
  const query = params.get("q") ?? "";
  const search = useDebounced(query.trim(), 300);
  const requested = params.get("status");
  const status = isImportStatus(requested) ? requested : null;
  const [page] = usePage(JSON.stringify({ search, status }));
  const list = useQuery({
    queryKey: ["word-imports", { search, status, page, size }],
    queryFn: ({ signal }) =>
      listWordImports(
        {
          page,
          limit: size,
          ...(status === null
            ? {}
            : {
                status: status === "processing" ? [...PROCESSING_STATUSES] : [status],
              }),
          ...(search === "" ? {} : { q: search }),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
    refetchInterval: (current) =>
      current.state.data?.items.some((item) => isActiveStatus(item.status))
        ? poll
        : false,
    refetchIntervalInBackground: false,
  });
  useRefetchOnResume(list.refetch);
  const facets = list.isPlaceholderData ? undefined : list.data?.facets;
  const filtered = search !== "" || status !== null;
  const clearFilters = () =>
    setParams(
      (current) => {
        const next = new URLSearchParams(current);
        next.delete("q");
        next.delete("status");
        next.delete("page");
        return next;
      },
      { replace: true },
    );
  const start = (
    <>
      <Button asChild size="sm" variant="outline">
        <Link to="/teacher/imports/new?source=paste">
          <ClipboardPaste aria-hidden="true" />
          {t("imports.pasteTest")}
        </Link>
      </Button>
      <Button asChild size="sm">
        <Link to="/teacher/imports/new">
          <FileUp aria-hidden="true" />
          {t("imports.start")}
        </Link>
      </Button>
    </>
  );

  const results = (data: NonNullable<typeof list.data>) => {
    if (data.items.length > 0)
      return (
        <>
          <div className="@container/import-history min-w-0">
            <Card
              role="table"
              aria-label={t("imports.historyTitle")}
              className="gap-0 overflow-hidden py-0"
            >
              <div role="rowgroup" className="hidden @[960px]/import-history:block">
                <div
                  role="row"
                  className={cn(
                    "bg-muted text-meta text-muted-fg grid items-center gap-3 px-4 py-2.5",
                    GRID,
                  )}
                >
                  <span role="columnheader">{t("imports.history.file")}</span>
                  <span role="columnheader">{t("imports.columnStatus")}</span>
                  <span role="columnheader">{t("imports.history.review")}</span>
                  <span role="columnheader">{t("imports.history.test")}</span>
                  <span role="columnheader" className="sr-only">
                    {t("imports.columnActions")}
                  </span>
                  <span role="columnheader" className="sr-only">
                    {t("common.actions")}
                  </span>
                </div>
              </div>
              <div role="rowgroup" className="divide-border divide-y">
                {data.items.map((item) => (
                  <HistoryRow
                    key={item.id}
                    item={item}
                    locale={locale}
                    processing={canStart}
                  />
                ))}
              </div>
            </Card>
          </div>
          <Pager page={data.page} pageSize={data.pageSize} total={data.total} />
        </>
      );
    if (filtered)
      return (
        <EmptyState
          action={
            <Button size="sm" variant="outline" onClick={clearFilters}>
              {t("imports.clearFilters")}
            </Button>
          }
        >
          {t("imports.noMatches")}
        </EmptyState>
      );
    if (!canStart) return <EmptyState>{t("imports.empty")}</EmptyState>;
    return (
      <EmptyState hint={t("imports.emptyHint")} action={start}>
        {t("imports.empty")}
      </EmptyState>
    );
  };

  return (
    <div
      className="mx-auto flex w-full max-w-[1320px] min-w-0 flex-col gap-4"
      data-scale="deck"
    >
      <PageHead
        className="[&>div:first-child]:max-w-[720px] [&>div:first-child]:flex-[1_1_360px]"
        title={t("imports.historyTitle")}
        description={t("imports.historyHint")}
        actions={canStart ? start : undefined}
      />
      {availability === "reviewOnly" ? (
        <ProcessingOffNotice>
          {t("imports.availability.processingOff")}
        </ProcessingOffNotice>
      ) : null}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <Segmented
          label={t("imports.statusFilter")}
          scroll
          value={status ?? ALL}
          options={[
            { value: ALL, label: t("imports.allStatuses"), count: facets?.all },
            ...HISTORY_FILTERS.map((value) => ({
              value,
              label: t(`imports.status.${value}`),
              count: facets?.[FACET_KEYS[value]],
            })),
          ]}
          onChange={(value) => setFilter("status", value === ALL ? null : value)}
        />
        <SearchInput
          className="max-w-[280px] min-w-[200px] flex-[1_1_280px]"
          value={query}
          onChange={(value) => setFilter("q", value)}
          placeholder={t("imports.searchPlaceholder")}
        />
      </div>
      <Select value={String(size)} onValueChange={(value) => setSize(Number(value))}>
        <SelectTrigger
          className="w-auto self-end"
          aria-label={t("imports.history.pageSize")}
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {PAGE_SIZES.map((value) => (
              <SelectItem key={value} value={String(value)}>
                {t("imports.history.rows", { count: value })}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>

      {list.data === undefined ? (
        <QueryStates
          query={list}
          skeleton={<ListSkeleton />}
          failed={t("imports.historyFailed")}
        >
          {results}
        </QueryStates>
      ) : (
        <>
          {list.isError ? <StaleNotice onRetry={() => void list.refetch()} /> : null}
          {results(list.data)}
        </>
      )}
    </div>
  );
}

function HistoryRow({
  item,
  locale,
  processing,
}: Readonly<{ item: HistoryItem; locale: Locale; processing: boolean }>) {
  const { t } = useTranslation();
  const creator = useAuthStore((state) =>
    state.user?.id === item.createdBy ? state.user.fullName : null,
  );
  const action =
    item.status === "awaiting_sources" && !processing ? "view" : item.status;
  const exam = item.sources.find((source) => source.role === "exam");
  const key = item.sources.find((source) => source.role === "answer_key");
  return (
    <div
      role="row"
      className={cn(
        "grid grid-cols-[minmax(0,1fr)_32px] items-center gap-2 px-4 py-3.5 text-sm @[960px]/import-history:gap-3",
        GRID,
      )}
    >
      <div
        role="cell"
        className="flex min-w-0 items-start gap-2.5 @[960px]/import-history:col-start-1"
      >
        <span className="bg-info-soft text-info-ink grid size-8 shrink-0 place-items-center rounded-lg">
          <FileText aria-hidden="true" className="size-4" />
        </span>
        <div className="min-w-0">
          {exam ? (
            <p className="truncate font-medium" title={exam.filename}>
              {exam.filename}
            </p>
          ) : null}
          <Link
            to={importHref(item)}
            title={item.title}
            className="block truncate font-medium hover:underline"
          >
            {item.title}
          </Link>
          <p
            className="text-muted-fg text-xs break-words"
            title={formatDateTime(item.createdAt, locale)}
          >
            {creator === null ? null : <>{creator} · </>}
            {formatRelative(item.createdAt, locale)}
            {key ? (
              <> · {t("imports.history.withKey", { name: key.filename })}</>
            ) : null}
          </p>
          {item.pendingUploads > 0 ? (
            <p className="text-muted-fg text-xs">
              {t("imports.pendingUploads", { count: item.pendingUploads })}
            </p>
          ) : null}
        </div>
      </div>
      <div
        role="cell"
        className="col-start-1 flex min-w-0 flex-wrap @[960px]/import-history:col-start-2 @[960px]/import-history:row-start-1"
      >
        <ImportStatusBadge status={item.status} />
      </div>
      <div
        role="cell"
        className="text-muted-fg col-start-1 min-w-0 text-xs @[960px]/import-history:col-start-3 @[960px]/import-history:row-start-1"
      >
        <ReviewFindings item={item} />
      </div>
      <div
        role="cell"
        className="col-start-1 min-w-0 @[960px]/import-history:col-start-4 @[960px]/import-history:row-start-1"
      >
        <span className="text-muted-fg @[960px]/import-history:hidden">
          {t("imports.history.test")}:{" "}
        </span>
        {item.testId ? (
          <Link
            to={`/teacher/tests/${item.testId}/edit`}
            aria-label={t("imports.history.openTest", { title: item.title })}
            className="block truncate font-medium hover:underline"
          >
            {item.title}
          </Link>
        ) : (
          <span className="text-muted-fg">{t("imports.history.noTest")}</span>
        )}
      </div>
      <div
        role="cell"
        className="col-start-1 @[960px]/import-history:col-start-5 @[960px]/import-history:row-start-1"
      >
        <Button
          asChild
          size="xs"
          className="h-auto min-h-8 max-w-full py-1.5 whitespace-normal"
          variant={item.status === "needs_review" ? "default" : "outline"}
        >
          <Link
            to={importActionHref(item)}
            aria-label={t(`imports.rowAction.${action}Named`, { title: item.title })}
          >
            {t(`imports.rowAction.${action}`)}
          </Link>
        </Button>
      </div>
      <div
        role="cell"
        className="col-start-2 row-start-1 self-start @[960px]/import-history:col-start-6 @[960px]/import-history:self-center"
      >
        <ImportDownloadMenu item={item} />
      </div>
    </div>
  );
}

function reviewLabel(item: WordImport, t: TFunction): string {
  if (item.filesRemovedAt) return t("imports.history.filesRemoved");
  if (item.status === "needs_review") return t("imports.history.openReview");
  if (isActiveStatus(item.status)) return t("imports.history.recognising");
  return "—";
}

function ReviewFindings({ item }: Readonly<{ item: HistoryItem }>) {
  const { t } = useTranslation();
  if (
    item.filesRemovedAt ||
    item.status !== "needs_review" ||
    item.reviewCounts === null
  )
    return <>{reviewLabel(item, t)}</>;
  const { needsAction, toConfirm } = item.reviewCounts;
  if (needsAction === 0 && toConfirm === 0)
    return <>{t("imports.history.nothingOpen")}</>;
  return (
    <div className="flex min-w-0 flex-wrap gap-1.5">
      {needsAction > 0 ? (
        <Badge variant="danger" className="in-data-[scale=deck]:rounded-sm">
          {t("imports.history.needsAction", { count: needsAction })}
        </Badge>
      ) : null}
      {toConfirm > 0 ? (
        <Badge variant="warning" className="in-data-[scale=deck]:rounded-sm">
          {t("imports.history.toConfirm", { count: toConfirm })}
        </Badge>
      ) : null}
    </div>
  );
}
