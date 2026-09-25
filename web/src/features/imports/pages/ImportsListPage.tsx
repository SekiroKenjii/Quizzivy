import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { FileUp } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { EmptyState, ListSkeleton, QueryStates } from "@/components/shared/ListState";
import { PageHeader } from "@/components/shared/PageHeader";
import { Pager } from "@/components/shared/Pager";
import { SearchInput } from "@/components/shared/SearchInput";
import { useListFilters } from "@/hooks/useListFilters";
import { usePage } from "@/hooks/usePage";
import { formatDateTime, formatRelative } from "@/lib/i18n/datetime";
import type { Locale } from "@/lib/i18n";
import { useLocale } from "@/lib/i18n/useLocale";
import { useDebounced } from "@/lib/useDebounced";
import { listWordImports, type WordImport } from "../api";
import { ImportStatusBadge } from "../components/ImportStatusBadge";
import { StaleNotice } from "../components/StaleNotice";
import {
  IMPORT_STATUSES,
  importActionHref,
  importHref,
  isActiveStatus,
  isImportStatus,
} from "../status";

const PAGE_SIZE = 20;
const ALL = "all";
const HISTORY_POLL_MS = 5000;

export default function ImportsListPage() {
  const { t } = useTranslation();
  const locale = useLocale();
  const { params, setParams, setFilter } = useListFilters();
  const query = params.get("q") ?? "";
  const search = useDebounced(query.trim(), 300);
  const requested = params.get("status");
  const status = isImportStatus(requested) ? requested : null;
  const [page] = usePage(JSON.stringify({ search, status }));
  const list = useQuery({
    queryKey: ["word-imports", { search, status, page }],
    queryFn: ({ signal }) =>
      listWordImports(
        {
          page,
          limit: PAGE_SIZE,
          ...(status === null ? {} : { status }),
          ...(search === "" ? {} : { q: search }),
        },
        signal,
      ),
    placeholderData: keepPreviousData,
    refetchInterval: (current) =>
      current.state.data?.items.some((item) => isActiveStatus(item.status))
        ? HISTORY_POLL_MS
        : false,
    refetchIntervalInBackground: false,
  });
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
    <Button asChild size="sm">
      <Link to="/admin/imports/new">
        <FileUp aria-hidden="true" />
        {t("imports.start")}
      </Link>
    </Button>
  );

  const results = (data: NonNullable<typeof list.data>) => {
    if (data.items.length > 0)
      return (
        <>
          <Card className="gap-0 overflow-hidden py-0">
            <Table className="min-w-[36rem] table-fixed">
              <TableHeader>
                <TableRow>
                  <TableHead>{t("imports.columnTitle")}</TableHead>
                  <TableHead className="w-44">{t("imports.columnStatus")}</TableHead>
                  <TableHead className="hidden w-[24%] xl:table-cell">
                    {t("imports.columnSources")}
                  </TableHead>
                  <TableHead className="hidden w-28 xl:table-cell">
                    {t("imports.columnCreated")}
                  </TableHead>
                  <TableHead className="w-28">{t("imports.columnUpdated")}</TableHead>
                  <TableHead className="w-40 text-right">
                    <span className="sr-only">{t("imports.columnActions")}</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.items.map((item) => (
                  <HistoryRow key={item.id} item={item} locale={locale} />
                ))}
              </TableBody>
            </Table>
          </Card>
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
    return (
      <EmptyState hint={t("imports.emptyHint")} action={start}>
        {t("imports.empty")}
      </EmptyState>
    );
  };

  return (
    <div className="space-y-4">
      <PageHeader
        title={t("imports.historyTitle")}
        backTo="/admin/tests"
        backLabel={t("imports.backToTests")}
        actions={start}
      />
      <p className="text-muted-foreground text-sm">{t("imports.historyHint")}</p>

      <div className="flex flex-wrap items-center gap-3">
        <SearchInput
          className="min-w-48 flex-1"
          value={query}
          onChange={(value) => setFilter("q", value)}
          placeholder={t("imports.searchPlaceholder")}
        />
        <Select
          value={status ?? ALL}
          onValueChange={(value) => setFilter("status", value === ALL ? null : value)}
        >
          <SelectTrigger
            className="w-auto min-w-44"
            aria-label={t("imports.statusFilter")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              <SelectItem value={ALL}>{t("imports.allStatuses")}</SelectItem>
              {IMPORT_STATUSES.map((value) => (
                <SelectItem key={value} value={value}>
                  {t(`imports.status.${value}`)}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
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

function HistoryRow({ item, locale }: Readonly<{ item: WordImport; locale: Locale }>) {
  const { t } = useTranslation();
  return (
    <TableRow>
      <TableCell>
        <Link
          to={importHref(item)}
          title={item.title}
          className="block truncate font-medium hover:underline"
        >
          {item.title}
        </Link>
      </TableCell>
      <TableCell>
        <ImportStatusBadge status={item.status} />
      </TableCell>
      <TableCell className="text-muted-foreground hidden text-xs xl:table-cell">
        {item.sources.length === 0 && item.pendingUploads === 0 ? (
          <span>{t("imports.noSources")}</span>
        ) : (
          <ul className="space-y-0.5">
            {item.sources.map((source) => (
              <li key={source.id} className="flex min-w-0 gap-1.5">
                <span className="shrink-0">{t(`imports.role.${source.role}`)}</span>
                <span className="text-foreground truncate" title={source.filename}>
                  {source.filename}
                </span>
              </li>
            ))}
            {item.pendingUploads > 0 ? (
              <li className="truncate">
                {t("imports.pendingUploads", { count: item.pendingUploads })}
              </li>
            ) : null}
          </ul>
        )}
      </TableCell>
      <TableCell
        className="text-muted-foreground hidden truncate xl:table-cell"
        title={formatDateTime(item.createdAt, locale)}
      >
        {formatRelative(item.createdAt, locale)}
      </TableCell>
      <TableCell
        className="text-muted-foreground truncate"
        title={formatDateTime(item.updatedAt, locale)}
      >
        {formatRelative(item.updatedAt, locale)}
      </TableCell>
      <TableCell className="text-right">
        <Button
          asChild
          size="xs"
          variant={item.status === "needs_review" ? "default" : "outline"}
        >
          <Link
            to={importActionHref(item)}
            aria-label={t(`imports.rowAction.${item.status}Named`, {
              title: item.title,
            })}
          >
            {t(`imports.rowAction.${item.status}`)}
          </Link>
        </Button>
      </TableCell>
    </TableRow>
  );
}
