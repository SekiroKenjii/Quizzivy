import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { ClipboardPaste, FileText, FileUp } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { EmptyState, ListSkeleton, QueryStates } from "@/components/shared/ListState";
import { PageHead } from "@/layouts/shell/PageHead";
import { Segmented } from "@/components/ui/segmented";
import { useIdlePolling, useRefetchOnResume } from "@/hooks/useIdlePolling";
import { useAuthStore } from "@/stores/auth";
import { ImportDownloadMenu } from "../../components/ImportDownloadMenu";
import { Pager } from "@/components/shared/data/Pager";
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
  "@[960px]/import-history:grid-cols-[minmax(200px,2.4fr)_minmax(160px,1fr)_minmax(150px,1.4fr)_minmax(110px,1fr)_minmax(150px,1fr)_32px] @[1240px]/import-history:grid-cols-[minmax(200px,2.4fr)_minmax(160px,1fr)_130px_minmax(150px,1.4fr)_minmax(110px,1fr)_minmax(150px,1fr)_32px]";
const CHIP = "in-data-[scale=deck]:rounded-sm in-data-[scale=deck]:px-2";
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
  const [size] = usePageSize(PAGE_SIZES);
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
      <Button asChild variant="outline">
        <Link to="/teacher/imports/new?source=paste">
          <ClipboardPaste aria-hidden="true" />
          {t("imports.pasteTest")}
        </Link>
      </Button>
      <Button asChild>
        <Link to="/teacher/imports/new">
          <FileUp aria-hidden="true" />
          {t("imports.start")}
        </Link>
      </Button>
    </>
  );

  const results = (data: NonNullable<typeof list.data>) => {
    const pager = (frame: "card" | "plain") => (
      <Pager
        page={data.page}
        pageSize={data.pageSize}
        total={data.total}
        noun={(count) => t("imports.history.noun", { count })}
        sizes={PAGE_SIZES}
        frame={frame}
      />
    );
    if (data.total > 0)
      return data.items.length > 0 ? (
        <div className="@container/import-history min-w-0">
          <Card className="gap-0 overflow-hidden py-0">
            <div role="table" aria-label={t("imports.historyTitle")}>
              <div role="rowgroup" className="hidden @[960px]/import-history:block">
                <div
                  role="row"
                  className={cn(
                    "bg-muted text-meta text-muted-fg grid h-10 items-center gap-3 px-4 font-medium",
                    GRID,
                  )}
                >
                  <span role="columnheader">{t("imports.history.file")}</span>
                  <span role="columnheader">{t("imports.columnStatus")}</span>
                  <span
                    role="columnheader"
                    className="hidden @[1240px]/import-history:block"
                  >
                    {t("imports.upload.source")}
                  </span>
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
              <div role="rowgroup">
                {data.items.map((item) => (
                  <HistoryRow
                    key={item.id}
                    item={item}
                    locale={locale}
                    processing={canStart}
                  />
                ))}
              </div>
            </div>
            {pager("card")}
          </Card>
        </div>
      ) : (
        pager("plain")
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
    <div className="flex w-full min-w-0 flex-col gap-4" data-scale="deck">
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
  const pasted = exam?.format === "text";
  const SourceIcon = pasted ? ClipboardPaste : FileText;
  const name =
    (pasted ? t("imports.history.pastedText") : exam?.filename) ?? item.title;
  return (
    <div
      role="row"
      className={cn(
        "grid grid-cols-[32px_auto_minmax(0,1fr)_auto_32px] items-center gap-2.5 border-t px-4 py-3.5 text-[13.5px] first:border-t-0 @[960px]/import-history:first:border-t",
        "@[960px]/import-history:min-h-[62px] @[960px]/import-history:gap-3 @[960px]/import-history:py-0",
        GRID,
      )}
    >
      <div
        role="cell"
        className="col-[1/5] row-start-1 flex min-w-0 items-start gap-2.5 @[960px]/import-history:col-[1] @[960px]/import-history:items-center"
      >
        <span className="bg-info-soft text-info-ink grid size-8 shrink-0 place-items-center rounded-lg">
          <SourceIcon aria-hidden="true" className="size-4" />
        </span>
        <div className="min-w-0">
          <Link
            to={importHref(item)}
            title={item.title}
            aria-label={
              name === item.title
                ? undefined
                : t("imports.history.fileNamed", { name, title: item.title })
            }
            className="block truncate text-[14px] font-medium hover:underline @[960px]/import-history:text-[13.5px]"
          >
            {name}
          </Link>
          <p
            className="text-muted-fg text-xs leading-[1.45] break-words @[960px]/import-history:truncate @[960px]/import-history:leading-normal"
            title={formatDateTime(item.createdAt, locale)}
          >
            {creator === null ? null : <>{creator} · </>}
            {formatRelative(item.createdAt, locale)}
            {pasted && exam.characters !== undefined ? (
              <>
                {" · "}
                {t("imports.history.characters", {
                  count: exam.characters,
                  n: new Intl.NumberFormat(locale).format(exam.characters),
                })}
              </>
            ) : null}
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
        className="hidden min-w-0 @[960px]/import-history:col-[2] @[960px]/import-history:row-start-1 @[960px]/import-history:flex"
      >
        <ImportStatusBadge status={item.status} />
      </div>
      <div
        role="cell"
        className="text-muted-fg hidden min-w-0 truncate text-[13px] @[1240px]/import-history:col-[3] @[1240px]/import-history:row-start-1 @[1240px]/import-history:block"
      >
        {sourceKind(exam?.format, t)}
      </div>
      <div
        role="cell"
        className="text-muted-fg col-[2/6] row-start-2 flex min-w-0 flex-wrap items-center gap-1.5 text-[12.5px] @[960px]/import-history:col-[3] @[960px]/import-history:row-start-1 @[960px]/import-history:text-[13px] @[1240px]/import-history:col-[4]"
      >
        <span className="inline-flex @[960px]/import-history:hidden">
          <ImportStatusBadge status={item.status} />
        </span>
        <ReviewFindings item={item} />
      </div>
      <div
        role="cell"
        className="col-[2/4] row-start-3 min-w-0 truncate text-[13px] @[960px]/import-history:col-[4] @[960px]/import-history:row-start-1 @[960px]/import-history:text-[13.5px] @[1240px]/import-history:col-[5]"
      >
        <span className="text-muted-fg @[960px]/import-history:hidden">
          {t("imports.history.test")}:{" "}
        </span>
        {item.testId ? (
          <Link
            to={`/teacher/tests/${item.testId}/edit`}
            aria-label={t("imports.history.openTest", { title: item.title })}
            className="font-medium hover:underline"
          >
            {item.title}
          </Link>
        ) : (
          <>
            <span className="text-muted-fg @[960px]/import-history:hidden">
              {t("imports.history.noTest")}
            </span>
            <span
              aria-hidden="true"
              className="text-muted-fg hidden @[960px]/import-history:inline"
            >
              —
            </span>
            <span className="sr-only hidden @[960px]/import-history:inline">
              {t("imports.history.noTest")}
            </span>
          </>
        )}
      </div>
      <div
        role="cell"
        className="col-[4/6] row-start-3 justify-self-end @[960px]/import-history:col-[5] @[960px]/import-history:row-start-1 @[960px]/import-history:justify-self-start @[1240px]/import-history:col-[6]"
      >
        <Button
          asChild
          size="sm"
          variant="outline"
          className="h-auto min-h-8 max-w-full rounded-md py-1 leading-[normal] whitespace-normal in-data-[scale=deck]:h-auto in-data-[scale=deck]:rounded-md"
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
        className="col-[5] row-start-1 self-start @[960px]/import-history:col-[6] @[960px]/import-history:self-center @[1240px]/import-history:col-[7]"
      >
        <ImportDownloadMenu item={item} />
      </div>
    </div>
  );
}

function sourceKind(format: string | undefined, t: TFunction): string {
  if (format === "text") return t("imports.history.pastedText");
  if (format === "pdf") return t("imports.history.formatPdf");
  if (format === "docx" || format === "doc") return t("imports.history.formatWord");
  return "—";
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
    <>
      {needsAction > 0 ? (
        <Badge variant="danger" className={CHIP}>
          {t("imports.history.needsAction", { count: needsAction })}
        </Badge>
      ) : null}
      {toConfirm > 0 ? (
        <Badge variant="warning" className={CHIP}>
          {t("imports.history.toConfirm", { count: toConfirm })}
        </Badge>
      ) : null}
    </>
  );
}
